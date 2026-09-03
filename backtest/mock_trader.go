package backtest

import (
	"fmt"
	"aetheris/utils"
	"sync"
	"time"
)

// MockTrader 实现了 trader.Trader 接口，用于回测
// 它不会真正的下单，而是记录订单信息并跟踪虚拟账户状态
type MockTrader struct {
	mu           sync.RWMutex
	Balance      map[string]interface{}
	Positions    map[string]map[string]interface{} // symbol -> position info
	Orders       []MockOrder
	MarketPrices map[string]float64 // 外部注入的即时价格 (用于模拟 GetMarketPrice)
	TimeProvider utils.TimeProvider
}

type MockOrder struct {
	Time       time.Time
	Symbol     string
	Side       string // "LONG" or "SHORT" action (OPEN_LONG, CLOSE_SHORT...)
	Action     string
	Quantity   float64
	Price      float64
	Leverage   int
	StopLoss   float64
	TakeProfit float64
}

func NewMockTrader(initialUSDT float64, timeProvider utils.TimeProvider) *MockTrader {
	return &MockTrader{
		Balance: map[string]interface{}{
			"total_equity":      initialUSDT,
			"available_balance": initialUSDT,
			"total_pnl":         0.0,
			"total_pnl_pct":     0.0,
			"position_count":    0,
			"margin_used_pct":   0.0,
		},
		Positions:    make(map[string]map[string]interface{}),
		Orders:       make([]MockOrder, 0),
		MarketPrices: make(map[string]float64),
		TimeProvider: timeProvider,
	}
}

// SetCurrentPrice 更新模拟器中的当前市场价格
func (m *MockTrader) SetCurrentPrice(symbol string, price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MarketPrices[symbol] = price
}

// --- Trader Interface Implementation ---

func (m *MockTrader) GetBalance() (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// 返回深拷贝以防止外部修改
	copy := make(map[string]interface{})
	for k, v := range m.Balance {
		copy[k] = v
	}
	return copy, nil
}

func (m *MockTrader) GetPositions() ([]map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var positions []map[string]interface{}
	for _, pos := range m.Positions {
		copy := make(map[string]interface{})
		for k, v := range pos {
			copy[k] = v
		}
		positions = append(positions, copy)
	}
	return positions, nil
}

func (m *MockTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	price := m.MarketPrices[symbol]
	if price == 0 {
		return nil, fmt.Errorf("mock: no price for %s", symbol)
	}

	m.Orders = append(m.Orders, MockOrder{
		Time:     m.TimeProvider.Now(),
		Symbol:   symbol,
		Action:   "OPEN_LONG",
		Side:     "LONG",
		Quantity: quantity,
		Price:    price,
		Leverage: leverage,
	})

	// 更新虚拟持仓 (简化逻辑：覆盖或累加)
	// 回测引擎会处理 PnL，这里主要是为了让 AutoTrader 看到持仓
	pos := map[string]interface{}{
		"symbol":            symbol,
		"side":              "long",
		"position_amt":      quantity,
		"entry_price":       price,
		"mark_price":        price,
		"unrealized_profit": 0.0,
		"leverage":          leverage,
		"stop_loss":         0.0,
		"take_profit":       0.0,
	}
	m.Positions[symbol] = pos

	return map[string]interface{}{"orderId": "mock_long_1"}, nil
}

func (m *MockTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	price := m.MarketPrices[symbol]
	if price == 0 {
		return nil, fmt.Errorf("mock: no price for %s", symbol)
	}

	m.Orders = append(m.Orders, MockOrder{
		Time:     m.TimeProvider.Now(),
		Symbol:   symbol,
		Action:   "OPEN_SHORT",
		Side:     "SHORT",
		Quantity: quantity,
		Price:    price,
		Leverage: leverage,
	})

	pos := map[string]interface{}{
		"symbol":            symbol,
		"side":              "short",
		"position_amt":      quantity,
		"entry_price":       price,
		"mark_price":        price,
		"unrealized_profit": 0.0,
		"leverage":          leverage,
		"stop_loss":         0.0,
		"take_profit":       0.0,
	}
	m.Positions[symbol] = pos

	return map[string]interface{}{"orderId": "mock_short_1"}, nil
}

func (m *MockTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pos, ok := m.Positions[symbol]
	if !ok {
		return nil, fmt.Errorf("no position to close for %s", symbol)
	}

	currentQty, _ := pos["position_amt"].(float64)

	// Handle partial close
	if quantity < currentQty {
		pos["position_amt"] = currentQty - quantity
	} else {
		delete(m.Positions, symbol)
	}

	price := m.MarketPrices[symbol]
	m.Orders = append(m.Orders, MockOrder{
		Time:     m.TimeProvider.Now(),
		Symbol:   symbol,
		Action:   "CLOSE_LONG",
		Quantity: quantity,
		Price:    price,
	})
	return map[string]interface{}{"orderId": "mock_close_long"}, nil
}

func (m *MockTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pos, ok := m.Positions[symbol]
	if !ok {
		return nil, fmt.Errorf("no position to close for %s", symbol)
	}

	currentQty, _ := pos["position_amt"].(float64)

	// Handle partial close
	if quantity < currentQty {
		pos["position_amt"] = currentQty - quantity
	} else {
		delete(m.Positions, symbol)
	}

	price := m.MarketPrices[symbol]
	// Short: buy to close
	m.Orders = append(m.Orders, MockOrder{
		Time:     m.TimeProvider.Now(),
		Symbol:   symbol,
		Action:   "CLOSE_SHORT",
		Quantity: quantity,
		Price:    price,
	})
	return map[string]interface{}{"orderId": "mock_close_short"}, nil
}

func (m *MockTrader) SetLeverage(symbol string, leverage int) error {
	return nil
}

func (m *MockTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}

func (m *MockTrader) GetMarketPrice(symbol string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	price, ok := m.MarketPrices[symbol]
	if !ok {
		return 0, fmt.Errorf("mock price not found for %s", symbol)
	}
	return price, nil
}

func (m *MockTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pos, ok := m.Positions[symbol]; ok {
		pos["stop_loss"] = stopPrice
	}
	return nil
}

func (m *MockTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pos, ok := m.Positions[symbol]; ok {
		pos["take_profit"] = takeProfitPrice
	}
	return nil
}

func (m *MockTrader) CancelStopLossOrders(symbol string) error {
	return nil
}

func (m *MockTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

func (m *MockTrader) CancelAllOrders(symbol string) error {
	return nil
}

func (m *MockTrader) CancelStopOrders(symbol string) error {
	return nil
}

func (m *MockTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return fmt.Sprintf("%.4f", quantity), nil
}

func (m *MockTrader) GetOrderProtection(symbol string, positionSide string) (float64, float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if pos, ok := m.Positions[symbol]; ok {
		sl, _ := pos["stop_loss"].(float64)
		tp, _ := pos["take_profit"].(float64)
		return sl, tp, nil
	}
	return 0, 0, fmt.Errorf("no position")
}

// GetOpenOrders 获取挂单 (Stub for MockTrader)
func (m *MockTrader) GetOpenOrders(symbol string) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

// GetUserTrades 获取用户成交历史 (Stub for MockTrader)
func (m *MockTrader) GetUserTrades(symbol string, limit int) ([]map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 简单的返回所有相关的 MockOrders 转换结果
	var trades []map[string]interface{}
	for _, order := range m.Orders {
		if order.Symbol == symbol {
			trade := map[string]interface{}{
				"symbol": order.Symbol,
				"price":  order.Price,
				"qty":    order.Quantity,
				"side":   order.Side,
				"time":   order.Time.UnixMilli(),
			}
			if order.Action == "CLOSE_LONG" || order.Action == "CLOSE_SHORT" {
				// 模拟 RealizedPnL (这就比较复杂了，暂且略过，或者简单的给个假数据)
				trade["realizedPnl"] = 0.0
			}
			trades = append(trades, trade)
		}
	}
	return trades, nil
}

// GetTradingFees 获取交易手续费率（回测使用币安费率）
// MockTrader: 假设 Maker 0.02%, Taker 0.05%
func (m *MockTrader) GetTradingFees() (makerFeeRate, takerFeeRate float64) {
	return 0.0002, 0.0005 // 与币安一致，回测时使用
}

// GetTradeHistory aggregates orders into trades
// NOTE: This is a simplified FIFO matcher.
func (m *MockTrader) GetTradeHistory() ([]TradeRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var trades []TradeRecord
	// Simplified matching: We assume 1 open matches 1 close for now, or we just list completed pairs.
	// Since MockTrader logic for Close is "close all", we can try to pair them.
	// A better way for simple backtest report is just list what happened.

	// Group by symbol
	ordersBySymbol := make(map[string][]MockOrder)
	for _, o := range m.Orders {
		ordersBySymbol[o.Symbol] = append(ordersBySymbol[o.Symbol], o)
	}

	for symbol, orders := range ordersBySymbol {
		var openOrder *MockOrder
		for _, o := range orders {
			if o.Action == "OPEN_LONG" || o.Action == "OPEN_SHORT" {
				openOrder = &o
			} else if (o.Action == "CLOSE_LONG" || o.Action == "CLOSE_SHORT") && openOrder != nil {
				// Match!
				direction := "LONG"
				grossPnL := 0.0

				// Calculate Fees (Taker fee assumption: 0.05% per side)
				feeRate := 0.0005
				entryFee := openOrder.Price * openOrder.Quantity * feeRate
				exitFee := o.Price * openOrder.Quantity * feeRate // Assuming close qty matches open for this simplified matcher
				totalFee := entryFee + exitFee

				if openOrder.Action == "OPEN_SHORT" {
					direction = "SHORT"
					// Short PnL: (Entry - Exit) * Qty
					grossPnL = (openOrder.Price - o.Price) * openOrder.Quantity
				} else {
					// Long PnL: (Exit - Entry) * Qty
					grossPnL = (o.Price - openOrder.Price) * openOrder.Quantity
				}

				netPnL := grossPnL - totalFee

				margin := (openOrder.Price * openOrder.Quantity) / float64(openOrder.Leverage)
				pnlPct := 0.0
				if margin > 0 {
					pnlPct = (netPnL / margin) * 100
				}

				trades = append(trades, TradeRecord{
					Symbol:     symbol,
					EntryTime:  openOrder.Time,
					ExitTime:   o.Time,
					EntryPrice: openOrder.Price,
					ExitPrice:  o.Price,
					Quantity:   openOrder.Quantity,
					PnL:        netPnL,
					Commission: totalFee,
					PnLPercent: pnlPct,
					Direction:  direction,
				})
				openOrder = nil
			}
		}
	}
	return trades, nil
}
