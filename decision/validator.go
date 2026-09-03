package decision

import (
	"fmt"
	"log"
	"math"
	"aetheris/market"
	"strings"
)

// PositionInfo 簡化的持倉信息接口，避免循環依賴（如果都在 decision 包則直接使用結構體）
// 假設 PositionInfo 在 engine.go 中定義，屬於同一個包

func isOpenAction(action string) bool {
	return action == "open_long" || action == "open_short"
}

func isCloseAction(action string) bool {
	return action == "close_long" || action == "close_short" || action == "partial_close"
}

func lastOpenIndex(decisions []Decision) int {
	for i := len(decisions) - 1; i >= 0; i-- {
		if isOpenAction(decisions[i].Action) {
			return i
		}
	}
	return -1
}

// SanitizeDecisions 公开入口：丢掉未通过验证的开仓，保留可执行的平仓/等待。
func SanitizeDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, positions []PositionInfo, marketDataMap map[string]*market.Data, minRiskRewardRatio float64, maxPositions int, feeInfo *TradingFeeInfo) []Decision {
	return sanitizeDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, positions, marketDataMap, minRiskRewardRatio, maxPositions, feeInfo)
}

// sanitizeDecisions 丢掉未通过验证的开仓，保留可执行的平仓/等待。
// 一个坏的 open 不再让整批（含 close）被跳过。
func sanitizeDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, positions []PositionInfo, marketDataMap map[string]*market.Data, minRiskRewardRatio float64, maxPositions int, feeInfo *TradingFeeInfo) []Decision {
	closing := make(map[string]bool)
	for _, d := range decisions {
		if isCloseAction(d.Action) {
			closing[d.Symbol] = true
		}
	}

	kept := make([]Decision, 0, len(decisions))
	for _, d := range decisions {
		if isOpenAction(d.Action) && closing[d.Symbol] {
			log.Printf("⚠️ 丢弃冲突开仓 %s %s（同轮已有平仓）", d.Symbol, d.Action)
			continue
		}
		kept = append(kept, d)
	}

	for i := 0; i < 32; i++ {
		if err := validateCrossDecisions(kept, accountEquity, maxPositions, positions); err != nil {
			idx := lastOpenIndex(kept)
			if idx < 0 {
				log.Printf("⚠️ 交叉验证失败且无开仓可丢弃: %v", err)
				break
			}
			log.Printf("⚠️ 交叉验证失败，丢弃开仓 %s: %v", kept[idx].Symbol, err)
			kept = append(kept[:idx], kept[idx+1:]...)
			continue
		}
		break
	}

	result := make([]Decision, 0, len(kept))
	for i := range kept {
		d := kept[i]
		if err := validateDecision(&d, accountEquity, btcEthLeverage, altcoinLeverage, positions, marketDataMap, minRiskRewardRatio, feeInfo); err != nil {
			log.Printf("⚠️ 丢弃未通过验证的决策 %s %s: %v", d.Symbol, d.Action, err)
			continue
		}
		result = append(result, d)
	}
	return result
}

// validateDecisions 验证所有决策（需要账户信息、杠杆配置、当前持仓和市场数据）
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, positions []PositionInfo, marketDataMap map[string]*market.Data, minRiskRewardRatio float64, maxPositions int, feeInfo *TradingFeeInfo) error {
	// ⚠️ 关键修复：跨决策交叉验证（在单独验证之前）
	if err := validateCrossDecisions(decisions, accountEquity, maxPositions, positions); err != nil {
		return err
	}

	for i := range decisions {
		// ⚠️ 关键修复：使用索引访问而非 range 副本，确保自动修正（如止损调整）能写回原 slice
		if err := validateDecision(&decisions[i], accountEquity, btcEthLeverage, altcoinLeverage, positions, marketDataMap, minRiskRewardRatio, feeInfo); err != nil {
			return fmt.Errorf("决策 #%d 验证失败: %w", i+1, err)
		}
	}
	return nil
}

// validateCrossDecisions 跨决策交叉验证
// 检测：1.同币种冲突 2.总风险累积 3.总开仓金额是否超过可用保证金 4.最大持仓数量限制
func validateCrossDecisions(decisions []Decision, accountEquity float64, maxPositions int, currentPositions []PositionInfo) error {
	symbolActions := make(map[string][]string) // symbol -> actions
	totalRiskUSD := 0.0
	totalOpenPositionUSD := 0.0

	for _, d := range decisions {
		// 收集每个币种的动作
		symbolActions[d.Symbol] = append(symbolActions[d.Symbol], d.Action)

		// 累计风险和开仓金额
		if d.Action == "open_long" || d.Action == "open_short" {
			totalRiskUSD += d.RiskUSD
			totalOpenPositionUSD += d.PositionSizeUSD
		}
	}

	// 检查0：最大持仓数量限制 (MaxPositions)
	// 逻辑：计算"交易后"的预计持仓数量
	// 1. 从当前持仓开始
	projectedHeldSymbols := make(map[string]bool)
	for _, p := range currentPositions {
		projectedHeldSymbols[p.Symbol] = true
	}

	// 2. 先处理平仓 (释放还在占用的名额)
	for _, d := range decisions {
		if d.Action == "close_long" || d.Action == "close_short" {
			delete(projectedHeldSymbols, d.Symbol)
		}
	}

	// 3. 再处理开仓 (占用名额)
	for _, d := range decisions {
		if d.Action == "open_long" || d.Action == "open_short" {
			projectedHeldSymbols[d.Symbol] = true
		}
	}

	if len(projectedHeldSymbols) > maxPositions {
		return fmt.Errorf("超过最大持仓数限制: 交易后预计持仓%d个 > 上限%d个 (请确保在开新仓前平掉旧仓位)",
			len(projectedHeldSymbols), maxPositions)
	}

	// 检查1：同币种冲突动作
	for symbol, actions := range symbolActions {
		hasOpen := false
		hasClose := false
		for _, action := range actions {
			if action == "open_long" || action == "open_short" {
				hasOpen = true
			}
			if action == "close_long" || action == "close_short" {
				hasClose = true
			}
		}
		if hasOpen && hasClose {
			log.Printf("🚨 检测到同币种冲突: %s 同时有开仓和平仓动作", symbol)
			return fmt.Errorf("同币种冲突: %s 不能同时开仓和平仓，请AI修正决策", symbol)
		}
	}

	// 检查1.5：投资组合相关性检查 (Portfolio Correlation - Max 2 Correlated Positions)
	// 逻辑：计算预计持仓的多空分布
	projectedLongs := 0
	projectedShorts := 0

	// 1. 统计现有持仓 (排除即将平仓的)
	for _, p := range currentPositions {
		willBeClosed := false
		for _, d := range decisions {
			if d.Symbol == p.Symbol && (d.Action == "close_long" || d.Action == "close_short") {
				willBeClosed = true
				break
			}
		}
		if !willBeClosed {
			switch p.Side {
			case "long":
				projectedLongs++
			case "short":
				projectedShorts++
			}
		}
	}

	// 2. 统计新开仓
	for _, d := range decisions {
		switch d.Action {
		case "open_long":
			projectedLongs++
		case "open_short":
			projectedShorts++
		}
	}

	// 3. 验证硬限制 (Max 2 Correlated)
	const maxCorrelatedLimit = 2
	if projectedLongs > maxCorrelatedLimit {
		return fmt.Errorf("同向倉位數量限制: 預計持有 %d 個多單(LONG) > 上限 %d 個。"+
			"規則: 最多同時持有 2 個 LONG 或 2 個 SHORT。"+
			"解決方案: (1) 先 close_long 平掉一個現有多單 (2) 或改開對沖空單 open_short。"+
			"注意: 這不是價格相關性檢查，而是單純的倉位數量計數。",
			projectedLongs, maxCorrelatedLimit)
	}
	if projectedShorts > maxCorrelatedLimit {
		return fmt.Errorf("同向倉位數量限制: 預計持有 %d 個空單(SHORT) > 上限 %d 個。"+
			"規則: 最多同時持有 2 個 LONG 或 2 個 SHORT。"+
			"解決方案: (1) 先 close_short 平掉一個現有空單 (2) 或改開對沖多單 open_long。"+
			"注意: 這不是價格相關性檢查，而是單純的倉位數量計數。",
			projectedShorts, maxCorrelatedLimit)
	}

	// 检查2：总风险累积不能超过账户净值的 5%（多笔同时开仓时）
	maxTotalRiskPct := 5.0
	if totalRiskUSD > 0 {
		totalRiskPct := (totalRiskUSD / accountEquity) * 100
		if totalRiskPct > maxTotalRiskPct {
			log.Printf("🚨 总风险过高: %.2f%% > %.1f%% (总风险 $%.2f, 净值 $%.2f)",
				totalRiskPct, maxTotalRiskPct, totalRiskUSD, accountEquity)
			return fmt.Errorf("总风险过高: %.2f%% 超过 %.1f%% 上限，请减少开仓数量或降低单笔风险",
				totalRiskPct, maxTotalRiskPct)
		}
	}

	// 检查3：总保证金占用检查 (P29: 防止 Insufficient Margin)
	totalMarginRequired := 0.0
	for _, d := range decisions {
		if (d.Action == "open_long" || d.Action == "open_short") && d.Leverage > 0 {
			totalMarginRequired += d.PositionSizeUSD / float64(d.Leverage)
		}
	}

	// 预留 2% 缓冲，防止手续费或滑点导致余额不足
	// 使用 accountEquity 作 AvailableBalance 的近似值（假设无其他持仓占用）
	availableMargin := accountEquity * 0.98

	if totalMarginRequired > availableMargin {
		return fmt.Errorf("保证金不足: 需要 $%.2f, 可用(估算) $%.2f (杠杆過低或倉位過大)",
			totalMarginRequired, availableMargin)
	}

	if totalOpenPositionUSD > accountEquity*3 {
		log.Printf("⚠️ 警告：总开仓金额较大 $%.2f > 账户净值3倍 ($%.2f×3)",
			totalOpenPositionUSD, accountEquity)
	}

	return nil
}

// validateDecision 验证单个决策的有效性
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, positions []PositionInfo, marketDataMap map[string]*market.Data, minRiskRewardRatio float64, feeInfo *TradingFeeInfo) error {
	// 验证action
	if err := validateAction(d); err != nil {
		return err
	}

	// ⚠️ Phantom Close Prevention: 檢查平倉或調整動作是否有對應持倉
	if d.Action == "close_long" || d.Action == "close_short" || d.Action == "partial_close" || d.Action == "update_stop_loss" || d.Action == "update_take_profit" {
		if !hasMatchingPosition(d.Symbol, d.Action, positions) {
			log.Printf("⚠️  [Phantom Close Prevention] 幣種 %s 動作 %s 但無對應持倉。自動修正為 wait。", d.Symbol, d.Action)
			d.Action = "wait"
			d.Reasoning = fmt.Sprintf("[Phantom Close Prevention] 試圖平倉或調整不存在的部位 (Action: %s)，已自動修正為 wait", d.Action)
			return nil
		}
	}

	// ⚠️ Phantom Trade Prevention: 检查市场数据是否存在（仅针对开仓）
	// 如果没有数据，绝对不允许开仓，否则会导致入场价为0或错误的决策
	if d.Action == "open_long" || d.Action == "open_short" {
		if _, ok := marketDataMap[d.Symbol]; !ok {
			return fmt.Errorf("严重错误: 试图交易无数据的币种 %s (Phantom Trade Prevention) - 请检查数据流是否正常", d.Symbol)
		}
	}

	// 验证开仓字段（仅对 open_long/open_short）
	if d.Action == "open_long" || d.Action == "open_short" {
		if err := validateOpenFields(d, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
			return err
		}
		// 关键修复：验证入场价与市场价的合理性
		if err := validateEntryPrice(d, marketDataMap); err != nil {
			return err
		}
	}

	// 动态调整止损验证
	if d.Action == "update_stop_loss" {
		if d.NewStopLoss <= 0 && d.StopLoss <= 0 {
			return fmt.Errorf("调整止损时必须提供新止损价格")
		}
	}

	// 动态调整止盈验证
	if d.Action == "update_take_profit" {
		if d.NewTakeProfit <= 0 && d.TakeProfit <= 0 {
			return fmt.Errorf("调整止盈时必须提供新止盈价格")
		}
	}

	// 部分平仓验证
	if d.Action == "partial_close" {
		if d.ClosePercentage <= 0 || d.ClosePercentage > 100 {
			return fmt.Errorf("平仓百分比必须在0-100之间: %.1f", d.ClosePercentage)
		}
		// ⚠️ 关键修复：验证部分平仓金额是否满足最小名义价值 (Min Notional ~$10)
		if err := validatePartialCloseSize(d, positions, marketDataMap); err != nil {
			return err
		}
	}

	// 验证止损距离（防止止损过近被波动扫掉）- 适用于 Open 和 Update
	if d.Action == "open_long" || d.Action == "open_short" || d.Action == "update_stop_loss" {
		if err := validateStopLossDistance(d, positions, marketDataMap, minRiskRewardRatio, feeInfo); err != nil {
			return err
		}
	}

	// 验证风险回报比 (仅对开仓) - 扣除手续费后的净风险回报比
	if d.Action == "open_long" || d.Action == "open_short" {
		if err := validateRiskRewardRatio(d, minRiskRewardRatio, feeInfo); err != nil {
			return err
		}
	}

	// 验证单笔最大亏损 (仅对开仓)
	if d.Action == "open_long" || d.Action == "open_short" {
		if err := validateMaxLoss(d, accountEquity); err != nil {
			return err
		}
	}

	return nil
}

// validateAction 验证action是否有效
func validateAction(d *Decision) error {
	validActions := map[string]bool{
		"open_long":          true,
		"open_short":         true,
		"close_long":         true,
		"close_short":        true,
		"update_stop_loss":   true,
		"update_take_profit": true,
		"partial_close":      true,
		"hold":               true,
		"wait":               true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("无效的action: '%s' (完整决策: %+v)", d.Action, *d)
	}
	return nil
}

// validateOpenFields 验证开仓决策的所有参数
func validateOpenFields(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	// 高价格币种列表（需要特殊杠杆和仓位限制）
	highPriceCoins := map[string]bool{
		"BTCUSDT": true,
		"ETHUSDT": true,
		// ASTERUSDT价格较低（约$1.11），使用普通山寨币规则
	}

	// 💰 自适应最小开仓金额定义
	minPositionSizeGeneral := 12.0
	var minPositionSizeHighPrice float64 = 15.0 + float64(btcEthLeverage)*2.0
	if d.Leverage > 0 {
		minPositionSizeHighPrice = 15.0 + float64(d.Leverage)*2.0
	}
	if minPositionSizeHighPrice > 30.0 {
		minPositionSizeHighPrice = 30.0
	}

	// 根据币种使用配置的杠杆上限
	maxLeverage := altcoinLeverage          // 山寨币使用配置的杠杆
	maxPositionValue := accountEquity * 1.5 // 山寨币最多1.5倍账户净值（与Prompt一致）
	if maxPositionValue < minPositionSizeGeneral {
		maxPositionValue = minPositionSizeGeneral // 💰 自适应对齐：上限不得小于下限，防止低净值下空集死锁
	}

	// ⚠️ 硬性安全上限 (Hard Safety Caps)
	const HardCapAlt = 10
	const HardCapBTC = 20

	if highPriceCoins[d.Symbol] {
		maxLeverage = btcEthLeverage // 高价格币种使用BTC/ETH of 杠杆配置
		if maxLeverage > HardCapBTC {
			maxLeverage = HardCapBTC // 强制限制在20x以内
		}
		maxPositionValue = accountEquity * 8 // 高价格币种最多8倍账户净值
		if maxPositionValue < minPositionSizeHighPrice {
			maxPositionValue = minPositionSizeHighPrice // 💰 自适应对齐：上限不得小于下限，防止低净值下空集死锁
		}
	} else {
		if maxLeverage > HardCapAlt {
			maxLeverage = HardCapAlt // 强制限制在10x以内
		}
	}

	// 验证杠杆
	if d.Leverage <= 0 || d.Leverage > maxLeverage {
		return fmt.Errorf("杠杆过高(x%d): 必须在1-%d之间 (硬性安全限制: 山寨币≤%dx, BTC/ETH≤%dx)",
			d.Leverage, maxLeverage, HardCapAlt, HardCapBTC)
	}

	// 验证仓位大小
	if d.PositionSizeUSD <= 0 {
		return fmt.Errorf("仓位大小必须大于0: %.2f", d.PositionSizeUSD)
	}

	// ⚠️ 关键修复：强制要求止损止盈（防止无限亏损）
	if d.StopLoss <= 0 {
		return fmt.Errorf("开仓决策必须设置止损（stop_loss > 0），当前值: %.2f", d.StopLoss)
	}
	if d.TakeProfit <= 0 {
		return fmt.Errorf("开仓决策必须设置止盈（take_profit > 0），当前值: %.2f", d.TakeProfit)
	}

	// ⚠️ 新增：强制要求入场价（确保风险计算准确）
	if d.EntryPrice <= 0 {
		return fmt.Errorf("开仓决策必须提供入场价（entry_price > 0），当前值: %.2f", d.EntryPrice)
	}

	// 验证最小开仓金额
	if err := validatePositionSize(d, btcEthLeverage); err != nil {
		return err
	}

	// 验证仓位价值上限
	if err := validatePositionValueLimit(d, maxPositionValue); err != nil {
		return err
	}

	// 验证止损止盈合理性
	if err := validateStopLossTakeProfit(d); err != nil {
		return err
	}

	return nil
}

// validatePositionSize 验证仓位大小（最小开仓金额）
func validatePositionSize(d *Decision, btcEthLeverage int) error {
	const minPositionSizeGeneral = 12.0 // 10 + 20% 安全边际

	highPriceCoins := map[string]bool{
		"BTCUSDT": true,
		"ETHUSDT": true,
	}

	isHighPriceCoin := highPriceCoins[d.Symbol]

	var minPositionSizeHighPrice float64
	if isHighPriceCoin {
		decisionLeverage := d.Leverage
		if decisionLeverage < 1 {
			decisionLeverage = btcEthLeverage
		}
		minPositionSizeHighPrice = 15.0 + float64(decisionLeverage)*2.0
		if minPositionSizeHighPrice > 30.0 {
			minPositionSizeHighPrice = 30.0
		}
	}

	if isHighPriceCoin {
		if d.PositionSizeUSD < minPositionSizeHighPrice {
			return fmt.Errorf("开仓金额过小(%.2f)，BTC/ETH类必须≥%.2f USDT (当前杠杆%dx)", d.PositionSizeUSD, minPositionSizeHighPrice, d.Leverage)
		}
	} else {
		if d.PositionSizeUSD < minPositionSizeGeneral {
			return fmt.Errorf("开仓金额过小(%.2f)，山寨币必须≥%.2f USDT", d.PositionSizeUSD, minPositionSizeGeneral)
		}
	}

	return nil
}

// validatePositionValueLimit 验证仓位价值上限
func validatePositionValueLimit(d *Decision, maxPositionValue float64) error {
	maxPositionValueRounded := math.Round(maxPositionValue)
	tolerance := 0.1

	if d.PositionSizeUSD > maxPositionValueRounded+tolerance {
		originalSize := d.PositionSizeUSD
		d.PositionSizeUSD = maxPositionValueRounded
		d.Reasoning += fmt.Sprintf(" [系统自动修正: 仓位 %.2f > 上限 %.0f, 已截断]", originalSize, maxPositionValueRounded)
	}
	return nil
}

// validateEntryPrice 关键修复：验证入场价与市场价的合理性
// 防止AI输出与市场价差异巨大的入场价（如Cycle19事件：ADA入场价30.21 vs 实际0.43）
func validateEntryPrice(d *Decision, marketDataMap map[string]*market.Data) error {
	if d.Action != "open_long" && d.Action != "open_short" {
		return nil
	}

	// 如果没有市场数据，记录警告但不阻止（允许降级）
	if marketDataMap == nil {
		log.Printf("⚠️ 无法验证 %s 入场价：marketDataMap 为空", d.Symbol)
		return nil
	}

	marketData, exists := marketDataMap[d.Symbol]
	if !exists {
		log.Printf("⚠️ 无法验证 %s 入场价：缺少市场数据", d.Symbol)
		return nil
	}

	marketPrice := marketData.CurrentPrice
	if marketPrice <= 0 {
		log.Printf("⚠️ 无法验证 %s 入场价：市场价格为0", d.Symbol)
		return nil
	}

	// 计算偏离百分比
	deviationPct := math.Abs(d.EntryPrice-marketPrice) / marketPrice * 100

	// 阈值：5% 偏离视为异常（考虑AI分析延迟3-5分钟和市场波动）
	const maxDeviation = 5.0
	if deviationPct > maxDeviation {
		log.Printf("🚨 入场价异常！%s AI提供=%.4f 市场价=%.4f 偏离=%.2f%% > %.1f%%",
			d.Symbol, d.EntryPrice, marketPrice, deviationPct, maxDeviation)
		return fmt.Errorf("⚠️ 入场价异常偏离市场价！AI提供: %.4f, 市场价: %.4f, 偏离: %.2f%% (超过%.1f%%阈值)，请检查AI决策",
			d.EntryPrice, marketPrice, deviationPct, maxDeviation)
	}

	// 偏离在3-5%之间，记录警告但允许通过
	if deviationPct > 3.0 {
		log.Printf("⚠️ 入场价偏离较大: %s AI=%.4f 市场=%.4f 偏离=%.2f%%",
			d.Symbol, d.EntryPrice, marketPrice, deviationPct)
	}

	return nil
}

// validateStopLossTakeProfit 验证止损止盈的合理性
func validateStopLossTakeProfit(d *Decision) error {
	if d.Action != "open_long" && d.Action != "open_short" {
		return nil
	}

	if d.StopLoss <= 0 || d.TakeProfit <= 0 {
		return fmt.Errorf("止损和止盈必须大于0")
	}

	if d.Action == "open_long" {
		if d.StopLoss >= d.TakeProfit {
			return fmt.Errorf("做多时止损价必须小于止盈价")
		}
		if d.EntryPrice > 0 {
			if d.StopLoss >= d.EntryPrice {
				return fmt.Errorf("做多时止损价(%.4f)必须低于入场价(%.4f)", d.StopLoss, d.EntryPrice)
			}
			if d.TakeProfit <= d.EntryPrice {
				return fmt.Errorf("做多时止盈价(%.4f)必须高于入场价(%.4f)", d.TakeProfit, d.EntryPrice)
			}
		}
	} else {
		if d.StopLoss <= d.TakeProfit {
			return fmt.Errorf("做空时止损价必须大于止盈价")
		}
		if d.EntryPrice > 0 {
			if d.StopLoss <= d.EntryPrice {
				return fmt.Errorf("做空时止损价(%.4f)必须高于入场价(%.4f)", d.StopLoss, d.EntryPrice)
			}
			if d.TakeProfit >= d.EntryPrice {
				return fmt.Errorf("做空时止盈价(%.4f)必须低于入场价(%.4f)", d.TakeProfit, d.EntryPrice)
			}
		}
	}

	return nil
}

// validateStopLossDistance 验证止损距离（防止止损过近被波动扫掉）
func validateStopLossDistance(d *Decision, positions []PositionInfo, marketDataMap map[string]*market.Data, minRiskRewardRatio float64, feeInfo *TradingFeeInfo) error {
	// 使用AI提供的入场价作为基准
	entryPrice := d.EntryPrice

	// 确定要验证的止损价格（对于 update_stop_loss，优先使用 NewStopLoss）
	stopLossToValidate := d.StopLoss
	if d.Action == "update_stop_loss" {
		if d.NewStopLoss > 0 {
			stopLossToValidate = d.NewStopLoss
		} else if d.StopLoss > 0 {
			// 兼容：AI 可能把新止损放在 stop_loss 字段
			stopLossToValidate = d.StopLoss
		} else {
			return fmt.Errorf("update_stop_loss 必须提供新止损价格 (new_stop_loss 或 stop_loss)")
		}
	}

	// 确定持仓方向（用于计算止损距离和判断是否保护利润）
	isLong := false
	var currentStopLoss float64 = 0 // 当前止损（用于检查是否调宽）

	if d.Action == "open_long" {
		isLong = true
	} else if d.Action == "open_short" {
		isLong = false
	} else if d.Action == "update_stop_loss" || d.Action == "update_take_profit" {
		// 查找现有持仓
		found := false
		for _, pos := range positions {
			if pos.Symbol == d.Symbol {
				entryPrice = pos.EntryPrice    // 更新为实际入场价
				currentStopLoss = pos.StopLoss // 获取当前止损
				if pos.Side == "long" {
					isLong = true
				} else {
					isLong = false
				}
				found = true
				break
			}
		}
		if !found {
			// 回退：尝试从逻辑推断
			if stopLossToValidate > 0 && d.TakeProfit > 0 {
				if stopLossToValidate < d.TakeProfit {
					isLong = true
				} else {
					isLong = false
				}
			} else {
				// 无法确定方向，只能跳过验证
				return nil
			}
		}
	} else {
		return nil
	}

	if entryPrice <= 0 {
		if d.Action == "open_long" || d.Action == "open_short" {
			return fmt.Errorf("无法确定入场价，无法验证止损距离")
		}
		return nil
	}

	// ⚠️ 关键修复：验证「禁止调宽止损」规则
	// Prompt 规则：多单的新止损必须 >= 旧止损；空单的新止损必须 <= 旧止损
	// ⚠️ 例外：如果当前止损距离不符合最低要求，则允许调宽以符合规定
	if d.Action == "update_stop_loss" && currentStopLoss > 0 {
		// 先计算当前止损距离，判断是否符合最低要求
		var currentStopLossDistancePct float64
		if isLong {
			currentStopLossDistancePct = (entryPrice - currentStopLoss) / entryPrice * 100
		} else {
			currentStopLossDistancePct = (currentStopLoss - entryPrice) / entryPrice * 100
		}

		// 检查当前止损是否符合最低要求
		highPriceCoinsCheck := map[string]bool{"BTCUSDT": true, "ETHUSDT": true}
		currentMinDistance := 0.6 // 山寨币 (Sniper Mode)
		if highPriceCoinsCheck[d.Symbol] {
			currentMinDistance = 0.4 // BTC/ETH (Sniper Mode)
		}

		currentSlIsCompliant := currentStopLossDistancePct >= currentMinDistance

		if isLong {
			// 多单：新止损不能低于旧止损（调宽 = 增加风险）
			// ⚠️ 例外：如果当前止损不合规，允许调宽以满足最低距离要求
			if stopLossToValidate < currentStopLoss {
				if currentSlIsCompliant {
					// 当前止损合规，禁止调宽
					return fmt.Errorf("禁止调宽止损！多单新止损(%.4f)必须≥旧止损(%.4f)，否则会增加风险",
						stopLossToValidate, currentStopLoss)
				} else {
					// 当前止损不合规，允许调宽以修复
					log.Printf("⚠️ [规则例外] 允许调宽止损：当前止损距离%.2f%% < 最低要求%.2f%%，新止损%.4f将使距离合规",
						currentStopLossDistancePct, currentMinDistance, stopLossToValidate)
				}
			}
		} else {
			// 空单：新止损不能高于旧止损（调宽 = 增加风险）
			// ⚠️ 例外：如果当前止损不合规，允许调宽以满足最低距离要求
			if stopLossToValidate > currentStopLoss {
				if currentSlIsCompliant {
					// 当前止损合规，禁止调宽
					return fmt.Errorf("禁止调宽止损！空单新止损(%.4f)必须≤旧止损(%.4f)，否则会增加风险",
						stopLossToValidate, currentStopLoss)
				} else {
					// 当前止损不合规，允许调宽以修复
					log.Printf("⚠️ [规则例外] 允许调宽止损：当前止损距离%.2f%% < 最低要求%.2f%%，新止损%.4f将使距离合规",
						currentStopLossDistancePct, currentMinDistance, stopLossToValidate)
				}
			}
		}
	}

	// ⚠️ 关键修复：止损距离应基于「当前价格」而非「入场价」计算
	// 原因：追蹤止損時，真正需要保證的是距離當前價格的波動空間，而非距離原始入場價的空間
	referencePrice := entryPrice
	if d.Action == "update_stop_loss" {
		marketData, ok := marketDataMap[d.Symbol]
		if ok && marketData.CurrentPrice > 0 {
			referencePrice = marketData.CurrentPrice
		} else {
			log.Printf("⚠️ 无法获取 %s 当前价格，降级使用入场价计算止损距离", d.Symbol)
		}
	}

	// 计算止损距离百分比 (相对于参考价格)
	var stopLossDistancePct float64
	if isLong {
		stopLossDistancePct = (referencePrice - stopLossToValidate) / referencePrice * 100
	} else {
		stopLossDistancePct = (stopLossToValidate - referencePrice) / referencePrice * 100
	}

	// 高价格币种列表
	highPriceCoins := map[string]bool{
		"BTCUSDT": true,
		"ETHUSDT": true,
	}

	// 动态计算最小止损距离 (Dynamic ATR Floor)
	// 默认硬底 (Hard Floor): 0.2% (防止极低波动时的过度剥头皮)
	minDistance := 0.2

	// 获取 ATR 数据
	var atr1hPct float64
	if marketData, ok := marketDataMap[d.Symbol]; ok && marketData.HourlyContext != nil {
		if marketData.CurrentPrice > 0 {
			atr1hPct = (marketData.HourlyContext.ATR14 / marketData.CurrentPrice) * 100
		}
	}

	// 如果有 ATR 数据，使用 ATR 的 0.5 倍作为动态下限
	// 逻辑：如果 ATR 是 2%，则最小止损为 1%。如果 ATR 是 0.2%，则最小止损为 0.1% (但被硬底 0.2% 覆盖)
	if atr1hPct > 0 {
		dynamicFloor := atr1hPct * 0.5
		if dynamicFloor > minDistance {
			minDistance = dynamicFloor
			log.Printf("🔍 [%s] 动态止损下限(Active): %.2f%% (基于 1H_ATR14=%.2f%%)", d.Symbol, minDistance, atr1hPct)
		} else {
			// Log even if not active, for verification
			log.Printf("🔍 [%s] 动态止损下限(Passive): %.2f%% <= 硬底 (1H_ATR14=%.2f%%)", d.Symbol, dynamicFloor, atr1hPct)
		}
	} else {
		// 降级逻辑：如果没有 ATR，使用保守值
		minDistance = 0.5
		if highPriceCoins[d.Symbol] {
			minDistance = 0.3
		}
		log.Printf("⚠️ [%s] 缺失 ATR 数据，使用静态止损下限: %.2f%%", d.Symbol, minDistance)
	}

	// ⚠️ 关键修正：对于 update_stop_loss（追踪止损），放宽距离限制 (P14 Fix)
	if d.Action == "update_stop_loss" {
		isProtectingProfit := false
		if isLong && stopLossToValidate > entryPrice {
			isProtectingProfit = true // 锁定利润
		} else if !isLong && stopLossToValidate < entryPrice {
			isProtectingProfit = true // 锁定利润
		}

		// 只要是保护利润（或减少亏损，向入场价移动），允许更紧的止损
		// 0.3% 足以覆盖 Aster 的点差和滑点
		if isProtectingProfit {
			minDistance = 0.3
			if highPriceCoins[d.Symbol] {
				minDistance = 0.2 // BTC/ETH 可更紧
			}
		} else {
			// 即使不是保本，如果是为了减少风险（更靠近现价），也适度放宽
			// 例如：亏损从 -2% 变为 -1%
			minDistance = 0.5
			if highPriceCoins[d.Symbol] {
				minDistance = 0.3
			}
		}
	}

	// ⚠️ 增强：自动修正并记录警告日志
	if stopLossDistancePct < minDistance {
		originalStopLoss := stopLossToValidate
		var correctedStopLoss float64
		if isLong {
			correctedStopLoss = referencePrice * (1 - minDistance/100)
		} else {
			correctedStopLoss = referencePrice * (1 + minDistance/100)
		}

		// 添加警告日志，便于监控AI决策质量
		log.Printf("⚠️ 止损距离不足，已自动修正: %s 原止损=%.4f 新止损=%.4f 距离=%.2f%%→%.2f%% (入场价=%.4f)",
			d.Symbol, originalStopLoss, correctedStopLoss, stopLossDistancePct, minDistance, entryPrice)

		// 更新决策中的止损值
		if d.Action == "update_stop_loss" {
			d.NewStopLoss = correctedStopLoss
		} else {
			d.StopLoss = correctedStopLoss
		}

		d.Reasoning += fmt.Sprintf(" [系统自动修正: 止损距离 %.2f%% < 最小要求 %.2f%%, 止损已调整 %.4f→%.4f (entry:%.4f)]",
			stopLossDistancePct, minDistance, originalStopLoss, correctedStopLoss, entryPrice)

		// ⚠️ 关键修复：自动修正止损后，重新验证风险回报比
		// 防止止损调宽后 R:R 低于要求 但未被拦截的问题
		if d.Action == "open_long" || d.Action == "open_short" {
			if err := validateRiskRewardRatioAfterCorrection(d, entryPrice, minRiskRewardRatio, feeInfo); err != nil {
				// 回滚止损修正
				d.StopLoss = originalStopLoss
				return fmt.Errorf("止损修正后风险回报比不满足要求: %w", err)
			}
		}
		return nil
	}

	return nil
}

// validateRiskRewardRatioAfterCorrection 修正止损后验证风险回报比（扣除手续费）
// 与 validateRiskRewardRatio 逻辑相同，但提供更清晰的错误信息
func validateRiskRewardRatioAfterCorrection(d *Decision, entryPrice, minRiskRewardRatio float64, feeInfo *TradingFeeInfo) error {
	if entryPrice <= 0 {
		return nil // 无法验证
	}

	var riskPercent, grossRewardPercent, netRewardPercent, netRiskRewardRatio float64
	if d.Action == "open_long" {
		riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
		grossRewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
	} else {
		riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
		grossRewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
	}

	// 💰 扣除手续费计算净利润 和 增加手续费计算净风险
	var netRiskPercent float64
	if feeInfo != nil {
		roundTripFeePct := feeInfo.RoundTripFeeRate * 100
		netRewardPercent = grossRewardPercent - roundTripFeePct
		netRiskPercent = riskPercent + roundTripFeePct // 风险也必须包含手续费损失

		if netRewardPercent <= 0 {
			return fmt.Errorf("修正后止盈距离过小(%.3f%%)，扣除手续费(%.3f%%)后净利润≤0", grossRewardPercent, roundTripFeePct)
		}
	} else {
		netRewardPercent = grossRewardPercent
		netRiskPercent = riskPercent
	}

	// 使用净风险计算 R:R
	if netRiskPercent > 0 {
		netRiskRewardRatio = netRewardPercent / netRiskPercent
	}

	// 硬约束: 使用配置的最小风险回报比 (带 0.05 容差)
	threshold := minRiskRewardRatio - 0.05
	if netRiskRewardRatio < threshold {
		if feeInfo != nil {
			return fmt.Errorf("修正后淨R:R=%.2f:1 < %.1f [入场:%.4f 止损:%.4f 止盈:%.4f | 净利%.3f%% / 净险%.3f%% (含手续费%.3f%%)]",
				netRiskRewardRatio, minRiskRewardRatio, entryPrice, d.StopLoss, d.TakeProfit,
				netRewardPercent, netRiskPercent, feeInfo.RoundTripFeeRate*100)
		} else {
			return fmt.Errorf("修正后R:R=%.2f:1 < %.1f [入场:%.4f 止损:%.4f 止盈:%.4f]",
				netRiskRewardRatio, minRiskRewardRatio, entryPrice, d.StopLoss, d.TakeProfit)
		}
	}

	log.Printf("✅ 止损修正后 净R:R 验证通过: %.2f:1", netRiskRewardRatio)
	return nil
}

// validateRiskRewardRatio 验证风险回报比（扣除手续费后的净利润）
func validateRiskRewardRatio(d *Decision, minRiskRewardRatio float64, feeInfo *TradingFeeInfo) error {
	entryPrice := d.EntryPrice
	if entryPrice <= 0 {
		if d.Action == "open_long" {
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.25
		} else {
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.25
		}
	}

	var riskPercent, grossRewardPercent, netRewardPercent, netRiskRewardRatio float64

	// 計算毛利潤（未扣費）
	if d.Action == "open_long" {
		riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
		grossRewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
	} else {
		riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
		grossRewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
	}

	// 💰 扣除手續費計算淨利潤 和 增加手續費計算淨風險
	var netRiskPercent float64
	if feeInfo != nil {
		roundTripFeePct := feeInfo.RoundTripFeeRate * 100 // 轉為百分比
		netRewardPercent = grossRewardPercent - roundTripFeePct
		netRiskPercent = riskPercent + roundTripFeePct // 虧損時也要付手續費！

		// 檢查扣費後是否仍有利潤
		if netRewardPercent <= 0 {
			return fmt.Errorf("止盈距離過小(%.3f%%)，扣除來回手續費(%.3f%%)後淨利潤≤0，此交易必虧！請增加止盈距離至 >%.3f%%",
				grossRewardPercent, roundTripFeePct, roundTripFeePct*2)
		}
	} else {
		// 如果沒有手續費信息，使用毛利潤（向後兼容）
		netRewardPercent = grossRewardPercent
		netRiskPercent = riskPercent
	}

	// 計算淨風險回報比
	if netRiskPercent > 0 {
		netRiskRewardRatio = netRewardPercent / netRiskPercent
	}

	// 硬约束: 使用配置的最小风险回报比 (带 0.05 容差)
	threshold := minRiskRewardRatio - 0.05
	if netRiskRewardRatio < threshold {
		if feeInfo != nil {
			return fmt.Errorf("淨風險回報比過低(%.2f:1)，必須≥%.1f:1 [入場:%.4f 止損:%.4f 止盈:%.4f | 净利%.3f%% / 净险%.3f%% (含手续费%.3f%%)]",
				netRiskRewardRatio, minRiskRewardRatio, entryPrice, d.StopLoss, d.TakeProfit,
				netRewardPercent, netRiskPercent, feeInfo.RoundTripFeeRate*100)
		} else {
			return fmt.Errorf("风险回报比过低(%.2f:1)，必须≥%.1f:1 [入场:%.4f 止损:%.4f 止盈:%.4f]",
				netRiskRewardRatio, minRiskRewardRatio, entryPrice, d.StopLoss, d.TakeProfit)
		}
	}

	return nil
}

// validateMaxLoss 验证单笔最大亏损
func validateMaxLoss(d *Decision, accountEquity float64) error {
	entryPrice := d.EntryPrice
	if entryPrice <= 0 {
		return nil
	}

	var riskAmount float64
	if d.Action == "open_long" {
		riskAmount = (entryPrice - d.StopLoss) * (d.PositionSizeUSD / entryPrice)
	} else {
		riskAmount = (d.StopLoss - entryPrice) * (d.PositionSizeUSD / entryPrice)
	}

	riskPercent := (riskAmount / accountEquity) * 100
	maxLossLimit := 3.05
	if accountEquity < 10.0 {
		maxLossLimit = 5.5 // 💰 自适应放宽：微型资金模式（净值 < 10 USDT）放宽至 5.5% 净值，以包容合理的 ATR 波动
	}
	if riskPercent > maxLossLimit {
		return fmt.Errorf("单笔最大亏损过大(%.2f%%)，必须≤%.1f%%账户净值", riskPercent, maxLossLimit)
	}
	return nil
}

// validatePartialCloseSize 验证部分平仓金额是否满足最小要求
func validatePartialCloseSize(d *Decision, positions []PositionInfo, marketDataMap map[string]*market.Data) error {
	// 查找当前持仓
	var currentPos *PositionInfo
	for _, p := range positions {
		if p.Symbol == d.Symbol {
			currentPos = &p
			break
		}
	}

	if currentPos == nil {
		// 如果找不到持仓，此处通过，留给Executor处理（由Executor报告"没有持仓"）
		return nil
	}

	// 确定计算价格
	price := 0.0
	if marketData, ok := marketDataMap[d.Symbol]; ok {
		price = marketData.CurrentPrice
	}
	if price <= 0 {
		price = currentPos.EntryPrice // 降级使用入场价
	}
	if price <= 0 {
		return nil // 无法计算价值，跳过验证
	}

	// 计算持倉總價值和平仓价值
	holdAmt := math.Abs(currentPos.Quantity)
	totalValue := holdAmt * price
	closeAmt := holdAmt * (d.ClosePercentage / 100.0)
	closeValue := closeAmt * price

	// Binance 最小名义价值通常为 5-10 USDT
	// 这里设置 conservative 阈值为 10.0
	const minNotional = 10.0

	// 特殊处理：如果平仓比例接近 100%，则认为是全平，通常不仅受MinNotional限制(取决于是否ReduceOnly)
	// 但如果全仓价值 < 10，平仓是可以的（清算）。
	// 问题是 Partial Close (e.g. 50%) 生成的 Order 必须满足 MinNotional。

	if d.ClosePercentage > 99.0 {
		return nil // 全平不受此限制（只要持仓存在）
	}

	if closeValue < minNotional {
		// 計算最低可平倉比例
		minPercentage := (minNotional / totalValue) * 100
		if minPercentage > 100 {
			minPercentage = 100
		}

		return fmt.Errorf("部分平倉價值過小($%.2f < $%.1f): 交易所最小單限制(MinNotional)。"+
			"當前持倉總價值: $%.2f。"+
			"解決方案: (1) 使用 close_long/close_short 全平此倉位 "+
			"(2) 或使用 hold 繼續持有 "+
			"(3) 如果堅持部分平倉，比例至少需要 %.0f%% 才能滿足最小金額。"+
			"禁止: 嘗試平倉比例 %.0f%% (金額 $%.2f)。",
			closeValue, minNotional, totalValue, minPercentage, d.ClosePercentage, closeValue)
	}

	return nil
}

// hasMatchingPosition 检查是否有匹配的持仓
func hasMatchingPosition(symbol string, action string, positions []PositionInfo) bool {
	upperSymbol := strings.ToUpper(symbol)
	expectedSide := ""
	switch action {
	case "close_long":
		expectedSide = "long"
	case "close_short":
		expectedSide = "short"
	case "partial_close", "update_stop_loss", "update_take_profit":
		// 仅需币种相同即可
		for _, p := range positions {
			if strings.ToUpper(p.Symbol) == upperSymbol {
				return true
			}
		}
		return false
	default:
		return true
	}

	for _, p := range positions {
		if strings.ToUpper(p.Symbol) == upperSymbol && strings.ToLower(p.Side) == expectedSide {
			return true
		}
	}
	return false
}
