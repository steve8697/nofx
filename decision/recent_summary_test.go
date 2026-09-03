package decision

import (
	"strings"
	"testing"
	"time"
)

func TestSummarizeRecentDecisionsDropsReasoning(t *testing.T) {
	d := []FullDecision{
		{
			Timestamp: time.Date(2026, 9, 2, 11, 0, 0, 0, time.UTC),
			Decisions: []Decision{{Action: "wait", Reasoning: "I already waited 100 times so wait again"}},
		},
		{
			Timestamp: time.Date(2026, 9, 2, 11, 3, 0, 0, time.UTC),
			Decisions: []Decision{{Action: "open_long", Symbol: "ETHUSDT", ExecutionError: "insufficient margin"}},
		},
	}
	got := SummarizeRecentDecisions(d, 12)
	if strings.Contains(got, "waited 100") {
		t.Fatalf("must not leak CoT: %s", got)
	}
	if !strings.Contains(got, "open_long ETHUSDT") || !strings.Contains(got, "insufficient margin") {
		t.Fatalf("facts missing: %s", got)
	}
}
