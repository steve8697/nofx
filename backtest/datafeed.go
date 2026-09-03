package backtest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"aetheris/market"
	"time"
)

// DataFeed manages the historical data stream for the backtest
type DataFeed struct {
	Symbol     string
	Timeframe  string
	Klines     []market.Kline
	CurrentIdx int
}

// NewDataFeedFromFile loads kline data from a JSON file
// Format expected: Array of Klines compatible with Binance API or our internal struct
func NewDataFeedFromFile(filePath string, symbol, timeframe string) (*DataFeed, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read data file: %w", err)
	}

	var klines []market.Kline
	if err := json.Unmarshal(data, &klines); err != nil {
		return nil, fmt.Errorf("failed to parse kline data: %w", err)
	}

	return &DataFeed{
		Symbol:     symbol,
		Timeframe:  timeframe,
		Klines:     klines,
		CurrentIdx: 0,
	}, nil
}

// NewDataFeedFromMemory allows injecting data directly (e.g. for testing)
func NewDataFeedFromMemory(klines []market.Kline, symbol, timeframe string) *DataFeed {
	return &DataFeed{
		Symbol:     symbol,
		Timeframe:  timeframe,
		Klines:     klines,
		CurrentIdx: 0,
	}
}

// HasNext checks if there is more data
func (df *DataFeed) HasNext() bool {
	return df.CurrentIdx < len(df.Klines)
}

// Next advances the cursor and returns the current simulation time and relevant data slice
// returns: currentKline, historySlice, error
func (df *DataFeed) Next(windowSize int) (market.Kline, []market.Kline, error) {
	if !df.HasNext() {
		return market.Kline{}, nil, fmt.Errorf("EOF")
	}

	currentKline := df.Klines[df.CurrentIdx]

	// Prepare history window (up to current index)
	start := df.CurrentIdx - windowSize + 1
	if start < 0 {
		start = 0
	}
	// history should usually include the current kline as the "latest" one
	history := df.Klines[start : df.CurrentIdx+1]

	df.CurrentIdx++
	return currentKline, history, nil
}

// Reset resets the cursor
func (df *DataFeed) Reset() {
	df.CurrentIdx = 0
}

// CurrentTime returns the OpenTime of the *next* kline to be processed (or current if we just peeked)
// Actually, if we use Next(), we get the kline.
// Helper to get time without advancing?
func (df *DataFeed) PeekTime() time.Time {
	if df.CurrentIdx < len(df.Klines) {
		return time.UnixMilli(df.Klines[df.CurrentIdx].OpenTime)
	}
	return time.Time{}
}
