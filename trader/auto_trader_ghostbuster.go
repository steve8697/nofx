package trader

import (
	"log"
	"time"
)

// startGhostbusterMonitor 启动幽灵订单清理（无持仓但有挂单）
func (at *AutoTrader) startGhostbusterMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()
		ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次幽灵订单
		defer ticker.Stop()

		log.Println("👻 Ghostbuster: 启动幽灵订单监控（5分钟周期）")

		for {
			select {
			case <-ticker.C:
				at.cleanupOrphanedOrders()
			case <-at.stopMonitorCh:
				log.Println("⏹ Ghostbuster: 停止幽灵订单监控")
				return
			}
		}
	}()
}

// cleanupOrphanedOrders 清理幽灵订单（有挂单但无持仓）
func (at *AutoTrader) cleanupOrphanedOrders() {
	// 1. 获取所有持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("❌ Ghostbuster: 获取持仓失败: %v", err)
		return
	}

	// 记录有持仓的 Symbol
	activePositions := make(map[string]bool)
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		// 检查持仓数量
		if amtStr, ok := pos["positionAmt"].(string); ok {
			// 如果是字符串（binance API 有时返回字符串）
			if amtStr != "0" && amtStr != "0.0" {
				activePositions[symbol] = true
			}
		} else if amt, ok := pos["positionAmt"].(float64); ok {
			// 如果是 float
			if amt != 0 {
				activePositions[symbol] = true
			}
		}
	}

	// 2. 遍历所有交易币种
	coins := at.config.TradingCoins
	if len(coins) == 0 {
		coins = at.config.DefaultCoins
	}

	for _, symbol := range coins {
		// 如果该币种有持仓，则跳过（挂单可能是正常的止损止盈）
		if activePositions[symbol] {
			continue
		}

		// 获取挂单
		// 注意：如果 OpenLong 是 Market Order，则平时不应该有 Open Orders 除非是 Limit
		// 我们的系统主要使用 Market Entry，所以无持仓时的挂单几乎都是残留的 SL/TP
		orders, err := at.trader.GetOpenOrders(symbol)
		if err != nil {
			continue
		}

		if len(orders) > 0 {
			log.Printf("👻 Ghostbuster: 发现 %s 有 %d 个幽灵订单 (无持仓)，执行清理...", symbol, len(orders))
			if err := at.trader.CancelAllOrders(symbol); err != nil {
				log.Printf("❌ Ghostbuster: 清理失败 %s: %v", symbol, err)
			} else {
				log.Printf("✅ Ghostbuster: 已成功清除 %s 的幽灵订单", symbol)
			}
		}
	}
}
