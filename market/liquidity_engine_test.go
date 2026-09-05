package market

import (
	"testing"
	"time"
)

func TestLiquidityEngineWickCalculation(t *testing.T) {
	le := &LiquidityEngine{
		Clusters: make(map[string][]LiquidityCluster),
	}

	nowMs := time.Now().UnixMilli()

	// 1. 暴跌大陰線（光頭光腳大陰線）
	// Open=100, Close=90, High=100, Low=90
	// 舊公式中 (High - Close) > (Close - Low)*3 -> (10 > 0*3) -> 誤判為長上影線！
	// 新公式必須正確識別此為實體暴跌，絕對不可判定為長上影線！
	bearishMarubozu := Kline{
		Open:      100.0,
		Close:     90.0,
		High:      100.0,
		Low:       90.0,
		Volume:    5000.0,
		CloseTime: nowMs,
	}

	le.Update("BTCUSDT", bearishMarubozu, nil)
	clusters := le.Clusters["BTCUSDT"]

	for _, c := range clusters {
		if c.Type == "Short Liquidation" {
			t.Fatalf("嚴重錯誤：暴跌大陰線被誤判為長上影線並生成了 Short Liquidation 簇: %+v", c)
		}
	}

	// 2. 真實長上影線（流星線 Shooting Star / 倒錘頭）
	// Open=90, Close=91, High=100, Low=90 (上影線=9, 實體=1, 下影線=0)
	shootingStar := Kline{
		Open:      90.0,
		Close:     91.0,
		High:      100.0,
		Low:       90.0,
		Volume:    5000.0,
		CloseTime: nowMs,
	}

	le.Update("ETHUSDT", shootingStar, nil)
	ethClusters := le.Clusters["ETHUSDT"]
	foundShortLiq := false
	for _, c := range ethClusters {
		if c.Type == "Short Liquidation" && c.Price == 100.0 {
			foundShortLiq = true
			break
		}
	}
	if !foundShortLiq {
		t.Fatalf("長上影線應該正確生成 Short Liquidation 簇，但未找到！簇列表: %+v", ethClusters)
	}

	// 3. 真實長下影線（錘頭線 Hammer）
	// Open=99, Close=100, High=100, Low=90 (上影線=0, 實體=1, 下影線=9)
	hammer := Kline{
		Open:      99.0,
		Close:     100.0,
		High:      100.0,
		Low:       90.0,
		Volume:    5000.0,
		CloseTime: nowMs,
	}

	le.Update("SOLUSDT", hammer, nil)
	solClusters := le.Clusters["SOLUSDT"]
	foundLongLiq := false
	for _, c := range solClusters {
		if c.Type == "Long Liquidation" && c.Price == 90.0 {
			foundLongLiq = true
			break
		}
	}
	if !foundLongLiq {
		t.Fatalf("長下影線應該正確生成 Long Liquidation 簇，但未找到！簇列表: %+v", solClusters)
	}
}
