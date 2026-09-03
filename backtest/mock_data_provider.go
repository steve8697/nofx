package backtest

import (
	"fmt"
	"aetheris/market"
)

// MockDataProvider implements market.DataProvider for backtesting.
// It returns historical data injected by the backtest engine.
type MockDataProvider struct {
	Klines       map[string][]market.Kline
	OpenInterest map[string]*market.OIData
	FundingRates map[string]float64
}

func NewMockDataProvider() *MockDataProvider {
	return &MockDataProvider{
		Klines:       make(map[string][]market.Kline),
		OpenInterest: make(map[string]*market.OIData),
		FundingRates: make(map[string]float64),
	}
}

func (m *MockDataProvider) GetCurrentKlines(symbol string, timeframe string) ([]market.Kline, error) {
	if klines, ok := m.Klines[symbol]; ok {
		return klines, nil
	}
	return nil, fmt.Errorf("mock: no klines for %s", symbol)
}

func (m *MockDataProvider) GetOpenInterest(symbol string) (*market.OIData, error) {
	if oi, ok := m.OpenInterest[symbol]; ok {
		return oi, nil
	}
	// Return default zero value instead of error strictly? Or error?
	return &market.OIData{Latest: 0, Average: 0}, nil
}

func (m *MockDataProvider) GetFundingRate(symbol string) (float64, error) {
	if rate, ok := m.FundingRates[symbol]; ok {
		return rate, nil
	}
	return 0, nil
}

// GetOpenInterestHistory stub for MockDataProvider
func (m *MockDataProvider) GetOpenInterestHistory(symbol string, period string, limit int) ([]map[string]interface{}, error) {
	// Return empty for now, or mock logic if needed
	return []map[string]interface{}{}, nil
}
