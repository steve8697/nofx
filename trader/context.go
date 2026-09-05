package trader

import (
	"fmt"
	"log"
	"aetheris/decision"
	"aetheris/logger"
	"aetheris/market"
	"aetheris/pool"
	"aetheris/utils"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ContextBuilder 负责构建交易上下文
type ContextBuilder struct {
	mu                    sync.RWMutex // 🔒 保護併發存取（孤兒監控 goroutine 與主循環）
	trader                Trader
	decisionLogger        *logger.DecisionLogger
	config                AutoTraderConfig
	positionFirstSeenTime map[string]int64
	startTime             time.Time
	dataProvider          market.DataProvider
	timeProvider          utils.TimeProvider
	// 交易历史追踪（冷却期检查）
	lastCloseTime   time.Time   // 上次平仓时间
	tradeTimestamps []time.Time // 过去的交易时间戳（用于计算1小时内交易次数）
	consecutiveLoss int         // 连续亏损次数
	consecutiveWait int         // 连续wait次数（用于触发 analysis paralysis 规则）
	persistence     *PersistenceManager
	liquidityEngine *market.LiquidityEngine // 🔥 持续化流动性引擎
	dailyPnL           float64                 // 🔥 当日累计盈亏 (持久化)
	lastResetTime      time.Time               // 🔥 上次重置日盈亏时间 (持久化)
	callCount          int
	decisionHistory    []decision.FullDecision
	peakPnLCache       map[string]float64
	peakEquity         float64 // 账户历史最高净值 (High-Water Mark)
	stopUntil          time.Time
	activeTradeReasons map[string]string // 🔧 内存持久化开仓理由 (Symbol_Side -> Reason)
}

// NewContextBuilder 创建ContextBuilder
func NewContextBuilder(trader Trader, decisionLogger *logger.DecisionLogger, config AutoTraderConfig, startTime time.Time, dataProvider market.DataProvider, timeProvider utils.TimeProvider, liquidityEngine *market.LiquidityEngine) *ContextBuilder {
	pm := NewPersistenceManager("data", config.ID) // 傳入交易員 ID 實現狀態隔離
	loadedState, err := pm.LoadPositionState()

	firstSeenTimes := make(map[string]int64)
	consecutiveLoss := 0
	dailyPnL := 0.0
	var lastCloseTime time.Time
	var lastResetTime time.Time
	callCount := 0
	var decisionHistory []decision.FullDecision
	var peakPnLCache map[string]float64
	var stopUntil time.Time
	peakEquity := 0.0
	activeTradeReasons := make(map[string]string)

	if err != nil {
		log.Printf("⚠️ 加载持仓状态失败: %v", err)
	} else {
		if loadedState.FirstSeenTimes != nil {
			firstSeenTimes = loadedState.FirstSeenTimes
		}
		consecutiveLoss = loadedState.ConsecutiveLosses
		dailyPnL = loadedState.DailyLoss
		if loadedState.LastTradeTime > 0 {
			lastCloseTime = time.UnixMilli(loadedState.LastTradeTime)
		}
		if loadedState.LastResetTime > 0 {
			lastResetTime = time.Unix(loadedState.LastResetTime, 0)
		}
		callCount = loadedState.CallCount
		decisionHistory = loadedState.DecisionHistory
		peakPnLCache = loadedState.PeakPnLCache
		peakEquity = loadedState.PeakEquity
		if loadedState.StopUntil > 0 {
			stopUntil = time.Unix(loadedState.StopUntil, 0)
		}
		if loadedState.ActiveTradeReasons != nil {
			activeTradeReasons = loadedState.ActiveTradeReasons
		}

		log.Printf("✓ 已加载历史状态: %d个持仓记录, 连续亏损%d次, 今日盈亏%.2f, 上次平仓%s, 周期#%d, 历史决策%d个, peak快取%d個, 活跃开仓理由%d个, 历史最高净值%.2f",
			len(firstSeenTimes), consecutiveLoss, dailyPnL, formatTimeAgo(lastCloseTime), callCount, len(decisionHistory), len(peakPnLCache), len(activeTradeReasons), peakEquity)
	}

	if peakEquity <= 0 && config.InitialBalance > 0 {
		peakEquity = config.InitialBalance
	}

	if decisionHistory == nil {
		decisionHistory = make([]decision.FullDecision, 0)
	}
	if peakPnLCache == nil {
		peakPnLCache = make(map[string]float64)
	}

	return &ContextBuilder{
		trader:                trader,
		decisionLogger:        decisionLogger,
		config:                config,
		positionFirstSeenTime: firstSeenTimes,
		startTime:             startTime,
		dataProvider:          dataProvider,
		timeProvider:          timeProvider,
		tradeTimestamps:       []time.Time{},
		consecutiveLoss:       consecutiveLoss,
		lastCloseTime:         lastCloseTime,
		persistence:           pm,
		liquidityEngine:       liquidityEngine, // 🔥 注入引擎
		dailyPnL:              dailyPnL,        // 🔥 初始化 persistence loaded value
		lastResetTime:         lastResetTime,   // 🔥 初始化 persistence loaded value
		callCount:             callCount,
		decisionHistory:       decisionHistory,
		peakPnLCache:          peakPnLCache,
		peakEquity:            peakEquity,
		stopUntil:             stopUntil,
		activeTradeReasons:    activeTradeReasons,
	}
}

// ReconcileOfflineCloses 启动时对账：检查离线期间的平仓 (The "Coma" Fix)
func (cb *ContextBuilder) ReconcileOfflineCloses() {
	cb.mu.RLock()
	keysToCheck := make(map[string]string)
	for k, v := range cb.activeTradeReasons {
		keysToCheck[k] = v
	}
	cb.mu.RUnlock()

	if len(keysToCheck) == 0 {
		return
	}

	log.Printf("🔍 启动对账 (Startup Reconciliation): 检查 %d 个记忆中的持仓...", len(keysToCheck))

	// 获取当前实际持仓
	positions, err := cb.trader.GetPositions()
	if err != nil {
		log.Printf("⚠️ 启动对账失败: 无法获取当前持仓: %v", err)
		return
	}

	currentHoldings := make(map[string]bool)
	for _, pos := range positions {
		key := fmt.Sprintf("%s_%s", pos["symbol"], pos["side"])
		currentHoldings[key] = true
	}

	// 检查记忆中存在但实际不存在的持仓
	reconciledCount := 0
	for key, reason := range keysToCheck {
		if !currentHoldings[key] {
			log.Printf("👻 发现离线平仓 (Offline Close): %s (原理由: %s)", key, reason)

			parts := strings.Split(key, "_")
			if len(parts) == 2 {
				symbol := parts[0]
				// 尝试获取成交历史 (过去24小时) 并同步累计至 dailyPnL
				cb.detectAndLogPassiveClose(symbol, key, reason, 24*60)
				cb.DeletePeakPnL(symbol)
			}

			// 确保无论是否匹配到成交历史，都彻底清除记忆并持久化最新真实状态
			cb.RemoveEntryReason(key)
			cb.DeletePositionFirstSeenTime(key)
			reconciledCount++
		}
	}

	if reconciledCount > 0 {
		log.Printf("✅ 启动对账完成: 修复并清理了 %d 个离线平仓记录", reconciledCount)
	} else {
		log.Printf("✅ 启动对账完成: 无异常")
	}
}

// SetTrader 更新交易器接口（用于回测注入）
func (cb *ContextBuilder) SetTrader(trader Trader) {
	cb.trader = trader
}

// Build 构建交易上下文
func (cb *ContextBuilder) Build(callCount int, recentDecisions []decision.FullDecision) (*decision.Context, error) {
	// 1. 获取账户信息
	balance, err := cb.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	// 获取账户字段
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity = 钱包余额 + 未实现盈亏
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 2. 获取持仓信息
	positions, err := cb.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0

	// 当前持仓的key集合（用于清理已平仓的记录）
	currentPositionKeys := make(map[string]bool)
	stateChanged := false

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // 空仓数量为负，转为正数
		}

		// 跳过已平仓的持仓（quantity = 0），防止"幽灵持仓"传递给AI
		if quantity == 0 {
			continue
		}

		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		// 计算盈亏百分比
		pnlPct := 0.0
		if entryPrice > 0 {
			if side == "long" {
				pnlPct = ((markPrice - entryPrice) / entryPrice) * 100
			} else {
				pnlPct = ((entryPrice - markPrice) / entryPrice) * 100
			}
		}

		// 计算占用保证金（估算）
		leverage := 10 // 默认值，实际应该从持仓信息获取
		if lev, ok := pos["leverage"].(float64); ok && lev > 0 {
			leverage = int(lev)
		}
		if leverage <= 0 {
			leverage = 1
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// 跟踪持仓首次出现时间 — 🔒 加鎖保護（與孤兒監控 goroutine 併發安全）
		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true
		cb.mu.Lock()
		if _, exists := cb.positionFirstSeenTime[posKey]; !exists {
			// 新持仓，记录当前时间
			cb.positionFirstSeenTime[posKey] = cb.timeProvider.Now().UnixMilli()
			stateChanged = true
		}
		updateTime := cb.positionFirstSeenTime[posKey]
		cb.mu.Unlock()

		// 🔒 获取当前持仓的止损止盈（解决"止损盲点"问题）
		stopLoss, takeProfit, err := cb.trader.GetOrderProtection(symbol, side)
		if err != nil {
			log.Printf("⚠️  获取止损止盈失败(%s): %v", symbol, err)
			// 继续执行，且SL/TP默认为0
		} else {
			// 将0值转为浮点数保留（如果API返回0，说明没设）
		}

		positionInfos = append(positionInfos, decision.PositionInfo{
			Symbol:           symbol,
			Side:             side,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			Quantity:         quantity,
			Leverage:         leverage,
			UnrealizedPnL:    unrealizedPnl,
			UnrealizedPnLPct: pnlPct,
			LiquidationPrice: liquidationPrice,
			MarginUsed:       marginUsed,
			UpdateTime:       updateTime,
			StopLoss:         stopLoss,
			TakeProfit:       takeProfit,

			EntryReasoning: cb.GetEntryReason(symbol, side), // 注入开仓理由
		})
	}


	// 清理已平仓的持仓记录 — 🔒 加鎖保護整個清理循環
	cb.mu.Lock()
	for key := range cb.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			// 🟢 P18: 被動平倉偵測 (Passive Trade Detection)
			parts := strings.Split(key, "_")
			if len(parts) == 2 {
				symbol := parts[0]
				side := parts[1]
				reason := cb.GetEntryReason(symbol, side)

				// 偵測並記錄被動平倉
				cb.detectAndLogPassiveClose(symbol, key, reason, 60)

				// 清理記憶
				cb.RemoveEntryReason(key)

				// 🛡️ 清除該幣種的盈虧峰值快取，徹底消除幽靈回撤止損
				delete(cb.peakPnLCache, symbol)
			}

			delete(cb.positionFirstSeenTime, key)
			stateChanged = true
		}
	}
	cb.mu.Unlock()
	// 3. 检查是否有状态变更，如有则保存
	if stateChanged {
		if err := cb.saveState(); err != nil {
			log.Printf("⚠️ 保存状态失败: %v", err)
		}
	}

	// 3. 获取交易员的候选币种池
	candidateCoins, err := cb.getCandidateCoins()
	if err != nil {
		return nil, fmt.Errorf("获取候选币种失败: %w", err)
	}

	// 4. 计算总盈亏与高水位峰值回撤
	totalPnL := totalEquity - cb.config.InitialBalance
	totalPnLPct := 0.0
	if cb.config.InitialBalance > 0 {
		totalPnLPct = (totalPnL / cb.config.InitialBalance) * 100
	}

	cb.mu.Lock()
	if totalEquity > cb.peakEquity {
		cb.peakEquity = totalEquity
		go cb.saveState()
	}
	currentPeak := cb.peakEquity
	cb.mu.Unlock()

	peakDrawdownPct := 0.0
	if currentPeak > 0 {
		peakDrawdownPct = ((totalEquity - currentPeak) / currentPeak) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. 分析历史表现（最近1000个周期，约7天）
	// 假设每10分钟一个周期，1000个周期 = 10000分钟 ≈ 7天，足够覆盖近期交易
	performance, err := cb.decisionLogger.AnalyzePerformance(1000)
	if err != nil {
		log.Printf("⚠️  分析历史表现失败: %v", err)
		// 不影响主流程，继续执行（但设置performance为nil以避免传递错误数据）
		performance = nil
	}

	// 6. 计算冷却期相关信息
	// 清理过期的交易时间戳（只保留1小时内的）
	cb.cleanupTradeTimestamps()
	tradesInHour := len(cb.tradeTimestamps)

	// 7. 构建上下文
	ctx := &decision.Context{
		CurrentTime:     cb.timeProvider.Now().Format("2006-01-02 15:04:05"),
		RuntimeMinutes:  int(cb.timeProvider.Now().Sub(cb.startTime).Minutes()), // 使用 Sub 来计算时间差
		CallCount:       callCount,
		BTCETHLeverage:  cb.config.BTCETHLeverage,  // 使用配置的杠杆倍数
		AltcoinLeverage: cb.config.AltcoinLeverage, // 使用配置的杠杆倍数
		Account: decision.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			PeakEquity:       currentPeak,
			PeakDrawdownPct:  peakDrawdownPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
			UnrealizedPnL:    totalUnrealizedProfit, // ✅ 记录未实现盈亏
		},
		Positions:      positionInfos,
		CandidateCoins: candidateCoins,
		Performance:    performance, // 添加历史表现分析
		DataProvider:   cb.dataProvider,
		PromptRules:    cb.config.PromptRules,
		TradeHistory: &decision.TradeHistory{
			LastCloseTime:   cb.lastCloseTime,
			TradesInHour:    tradesInHour,
			LastTradeTime:   cb.getLastTradeTime(),
			ConsecutiveLoss: cb.consecutiveLoss,
			ConsecutiveWait: cb.consecutiveWait,
		},
		RecentDecisions: recentDecisions,
		LiquidityEngine: cb.liquidityEngine, // 🔥 注入持久化引擎
	}

	// 💰 添加交易手续费信息（用于AI决策时考虑成本）
	if cb.trader != nil {
		makerFee, takerFee := cb.trader.GetTradingFees()
		roundTripFee := takerFee * 2 // 开仓 + 平仓都用Taker费率（保守估计）
		ctx.TradingFeeInfo = &decision.TradingFeeInfo{
			MakerFeeRate:     makerFee,
			TakerFeeRate:     takerFee,
			RoundTripFeeRate: roundTripFee,
			MinProfitPct:     roundTripFee * 100 * 2, // 至少赚2倍手续费才值得
			ExchangeName:     cb.config.Exchange,
		}
		log.Printf("💰 手续费信息: Exchange=%s, Maker=%.4f%%, Taker=%.4f%%, 来回=%.4f%%, 最小止盈距离=%.4f%%",
			cb.config.Exchange,
			makerFee*100, takerFee*100, roundTripFee*100, ctx.TradingFeeInfo.MinProfitPct)
	}

	return ctx, nil
}

// cleanupTradeTimestamps 清理过期的交易时间戳（只保留1小时内的）
func (cb *ContextBuilder) cleanupTradeTimestamps() {
	oneHourAgo := cb.timeProvider.Now().Add(-1 * time.Hour)
	var validTimestamps []time.Time
	for _, ts := range cb.tradeTimestamps {
		if ts.After(oneHourAgo) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	cb.tradeTimestamps = validTimestamps
}

// getLastTradeTime 获取最后一次交易时间
func (cb *ContextBuilder) getLastTradeTime() time.Time {
	if len(cb.tradeTimestamps) == 0 {
		return time.Time{}
	}
	return cb.tradeTimestamps[len(cb.tradeTimestamps)-1]
}

// RecordTrade 记录一次交易（开仓或平仓）
func (cb *ContextBuilder) RecordTrade(isClose bool, isProfitable bool) {
	now := cb.timeProvider.Now()
	cb.tradeTimestamps = append(cb.tradeTimestamps, now)

	// 有交易发生，重置连续wait计数
	cb.consecutiveWait = 0

	stateChanged := false

	if isClose {
		cb.lastCloseTime = now
		stateChanged = true

		// 更新连续亏损计数
		if isProfitable {
			cb.consecutiveLoss = 0
		} else {
			cb.consecutiveLoss++
		}
	}

	// 清理过期时间戳
	cb.cleanupTradeTimestamps()

	if stateChanged {
		// 保存更新后的状态
		if err := cb.saveState(); err != nil {
			log.Printf("⚠️ 保存交易后状态失败: %v", err)
		}
	}
}

// saveState 保存当前的所有状态
func (cb *ContextBuilder) saveState() error {
	var lastTradeTime int64
	if !cb.lastCloseTime.IsZero() {
		lastTradeTime = cb.lastCloseTime.UnixMilli()
	}

	// 🔒 複製 map 與 slice 以避免併發存取問題
	cb.mu.RLock()
	firstSeenCopy := make(map[string]int64, len(cb.positionFirstSeenTime))
	for k, v := range cb.positionFirstSeenTime {
		firstSeenCopy[k] = v
	}

	historyCopy := make([]decision.FullDecision, len(cb.decisionHistory))
	copy(historyCopy, cb.decisionHistory)

	peakPnLCopy := make(map[string]float64)
	for k, v := range cb.peakPnLCache {
		peakPnLCopy[k] = v
	}
	reasonsCopy := make(map[string]string)
	for k, v := range cb.activeTradeReasons {
		reasonsCopy[k] = v
	}
	callCount := cb.callCount
	peakEquity := cb.peakEquity

	var lastResetUnix int64
	if !cb.lastResetTime.IsZero() {
		lastResetUnix = cb.lastResetTime.Unix()
	}
	cb.mu.RUnlock()

	state := &PositionState{
		FirstSeenTimes:     firstSeenCopy,
		ActiveTradeReasons: reasonsCopy, // 🔧 关键修复（RSK-05）：保存活跃持仓开仓理由，修复杂项覆盖为null的问题
		ConsecutiveLosses:  cb.consecutiveLoss,
		LastTradeTime:      lastTradeTime,
		DailyLoss:          cb.dailyPnL, // ✅ 修复: 保存真实的 DailyLoss
		LastResetTime:      lastResetUnix, // 🔧 修复零值时间负数问题
		CallCount:          callCount,
		PeakEquity:         peakEquity,
		PeakPnLCache:       peakPnLCopy,
		DecisionHistory:    historyCopy,
		StopUntil:          unixStopUntil(cb.stopUntil),
	}
	return cb.persistence.SavePositionState(state)
}

func unixStopUntil(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// GetStopUntil 返回持久化的风控暂停截止时间。
func (cb *ContextBuilder) GetStopUntil() time.Time {
	return cb.stopUntil
}

// GetConsecutiveWait 连续无开仓周期数。
func (cb *ContextBuilder) GetConsecutiveWait() int {
	return cb.consecutiveWait
}

// UpdateStopUntil 写入风控暂停截止并落盘。
func (cb *ContextBuilder) UpdateStopUntil(t time.Time) error {
	cb.stopUntil = t
	return cb.saveState()
}

// GetLastResetTime 获取持久化的上次重置时间
func (cb *ContextBuilder) GetLastResetTime() time.Time {
	return cb.lastResetTime
}

// UpdateLastResetTime 更新持久化的上次重置时间
func (cb *ContextBuilder) UpdateLastResetTime(t time.Time) error {
	cb.lastResetTime = t
	return cb.saveState()
}

// GetDailyPnL 获取持久化的当日盈亏
func (cb *ContextBuilder) GetDailyPnL() float64 {
	return cb.dailyPnL
}

// UpdateDailyPnL 更新当日盈亏并保存
func (cb *ContextBuilder) UpdateDailyPnL(pnl float64) error {
	cb.mu.Lock()
	cb.dailyPnL = pnl
	cb.mu.Unlock()
	return cb.saveState()
}

// AddDailyPnL 累加当日盈亏并保存 (線程安全，避免被動平倉損益被覆蓋)
func (cb *ContextBuilder) AddDailyPnL(pnl float64) float64 {
	cb.mu.Lock()
	cb.dailyPnL += pnl
	current := cb.dailyPnL
	cb.mu.Unlock()
	cb.saveState()
	return current
}

// RecordWait 记录一次wait决策（用于追踪连续wait次数）
func (cb *ContextBuilder) RecordWait() {
	cb.consecutiveWait++
}

// GetCooldownStatus 获取冷却期状态（用于日志和调试）
func (cb *ContextBuilder) GetCooldownStatus() (minutesSinceClose int, tradesInHour int, consecutiveLoss int) {
	if cb.lastCloseTime.IsZero() {
		minutesSinceClose = -1 // 表示没有平仓记录
	} else {
		minutesSinceClose = int(cb.timeProvider.Now().Sub(cb.lastCloseTime).Minutes())
	}
	cb.cleanupTradeTimestamps()
	return minutesSinceClose, len(cb.tradeTimestamps), cb.consecutiveLoss
}

// getCandidateCoins 获取交易员的候选币种列表
func (cb *ContextBuilder) getCandidateCoins() ([]decision.CandidateCoin, error) {
	if len(cb.config.TradingCoins) == 0 {
		// 使用数据库配置的默认币种列表
		var candidateCoins []decision.CandidateCoin

		if len(cb.config.DefaultCoins) > 0 {
			// 使用数据库中配置的默认币种
			for _, coin := range cb.config.DefaultCoins {
				symbol := normalizeSymbol(coin)
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"default"}, // 标记为数据库默认币种
				})
			}
			log.Printf("📋 使用数据库默认币种: %d个币种 %v", len(candidateCoins), cb.config.DefaultCoins)
		} else {
			// 如果数据库中没有配置默认币种，则使用AI500+OI Top作为fallback
			const ai500Limit = 20 // AI500取前20个评分最高的币种

			mergedPool, err := pool.GetMergedCoinPool(ai500Limit)
			if err != nil {
				return nil, fmt.Errorf("获取合并币种池失败: %w", err)
			}

			// 构建候选币种列表（包含来源信息）
			for _, symbol := range mergedPool.AllSymbols {
				sources := mergedPool.SymbolSources[symbol]
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: sources, // "ai500" 和/或 "oi_top"
				})
			}

			log.Printf("📋 数据库无默认币种配置，使用AI500+OI Top: AI500前%d + OI_Top20 = 总计%d个候选币种",
				ai500Limit, len(candidateCoins))
		}

		// 过滤掉交易所不支持的币种
		filteredCoins, removedCount := cb.filterUnsupportedCoins(candidateCoins)
		if removedCount > 0 {
			log.Printf("⚠️ 已过滤 %d 个交易所不支持的币种（如 DASHUSDT 等）", removedCount)
		}
		return filteredCoins, nil
	} else {
		// 使用自定义币种列表
		var candidateCoins []decision.CandidateCoin
		for _, coin := range cb.config.TradingCoins {
			// 确保币种格式正确（转为大写USDT交易对）
			symbol := normalizeSymbol(coin)
			candidateCoins = append(candidateCoins, decision.CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"custom"}, // 标记为自定义来源
			})
		}

		// 过滤掉交易所不支持的币种
		filteredCoins, removedCount := cb.filterUnsupportedCoins(candidateCoins)
		if removedCount > 0 {
			log.Printf("⚠️ 已过滤 %d 个交易所不支持的币种（如 DASHUSDT 等）", removedCount)
		}

		log.Printf("📋 使用自定义币种: %d个币种（过滤后） %v",
			len(filteredCoins), cb.config.TradingCoins)
		return filteredCoins, nil
	}
}

// filterUnsupportedCoins 过滤掉交易所不支持的币种
func (cb *ContextBuilder) filterUnsupportedCoins(candidateCoins []decision.CandidateCoin) ([]decision.CandidateCoin, int) {
	// 对于 Hyperliquid，需要验证币种是否可用
	if cb.config.Exchange == "hyperliquid" {
		// 使用类型断言获取 HyperliquidTrader
		hyperliquidTrader, ok := cb.trader.(*HyperliquidTrader)
		if !ok {
			log.Printf("⚠️ 无法获取 HyperliquidTrader 实例，跳过币种过滤")
			return candidateCoins, 0
		}

		// 一次性获取所有支持的币种价格（更高效）
		allMids, err := hyperliquidTrader.exchange.Info().AllMids(hyperliquidTrader.ctx)
		if err != nil {
			log.Printf("⚠️ 获取 Hyperliquid 支持的币种列表失败，跳过过滤: %v", err)
			return candidateCoins, 0
		}

		// 构建支持的币种集合（Hyperliquid 使用币种名如 "BTC"，我们需要转换为 "BTCUSDT"）
		supportedSymbols := make(map[string]bool)
		for coin := range allMids {
			// Hyperliquid 返回的是币种名（如 "BTC"），转换为 "BTCUSDT"
			symbol := coin + "USDT"
			supportedSymbols[symbol] = true
		}

		// 过滤候选币种
		var supportedCoins []decision.CandidateCoin
		removedCount := 0

		for _, coin := range candidateCoins {
			// 检查币种是否在支持的列表中
			if supportedSymbols[coin.Symbol] {
				supportedCoins = append(supportedCoins, coin)
			} else {
				log.Printf("  ⚠️ 币种 %s 在 Hyperliquid 不支持，已过滤", coin.Symbol)
				removedCount++
			}
		}

		return supportedCoins, removedCount
	}

	// 对于其他交易所（Binance、Aster），暂时不过滤
	// 因为这些交易所通常支持更多币种，且验证成本较高
	return candidateCoins, 0
}

// normalizeSymbol 标准化币种符号（确保以USDT结尾）
func normalizeSymbol(symbol string) string {
	// 转为大写
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// 确保以USDT结尾
	if !strings.HasSuffix(symbol, "USDT") {
		symbol = symbol + "USDT"
	}

	return symbol
}

// RecordPositionTime 记录持仓首次出现时间（用于外部调用，如开仓后）
func (cb *ContextBuilder) RecordPositionTime(symbol, side string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	posKey := symbol + "_" + side
	cb.positionFirstSeenTime[posKey] = cb.timeProvider.Now().UnixMilli()
}

// GetPositionFirstSeenTime 安全取得持倉首次出現時間（併發安全）
func (cb *ContextBuilder) GetPositionFirstSeenTime(posKey string) (int64, bool) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	v, ok := cb.positionFirstSeenTime[posKey]
	return v, ok
}

// SetPositionFirstSeenTime 安全設置持倉首次出現時間（併發安全）
func (cb *ContextBuilder) SetPositionFirstSeenTime(posKey string, timeMs int64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.positionFirstSeenTime[posKey] = timeMs
}

// DeletePositionFirstSeenTime 安全刪除持倉首次出現時間（併發安全）
func (cb *ContextBuilder) DeletePositionFirstSeenTime(posKey string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.positionFirstSeenTime, posKey)
}

// RecordEntryReason 记录开仓理由 (Trade Memory)
func (cb *ContextBuilder) RecordEntryReason(symbol, side, reason string) {
	posKey := symbol + "_" + side
	cb.mu.Lock()
	cb.activeTradeReasons[posKey] = reason
	cb.mu.Unlock()

	if err := cb.saveState(); err != nil {
		log.Printf("⚠️ 保存开仓理由失败: %v", err)
	} else {
		log.Printf("🧠 [Trade Memory] 已记忆开仓理由 (%s): %s", posKey, reason)
	}
}

// RemoveEntryReason 移除开仓理由 (平仓后调用)
func (cb *ContextBuilder) RemoveEntryReason(posKey string) {
	cb.mu.Lock()
	delete(cb.activeTradeReasons, posKey)
	cb.mu.Unlock()

	// 🔧 关键修复（EXE-02）：平仓后同步清理持仓首次出现时间，防止下次再开同币种时被孤儿监控秒平
	cb.DeletePositionFirstSeenTime(posKey)

	if err := cb.saveState(); err != nil {
		log.Printf("⚠️ 更新平仓记忆失败: %v", err)
	} else {
		log.Printf("🧠 [Trade Memory] 已移除开仓理由并清理时间戳 (%s)", posKey)
	}
}

// GetEntryReason 获取开仓理由 (支持大小写标准化)
func (cb *ContextBuilder) GetEntryReason(symbol, side string) string {
	posKey := symbol + "_" + strings.ToLower(side)
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if reason, ok := cb.activeTradeReasons[posKey]; ok && reason != "" {
		return reason
	}
	posKeyUpper := symbol + "_" + strings.ToUpper(side)
	return cb.activeTradeReasons[posKeyUpper]
}

// detectAndLogPassiveClose 侦测并记录被动平仓
func (cb *ContextBuilder) detectAndLogPassiveClose(symbol, key, originalReason string, windowMinutes int) {
	// 調用通用的 GetUserTrades (支持所有交易所)
	trades, err := cb.trader.GetUserTrades(symbol, 50)
	if err != nil {
		log.Printf("⚠️ 查詢成交歷史失敗 (%s): %v", symbol, err)
		return
	}

	lookbackTime := cb.timeProvider.Now().Add(time.Duration(-windowMinutes) * time.Minute).UnixMilli()
	foundPassiveTrade := false

	for _, tr := range trades {
		// 解析時間
		var tradeTime int64
		if tVal, ok := tr["time"].(float64); ok {
			tradeTime = int64(tVal)
		} else {
			continue
		}

		if tradeTime < lookbackTime {
			continue
		}

		// 解析 PnL
		var pnl float64
		if pnlDesc, ok := tr["realizedPnl"]; ok {
			switch v := pnlDesc.(type) {
			case float64:
				pnl = v
			case string:
				pnl, _ = strconv.ParseFloat(v, 64)
			}
		}

		// 如果有實現盈虧，視為平倉事件
		if pnl != 0 {
			closeType := "Stop Loss"
			if pnl > 0 {
				closeType = "Take Profit"
			}

			log.Printf("🕵️ 發現被動平倉 (%s): %s | PnL: %.4f | Original Reason: %s",
				closeType, symbol, pnl, originalReason)

			// 构造决策理由
			reasonMsg := fmt.Sprintf("Passive Close: %s Triggered (PnL: %.2f) | Original Reason: %s",
				closeType, pnl, originalReason)

			// 解析成交量和价格
			var tradePrice, tradeQty float64
			if p, ok := tr["price"].(float64); ok {
				tradePrice = p
			} else if pStr, ok := tr["price"].(string); ok {
				tradePrice, _ = strconv.ParseFloat(pStr, 64)
			}

			if q, ok := tr["qty"].(float64); ok {
				tradeQty = q
			} else if qStr, ok := tr["qty"].(string); ok {
				tradeQty, _ = strconv.ParseFloat(qStr, 64)
			}

			// 构造 DecisionRecord
			actionRec := logger.DecisionAction{
				Timestamp: time.UnixMilli(tradeTime),
				Symbol:    symbol,
				Action:    "passive_close",
				Price:     tradePrice, // ✅ 补充价格
				Quantity:  tradeQty,   // ✅ 补充数量
				Reasoning: reasonMsg,
				Success:   true,
			}

			record := logger.DecisionRecord{
				Timestamp:   time.UnixMilli(tradeTime),
				Decisions:   []logger.DecisionAction{actionRec},
				CycleNumber: -1, // 表示非正常周期
				Success:     true,
			}

			if err := cb.decisionLogger.LogDecision(&record); err != nil {
				log.Printf("⚠️ 记录被动平仓日志失败: %v", err)
			}

			// 更新連續虧損狀態
			if pnl < 0 {
				cb.consecutiveLoss++
			} else {
				cb.consecutiveLoss = 0
			}

			// 🔧 关键修复（RSK-01）：被动平仓（止损/止盈/强平）同步累计至 dailyPnL
			cb.dailyPnL += pnl

			// 🔧 关键修复（EXE-02）：被动平仓成功后清理开仓理由与孤儿时间戳
			cb.RemoveEntryReason(key)
			cb.DeletePositionFirstSeenTime(key)
			cb.saveState()

			cb.lastCloseTime = cb.timeProvider.Now()
			foundPassiveTrade = true
			break // 只處理最近的一筆
		}
	}

	if !foundPassiveTrade {
		log.Printf("ℹ️ 持倉 %s 消失，但未檢測到近期成交 (可能過期或手動平倉)", key)
		cb.RemoveEntryReason(key)
		cb.DeletePositionFirstSeenTime(key)
		cb.saveState()
	}
}

// RecoverTradeHistory 从历史决策记录中恢复交易历史状态
// 在系统重启后调用，用于恢复连续亏损次数和近期交易时间戳
// 如果恢复失败，会记录警告但不会阻止程序启动
func (cb *ContextBuilder) RecoverTradeHistory() {
	if cb.decisionLogger == nil {
		log.Printf("⚠️ DecisionLogger 未配置，跳过交易历史恢复")
		return
	}

	// 获取最近的决策记录（足够覆盖连续亏损检测需求）
	records, err := cb.decisionLogger.GetLatestRecords(100)
	if err != nil {
		log.Printf("⚠️ 读取历史决策记录失败: %v（将使用默认值）", err)
		return
	}

	if len(records) == 0 {
		log.Printf("ℹ️ 没有历史决策记录，跳过恢复")
		return
	}

	now := cb.timeProvider.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	// 用于计算连续亏损的交易盈亏列表（从新到旧）
	var tradePnLs []float64 // 正数=盈利, 负数=亏损

	// 追踪开仓信息：posKey -> openPrice
	openPositions := make(map[string]float64)

	// 从旧到新遍历记录（records 已经是时间正序）
	for _, record := range records {
		for _, action := range record.Decisions {
			// 跳过失败的动作
			if !action.Success {
				continue
			}

			symbol := action.Symbol
			side := ""

			// 确定交易方向
			switch action.Action {
			case "open_long", "close_long", "auto_close_long":
				side = "long"
			case "open_short", "close_short", "auto_close_short":
				side = "short"
			case "partial_close":
				// partial_close 需要从现有持仓确定方向
				for key := range openPositions {
					if strings.HasPrefix(key, symbol+"_") {
						side = strings.TrimPrefix(key, symbol+"_")
						break
					}
				}
			default:
				continue // 忽略 hold, wait, update_* 等动作
			}

			if side == "" {
				continue
			}

			posKey := symbol + "_" + side

			switch action.Action {
			case "open_long", "open_short":
				// 记录开仓价格
				openPositions[posKey] = action.Price

			case "close_long", "close_short", "auto_close_long", "auto_close_short", "partial_close":
				// 计算盈亏
				openPrice, exists := openPositions[posKey]
				if !exists || openPrice == 0 {
					continue // 找不到开仓记录，跳过
				}

				closePrice := action.Price
				qty := action.Quantity
				if qty <= 0 {
					qty = 1.0 // 兼容未记录数量的历史数据回退
				}
				var pnl float64
				if side == "long" {
					pnl = (closePrice - openPrice) * qty // 多仓：(平仓价 - 开仓价) * 数量
				} else {
					pnl = (openPrice - closePrice) * qty // 空仓：(开仓价 - 平仓价) * 数量
				}

				// 只在完全平仓时记录（partial_close 不计入连续亏损）
				if action.Action != "partial_close" {
					tradePnLs = append(tradePnLs, pnl)
					delete(openPositions, posKey)
				}

				// 恢复1小时内的交易时间戳
				if action.Timestamp.After(oneHourAgo) {
					cb.tradeTimestamps = append(cb.tradeTimestamps, action.Timestamp)
				}

				// 恢复最后平仓时间
				if action.Timestamp.After(cb.lastCloseTime) {
					cb.lastCloseTime = action.Timestamp
				}
			}
		}
	}

	// 计算连续亏损次数（从最新交易往回数）
	cb.consecutiveLoss = 0
	for i := len(tradePnLs) - 1; i >= 0; i-- {
		if tradePnLs[i] < 0 {
			cb.consecutiveLoss++
		} else {
			break // 遇到盈利交易，停止计数
		}
	}

	log.Printf("📋 恢复历史: 读取到 %d 条决策记录", len(records))

	// 计算连续wait次数（从最新记录往回数）
	// 规则：从最新记录开始，如果决策列表为空或只有wait动作（无hold/open/close），则计为wait
	cb.consecutiveWait = 0
	for i := len(records) - 1; i >= 0; i-- {
		record := records[i]

		// 检查该周期是否有活跃动作
		hasActiveAction := false
		hasHoldAction := false
		hasWaitAction := false

		decisionCount := len(record.Decisions)
		if decisionCount > 0 {
			for _, action := range record.Decisions {
				switch action.Action {
				case "wait":
					hasWaitAction = true
				case "hold":
					hasHoldAction = true
				case "open_long", "open_short", "close_long", "close_short",
					"update_stop_loss", "update_take_profit", "partial_close":
					hasActiveAction = true
				}
			}
		}

		// 只有纯wait周期才计入连续wait
		if (hasWaitAction && !hasActiveAction && !hasHoldAction) || decisionCount == 0 {
			cb.consecutiveWait++
		} else {
			// 遇到非wait周期，停止计数
			// Debug日志：由哪条记录中断
			if i == len(records)-1 {
				log.Printf("⚠️ 最新记录(Cycle %d)非 Wait 周期，中断计数。hasWait=%v, Active=%v, Hold=%v, DecCount=%d",
					record.CycleNumber, hasWaitAction, hasActiveAction, hasHoldAction, decisionCount)
				if decisionCount > 0 {
					log.Printf("   -> 动作: %s", record.Decisions[0].Action)
				}
			}
			break
		}
	}

	log.Printf("✓ 已恢复交易历史: 连续亏损=%d笔, 1小时内交易=%d笔, 连续wait=%d周期, 上次平仓=%s",
		cb.consecutiveLoss,
		len(cb.tradeTimestamps),
		cb.consecutiveWait,
		formatTimeAgo(cb.lastCloseTime))
}

// formatTimeAgo 格式化时间差为可读字符串
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "无记录"
	}
	duration := time.Since(t)
	if duration < time.Minute {
		return "刚刚"
	} else if duration < time.Hour {
		return fmt.Sprintf("%d分钟前", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%d小时前", int(duration.Hours()))
	}
	return fmt.Sprintf("%d天前", int(duration.Hours()/24))
}

// GetCallCount 获取持久化的AI决策周期
func (cb *ContextBuilder) GetCallCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.callCount
}

// UpdateCallCount 更新周期数并保存
func (cb *ContextBuilder) UpdateCallCount(count int) error {
	cb.mu.Lock()
	cb.callCount = count
	cb.mu.Unlock()
	return cb.saveState()
}

// GetDecisionHistory 获取持久化的AI决策历史记录
func (cb *ContextBuilder) GetDecisionHistory() []decision.FullDecision {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	// 返回深拷貝副本
	historyCopy := make([]decision.FullDecision, len(cb.decisionHistory))
	copy(historyCopy, cb.decisionHistory)
	return historyCopy
}

// UpdateDecisionHistory 更新决策历史记录并保存
func (cb *ContextBuilder) UpdateDecisionHistory(history []decision.FullDecision) error {
	cb.mu.Lock()
	
	// 深拷貝保存
	historyCopy := make([]decision.FullDecision, len(history))
	copy(historyCopy, history)
	cb.decisionHistory = historyCopy
	
	cb.mu.Unlock()
	return cb.saveState()
}

// GetPeakPnLCache 获取持久化的盈亏峰值快取
func (cb *ContextBuilder) GetPeakPnLCache() map[string]float64 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	// 返回拷貝副本
	cacheCopy := make(map[string]float64)
	for k, v := range cb.peakPnLCache {
		cacheCopy[k] = v
	}
	return cacheCopy
}

// UpdatePeakPnLCache 更新盈亏峰值快取并保存
func (cb *ContextBuilder) UpdatePeakPnLCache(cache map[string]float64) error {
	cb.mu.Lock()
	
	// 拷貝保存
	cacheCopy := make(map[string]float64)
	for k, v := range cache {
		cacheCopy[k] = v
	}
	cb.peakPnLCache = cacheCopy
	
	cb.mu.Unlock()
	return cb.saveState()
}

// DeletePeakPnL 删除指定币种的盈亏峰值快取
func (cb *ContextBuilder) DeletePeakPnL(symbol string) {
	cb.mu.Lock()
	delete(cb.peakPnLCache, symbol)
	cb.mu.Unlock()
}

// GetPeakEquity 获取历史最高净值
func (cb *ContextBuilder) GetPeakEquity() float64 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.peakEquity
}
