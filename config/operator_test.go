package config

import (
	"strings"
	"testing"
	"time"
)

func TestResolveDirectiveLastWriteWins(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	exp := now.Add(2 * time.Hour)
	events := []OperatorEvent{
		{Ts: now.Add(-3 * time.Hour), Actor: "cron", Action: OperatorPauseOpens, ExpiresAt: &exp},
		{Ts: now.Add(-1 * time.Hour), Actor: "grok", Action: OperatorResumeOpens},
		{Ts: now.Add(-30 * time.Minute), Actor: "pi", Action: OperatorNote, Note: "do not open ETH", ExpiresAt: &exp},
	}
	d := ResolveDirective(events, now)
	if d.PauseOpens {
		t.Fatalf("resume should have cleared pause: %+v", d)
	}
	if len(d.Notes) != 1 || d.Notes[0].Note != "do not open ETH" {
		t.Fatalf("notes: %+v", d.Notes)
	}

	laterPause := now.Add(time.Hour)
	events = append(events, OperatorEvent{Ts: now.Add(-5 * time.Minute), Actor: "web-ui", Action: OperatorPauseOpens, ExpiresAt: &laterPause})
	d = ResolveDirective(events, now)
	if !d.PauseOpens || d.PauseActor != "web-ui" {
		t.Fatalf("latest pause should win: %+v", d)
	}
}

func TestResolveDirectiveExpiredPauseIgnored(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Minute)
	events := []OperatorEvent{
		{Ts: now.Add(-2 * time.Hour), Actor: "cron", Action: OperatorPauseOpens, ExpiresAt: &expired},
	}
	d := ResolveDirective(events, now)
	if d.PauseOpens {
		t.Fatal("expired pause must not apply")
	}
}

func TestOperatorDigest(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	until := now.Add(2 * time.Hour)
	d := OperatorDirective{
		PauseOpens: true,
		PauseUntil: &until,
		PauseActor: "grok",
		Notes: []OperatorEvent{
			{Ts: now, Actor: "grok", Note: "BTC only manage, no new alts"},
		},
	}
	text := OperatorDigest(d, now)
	if !strings.Contains(text, "pause new opens") || !strings.Contains(text, "BTC only manage") {
		t.Fatalf("digest:\n%s", text)
	}
	if OperatorDigest(OperatorDirective{}, now) != "" {
		t.Fatal("empty directive should yield empty digest")
	}
}
