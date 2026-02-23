// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyse_EmptySession_Good(t *testing.T) {
	sess := &Session{
		ID:        "empty",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events:    nil,
	}

	a := Analyse(sess)
	require.NotNil(t, a)

	assert.Equal(t, time.Duration(0), a.Duration)
	assert.Equal(t, time.Duration(0), a.ActiveTime)
	assert.Equal(t, 0, a.EventCount)
	assert.Equal(t, 0.0, a.SuccessRate)
	assert.Empty(t, a.ToolCounts)
	assert.Empty(t, a.ErrorCounts)
	assert.Equal(t, 0, a.EstimatedInputTokens)
	assert.Equal(t, 0, a.EstimatedOutputTokens)
}

func TestAnalyse_NilSession_Good(t *testing.T) {
	a := Analyse(nil)
	require.NotNil(t, a)
	assert.Equal(t, 0, a.EventCount)
}

func TestAnalyse_SingleToolCall_Good(t *testing.T) {
	sess := &Session{
		ID:        "single",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 20, 10, 0, 5, 0, time.UTC),
		Events: []Event{
			{
				Timestamp: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
				Type:      "tool_use",
				Tool:      "Bash",
				Input:     "go test ./...",
				Output:    "PASS",
				Duration:  2 * time.Second,
				Success:   true,
			},
		},
	}

	a := Analyse(sess)

	assert.Equal(t, 5*time.Second, a.Duration)
	assert.Equal(t, 2*time.Second, a.ActiveTime)
	assert.Equal(t, 1, a.EventCount)
	assert.Equal(t, 1.0, a.SuccessRate)
	assert.Equal(t, 1, a.ToolCounts["Bash"])
	assert.Equal(t, 0, a.ErrorCounts["Bash"])
	assert.Equal(t, 2*time.Second, a.AvgLatency["Bash"])
	assert.Equal(t, 2*time.Second, a.MaxLatency["Bash"])
}

func TestAnalyse_MixedToolsWithErrors_Good(t *testing.T) {
	sess := &Session{
		ID:        "mixed",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 20, 10, 5, 0, 0, time.UTC),
		Events: []Event{
			{
				Type:  "user",
				Input: "Please help",
			},
			{
				Type:     "tool_use",
				Tool:     "Bash",
				Input:    "ls -la",
				Output:   "total 42",
				Duration: 1 * time.Second,
				Success:  true,
			},
			{
				Type:     "tool_use",
				Tool:     "Bash",
				Input:    "cat /missing",
				Output:   "No such file",
				Duration: 500 * time.Millisecond,
				Success:  false,
				ErrorMsg: "No such file",
			},
			{
				Type:     "tool_use",
				Tool:     "Read",
				Input:    "/tmp/file.go",
				Output:   "package main",
				Duration: 200 * time.Millisecond,
				Success:  true,
			},
			{
				Type:     "tool_use",
				Tool:     "Read",
				Input:    "/tmp/missing.go",
				Output:   "file not found",
				Duration: 100 * time.Millisecond,
				Success:  false,
				ErrorMsg: "file not found",
			},
			{
				Type:     "tool_use",
				Tool:     "Edit",
				Input:    "/tmp/file.go (edit)",
				Output:   "ok",
				Duration: 300 * time.Millisecond,
				Success:  true,
			},
			{
				Type:  "assistant",
				Input: "All done.",
			},
		},
	}

	a := Analyse(sess)

	assert.Equal(t, 5*time.Minute, a.Duration)
	assert.Equal(t, 7, a.EventCount)

	// Tool counts
	assert.Equal(t, 2, a.ToolCounts["Bash"])
	assert.Equal(t, 2, a.ToolCounts["Read"])
	assert.Equal(t, 1, a.ToolCounts["Edit"])

	// Error counts
	assert.Equal(t, 1, a.ErrorCounts["Bash"])
	assert.Equal(t, 1, a.ErrorCounts["Read"])
	assert.Equal(t, 0, a.ErrorCounts["Edit"])

	// Success rate: 3 successes out of 5 tool calls = 0.6
	assert.InDelta(t, 0.6, a.SuccessRate, 0.001)

	// Active time: 1s + 500ms + 200ms + 100ms + 300ms = 2.1s
	assert.Equal(t, 2100*time.Millisecond, a.ActiveTime)
}

func TestAnalyse_LatencyCalculations_Good(t *testing.T) {
	sess := &Session{
		ID:        "latency",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 20, 10, 1, 0, 0, time.UTC),
		Events: []Event{
			{
				Type:     "tool_use",
				Tool:     "Bash",
				Duration: 1 * time.Second,
				Success:  true,
			},
			{
				Type:     "tool_use",
				Tool:     "Bash",
				Duration: 3 * time.Second,
				Success:  true,
			},
			{
				Type:     "tool_use",
				Tool:     "Bash",
				Duration: 5 * time.Second,
				Success:  true,
			},
			{
				Type:     "tool_use",
				Tool:     "Read",
				Duration: 200 * time.Millisecond,
				Success:  true,
			},
		},
	}

	a := Analyse(sess)

	// Bash: avg = (1+3+5)/3 = 3s, max = 5s
	assert.Equal(t, 3*time.Second, a.AvgLatency["Bash"])
	assert.Equal(t, 5*time.Second, a.MaxLatency["Bash"])

	// Read: avg = 200ms, max = 200ms
	assert.Equal(t, 200*time.Millisecond, a.AvgLatency["Read"])
	assert.Equal(t, 200*time.Millisecond, a.MaxLatency["Read"])
}

func TestAnalyse_TokenEstimation_Good(t *testing.T) {
	// 4 chars = ~1 token
	sess := &Session{
		ID:        "tokens",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 20, 10, 0, 1, 0, time.UTC),
		Events: []Event{
			{
				Type:  "user",
				Input: strings.Repeat("a", 400), // 100 tokens
			},
			{
				Type:     "tool_use",
				Tool:     "Bash",
				Input:    strings.Repeat("b", 80),  // 20 tokens
				Output:   strings.Repeat("c", 200), // 50 tokens
				Duration: time.Second,
				Success:  true,
			},
			{
				Type:  "assistant",
				Input: strings.Repeat("d", 120), // 30 tokens
			},
		},
	}

	a := Analyse(sess)

	// Input tokens: 400/4 + 80/4 + 120/4 = 100 + 20 + 30 = 150
	assert.Equal(t, 150, a.EstimatedInputTokens)
	// Output tokens: 0 + 200/4 + 0 = 50
	assert.Equal(t, 50, a.EstimatedOutputTokens)
}

func TestFormatAnalytics_Output_Good(t *testing.T) {
	a := &SessionAnalytics{
		Duration:              5 * time.Minute,
		ActiveTime:            2 * time.Minute,
		EventCount:            42,
		SuccessRate:           0.85,
		EstimatedInputTokens:  1500,
		EstimatedOutputTokens: 3000,
		ToolCounts: map[string]int{
			"Bash": 20,
			"Read": 15,
			"Edit": 7,
		},
		ErrorCounts: map[string]int{
			"Bash": 3,
		},
		AvgLatency: map[string]time.Duration{
			"Bash": 2 * time.Second,
			"Read": 500 * time.Millisecond,
			"Edit": 300 * time.Millisecond,
		},
		MaxLatency: map[string]time.Duration{
			"Bash": 10 * time.Second,
			"Read": 1 * time.Second,
			"Edit": 800 * time.Millisecond,
		},
	}

	output := FormatAnalytics(a)

	assert.Contains(t, output, "Session Analytics")
	assert.Contains(t, output, "5m0s")
	assert.Contains(t, output, "2m0s")
	assert.Contains(t, output, "42")
	assert.Contains(t, output, "85.0%")
	assert.Contains(t, output, "1500")
	assert.Contains(t, output, "3000")
	assert.Contains(t, output, "Bash")
	assert.Contains(t, output, "Read")
	assert.Contains(t, output, "Edit")
	assert.Contains(t, output, "Tool Breakdown")
}

func TestFormatAnalytics_EmptyAnalytics_Good(t *testing.T) {
	a := &SessionAnalytics{
		ToolCounts:  make(map[string]int),
		ErrorCounts: make(map[string]int),
		AvgLatency:  make(map[string]time.Duration),
		MaxLatency:  make(map[string]time.Duration),
	}

	output := FormatAnalytics(a)

	assert.Contains(t, output, "Session Analytics")
	assert.Contains(t, output, "0.0%")
	// No tool breakdown section when no tools
	assert.NotContains(t, output, "Tool Breakdown")
}
