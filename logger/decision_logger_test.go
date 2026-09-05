package logger

import (
	"testing"
)

func TestCalculateSharpeRatio(t *testing.T) {
	dl := &DecisionLogger{}

	// Test 1: Empty or insufficient records
	if sr := dl.calculateSharpeRatio(nil); sr != 0.0 {
		t.Errorf("Expected 0.0 for nil records, got %f", sr)
	}

	// Test 2: Identical flat returns (stdDev == 0) should return 0.0 instead of sentinel 999/-999
	flatRecords := []*DecisionRecord{
		{AccountState: AccountSnapshot{TotalBalance: 100.0}},
		{AccountState: AccountSnapshot{TotalBalance: 100.0}},
		{AccountState: AccountSnapshot{TotalBalance: 100.0}},
	}
	if sr := dl.calculateSharpeRatio(flatRecords); sr != 0.0 {
		t.Errorf("Expected 0.0 for flat returns, got %f", sr)
	}

	// Test 3: Constant negative returns with stdDev == 0
	downRecords := []*DecisionRecord{
		{AccountState: AccountSnapshot{TotalBalance: 100.0}},
		{AccountState: AccountSnapshot{TotalBalance: 90.0}}, // -10%
		{AccountState: AccountSnapshot{TotalBalance: 81.0}}, // -10%
	}
	if sr := dl.calculateSharpeRatio(downRecords); sr != 0.0 {
		t.Errorf("Expected 0.0 for zero variance negative returns, got %f", sr)
	}

	// Test 4: Extreme returns clamped to [-3.0, 3.0]
	extremeHighRecords := []*DecisionRecord{
		{AccountState: AccountSnapshot{TotalBalance: 100.0}},
		{AccountState: AccountSnapshot{TotalBalance: 200.0}}, // +100%
		{AccountState: AccountSnapshot{TotalBalance: 200.01}}, // tiny variance
	}
	sr := dl.calculateSharpeRatio(extremeHighRecords)
	if sr > 3.0 || sr < -3.0 {
		t.Errorf("Expected Sharpe ratio to be clamped between -3.0 and 3.0, got %f", sr)
	}
}
