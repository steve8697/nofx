package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"aetheris/market"
	"os"
	"time"
)

func main() {
	symbol := "BTCUSDT"
	timeframe := "15m"
	count := 1000
	startPrice := 50000.0

	fmt.Printf("Generating %d candles for %s %s...\n", count, symbol, timeframe)

	klines := make([]market.Kline, count)
	currentPrice := startPrice
	startTime := time.Now().Add(-time.Duration(count) * 15 * time.Minute)

	rand.Seed(time.Now().UnixNano())

	for i := 0; i < count; i++ {
		// Mock price movement (random walk)
		changePct := (rand.Float64() - 0.5) * 0.02 // +/- 1%
		closePrice := currentPrice * (1 + changePct)
		high := math.Max(currentPrice, closePrice) * (1 + rand.Float64()*0.005)
		low := math.Min(currentPrice, closePrice) * (1 - rand.Float64()*0.005)

		klines[i] = market.Kline{
			OpenTime:  startTime.Add(time.Duration(i) * 15 * time.Minute).UnixMilli(),
			Open:      currentPrice,
			High:      high,
			Low:       low,
			Close:     closePrice,
			Volume:    100 + rand.Float64()*1000,
			CloseTime: startTime.Add(time.Duration(i+1) * 15 * time.Minute).UnixMilli(),
		}

		currentPrice = closePrice
	}

	// Create output dir if not exists
	os.MkdirAll("data", 0755)

	filename := fmt.Sprintf("data/%s_%s.json", symbol, timeframe)
	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(klines); err != nil {
		panic(err)
	}

	fmt.Printf("✓ Data saved to %s\n", filename)
}
