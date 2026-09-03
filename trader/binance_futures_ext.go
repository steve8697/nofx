package trader

import (
	"context"
	"strconv"
	"strings"

	"github.com/adshao/go-binance/v2/futures"
)

// GetOrderProtection 获取当前持仓的止损止盈价格
func (t *FuturesTrader) GetOrderProtection(symbol string, positionSide string) (float64, float64, error) {
	// 获取所有挂单
	orders, err := t.client.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, 0, err
	}

	var stopLoss, takeProfit float64
	positionSide = strings.ToUpper(positionSide)

	for _, order := range orders {
		// 过滤非目标方向的订单
		// 对于双向持仓模式：
		// LONG持仓的止损止盈单方向必须是 SELL
		// SHORT持仓的止损止盈单方向必须是 BUY
		// 并没有直接的 PositionSide 字段来区分是哪个仓位的止损（除非开了双向模式且订单上有 positionSide 字段？）
		// 币安 API 返回的 Order 结构体包含 PositionSide 字段！
		// 检查 order.PositionSide

		orderPosSide := string(order.PositionSide) // LONG, SHORT, or BOTH
		if orderPosSide != positionSide {
			continue
		}

		// 解析止损价格
		if order.Type == futures.OrderTypeStopMarket || order.Type == futures.OrderTypeStop {
			// 转换为 float64
			price, _ := strconv.ParseFloat(order.StopPrice, 64)
			if price > 0 {
				// 如果有多个止损单，取最新的？或者最极端的？
				// 正常情况应该只有一个
				stopLoss = price
			}
		}

		// 解析止盈价格
		if order.Type == futures.OrderTypeTakeProfitMarket || order.Type == futures.OrderTypeTakeProfit {
			price, _ := strconv.ParseFloat(order.StopPrice, 64)
			if price > 0 {
				takeProfit = price
			}
		}
	}

	return stopLoss, takeProfit, nil
}
