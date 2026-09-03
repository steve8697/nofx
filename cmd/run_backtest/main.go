package main

import (
	"fmt"
	"log"
	"aetheris/backtest"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env if present
	_ = godotenv.Load()

	config := backtest.RunConfig{
		ID:             "verify_mock_001",
		Strategy:       "test",
		Symbol:         "BTCUSDT",
		Timeframe:      "15m",
		InitialBalance: 10000,
		Leverage:       5,
		Description:    "Mock Verification Run",
		Status:         "pending",
		StartTime:      time.Now().Unix(),
		Limit:          3,
	}

	dataFilePath := "data/BTCUSDT_15m.json"

	fmt.Println("🚀 Initializing Backtest Engine (Real AI Mode)...")
	engine, err := backtest.NewBacktestEngine(config, dataFilePath)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	fmt.Println("🏃 Running Backtest...")
	startTime := time.Now()

	result, err := engine.Run()
	if err != nil {
		log.Fatalf("Backtest failed: %v", err)
	}

	duration := time.Since(startTime)

	fmt.Printf("\n✅ Backtest Completed in %v\n", duration)
	fmt.Printf("Total PnL: %.2f (%.2f%%)\n", result.TotalPnL, result.TotalPnLPct)
	fmt.Printf("Trades: %d\n", len(result.Trades))

	if len(result.Trades) > 0 {
		fmt.Println("--- Recent Trades ---")
		for i, t := range result.Trades {
			if i >= 5 {
				break
			}
			fmt.Printf("%s %s PnL: %.2f\n", t.Direction, t.Symbol, t.PnL)
		}
	}
}
