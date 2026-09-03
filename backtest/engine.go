package backtest

import (
	"fmt"
	"log"
	"aetheris/market"
	"aetheris/trader"
	"time"
)

// BacktestEngine orchestrates the simulation
type BacktestEngine struct {
	Config       RunConfig
	Trader       *trader.AutoTrader
	MockTrader   *MockTrader
	DataFeed     *DataFeed
	DataProvider *MockDataProvider
	TimeProvider *MockTimeProvider
	Equity       []EquityPoint
	Result       *BacktestResult
}

// NewBacktestEngine creates a new engine instance
func NewBacktestEngine(config RunConfig, dataFilePath string) (*BacktestEngine, error) {
	// 1. Load DataFeed
	feed, err := NewDataFeedFromFile(dataFilePath, config.Symbol, config.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("failed to load data feed: %w", err)
	}

	// 2. Initialize Mocks
	mockTimeProvider := &MockTimeProvider{} // Initialize TimeProvider first
	mockTrader := NewMockTrader(config.InitialBalance, mockTimeProvider)
	mockDataProvider := NewMockDataProvider()

	// 3. Configure AutoTrader
	// We need to map RunConfig to AutoTraderConfig
	// Note: API Keys are empty for safety.
	atConfig := trader.AutoTraderConfig{
		ID:              config.ID,
		Name:            "Backtest_" + config.ID,
		AIModel:         "mock", // Default to mock, override if keys present
		Exchange:        "backtest",
		InitialBalance:  config.InitialBalance,
		BTCETHLeverage:  config.Leverage,
		AltcoinLeverage: config.Leverage,
		TradingCoins:    []string{config.Symbol},
		ScanInterval:    1 * time.Second,
		DeepSeekKey:     config.DeepSeekKey,
		QwenKey:         config.QwenKey,
	}

	if config.DeepSeekKey != "" {
		atConfig.AIModel = "deepseek"
	} else if config.QwenKey != "" {
		atConfig.AIModel = "qwen"
	}

	// 4. Initialize AutoTrader
	// We pass mocks here.
	noOpSleep := func(d time.Duration) {}
	at, err := trader.NewAutoTrader(atConfig, nil, "backtest_user", mockDataProvider, mockTimeProvider, noOpSleep)
	if err != nil {
		return nil, fmt.Errorf("failed to create auto trader: %w", err)
	}
	// Inject MockTrader implementation into AutoTrader
	// (AutoTrader has a `trader` field, we need to ensure usage of MockTrader)
	// Currently NewAutoTrader creates a concrete trader based on config.
	// We might need a way to injecting the trader implementation OR use an existing one if supported.
	// Looking at NewAutoTrader logic:
	// if config.Exchange == "binance" { ... }
	// We need a way to support "backtest" exchange or inject the trader.

	// HACK: Since we cannot easily inject the Trader interface via NewAutoTrader (it creates it internally),
	// We will use a setter if available, OR we rely on the fact that we passed "backtest" exchange
	// and modifying NewAutoTrader to handle it, OR we directly set the field using reflection/helper.
	// BETTER APPROACH: Add SetTrader method to AutoTrader for testing/backtesting.

	at.SetTrader(mockTrader) // Need to implement this method on AutoTrader or use existing SetTrader if it exists.

	return &BacktestEngine{
		Config:       config,
		Trader:       at,
		MockTrader:   mockTrader,
		DataFeed:     feed,
		DataProvider: mockDataProvider,
		TimeProvider: mockTimeProvider,
		Equity:       make([]EquityPoint, 0),
		Result:       &BacktestResult{Config: config},
	}, nil
}

// Run executes the backtest
func (be *BacktestEngine) Run() (*BacktestResult, error) {
	log.Printf("🚀 Starting Backtest %s on %s", be.Config.ID, be.Config.Symbol)

	// Simulation Loop
	windowSize := 100 // Need enough history for indicators

	// Enable trader for backtesting
	be.Trader.SetRunning(true)

	count := 0
	for be.DataFeed.HasNext() {
		// Limitation for cost control
		if be.Config.Limit > 0 && count >= be.Config.Limit {
			log.Printf("🛑 Reached limit of %d cycles", be.Config.Limit)
			break
		}
		count++

		// 1. Advance Time
		currentKline, history, err := be.DataFeed.Next(windowSize)
		if err != nil {
			break
		}

		simTime := time.UnixMilli(currentKline.CloseTime) // Use close time or open time? usually decisions are made at close of candle.
		// Actually, usually we run at OpenTime of the NEW candle (using closed data from previous).
		// But here we are processing history.
		// Let's assume we run at CloseTime of the current kline.
		be.TimeProvider.SetTime(simTime)

		// 2. Update Market Data
		// Inject history into MockDataProvider
		be.DataProvider.Klines[be.Config.Symbol] = history

		// Inject Mock Open Interest (High enough to pass 15M filter)
		be.DataProvider.OpenInterest[be.Config.Symbol] = &market.OIData{
			Latest:  500 * 1_000_000, // 500M
			Average: 450 * 1_000_000,
		}
		// Update current price in MockTrader for PnL calcs
		be.MockTrader.SetCurrentPrice(be.Config.Symbol, currentKline.Close)

		// 3. Trigger Strategy
		if err := be.Trader.RunCycle(); err != nil {
			log.Printf("⚠️ Cycle error at %v: %v", simTime, err)
		}

		// 4. Record State
		balance, _ := be.MockTrader.GetBalance()
		equity, _ := balance["total_equity"].(float64)

		be.Equity = append(be.Equity, EquityPoint{
			Time:   simTime.UnixMilli(),
			Equity: equity,
			Price:  currentKline.Close,
		})
	}

	// Finalize Results
	// Populate Trades
	trades, _ := be.MockTrader.GetTradeHistory()
	be.Result.Trades = trades

	be.finalizeResults()

	log.Printf("🏁 Backtest Finished. Final Equity: %.2f", be.Result.TotalPnL+be.Config.InitialBalance)
	return be.Result, nil
}

func (be *BacktestEngine) finalizeResults() {
	if len(be.Equity) == 0 {
		return
	}
	finalEquity := be.Equity[len(be.Equity)-1].Equity
	be.Result.TotalPnL = finalEquity - be.Config.InitialBalance
	be.Result.TotalPnLPct = (be.Result.TotalPnL / be.Config.InitialBalance) * 100
	be.Result.EquityCurve = be.Equity
	be.Result.Config.Status = "completed"

	// Calculate Max Drawdown
	maxEquity := 0.0
	maxDD := 0.0
	for _, p := range be.Equity {
		if p.Equity > maxEquity {
			maxEquity = p.Equity
		}
		dd := (maxEquity - p.Equity) / maxEquity * 100
		if dd > maxDD {
			maxDD = dd
		}
	}
	be.Result.MaxDrawdown = maxDD
}
