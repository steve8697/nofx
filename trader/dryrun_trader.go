package trader

import (
	"fmt"
	"log"
	"sync"
)

// DryRunTrader 包住真实交易所客户端：读操作照常，写操作只记日志。
// 用于单周期调试，物理上不会下单。
type DryRunTrader struct {
	inner Trader
	mu    sync.Mutex
	Calls []string
}

func NewDryRunTrader(inner Trader) *DryRunTrader {
	return &DryRunTrader{inner: inner}
}

func (d *DryRunTrader) note(op string) {
	d.mu.Lock()
	d.Calls = append(d.Calls, op)
	d.mu.Unlock()
	log.Printf("DRY-RUN skip write: %s", op)
}

func (d *DryRunTrader) GetBalance() (map[string]interface{}, error) {
	return d.inner.GetBalance()
}

func (d *DryRunTrader) GetPositions() ([]map[string]interface{}, error) {
	return d.inner.GetPositions()
}

func (d *DryRunTrader) GetOpenOrders(symbol string) ([]map[string]interface{}, error) {
	return d.inner.GetOpenOrders(symbol)
}

func (d *DryRunTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	d.note(fmt.Sprintf("OpenLong %s qty=%.6f lev=%d", symbol, quantity, leverage))
	return map[string]interface{}{"dry_run": true, "symbol": symbol}, nil
}

func (d *DryRunTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	d.note(fmt.Sprintf("OpenShort %s qty=%.6f lev=%d", symbol, quantity, leverage))
	return map[string]interface{}{"dry_run": true, "symbol": symbol}, nil
}

func (d *DryRunTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	d.note(fmt.Sprintf("CloseLong %s qty=%.6f", symbol, quantity))
	return map[string]interface{}{"dry_run": true, "symbol": symbol}, nil
}

func (d *DryRunTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	d.note(fmt.Sprintf("CloseShort %s qty=%.6f", symbol, quantity))
	return map[string]interface{}{"dry_run": true, "symbol": symbol}, nil
}

func (d *DryRunTrader) SetLeverage(symbol string, leverage int) error {
	d.note(fmt.Sprintf("SetLeverage %s %d", symbol, leverage))
	return nil
}

func (d *DryRunTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	d.note(fmt.Sprintf("SetMarginMode %s cross=%v", symbol, isCrossMargin))
	return nil
}

func (d *DryRunTrader) GetMarketPrice(symbol string) (float64, error) {
	return d.inner.GetMarketPrice(symbol)
}

func (d *DryRunTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	d.note(fmt.Sprintf("SetStopLoss %s %s qty=%.6f px=%.6f", symbol, positionSide, quantity, stopPrice))
	return nil
}

func (d *DryRunTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	d.note(fmt.Sprintf("SetTakeProfit %s %s qty=%.6f px=%.6f", symbol, positionSide, quantity, takeProfitPrice))
	return nil
}

func (d *DryRunTrader) CancelStopLossOrders(symbol string) error {
	d.note("CancelStopLossOrders " + symbol)
	return nil
}

func (d *DryRunTrader) CancelTakeProfitOrders(symbol string) error {
	d.note("CancelTakeProfitOrders " + symbol)
	return nil
}

func (d *DryRunTrader) GetUserTrades(symbol string, limit int) ([]map[string]interface{}, error) {
	return d.inner.GetUserTrades(symbol, limit)
}

func (d *DryRunTrader) CancelAllOrders(symbol string) error {
	d.note("CancelAllOrders " + symbol)
	return nil
}

func (d *DryRunTrader) CancelStopOrders(symbol string) error {
	d.note("CancelStopOrders " + symbol)
	return nil
}

func (d *DryRunTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return d.inner.FormatQuantity(symbol, quantity)
}

func (d *DryRunTrader) GetOrderProtection(symbol string, positionSide string) (float64, float64, error) {
	return d.inner.GetOrderProtection(symbol, positionSide)
}

func (d *DryRunTrader) GetTradingFees() (makerFeeRate, takerFeeRate float64) {
	return d.inner.GetTradingFees()
}

// WrapDryRun 把内部交易所客户端换成只记录写操作的包装。必须在 RunCycle 之前调用。
func (at *AutoTrader) WrapDryRun() *DryRunTrader {
	d := NewDryRunTrader(at.trader)
	at.SetTrader(d)
	return d
}
