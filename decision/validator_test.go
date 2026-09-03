package decision

import (
	"aetheris/market"
	"testing"
)

func TestSanitizeKeepsCloseDropsConflictingOpen(t *testing.T) {
	decisions := []Decision{
		{Symbol: "ETHUSDT", Action: "close_long", Reasoning: "exit"},
		{Symbol: "ETHUSDT", Action: "open_short", Leverage: 5, PositionSizeUSD: 20, StopLoss: 1, TakeProfit: 2, Reasoning: "flip"},
	}
	kept := sanitizeDecisions(decisions, 22, 5, 5, []PositionInfo{
		{Symbol: "ETHUSDT", Side: "long", Quantity: 1, EntryPrice: 3000, MarkPrice: 2990},
	}, map[string]*market.Data{}, 2.0, 3, nil)

	if len(kept) != 1 {
		t.Fatalf("kept %d decisions: %+v", len(kept), kept)
	}
	if kept[0].Action != "close_long" {
		t.Fatalf("expected close_long, got %s", kept[0].Action)
	}
}

func TestSanitizeDropsOpenWithoutMarketDataKeepsWait(t *testing.T) {
	decisions := []Decision{
		{Symbol: "BTCUSDT", Action: "open_long", Leverage: 5, PositionSizeUSD: 20, StopLoss: 1, TakeProfit: 2, Reasoning: "bad"},
		{Symbol: "BTCUSDT", Action: "wait", Reasoning: "stand by"},
	}
	kept := sanitizeDecisions(decisions, 22, 5, 5, nil, map[string]*market.Data{}, 2.0, 3, nil)

	if len(kept) != 1 || kept[0].Action != "wait" {
		t.Fatalf("expected only wait, got %+v", kept)
	}
}

func TestSanitizePhantomCloseBecomesWait(t *testing.T) {
	decisions := []Decision{
		{Symbol: "SOLUSDT", Action: "close_long", Reasoning: "ghost"},
	}
	kept := sanitizeDecisions(decisions, 22, 5, 5, nil, map[string]*market.Data{}, 2.0, 3, nil)
	if len(kept) != 1 || kept[0].Action != "wait" {
		t.Fatalf("expected wait conversion, got %+v", kept)
	}
}
