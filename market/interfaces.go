package market

// DataProvider defines the interface for accessing market data.
// This allows switching between live WebSocket data (WSMonitor) and historical backtest data.
type DataProvider interface {
	// GetCurrentKlines retrieves the kline history for a given symbol and timeframe.
	GetCurrentKlines(symbol string, timeframe string) ([]Kline, error)

	// GetOpenInterest retrieves the latest Open Interest data.
	GetOpenInterest(symbol string) (*OIData, error)

	// GetOpenInterestHistory retrieves historical Open Interest data.
	GetOpenInterestHistory(symbol string, period string, limit int) ([]map[string]interface{}, error)

	// GetFundingRate retrieves the latest Funding Rate.
	GetFundingRate(symbol string) (float64, error)
}
