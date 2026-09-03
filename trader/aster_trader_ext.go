package trader

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GetOrderProtection 获取当前持仓的止损止盈价格 (Aster 实现)
func (t *AsterTrader) GetOrderProtection(symbol string, positionSide string) (float64, float64, error) {
	// 获取该币种的所有未完成订单
	params := map[string]interface{}{
		"symbol": symbol,
	}

	body, err := t.request("GET", "/fapi/v3/openOrders", params)
	if err != nil {
		return 0, 0, fmt.Errorf("获取挂单失败: %w", err)
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(body, &orders); err != nil {
		return 0, 0, fmt.Errorf("解析挂单失败: %w", err)
	}

	var stopLoss, takeProfit float64
	positionSide = strings.ToUpper(positionSide)

	for _, order := range orders {
		orderType, _ := order["type"].(string)
		orderSide, _ := order["side"].(string) // BUY or SELL

		// 检查方向是否匹配
		// Long持仓 -> 止损/止盈单必须是 SELL
		// Short持仓 -> 止损/止盈单必须是 BUY
		if positionSide == "LONG" && orderSide != "SELL" {
			continue
		}
		if positionSide == "SHORT" && orderSide != "BUY" {
			continue
		}

		stopPriceStr, ok := order["stopPrice"].(string)
		if !ok {
			continue
		}
		stopPrice, _ := strconv.ParseFloat(stopPriceStr, 64)

		// 检查类型
		if orderType == "STOP_MARKET" || orderType == "STOP" {
			if stopPrice > 0 {
				stopLoss = stopPrice
			}
		}

		if orderType == "TAKE_PROFIT_MARKET" || orderType == "TAKE_PROFIT" {
			if stopPrice > 0 {
				takeProfit = stopPrice
			}
		}
	}

	return stopLoss, takeProfit, nil
}
