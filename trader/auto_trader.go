package trader

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"aetheris/config"
	"aetheris/decision"
	"aetheris/logger"
	"aetheris/market"
	"aetheris/mcp"
	"aetheris/pool"
	"aetheris/utils"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// AutoTraderConfig 自动交易配置（简化版 - AI全权决策）
type AutoTraderConfig struct {
	// Trader标识
	ID      string // Trader唯一标识（用于日志目录等）
	Name    string // Trader显示名称
	AIModel string // AI模型: "qwen" 或 "deepseek"

	// 交易平台选择
	Exchange string // "binance", "hyperliquid" 或 "aster"

	// 币安API配置
	BinanceAPIKey    string
	BinanceSecretKey string

	// Hyperliquid配置
	HyperliquidPrivateKey string
	HyperliquidWalletAddr string
	HyperliquidTestnet    bool

	// Aster配置
	AsterUser       string // Aster主钱包地址
	AsterSigner     string // Aster API钱包地址
	AsterPrivateKey string // Aster API钱包私钥
	AsterTestnet    bool   // Aster测试网模式

	CoinPoolAPIURL string

	// AI配置
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string

	// 自定义AI API配置
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// 扫描配置
	ScanInterval time.Duration // 扫描间隔（建议3分钟）

	// 账户配置
	InitialBalance float64 // 初始金额（用于计算盈亏，需手动设置）

	// 杠杆配置
	BTCETHLeverage  int // BTC和ETH的杠杆倍数
	AltcoinLeverage int // 山寨币的杠杆倍数

	// 风险控制（Go 硬限制：触发后暂停新开仓，AI 无法覆盖）
	MaxDailyLoss    float64       // 最大日亏损百分比
	MaxDrawdown     float64       // 最大回撤百分比
	StopTradingTime time.Duration // 回撤触发后暂停时长；日亏损暂停到次日 00:00

	// 仓位模式
	IsCrossMargin bool // true=全仓模式, false=逐仓模式

	// 币种配置
	DefaultCoins []string // 默认币种列表（从数据库获取）
	TradingCoins []string // 实际交易币种列表

	// 系统提示词模板
	SystemPromptTemplate string // 系统提示词模板名称（如 "default", "aggressive"）

	// 提示词规则配置
	PromptRules *config.PromptRules
}

// AutoTrader 自动交易器
type AutoTrader struct {
	id                   string // Trader唯一标识
	name                 string // Trader显示名称
	aiModel              string // AI模型名称
	exchange             string // 交易平台名称
	config               AutoTraderConfig
	trader               Trader // 使用Trader接口（支持多平台）
	mcpClient            *mcp.Client
	decisionLogger       *logger.DecisionLogger // 决策日志记录器
	initialBalance       float64
	dailyPnL             float64
	customPrompt         string   // 自定义交易策略prompt
	overrideBasePrompt   bool     // 是否覆盖基础prompt
	systemPromptTemplate string   // 系统提示词模板名称
	defaultCoins         []string // 默认币种列表（从数据库获取）
	tradingCoins         []string // 实际交易币种列表
	lastResetTime        time.Time
	stopUntil            time.Time
	isRunning            bool
	startTime            time.Time // 系统启动时间
	callCount            int       // AI调用次数
	contextBuilder       *ContextBuilder
	executor             *DecisionExecutor
	dataProvider         market.DataProvider     // 市场数据提供者
	timeProvider         utils.TimeProvider      // 时间提供者
	stopMonitorCh        chan struct{}           // 用于停止监控goroutine
	monitorWg            sync.WaitGroup          // 用于等待监控goroutine结束
	decisionHistory      []decision.FullDecision // AI决策历史（最近5次）
	liquidityEngine      *market.LiquidityEngine // 🔥 持续化流动性引擎
	peakPnLCache         map[string]float64      // 最高收益缓存 (symbol -> 峰值盈亏百分比)
	peakPnLCacheMutex    sync.RWMutex            // 缓存读写锁
	lastBalanceSyncTime  time.Time               // 上次余额同步时间
	database             interface{}             // 数据库引用（用于自动更新余额）
	userID               string                  // 用户ID
	stopOnce             sync.Once               // 🔒 保護 stopMonitorCh 不會被重複關閉
	lastPlaybooks        []string                // 最近一轮注入的 skills
}

// NewAutoTrader 创建自动交易器
func NewAutoTrader(config AutoTraderConfig, database interface{}, userID string, dataProvider market.DataProvider, timeProvider utils.TimeProvider, sleepFunc func(time.Duration)) (*AutoTrader, error) {
	// 如果未提供 timeProvider，使用默认的 RealTimeProvider
	if timeProvider == nil {
		timeProvider = &utils.RealTimeProvider{}
	}
	// 设置默认值
	if config.ID == "" {
		config.ID = "default_trader"
	}
	if config.Name == "" {
		config.Name = "Default Trader"
	}
	if config.AIModel == "" {
		if config.UseQwen {
			config.AIModel = "qwen"
		} else {
			config.AIModel = "deepseek"
		}
	}

	mcpClient := mcp.New()

	// 初始化AI
	if config.AIModel == "custom" {
		// 使用自定义API
		mcpClient.SetCustomAPI(config.CustomAPIURL, config.CustomAPIKey, config.CustomModelName)
		log.Printf("🤖 [%s] 使用自定义AI API: %s (模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
	} else if config.UseQwen || config.AIModel == "qwen" {
		// 使用Qwen (支持自定义URL和Model)
		mcpClient.SetQwenAPIKey(config.QwenKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] 使用阿里云Qwen AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] 使用阿里云Qwen AI", config.Name)
		}
	} else if config.AIModel == "mock" {
		// Mock模式 for Backtest Verification
		mcpClient.SetMock()
		log.Printf("🤖 [%s] 使用 Mock AI (Backtest Verification)", config.Name)
	} else {
		// 默认使用DeepSeek (支持自定义URL和Model)
		mcpClient.SetDeepSeekAPIKey(config.DeepSeekKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] 使用DeepSeek AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] 使用DeepSeek AI", config.Name)
		}
	}

	// 初始化币种池API
	if config.CoinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(config.CoinPoolAPIURL)
	}

	// 设置默认交易平台
	if config.Exchange == "" {
		config.Exchange = "binance"
	}

	// 根据配置创建对应的交易器
	var trader Trader
	var err error

	// 记录仓位模式（通用）
	marginModeStr := "全仓"
	if !config.IsCrossMargin {
		marginModeStr = "逐仓"
	}
	log.Printf("📊 [%s] 仓位模式: %s", config.Name, marginModeStr)

	switch config.Exchange {
	case "binance":
		log.Printf("🏦 [%s] 使用币安合约交易", config.Name)
		trader = NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey)
	case "hyperliquid":
		log.Printf("🏦 [%s] 使用Hyperliquid交易", config.Name)
		trader, err = NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
		if err != nil {
			return nil, fmt.Errorf("初始化Hyperliquid交易器失败: %w", err)
		}
	case "aster":
		log.Printf("🏦 [%s] 使用Aster交易 (Testnet: %v)", config.Name, config.AsterTestnet)
		// Trim whitespace to ensure empty string triggers auto-derivation
		signerToCheck := strings.TrimSpace(config.AsterSigner)

		// Validate address format (must be 42 chars starting with 0x)
		if len(signerToCheck) > 0 {
			if len(signerToCheck) != 42 || !strings.HasPrefix(signerToCheck, "0x") {
				log.Printf("⚠️ 警告: 提供的 Signer 地址格式不正确 (长度=%d, 期望=42, 需以0x开头). 值='%s'. 将忽略该值并尝试自动推导.",
					len(signerToCheck), signerToCheck)
				signerToCheck = ""
			}
		}

		maskedKey := "EMPTY"
		if len(config.AsterPrivateKey) > 4 {
			maskedKey = "..." + config.AsterPrivateKey[len(config.AsterPrivateKey)-4:]
		}

		log.Printf("🔍 Debug Aster Init - User: %s, Signer: '%s' (len=%d), PrivateKey: %s",
			config.AsterUser, config.AsterSigner, len(config.AsterSigner), maskedKey)

		trader, err = NewAsterTrader(config.AsterUser, signerToCheck, config.AsterPrivateKey, config.AsterTestnet)
		if err != nil {
			return nil, fmt.Errorf("初始化Aster交易器失败: %w", err)
		}
	case "backtest":
		// Backtest mode: Trader will be injected via SetTrader
		log.Println("⚠️ Using Backtest Exchange Mode (Waiting for MockTrader injection)")
		trader = nil // The actual trader will be set later via SetTrader
	default:
		return nil, fmt.Errorf("不支持的交易平台: %s", config.Exchange)
	}

	// 验证初始金额配置
	if config.InitialBalance <= 0 {
		return nil, fmt.Errorf("初始金额必须大于0，请在配置中设置InitialBalance")
	}

	// 初始化决策日志记录器（使用trader ID创建独立目录）
	logDir := fmt.Sprintf("decision_logs/%s", config.ID)
	decisionLogger := logger.NewDecisionLogger(logDir)

	// 停机超过两周再启动时，14 天清理会把研究日志整批删掉。保留 180 天。
	if err := decisionLogger.CleanOldRecords(180); err != nil {
		log.Printf("⚠️ 清理旧日志失败: %v", err)
	}

	// 🔥 初始化流动性引擎，引入 config.ID 隔离热力图路径，防止文件读写冲突
	heatmapPath := "data/liquidity_heatmap.json"
	if config.ID != "" && config.ID != "default_trader" {
		heatmapPath = fmt.Sprintf("data/liquidity_heatmap_%s.json", config.ID)
	}
	liquidityEngine := market.NewLiquidityEngine(heatmapPath, dataProvider)

	// 初始化ContextBuilder
	startTime := timeProvider.Now()
	contextBuilder := NewContextBuilder(trader, decisionLogger, config, startTime, dataProvider, timeProvider, liquidityEngine)

	// 从历史记录恢复交易状态（连续亏损、近期交易次数等）
	contextBuilder.RecoverTradeHistory()
	// 启动时对账：检查离线期间的平仓
	contextBuilder.ReconcileOfflineCloses()

	// 初始化Executor
	executor := NewDecisionExecutor(trader, config, contextBuilder, dataProvider, sleepFunc)

	// 从 ContextBuilder 加载上次重置时间
	lastResetTime := contextBuilder.GetLastResetTime()
	if lastResetTime.IsZero() {
		// 如果是全新的状态，设置为现在
		lastResetTime = startTime
	}

	at := &AutoTrader{
		id:                   config.ID,
		name:                 config.Name,
		aiModel:              config.AIModel,
		exchange:             config.Exchange,
		config:               config,
		trader:               trader,
		mcpClient:            mcpClient,
		decisionLogger:       decisionLogger,
		initialBalance:       config.InitialBalance,
		dailyPnL:             contextBuilder.GetDailyPnL(), // ✅ 从持久化状态加载
		customPrompt:         "",
		overrideBasePrompt:   false,
		systemPromptTemplate: config.SystemPromptTemplate,
		defaultCoins:         config.DefaultCoins,
		tradingCoins:         config.TradingCoins,
		isRunning:            false,
		startTime:            startTime,
		callCount:            contextBuilder.GetCallCount(),
		contextBuilder:       contextBuilder,
		executor:             executor,
		dataProvider:         dataProvider,
		timeProvider:         timeProvider,
		stopMonitorCh:        make(chan struct{}),
		decisionHistory:      contextBuilder.GetDecisionHistory(),
		liquidityEngine:      liquidityEngine, // 🔧 修復: 使用已初始化的引擎，避免雙重初始化造成記憶體洩漏
		peakPnLCache:         contextBuilder.GetPeakPnLCache(),
		lastBalanceSyncTime:  timeProvider.Now(),
		lastResetTime:        lastResetTime, // ✅ 设置持久化的上次重置时间
		stopUntil:            contextBuilder.GetStopUntil(),
		database:             database,
		userID:               userID,
	}

	// 设置默认系统提示词模板
	if at.systemPromptTemplate == "" {
		// feature/partial-close-dynamic-tpsl 分支默认使用 adaptive（支持动态止盈止损）
		at.systemPromptTemplate = "adaptive"
	}

	return at, nil
}

// Run 运行自动交易主循环
func (at *AutoTrader) Run() (errRet error) {
	// 🚀 防禦重複啟動，阻止協程洩漏
	if at.isRunning {
		return fmt.Errorf("交易员 %s (ID: %s) 已经在运行中，请勿重复启动", at.name, at.id)
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ [%s] AutoTrader.Run 捕获严重 panic: %v\n堆栈:\n%s", at.name, r, debug.Stack())
			at.isRunning = false
			errRet = fmt.Errorf("panic in AutoTrader.Run: %v", r)
		}
	}()

	// 重置停止信號，允許重複啟動與停止
	at.stopMonitorCh = make(chan struct{})
	at.stopOnce = sync.Once{}
	
	at.isRunning = true
	log.Println("🚀 AI驱动自动交易系统启动")
	log.Printf("💰 初始余额: %.2f USDT (从配置加载: %.2f USDT)", at.initialBalance, at.config.InitialBalance)
	if math.Abs(at.initialBalance-at.config.InitialBalance) > 0.01 {
		log.Printf("⚠️ 警告：内存中的初始余额 (%.2f) 与配置中的初始余额 (%.2f) 不一致！", at.initialBalance, at.config.InitialBalance)
	}
	log.Printf("⚙️  扫描间隔: %v", at.config.ScanInterval)
	log.Println("🤖 AI将全权决定杠杆、仓位大小、止损止盈等参数")

	// 启动回撤监控
	at.startDrawdownMonitor()

	// 启动幽灵订单监控 (Ghostbusters)
	at.startGhostbusterMonitor()

	ticker := time.NewTicker(at.config.ScanInterval)
	defer ticker.Stop()

	// 首次立即执行
	if at.isRunning {
		if err := at.RunCycle(); err != nil {
			log.Printf("❌ 执行失败: %v", err)
		}
	}

	// 使用 select 同时监听 ticker 和 stop 信号，确保能立即响应停止请求
	for at.isRunning {
		select {
		case <-at.stopMonitorCh:
			// 收到停止信号，立即退出循环
			log.Printf("⏹ [%s] 收到停止信号，退出主循环", at.name)
			return nil
		case <-ticker.C:
			// 定时器触发，执行交易周期
			if !at.isRunning {
				return nil
			}
			if err := at.RunCycle(); err != nil {
				log.Printf("❌ 执行失败: %v", err)
			}
		}
	}

	return nil
}

// Stop 停止自动交易
func (at *AutoTrader) Stop() {
	at.stopOnce.Do(func() {
		at.isRunning = false
		close(at.stopMonitorCh) // 通知监控goroutine停止
	})
	at.monitorWg.Wait()     // 等待监控goroutine结束
	log.Println("⏹ 自动交易系统停止")
}

// SetRunning 设置运行状态 (用于回测控制)
func (at *AutoTrader) SetRunning(running bool) {
	at.isRunning = running
}

// SetTrader sets the underlying trader interface (used for backtesting)
func (at *AutoTrader) SetTrader(t Trader) {
	at.trader = t
	if at.contextBuilder != nil {
		at.contextBuilder.SetTrader(t)
	}
	if at.executor != nil {
		at.executor.SetTrader(t)
	}
}

// RunCycle 执行一次完整的交易周期 (公开用于回测)
func (at *AutoTrader) RunCycle() (errRet error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ [%s] RunCycle 捕获严重 panic: %v\n堆栈:\n%s", at.name, r, debug.Stack())
			errRet = fmt.Errorf("panic in RunCycle: %v", r)
		}
	}()

	cycleStartTime := at.timeProvider.Now()
	at.callCount++
	if err := at.contextBuilder.UpdateCallCount(at.callCount); err != nil {
		log.Printf("⚠️ 保存决策周期数失败: %v", err)
	}

	log.Print("\n" + strings.Repeat("=", 70) + "\n")
	log.Printf("⏰ %s - AI决策周期 #%d", at.timeProvider.Now().Format("2006-01-02 15:04:05"), at.callCount)
	log.Println(strings.Repeat("=", 70))

	// ✅ 检查1: 周期开始时检查是否已停止
	if !at.isRunning {
		log.Printf("⏹ [%s] 交易员已停止，跳过本周期", at.name)
		return nil
	}

	// 创建决策记录
	record := &logger.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
	}

	// 2. 重置日盈亏（跨自然日重置，修復本機重啟與滾動24小時偏差）
	now := at.timeProvider.Now()
	if now.Year() != at.lastResetTime.Year() || now.YearDay() != at.lastResetTime.YearDay() {
		at.dailyPnL = 0
		at.contextBuilder.UpdateDailyPnL(0) // ✅ 同步到持久化
		at.lastResetTime = now
		at.contextBuilder.UpdateLastResetTime(now) // ✅ 更新持久化的 ResetTime
		log.Println("📅 跨越自然日，日盈亏已成功重置")
	}

	// 3. 收集交易上下文
	ctx, err := at.contextBuilder.Build(at.callCount, at.decisionHistory)
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("构建交易上下文失败: %v", err)
		at.decisionLogger.LogDecision(record)
		log.Printf("✅ [%s] 周期执行完成 (耗时: %v)", at.name, time.Since(cycleStartTime))
		return fmt.Errorf("构建交易上下文失败: %w", err)
	}

	// 🔄 同步最新 dailyPnL（將被動平倉在 Build 期間偵測到的損益即時同步給 AutoTrader）
	at.dailyPnL = at.contextBuilder.GetDailyPnL()

	// 保存账户状态快照
	record.AccountState = logger.AccountSnapshot{
		TotalBalance:          ctx.Account.TotalEquity,
		AvailableBalance:      ctx.Account.AvailableBalance,
		TotalUnrealizedProfit: ctx.Account.UnrealizedPnL,
		PositionCount:         ctx.Account.PositionCount,
		MarginUsedPct:         ctx.Account.MarginUsedPct,
	}

	blockOpens := at.enforceRiskHalt(ctx, record)
	if at.applyOperatorDirective(ctx, record) {
		blockOpens = true
	}
	if blockOpens && ctx.Account.PositionCount == 0 {
		if record != nil && record.ErrorMessage == "" {
			record.Success = true
			record.ErrorMessage = "人工干预暂停开仓 (无持仓需管理)"
			at.decisionLogger.LogDecision(record)
		}
		log.Printf("✅ [%s] 周期执行完成 (耗时: %v)", at.name, time.Since(cycleStartTime))
		return nil
	}

	// 保存持仓快照
	for _, pos := range ctx.Positions {
		record.Positions = append(record.Positions, logger.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        pos.MarkPrice,
			UnrealizedProfit: pos.UnrealizedPnL,
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
			StopLoss:         pos.StopLoss,
			TakeProfit:       pos.TakeProfit,
		})
	}

	log.Print(strings.Repeat("=", 70))
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	log.Printf("📊 账户净值: %.2f USDT | 可用: %.2f USDT | 持仓: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// ✅ 检查2: AI决策前检查是否已停止
	if !at.isRunning {
		log.Printf("⏹ [%s] 交易员已停止，跳过AI决策", at.name)
		record.Success = false
		record.ErrorMessage = "交易员已停止"
		at.decisionLogger.LogDecision(record)
		return nil
	}

	// 5. 调用AI获取完整决策
	log.Printf("🤖 正在请求AI分析并决策... [模板: %s]", at.systemPromptTemplate)

	decisionResult, decisionErr := decision.GetFullDecisionWithCustomPrompt(ctx, at.mcpClient, at.customPrompt, at.overrideBasePrompt, at.systemPromptTemplate)
	if decisionResult != nil && len(decisionResult.Playbooks) > 0 {
		at.lastPlaybooks = decisionResult.Playbooks
	}

	// 即使有错误，也保存思维链、决策和输入prompt（用于debug）
	if decisionResult != nil {
		record.SystemPrompt = decisionResult.SystemPrompt
		// 保存系统提示词
		record.InputPrompt = decisionResult.UserPrompt
		record.CoTTrace = decisionResult.CoTTrace
		if len(decisionResult.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(decisionResult.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	if blockOpens && decisionResult != nil {
		filtered := make([]decision.Decision, 0, len(decisionResult.Decisions))
		for _, d := range decisionResult.Decisions {
			if d.Action == "open_long" || d.Action == "open_short" {
				log.Printf("⏸ 风控暂停中，丢弃开仓 %s %s", d.Symbol, d.Action)
				continue
			}
			filtered = append(filtered, d)
		}
		decisionResult.Decisions = filtered
		log.Printf("⏸ 风控暂停中：仅允许平仓/持仓管理，本周期禁止新开仓")
	}

	if decisionErr != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("获取AI决策失败: %v", decisionErr)

		// 打印系统提示词和AI思维链（即使有错误，也要输出以便调试）
		if decisionResult != nil {
			log.Print("\n" + strings.Repeat("=", 70) + "\n")
			log.Printf("📋 系统提示词 [模板: %s] (错误情况)", at.systemPromptTemplate)
			log.Println(strings.Repeat("=", 70))
			log.Println(decisionResult.SystemPrompt)
			log.Println(strings.Repeat("=", 70))

			if decisionResult.CoTTrace != "" {
				log.Print("\n" + strings.Repeat("-", 70) + "\n")
				log.Println("💭 AI思维链分析（错误情况）:")
				log.Println(strings.Repeat("-", 70))
				log.Println(decisionResult.CoTTrace)
				log.Println(strings.Repeat("-", 70))
			}
		}

		// 如果有部分決策導致的錯誤，也需要記錄到歷史
		if decisionResult != nil {
			at.addToHistory(*decisionResult)
		}

		at.decisionLogger.LogDecision(record)
		log.Printf("✅ [%s] 周期执行完成 (耗时: %v)", at.name, time.Since(cycleStartTime))
		return fmt.Errorf("获取AI决策失败: %w", decisionErr)
	}

	// // 5. 打印系统提示词
	// log.Printf("\n" + strings.Repeat("=", 70))
	// log.Printf("📋 系统提示词 [模板: %s]", at.systemPromptTemplate)
	// log.Println(strings.Repeat("=", 70))
	// log.Println(decisionResult.SystemPrompt)
	// log.Printf(strings.Repeat("=", 70) + "\n")

	// 6. 打印AI思维链
	// log.Printf("\n" + strings.Repeat("-", 70))
	// log.Println("💭 AI思维链分析:")
	// log.Println(strings.Repeat("-", 70))
	// log.Println(decisionResult.CoTTrace)
	// log.Printf(strings.Repeat("-", 70) + "\n")

	// ✅ 延迟填充技术分析和价格行为数据（确保在AI获取数据后）
	record.TechnicalAnalysis = make(map[string]*market.TechnicalAnalysis)
	record.PriceAction = make(map[string]*market.PriceActionData)

	for symbol, data := range ctx.MarketDataMap {
		if data != nil && data.TechnicalAnalysis != nil {
			if data.HourlyContext != nil {
				data.TechnicalAnalysis.VolumeZScore = data.HourlyContext.VolumeZScore
			}
			record.TechnicalAnalysis[symbol] = data.TechnicalAnalysis
		}
		if data != nil && data.PriceAction != nil {
			record.PriceAction[symbol] = data.PriceAction
		}
	}

	// 7. 打印AI决策
	// log.Printf("📋 AI决策列表 (%d 个):\n", len(decisionResult.Decisions))
	// for i, d := range decisionResult.Decisions {
	//     log.Printf("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
	//     if d.Action == "open_long" || d.Action == "open_short" {
	//        log.Printf("      杠杆: %dx | 仓位: %.2f USDT | 止损: %.4f | 止盈: %.4f",
	//           d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
	//     }
	// }
	log.Println()
	log.Print(strings.Repeat("-", 70))
	// 8. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	log.Print(strings.Repeat("-", 70))

	// 4. 对决策排序：先平仓，再开仓
	decisions := decisionResult.Decisions
	decisions = SortDecisionsByPriority(decisions)

	// 5. 过滤冲突决策
	currentPositions, _ := at.trader.GetPositions()
	decisions = FilterConflictingDecisions(decisions, currentPositions)

	// 9. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	sortedDecisions := decisions

	log.Println("🔄 执行顺序（已优化）: 先平仓→后开仓")
	for i, d := range sortedDecisions {
		log.Printf("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	log.Println()

	// ✅ 检查3: 执行交易前检查是否已停止
	if !at.isRunning {
		log.Printf("⏹ [%s] 交易员已停止，跳过交易执行", at.name)
		record.Success = false
		record.ErrorMessage = "交易员已停止，未执行交易"
		at.decisionLogger.LogDecision(record)
		return nil
	}

	// 执行决策并记录结果
	for _, d := range sortedDecisions {
		actionRecord := logger.DecisionAction{
			Action:     d.Action,
			Symbol:     d.Symbol,
			Quantity:   0,
			Leverage:   d.Leverage,
			Price:      0,
			StopLoss:   d.StopLoss,                       // ✅ 保存止损价格
			TakeProfit: d.TakeProfit,                     // ✅ 保存止盈价格
			Reasoning:  truncateString(d.Reasoning, 500), // ✅ 保存完整理由（最多500字符）
			Timestamp:  at.timeProvider.Now(),
			Success:    false,
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			log.Printf("❌ 执行决策失败 (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 失败: %v", d.Symbol, d.Action, err))

			// 🔥 关键修复：将错误回写到 Decision History，让AI在其“记忆”中看到这次失败
			if decisionResult != nil {
				for i := range decisionResult.Decisions {
					// 找到对应的决策 (使用同一个指针或者匹配特征)
					if decisionResult.Decisions[i].Symbol == d.Symbol && decisionResult.Decisions[i].Action == d.Action {
						decisionResult.Decisions[i].ExecutionError = err.Error()
						break
					}
				}
			}
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 成功", d.Symbol, d.Action))

			// ✅ 成功執行後，更新 PnL 和 交易計數
			// ✅ 成功執行後，更新 PnL 和 交易計數
			switch d.Action {
			case "open_long", "open_short":
				at.contextBuilder.RecordTrade(false, false)
			case "close_long", "close_short", "partial_close":
				// 短暫延遲等待成交數據更新
				time.Sleep(2 * time.Second)

				// 嘗試獲取該筆交易的盈虧
				trades, err := at.trader.GetUserTrades(d.Symbol, 1)
				if err == nil && len(trades) > 0 {
					// 簡單取最後一筆 (需要更精確的匹配但在此足夠捕捉大部分情況)
					lastTrade := trades[0]

					var realizedPnl float64
					if pnlVal, ok := lastTrade["realizedPnl"].(float64); ok {
						realizedPnl = pnlVal
					} else if pnlStr, ok := lastTrade["realizedPnl"].(string); ok {
						// 嘗試解析 string
						fmt.Sscanf(pnlStr, "%f", &realizedPnl)
					}

					at.dailyPnL = at.contextBuilder.AddDailyPnL(realizedPnl)
					log.Printf("💰 更新 Daily PnL: %.4f (Total: %.4f)", realizedPnl, at.dailyPnL)

					at.contextBuilder.RecordTrade(true, realizedPnl > 0)
				} else {
					// 如果獲取失敗，至少記錄是一次平倉
					at.contextBuilder.RecordTrade(true, false)
				}
			default:
				// 成功执行后短暂延迟
				time.Sleep(1 * time.Second)
			}
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// ✅ 最關鍵修復：在執行循環結束後，才將帶有 ExecutionError 的決策記錄到歷史
	if decisionResult != nil {
		at.addToHistory(*decisionResult)
	}

	// 10. 追踪连续wait周期（用于触发分析瘫痪规则）
	// ⚠️ 分析瘫痪规则应针对「无新开仓机会」，而非「纯wait」
	// 情况1: wait + hold = 有持仓但没新机会 → 应计入「无新开仓机会」周期
	// 情况2: 纯hold = 有持仓在管理中 → 不计入（专注于持仓管理）
	// 情况3: wait only = 无持仓且无机会 → 应计入
	// 情况4: 有活跃动作 = 开仓/平仓等 → 重置计数
	hasHoldAction := false // 是否有hold动作
	hasWaitAction := false // 是否有wait动作
	hasOpenAction := false // 是否有开仓动作
	for _, d := range sortedDecisions {
		switch d.Action {
		case "wait":
			hasWaitAction = true
		case "hold":
			hasHoldAction = true
		case "open_long", "open_short":
			hasOpenAction = true
		default:
			// close_*, update_*, partial_close 等都是活跃动作（不影响分析瘫痪计数）
		}
	}

	// 修正：「无新开仓机会」的定义
	// - 没有任何开仓动作（open_long/open_short）
	// - wait + hold 也算「无新开仓机会」（有持仓但找不到新机会）
	noNewOpportunity := !hasOpenAction && (hasWaitAction || len(sortedDecisions) == 0)

	if noNewOpportunity && !hasHoldAction {
		// 纯wait周期（无持仓、无机会）
		at.contextBuilder.RecordWait()
		if at.contextBuilder.consecutiveWait >= 5 {
			log.Printf("⚠️ 已连续 %d 个周期wait（无持仓、无机会），触发分析瘫痪规则", at.contextBuilder.consecutiveWait)
		}
	} else if noNewOpportunity && hasHoldAction {
		// wait + hold 混合周期（有持仓但无新机会）
		at.contextBuilder.RecordWait()
		if at.contextBuilder.consecutiveWait >= 5 {
			log.Printf("⚠️ 已连续 %d 个周期无新开仓机会（有持仓管理中），触发分析瘫痪规则", at.contextBuilder.consecutiveWait)
		}
	} else if hasOpenAction {
		// 有开仓动作，重置连续wait计数
		at.contextBuilder.consecutiveWait = 0
	} else if hasHoldAction && !hasWaitAction {
		// 纯hold周期（专注持仓管理），不增加也不重置
		log.Printf("📊 持仓管理周期（纯hold），连续无机会计数保持: %d", at.contextBuilder.consecutiveWait)
	}

	// 11. 保存决策记录
	if err := at.decisionLogger.LogDecision(record); err != nil {
		log.Printf("⚠ 保存决策记录失败: %v", err)
	}

	return nil
}

// executeDecisionWithRecord 执行AI决策并记录详细信息
func (at *AutoTrader) executeDecisionWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	return at.executor.ExecuteDecisionWithRecord(decision, actionRecord)
}

func (at *AutoTrader) enforceRiskHalt(ctx *decision.Context, record *logger.DecisionRecord) bool {
	if at.timeProvider.Now().Before(at.stopUntil) {
		remaining := at.stopUntil.Sub(at.timeProvider.Now())
		log.Printf("⏸ 风险控制：暂停新开仓中，剩余 %.0f 分钟（至 %s）", remaining.Minutes(), at.stopUntil.Format(time.RFC3339))
		if ctx.Account.PositionCount == 0 {
			record.Success = false
			record.ErrorMessage = fmt.Sprintf("风险控制暂停中，剩余 %.0f 分钟", remaining.Minutes())
			at.decisionLogger.LogDecision(record)
		}
		return true
	}

	consecutiveLosses := 0
	if ctx.TradeHistory != nil {
		consecutiveLosses = ctx.TradeHistory.ConsecutiveLoss
	}

	kind, reason, halt := EvaluateRiskHalt(RiskHaltInput{
		DailyPnL:          at.dailyPnL,
		InitialBalance:    at.initialBalance,
		TotalPnLPct:       ctx.Account.TotalPnLPct,
		PeakDrawdownPct:   ctx.Account.PeakDrawdownPct,
		MaxDailyLossPct:   at.config.MaxDailyLoss,
		MaxDrawdownPct:    at.config.MaxDrawdown,
		ConsecutiveLosses: consecutiveLosses,
	})
	if !halt {
		return false
	}

	pauseDuration := at.config.StopTradingTime
	if kind == HaltConsecutiveLoss {
		if consecutiveLosses >= 3 {
			pauseDuration = 24 * time.Hour
		} else {
			pauseDuration = 45 * time.Minute
		}
	}

	at.stopUntil = HaltUntil(at.timeProvider.Now(), kind, pauseDuration)
	if err := at.contextBuilder.UpdateStopUntil(at.stopUntil); err != nil {
		log.Printf("⚠️ 保存风控暂停截止失败: %v", err)
	}
	log.Printf("🚨 [%s] 触发硬风控: %s；暂停新开仓至 %s", at.name, reason, at.stopUntil.Format(time.RFC3339))
	if ctx.Account.PositionCount == 0 {
		record.Success = false
		record.ErrorMessage = reason
		at.decisionLogger.LogDecision(record)
	} else {
		record.ErrorMessage = reason
	}
	return true
}

// GetID 获取trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetName 获取trader名称
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetAIModel 获取AI模型
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetExchange 获取交易所
func (at *AutoTrader) GetExchange() string {
	return at.exchange
}

// SetCustomPrompt 设置自定义交易策略prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
	at.customPrompt = prompt
}

// SetOverrideBasePrompt 设置是否覆盖基础prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
	at.overrideBasePrompt = override
}

// SetSystemPromptTemplate 设置系统提示词模板
func (at *AutoTrader) SetSystemPromptTemplate(templateName string) {
	at.systemPromptTemplate = templateName
}

// GetSystemPromptTemplate 获取当前系统提示词模板名称
func (at *AutoTrader) GetSystemPromptTemplate() string {
	return at.systemPromptTemplate
}

// GetDecisionLogger 获取决策日志记录器
func (at *AutoTrader) GetDecisionLogger() *logger.DecisionLogger {
	return at.decisionLogger
}

// GetInitialBalance 获取初始余额（用于调试和验证）
func (at *AutoTrader) GetInitialBalance() float64 {
	return at.initialBalance
}

// UpdateInitialBalance 更新初始余额（用于同步数据库中的初始余额）
// ✅ 修正：添加保護機制，只在合理情況下更新初始餘額
func (at *AutoTrader) UpdateInitialBalance(newInitialBalance float64) {
	oldBalance := at.initialBalance

	// 保護機制：檢查更新是否合理
	if newInitialBalance <= 0 {
		log.Printf("❌ [%s] 拒绝更新初始余额：新值无效 (%.2f USDT)", at.name, newInitialBalance)
		return
	}

	// 如果差異太小（可能是浮點誤差），不更新
	if math.Abs(newInitialBalance-oldBalance) < 0.01 {
		log.Printf("ℹ️ [%s] 初始余额差异很小，无需更新: %.2f USDT", at.name, oldBalance)
		return
	}

	// 警告：如果差異很大，可能是錯誤操作
	if math.Abs(newInitialBalance-oldBalance)/oldBalance > 0.5 {
		log.Printf("⚠️ [%s] 警告：初始余额变化超过50%% (%.2f → %.2f)，请确认是否正确",
			at.name, oldBalance, newInitialBalance)
	}

	at.initialBalance = newInitialBalance
	at.config.InitialBalance = newInitialBalance // 同时更新配置中的初始余额
	log.Printf("✅ [%s] 更新初始余额（本金）: %.2f → %.2f USDT", at.name, oldBalance, newInitialBalance)
	log.Printf("ℹ️ [%s] 提醒：初始余额是投入本金，只应在充值/提现时更新", at.name)
}

// GetConfig 获取当前配置
func (at *AutoTrader) GetConfig() AutoTraderConfig {
	return at.config
}

// GetTradingCoins 获取交易币种列表（用于资源清理）
func (at *AutoTrader) GetTradingCoins() []string {
	return at.tradingCoins
}

// GetStatus 获取系统状态（用于API）
func (at *AutoTrader) GetStatus() map[string]interface{} {
	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	halted := at.timeProvider.Now().Before(at.stopUntil)
	consecutiveWait := 0
	if at.contextBuilder != nil {
		consecutiveWait = at.contextBuilder.GetConsecutiveWait()
	}
	status := map[string]interface{}{
		"trader_id":        at.id,
		"trader_name":      at.name,
		"ai_model":         at.aiModel,
		"exchange":         at.exchange,
		"is_running":       at.isRunning,
		"start_time":       at.startTime.Format(time.RFC3339),
		"runtime_minutes":  int(time.Since(at.startTime).Minutes()),
		"call_count":       at.callCount,
		"initial_balance":  at.initialBalance,
		"scan_interval":    at.config.ScanInterval.String(),
		"stop_until":       at.stopUntil.Format(time.RFC3339),
		"last_reset_time":  at.lastResetTime.Format(time.RFC3339),
		"ai_provider":      aiProvider,
		"risk_halted":      halted,
		"consecutive_wait": consecutiveWait,
		"daily_pnl":        at.dailyPnL,
		"injected_skills":  at.lastPlaybooks,
	}
	if dir, err := at.currentOperatorDirective(); err == nil {
		status["operator_pause_opens"] = dir.PauseOpens
		status["operator_pause_actor"] = dir.PauseActor
		if dir.PauseUntil != nil {
			status["operator_pause_until"] = dir.PauseUntil.Format(time.RFC3339)
		}
		status["operator_note_count"] = len(dir.Notes)
	}
	return status
}

func (at *AutoTrader) currentOperatorDirective() (config.OperatorDirective, error) {
	db, ok := at.database.(*config.Database)
	if !ok || db == nil {
		return config.OperatorDirective{}, fmt.Errorf("no config database")
	}
	return db.CurrentOperatorDirective(at.timeProvider.Now())
}

func (at *AutoTrader) applyOperatorDirective(ctx *decision.Context, record *logger.DecisionRecord) bool {
	dir, err := at.currentOperatorDirective()
	if err != nil {
		return false
	}
	digest := config.OperatorDigest(dir, at.timeProvider.Now())
	if ctx != nil {
		ctx.OperatorDigest = digest
	}
	if !dir.PauseOpens {
		return false
	}
	msg := fmt.Sprintf("operator pause_opens by %s", dir.PauseActor)
	if dir.PauseUntil != nil {
		msg += " until " + dir.PauseUntil.Format(time.RFC3339)
	}
	log.Printf("⏸ [%s] %s", at.name, msg)
	if record != nil {
		record.ExecutionLog = append(record.ExecutionLog, msg)
	}
	return true
}

// GetAccountInfo 获取账户信息（用于API）
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取余额失败: %w", err)
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

	// 获取持仓计算总保证金
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnL := 0.0
	for _, pos := range positions {
		markPrice, _ := pos["markPrice"].(float64)
		quantity, _ := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl, _ := pos["unRealizedProfit"].(float64)
		totalUnrealizedPnL += unrealizedPnl

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	// ⚠️ 重要：总盈亏计算 = 当前总净值 - 初始余额
	// 初始余额应该是用户设置的起始本金（如 20.13 USDT）
	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	// 始終輸出調試日誌，以便追蹤初始餘額問題
	log.Printf("📊 [%s] 账户信息计算详情:", at.name)
	log.Printf("  • 配置中的初始余额 (config.InitialBalance): %.2f USDT", at.config.InitialBalance)
	log.Printf("  • 内存中的初始余额 (at.initialBalance): %.2f USDT", at.initialBalance)
	log.Printf("  • 钱包余额 (wallet_balance): %.2f USDT", totalWalletBalance)
	log.Printf("  • 未实现盈亏 (unrealized_profit): %.2f USDT", totalUnrealizedProfit)
	log.Printf("  • 总净值 (total_equity): %.2f USDT", totalEquity)
	log.Printf("  • 总盈亏 (total_pnl): %.2f USDT = %.2f - %.2f", totalPnL, totalEquity, at.initialBalance)
	log.Printf("  • 总盈亏百分比 (total_pnl_pct): %.2f%%", totalPnLPct)

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	return map[string]interface{}{
		// 核心字段
		"total_equity":      totalEquity,           // 账户净值 = wallet + unrealized
		"wallet_balance":    totalWalletBalance,    // 钱包余额（不含未实现盈亏）
		"unrealized_profit": totalUnrealizedProfit, // 未实现盈亏（从API）
		"available_balance": availableBalance,      // 可用余额

		// 盈亏统计
		"total_pnl":            totalPnL,           // 总盈亏 = equity - initial
		"total_pnl_pct":        totalPnLPct,        // 总盈亏百分比
		"total_unrealized_pnl": totalUnrealizedPnL, // 未实现盈亏（从持仓计算）
		"initial_balance":      at.initialBalance,  // 初始余额
		"daily_pnl":            at.dailyPnL,        // 日盈亏

		// 持仓信息
		"position_count":  len(positions),  // 持仓数量
		"margin_used":     totalMarginUsed, // 保证金占用
		"margin_used_pct": marginUsedPct,   // 保证金使用率
	}, nil
}

// GetPositions 获取持仓列表（用于API）
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		entryPrice, _ := pos["entryPrice"].(float64)
		markPrice, _ := pos["markPrice"].(float64)
		quantity, _ := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl, _ := pos["unRealizedProfit"].(float64)
		liquidationPrice, _ := pos["liquidationPrice"].(float64)

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		// 计算占用保证金
		marginUsed := (quantity * markPrice) / float64(leverage)

		// 计算盈亏百分比（基于保证金）
		// 收益率 = 未实现盈亏 / 保证金 × 100%
		pnlPct := 0.0
		if marginUsed > 0 {
			pnlPct = (unrealizedPnl / marginUsed) * 100
		}

		result = append(result, map[string]interface{}{
			"symbol":             symbol,
			"side":               side,
			"entry_price":        entryPrice,
			"mark_price":         markPrice,
			"quantity":           quantity,
			"leverage":           leverage,
			"unrealized_pnl":     unrealizedPnl,
			"unrealized_pnl_pct": pnlPct,
			"liquidation_price":  liquidationPrice,
			"margin_used":        marginUsed,
		})
	}

	return result, nil
}

// startOrphanMonitor 启动孤儿仓位监控（无止损保护的仓位）
func (at *AutoTrader) startDrawdownMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		orphanTicker := time.NewTicker(30 * time.Second) // 每30秒检查孤儿仓位
		defer orphanTicker.Stop()

		log.Println("📊 启动孤儿仓位监控（30秒周期）")

		for {
			select {
			case <-orphanTicker.C:
				at.checkOrphanPositions()
			case <-at.stopMonitorCh:
				log.Println("⏹ 停止孤儿仓位监控")
				return
			}
		}
	}()
}

// checkOrphanPositions 检查孤儿仓位（无止损保护的仓位）
// 如果发现持仓时间 > 60秒且无止损，立即紧急平仓
func (at *AutoTrader) checkOrphanPositions() {
	positions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("❌ 孤儿仓位检测：获取持仓失败: %v", err)
		return
	}

	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		quantity, _ := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}

		// 跳过空仓位
		if quantity == 0 || symbol == "" || side == "" {
			continue
		}

		// 仅对机器人开立的持仓进行孤儿止损监控，避免误平用户在交易所或网页端手工开立的单子
		if at.contextBuilder.GetEntryReason(symbol, side) == "" {
			continue
		}

		// 获取持仓时间 — 🔒 使用併發安全的存取方法
		posKey := symbol + "_" + side
		firstSeenTime, exists := at.contextBuilder.GetPositionFirstSeenTime(posKey)
		if !exists {
			// 首次看到，记录时间并跳过（给予 60 秒宽限期）
			at.contextBuilder.SetPositionFirstSeenTime(posKey, time.Now().UnixMilli())
			continue
		}

		// 检查持仓时间是否超过 60 秒
		holdingDuration := time.Since(time.UnixMilli(firstSeenTime))
		if holdingDuration < 60*time.Second {
			continue // 宽限期内，跳过
		}

		// 检查是否有止损保护
		stopLoss, _, err := at.trader.GetOrderProtection(symbol, side)
		if err != nil {
			log.Printf("⚠️ 孤儿仓位检测：获取止损信息失败 (%s %s): %v", symbol, side, err)
			continue
		}

		// 如果没有止损保护（stopLoss == 0），触发紧急平仓
		if stopLoss == 0 {
			log.Printf("🚨 检测到孤儿仓位: %s %s (持仓 %s，无止损保护)，执行紧急平仓！",
				symbol, side, holdingDuration.Round(time.Second))

			if err := at.executor.EmergencyClosePosition(symbol, side); err != nil {
				log.Printf("❌ 孤儿仓位紧急平仓失败 (%s %s): %v（请手动处理！）", symbol, side, err)
			} else {
				log.Printf("✅ 孤儿仓位紧急平仓成功: %s %s", symbol, side)
				// 清理缓存 — 🔒 使用併發安全的刪除方法
				at.contextBuilder.DeletePositionFirstSeenTime(posKey)
			}
		}
	}
}

// GetPeakPnLCache 获取最高收益缓存
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// 返回缓存的副本
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL 更新最高收益缓存
func (at *AutoTrader) UpdatePeakPnL(symbol string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	
	updated := false
	if peak, exists := at.peakPnLCache[symbol]; exists {
		// 更新峰值（如果是多头，取较大值；如果是空头，currentPnLPct为负，也要比较）
		if currentPnLPct > peak {
			at.peakPnLCache[symbol] = currentPnLPct
			updated = true
		}
	} else {
		// 首次记录
		at.peakPnLCache[symbol] = currentPnLPct
		updated = true
	}

	cacheCopy := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cacheCopy[k] = v
	}
	at.peakPnLCacheMutex.Unlock()

	if updated {
		if err := at.contextBuilder.UpdatePeakPnLCache(cacheCopy); err != nil {
			log.Printf("⚠️ 保存盈亏峰值快取失败: %v", err)
		}
	}
}

// ClearPeakPnLCache 清除指定symbol的峰值缓存
func (at *AutoTrader) ClearPeakPnLCache(symbol string) {
	at.peakPnLCacheMutex.Lock()
	delete(at.peakPnLCache, symbol)

	cacheCopy := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cacheCopy[k] = v
	}
	at.peakPnLCacheMutex.Unlock()

	if err := at.contextBuilder.UpdatePeakPnLCache(cacheCopy); err != nil {
		log.Printf("⚠️ 保存盈亏峰值快取失败: %v", err)
	}
}

// truncateString 截取字符串（处理多字节字符）
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// addToHistory 添加决策到历史记录（保持最近5条）
func (at *AutoTrader) addToHistory(decision decision.FullDecision) {
	// 将新决策添加到末尾
	at.decisionHistory = append(at.decisionHistory, decision)

	// 如果超过限制，移除最早的记录
	maxHistory := 5
	if len(at.decisionHistory) > maxHistory {
		at.decisionHistory = at.decisionHistory[len(at.decisionHistory)-maxHistory:]
	}

	if err := at.contextBuilder.UpdateDecisionHistory(at.decisionHistory); err != nil {
		log.Printf("⚠️ 保存决策历史失败: %v", err)
	}
}
