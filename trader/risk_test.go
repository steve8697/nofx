package trader

import (
	"testing"
	"time"
)

func TestEvaluateRiskHaltDailyLoss(t *testing.T) {
	kind, _, halt := EvaluateRiskHalt(RiskHaltInput{
		DailyPnL:        -2.5,
		InitialBalance:  22,
		MaxDailyLossPct: 10,
		MaxDrawdownPct:  20,
	})
	if !halt || kind != HaltDailyLoss {
		t.Fatalf("expected daily-loss halt, got kind=%v halt=%v", kind, halt)
	}
}

func TestEvaluateRiskHaltDrawdown(t *testing.T) {
	kind, _, halt := EvaluateRiskHalt(RiskHaltInput{
		DailyPnL:        0,
		InitialBalance:  22,
		TotalPnLPct:     -21,
		MaxDailyLossPct: 10,
		MaxDrawdownPct:  20,
	})
	if !halt || kind != HaltDrawdown {
		t.Fatalf("expected drawdown halt, got kind=%v halt=%v", kind, halt)
	}
}

func TestEvaluateRiskHaltPeakDrawdown(t *testing.T) {
	// Account started at 500, peaked at 1000, now at 750 (TotalPnLPct is +50%, but Drawdown from peak is -25%)
	kind, reason, halt := EvaluateRiskHalt(RiskHaltInput{
		DailyPnL:        0,
		InitialBalance:  500,
		TotalPnLPct:     50,  // Still in net profit!
		PeakDrawdownPct: -25, // Dropped 25% from 1000 peak
		MaxDailyLossPct: 10,
		MaxDrawdownPct:  20,
	})
	if !halt || kind != HaltDrawdown {
		t.Fatalf("expected peak drawdown halt, got kind=%v halt=%v, reason=%s", kind, halt, reason)
	}
}

func TestEvaluateRiskHaltDisabledLimits(t *testing.T) {
	_, _, halt := EvaluateRiskHalt(RiskHaltInput{
		DailyPnL:       -100,
		InitialBalance: 22,
		TotalPnLPct:    -50,
	})
	if halt {
		t.Fatal("limits at 0 must not halt")
	}
}

func TestEvaluateRiskHaltWithinLimits(t *testing.T) {
	_, _, halt := EvaluateRiskHalt(RiskHaltInput{
		DailyPnL:        -0.5,
		InitialBalance:  22,
		TotalPnLPct:     -5,
		MaxDailyLossPct: 10,
		MaxDrawdownPct:  20,
	})
	if halt {
		t.Fatal("within limits must not halt")
	}
}

func TestHaltUntilDailyLossIsNextMidnight(t *testing.T) {
	now := time.Date(2026, 9, 2, 13, 56, 0, 0, time.FixedZone("CST", 8*3600))
	until := HaltUntil(now, HaltDailyLoss, time.Hour)
	want := time.Date(2026, 9, 3, 0, 0, 0, 0, now.Location())
	if !until.Equal(want) {
		t.Fatalf("got %v want %v", until, want)
	}
}

func TestHaltUntilDrawdownUsesPause(t *testing.T) {
	now := time.Date(2026, 9, 2, 13, 56, 0, 0, time.UTC)
	until := HaltUntil(now, HaltDrawdown, 90*time.Minute)
	if until.Sub(now) != 90*time.Minute {
		t.Fatalf("got %v", until)
	}
}
