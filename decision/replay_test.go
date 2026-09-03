package decision

import (
	"testing"
	"time"

	"aetheris/logger"
)

func TestReplaySanitizeKeepsWait(t *testing.T) {
	rec := logger.DecisionRecord{
		Timestamp:    time.Date(2026, 7, 20, 13, 52, 0, 0, time.UTC),
		CycleNumber:  1,
		DecisionJSON: `[{"symbol":"BTCUSDT","action":"wait","reasoning":"stand by"}]`,
		AccountState: logger.AccountSnapshot{TotalBalance: 22.02},
		Decisions: []logger.DecisionAction{
			{Symbol: "BTCUSDT", Action: "wait", Success: true},
		},
		Success: true,
	}
	got := ReplaySanitize(rec, 5, 5, 3, 2)
	if len(got.Kept) != 1 || got.Kept[0] != "wait BTCUSDT" {
		t.Fatalf("kept=%v", got.Kept)
	}
	if len(got.Dropped) != 0 {
		t.Fatalf("dropped=%v", got.Dropped)
	}
}

func TestReplaySanitizeDropsOpenWithoutMarket(t *testing.T) {
	rec := logger.DecisionRecord{
		DecisionJSON: `[{"symbol":"ETHUSDT","action":"open_long","leverage":5,"position_size_usd":20,"stop_loss":1,"take_profit":2,"reasoning":"x"}]`,
		AccountState: logger.AccountSnapshot{TotalBalance: 22},
		Success:      true,
	}
	got := ReplaySanitize(rec, 5, 5, 3, 2)
	if len(got.Dropped) != 1 {
		t.Fatalf("expected dropped open, got %+v", got)
	}
}
