package trader

import (
	"fmt"
	"log"
	"math"
	"aetheris/decision"
	"aetheris/logger"
	"aetheris/market"
	"strings"
	"time"
)

// DecisionExecutor 负责执行交易决策
type DecisionExecutor struct {
	trader         Trader
	config         AutoTraderConfig
	contextBuilder *ContextBuilder
	dataProvider   market.DataProvider
	sleepFunc      func(time.Duration) // 注入的休眠函数
}

// NewDecisionExecutor 创建DecisionExecutor
func NewDecisionExecutor(trader Trader, config AutoTraderConfig, contextBuilder *ContextBuilder, dataProvider market.DataProvider, sleepFunc func(time.Duration)) *DecisionExecutor {
	// 如果未提供 sleepFunc，默认为 time.Sleep（实盘安全模式）
	if sleepFunc == nil {
		sleepFunc = time.Sleep
	}
	return &DecisionExecutor{
		trader:         trader,
		config:         config,
		contextBuilder: contextBuilder,
		dataProvider:   dataProvider,
		sleepFunc:      sleepFunc,
	}
}

// SetTrader 更新交易器接口（用于回测注入）
func (de *DecisionExecutor) SetTrader(trader Trader) {
	de.trader = trader
}

// ExecuteDecisionWithRecord 执行AI决策并记录详细信息
func (de *DecisionExecutor) ExecuteDecisionWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	switch decision.Action {
	case "open_long":
		return de.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		return de.executeOpenShortWithRecord(decision, actionRecord)
	case "close_long":
		return de.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		return de.executeCloseShortWithRecord(decision, actionRecord)
	case "update_stop_loss":
		return de.executeUpdateStopLossWithRecord(decision, actionRecord)
	case "update_take_profit":
		return de.executeUpdateTakeProfitWithRecord(decision, actionRecord)
	case "partial_close":
		return de.executePartialCloseWithRecord(decision, actionRecord)
	case "hold", "wait":
		// 无需执行，仅记录
		return nil
	default:
		return fmt.Errorf("未知的action: %s", decision.Action)
	}
}

// executeOpenLongWithRecord 执行开多仓并记录详细信息
func (de *DecisionExecutor) executeOpenLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📈 开多仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
	positions, err := de.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				return fmt.Errorf("❌ %s 已有多仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_long 决策", decision.Symbol)
			}
		}
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, de.dataProvider, nil)
	if err != nil {
		return err
	}

	// Pro Trader Check: 实时滑点风险检查
	// 验证当前价格是否导致盈亏比恶化
	// Long: SL < Price < TP
	if marketData.CurrentPrice <= decision.StopLoss {
		return fmt.Errorf("❌ 价格已触及/低于止损位 (Cur:%.4f <= SL:%.4f)，放弃开仓", marketData.CurrentPrice, decision.StopLoss)
	}
	if marketData.CurrentPrice >= decision.TakeProfit {
		return fmt.Errorf("❌ 价格已触及/高于止盈位 (Cur:%.4f >= TP:%.4f)，放弃开仓", marketData.CurrentPrice, decision.TakeProfit)
	}

	currentRisk := (marketData.CurrentPrice - decision.StopLoss) / marketData.CurrentPrice
	currentReward := (decision.TakeProfit - marketData.CurrentPrice) / marketData.CurrentPrice
	if currentRisk > 0 {
		currentRR := currentReward / currentRisk
		// 允许一定滑点，但不能低于 1.2 (Plan是2.0)
		if currentRR < 1.2 {
			log.Printf("⚠️ 盈亏比恶化警告: 计划R:R > 1.5, 实际R:R = %.2f (滑点导致)", currentRR)
			return fmt.Errorf("❌ 实时滑点导致盈亏比恶化 (%.2f < 1.2)，放弃开多", currentRR)
		}
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// ⚠️ 保证金验证：防止保证金不足错误（code=-2019）
	requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)

	balance, err := de.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// 手续费估算（Taker费率 0.04%）
	estimatedFee := decision.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee

	if totalRequired > availableBalance {
		// ✅ 自动调整仓位大小
		// 公式: size * (1/leverage + feeRate) <= availableBalance
		// feeRate = 0.0004 (taker) + 0.0002 (buffer) = 0.0006
		maxSizeUSD := availableBalance / (1.0/float64(decision.Leverage) + 0.0006)

		// 检查调整后的金额是否过小 (例如小于 5 USDT)
		if maxSizeUSD < 5.0 {
			return fmt.Errorf("❌ 保证金不足且调整后金额过小: 需要 %.2f USDT，可用 %.2f USDT",
				totalRequired, availableBalance)
		}

		log.Printf("⚠️ 余额不足 (需 %.2f, 有 %.2f)，自动调整仓位: %.2f -> %.2f USDT",
			totalRequired, availableBalance, decision.PositionSizeUSD, maxSizeUSD)

		// 更新决策金额和数量
		decision.PositionSizeUSD = maxSizeUSD
		quantity = decision.PositionSizeUSD / marketData.CurrentPrice
		actionRecord.Quantity = quantity
	}

	// 设置仓位模式
	if err := de.trader.SetMarginMode(decision.Symbol, de.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		// 继续执行，不影响交易
	}

	// 开仓
	order, err := de.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

	// ✅ 驗證實際開倉數量（防止數量精度問題導致開倉失敗）
	time.Sleep(500 * time.Millisecond) // 等待交易所處理訂單
	actualPositions, posErr := de.trader.GetPositions()
	if posErr == nil {
		var actualPosition map[string]interface{}
		for _, pos := range actualPositions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				actualPosition = pos
				break
			}
		}

		if actualPosition == nil {
			log.Printf("  ⚠️ 警告：開倉後未找到實際持倉，可能開倉失敗或數量過小被四舍五入為0")
			return fmt.Errorf("開倉後未找到實際持倉，請檢查數量精度或開倉金額")
		}

		actualQuantity := 0.0
		if qty, ok := actualPosition["positionAmt"].(float64); ok {
			if qty < 0 {
				actualQuantity = -qty
			} else {
				actualQuantity = qty
			}
		}

		if actualQuantity < 0.0001 {
			log.Printf("  ⚠️ 警告：實際開倉數量過小(%.8f)，可能被四舍五入為0，建議增加開倉金額", actualQuantity)
			return fmt.Errorf("實際開倉數量過小(%.8f)，請增加開倉金額", actualQuantity)
		}

		log.Printf("  ✓ 驗證成功：實際持倉數量=%.8f", actualQuantity)
		// 使用實際持倉數量設置止損止盈
		quantity = actualQuantity
	}

	// 记录开仓时间
	de.contextBuilder.RecordPositionTime(decision.Symbol, "long")
	// 记录开仓理由
	de.contextBuilder.RecordEntryReason(decision.Symbol, "long", decision.Reasoning)

	// ✅ 驗證止損止盈價格合理性（防止立即觸發）
	currentPrice := marketData.CurrentPrice

	// ⚠️ SL/TP 设置失败时重试，仍失败则立即平仓
	// 確保不會有無保護的倉位存在
	slSetSuccess := false
	slPriceInvalid := false // 追蹤止損價格是否無效
	tpSetSuccess := false

	// 設置止損（帶重試）
	if decision.StopLoss >= currentPrice {
		log.Printf("  ⚠️ 警告：做多時止損價格(%.4f)不應高於或等於當前價格(%.4f)，跳過設置止損", decision.StopLoss, currentPrice)
		slPriceInvalid = true // 標記價格無效
	} else {
		for attempt := 1; attempt <= 2; attempt++ {
			if err := de.trader.SetStopLoss(decision.Symbol, "LONG", quantity, decision.StopLoss); err != nil {
				log.Printf("  ⚠️ 設置止損失敗 (嘗試 %d/2): %v", attempt, err)
				if attempt < 2 {
					time.Sleep(500 * time.Millisecond)
				}
			} else {
				log.Printf("  ✓ 止损设置成功: %.4f", decision.StopLoss)
				slSetSuccess = true
				break
			}
		}
	}

	// 設置止盈（帶重試）
	if decision.TakeProfit <= currentPrice {
		log.Printf("  ⚠️ 警告：做多時止盈價格(%.4f)不應低於或等於當前價格(%.4f)，跳過設置止盈", decision.TakeProfit, currentPrice)
		tpSetSuccess = true // 跳過也算成功（價格不合理）
	} else {
		for attempt := 1; attempt <= 2; attempt++ {
			if err := de.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, decision.TakeProfit); err != nil {
				log.Printf("  ⚠️ 設置止盈失敗 (嘗試 %d/2): %v", attempt, err)
				if attempt < 2 {
					time.Sleep(500 * time.Millisecond)
				}
			} else {
				log.Printf("  ✓ 止盈设置成功: %.4f", decision.TakeProfit)
				tpSetSuccess = true
				break
			}
		}
	}

	// ⚠️ 關鍵修復：如果止損設置失敗或價格無效，立即平倉（止盈失敗可以容忍）
	// 情況1: SL 設置失敗（API 錯誤）
	// 情況2: SL 價格無效（AI 給出錯誤的止損價格）
	if !slSetSuccess && !slPriceInvalid {
		// SL 設置 API 失敗
		log.Printf("  ❌ 嚴重：止損設置失敗且重試仍失敗，立即平倉以避免無保護風險！")
		if _, closeErr := de.trader.CloseLong(decision.Symbol, 0); closeErr != nil {
			log.Printf("  ❌ 緊急平倉也失敗: %v（請手動處理！）", closeErr)
			return fmt.Errorf("止損設置失敗且無法緊急平倉: %w", closeErr)
		}
		log.Printf("  ✓ 已緊急平倉，避免無保護風險")
		return fmt.Errorf("止損設置失敗，已緊急平倉以保護資金")
	}

	if slPriceInvalid {
		// SL 價格無效（AI 錯誤）
		log.Printf("  ❌ 嚴重：AI 給出的止損價格無效(%.4f >= 當前價%.4f)，立即平倉！", decision.StopLoss, currentPrice)
		if _, closeErr := de.trader.CloseLong(decision.Symbol, 0); closeErr != nil {
			log.Printf("  ❌ 緊急平倉也失敗: %v（請手動處理！）", closeErr)
			return fmt.Errorf("止損價格無效且無法緊急平倉: %w", closeErr)
		}
		log.Printf("  ✓ 已緊急平倉，避免無保護風險")
		return fmt.Errorf("止損價格無效，已緊急平倉以保護資金")
	}

	// 如果止盈失敗但止損成功，只是警告（至少有止損保護）
	if !tpSetSuccess {
		log.Printf("  ⚠️ 止盈設置失敗，但止損已設置，倉位有基本保護")
	}

	// ⚠️ RecordTrade 已移至 RunCycle 中統一處理，避免重複記錄交易頻率

	return nil
}

// executeOpenShortWithRecord 执行开空仓并记录详细信息
func (de *DecisionExecutor) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📉 开空仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
	positions, err := de.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				return fmt.Errorf("❌ %s 已有空仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_short 决策", decision.Symbol)
			}
		}
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, de.dataProvider, nil)
	if err != nil {
		return err
	}

	// Pro Trader Check: 实时滑点风险检查
	// 验证当前价格是否导致盈亏比恶化
	// Short: SL > Price > TP
	if marketData.CurrentPrice >= decision.StopLoss {
		return fmt.Errorf("❌ 价格已触及/超过止损位 (Cur:%.4f >= SL:%.4f)，放弃开仓", marketData.CurrentPrice, decision.StopLoss)
	}
	if marketData.CurrentPrice <= decision.TakeProfit {
		return fmt.Errorf("❌ 价格已触及/低于止盈位 (Cur:%.4f <= TP:%.4f)，放弃开仓", marketData.CurrentPrice, decision.TakeProfit)
	}

	currentRisk := (decision.StopLoss - marketData.CurrentPrice) / marketData.CurrentPrice
	currentReward := (marketData.CurrentPrice - decision.TakeProfit) / marketData.CurrentPrice
	if currentRisk > 0 {
		currentRR := currentReward / currentRisk
		// 允许一定滑点，但不能低于 1.2 (Plan是2.0)
		if currentRR < 1.2 {
			log.Printf("⚠️ 盈亏比恶化警告: 计划R:R > 1.5, 实际R:R = %.2f (滑点导致)", currentRR)
			return fmt.Errorf("❌ 实时滑点导致盈亏比恶化 (%.2f < 1.2)，放弃开空", currentRR)
		}
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// ⚠️ 保证金验证：防止保证金不足错误（code=-2019）
	requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)

	balance, err := de.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// 手续费估算（Taker费率 0.04%）
	estimatedFee := decision.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee

	if totalRequired > availableBalance {
		// ✅ 自动调整仓位大小
		// 公式: size * (1/leverage + feeRate) <= availableBalance
		// feeRate = 0.0004 (taker) + 0.0002 (buffer) = 0.0006
		maxSizeUSD := availableBalance / (1.0/float64(decision.Leverage) + 0.0006)

		// 检查调整后的金额是否过小 (例如小于 5 USDT)
		if maxSizeUSD < 5.0 {
			return fmt.Errorf("❌ 保证金不足且调整后金额过小: 需要 %.2f USDT，可用 %.2f USDT",
				totalRequired, availableBalance)
		}

		log.Printf("⚠️ 余额不足 (需 %.2f, 有 %.2f)，自动调整仓位: %.2f -> %.2f USDT",
			totalRequired, availableBalance, decision.PositionSizeUSD, maxSizeUSD)

		// 更新决策金额和数量
		decision.PositionSizeUSD = maxSizeUSD
		quantity = decision.PositionSizeUSD / marketData.CurrentPrice
		actionRecord.Quantity = quantity
	}

	// 设置仓位模式
	if err := de.trader.SetMarginMode(decision.Symbol, de.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		// 继续执行，不影响交易
	}

	// 开仓
	order, err := de.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

	// ✅ 驗證實際開倉數量（防止數量精度問題導致開倉失敗）
	time.Sleep(500 * time.Millisecond) // 等待交易所處理訂單
	actualPositions, posErr := de.trader.GetPositions()
	if posErr == nil {
		var actualPosition map[string]interface{}
		for _, pos := range actualPositions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				actualPosition = pos
				break
			}
		}

		if actualPosition == nil {
			log.Printf("  ⚠️ 警告：開倉後未找到實際持倉，可能開倉失敗或數量過小被四舍五入為0")
			return fmt.Errorf("開倉後未找到實際持倉，請檢查數量精度或開倉金額")
		}

		actualQuantity := 0.0
		if qty, ok := actualPosition["positionAmt"].(float64); ok {
			if qty < 0 {
				actualQuantity = -qty
			} else {
				actualQuantity = qty
			}
		}

		if actualQuantity < 0.0001 {
			log.Printf("  ⚠️ 警告：實際開倉數量過小(%.8f)，可能被四舍五入為0，建議增加開倉金額", actualQuantity)
			return fmt.Errorf("實際開倉數量過小(%.8f)，請增加開倉金額", actualQuantity)
		}

		log.Printf("  ✓ 驗證成功：實際持倉數量=%.8f", actualQuantity)
		// 使用實際持倉數量設置止損止盈
		quantity = actualQuantity
	}

	// 记录开仓时间
	de.contextBuilder.RecordPositionTime(decision.Symbol, "short")
	// 记录开仓理由
	de.contextBuilder.RecordEntryReason(decision.Symbol, "short", decision.Reasoning)

	// ✅ 驗證止損止盈價格合理性（防止立即觸發）
	currentPrice := marketData.CurrentPrice

	// ⚠️ SL/TP 设置失败时重试，仍失败则立即平仓
	// 確保不會有無保護的倉位存在
	slSetSuccess := false
	slPriceInvalid := false // 追蹤止損價格是否無效
	tpSetSuccess := false

	// 設置止損（帶重試）
	if decision.StopLoss <= currentPrice {
		log.Printf("  ⚠️ 警告：做空時止損價格(%.4f)不應低於或等於當前價格(%.4f)，跳過設置止損", decision.StopLoss, currentPrice)
		slPriceInvalid = true // 標記價格無效
	} else {
		for attempt := 1; attempt <= 2; attempt++ {
			if err := de.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, decision.StopLoss); err != nil {
				log.Printf("  ⚠️ 設置止損失敗 (嘗試 %d/2): %v", attempt, err)
				if attempt < 2 {
					time.Sleep(500 * time.Millisecond)
				}
			} else {
				log.Printf("  ✓ 止损设置成功: %.4f", decision.StopLoss)
				slSetSuccess = true
				break
			}
		}
	}

	// 設置止盈（帶重試）
	if decision.TakeProfit >= currentPrice {
		log.Printf("  ⚠️ 警告：做空時止盈價格(%.4f)不應高於或等於當前價格(%.4f)，跳過設置止盈", decision.TakeProfit, currentPrice)
		tpSetSuccess = true // 跳過也算成功（價格不合理）
	} else {
		for attempt := 1; attempt <= 2; attempt++ {
			if err := de.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, decision.TakeProfit); err != nil {
				log.Printf("  ⚠️ 設置止盈失敗 (嘗試 %d/2): %v", attempt, err)
				if attempt < 2 {
					time.Sleep(500 * time.Millisecond)
				}
			} else {
				log.Printf("  ✓ 止盈设置成功: %.4f", decision.TakeProfit)
				tpSetSuccess = true
				break
			}
		}
	}

	// ⚠️ 關鍵修復：如果止損設置失敗或價格無效，立即平倉（止盈失敗可以容忍）
	if !slSetSuccess && !slPriceInvalid {
		// SL 設置 API 失敗
		log.Printf("  ❌ 嚴重：止損設置失敗且重試仍失敗，立即平倉以避免無保護風險！")
		if _, closeErr := de.trader.CloseShort(decision.Symbol, 0); closeErr != nil {
			log.Printf("  ❌ 緊急平倉也失敗: %v（請手動處理！）", closeErr)
			return fmt.Errorf("止損設置失敗且無法緊急平倉: %w", closeErr)
		}
		log.Printf("  ✓ 已緊急平倉，避免無保護風險")
		return fmt.Errorf("止損設置失敗，已緊急平倉以保護資金")
	}

	if slPriceInvalid {
		// SL 價格無效（AI 錯誤）
		log.Printf("  ❌ 嚴重：AI 給出的止損價格無效(%.4f <= 當前價%.4f)，立即平倉！", decision.StopLoss, currentPrice)
		if _, closeErr := de.trader.CloseShort(decision.Symbol, 0); closeErr != nil {
			log.Printf("  ❌ 緊急平倉也失敗: %v（請手動處理！）", closeErr)
			return fmt.Errorf("止損價格無效且無法緊急平倉: %w", closeErr)
		}
		log.Printf("  ✓ 已緊急平倉，避免無保護風險")
		return fmt.Errorf("止損價格無效，已緊急平倉以保護資金")
	}

	// 如果止盈失敗但止損成功，只是警告（至少有止損保護）
	if !tpSetSuccess {
		log.Printf("  ⚠️ 止盈設置失敗，但止損已設置，倉位有基本保護")
	}

	// ⚠️ RecordTrade 已移至 RunCycle 中統一處理，避免重複記錄交易頻率

	return nil
}

// executeCloseLongWithRecord 执行平多仓并记录详细信息
func (de *DecisionExecutor) executeCloseLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平多仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, de.dataProvider, nil)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 1. 获取平仓前的持仓均价（用于计算盈亏）
	var entryPrice float64
	positions, err := de.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				if price, ok := pos["entryPrice"].(float64); ok {
					entryPrice = price
				}
				break
			}
		}
	}

	// 2. 平仓
	order, err := de.trader.CloseLong(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")

	// 3. 记录交易结果（更新连续亏损）
	if entryPrice > 0 && marketData.CurrentPrice > 0 {
		// 做多：平仓价 > 入场价 * (1 + 费率) = 真实盈利
		// Taker费率约 0.05% * 2 = 0.1%。使用 0.15% 作为安全缓冲
		isProfitable := marketData.CurrentPrice > entryPrice*1.0015
		de.contextBuilder.RecordTrade(true, isProfitable)

		resultStr := "亏损"
		if isProfitable {
			resultStr = "盈利"
		}
		log.Printf("  📊 交易结果记录: %s (均价: %.4f -> 平仓: %.4f)", resultStr, entryPrice, marketData.CurrentPrice)
	}

	return nil
}

// executeCloseShortWithRecord 执行平空仓并记录详细信息
func (de *DecisionExecutor) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平空仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, de.dataProvider, nil)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 1. 获取平仓前的持仓均价（用于计算盈亏）
	var entryPrice float64
	positions, err := de.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				if price, ok := pos["entryPrice"].(float64); ok {
					entryPrice = price
				}
				break
			}
		}
	}

	// 2. 平仓
	order, err := de.trader.CloseShort(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")

	// 3. 记录交易结果（更新连续亏损）
	if entryPrice > 0 && marketData.CurrentPrice > 0 {
		// 做空：平仓价 < 入场价 * (1 - 费率) = 真实盈利
		// Taker费率约 0.05% * 2 = 0.1%。使用 0.15% 作为安全缓冲
		isProfitable := marketData.CurrentPrice < entryPrice*0.9985
		de.contextBuilder.RecordTrade(true, isProfitable)

		resultStr := "亏损"
		if isProfitable {
			resultStr = "盈利"
		}
		log.Printf("  📊 交易结果记录: %s (均价: %.4f -> 平仓: %.4f)", resultStr, entryPrice, marketData.CurrentPrice)
	}

	return nil
}

// executeUpdateStopLossWithRecord 执行调整止损并记录详细信息
func (de *DecisionExecutor) executeUpdateStopLossWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 调整止损: %s → %.2f", decision.Symbol, decision.NewStopLoss)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, de.dataProvider, nil)
	if err != nil {
		return err
	}
	// 记录的是目标止损价格，而非当前市场价格（前端显示优化）
	actionRecord.Price = decision.NewStopLoss

	// 获取当前持仓
	positions, err := de.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 验证新止损价格合理性
	if positionSide == "LONG" && decision.NewStopLoss >= marketData.CurrentPrice {
		return fmt.Errorf("多单止损必须低于当前价格 (当前: %.2f, 新止损: %.2f)", marketData.CurrentPrice, decision.NewStopLoss)
	}
	if positionSide == "SHORT" && decision.NewStopLoss <= marketData.CurrentPrice {
		return fmt.Errorf("空单止损必须高于当前价格 (当前: %.2f, 新止损: %.2f)", marketData.CurrentPrice, decision.NewStopLoss)
	}

	// 取消旧的止损单（避免多个止损单共存）
	if err := de.trader.CancelStopLossOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止损单失败: %v", err)
		// 不中断执行，继续设置新止损
	}

	// 调用交易所 API 修改止损
	quantity := math.Abs(positionAmt)
	err = de.trader.SetStopLoss(decision.Symbol, positionSide, quantity, decision.NewStopLoss)
	if err != nil {
		// 🔴 P15 Fail-Safe: 如果设置止损失败，此时旧止损已被取消，仓位处于裸奔状态
		// 必须执行 "紧急平仓" 以保护资金安全
		log.Printf("🚨 严重错误: 修改止损失败，且旧止损已取消！正在执行紧急平仓保护... (Symbol: %s, Err: %v)", decision.Symbol, err)

		var closeErr error
		if positionSide == "LONG" {
			_, closeErr = de.trader.CloseLong(decision.Symbol, quantity)
		} else {
			_, closeErr = de.trader.CloseShort(decision.Symbol, quantity)
		}

		if closeErr != nil {
			log.Printf("🔥 灾难性错误: 紧急平仓也失败了！！！请人工立即介入！！！(Symbol: %s, Err: %v)", decision.Symbol, closeErr)
			return fmt.Errorf("修改止损失败 且 紧急平仓失败: %v | %v", err, closeErr)
		}

		log.Printf("✅ 紧急平仓成功：已清除无保护的持仓 %s", decision.Symbol)
		return fmt.Errorf("修改止损失败，已执行紧急平仓保护: %w", err)
	}

	log.Printf("  ✓ 止损已调整: %.2f (当前价格: %.2f)", decision.NewStopLoss, marketData.CurrentPrice)
	return nil
}

// executeUpdateTakeProfitWithRecord 执行调整止盈并记录详细信息
func (de *DecisionExecutor) executeUpdateTakeProfitWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 调整止盈: %s → %.2f", decision.Symbol, decision.NewTakeProfit)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, de.dataProvider, nil)
	if err != nil {
		return err
	}
	// 记录的是目标止盈价格，而非当前市场价格（前端显示优化）
	actionRecord.Price = decision.NewTakeProfit

	// 获取当前持仓
	positions, err := de.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 验证新止盈价格合理性
	if positionSide == "LONG" && decision.NewTakeProfit <= marketData.CurrentPrice {
		return fmt.Errorf("多单止盈必须高于当前价格 (当前: %.2f, 新止盈: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}
	if positionSide == "SHORT" && decision.NewTakeProfit >= marketData.CurrentPrice {
		return fmt.Errorf("空单止盈必须低于当前价格 (当前: %.2f, 新止盈: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}

	// 取消旧的止盈单（避免多个止盈单共存）
	if err := de.trader.CancelTakeProfitOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止盈单失败: %v", err)
		// 不中断执行，继续设置新止盈
	}

	// 调用交易所 API 修改止盈
	quantity := math.Abs(positionAmt)
	err = de.trader.SetTakeProfit(decision.Symbol, positionSide, quantity, decision.NewTakeProfit)
	if err != nil {
		return fmt.Errorf("修改止盈失败: %w", err)
	}

	log.Printf("  ✓ 止盈已调整: %.2f (当前价格: %.2f)", decision.NewTakeProfit, marketData.CurrentPrice)
	return nil
}

// executePartialCloseWithRecord 执行部分平仓并记录详细信息
func (de *DecisionExecutor) executePartialCloseWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📊 部分平仓: %s %.1f%%", decision.Symbol, decision.ClosePercentage)

	// 验证百分比范围
	if decision.ClosePercentage <= 0 || decision.ClosePercentage > 100 {
		return fmt.Errorf("平仓百分比必须在 0-100 之间，当前: %.1f", decision.ClosePercentage)
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, de.dataProvider, nil)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := de.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 计算平仓数量
	totalQuantity := math.Abs(positionAmt)
	closeQuantity := totalQuantity * (decision.ClosePercentage / 100.0)
	actionRecord.Quantity = closeQuantity

	// 🔴 P16 Synchronization: 获取当前 SL/TP 价格 (在平仓前保存状态)
	currentSL, currentTP, err := de.trader.GetOrderProtection(decision.Symbol, positionSide)
	if err != nil {
		log.Printf("  ⚠️ 警告: 部分平仓前无法获取 SL/TP 价格，后续可能无法自动调整挂单: %v", err)
	}

	// 执行平仓
	var order map[string]interface{}
	if positionSide == "LONG" {
		order, err = de.trader.CloseLong(decision.Symbol, closeQuantity)
	} else {
		order, err = de.trader.CloseShort(decision.Symbol, closeQuantity)
	}

	if err != nil {
		return fmt.Errorf("部分平仓失败: %w", err)
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	remainingQuantity := totalQuantity - closeQuantity
	log.Printf("  ✓ 部分平仓成功: 平仓 %.4f (%.1f%%), 剩余 %.4f",
		closeQuantity, decision.ClosePercentage, remainingQuantity)

	// 🔴 P16 Synchronization: 重置 SL/TP 以匹配剩余仓位
	// 避免 "挂单量 > 持仓量" 导致 ReduceOnly 订单在关键时刻被拒
	if remainingQuantity > 0 {
		// 1. 同步止损
		if currentSL > 0 {
			log.Printf("  🔄 P16: 同步止损单 (数量 %.4f @ %.4f)...", remainingQuantity, currentSL)
			de.trader.CancelStopLossOrders(decision.Symbol) // 先取消旧单
			if err := de.trader.SetStopLoss(decision.Symbol, positionSide, remainingQuantity, currentSL); err != nil {
				log.Printf("  ⚠️ 同步止损失败 (请手动检查): %v", err)
			}
		}
		// 2. 同步止盈
		if currentTP > 0 {
			log.Printf("  🔄 P16: 同步止盈单 (数量 %.4f @ %.4f)...", remainingQuantity, currentTP)
			de.trader.CancelTakeProfitOrders(decision.Symbol) // 先取消旧单
			if err := de.trader.SetTakeProfit(decision.Symbol, positionSide, remainingQuantity, currentTP); err != nil {
				log.Printf("  ⚠️ 同步止盈失败 (请手动检查): %v", err)
			}
		}
	}

	// 4. Record Trade (Treat Partial Profit Taking as a "Win" update?)
	// If it's partial, we might not want to reset the streak completely, or maybe we do.
	// Let's assume ANY profit taking is good.
	// Use simplified profitability check since entryPrice is available from targetPosition context if we extracted it.
	// But targetPosition is generic map.
	isProfitable := false
	if ePrice, ok := targetPosition["entryPrice"].(float64); ok {
		if positionSide == "LONG" {
			isProfitable = marketData.CurrentPrice > ePrice
		} else {
			isProfitable = marketData.CurrentPrice < ePrice
		}
	}
	de.contextBuilder.RecordTrade(true, isProfitable)

	return nil
}

// SortDecisionsByPriority 对决策排序：先平仓，再开仓，最后hold/wait
// 这样可以避免换仓时仓位叠加超限
func SortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// 定义优先级
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short", "partial_close":
			return 1 // 最高优先级：先平仓（包括部分平仓）
		case "update_stop_loss", "update_take_profit":
			return 2 // 调整持仓止盈止损
		case "open_long", "open_short":
			return 3 // 次优先级：后开仓
		case "hold", "wait":
			return 4 // 最低优先级：观望
		default:
			return 999 // 未知动作放最后
		}
	}

	// 复制决策列表
	sorted := make([]decision.Decision, len(decisions))
	copy(sorted, decisions)

	// 按优先级排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// FilterConflictingDecisions 过滤冲突决策：如果同一币种在同一周期内既有开仓又有平仓，只保留平仓
// 这样可以防止 AI 在同一周期内给出 open_long 和 close_long 导致开仓后立即平仓
// ⚠️ 新增：如果 AI 试图对已有持仓的币种重复开仓，自动转换为 hold
func FilterConflictingDecisions(decisions []decision.Decision, currentPositions []map[string]interface{}) []decision.Decision {
	if len(decisions) == 0 {
		return decisions
	}

	// 统计每个币种的操作类型
	symbolActions := make(map[string]map[string]bool) // symbol -> action -> exists
	for _, d := range decisions {
		if symbolActions[d.Symbol] == nil {
			symbolActions[d.Symbol] = make(map[string]bool)
		}
		symbolActions[d.Symbol][d.Action] = true
	}

	// 检查当前持仓状态
	hasPosition := make(map[string]string) // symbol -> side (long/short)
	for _, pos := range currentPositions {
		if symbol, ok := pos["symbol"].(string); ok {
			if side, ok := pos["side"].(string); ok {
				hasPosition[symbol] = side
			}
		}
	}

	// 过滤冲突决策
	filtered := []decision.Decision{}
	for _, d := range decisions {
		actions := symbolActions[d.Symbol]

		// ⚠️ 新增逻辑：检查是否对已有持仓的同方向币种发出重复开仓
		currentSide, hasCurrentPos := hasPosition[d.Symbol]
		if hasCurrentPos {
			if (d.Action == "open_long" && currentSide == "long") ||
				(d.Action == "open_short" && currentSide == "short") {
				// 自动转换为 hold（而不是拒绝执行）
				log.Printf("⚠️ 自动转换决策: %s %s → hold (已有 %s 持仓，禁止重复开仓)",
					d.Symbol, d.Action, currentSide)
				holdDecision := d
				holdDecision.Action = "hold"
				holdDecision.Reasoning = fmt.Sprintf("[系统自动转换] 原决策为%s，但已有%s持仓，自动转为hold。原因: %s",
					d.Action, currentSide, d.Reasoning)
				filtered = append(filtered, holdDecision)
				continue
			}
		}

		// 检查是否有冲突：同一币种既有开仓又有平仓
		hasOpenLong := actions["open_long"]
		hasCloseLong := actions["close_long"]
		hasOpenShort := actions["open_short"]
		hasCloseShort := actions["close_short"]

		// 如果同一币种在同一周期内既有开仓又有平仓
		if (hasOpenLong && hasCloseLong) || (hasOpenShort && hasCloseShort) {
			// 如果当前没有持仓，过滤掉平仓决策（因为平仓会失败），保留开仓决策
			if !hasCurrentPos {
				// 没有持仓，过滤掉平仓决策
				if d.Action == "close_long" || d.Action == "close_short" {
					log.Printf("⚠️ 过滤冲突决策: %s %s (当前无持仓，跳过平仓，保留开仓)", d.Symbol, d.Action)
					continue
				}
			} else {
				// 有持仓，过滤掉开仓决策（因为这是换仓，应该先平后开，但我们已经排序了）
				// 但为了安全，如果同一周期内既有开仓又有平仓，我们只保留平仓
				if d.Action == "open_long" || d.Action == "open_short" {
					// 检查方向是否匹配
					expectedSide := "long"
					if d.Action == "open_short" {
						expectedSide = "short"
					}
					if currentSide == expectedSide {
						log.Printf("⚠️ 过滤冲突决策: %s %s (当前已有 %s 持仓，同一周期内已有平仓，跳过开仓)",
							d.Symbol, d.Action, currentSide)
						continue
					}
				}
			}
		}

		filtered = append(filtered, d)
	}

	return filtered
}

// EmergencyClosePosition 紧急平仓函数
func (de *DecisionExecutor) EmergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := de.trader.CloseLong(symbol, 0) // 0 = 全部平仓
		if err != nil {
			return err
		}
		log.Printf("✅ 紧急平多仓成功，订单ID: %v", order["orderId"])
	case "short":
		order, err := de.trader.CloseShort(symbol, 0) // 0 = 全部平仓
		if err != nil {
			return err
		}
		log.Printf("✅ 紧急平空仓成功，订单ID: %v", order["orderId"])
	default:
		return fmt.Errorf("未知的持仓方向: %s", side)
	}

	return nil
}
