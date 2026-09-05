package market

import (
	"testing"
	"time"
)

func TestAnalyzeMarketStructureBOSAndCHoCH(t *testing.T) {
	now := time.Now()

	// 1. 構建看漲趨勢擺動點 (Higher Highs & Higher Lows)
	bullishHighs := []SwingPoint{
		{Price: 100.0, Time: now.Add(-40 * time.Minute)},
		{Price: 110.0, Time: now.Add(-20 * time.Minute)}, // lastHigh = 110
	}
	bullishLows := []SwingPoint{
		{Price: 80.0, Time: now.Add(-50 * time.Minute)},
		{Price: 90.0, Time: now.Add(-30 * time.Minute)}, // lastLow = 90
	}

	// 1a. 突破前高 (115 > 110) -> 必須觸發 Bullish BOS (順勢結構突破)
	klinesBOS := []Kline{
		{Close: 115.0},
	}
	msBOS := analyzeMarketStructure(klinesBOS, bullishHighs, bullishLows)
	if msBOS.Trend != "Bullish" {
		t.Fatalf("預期趨勢為 Bullish，實際為 %s", msBOS.Trend)
	}
	if msBOS.BreakOfStructure != "Bullish" {
		t.Fatalf("突破前高預期 BOS 為 Bullish，實際為: %s", msBOS.BreakOfStructure)
	}

	// 1b. 跌破前低 (85 < 90) -> 必須觸發 Bearish CHoCH (趨勢轉變)
	klinesCHoCH := []Kline{
		{Close: 85.0},
	}
	msCHoCH := analyzeMarketStructure(klinesCHoCH, bullishHighs, bullishLows)
	if msCHoCH.BreakOfStructure != "Bearish CHoCH" {
		t.Fatalf("跌破前低預期 BOS 為 Bearish CHoCH，實際為: %s", msCHoCH.BreakOfStructure)
	}

	// 2. 構建看跌趨勢擺動點 (Lower Highs & Lower Lows)
	bearishHighs := []SwingPoint{
		{Price: 110.0, Time: now.Add(-40 * time.Minute)},
		{Price: 100.0, Time: now.Add(-20 * time.Minute)}, // lastHigh = 100
	}
	bearishLows := []SwingPoint{
		{Price: 90.0, Time: now.Add(-50 * time.Minute)},
		{Price: 80.0, Time: now.Add(-30 * time.Minute)}, // lastLow = 80
	}

	// 2a. 跌破前低 (75 < 80) -> 必須觸發 Bearish BOS (空頭順勢突破)
	klinesBearBOS := []Kline{
		{Close: 75.0},
	}
	msBearBOS := analyzeMarketStructure(klinesBearBOS, bearishHighs, bearishLows)
	if msBearBOS.Trend != "Bearish" {
		t.Fatalf("預期趨勢為 Bearish，實際為 %s", msBearBOS.Trend)
	}
	if msBearBOS.BreakOfStructure != "Bearish" {
		t.Fatalf("跌破前低預期 BOS 為 Bearish，實際為: %s", msBearBOS.BreakOfStructure)
	}

	// 2b. 突破前高 (105 > 100) -> 必須觸發 Bullish CHoCH (空轉多反轉)
	klinesBullCHoCH := []Kline{
		{Close: 105.0},
	}
	msBullCHoCH := analyzeMarketStructure(klinesBullCHoCH, bearishHighs, bearishLows)
	if msBullCHoCH.BreakOfStructure != "Bullish CHoCH" {
		t.Fatalf("突破前高預期 BOS 為 Bullish CHoCH，實際為: %s", msBullCHoCH.BreakOfStructure)
	}
}
