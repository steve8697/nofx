package market

import (
	"time"
)

// SMCData Holds detected refined structures
type SMCData struct {
	OrderBlocks []OrderBlock    `json:"order_blocks"`
	FVGs        []FairValueGap  `json:"fvgs"`
	Structure   MarketStructure `json:"structure"`
}

// OrderBlock represents a supply or demand zone
type OrderBlock struct {
	Type     string    `json:"type"` // "Bullish" (Demand) or "Bearish" (Supply)
	Top      float64   `json:"top"`
	Bottom   float64   `json:"bottom"`
	Time     time.Time `json:"time"`
	Strength string    `json:"strength"` // "Standard", "Strong" (Breaks Structure)
}

// FairValueGap represents a price imbalance
type FairValueGap struct {
	Type   string    `json:"type"` // "Bullish" or "Bearish"
	Top    float64   `json:"top"`
	Bottom float64   `json:"bottom"`
	Time   time.Time `json:"time"`
}

// MarketStructure Swing Highs/Lows and Trend
type MarketStructure struct {
	Trend            string     `json:"trend"` // "Bullish", "Bearish", "Neutral"
	LastSwingHigh    SwingPoint `json:"last_swing_high"`
	LastSwingLow     SwingPoint `json:"last_swing_low"`
	BreakOfStructure string     `json:"bos"` // "Bullish", "Bearish", "None"
}

// SwingPoint represents a local high or low
type SwingPoint struct {
	Price float64   `json:"price"`
	Time  time.Time `json:"time"`
}

// CalculateSMC performs SMC analysis on the given klines
func CalculateSMC(klines []Kline) *SMCData {
	if len(klines) < 50 {
		return &SMCData{
			Structure: MarketStructure{Trend: "Neutral"},
		}
	}

	swingHighs, swingLows := detectSwingPoints(klines, 5) // Fractal 5 (2 left, 2 right)
	fvgs := detectFVGs(klines)
	orderBlocks := detectOrderBlocks(klines)

	structure := analyzeMarketStructure(klines, swingHighs, swingLows)

	return &SMCData{
		OrderBlocks: orderBlocks,
		FVGs:        fvgs,
		Structure:   structure,
	}
}

// detectSwingPoints finds local highs and lows using fractal logic
func detectSwingPoints(klines []Kline, period int) ([]SwingPoint, []SwingPoint) {
	var highs []SwingPoint
	var lows []SwingPoint

	lookback := period / 2 // e.g., 5 -> 2

	for i := lookback; i < len(klines)-lookback; i++ {
		isHigh := true
		isLow := true

		// Check left and right neighbors
		for j := 1; j <= lookback; j++ {
			if klines[i-j].High > klines[i].High || klines[i+j].High > klines[i].High {
				isHigh = false
			}
			if klines[i-j].Low < klines[i].Low || klines[i+j].Low < klines[i].Low {
				isLow = false
			}
		}

		if isHigh {
			highs = append(highs, SwingPoint{
				Price: klines[i].High,
				Time:  time.UnixMilli(klines[i].CloseTime),
			})
		}
		if isLow {
			lows = append(lows, SwingPoint{
				Price: klines[i].Low,
				Time:  time.UnixMilli(klines[i].CloseTime),
			})
		}
	}
	return highs, lows
}

// detectFVGs identifies Fair Value Gaps
func detectFVGs(klines []Kline) []FairValueGap {
	var fvgs []FairValueGap

	// Scan back 50 candles
	start := len(klines) - 50
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines)-2; i++ {
		// Bullish FVG: Candle 1 High < Candle 3 Low
		if klines[i+2].Low > klines[i].High {
			gapSize := klines[i+2].Low - klines[i].High
			if gapSize > klines[i].Close*0.001 { // Ignore micro gaps (<0.1%)
				fvgs = append(fvgs, FairValueGap{
					Type:   "Bullish",
					Top:    klines[i+2].Low,
					Bottom: klines[i].High,
					Time:   time.UnixMilli(klines[i+1].CloseTime),
				})
			}
		}

		// Bearish FVG: Candle 1 Low > Candle 3 High
		if klines[i+2].High < klines[i].Low {
			gapSize := klines[i].Low - klines[i+2].High
			if gapSize > klines[i].Close*0.001 { // Ignore micro gaps (<0.1%)
				fvgs = append(fvgs, FairValueGap{
					Type:   "Bearish",
					Top:    klines[i].Low,
					Bottom: klines[i+2].High,
					Time:   time.UnixMilli(klines[i+1].CloseTime),
				})
			}
		}
	}
	return fvgs
}

// detectOrderBlocks identifies potential supply/demand zones
// Simplified logic: The last contrary candle before a strong move that breaks structure/creates FVG
func detectOrderBlocks(klines []Kline) []OrderBlock {
	var obs []OrderBlock
	if len(klines) < 10 {
		return obs
	}

	// Heuristic: Look for large impulse candles
	start := len(klines) - 50
	if start < 1 {
		start = 1
	}

	for i := start; i < len(klines)-1; i++ {
		candle := klines[i]
		nextCandle := klines[i+1]

		isBullish := candle.Close > candle.Open
		nextIsMegaBullish := nextCandle.Close > nextCandle.Open && (nextCandle.Close-nextCandle.Open) > (candle.High-candle.Low)*2

		// Potential Bullish OB: Bearish candle followed by explosive Bullish move
		if !isBullish && nextIsMegaBullish {
			obs = append(obs, OrderBlock{
				Type:     "Bullish",
				Top:      candle.High,
				Bottom:   candle.Low,
				Time:     time.UnixMilli(candle.CloseTime),
				Strength: "Standard",
			})
		}

		nextIsMegaBearish := nextCandle.Close < nextCandle.Open && (nextCandle.Open-nextCandle.Close) > (candle.High-candle.Low)*2
		// Potential Bearish OB: Bullish candle followed by explosive Bearish move
		if isBullish && nextIsMegaBearish {
			obs = append(obs, OrderBlock{
				Type:     "Bearish",
				Top:      candle.High,
				Bottom:   candle.Low,
				Time:     time.UnixMilli(candle.CloseTime),
				Strength: "Standard",
			})
		}
	}

	// Keep only the latest 5
	if len(obs) > 5 {
		obs = obs[len(obs)-5:]
	}

	return obs
}

// analyzeMarketStructure Determines trend based on swings
func analyzeMarketStructure(klines []Kline, highs, lows []SwingPoint) MarketStructure {
	ms := MarketStructure{
		Trend:            "Neutral",
		BreakOfStructure: "None",
	}

	if len(highs) < 2 || len(lows) < 2 {
		return ms
	}

	lastHigh := highs[len(highs)-1]
	prevHigh := highs[len(highs)-2]
	lastLow := lows[len(lows)-1]
	prevLow := lows[len(lows)-2]

	ms.LastSwingHigh = lastHigh
	ms.LastSwingLow = lastLow

	// Higher Highs & Higher Lows = Bullish
	if lastHigh.Price > prevHigh.Price && lastLow.Price > prevLow.Price {
		ms.Trend = "Bullish"
	}
	// Lower Highs & Lower Lows = Bearish
	if lastHigh.Price < prevHigh.Price && lastLow.Price < prevLow.Price {
		ms.Trend = "Bearish"
	}

	// Check for immediate BOS (Break of Structure) or CHoCH
	currentPrice := klines[len(klines)-1].Close
	if ms.Trend == "Bullish" {
		if currentPrice > lastHigh.Price {
			ms.BreakOfStructure = "Bullish" // 順勢多頭突破 BOS
		} else if currentPrice < lastLow.Price {
			ms.BreakOfStructure = "Bearish CHoCH" // 多轉空反轉
		}
	} else if ms.Trend == "Bearish" {
		if currentPrice < lastLow.Price {
			ms.BreakOfStructure = "Bearish" // 順勢空頭突破 BOS
		} else if currentPrice > lastHigh.Price {
			ms.BreakOfStructure = "Bullish CHoCH" // 空轉多反轉
		}
	} else {
		if currentPrice > lastHigh.Price {
			ms.BreakOfStructure = "Bullish"
		} else if currentPrice < lastLow.Price {
			ms.BreakOfStructure = "Bearish"
		}
	}

	return ms
}
