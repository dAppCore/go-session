// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"maps"
	"slices"
	"time"

	core "dappco.re/go/core"
)

// SessionAnalytics holds computed metrics for a parsed session.
//
// Example:
// analytics := session.Analyse(sess)
type SessionAnalytics struct {
	Duration              time.Duration
	ActiveTime            time.Duration
	EventCount            int
	ToolCounts            map[string]int
	ErrorCounts           map[string]int
	SuccessRate           float64
	AvgLatency            map[string]time.Duration
	MaxLatency            map[string]time.Duration
	EstimatedInputTokens  int
	EstimatedOutputTokens int
}

// Analyse iterates session events and computes analytics. Pure function, no I/O.
//
// Example:
// analytics := session.Analyse(sess)
func Analyse(sess *Session) *SessionAnalytics {
	a := &SessionAnalytics{
		ToolCounts:  make(map[string]int),
		ErrorCounts: make(map[string]int),
		AvgLatency:  make(map[string]time.Duration),
		MaxLatency:  make(map[string]time.Duration),
	}

	if sess == nil {
		return a
	}

	a.Duration = sess.EndTime.Sub(sess.StartTime)
	a.EventCount = len(sess.Events)

	// Track totals for latency averaging
	type latencyAccum struct {
		total time.Duration
		count int
	}
	latencies := make(map[string]*latencyAccum)

	var totalToolCalls int
	var totalErrors int

	for evt := range sess.EventsSeq() {
		// Token estimation: ~4 chars per token
		a.EstimatedInputTokens += len(evt.Input) / 4
		a.EstimatedOutputTokens += len(evt.Output) / 4

		if evt.Type != "tool_use" {
			continue
		}

		totalToolCalls++
		a.ToolCounts[evt.Tool]++

		if !evt.Success {
			totalErrors++
			a.ErrorCounts[evt.Tool]++
		}

		// Active time: sum of tool call durations
		a.ActiveTime += evt.Duration

		// Latency tracking
		if _, ok := latencies[evt.Tool]; !ok {
			latencies[evt.Tool] = &latencyAccum{}
		}
		latencies[evt.Tool].total += evt.Duration
		latencies[evt.Tool].count++

		if evt.Duration > a.MaxLatency[evt.Tool] {
			a.MaxLatency[evt.Tool] = evt.Duration
		}
	}

	// Compute averages
	for tool, acc := range latencies {
		if acc.count > 0 {
			a.AvgLatency[tool] = acc.total / time.Duration(acc.count)
		}
	}

	// Success rate
	if totalToolCalls > 0 {
		a.SuccessRate = float64(totalToolCalls-totalErrors) / float64(totalToolCalls)
	}

	return a
}

// FormatAnalytics returns a tabular text summary suitable for CLI display.
//
// Example:
// summary := session.FormatAnalytics(analytics)
func FormatAnalytics(a *SessionAnalytics) string {
	b := core.NewBuilder()

	b.WriteString("Session Analytics\n")
	b.WriteString(repeatString("=", 50) + "\n\n")

	b.WriteString(core.Sprintf("  Duration:       %s\n", formatDuration(a.Duration)))
	b.WriteString(core.Sprintf("  Active Time:    %s\n", formatDuration(a.ActiveTime)))
	b.WriteString(core.Sprintf("  Events:         %d\n", a.EventCount))
	b.WriteString(core.Sprintf("  Success Rate:   %.1f%%\n", a.SuccessRate*100))
	b.WriteString(core.Sprintf("  Est. Input Tk:  %d\n", a.EstimatedInputTokens))
	b.WriteString(core.Sprintf("  Est. Output Tk: %d\n", a.EstimatedOutputTokens))

	if len(a.ToolCounts) > 0 {
		b.WriteString("\n  Tool Breakdown\n")
		b.WriteString("  " + repeatString("-", 48) + "\n")
		b.WriteString(core.Sprintf("  %-14s %6s %6s %10s %10s\n",
			"Tool", "Calls", "Errors", "Avg", "Max"))
		b.WriteString("  " + repeatString("-", 48) + "\n")

		// Sort tools for deterministic output
		for _, tool := range slices.Sorted(maps.Keys(a.ToolCounts)) {
			errors := a.ErrorCounts[tool]
			avg := a.AvgLatency[tool]
			max := a.MaxLatency[tool]
			b.WriteString(core.Sprintf("  %-14s %6d %6d %10s %10s\n",
				tool, a.ToolCounts[tool], errors,
				formatDuration(avg), formatDuration(max)))
		}
	}

	return b.String()
}
