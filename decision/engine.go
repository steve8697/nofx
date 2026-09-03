package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"aetheris/config"
	"aetheris/logger"
	"aetheris/market"
	"aetheris/mcp"
	"aetheris/pool"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // 持仓更新时间戳（毫秒）
	StopLoss         float64 `json:"stop_loss,omitempty"`
	TakeProfit       float64 `json:"take_profit,omitempty"`
	EntryReasoning   string  `json:"entry_reasoning,omitempty"` // 开仓理由 (从 Trade Memory 获取)
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // 未实现盈亏 (Floating PnL)
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// TradeHistory 交易历史追踪（用于冷却期检查）
type TradeHistory struct {
	LastCloseTime   time.Time `json:"-"` // 上次平仓时间
	TradesInHour    int       `json:"-"` // 过去1小时交易次数
	LastTradeTime   time.Time `json:"-"` // 上次交易时间
	ConsecutiveLoss int       `json:"-"` // 连续亏损次数
	ConsecutiveWait int       `json:"-"` // 连续wait次数（用于触发分析瘫痪规则）
}

// Context 交易上下文（传递给AI的完整信息）
type Context struct {
	CurrentTime     string                  `json:"current_time"`
	RuntimeMinutes  int                     `json:"runtime_minutes"`
	CallCount       int                     `json:"call_count"`
	Account         AccountInfo             `json:"account"`
	Positions       []PositionInfo          `json:"positions"`
	CandidateCoins  []CandidateCoin         `json:"candidate_coins"`
	MarketDataMap   map[string]*market.Data `json:"-"` // 不序列化，但内部使用
	OITopDataMap    map[string]*OITopData   `json:"-"` // OI Top数据映射
	Performance     interface{}             `json:"-"` // 历史表现分析（logger.PerformanceAnalysis）
	BTCETHLeverage  int                     `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage int                     `json:"-"` // 山寨币杠杆倍数（从配置读取）
	DataProvider    market.DataProvider     `json:"-"` // 市场数据提供者 (替换 WSMonitor)
	PromptRules     *config.PromptRules     `json:"-"` // 提示词规则配置
	TradeHistory    *TradeHistory           `json:"-"` // 交易历史追踪（冷却期检查）
	RecentDecisions []FullDecision          `json:"-"` // AI最近的决策历史（短期记忆）
	OperatorDigest  string                  `json:"-"` // 未过期的外部干涉摘要
	LiquidityEngine *market.LiquidityEngine `json:"-"` // 🔥 持续化流动性引擎

	// 💰 手续费信息（用于AI决策时考虑交易成本）
	TradingFeeInfo *TradingFeeInfo `json:"-"`
}

// TradingFeeInfo 交易手续费信息
type TradingFeeInfo struct {
	MakerFeeRate     float64 // Maker费率 (例如 0.0001 = 0.01%)
	TakerFeeRate     float64 // Taker费率 (例如 0.00035 = 0.035%)
	RoundTripFeeRate float64 // 来回总费率 (Taker开仓 + Taker平仓)
	MinProfitPct     float64 // 最小止盈百分比 (需 > 来回费率才能盈利)
	ExchangeName     string  // 交易所名称
}

// Decision AI的交易决策
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // "open_long", "open_short", "close_long", "close_short", "update_stop_loss", "update_take_profit", "partial_close", "hold", "wait"

	// 开仓参数
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	EntryPrice      float64 `json:"entry_price,omitempty"` // 入场价格（当前市价）- 用于准确计算风险回报比
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`

	// 调整参数（新增）
	NewStopLoss     float64 `json:"new_stop_loss,omitempty"`    // 用于 update_stop_loss
	NewTakeProfit   float64 `json:"new_take_profit,omitempty"`  // 用于 update_take_profit
	ClosePercentage float64 `json:"close_percentage,omitempty"` // 用于 partial_close (0-100)

	// 通用参数
	Confidence int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning  string  `json:"reasoning"`

	// 执行反馈（用于下一周期记忆）
	ExecutionError string `json:"execution_error,omitempty"` // 执行错误信息
}

// FullDecision AI的完整决策（包含思维链）
type FullDecision struct {
	SystemPrompt string     `json:"system_prompt"` // 系统提示词（发送给AI的系统prompt）
	UserPrompt   string     `json:"user_prompt"`   // 发送给AI的输入prompt
	CoTTrace     string     `json:"cot_trace"`     // 思维链分析（AI输出）
	Decisions    []Decision `json:"decisions"`     // 具体决策列表
	Timestamp    time.Time  `json:"timestamp"`
	Playbooks    []string   `json:"playbooks,omitempty"` // 本轮注入的 skills
}

// GetFullDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullDecision(ctx *Context, mcpClient *mcp.Client) (*FullDecision, error) {
	return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false, "")
}

// GetFullDecisionWithCustomPrompt 获取AI的完整交易决策（支持自定义prompt和模板选择）
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient *mcp.Client, customPrompt string, overrideBase bool, templateName string) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	consecutiveWait := 0
	if ctx.TradeHistory != nil {
		consecutiveWait = ctx.TradeHistory.ConsecutiveWait
	}
	maxPosForSkills := 3
	if ctx.PromptRules != nil && ctx.PromptRules.MaxPositions > 0 {
		maxPosForSkills = ctx.PromptRules.MaxPositions
	}
	playbooks := SelectPlaybooks(len(ctx.Positions), maxPosForSkills)
	systemPrompt := buildSystemPromptWithCustom(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, ctx.PromptRules, customPrompt, overrideBase, templateName, consecutiveWait, playbooks)
	userPrompt := buildUserPrompt(ctx)

	// 3. 调用AI API（使用 system + user prompt）
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	// 4. 解析AI响应（现在传入市场数据用于入场价验证）
	// ⚠️ 关键修复：从配置中解析 Validator 所需的动态参数
	minRR := 2.0 // 默认值
	if ctx.PromptRules != nil && ctx.PromptRules.RiskRewardRatio != "" {
		minRR = parseRiskRewardRatio(ctx.PromptRules.RiskRewardRatio)
	}
	maxPositions := 3
	if ctx.PromptRules != nil && ctx.PromptRules.MaxPositions > 0 {
		maxPositions = ctx.PromptRules.MaxPositions
	}

	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, ctx.Positions, ctx.MarketDataMap, minRR, maxPositions, ctx.TradingFeeInfo)

	// Assign prompt to decision immediately so it's available even if parsing fails partially
	// or if validation fails.
	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt // 保存系统prompt
		decision.UserPrompt = userPrompt     // 保存输入prompt
		decision.Playbooks = playbooks
	}

	if err != nil {
		return decision, fmt.Errorf("解析AI响应失败: %w", err)
	}

	return decision, nil
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// 收集所有需要获取数据的币种
	symbolSet := make(map[string]bool)

	// Fetch Global Sentiment once (Optimization)
	sentiment, err := market.GetCryptoFearGreedIndex()
	if err != nil {
		log.Printf("⚠️ 获取市场情绪失败: %v", err)
	}

	// 1. 优先获取持仓币种的数据（这是必须的）
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// 并发获取市场数据
	// 持仓币种集合（用于判断是否跳过OI检查）
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	// 批量获取CoinGecko数据（优化：减少API调用次数）
	symbolsList := make([]string, 0, len(symbolSet))
	for symbol := range symbolSet {
		symbolsList = append(symbolsList, symbol)
	}
	coinGeckoDataMap := market.GetCoinGeckoDataBatch(symbolsList)

	for symbol := range symbolSet {
		// 🔥 Phase 30: Use persistent LiquidityEngine
		data, err := market.Get(symbol, ctx.DataProvider, ctx.LiquidityEngine)
		if err != nil {
			// 单个币种失败不影响整体，只记录错误
			log.Printf("⚠️ 获取 %s 市场数据失败: %v", symbol, err)
			continue
		}

		// 添加CoinGecko数据（如果可用）
		if coinGeckoData, ok := coinGeckoDataMap[symbol]; ok {
			data.CoinGeckoData = coinGeckoData
		}

		// ⚠️ 流动性过滤：持仓价值低于阈值的币种不做（多空都不做）
		// 持仓价值 = 持仓量 × 当前价格
		// 但现有持仓必须保留（需要决策是否平仓）
		// 💡 OI 門檻配置：用戶可根據風險偏好調整
		// 💡 OI 門檻配置：用戶可根據風險偏好調整
		minOIThresholdMillions := 15.0 // 默认值
		if ctx.PromptRules != nil && ctx.PromptRules.OIThresholdMillions > 0 {
			minOIThresholdMillions = ctx.PromptRules.OIThresholdMillions
		}

		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// 计算持仓价值（USD）= 持仓量 × 当前价格
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // 转换为百万美元单位
			if oiValueInMillions < minOIThresholdMillions {
				log.Printf("⚠️  %s 持仓价值过低(%.2fM USD < %.1fM)，跳过此币种 [持仓量:%.0f × 价格:%.4f]",
					symbol, oiValueInMillions, minOIThresholdMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		// 🔥 Phase 30: Use Persistent Liquidity Engine if available
		if ctx.LiquidityEngine != nil {
			// Init if needed (only once per symbol usually, but safe to call)
			ctx.LiquidityEngine.Init(symbol)

			// Update with latest candle and OI
			// Note: We need the *latest* kline. Accessing data.IntradaySeries (3m) or 15m klines?
			// CalculateLiquidityClusters in market.Get uses klines1h.
			// Let's use 15m or 1h? LiquidityEngine logic supports 15m history.
			// Ideally we assume market.Get has already fetched klines.
			// But market.Data struct doesn't expose raw klines easily (it exposes Series).
			// We can't easy pass Klines here without a refactor of market.Get return type.
			// WORKAROUND: We will stick to the plan:
			// The LiquidityEngine.Update takes (Kyle, OIData).
			// We don't have raw Kline here easily.
			// WAIT. market.Get returns *Data. It doesn't return raw Klines.
			// However, LiquidityClusters were calculated in market.Get using a "Lite" version.
			// If we want to use the "Real" engine, we should ideally do it INSIDE market.Get
			// or pass the raw klines out.
			// modifying market.Get to take the engine is the cleanest "System" way, but invasive.

			// Alternative: `data` contains `IntradaySeries` (3m).
			// We can construct a "Synthetic" Kline from the latest data point? No, too hacky.

			// Let's go with the invasive but correct route:
			// Modify market.Get to accept LiquidityEngine?
			// OR
			// Just verify that the "Lite" version is actually "Good Enough" for now?
			// User said "Check if Plugged In". The "Real" engine is NOT plugged in.
			// If I can't plug it in here easily, I should have modified market.Get.

			// Let's backtrack slightly.
			// `market.Get` is in `market/data.go`. It has access to multiple timeframes of Klines.
			// Passing `LiquidityEngine` to `market.Get` is the BEST way.
			// `market.Get(symbol, provider, liquidityEngine)`
			// Then `market.Get` can call `liquidityEngine.Update(latestKline)` and use it.

			// But `market.Get` is called in `engine.go`.
			// I can change `market.Get` signature.
		}

		// Inject Global Sentiment
		data.Sentiment = sentiment

		ctx.MarketDataMap[symbol] = data
	}

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *Context) int {
	// ⚠️ 重要：限制候选币种数量，避免 Prompt 过大
	// 根据持仓数量动态调整：持仓越少，可以分析更多候选币
	const (
		maxCandidatesWhenEmpty    = 30 // 无持仓时最多分析30个候选币
		maxCandidatesWhenHolding1 = 25 // 持仓1个时最多分析25个候选币
		maxCandidatesWhenHolding2 = 20 // 持仓2个时最多分析20个候选币
		maxCandidatesWhenHolding3 = 15 // 持仓3个时最多分析15个候选币（避免 Prompt 过大）
	)

	positionCount := len(ctx.Positions)
	var maxCandidates int

	switch positionCount {
	case 0:
		maxCandidates = maxCandidatesWhenEmpty
	case 1:
		maxCandidates = maxCandidatesWhenHolding1
	case 2:
		maxCandidates = maxCandidatesWhenHolding2
	default: // 3+ 持仓
		maxCandidates = maxCandidatesWhenHolding3
	}

	// 返回实际候选币数量和上限中的较小值
	return min(len(ctx.CandidateCoins), maxCandidates)
}

// buildSystemPromptWithCustom 构建包含自定义内容的 System Prompt
func buildSystemPromptWithCustom(accountEquity float64, btcEthLeverage, altcoinLeverage int, promptRules *config.PromptRules, customPrompt string, overrideBase bool, templateName string, consecutiveWait int, playbooks []string) string {
	// 如果覆盖基础prompt且有自定义prompt，只使用自定义prompt
	if overrideBase && customPrompt != "" {
		return customPrompt
	}

	// 获取基础prompt（使用指定的模板）
	basePrompt := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, promptRules, templateName, consecutiveWait, playbooks)

	// 如果没有自定义prompt，直接返回基础prompt
	if customPrompt == "" {
		return basePrompt
	}

	// 添加自定义prompt部分到基础prompt
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📌 个性化交易策略\n\n")
	sb.WriteString(customPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("注意: 以上个性化策略是对基础规则的补充，不能违背基础风险控制原则。\n")

	return sb.String()
}

// buildSystemPrompt 构建 System Prompt（使用模板+动态部分）
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, promptRules *config.PromptRules, templateName string, consecutiveWait int, playbooks []string) string {
	var sb strings.Builder

	// 1. 加载提示词模板（核心交易策略部分）
	if templateName == "" {
		templateName = "default" // 默认使用 default 模板
	}

	template, err := GetPromptTemplate(templateName)
	var templateContent string
	if err != nil {
		// 如果模板不存在，记录错误并使用 default
		log.Printf("⚠️  提示词模板 '%s' 不存在，使用 default: %v", templateName, err)
		template, err = GetPromptTemplate("default")
		if err != nil {
			// 如果连 default 都不存在，使用内置的简化版本
			log.Printf("❌ 无法加载任何提示词模板，使用内置简化版本")
			templateContent = "你是专业的加密货币交易AI。请根据市场数据做出交易决策。\n\n"
		} else {
			templateContent = template.Content + "\n\n"
		}
	} else {
		templateContent = template.Content + "\n\n"
	}

	if playbooksLoaded() {
		injected := assemblePlaybooks(playbooks)
		templateContent += injected
		if injected != "" {
			log.Printf("📎 本轮注入 skills: %s", strings.Join(playbooks, ", "))
		}
	} else if monolith, err := os.ReadFile(filepath.Join("prompts", "reference", "adaptive_full.md")); err == nil {
		log.Printf("⚠️ skills 未加载，回退 prompts/reference/adaptive_full.md")
		templateContent = string(monolith) + "\n\n"
	}

	// ⚠️ 动态调整：周期性触发分析瘫痪探测机制
	// 逻辑：每15个周期作为一个大窗口
	// 0-4: 正常标准
	// 5-7: 降级探测 (3个周期) -> 尝试寻找被略过的机会
	// 8-14: 恢复正常 (7个周期) -> 避免长期低标准
	// 15-19: 正常 (进入下一个循环)
	// 20-22: 再次降级探测...

	cyclePos := consecutiveWait % 15
	isProbePhase := consecutiveWait >= 5 && (cyclePos >= 5 && cyclePos <= 7)

	// 确保promptRules不为nil（必须在使用前检查）
	if promptRules == nil {
		promptRules = &config.PromptRules{
			RiskRewardRatio:        "1:3",
			MaxPositions:           3,
			MaxPositionSizeAltcoin: 1.2,
			MaxPositionSizeBTCETH:  8.0,
			MarginUsageLimit:       90.0,
			MinPositionSize:        12.0,
		}
	}

	// 動態計算高价格币种（BTC/ETH）最小開倉金額（與驗證邏輯一致）
	minPositionSizeHighPrice := promptRules.MinPositionSize + 3.0 + float64(btcEthLeverage)*2.0
	if minPositionSizeHighPrice > 30.0 {
		minPositionSizeHighPrice = 30.0
	}

	// 💰 動態計算自適應上限，避免「上限小於下限」的空集矛盾
	isMicroEquityMode := accountEquity < 10.0
	maxAltcoinSize := accountEquity * promptRules.MaxPositionSizeAltcoin
	if maxAltcoinSize < promptRules.MinPositionSize {
		maxAltcoinSize = promptRules.MinPositionSize
	}
	maxBtcEthSize := accountEquity * promptRules.MaxPositionSizeBTCETH
	if maxBtcEthSize < minPositionSizeHighPrice {
		maxBtcEthSize = minPositionSizeHighPrice
	}

	if isProbePhase {
		log.Printf("⚠️ 检测到连续 %d 次wait（探测阶段 %d/7），正在动态调整System Prompt阈值...", consecutiveWait, cyclePos)

		// 替换技术评分门槛（关键：仅放宽拒绝门槛，不改变高分定义，避免风险放大）
		// 1. 放宽拒绝门槛
		templateContent = strings.ReplaceAll(templateContent, "技术评分 < 70", "技术评分 < 60")
		templateContent = strings.ReplaceAll(templateContent, "**< 70**：Wait", "**< 60**：Wait")

		// 2. 调整微仓位区间（允许60-70分段进入微仓位或中等仓位）
		templateContent = strings.ReplaceAll(templateContent, "**70-79**：微仓位", "**60-79**：微仓位")

		// 3. 消除 60-69 分段的仓位矛盾（原定义75%，恢复期强制微仓位）
		templateContent = strings.ReplaceAll(templateContent, "| 60-69 | 75% |", "| 60-69 | 微(恢复) |")
		templateContent = strings.ReplaceAll(templateContent, "→ 75% 仓位", "→ 微仓位(恢复期)")

		// 注意：不要替换 "技术评分 ≥ 70"，保留它作为强力信号的标准
		// 这样 60-69 分将自然落入 "中等信号" (60-69) 或 "微仓位" 区间，而不是被当做强力信号

		// 替换信心度门槛（信心度可以整体下调，因為它是主觀評估）
		templateContent = strings.ReplaceAll(templateContent, "信心度≥80", "信心度≥75")
		templateContent = strings.ReplaceAll(templateContent, "信心度 80-85", "信心度 75-85")
		templateContent = strings.ReplaceAll(templateContent, "信心度 <70", "信心度 <60")
		templateContent = strings.ReplaceAll(templateContent, "信心度 70-79", "信心度 60-75")
		templateContent = strings.ReplaceAll(templateContent, "评分≥80", "评分≥75") // 针对行41

		// 在开头添加显式说明
		extraNotice := fmt.Sprintf("# ⚠️ 特殊模式：分析瘫痪周期性探测 (连续Wait: %d | 阶段: %d/7)\n**评分门槛暂降至60，信心度门槛暂降至75。此调整将在本轮探测结束（阶段>7）后自动恢复正常。**\n**强制风控：在恢复期内，所有 60-75 分段的信号（无论是技术评分还是信心度）都必须严格执行微仓位（1%% Risk）风控，禁止大仓位博弈。**\n\n", consecutiveWait, cyclePos)
		templateContent = extraNotice + templateContent
	} else if consecutiveWait >= 8 {
		// 非探测阶段，且wait时间较长，恢复正常标准并提醒
		log.Printf("ℹ️ 连续wait已达 %d 周期，处于正常标准观察期 (下一次探测在 +%d 周期后)",
			consecutiveWait, 15-cyclePos+5)
		extraNotice := fmt.Sprintf("# ℹ️ 市场观望期 (连续Wait: %d)\n**已恢复正常评分标准。当前市场可能确实缺乏良好机会，请保持耐心等待高质量信号。**\n(下一次低门槛探测将在 %d 个周期后自动触发)\n\n",
			consecutiveWait, (15-cyclePos+5)%15)
		templateContent = extraNotice + templateContent
	}

	sb.WriteString(templateContent)

	// 2. 硬约束（风险控制）- 动态生成
	sb.WriteString("# 硬约束（风险控制）\n\n")
	if isMicroEquityMode {
		sb.WriteString("⚠️ **【自適應微型資金風控模式已啟動 (Micro-Equity Mode)】**\n")
		sb.WriteString(fmt.Sprintf("   - 當前賬戶淨值極低（%.2f USDT < 10 USDT）。\n", accountEquity))
		sb.WriteString(fmt.Sprintf("   - 為避免開倉「名義價值上限小於下限」的數學空集死鎖，系統已將單幣名義價值上限與開倉下限對齊（山寨幣 %.0f USDT，BTC/ETH %.0f USDT）。\n", promptRules.MinPositionSize, minPositionSizeHighPrice))
		sb.WriteString("   - 單筆最大風險 (risk_usd) 已放寬至淨值的 4%-5.5%，以容納正常的 ATR 波動空間。請使用最低槓桿（2x-5x）進行極小額度開倉嘗試。\n\n")
	}

	sb.WriteString(fmt.Sprintf("1. 风险回报比: 必须 ≥ %s（格式說明：X:1 表示賺X冒1的風險，例如 3:1 表示賺3冒1）\n", promptRules.RiskRewardRatio))
	sb.WriteString(fmt.Sprintf("2. 最多持仓: %d个币种（质量>数量）\n", promptRules.MaxPositions))
	sb.WriteString(fmt.Sprintf("3. 单币仓位（名義價值上限）: **山寨币 ≤%.0f USDT** | **BTC/ETH ≤%.0f USDT**\n",
		maxAltcoinSize, maxBtcEthSize))
	sb.WriteString("   ⚠️ **重要**：position_size_usd 是名義價值（包含槓桿），必須遵守上述上限！\n")
	sb.WriteString(fmt.Sprintf("   💡 **計算方法**：名義價值 = 可用餘額 × 槓桿，但**絕對不得超過 %.0f USDT**（山寨幣）或 %.0f USDT（BTC/ETH）\n",
		maxAltcoinSize, maxBtcEthSize))
	sb.WriteString(fmt.Sprintf("   ⚠️ **關鍵**：上述上限是基於**賬戶淨值（%.2f USDT）**與最低開倉下限自適應調整計算的，不是基於可用餘額！\n", accountEquity))
	sb.WriteString("   ⚠️ **常見錯誤1**：不要直接用可用餘額作为position_size_usd！position_size_usd = 可用餘額 × 槓桿（名義價值）\n")
	sb.WriteString("   ⚠️ **常見錯誤2**：不要用賬戶淨值 × 倍數作為position_size_usd！應該用可用餘額 × 槓桿，且不超過上述上限\n")
	sb.WriteString("   ⚠️ **如果可用餘額 < 最小開倉金額，禁止開倉，必須給出 wait 決策**\n")
	sb.WriteString(fmt.Sprintf("4. 杠杆限制: **山寨币最大%dx杠杆** | **BTC/ETH最大%dx杠杆** (⚠️ 严格执行，不可超过)\n", altcoinLeverage, btcEthLeverage))
	sb.WriteString(fmt.Sprintf("5. 保证金: 总使用率 ≤ %.0f%%\n", promptRules.MarginUsageLimit))

	sb.WriteString(fmt.Sprintf("6. 开仓金额（position_size_usd，名義價值）: **山寨币 ≥%.0f USDT** | **BTC/ETH 必须 ≥%.0f USDT** (使用配置杠杆%dx时，因价格高且精度限制，避免数量四舍五入为0)\n",
		promptRules.MinPositionSize, minPositionSizeHighPrice, btcEthLeverage))
	sb.WriteString("   💡 **概念澄清**：position_size_usd 是**名義價值**（包含槓桿），實際需要的**保證金** = position_size_usd ÷ 槓桿\n")
	sb.WriteString("   💡 **例如**：開倉 30 USDT 名義價值（BTC/ETH 最小要求），使用 10x 槓桿，實際只需 30÷10 = **3 USDT 保證金**\n")
	sb.WriteString(fmt.Sprintf("   ⚠️ **如果可開倉名義價值（餘額×槓桿）< %.0f USDT，禁止開 BTC/ETH 仓位，只能交易山寨币**\n", minPositionSizeHighPrice))
	sb.WriteString(fmt.Sprintf("   ⚠️ **如果可開倉名義價值（餘額×槓桿）< %.0f USDT，禁止开任何仓位，必须给出 wait 决策**\n", promptRules.MinPositionSize))
	sb.WriteString(fmt.Sprintf("   ⚠️ **重要：最小开仓金额会根据你选择的杠杆倍数动态调整（BTC/ETH: %.0f + 杠杆×2，最高30 USDT）**\n", promptRules.MinPositionSize+3.0))

	sb.WriteString("7. 止损止盈规则:\n")
	sb.WriteString("   - **做多 (Long)**: 止损 < 入场价 < 止盈\n")
	sb.WriteString("   - **做空 (Short)**: 止盈 < 入场价 < 止损\n")
	sb.WriteString("   - ⚠️ **调整止损 (update_stop_loss)**:\n")
	sb.WriteString("     - 多单: 新止损必须 < 当前价格 (否则视为止盈或立即触发)\n")
	sb.WriteString("     - 空单: 新止损必须 > 当前价格 (否则视为止盈或立即触发)\n")
	sb.WriteString("   - ⚠️ **调整止盈 (update_take_profit)**:\n")
	sb.WriteString("     - 多单: 新止盈必须 > 当前价格\n")
	sb.WriteString("     - 空单: 新止盈必须 < 当前价格\n\n")

	// 3. 输出格式 - 动态生成
	sb.WriteString("#输出格式\n\n")
	sb.WriteString("第一步: 思维链（纯文本）\n")
	sb.WriteString("简洁分析你的思考过程\n\n")
	sb.WriteString("第二步: JSON决策数组\n\n")
	sb.WriteString("⚠️ **严格格式要求**：\n")
	sb.WriteString("1. **数值字段必须是纯数字**，禁止包含算术表达式（如 `12*3`） or 文字说明。\n")
	sb.WriteString("2. **risk_usd 计算公式**：\n")
	sb.WriteString("   - 做多: `risk_usd = position_size_usd * (entry_price - stop_loss) / entry_price`\n")
	sb.WriteString("   - 做空: `risk_usd = position_size_usd * (stop_loss - entry_price) / entry_price`\n")
	sb.WriteString("   - 请在思维链中计算，JSON中只填最终结果。\n")
	sb.WriteString("3. ⚠️ **禁止直接复制示例**：以下 JSON 仅为格式结构示例，其中的币种代号（带有 DEMO_ 前缀，例如 DEMO_BTCUSDT）和数值均为演示数据。请勿在你的决策中直接输出这些模拟币种或完全照抄这些模拟数值，否则该决策将被系统作废！\n\n")

	// ⚠️ 关键修复：示例中的position_size_usd必须符合实际限制
	// 计算合理的示例值（不超过上限，且符合最小要求）
	examplePositionSizeHighPrice := accountEquity * 5.0 // 高价格币种最多10倍，这里用5倍作为示例
	if examplePositionSizeHighPrice > maxBtcEthSize {
		examplePositionSizeHighPrice = maxBtcEthSize
	}
	if examplePositionSizeHighPrice < minPositionSizeHighPrice {
		// 如果账户太小，使用最小开仓金额作为示例
		examplePositionSizeHighPrice = minPositionSizeHighPrice
	}

	examplePositionSizeAltcoin := accountEquity * 1.0 // 山寨币最多1.5倍，这里用1倍作为示例
	if examplePositionSizeAltcoin > maxAltcoinSize {
		examplePositionSizeAltcoin = maxAltcoinSize
	}
	if examplePositionSizeAltcoin < promptRules.MinPositionSize {
		// 如果账户太小，使用最小开仓金额作为示例
		examplePositionSizeAltcoin = promptRules.MinPositionSize
	}

	sb.WriteString("```json\n[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"DEMO_BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300, \"reasoning\": \"下跌趋势+MACD死叉（示例数据，禁止复制）\"},\n", btcEthLeverage, examplePositionSizeHighPrice))
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"DEMO_SOLUSDT\", \"action\": \"open_long\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 150, \"take_profit\": 165, \"confidence\": 80, \"risk_usd\": 50, \"reasoning\": \"突破阻力位（示例数据，禁止复制）\"},\n", altcoinLeverage, examplePositionSizeAltcoin))
	sb.WriteString("  {\"symbol\": \"DEMO_ETHUSDT\", \"action\": \"close_long\", \"reasoning\": \"止盈离场（示例数据，禁止复制）\"}\n")
	sb.WriteString("]\n```\n\n")
	sb.WriteString("字段说明:\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString("- `confidence`: 0-100（开仓建议≥75）\n")
	sb.WriteString("- 开仓时必填: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd, reasoning\n")
	sb.WriteString(fmt.Sprintf("- ⚠️ **重要**：示例中的position_size_usd仅供参考，实际值必须遵守上限（山寨币≤%.0f USDT，BTC/ETH≤%.0f USDT）和下限（山寨币≥%.0f USDT，BTC/ETH≥%.0f USDT）\n",
		maxAltcoinSize, maxBtcEthSize, promptRules.MinPositionSize, minPositionSizeHighPrice))
	sb.WriteString("- ⚠️ **JSON格式要求**：所有数值字段（leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd）必须是精确的单一数字，不可使用范围符号~。reasoning字段是文本描述，可以使用~符号（如\"夏普比率-0.07在-0.5~0区间\"）\n\n")

	return sb.String()
}

// buildUserPrompt 构建 User Prompt（动态数据）
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("时间: %s | 周期: #%d | 运行: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	if digest := strings.TrimSpace(ctx.OperatorDigest); digest != "" {
		sb.WriteString(digest)
		sb.WriteString("\n")
	}

	var executionErrors []string
	if summary := SummarizeRecentDecisions(ctx.RecentDecisions, ctx.CallCount); summary != "" {
		sb.WriteString(summary)
		sb.WriteString("\n")
		for _, d := range ctx.RecentDecisions {
			for _, act := range d.Decisions {
				if act.ExecutionError != "" {
					executionErrors = append(executionErrors, fmt.Sprintf("%s %s FAILED: %s", act.Symbol, act.Action, act.ExecutionError))
				}
			}
		}
	}

	// 🚨 如果存在执行错误，追加强制修正指令
	if len(executionErrors) > 0 {
		sb.WriteString("### ⚠️ CRITICAL ERROR CORRECTION REQUIRED ⚠️\n")
		sb.WriteString("You encountered EXECUTION ERRORS in previous cycles. You MUST Correct them:\n")
		for _, errMsg := range executionErrors {
			sb.WriteString(fmt.Sprintf("- %s\n", errMsg))
			if strings.Contains(errMsg, "Leverage") && strings.Contains(errMsg, "not valid") {
				sb.WriteString("  -> FIXED ACTION: You MUST LOWER the leverage (e.g., try 5x or 3x). Do NOT repeat the same leverage.\n")
			}
			if strings.Contains(errMsg, "insufficient balance") || strings.Contains(errMsg, "Margin is insufficient") {
				sb.WriteString("  -> FIXED ACTION: You MUST REDUCE position_size_usd.\n")
			}
		}
		sb.WriteString("Do NOT be stubborn. Adapt to the exchange constraints immediately.\n\n")
	}

	// 📉 注入近期交易结果 (Reflection Loop)
	// 让AI看到自己过去的“战绩” (PnL)，从而反思策略
	if ctx.Performance != nil {
		if perf, ok := ctx.Performance.(*logger.PerformanceAnalysis); ok && len(perf.RecentTrades) > 0 {
			sb.WriteString("# 📉 近期交易结果 (Reflection)\n")
			// RecentTrades 已经是倒序（最新的在前）
			count := 0
			for _, trade := range perf.RecentTrades {
				if count >= 5 { // 只显示最近5笔
					break
				}

				// 格式: ETHUSDT LONG: Entry 3000 -> Exit 3050 (+1.66%) | Time: 10:00-14:00 | Reason: ...
				pnlSign := "+"
				if trade.PnLPct < 0 {
					pnlSign = "" // 负号自带
				}

				// 简化的理由
				reason := trade.Reasoning
				if reason == "" {
					reason = "无理由"
				}

				sb.WriteString(fmt.Sprintf("- %s %s: Entry %.2f -> Exit %.2f (%s%.2f%%) | PnL: %.2f | Time: %s | Reason: %s\n",
					trade.Symbol, strings.ToUpper(trade.Side),
					trade.OpenPrice, trade.ClosePrice,
					pnlSign, trade.PnLPct,
					trade.PnL,
					trade.CloseTime.Format("15:04"),
					reason))
				count++
			}
			sb.WriteString("\n")
		}
	}

	// BTC 市场（添加整数关口计算）
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		// 0. 检查数据是否过期 (P32: Stale Data Protection)
		// 如果数据超过5分钟未更新，强制警告
		if !btcData.Timestamp.IsZero() && time.Since(btcData.Timestamp) > 5*time.Minute {
			sb.WriteString(fmt.Sprintf("\n⚠️⚠️⚠️ **严重警告：市场数据已过期！** (延迟 %s)\n", time.Since(btcData.Timestamp).Round(time.Second)))
			sb.WriteString("⚠️ **数据最后更新于**：" + btcData.Timestamp.Format("15:04:05") + "\n")
			sb.WriteString("⚠️ **必须立即执行 WAIT 决策，禁止任何交易！**\n\n")
		}

		// 1. 获取1h/4h MACD (P26: 增强数据可见性)
		macd1h := 0.0
		if btcData.HourlyContext != nil && len(btcData.HourlyContext.MACDValues) > 0 {
			macd1h = btcData.HourlyContext.MACDValues[len(btcData.HourlyContext.MACDValues)-1]
		}

		macd4h := 0.0
		if btcData.LongerTermContext != nil && len(btcData.LongerTermContext.MACDValues) > 0 {
			macd4h = btcData.LongerTermContext.MACDValues[len(btcData.LongerTermContext.MACDValues)-1]
		}

		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD(15m): %.4f | MACD(1h): %.4f | MACD(4h): %.4f | RSI(15m): %.2f\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.MACD15m, macd1h, macd4h, btcData.RSI15m))

		// 计算最近的整数关口（万位）
		price := btcData.CurrentPrice
		nearestRound := math.Round(price/10000) * 10000
		distance := math.Abs(price - nearestRound)
		distancePct := (distance / nearestRound) * 100

		if distancePct <= 0.6 {
			sb.WriteString(fmt.Sprintf("⚠️ **BTC整数关口警告：距离%.0f仅%.2f%%（≤0.6%%），高度不确定，建议wait**\n\n",
				nearestRound, distancePct))
		} else {
			sb.WriteString(fmt.Sprintf("💡 BTC整数关口：%.0f，当前距离%.2f%%（>0.6%%，处于安全区域）\n\n",
				nearestRound, distancePct))
		}
	}

	// 账户（增加最小开仓金额提示）
	// 计算最小开仓金额（基于配置的杠杆上限）
	minPositionSize := 12.0
	if ctx.PromptRules != nil && ctx.PromptRules.MinPositionSize > 0 {
		minPositionSize = ctx.PromptRules.MinPositionSize
	}

	minPositionSizeHighPrice := minPositionSize + 3.0 + float64(ctx.BTCETHLeverage)*2.0
	if minPositionSizeHighPrice > 30.0 {
		minPositionSizeHighPrice = 30.0
	}

	sb.WriteString(fmt.Sprintf("账户: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% (浮动%.2f) | 保证金%.1f%% | 持仓%d个\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.UnrealizedPnL,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 💰 显示交易手续费信息（让AI意识到交易成本）
	if ctx.TradingFeeInfo != nil {
		sb.WriteString(fmt.Sprintf("💰 交易成本: **开仓%.3f%% + 平仓%.3f%% = 来回%.3f%%** | 最小止盈距离: **>%.2f%%** (交易所: %s)\n",
			ctx.TradingFeeInfo.TakerFeeRate*100,
			ctx.TradingFeeInfo.TakerFeeRate*100,
			ctx.TradingFeeInfo.RoundTripFeeRate*100,
			ctx.TradingFeeInfo.MinProfitPct,
			ctx.TradingFeeInfo.ExchangeName))
		sb.WriteString("   ⚠️ **止盈距离必须 > 来回手续费，否则即使价格到达止盈位也是亏损！**\n")
	}

	// 🧠 LLM Simulator Identity Injection (P33: Dynamic Persona)
	pnl := ctx.Account.TotalPnLPct
	if pnl < -2.0 {
		sb.WriteString("\n🛑 **CRITICAL STATUS: DEPLOYING SNIPER RECOVERY TEAM (狙击手恢复小队)** 🛑\n")
		sb.WriteString(fmt.Sprintf("Current Drawdown: %.2f%%.\n", pnl))
		sb.WriteString("Your Persona: **The Cold-Blooded Trading Sniper** (Head of the Council).\n")
		sb.WriteString("Objective: Precision Recovery via High Quality Setups.\n")
		sb.WriteString("Rule: **STRUCTURE FIRST**. REJECT mediocre setups. ONLY take **A+ Setups** (Score >= 75 OR Perfect Structure + OB).\n")
		sb.WriteString("Sizing: For A+ Setups, use **NORMAL Sizing** (do not shrink). We need conviction to recover.\n")
		sb.WriteString("Council Check: 'Is this the PERFECT shot?' If Risk Manager or Psychologist disagrees, WAIT. (寧可錯過，絕不做平庸交易)\n\n")
	} else if pnl > 2.0 {
		sb.WriteString("\n🛡️ **STATUS: PROTECTIVE MODE (守成模式)** 🛡️\n")
		sb.WriteString(fmt.Sprintf("Current Profit: +%.2f%%.\n", pnl))
		sb.WriteString("Your Persona: **The Portfolio Risk Manager**.\n")
		sb.WriteString("Objective: Protect Gains. Do not give back profits.\n")
		sb.WriteString("Rule: Reduce Risk. Take partial profits early.\n\n")
	} else {
		sb.WriteString("\n⚖️ **STATUS: BALANCED MODE (平衡模式)** ⚖️\n")
		sb.WriteString("Current Status: BREAK-EVEN Zone.\n")
		sb.WriteString("Your Persona: **The Patient Trading Sniper**.\n")
		sb.WriteString("Objective: Find High Probability Trades authorized by the Council.\n\n")
	}

	// ⚠️ 余额不足警告
	// 计算最大可开仓名义价值 = 可用余额 * 杠杆
	// 注意：这里使用配置的杠杆上限作为参考，实际AI可能会选择更低杠杆，所以这是一个宽松的检查
	maxNotionalAltcoin := ctx.Account.AvailableBalance * float64(ctx.AltcoinLeverage)
	maxNotionalBTCETH := ctx.Account.AvailableBalance * float64(ctx.BTCETHLeverage)

	if maxNotionalAltcoin < minPositionSize {
		sb.WriteString(fmt.Sprintf("⚠️ **余额不足警告：可用余额(%.2f USDT) × 杠杆(%d) = %.2f < 最小开仓金额(%.0f USDT)，禁止开任何仓位，必须给出 wait 决策**\n",
			ctx.Account.AvailableBalance, ctx.AltcoinLeverage, maxNotionalAltcoin, minPositionSize))
	} else if maxNotionalBTCETH < minPositionSizeHighPrice {
		sb.WriteString(fmt.Sprintf("⚠️ **名義價值不足：可用余额(%.2f USDT) × 杠杆(%d) = %.2f USDT（名義價值）< BTC/ETH最小开仓要求(%.0f USDT 名義價值），只能交易山寨币（≥%.0f USDT 名義價值）**\n",
			ctx.Account.AvailableBalance, ctx.BTCETHLeverage, maxNotionalBTCETH, minPositionSizeHighPrice, minPositionSize))
		sb.WriteString(fmt.Sprintf("💡 最小开仓金额：山寨币≥%.0f USDT | BTC/ETH≥%.0f USDT（根据选择的杠杆倍数动态调整）\n",
			minPositionSize, minPositionSizeHighPrice))
	}

	// 💡 提示最大开仓金额（辅助AI计算，避免算错）
	// 计算基于账户净值的最大持仓限制
	var maxPosSizeAltcoin, maxPosSizeBTCETH float64
	if ctx.PromptRules != nil {
		maxPosSizeAltcoin = ctx.Account.TotalEquity * ctx.PromptRules.MaxPositionSizeAltcoin
		maxPosSizeBTCETH = ctx.Account.TotalEquity * ctx.PromptRules.MaxPositionSizeBTCETH
	} else {
		// 如果规则未加载，使用默认保守值 (1.5x 和 8x)
		maxPosSizeAltcoin = ctx.Account.TotalEquity * 1.5
		maxPosSizeBTCETH = ctx.Account.TotalEquity * 8.0
	}
	// 💰 自適應上限對齊：上限不得小於開倉下限，防止 AI 混亂
	if maxPosSizeAltcoin < minPositionSize {
		maxPosSizeAltcoin = minPositionSize
	}
	if maxPosSizeBTCETH < minPositionSizeHighPrice {
		maxPosSizeBTCETH = minPositionSizeHighPrice
	}
	sb.WriteString(fmt.Sprintf("💡 最大开仓金额（名義價值上限）：山寨币 ≤%.0f USDT | BTC/ETH ≤%.0f USDT\n",
		maxPosSizeAltcoin, maxPosSizeBTCETH))

	// ⏱️ 交易状态（仅供AI参考，不强制限制）
	if ctx.TradeHistory != nil {
		// 显示当前交易状态（帮助AI了解情况）
		sb.WriteString(fmt.Sprintf("⏱️ 交易状态：过去1小时交易%d次 | 连续亏损: %d次 | 连续wait: %d次\n",
			ctx.TradeHistory.TradesInHour, ctx.TradeHistory.ConsecutiveLoss, ctx.TradeHistory.ConsecutiveWait))

		// 交易频率提醒（信息提示，非强制禁令）
		if ctx.TradeHistory.TradesInHour >= 3 {
			sb.WriteString("💡 交易频率提醒：过去1小时已交易3+次，请注意手续费成本累积\n")
		}

	}

	sb.WriteString("\n")

	// 持仓（完整市场数据）
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 当前持仓\n\n")
		// ⚠️ 增加全局禁止开仓警告
		sb.WriteString("⚠️ **重要提醒：以下币种已有持仓，禁止发送 `open_long` 或 `open_short`！只能选择 `hold`/`update_stop_loss`/`update_take_profit`/`partial_close`/`close_long`/`close_short`**\n\n")
		for i, pos := range ctx.Positions {
			// Token Guardrail: Check length before adding more positions
			if sb.Len() > 80000 {
				sb.WriteString("\n⚠️ **(User Prompt Truncated: Too many positions. Please manage current portfolio first)** ...\n")
				break
			}

			// 计算持仓时长
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // 转换为分钟
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | 持仓时长%d分钟", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf(" | 持仓时长%d小时%d分钟", durationHour, durationMinRemainder)
				}
			}

			// 根据持仓方向输出禁止的动作
			forbiddenAction := "open_long"
			allowedClose := "close_long"
			if pos.Side == "short" {
				forbiddenAction = "open_short"
				allowedClose = "close_short"
			}

			sb.WriteString(fmt.Sprintf("%d. **%s %s** 🚫禁止%s | 入场价%.4f 当前价%.4f | 盈亏%+.2f%% | 杠杆%dx | 保证金%.0f | 强平价%.4f%s\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side), forbiddenAction,
				pos.EntryPrice, pos.MarkPrice, pos.UnrealizedPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

			// 🧠 Trade Memory Injection
			if pos.EntryReasoning != "" {
				sb.WriteString(fmt.Sprintf("   🧠 **开仓理由 (Memory)**: %s\n", pos.EntryReasoning))
			}

			// ⚠️ 关键修复：输出当前止损止盈，让AI知道当前保护水平
			// 这对于 update_stop_loss 决策至关重要（AI 需要知道当前止损才能判断是收紧还是调宽）
			if pos.StopLoss > 0 || pos.TakeProfit > 0 {
				slDisplay := "未设置"
				tpDisplay := "未设置"
				if pos.StopLoss > 0 {
					slDisplay = fmt.Sprintf("%.4f", pos.StopLoss)
				}
				if pos.TakeProfit > 0 {
					tpDisplay = fmt.Sprintf("%.4f", pos.TakeProfit)
				}
				sb.WriteString(fmt.Sprintf("   📍 当前止损: %s | 当前止盈: %s\n", slDisplay, tpDisplay))
			} else {
				sb.WriteString("   ⚠️ 当前止损: 未设置 | 当前止盈: 未设置 (危险!)\n")
			}

			sb.WriteString(fmt.Sprintf("   → 可用动作: `hold` | `update_stop_loss` | `update_take_profit` | `partial_close` | `%s`\n\n", allowedClose))

			// 使用FormatMarketData输出完整市场数据
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(market.Format(marketData))
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString("当前持仓: 无\n\n")
	}

	// 候选币种（完整市场数据）
	sb.WriteString(fmt.Sprintf("## 候选币种 (%d个)\n\n", len(ctx.MarketDataMap)))
	sb.WriteString("⚠️ **重要**：第6步多空确认清单评估时，必须针对至少1-2个币种填写完整的8项清单表格，即使最终选择wait也要说明原因。\n\n")

	// Token Guardrail: If positions already took too much space, don't even start candidates
	if sb.Len() > 100000 {
		sb.WriteString("\n⚠️ **(Candidate Coins Omitted for Token Safety due to large portfolio)** ...\n")
		return sb.String()
	}

	// 候选币种详情
	sb.WriteString("## 候选币种\n\n")

	// ⚠️ 候选币种 Stale Data Check (Phase 23)
	// 遍历所有候选币种，确保数据新鲜
	hasStaleData := false
	for _, coin := range ctx.CandidateCoins {
		if data, ok := ctx.MarketDataMap[coin.Symbol]; ok {
			if !data.Timestamp.IsZero() && time.Since(data.Timestamp) > 5*time.Minute {
				hasStaleData = true
				sb.WriteString(fmt.Sprintf("⚠️⚠️⚠️ **严重警告：%s 数据已过期！** (延迟 %s)\n",
					coin.Symbol, time.Since(data.Timestamp).Round(time.Second)))
			}
		}
	}
	if hasStaleData {
		sb.WriteString("⚠️ **发现过期数据：必须对受影响币种执行 WAIT 决策！**\n\n")
	}

	// Pre-calculate Global Candle Progress (Optimization)
	now := time.Now()
	currentMinute := now.Minute()
	candleProgress := float64(currentMinute) / 60.0 * 100.0
	timeUntilClose := 60 - currentMinute

	// Create set of position symbols to avoid double printing
	positionSymbolSet := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbolSet[pos.Symbol] = true
	}

	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		// Skip if already in positions (Duplicate check)
		if positionSymbolSet[coin.Symbol] {
			continue
		}

		displayedCount++

		// Token Guardrail: Check length before adding more
		if sb.Len() > 100000 {
			sb.WriteString("\n⚠️ **(User Prompt Truncated for Token Safety)** ...\n")
			break
		}

		sourceTags := ""
		if len(coin.Sources) > 1 {
			sourceTags = " (AI500+OI_Top双重信号)"
		} else if len(coin.Sources) == 1 && coin.Sources[0] == "oi_top" {
			sourceTags = " (OI_Top持仓增长)"
		}

		// 使用FormatMarketData输出完整市场数据
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(market.Format(marketData))

		// 使用全局计算的进度
		sb.WriteString(fmt.Sprintf("K-Line Progress (1H): %.1f%% completed | Time Until Close: %d min\n", candleProgress, timeUntilClose))

		// 添加OI Top数据（如果存在）
		if oiTopData, hasOITop := ctx.OITopDataMap[coin.Symbol]; hasOITop {
			sb.WriteString("📊 OI Top 持仓量增长数据:\n\n")
			sb.WriteString(fmt.Sprintf("排名: #%d\n", oiTopData.Rank))
			sb.WriteString(fmt.Sprintf("持仓量变化: %+.2f%% (价值: %+.2f USDT)\n",
				oiTopData.OIDeltaPercent, oiTopData.OIDeltaValue))
			sb.WriteString(fmt.Sprintf("价格变化: %+.2f%%\n", oiTopData.PriceDeltaPercent))
			if oiTopData.NetLong > 0 || oiTopData.NetShort > 0 {
				sb.WriteString(fmt.Sprintf("净多仓: %.2f | 净空仓: %.2f\n",
					oiTopData.NetLong, oiTopData.NetShort))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// 历史表现分析（完整数据，帮助AI学习）
	if ctx.Performance != nil {
		type PerformanceData struct {
			TotalTrades   int     `json:"total_trades"`
			WinningTrades int     `json:"winning_trades"`
			LosingTrades  int     `json:"losing_trades"`
			WinRate       float64 `json:"win_rate"`
			// Note: AvgWin/AvgLoss removed if they bloat
			ProfitFactor float64                           `json:"profit_factor"`
			SharpeRatio  float64                           `json:"sharpe_ratio"`
			RecentTrades []map[string]interface{}          `json:"recent_trades"`
			SymbolStats  map[string]map[string]interface{} `json:"symbol_stats"`
			BestSymbol   string                            `json:"best_symbol"`
			WorstSymbol  string                            `json:"worst_symbol"`
		}
		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				sb.WriteString("## 📊 历史表现分析（学习参考）\n\n")

				if perfData.TotalTrades > 0 {
					sb.WriteString("**总体表现**：\n")
					sb.WriteString(fmt.Sprintf("- 总交易数: %d 笔\n", perfData.TotalTrades))
					sb.WriteString(fmt.Sprintf("- 胜率: %.1f%% (%d 胜 / %d 负)\n", perfData.WinRate, perfData.WinningTrades, perfData.LosingTrades))
					if perfData.ProfitFactor > 0 {
						sb.WriteString(fmt.Sprintf("- 盈亏比: %.2f\n", perfData.ProfitFactor))
					}
					sb.WriteString(fmt.Sprintf("- 夏普比率 (Sharpe): **%.2f**\n", perfData.SharpeRatio))
					sb.WriteString("\n")

					// Token Guardrail for SymbolStats
					if sb.Len() > 120000 {
						sb.WriteString("💡 (Symbol statistics omitted due to prompt size)\n\n")
					} else if len(perfData.SymbolStats) > 0 {
						sb.WriteString("**各币种表现**：\n")
						for symbol, stats := range perfData.SymbolStats {
							if totalTrades, ok := stats["total_trades"].(float64); ok && totalTrades > 0 {
								winRate, _ := stats["win_rate"].(float64)
								totalPnL, _ := stats["total_pn_l"].(float64)
								sb.WriteString(fmt.Sprintf("- %s: %d 笔交易, 胜率 %.1f%%, 总盈亏 %.2f USDT\n",
									symbol, int(totalTrades), winRate, totalPnL))
							}
						}
						if perfData.BestSymbol != "" {
							sb.WriteString(fmt.Sprintf("✅ 表现最好: %s\n", perfData.BestSymbol))
						}
						if perfData.WorstSymbol != "" {
							sb.WriteString(fmt.Sprintf("❌ 表现最差: %s\n", perfData.WorstSymbol))
						}
						sb.WriteString("\n")
					}

					// 最近交易记录（最多5笔）
					if len(perfData.RecentTrades) > 0 {
						sb.WriteString("**最近交易记录**（学习参考）：\n")
						displayCount := 5
						if len(perfData.RecentTrades) < displayCount {
							displayCount = len(perfData.RecentTrades)
						}
						for i := 0; i < displayCount; i++ {
							trade := perfData.RecentTrades[i]
							symbol, _ := trade["symbol"].(string)
							side, _ := trade["side"].(string)
							openPrice, _ := trade["open_price"].(float64)
							closePrice, _ := trade["close_price"].(float64)
							pnl, _ := trade["pn_l"].(float64) // 注意：JSON字段是pn_l，不是pnl
							pnlPct, _ := trade["pn_l_pct"].(float64)
							duration, _ := trade["duration"].(string)

							reasoning, _ := trade["reasoning"].(string) // 获取理由

							result := "✅"
							if pnl < 0 {
								result = "❌"
							}

							// 构建显示字符串
							tradeInfo := fmt.Sprintf("%s %s %s: 开仓 %.4f → 平仓 %.4f, 盈亏 %.2f USDT (%.2f%%), 持仓 %s",
								result, symbol, strings.ToUpper(side), openPrice, closePrice, pnl, pnlPct, duration)

							// 如果有理由，添加到末尾
							if reasoning != "" {
								tradeInfo += fmt.Sprintf(" (理由: %s)", reasoning)
							}

							sb.WriteString(tradeInfo + "\n")
						}
						sb.WriteString("\n")
					}
				} else {
					sb.WriteString("**暂无历史交易记录**（新账户或最近无交易）\n\n")
				}
			}
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("现在请分析并输出决策（思维链 + JSON）\n")

	return sb.String()
}

// parseRiskRewardRatio 解析风险回报比字符串 (e.g. "1:3" -> 3.0, "2:1" -> 2.0)
// 支持 X:Y 格式，如果 X < Y (如 1:3), 返回 Y/X (3.0)
// 如果 X > Y (如 2:1), 返回 X/Y (2.0)
// 这样可以同时兼容 "Risk:Reward" (1:3) 和 "Reward:Risk" (2:1) 的写法
func parseRiskRewardRatio(rrStr string) float64 {
	parts := strings.Split(rrStr, ":")
	if len(parts) != 2 {
		// 尝试直接解析数字
		if val, err := strconv.ParseFloat(rrStr, 64); err == nil {
			return val
		}
		return 2.0 // 解析失败，返回默认值
	}

	val1, err1 := strconv.ParseFloat(parts[0], 64)
	val2, err2 := strconv.ParseFloat(parts[1], 64)

	if err1 != nil || err2 != nil || val1 == 0 || val2 == 0 {
		return 2.0
	}

	// 智能推断：通常 R:R 要求 > 1.0 (賺的多於冒的風險)
	ratio1 := val1 / val2
	ratio2 := val2 / val1

	if ratio1 > ratio2 {
		return ratio1
	}
	return ratio2
}

// truncateString 截取字符串（处理多字节字符）
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
