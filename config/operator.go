package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	OperatorPauseOpens  = "pause_opens"
	OperatorResumeOpens = "resume_opens"
	OperatorNote        = "note"
)

// OperatorEvent is an append-only intervention from an external agent, cron, or the UI.
type OperatorEvent struct {
	ID        int64      `json:"id"`
	Ts        time.Time  `json:"ts"`
	Actor     string     `json:"actor"`
	Action    string     `json:"action"`
	Note      string     `json:"note"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// OperatorDirective is the resolved view the trader loop and UI consume.
type OperatorDirective struct {
	PauseOpens bool            `json:"pause_opens"`
	PauseUntil *time.Time      `json:"pause_until,omitempty"`
	PauseActor string          `json:"pause_actor,omitempty"`
	Notes      []OperatorEvent `json:"notes"`
}

func NormalizeOperatorAction(action string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case OperatorPauseOpens:
		return OperatorPauseOpens, nil
	case OperatorResumeOpens:
		return OperatorResumeOpens, nil
	case OperatorNote:
		return OperatorNote, nil
	default:
		return "", fmt.Errorf("unsupported action %q (pause_opens|resume_opens|note)", action)
	}
}

func NormalizeActor(actor string) string {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return "unknown"
	}
	if len(actor) > 64 {
		return actor[:64]
	}
	return actor
}

func ClampNote(note string) string {
	note = strings.TrimSpace(note)
	if len(note) > 500 {
		return note[:500]
	}
	return note
}

func eventActive(e OperatorEvent, now time.Time) bool {
	if e.ExpiresAt == nil {
		return true
	}
	return now.Before(*e.ExpiresAt)
}

// ResolveDirective walks events in chronological order. Last unexpired
// pause_opens / resume_opens wins. Notes that have not expired are kept.
func ResolveDirective(events []OperatorEvent, now time.Time) OperatorDirective {
	var d OperatorDirective
	for _, e := range events {
		switch e.Action {
		case OperatorPauseOpens:
			if !eventActive(e, now) {
				continue
			}
			d.PauseOpens = true
			d.PauseActor = e.Actor
			d.PauseUntil = e.ExpiresAt
		case OperatorResumeOpens:
			if !eventActive(e, now) {
				continue
			}
			d.PauseOpens = false
			d.PauseActor = e.Actor
			d.PauseUntil = nil
		case OperatorNote:
			if eventActive(e, now) && strings.TrimSpace(e.Note) != "" {
				d.Notes = append(d.Notes, e)
			}
		}
	}
	if len(d.Notes) > 8 {
		d.Notes = d.Notes[len(d.Notes)-8:]
	}
	return d
}

// OperatorDigest is the short block injected into the next trading cycle prompt.
func OperatorDigest(d OperatorDirective, now time.Time) string {
	var lines []string
	if d.PauseOpens {
		until := "until manually resumed"
		if d.PauseUntil != nil {
			until = "until " + d.PauseUntil.Format("15:04")
		}
		actor := d.PauseActor
		if actor == "" {
			actor = "operator"
		}
		lines = append(lines, fmt.Sprintf("[operator %s] pause new opens %s. Closes and risk exits still allowed. Do not request open_long/open_short.", actor, until))
	}
	for _, n := range d.Notes {
		ts := n.Ts.Format("15:04")
		lines = append(lines, fmt.Sprintf("[operator %s %s] %s", n.Actor, ts, n.Note))
	}
	if len(lines) == 0 {
		return ""
	}
	return "# External operator interventions (facts, not your prior opinions)\n" + strings.Join(lines, "\n") + "\nThese are already in effect in the runtime. Acknowledge them; do not re-argue them.\n"
}
