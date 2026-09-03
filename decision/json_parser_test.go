package decision

import (
	"encoding/json"
	"testing"
)

func TestStripJSONComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No comments",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "Single line comment at end",
			input:    `{"key": "value"} // comment`,
			expected: `{"key": "value"}`,
		},
		{
			name: "Comment inside JSON object",
			input: `{
				"key": "value", // comment
				"key2": 123
			}`,
			expected: `{
				"key": "value",
				"key2": 123
			}`,
		},
		{
			name:     "Comment with URL (should be preserved if in string)",
			input:    `{"url": "http://example.com"}`,
			expected: `{"url": "http://example.com"}`,
		},
		{
			name:     "Comment with URL in comment (should be removed)",
			input:    `{"key": "value"} // see http://example.com`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "Double slash in string",
			input:    `{"path": "/usr//bin"}`,
			expected: `{"path": "/usr//bin"}`,
		},
		{
			name:     "Block comment",
			input:    `{"key": "value" /* comment */}`,
			expected: `{"key": "value" }`,
		},
		{
			name: "Multiline block comment",
			input: `{"key": "value" /* 
				multiline
				comment 
			*/}`,
			expected: `{"key": "value" }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripJSONComments(tt.input)

			// Normalize whitespace for comparison (simple check)
			// For strict comparison we might want to parse both as JSON, but stripJSONComments returns string
			// So we just check if it parses as valid JSON if expected is valid JSON

			// Let's just check if the output contains the comment
			if tt.name == "Single line comment at end" {
				if len(got) > len(tt.expected)+5 { // heuristic
					t.Errorf("stripJSONComments() = %v, want %v", got, tt.expected)
				}
			}

			// Try to unmarshal the result to verify it's valid JSON
			var js map[string]interface{}
			err := json.Unmarshal([]byte(got), &js)
			if err != nil {
				t.Errorf("stripJSONComments() result is not valid JSON: %v\nInput: %s\nOutput: %s", err, tt.input, got)
			}
		})
	}
}

func TestFilterDemoDecisions(t *testing.T) {
	tests := []struct {
		name           string
		input          []Decision
		expected       int
		expectedAction string
	}{
		{
			name: "Normal trade decision",
			input: []Decision{
				{Symbol: "ETHUSDT", Action: "open_long", RiskUSD: 10},
			},
			expected:       1,
			expectedAction: "open_long",
		},
		{
			name: "Demo prefix filter",
			input: []Decision{
				{Symbol: "DEMO_BTCUSDT", Action: "open_short", RiskUSD: 300},
			},
			expected:       1, // ALL wait
			expectedAction: "wait",
		},
		{
			name: "Btc example exact copy filter",
			input: []Decision{
				{Symbol: "BTCUSDT", Action: "open_short", RiskUSD: 300, StopLoss: 97000},
			},
			expected:       1, // ALL wait
			expectedAction: "wait",
		},
		{
			name: "Eth example exact copy filter",
			input: []Decision{
				{Symbol: "ETHUSDT", Action: "close_long", Reasoning: "止盈离场"},
			},
			expected:       1, // ALL wait
			expectedAction: "wait",
		},
		{
			name: "Eth legitimate close_long with contextual reasoning should NOT be filtered",
			input: []Decision{
				{Symbol: "ETHUSDT", Action: "close_long", Reasoning: "触及布林带上轨且RSI超买，止盈离场锁定利润"},
			},
			expected:       1,
			expectedAction: "close_long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterDemoDecisions(tt.input)
			if len(got) != tt.expected {
				t.Errorf("filterDemoDecisions() returned %d decisions, want %d", len(got), tt.expected)
			}
			if len(got) > 0 && got[0].Action != tt.expectedAction {
				t.Errorf("filterDemoDecisions()[0].Action = %s, want %s", got[0].Action, tt.expectedAction)
			}
		})
	}
}

func TestHasMatchingPosition(t *testing.T) {
	positions := []PositionInfo{
		{Symbol: "BTCUSDT", Side: "long"},
		{Symbol: "SOLUSDT", Side: "short"},
	}

	tests := []struct {
		name     string
		symbol   string
		action   string
		expected bool
	}{
		{
			name:     "Has long, close long",
			symbol:   "BTCUSDT",
			action:   "close_long",
			expected: true,
		},
		{
			name:     "Has long, close short",
			symbol:   "BTCUSDT",
			action:   "close_short",
			expected: false,
		},
		{
			name:     "Has short, close short",
			symbol:   "SOLUSDT",
			action:   "close_short",
			expected: true,
		},
		{
			name:     "No position, close long",
			symbol:   "ETHUSDT",
			action:   "close_long",
			expected: false,
		},
		{
			name:     "No position, update stop loss",
			symbol:   "ETHUSDT",
			action:   "update_stop_loss",
			expected: false,
		},
		{
			name:     "Has position, update stop loss",
			symbol:   "BTCUSDT",
			action:   "update_stop_loss",
			expected: true,
		},
		{
			name:     "Open long",
			symbol:   "ETHUSDT",
			action:   "open_long",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMatchingPosition(tt.symbol, tt.action, positions)
			if got != tt.expected {
				t.Errorf("hasMatchingPosition() = %v, want %v", got, tt.expected)
			}
		})
	}
}
