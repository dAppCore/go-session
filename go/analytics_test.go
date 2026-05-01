// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"
	"time"

	core "dappco.re/go"
)

func TestAnalytics_Analyse_Good(t *testing.T) {
	sess := &Session{StartTime: time.Unix(0, 0), EndTime: time.Unix(4, 0), Events: []Event{
		{Type: "tool_use", Tool: "Bash", Input: "abcd", Output: "abcdefgh", Duration: 2 * time.Second, Success: true},
	}}

	got := Analyse(sess)

	core.AssertEqual(t, 1, got.EventCount)
	core.AssertEqual(t, 1.0, got.SuccessRate)
	core.AssertEqual(t, 2*time.Second, got.ActiveTime)
	core.AssertEqual(t, 1, got.EstimatedInputTokens)
	core.AssertEqual(t, 2, got.EstimatedOutputTokens)
}

func TestAnalytics_Analyse_Bad(t *testing.T) {
	sess := &Session{Events: []Event{
		{Type: "tool_use", Tool: "Read", Duration: time.Second, Success: false},
	}}

	got := Analyse(sess)

	core.AssertEqual(t, 0.0, got.SuccessRate)
	core.AssertEqual(t, 1, got.ErrorCounts["Read"])
	core.AssertEqual(t, time.Second, got.MaxLatency["Read"])
}

func TestAnalytics_Analyse_Ugly(t *testing.T) {
	got := Analyse(nil)

	core.AssertNotNil(t, got)
	core.AssertEqual(t, 0, got.EventCount)
	core.AssertEmpty(t, got.ToolCounts)
}

func TestAnalytics_FormatAnalytics_Good(t *testing.T) {
	text := FormatAnalytics(&SessionAnalytics{
		Duration:    time.Minute,
		ActiveTime:  time.Second,
		EventCount:  2,
		ToolCounts:  map[string]int{"Bash": 1},
		ErrorCounts: map[string]int{},
		AvgLatency:  map[string]time.Duration{"Bash": time.Second},
		MaxLatency:  map[string]time.Duration{"Bash": time.Second},
		SuccessRate: 1,
	})

	core.AssertContains(t, text, "Session Analytics")
	core.AssertContains(t, text, "Bash")
	core.AssertContains(t, text, "100.0%")
}

func TestAnalytics_FormatAnalytics_Bad(t *testing.T) {
	text := FormatAnalytics(&SessionAnalytics{ToolCounts: map[string]int{}, ErrorCounts: map[string]int{}, AvgLatency: map[string]time.Duration{}, MaxLatency: map[string]time.Duration{}})

	core.AssertContains(t, text, "Events:")
	core.AssertNotContains(t, text, "Tool Breakdown")
}

func TestAnalytics_FormatAnalytics_Ugly(t *testing.T) {
	text := FormatAnalytics(&SessionAnalytics{SuccessRate: 0.333})

	core.AssertContains(t, text, "33.3%")
	core.AssertContains(t, text, "0ms")
}
