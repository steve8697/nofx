package decision

import (
	"fmt"
	"strings"
)

// SummarizeRecentDecisions keeps facts only: action, symbol, execution failure.
// It drops CoT / long reasoning so the next cycle is not anchored to stale opinions.
func SummarizeRecentDecisions(decisions []FullDecision, callCount int) string {
	if len(decisions) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# Recent actions (facts only)\n")
	start := 0
	if len(decisions) > 5 {
		start = len(decisions) - 5
	}
	slice := decisions[start:]
	for i := len(slice) - 1; i >= 0; i-- {
		d := slice[i]
		cycle := callCount - (len(slice) - 1 - i)
		summary := "wait"
		var fails []string
		if len(d.Decisions) > 0 {
			var parts []string
			for _, act := range d.Decisions {
				label := act.Action
				if act.Symbol != "" && act.Action != "wait" && act.Action != "hold" {
					label = act.Action + " " + act.Symbol
				}
				parts = append(parts, label)
				if act.ExecutionError != "" {
					fails = append(fails, fmt.Sprintf("%s failed: %s", label, trimFact(act.ExecutionError, 80)))
				}
			}
			summary = strings.Join(parts, ", ")
		}
		line := fmt.Sprintf("- cycle #%d %s: %s", cycle, d.Timestamp.Format("15:04"), summary)
		if len(fails) > 0 {
			line += " | " + strings.Join(fails, "; ")
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString("Judge this bar from live data. Do not continue a wait streak just because prior cycles waited.\n")
	return sb.String()
}

func trimFact(s string, n int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
