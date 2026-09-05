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

func TestValidatePartialCloseRemainingValue(t *testing.T) {
	marketData := map[string]*market.Data{
		"LTCUSDT": {
			CurrentPrice: 50.0,
		},
	}

	positions := []PositionInfo{
		{
			Symbol:     "LTCUSDT",
			Side:       "long",
			Quantity:   0.36, // 0.36 * 50 = $18.00 total value
			EntryPrice: 50.0,
			MarkPrice:  50.0,
		},
	}

	// 案例 1: 倉位 $18，平倉 58% -> 平倉 $10.44，剩餘 $7.56 (< $10 minNotional) -> 必須被攔截拒絕
	d1 := &Decision{
		Symbol:          "LTCUSDT",
		Action:          "partial_close",
		ClosePercentage: 58.0,
	}
	err1 := validatePartialCloseSize(d1, positions, marketData)
	if err1 == nil {
		t.Fatalf("預期應該拒絕 (剩餘部位 $7.56 < $10.0)，但卻通過了！")
	}

	// 案例 2: 倉位 $18，平倉 30% -> 平倉 $5.40 (< $10 minNotional) -> 必須被平倉金額不足攔截
	d2 := &Decision{
		Symbol:          "LTCUSDT",
		Action:          "partial_close",
		ClosePercentage: 30.0,
	}
	err2 := validatePartialCloseSize(d2, positions, marketData)
	if err2 == nil {
		t.Fatalf("預期應該拒絕 (平倉金額 $5.40 < $10.0)，但卻通過了！")
	}

	// 案例 3: 倉位 $18，平倉 100% (全平) -> 必須放行
	d3 := &Decision{
		Symbol:          "LTCUSDT",
		Action:          "partial_close",
		ClosePercentage: 100.0,
	}
	err3 := validatePartialCloseSize(d3, positions, marketData)
	if err3 != nil {
		t.Fatalf("全平應該放行，但報錯: %v", err3)
	}

	// 案例 4: 倉位 $30 (0.6 LTC)，平倉 50% -> 平倉 $15，剩餘 $15 (均 >= $10) -> 必須通過
	positionsBig := []PositionInfo{
		{
			Symbol:     "LTCUSDT",
			Side:       "long",
			Quantity:   0.6, // 0.6 * 50 = $30.00
			EntryPrice: 50.0,
			MarkPrice:  50.0,
		},
	}
	d4 := &Decision{
		Symbol:          "LTCUSDT",
		Action:          "partial_close",
		ClosePercentage: 50.0,
	}
	err4 := validatePartialCloseSize(d4, positionsBig, marketData)
	if err4 != nil {
		t.Fatalf("兩者均充足時應該通過，但報錯: %v", err4)
	}

	// 案例 5: 倉位 $30，平倉 20% (平倉金額 $6 < $10) 與 平倉 80% (剩餘金額 $6 < $10) -> 均必須被區間攔截
	d5Low := &Decision{
		Symbol:          "LTCUSDT",
		Action:          "partial_close",
		ClosePercentage: 20.0,
	}
	if err := validatePartialCloseSize(d5Low, positionsBig, marketData); err == nil {
		t.Fatalf("平倉 20%% 應該被下限攔截！")
	}

	d5High := &Decision{
		Symbol:          "LTCUSDT",
		Action:          "partial_close",
		ClosePercentage: 80.0,
	}
	if err := validatePartialCloseSize(d5High, positionsBig, marketData); err == nil {
		t.Fatalf("平倉 80%% 應該被上限攔截！")
	}
}
