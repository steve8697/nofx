package market

import (
	"math"
)

// CVDData holds Cumulative Volume Delta information
type CVDData struct {
	CurrentCVD      float64   `json:"current_cvd"`
	Divergence      string    `json:"divergence"` // "Bullish", "Bearish", "None"
	CDVSeries       []float64 `json:"cvd_series"`
	DeltaSeries     []float64 `json:"delta_series"`
	AbsorptionState string    `json:"absorption_state"` // "Absorption", "Exhaustion", "None"
}

// CalculateCVD computes CVD and detects divergences
// Formula: Delta = BuyVol - SellVol = 2*TakerBuyBaseVol - TotalVol
func CalculateCVD(klines []Kline) *CVDData {
	if len(klines) < 20 {
		return nil
	}

	cvdSeries := make([]float64, len(klines))
	deltaSeries := make([]float64, len(klines))
	runningCVD := 0.0

	for i, k := range klines {
		buyVol := k.TakerBuyBaseVolume
		sellVol := k.Volume - buyVol
		delta := buyVol - sellVol

		runningCVD += delta
		cvdSeries[i] = runningCVD
		deltaSeries[i] = delta
	}

	divergence := detectCVDDivergence(klines, cvdSeries)
	absorption := detectAbsorption(klines, deltaSeries)

	return &CVDData{
		CurrentCVD:      runningCVD,
		Divergence:      divergence,
		CDVSeries:       cvdSeries,
		DeltaSeries:     deltaSeries,
		AbsorptionState: absorption,
	}
}

// detectCVDDivergence checks for Price vs CVD divergence
func detectCVDDivergence(klines []Kline, cvd []float64) string {
	if len(klines) < 10 {
		return "None"
	}

	// Simple validation: comparing recent trend vs CVD trend

	// Bullish Divergence (Absorption): Price Lower Low, CVD Higher Low
	// Bearish Divergence (Exhaustion): Price Higher High, CVD Lower High

	// Let's use a simpler heuristic for now: comparing the last pivot
	// If Price is making a new Low but CVD is NOT, that's Bullish Div (Absorption).

	lastPrice := klines[len(klines)-1].Close
	prevPrice := klines[len(klines)-5].Close

	lastCVD := cvd[len(cvd)-1]
	prevCVD := cvd[len(cvd)-5]

	// Bullish Divergence: Price Drop, CVD Rise (Strong Buying into dip)
	if lastPrice < prevPrice && lastCVD > prevCVD {
		return "Bullish (Absorption)"
	}

	// Bearish Divergence: Price Rise, CVD Drop (Strong Selling into pump)
	if lastPrice > prevPrice && lastCVD < prevCVD {
		return "Bearish (Exhaustion)"
	}

	return "None"
}

// detectAbsorption identifies high volume nodes with little price movement
func detectAbsorption(klines []Kline, deltas []float64) string {
	if len(klines) < 3 {
		return "None"
	}

	lastIndex := len(klines) - 1
	k := klines[lastIndex]
	delta := deltas[lastIndex]

	avgVol := 0.0
	for i := 1; i <= 5; i++ {
		if lastIndex-i >= 0 {
			avgVol += klines[lastIndex-i].Volume
		}
	}
	avgVol /= 5.0

	// Absorption Long: Price is near Low, High Sell Delta, but Candle closes high?
	// or Price Stalls (Doji) + Massive Volume + Negative Delta (Limit Buys eating Sells)

	isBodySmall := math.Abs(k.Close-k.Open) < (k.High-k.Low)*0.3
	isHighVol := k.Volume > avgVol*1.5

	if isBodySmall && isHighVol {
		if delta < 0 { // Heavy Selling absorbed
			return "Potential Accumulation (Limit Buys)"
		}
		if delta > 0 { // Heavy Buying absorbed
			return "Potential Distribution (Limit Sells)"
		}
	}

	return "None"
}
