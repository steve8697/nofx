package trader

// GetOrderProtection 获取当前持仓的止损止盈价格 (Stub for Hyperliquid)
func (t *HyperliquidTrader) GetOrderProtection(symbol string, positionSide string) (float64, float64, error) {
	// 暂未支持 Hyperliquid 的止损止盈查询
	return 0, 0, nil
}
