// SPDX-Licence-Identifier: EUPL-1.2
package main

import (
	"time"

	core "dappco.re/go"
	session "dappco.re/go/session"
)

const transcript = `{"type":"user","timestamp":"2026-02-20T10:00:00Z","sessionId":"ax10-session","message":{"role":"user","content":[{"type":"text","text":"Run the AX-10 smoke test"}]}}
{"type":"assistant","timestamp":"2026-02-20T10:00:01Z","sessionId":"ax10-session","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","id":"tool-bash-1","input":{"command":"echo ax10","description":"smoke test"}}]}}
{"type":"user","timestamp":"2026-02-20T10:00:02Z","sessionId":"ax10-session","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-bash-1","content":"ax10\n","is_error":false}]}}
{"type":"assistant","timestamp":"2026-02-20T10:00:03Z","sessionId":"ax10-session","message":{"role":"assistant","content":[{"type":"text","text":"AX-10 complete"}]}}
`

// main runs the CLI session smoke test.
func main() {
	fs := (&core.Fs{}).NewUnrestricted()
	dir := fs.TempDir("go-session-ax10-")
	require(dir != "", "create temporary directory")
	defer func() {
		deleteResult := fs.DeleteAll(dir)
		require(deleteResult.OK, "delete temporary directory")
	}()

	transcriptPath := core.Path(dir, "ax10-session.jsonl")
	writeResult := fs.WriteMode(transcriptPath, transcript, 0o600)
	require(writeResult.OK, "write transcript")

	sess, stats, err := session.ParseTranscript(transcriptPath)
	requireNoError(err, "parse transcript")
	require(sess.ID == "ax10-session", "session ID should come from the file name")
	require(sess.Path == transcriptPath, "session path should match the parsed file")
	require(len(sess.Events) == 3, "expected user, tool, and assistant events")
	require(stats.TotalLines == 4, "expected all transcript lines to be scanned")
	require(stats.SkippedLines == 0, "expected no skipped transcript lines")
	require(stats.OrphanedToolCalls == 0, "expected no orphaned tool calls")

	tool := sess.Events[1]
	require(tool.Type == "tool_use", "expected second event to be the tool call")
	require(tool.Tool == "Bash", "expected Bash tool call")
	require(tool.Input == "echo ax10 # smoke test", "expected Bash input to include command and description")
	require(tool.Output == "ax10\n", "expected Bash output to be preserved")
	expectedDuration := time.Second
	require(tool.Duration == expectedDuration, "expected tool duration to match transcript timestamps")
	require(tool.Success, "expected successful tool call")

	analytics := session.Analyse(sess)
	require(analytics.EventCount == 3, "expected analytics event count")
	require(analytics.ToolCounts["Bash"] == 1, "expected analytics Bash count")
	expectedSuccessRate := successfulToolRate(sess)
	require(analytics.SuccessRate == expectedSuccessRate, "expected analytics success rate")
	require(core.Contains(session.FormatAnalytics(analytics), "Bash"), "expected formatted analytics to include Bash")

	results, err := session.Search(dir, "ax10")
	requireNoError(err, "search sessions")
	require(len(results) == 1, "expected one search result")
	require(results[0].SessionID == "ax10-session", "expected search result session ID")

	sessions, err := session.ListSessions(dir)
	requireNoError(err, "list sessions")
	require(len(sessions) == 1, "expected one listed session")
	require(sessions[0].ID == "ax10-session", "expected listed session ID")

	fetched, _, err := session.FetchSession(dir, "ax10-session")
	requireNoError(err, "fetch session")
	require(fetched.ID == sess.ID, "expected fetched session to match parsed session")

	htmlPath := core.Path(dir, "timeline.html")
	requireNoError(session.RenderHTML(sess, htmlPath), "render HTML")
	readResult := fs.Read(htmlPath)
	require(readResult.OK, "read rendered HTML")
	html, ok := readResult.Value.(string)
	require(ok, "read rendered HTML as string")
	require(core.Contains(html, "Session ax10"), "expected rendered HTML session title")
	require(core.Contains(html, "echo ax10"), "expected rendered HTML tool input")
}

// successfulToolRate calculates the same tool-call success ratio as session.Analyse.
func successfulToolRate(sess *session.Session) float64 {
	var successful, total int
	for _, evt := range sess.Events {
		if evt.Type != "tool_use" {
			continue
		}
		total++
		if evt.Success {
			successful++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(successful) / float64(total)
}

// require stops the current test case when its condition is not met.
func require(ok bool, msg string) {
	if !ok {
		panic(msg)
	}
}

// requireNoError stops the current test case when its condition is not met.
func requireNoError(err error, msg string) {
	if err != nil {
		panic(msg + ": " + err.Error())
	}
}
