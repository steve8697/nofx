package backtest

import (
	"time"
)

// RunConfig replaces the upstream store.RunMetadata
// Defines the parameters for a specific backtest run.
type RunConfig struct {
	ID             string  `json:"id"`
	Strategy       string  `json:"strategy"` // "conservative", "aggressive", etc. (Mapped to Prompt Logic)
	Symbol         string  `json:"symbol"`
	Timeframe      string  `json:"timeframe"` // e.g. "15m"
	StartTime      int64   `json:"start_time"`
	EndTime        int64   `json:"end_time"`
	InitialBalance float64 `json:"initial_balance"`
	Leverage       int     `json:"leverage"`
	Description    string  `json:"description"`
	Status         string  `json:"status"` // pending, running, completed, failed
	DeepSeekKey    string  `json:"-"`      // API Key for Real AI verification (optional)
	QwenKey        string  `json:"-"`      // API Key for Real AI verification (optional)
	Limit          int     `json:"-"`      // Limit number of cycles (0 = no limit)
}

// TradeRecord represents a single trade (buy + sell)
type TradeRecord struct {
	Symbol     string    `json:"symbol"`
	EntryTime  time.Time `json:"entry_time"`
	ExitTime   time.Time `json:"exit_time"`
	EntryPrice float64   `json:"entry_price"`
	ExitPrice  float64   `json:"exit_price"`
	Quantity   float64   `json:"quantity"`
	PnL        float64   `json:"pnl"`
	Commission float64   `json:"commission"` // Trading fees
	PnLPercent float64   `json:"pnl_percent"`
	Direction  string    `json:"direction"` // "LONG" or "SHORT"
}

// BacktestResult stores the outcome of a backtest
type BacktestResult struct {
	Config       RunConfig     `json:"config"`
	TotalPnL     float64       `json:"total_pnl"`
	TotalPnLPct  float64       `json:"total_pnl_pct"`
	MaxDrawdown  float64       `json:"max_drawdown"`
	WinRate      float64       `json:"win_rate"`
	Trades       []TradeRecord `json:"trades"`
	EquityCurve  []EquityPoint `json:"equity_curve"`
	ExecutionLog []string      `json:"execution_log"`
}

type EquityPoint struct {
	Time   int64   `json:"time"`
	Equity float64 `json:"equity"`
	Price  float64 `json:"price"`
}

// MockTimeProvider implements utils.TimeProvider for backtesting
type MockTimeProvider struct {
	CurrentTime time.Time
}

func (m *MockTimeProvider) Now() time.Time {
	return m.CurrentTime
}

func (m *MockTimeProvider) SetTime(t time.Time) {
	m.CurrentTime = t
}
