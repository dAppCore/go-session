// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"bytes"
	"path"
	"syscall"
	"testing"
	"time"

	core "dappco.re/go"
)

// --- helpers to build synthetic JSONL ---

// ts returns a stable timestamp offset by the given seconds from a fixed epoch.
func ts(offsetSec int) string {
	base := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(offsetSec) * time.Second).Format(time.RFC3339Nano)
}

// jsonlLine marshals an arbitrary map to a single JSONL line.
func jsonlLine(m map[string]any) string {
	marshalResult := core.JSONMarshal(m)
	if !marshalResult.OK {
		panic(resultError(marshalResult))
	}
	return string(marshalResult.Value.([]byte))
}

// userTextEntry creates a JSONL line for a user text message.
func userTextEntry(timestamp string, text string) string {
	return jsonlLine(map[string]any{
		"type":      "user",
		"timestamp": timestamp,
		"sessionId": "test-session",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		},
	})
}

// assistantTextEntry creates a JSONL line for an assistant text message.
func assistantTextEntry(timestamp string, text string) string {
	return jsonlLine(map[string]any{
		"type":      "assistant",
		"timestamp": timestamp,
		"sessionId": "test-session",
		"message": map[string]any{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		},
	})
}

// toolUseEntry creates a JSONL line for an assistant message containing a tool_use block.
func toolUseEntry(timestamp, toolName, toolID string, input map[string]any) string {
	return jsonlLine(map[string]any{
		"type":      "assistant",
		"timestamp": timestamp,
		"sessionId": "test-session",
		"message": map[string]any{
			"role": "assistant",
			"content": []map[string]any{
				{
					"type":  "tool_use",
					"name":  toolName,
					"id":    toolID,
					"input": input,
				},
			},
		},
	})
}

// toolResultEntry creates a JSONL line for a user message containing a tool_result block.
func toolResultEntry(timestamp, toolUseID string, content any, isError bool) string {
	entry := map[string]any{
		"type":      "user",
		"timestamp": timestamp,
		"sessionId": "test-session",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{
				{
					"type":        "tool_result",
					"tool_use_id": toolUseID,
					"content":     content,
					"is_error":    isError,
				},
			},
		},
	}
	return jsonlLine(entry)
}

// writeJSONL writes lines to a temp .jsonl file and returns its path.
func writeJSONL(t *testing.T, dir string, name string, lines ...string) string {
	t.Helper()
	filePath := path.Join(dir, name)
	writeResult := hostFS.Write(filePath, core.Concat(core.Join("\n", lines...), "\n"))
	requireTrue(t, writeResult.OK)
	return filePath
}

// setFileTimes supports the session test suite.
func setFileTimes(filePath string, atime, mtime time.Time) error {
	return syscall.UtimesNano(filePath, []syscall.Timespec{
		syscall.NsecToTimespec(atime.UnixNano()),
		syscall.NsecToTimespec(mtime.UnixNano()),
	})
}

// --- ParseTranscript tests ---

// TestParser_ParseTranscriptMinimalValid_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptMinimalValid_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "minimal.jsonl",
		userTextEntry(ts(0), "Hello"),
		assistantTextEntry(ts(1), "Hi there!"),
	)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)
	requireNotNil(t, sess)

	assertEqual(t, "minimal", sess.ID)
	assertEqual(t, path, sess.Path)
	assertFalse(t, sess.StartTime.IsZero(), "StartTime should be set")
	assertFalse(t, sess.EndTime.IsZero(), "EndTime should be set")
	assertTrue(t, sess.EndTime.After(sess.StartTime) || sess.EndTime.Equal(sess.StartTime))

	// Should have a user event and an assistant event
	requireLen(t, sess.Events, 2)
	assertEqual(t, "user", sess.Events[0].Type)
	assertEqual(t, "Hello", sess.Events[0].Input)
	assertEqual(t, "assistant", sess.Events[1].Type)
	assertEqual(t, "Hi there!", sess.Events[1].Input)
}

// TestParser_ParseTranscriptToolCalls_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptToolCalls_Good(t *testing.T) {
	dir := t.TempDir()

	lines := []string{
		userTextEntry(ts(0), "Run a command"),
		// Bash tool_use
		toolUseEntry(ts(1), "Bash", "tool-bash-1", map[string]any{
			"command":     "ls -la",
			"description": "list files",
		}),
		toolResultEntry(ts(2), "tool-bash-1", "total 42\ndrwxr-xr-x 5 user staff 160 Feb 20 10:00 .", false),
		// Read tool_use
		toolUseEntry(ts(3), "Read", "tool-read-1", map[string]any{
			"file_path": "/tmp/test.go",
		}),
		toolResultEntry(ts(4), "tool-read-1", "package main\n\nfunc main() {}", false),
		// Edit tool_use
		toolUseEntry(ts(5), "Edit", "tool-edit-1", map[string]any{
			"file_path":  "/tmp/test.go",
			"old_string": "main",
			"new_string": "app",
		}),
		toolResultEntry(ts(6), "tool-edit-1", "ok", false),
		// Write tool_use
		toolUseEntry(ts(7), "Write", "tool-write-1", map[string]any{
			"file_path": "/tmp/new.go",
			"content":   "package new\n",
		}),
		toolResultEntry(ts(8), "tool-write-1", "ok", false),
		// Grep tool_use
		toolUseEntry(ts(9), "Grep", "tool-grep-1", map[string]any{
			"pattern": "TODO",
			"path":    "/tmp",
		}),
		toolResultEntry(ts(10), "tool-grep-1", "/tmp/test.go:3:// TODO fix this", false),
		// Glob tool_use
		toolUseEntry(ts(11), "Glob", "tool-glob-1", map[string]any{
			"pattern": "**/*.go",
		}),
		toolResultEntry(ts(12), "tool-glob-1", "/tmp/test.go\n/tmp/new.go", false),
		// Task tool_use
		toolUseEntry(ts(13), "Task", "tool-task-1", map[string]any{
			"prompt":        "Analyse the code",
			"description":   "Code analysis",
			"subagent_type": "research",
		}),
		toolResultEntry(ts(14), "tool-task-1", "Analysis complete", false),
		assistantTextEntry(ts(15), "All done."),
	}

	path := writeJSONL(t, dir, "tools.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	// Count tool_use events
	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	requireLen(t, toolEvents, 7, "should have 7 tool_use events")

	// Verify each tool was parsed correctly
	assertEqual(t, "Bash", toolEvents[0].Tool)
	assertContains(t, toolEvents[0].Input, "ls -la")
	assertContains(t, toolEvents[0].Input, "# list files")
	assertTrue(t, toolEvents[0].Success)
	assertEqual(t, time.Second, toolEvents[0].Duration)

	assertEqual(t, "Read", toolEvents[1].Tool)
	assertEqual(t, "/tmp/test.go", toolEvents[1].Input)

	assertEqual(t, "Edit", toolEvents[2].Tool)
	assertEqual(t, "/tmp/test.go (edit)", toolEvents[2].Input)

	assertEqual(t, "Write", toolEvents[3].Tool)
	assertEqual(t, "/tmp/new.go (12 bytes)", toolEvents[3].Input)

	assertEqual(t, "Grep", toolEvents[4].Tool)
	assertEqual(t, "/TODO/ in /tmp", toolEvents[4].Input)

	assertEqual(t, "Glob", toolEvents[5].Tool)
	assertEqual(t, "**/*.go", toolEvents[5].Input)

	assertEqual(t, "Task", toolEvents[6].Tool)
	assertEqual(t, "[research] Code analysis", toolEvents[6].Input)
}

// TestParser_ParseTranscriptToolError_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptToolError_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "error.jsonl",
		toolUseEntry(ts(0), "Bash", "tool-err-1", map[string]any{
			"command": "cat /nonexistent",
		}),
		toolResultEntry(ts(1), "tool-err-1", "cat: /nonexistent: No such file or directory", true),
	)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	requireLen(t, toolEvents, 1)
	assertFalse(t, toolEvents[0].Success)
	assertContains(t, toolEvents[0].ErrorMsg, "No such file or directory")
}

// TestParser_ParseTranscriptEmptyFile_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptEmptyFile_Bad(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "empty.jsonl")
	// Write a truly empty file
	writeResult := hostFS.Write(path, "")
	requireTrue(t, writeResult.OK)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)
	requireNotNil(t, sess)
	assertEmpty(t, sess.Events)
	assertTrue(t, sess.StartTime.IsZero())
}

// TestParser_ParseTranscriptMalformedJSON_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptMalformedJSON_Bad(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "malformed.jsonl",
		`{invalid json`,
		`{"type": "user", "timestamp": "`+ts(0)+`", not valid`,
		userTextEntry(ts(1), "This line is valid"),
		`}}}`,
		assistantTextEntry(ts(2), "This is also valid"),
	)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err, "malformed lines should be skipped, not cause an error")
	requireNotNil(t, sess)

	// Only the valid lines should produce events
	assertLen(t, sess.Events, 2)
	assertEqual(t, "user", sess.Events[0].Type)
	assertEqual(t, "assistant", sess.Events[1].Type)
}

// TestParser_ParseTranscriptTruncatedJSONL_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptTruncatedJSONL_Bad(t *testing.T) {
	dir := t.TempDir()
	validLine := userTextEntry(ts(0), "Hello")
	// Truncated line: cut a valid JSON line in half
	truncated := assistantTextEntry(ts(1), "World")
	truncated = truncated[:len(truncated)/2]

	path := writeJSONL(t, dir, "truncated.jsonl", validLine, truncated)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err, "truncated last line should be skipped gracefully")
	requireNotNil(t, sess)

	// Only the first valid line should produce an event
	assertLen(t, sess.Events, 1)
	assertEqual(t, "user", sess.Events[0].Type)
}

// TestParser_ParseTranscriptLargeSession_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptLargeSession_Good(t *testing.T) {
	dir := t.TempDir()

	var lines []string
	lines = append(lines, userTextEntry(ts(0), "Start large session"))

	// Generate 1000+ tool call pairs
	for i := range 1100 {
		toolID := core.Sprintf("tool-%d", i)
		offset := (i * 2) + 1
		lines = append(lines,
			toolUseEntry(ts(offset), "Bash", toolID, map[string]any{
				"command": core.Sprintf("echo %d", i),
			}),
			toolResultEntry(ts(offset+1), toolID, core.Sprintf("output %d", i), false),
		)
	}
	lines = append(lines, assistantTextEntry(ts(2202), "Done"))

	path := writeJSONL(t, dir, "large.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	var toolCount int
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolCount++
		}
	}
	assertEqual(t, 1100, toolCount, "all 1100 tool events should be parsed")
}

// TestParser_ParseTranscriptNestedToolResults_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptNestedToolResults_Good(t *testing.T) {
	dir := t.TempDir()

	// Tool result with array content (multiple text blocks)
	arrayContent := []map[string]any{
		{"type": "text", "text": "First block"},
		{"type": "text", "text": "Second block"},
	}

	lines := []string{
		toolUseEntry(ts(0), "Bash", "tool-nested-1", map[string]any{
			"command": "complex output",
		}),
		// Build the tool result with array content manually
		jsonlLine(map[string]any{
			"type":      "user",
			"timestamp": ts(1),
			"sessionId": "test-session",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "tool-nested-1",
						"content":     arrayContent,
						"is_error":    false,
					},
				},
			},
		}),
	}

	path := writeJSONL(t, dir, "nested.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	requireLen(t, toolEvents, 1)
	assertContains(t, toolEvents[0].Output, "First block")
	assertContains(t, toolEvents[0].Output, "Second block")
}

// TestParser_ParseTranscriptNestedMapResult_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptNestedMapResult_Good(t *testing.T) {
	dir := t.TempDir()

	lines := []string{
		toolUseEntry(ts(0), "Read", "tool-map-1", map[string]any{
			"file_path": "/tmp/data.json",
		}),
		// Build a tool result with map content
		jsonlLine(map[string]any{
			"type":      "user",
			"timestamp": ts(1),
			"sessionId": "test-session",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "tool-map-1",
						"content": map[string]any{
							"text": "file contents here",
						},
						"is_error": false,
					},
				},
			},
		}),
	}

	path := writeJSONL(t, dir, "map-result.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	requireLen(t, toolEvents, 1)
	assertContains(t, toolEvents[0].Output, "file contents here")
}

// TestParser_ParseTranscriptFileNotFound_Ugly verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptFileNotFound_Ugly(t *testing.T) {
	_, _, err := ParseTranscript("/nonexistent/path/session.jsonl")
	requireError(t, err)
	assertContains(t, err.Error(), "open transcript")
}

// TestParser_ParseTranscriptSessionIDFromFilename_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptSessionIDFromFilename_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "abc123def456.jsonl",
		userTextEntry(ts(0), "test"),
	)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)
	assertEqual(t, "abc123def456", sess.ID)
}

// TestParser_ParseTranscriptTimestampsTracked_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptTimestampsTracked_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "timestamps.jsonl",
		userTextEntry(ts(0), "start"),
		assistantTextEntry(ts(5), "middle"),
		userTextEntry(ts(10), "end"),
	)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	expectedStart, _ := time.Parse(time.RFC3339Nano, ts(0))
	expectedEnd, _ := time.Parse(time.RFC3339Nano, ts(10))

	assertEqual(t, expectedStart, sess.StartTime)
	assertEqual(t, expectedEnd, sess.EndTime)
}

// TestParser_ParseTranscriptTextTruncation_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptTextTruncation_Good(t *testing.T) {
	dir := t.TempDir()
	longText := repeatString("x", 600)
	path := writeJSONL(t, dir, "truncation.jsonl",
		userTextEntry(ts(0), longText),
	)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	requireLen(t, sess.Events, 1)
	// Input should be truncated to 500 + "..."
	assertTrue(t, len(sess.Events[0].Input) <= 504, "input should be truncated")
	assertTrue(t, core.HasSuffix(sess.Events[0].Input, "..."), "truncated text should end with ...")
}

// TestParser_SessionEventsSeq_Good verifies the behaviour covered by this test case.
func TestParser_SessionEventsSeq_Good(t *testing.T) {
	sess := &Session{
		Events: []Event{
			{Type: "user", Input: "one"},
			{Type: "assistant", Input: "two"},
			{Type: "tool_use", Tool: "Bash", Input: "three"},
		},
	}

	var events []Event
	for e := range sess.EventsSeq() {
		events = append(events, e)
	}

	assertEqual(t, sess.Events, events)
}

// TestParser_ParseTranscriptMixedContentBlocks_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptMixedContentBlocks_Good(t *testing.T) {
	// Assistant message with both text and tool_use in the same message
	dir := t.TempDir()

	lines := []string{
		// An assistant message with text + tool_use in the same content array
		jsonlLine(map[string]any{
			"type":      "assistant",
			"timestamp": ts(0),
			"sessionId": "test-session",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "Let me check that file."},
					{
						"type":  "tool_use",
						"name":  "Read",
						"id":    "tool-mixed-1",
						"input": map[string]any{"file_path": "/tmp/mix.go"},
					},
				},
			},
		}),
		toolResultEntry(ts(1), "tool-mixed-1", "package mix", false),
	}

	path := writeJSONL(t, dir, "mixed.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	// Should have an assistant text event + a tool_use event
	requireLen(t, sess.Events, 2)
	assertEqual(t, "assistant", sess.Events[0].Type)
	assertEqual(t, "tool_use", sess.Events[1].Type)
	assertEqual(t, "Read", sess.Events[1].Tool)
}

// TestParser_ParseTranscriptUnmatchedToolResult_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptUnmatchedToolResult_Bad(t *testing.T) {
	// A tool_result with no matching tool_use should be silently ignored
	dir := t.TempDir()
	path := writeJSONL(t, dir, "unmatched.jsonl",
		toolResultEntry(ts(0), "nonexistent-tool-id", "orphan result", false),
		userTextEntry(ts(1), "Normal message"),
	)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	// Only the user text event should appear; the orphan tool result is ignored
	requireLen(t, sess.Events, 1)
	assertEqual(t, "user", sess.Events[0].Type)
}

// TestParser_ParseTranscriptEmptyTimestamp_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptEmptyTimestamp_Bad(t *testing.T) {
	dir := t.TempDir()
	// Entry with empty timestamp
	line := jsonlLine(map[string]any{
		"type":      "user",
		"timestamp": "",
		"sessionId": "test-session",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": "No timestamp"},
			},
		},
	})
	path := writeJSONL(t, dir, "no-ts.jsonl", line)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	// The event should still be parsed, but StartTime remains zero
	assertTrue(t, sess.StartTime.IsZero())
}

// --- ListSessions tests ---

// TestParser_ListSessionsEmptyDir_Good verifies the behaviour covered by this test case.
func TestParser_ListSessionsEmptyDir_Good(t *testing.T) {
	dir := t.TempDir()

	sessions, err := ListSessions(dir)
	requireNoError(t, err)
	assertEmpty(t, sessions)
}

// TestParser_ListSessionsSingleSession_Good verifies the behaviour covered by this test case.
func TestParser_ListSessionsSingleSession_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "session-abc.jsonl",
		userTextEntry(ts(0), "Hello"),
		assistantTextEntry(ts(5), "World"),
	)

	sessions, err := ListSessions(dir)
	requireNoError(t, err)
	requireLen(t, sessions, 1)

	assertEqual(t, "session-abc", sessions[0].ID)
	assertFalse(t, sessions[0].StartTime.IsZero())
	assertFalse(t, sessions[0].EndTime.IsZero())
}

// TestParser_ListSessionsMultipleSorted_Good verifies the behaviour covered by this test case.
func TestParser_ListSessionsMultipleSorted_Good(t *testing.T) {
	dir := t.TempDir()

	// Create three sessions with different timestamps.
	// Session "old" starts at ts(0), "mid" at ts(100), "new" at ts(200).
	writeJSONL(t, dir, "old.jsonl",
		userTextEntry(ts(0), "old session"),
	)
	writeJSONL(t, dir, "mid.jsonl",
		userTextEntry(ts(100), "mid session"),
	)
	writeJSONL(t, dir, "new.jsonl",
		userTextEntry(ts(200), "new session"),
	)

	sessions, err := ListSessions(dir)
	requireNoError(t, err)
	requireLen(t, sessions, 3)

	// Should be sorted newest first
	assertEqual(t, "new", sessions[0].ID)
	assertEqual(t, "mid", sessions[1].ID)
	assertEqual(t, "old", sessions[2].ID)
}

// TestParser_ListSessionsNonJSONLIgnored_Good verifies the behaviour covered by this test case.
func TestParser_ListSessionsNonJSONLIgnored_Good(t *testing.T) {
	dir := t.TempDir()

	writeJSONL(t, dir, "real-session.jsonl",
		userTextEntry(ts(0), "real"),
	)
	// Write non-JSONL files
	requireTrue(t, hostFS.Write(path.Join(dir, "readme.md"), "# Hello").OK)
	requireTrue(t, hostFS.Write(path.Join(dir, "notes.txt"), "notes").OK)
	requireTrue(t, hostFS.Write(path.Join(dir, "data.json"), "{}").OK)

	sessions, err := ListSessions(dir)
	requireNoError(t, err)
	requireLen(t, sessions, 1)
	assertEqual(t, "real-session", sessions[0].ID)
}

// TestParser_ListSessionsSeqMultipleSorted_Good verifies the behaviour covered by this test case.
func TestParser_ListSessionsSeqMultipleSorted_Good(t *testing.T) {
	dir := t.TempDir()

	// Create three sessions with different timestamps.
	writeJSONL(t, dir, "old.jsonl", userTextEntry(ts(0), "old"))
	writeJSONL(t, dir, "mid.jsonl", userTextEntry(ts(100), "mid"))
	writeJSONL(t, dir, "new.jsonl", userTextEntry(ts(200), "new"))

	var sessions []Session
	for s := range ListSessionsSeq(dir) {
		sessions = append(sessions, s)
	}

	requireLen(t, sessions, 3)
	// Should be sorted newest first
	assertEqual(t, "new", sessions[0].ID)
	assertEqual(t, "mid", sessions[1].ID)
	assertEqual(t, "old", sessions[2].ID)
}

// TestParser_ListSessionsMalformedJSONLStillListed_Bad verifies the behaviour covered by this test case.
func TestParser_ListSessionsMalformedJSONLStillListed_Bad(t *testing.T) {
	dir := t.TempDir()

	// A .jsonl file with no valid timestamps — should still list with zero time or modtime
	writeJSONL(t, dir, "broken.jsonl",
		`{invalid json}`,
		`also not valid`,
	)

	sessions, err := ListSessions(dir)
	requireNoError(t, err)
	requireLen(t, sessions, 1)
	assertEqual(t, "broken", sessions[0].ID)
	// StartTime should fall back to file modtime since no valid timestamps
	assertFalse(t, sessions[0].StartTime.IsZero(), "should fall back to file modtime")
}

// --- extractToolInput tests ---

// TestParser_ExtractToolInputBash_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputBash_Good(t *testing.T) {
	input := rawJSON([]byte(`{"command":"go test ./...","description":"run tests","timeout":120}`))
	result := extractToolInput("Bash", input)
	assertEqual(t, "go test ./... # run tests", result)
}

// TestParser_ExtractToolInputBashNoDescription_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputBashNoDescription_Good(t *testing.T) {
	input := rawJSON([]byte(`{"command":"ls -la"}`))
	result := extractToolInput("Bash", input)
	assertEqual(t, "ls -la", result)
}

// TestParser_ExtractToolInputRead_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputRead_Good(t *testing.T) {
	input := rawJSON([]byte(`{"file_path":"/Users/test/main.go","offset":10,"limit":50}`))
	result := extractToolInput("Read", input)
	assertEqual(t, "/Users/test/main.go", result)
}

// TestParser_ExtractToolInputEdit_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputEdit_Good(t *testing.T) {
	input := rawJSON([]byte(`{"file_path":"/tmp/app.go","old_string":"foo","new_string":"bar"}`))
	result := extractToolInput("Edit", input)
	assertEqual(t, "/tmp/app.go (edit)", result)
}

// TestParser_ExtractToolInputWrite_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputWrite_Good(t *testing.T) {
	input := rawJSON([]byte(`{"file_path":"/tmp/out.txt","content":"hello world"}`))
	result := extractToolInput("Write", input)
	assertEqual(t, "/tmp/out.txt (11 bytes)", result)
}

// TestParser_ExtractToolInputGrep_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputGrep_Good(t *testing.T) {
	input := rawJSON([]byte(`{"pattern":"TODO","path":"/src"}`))
	result := extractToolInput("Grep", input)
	assertEqual(t, "/TODO/ in /src", result)
}

// TestParser_ExtractToolInputGrepNoPath_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputGrepNoPath_Good(t *testing.T) {
	input := rawJSON([]byte(`{"pattern":"FIXME"}`))
	result := extractToolInput("Grep", input)
	assertEqual(t, "/FIXME/ in .", result)
}

// TestParser_ExtractToolInputGlob_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputGlob_Good(t *testing.T) {
	input := rawJSON([]byte(`{"pattern":"**/*.go","path":"/src"}`))
	result := extractToolInput("Glob", input)
	assertEqual(t, "**/*.go", result)
}

// TestParser_ExtractToolInputTask_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputTask_Good(t *testing.T) {
	input := rawJSON([]byte(`{"prompt":"Analyse the codebase","description":"Code review","subagent_type":"research"}`))
	result := extractToolInput("Task", input)
	assertEqual(t, "[research] Code review", result)
}

// TestParser_ExtractToolInputTaskNoDescription_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputTaskNoDescription_Good(t *testing.T) {
	input := rawJSON([]byte(`{"prompt":"Short prompt","subagent_type":"codegen"}`))
	result := extractToolInput("Task", input)
	assertEqual(t, "[codegen] Short prompt", result)
}

// TestParser_ExtractToolInputUnknownTool_Good verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputUnknownTool_Good(t *testing.T) {
	input := rawJSON([]byte(`{"alpha":"one","beta":"two"}`))
	result := extractToolInput("CustomTool", input)
	// Fallback: sorted keys
	assertEqual(t, "alpha, beta", result)
}

// TestParser_ExtractToolInputNilInput_Bad verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputNilInput_Bad(t *testing.T) {
	var input rawJSON
	result := extractToolInput("Bash", input)

	assertEqual(t, "", result)
	assertEqual(t, 0, len(input))
}

// TestParser_ExtractToolInputInvalidJSON_Bad verifies the behaviour covered by this test case.
func TestParser_ExtractToolInputInvalidJSON_Bad(t *testing.T) {
	input := rawJSON([]byte(`{broken`))
	result := extractToolInput("Bash", input)
	// All unmarshals fail, including the fallback map unmarshal
	assertEqual(t, "", result)
}

// --- extractResultContent tests ---

// TestParser_ExtractResultContentString_Good verifies the behaviour covered by this test case.
func TestParser_ExtractResultContentString_Good(t *testing.T) {
	content := "simple string"
	result := extractResultContent(content)

	assertEqual(t, content, result)
	assertContains(t, result, "simple")
}

// TestParser_ExtractResultContentArray_Good verifies the behaviour covered by this test case.
func TestParser_ExtractResultContentArray_Good(t *testing.T) {
	content := []any{
		map[string]any{"type": "text", "text": "line one"},
		map[string]any{"type": "text", "text": "line two"},
	}
	result := extractResultContent(content)
	assertEqual(t, "line one\nline two", result)
}

// TestParser_ExtractResultContentMap_Good verifies the behaviour covered by this test case.
func TestParser_ExtractResultContentMap_Good(t *testing.T) {
	content := map[string]any{"text": "from map"}
	result := extractResultContent(content)
	assertEqual(t, "from map", result)
}

// TestParser_ExtractResultContentOther_Bad verifies the behaviour covered by this test case.
func TestParser_ExtractResultContentOther_Bad(t *testing.T) {
	content := 42
	result := extractResultContent(content)

	assertEqual(t, "42", result)
	assertEqual(t, "42", core.Sprintf("%d", content))
}

// --- truncate tests ---

// TestParser_TruncateShort_Good verifies the behaviour covered by this test case.
func TestParser_TruncateShort_Good(t *testing.T) {
	input := "hello"
	result := truncate(input, 10)

	assertEqual(t, input, result)
	assertEqual(t, len(input), len(result))
}

// TestParser_TruncateExact_Good verifies the behaviour covered by this test case.
func TestParser_TruncateExact_Good(t *testing.T) {
	input := "hello"
	result := truncate(input, len(input))

	assertEqual(t, input, result)
	assertEqual(t, len(input), len(result))
}

// TestParser_TruncateLong_Good verifies the behaviour covered by this test case.
func TestParser_TruncateLong_Good(t *testing.T) {
	input := "hello world"
	result := truncate("hello world", 5)

	assertEqual(t, "hello...", result)
	assertTrue(t, len(result) < len(input))
}

// TestParser_TruncateEmpty_Good verifies the behaviour covered by this test case.
func TestParser_TruncateEmpty_Good(t *testing.T) {
	input := ""
	result := truncate(input, 10)

	assertEqual(t, input, result)
	assertEqual(t, 0, len(result))
}

// --- helper function tests ---

// TestParser_ShortIDTruncatesAndPreservesLength_Good verifies the behaviour covered by this test case.
func TestParser_ShortIDTruncatesAndPreservesLength_Good(t *testing.T) {
	assertEqual(t, "abcdefgh", shortID("abcdefghijklmnop"))
	assertEqual(t, "short", shortID("short"))
	assertEqual(t, "12345678", shortID("12345678"))
}

// TestParser_FormatDurationCommonDurations_Good verifies the behaviour covered by this test case.
func TestParser_FormatDurationCommonDurations_Good(t *testing.T) {
	assertEqual(t, "500ms", formatDuration(500*time.Millisecond))
	assertEqual(t, "1.5s", formatDuration(1500*time.Millisecond))
	assertEqual(t, "2m30s", formatDuration(2*time.Minute+30*time.Second))
	assertEqual(t, "1h5m", formatDuration(1*time.Hour+5*time.Minute))
}

// --- ParseStats tests ---

// TestParser_ParseStatsCleanJSONL_Good verifies the behaviour covered by this test case.
func TestParser_ParseStatsCleanJSONL_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "clean.jsonl",
		userTextEntry(ts(0), "Hello"),
		toolUseEntry(ts(1), "Bash", "tool-1", map[string]any{
			"command": "ls",
		}),
		toolResultEntry(ts(2), "tool-1", "ok", false),
		assistantTextEntry(ts(3), "Done"),
	)

	_, stats, err := ParseTranscript(path)
	requireNoError(t, err)
	requireNotNil(t, stats)

	assertEqual(t, 4, stats.TotalLines)
	assertEqual(t, 0, stats.SkippedLines)
	assertEqual(t, 0, stats.OrphanedToolCalls)
	assertEmpty(t, stats.Warnings)
}

// TestParser_ParseStatsMalformedLines_Good verifies the behaviour covered by this test case.
func TestParser_ParseStatsMalformedLines_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "malformed-stats.jsonl",
		`{bad json line one`,
		userTextEntry(ts(0), "Valid line"),
		`{another bad line}}}`,
		`not even close to json`,
		assistantTextEntry(ts(1), "Also valid"),
	)

	_, stats, err := ParseTranscript(path)
	requireNoError(t, err)
	requireNotNil(t, stats)

	assertEqual(t, 5, stats.TotalLines)
	assertEqual(t, 3, stats.SkippedLines)
	assertLen(t, stats.Warnings, 3)

	// Each warning should contain line number and preview
	for _, w := range stats.Warnings {
		assertContains(t, w, "skipped (bad JSON)")
	}
}

// TestParser_ParseStatsOrphanedToolCalls_Ugly verifies the behaviour covered by this test case.
func TestParser_ParseStatsOrphanedToolCalls_Ugly(t *testing.T) {
	dir := t.TempDir()
	// Two tool_use entries with no matching tool_result
	path := writeJSONL(t, dir, "orphaned.jsonl",
		toolUseEntry(ts(0), "Bash", "orphan-1", map[string]any{
			"command": "ls",
		}),
		toolUseEntry(ts(1), "Read", "orphan-2", map[string]any{
			"file_path": "/tmp/file.go",
		}),
		assistantTextEntry(ts(2), "Never got results"),
	)

	_, stats, err := ParseTranscript(path)
	requireNoError(t, err)
	requireNotNil(t, stats)

	assertEqual(t, 2, stats.OrphanedToolCalls)

	// Warnings should mention orphaned tool IDs
	var orphanWarnings int
	for _, w := range stats.Warnings {
		if core.Contains(w, "orphaned tool call") {
			orphanWarnings++
		}
	}
	assertEqual(t, 2, orphanWarnings)
}

// TestParser_ParseStatsTruncatedFinalLine_Good verifies the behaviour covered by this test case.
func TestParser_ParseStatsTruncatedFinalLine_Good(t *testing.T) {
	dir := t.TempDir()
	validLine := userTextEntry(ts(0), "Hello")
	truncatedLine := `{"type":"assi`

	// Write without trailing newline after truncated line.
	path := path.Join(dir, "truncfinal.jsonl")
	requireTrue(t, hostFS.Write(path, validLine+"\n"+truncatedLine).OK)

	_, stats, err := ParseTranscript(path)
	requireNoError(t, err)
	requireNotNil(t, stats)

	assertEqual(t, 1, stats.SkippedLines)

	// Should detect truncated final line
	var foundTruncated bool
	for _, w := range stats.Warnings {
		if core.Contains(w, "truncated final line") {
			foundTruncated = true
		}
	}
	assertTrue(t, foundTruncated, "should detect truncated final line")
}

// TestParser_ParseStatsFileEndingMidJSON_Good verifies the behaviour covered by this test case.
func TestParser_ParseStatsFileEndingMidJSON_Good(t *testing.T) {
	dir := t.TempDir()
	validLine := userTextEntry(ts(0), "Hello")
	midJSON := `{"type":"assistant","timestamp":"2026-02-20T10:00:01Z","sessionId":"test","message":{"role":"assi`

	path := path.Join(dir, "midjson.jsonl")
	requireTrue(t, hostFS.Write(path, validLine+"\n"+midJSON).OK)

	sess, stats, err := ParseTranscript(path)
	requireNoError(t, err)
	requireNotNil(t, sess)
	requireNotNil(t, stats)

	assertEqual(t, 1, stats.SkippedLines)

	var foundTruncated bool
	for _, w := range stats.Warnings {
		if core.Contains(w, "truncated final line") {
			foundTruncated = true
		}
	}
	assertTrue(t, foundTruncated)
}

// TestParser_ParseStatsCompleteFileNoTrailingNewline_Good verifies the behaviour covered by this test case.
func TestParser_ParseStatsCompleteFileNoTrailingNewline_Good(t *testing.T) {
	dir := t.TempDir()
	line := userTextEntry(ts(0), "Hello")

	// Write without trailing newline — should still parse fine
	path := path.Join(dir, "nonewline.jsonl")
	requireTrue(t, hostFS.Write(path, line).OK)

	sess, stats, err := ParseTranscript(path)
	requireNoError(t, err)
	requireNotNil(t, sess)
	requireNotNil(t, stats)

	assertEqual(t, 0, stats.SkippedLines)
	assertEqual(t, 0, stats.OrphanedToolCalls)
	assertLen(t, sess.Events, 1)

	// No truncation warning since the line parsed successfully
	var foundTruncated bool
	for _, w := range stats.Warnings {
		if core.Contains(w, "truncated final line") {
			foundTruncated = true
		}
	}
	assertFalse(t, foundTruncated)
}

// TestParser_ParseStatsWarningPreviewTruncated_Good verifies the behaviour covered by this test case.
func TestParser_ParseStatsWarningPreviewTruncated_Good(t *testing.T) {
	dir := t.TempDir()
	// A malformed line longer than 100 chars
	longBadLine := `{` + repeatString("x", 200)
	path := writeJSONL(t, dir, "longbad.jsonl",
		longBadLine,
		userTextEntry(ts(0), "Valid"),
	)

	_, stats, err := ParseTranscript(path)
	requireNoError(t, err)

	requireLen(t, stats.Warnings, 1) // 1 skipped line (last line is valid, no truncation)
	// The preview in the warning should be at most ~100 chars of the bad line
	assertTrue(t, len(stats.Warnings[0]) < 200,
		"warning preview should be truncated for long lines")
	assertContains(t, stats.Warnings[0], "line 1:")
}

// --- ParseTranscriptReader (streaming) tests ---

// TestParser_ParseTranscriptReaderMinimalValid_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptReaderMinimalValid_Good(t *testing.T) {
	// Parse directly from an in-memory reader.
	data := core.Join("\n", []string{
		userTextEntry(ts(0), "hello"),
		assistantTextEntry(ts(1), "world"),
	}...) + "\n"
	reader := core.NewReader(data)

	sess, stats, err := ParseTranscriptReader(reader, "stream-session")
	requireNoError(t, err)
	requireNotNil(t, sess)
	requireNotNil(t, stats)

	assertEqual(t, "stream-session", sess.ID)
	assertEmpty(t, sess.Path, "reader-based parse should have empty path")
	assertLen(t, sess.Events, 2)
	assertEqual(t, "hello", sess.Events[0].Input)
	assertEqual(t, "world", sess.Events[1].Input)
	assertEqual(t, 2, stats.TotalLines)
	assertEqual(t, 0, stats.SkippedLines)
}

// TestParser_ParseTranscriptReaderBytesBuffer_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptReaderBytesBuffer_Good(t *testing.T) {
	// Parse from a bytes.Buffer (common streaming use case).
	data := core.Join("\n", []string{
		toolUseEntry(ts(0), "Bash", "tu-buf-1", map[string]any{
			"command":     "echo ok",
			"description": "test",
		}),
		toolResultEntry(ts(1), "tu-buf-1", "ok", false),
	}...) + "\n"
	buf := bytes.NewBufferString(data)

	sess, _, err := ParseTranscriptReader(buf, "buf-session")
	requireNoError(t, err)
	requireLen(t, sess.Events, 1)
	assertEqual(t, "Bash", sess.Events[0].Tool)
	assertTrue(t, sess.Events[0].Success)
}

// TestParser_ParseTranscriptReaderEmptyReader_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptReaderEmptyReader_Good(t *testing.T) {
	reader := core.NewReader("")

	sess, stats, err := ParseTranscriptReader(reader, "empty")
	requireNoError(t, err)
	requireNotNil(t, sess)
	assertEmpty(t, sess.Events)
	assertEqual(t, 0, stats.TotalLines)
}

// TestParser_ParseTranscriptReaderLargeLines_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptReaderLargeLines_Good(t *testing.T) {
	// Verify the scanner handles very long lines (> 64KB).
	longText := repeatString("x", 128*1024) // 128KB of text
	data := userTextEntry(ts(0), longText) + "\n"
	reader := core.NewReader(data)

	sess, _, err := ParseTranscriptReader(reader, "big-session")
	requireNoError(t, err)
	requireLen(t, sess.Events, 1)
	// Input gets truncated to 500 chars by the truncate function.
	assertLen(t, sess.Events[0].Input, 503) // 500 + "..."
}

// TestParser_ParseTranscriptReaderMalformedWithStats_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptReaderMalformedWithStats_Good(t *testing.T) {
	// Malformed lines in a reader should still produce correct stats.
	data := core.Join("\n", []string{
		`{bad json`,
		userTextEntry(ts(0), "valid"),
		`also bad`,
	}...) + "\n"
	reader := core.NewReader(data)

	sess, stats, err := ParseTranscriptReader(reader, "mixed")
	requireNoError(t, err)
	assertLen(t, sess.Events, 1)
	assertEqual(t, 3, stats.TotalLines)
	assertEqual(t, 2, stats.SkippedLines)
}

// TestParser_ParseTranscriptReaderOrphanedTools_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptReaderOrphanedTools_Good(t *testing.T) {
	// Tool calls without results should be tracked in stats.
	data := core.Join("\n", []string{
		toolUseEntry(ts(0), "Bash", "orphan-r1", map[string]any{
			"command": "ls",
		}),
		assistantTextEntry(ts(1), "No result arrived"),
	}...) + "\n"
	reader := core.NewReader(data)

	_, stats, err := ParseTranscriptReader(reader, "orphan-reader")
	requireNoError(t, err)
	assertEqual(t, 1, stats.OrphanedToolCalls)
}

// TestParser_ParseTranscriptToolUseInputTruncated_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptToolUseInputTruncated_Bad(t *testing.T) {
	// Pending tool inputs should not retain an entire scanner-sized line.
	hugeCommand := repeatString("x", 1024*1024)
	data := core.Join("\n", []string{
		toolUseEntry(ts(0), "Bash", "tool-big-input", map[string]any{
			"command": hugeCommand,
		}),
		toolResultEntry(ts(1), "tool-big-input", "ok", false),
	}...) + "\n"

	sess, _, err := ParseTranscriptReader(core.NewReader(data), "big-tool-input")
	requireNoError(t, err)
	requireLen(t, sess.Events, 1)
	assertLen(t, sess.Events[0].Input, 503)
}

// TestParser_ParseTranscriptPendingToolLimit_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptPendingToolLimit_Bad(t *testing.T) {
	// Unmatched tool_use entries are attacker-controlled and must be bounded.
	lines := make([]string, 0, maxPendingToolCalls+1)
	for i := range maxPendingToolCalls + 1 {
		toolID := core.Sprintf("orphan-%d", i)
		lines = append(lines, toolUseEntry(ts(i), "Bash", toolID, map[string]any{
			"command": "true",
		}))
	}
	data := core.Join("\n", lines...) + "\n"

	_, stats, err := ParseTranscriptReader(core.NewReader(data), "many-orphans")
	requireNoError(t, err)
	requireNotNil(t, stats)
	assertEqual(t, maxPendingToolCalls, stats.OrphanedToolCalls)
	assertContains(t, core.Join("\n", stats.Warnings...), "pending tool limit reached")
}

// TestParser_ParseTranscriptDeeplyNestedJSON_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptDeeplyNestedJSON_Bad(t *testing.T) {
	// Deep malformed JSON should be reported as a skipped line, not panic.
	deep := repeatString("[", 1200) + repeatString("]", 1200)
	data := deep + "\n" + userTextEntry(ts(0), "after deep json") + "\n"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	sess, stats, err := ParseTranscriptReader(core.NewReader(data), "deep-json")
	requireNoError(t, err)
	requireLen(t, sess.Events, 1)
	assertEqual(t, "after deep json", sess.Events[0].Input)
	assertEqual(t, 1, stats.SkippedLines)
}

// TestParser_ParseTranscriptUnexpectedToolTypes_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptUnexpectedToolTypes_Bad(t *testing.T) {
	// Unexpected input/content JSON types should not panic type extraction.
	data := core.Join("\n", []string{
		`{"type":"assistant","timestamp":"` + ts(0) + `","sessionId":"bad-types","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","id":"num-input","input":42}]}}`,
		`{"type":"user","timestamp":"` + ts(1) + `","sessionId":"bad-types","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"num-input","content":42,"is_error":false}]}}`,
	}...) + "\n"

	sess, _, err := ParseTranscriptReader(core.NewReader(data), "bad-types")
	requireNoError(t, err)
	requireLen(t, sess.Events, 1)
	assertEqual(t, "", sess.Events[0].Input)
	assertEqual(t, "42", sess.Events[0].Output)
}

// TestParser_ParseTranscriptUTF16SurrogateHalf_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptUTF16SurrogateHalf_Bad(t *testing.T) {
	// Lone UTF-16 surrogate escapes are accepted by encoding/json as replacement runes.
	data := `{"type":"user","timestamp":"` + ts(0) + `","sessionId":"utf","message":{"role":"user","content":[{"type":"text","text":"bad \ud800 text"}]}}` + "\n"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	sess, _, err := ParseTranscriptReader(core.NewReader(data), "utf-surrogate")
	requireNoError(t, err)
	requireLen(t, sess.Events, 1)
	assertContains(t, sess.Events[0].Input, "bad ")
}

// --- Custom MCP tool tests ---

// TestParser_ParseTranscriptCustomMCPTool_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptCustomMCPTool_Good(t *testing.T) {
	// A tool_use with a non-standard MCP tool name (e.g. mcp__server__tool).
	dir := t.TempDir()
	lines := []string{
		toolUseEntry(ts(0), "mcp__forge__create_issue", "tu-mcp-1", map[string]any{
			"title": "bug report",
			"body":  "something broke",
			"repo":  "core/go",
		}),
		toolResultEntry(ts(1), "tu-mcp-1", "Issue #42 created", false),
	}
	path := writeJSONL(t, dir, "mcp_tool.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	requireLen(t, toolEvents, 1)
	assertEqual(t, "mcp__forge__create_issue", toolEvents[0].Tool)
	// Fallback should show sorted keys.
	assertContains(t, toolEvents[0].Input, "body")
	assertContains(t, toolEvents[0].Input, "repo")
	assertContains(t, toolEvents[0].Input, "title")
	assertTrue(t, toolEvents[0].Success)
}

// TestParser_ParseTranscriptCustomMCPToolNestedInput_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptCustomMCPToolNestedInput_Good(t *testing.T) {
	// MCP tool with nested JSON input — should show top-level keys.
	dir := t.TempDir()
	lines := []string{
		toolUseEntry(ts(0), "mcp__db__query", "tu-nested-1", map[string]any{
			"query":  "SELECT *",
			"params": map[string]any{"limit": 10, "offset": 0},
		}),
		toolResultEntry(ts(1), "tu-nested-1", "3 rows returned", false),
	}
	path := writeJSONL(t, dir, "mcp_nested.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	requireLen(t, toolEvents, 1)
	assertContains(t, toolEvents[0].Input, "params")
	assertContains(t, toolEvents[0].Input, "query")
}

// TestParser_ParseTranscriptUnknownToolEmptyInput_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptUnknownToolEmptyInput_Good(t *testing.T) {
	// A tool_use with an empty input object.
	dir := t.TempDir()
	lines := []string{
		toolUseEntry(ts(0), "SomeTool", "tu-empty-1", map[string]any{}),
		toolResultEntry(ts(1), "tu-empty-1", "done", false),
	}
	path := writeJSONL(t, dir, "empty_input.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	requireLen(t, toolEvents, 1)
	// Empty object should produce empty string from fallback.
	assertEqual(t, "", toolEvents[0].Input)
}

// --- Edge case error recovery tests ---

// TestParser_ParseTranscriptBinaryGarbage_Ugly verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptBinaryGarbage_Ugly(t *testing.T) {
	// Binary garbage interspersed with valid lines — must not panic.
	dir := t.TempDir()
	garbage := string([]byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd})
	lines := []string{
		garbage,
		userTextEntry(ts(0), "survived"),
		garbage + garbage,
	}
	path := writeJSONL(t, dir, "binary.jsonl", lines...)

	sess, stats, err := ParseTranscript(path)
	requireNoError(t, err)

	// Only the valid line should produce an event.
	var userEvents []Event
	for _, e := range sess.Events {
		if e.Type == "user" {
			userEvents = append(userEvents, e)
		}
	}
	requireLen(t, userEvents, 1)
	assertEqual(t, "survived", userEvents[0].Input)
	assertEqual(t, 2, stats.SkippedLines)
}

// TestParser_ParseTranscriptNullBytes_Ugly verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptNullBytes_Ugly(t *testing.T) {
	// Lines with embedded null bytes.
	dir := t.TempDir()
	lines := []string{
		`{"type":"user","timestamp":"` + ts(0) + `","sessionId":"n","message":` + string([]byte{0x00}) + `}`,
		userTextEntry(ts(1), "ok"),
	}
	path := writeJSONL(t, dir, "null_bytes.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)
	assertLen(t, sess.Events, 1)
}

// TestParser_ParseTranscriptVeryLongLine_Ugly verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptVeryLongLine_Ugly(t *testing.T) {
	// A single line that exceeds the default bufio.Scanner buffer.
	// The parser should handle this without error thanks to the enlarged buffer.
	dir := t.TempDir()
	huge := repeatString("a", 5*1024*1024) // 5MB text
	path := writeJSONL(t, dir, "huge_line.jsonl",
		userTextEntry(ts(0), huge),
	)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)
	requireLen(t, sess.Events, 1)
}

// TestParser_ParseTranscriptMalformedMessageJSON_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptMalformedMessageJSON_Bad(t *testing.T) {
	// Valid outer JSON but the message field is not valid message structure.
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(0) + `","sessionId":"b","message":"not an object"}`,
		userTextEntry(ts(1), "ok"),
	}
	path := writeJSONL(t, dir, "bad_msg.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)
	// First line's message is a string, not object — should be skipped.
	assertLen(t, sess.Events, 1)
	assertEqual(t, "ok", sess.Events[0].Input)
}

// TestParser_ParseTranscriptMalformedContentBlock_Bad verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptMalformedContentBlock_Bad(t *testing.T) {
	// Valid message structure but content blocks are malformed.
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(0) + `","sessionId":"c","message":{"role":"assistant","content":["not a block object"]}}`,
		userTextEntry(ts(1), "still ok"),
	}
	path := writeJSONL(t, dir, "bad_block.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)
	assertLen(t, sess.Events, 1)
	assertEqual(t, "still ok", sess.Events[0].Input)
}

// TestParser_ParseTranscriptTruncatedMissingBrace_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptTruncatedMissingBrace_Good(t *testing.T) {
	// Final line is missing its closing brace — should be skipped gracefully.
	dir := t.TempDir()
	lines := []string{
		userTextEntry(ts(0), "valid"),
		assistantTextEntry(ts(1), "also valid"),
		`{"type":"user","timestamp":"` + ts(2) + `","sessionId":"t","message":{"role":"user","content":[{"type":"text","text":"truncated"`,
	}
	path := writeJSONL(t, dir, "trunc_brace.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)
	// Only the two complete lines should produce events.
	assertLen(t, sess.Events, 2)
	assertEqual(t, "valid", sess.Events[0].Input)
	assertEqual(t, "also valid", sess.Events[1].Input)
}

// TestParser_ParseTranscriptTruncatedMidKey_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptTruncatedMidKey_Good(t *testing.T) {
	// Line truncated in the middle of a JSON key.
	dir := t.TempDir()
	lines := []string{
		userTextEntry(ts(0), "first"),
		`{"type":"assis`,
	}
	path := writeJSONL(t, dir, "trunc_midkey.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	requireNoError(t, err)
	assertLen(t, sess.Events, 1)
	assertEqual(t, "first", sess.Events[0].Input)
}

// TestParser_ParseTranscriptAllBadLines_Good verifies the behaviour covered by this test case.
func TestParser_ParseTranscriptAllBadLines_Good(t *testing.T) {
	// Every line is truncated/malformed — result should be empty, no error.
	dir := t.TempDir()
	lines := []string{
		`{"type":"user","timestamp`,
		`{"broken`,
		`not even json`,
	}
	path := writeJSONL(t, dir, "all_bad.jsonl", lines...)

	sess, stats, err := ParseTranscript(path)
	requireNoError(t, err)
	assertEmpty(t, sess.Events)
	assertTrue(t, sess.StartTime.IsZero())
	assertEqual(t, 3, stats.SkippedLines)
}

// --- ListSessions with truncated files ---

// --- PruneSessions tests ---

// TestParser_PruneSessionsDeletesOldFiles_Good verifies the behaviour covered by this test case.
func TestParser_PruneSessionsDeletesOldFiles_Good(t *testing.T) {
	dir := t.TempDir()

	// Create a session file with an old modification time.
	path := writeJSONL(t, dir, "old-session.jsonl",
		userTextEntry(ts(0), "old"),
	)
	// Backdate the file's mtime by 2 hours.
	oldTime := time.Now().Add(-2 * time.Hour)
	requireNoError(t, setFileTimes(path, oldTime, oldTime))

	// Create a recent session file.
	writeJSONL(t, dir, "new-session.jsonl",
		userTextEntry(ts(0), "new"),
	)

	// Prune sessions older than 1 hour.
	deleted, err := PruneSessions(dir, 1*time.Hour)
	requireNoError(t, err)
	assertEqual(t, 1, deleted)

	// Verify only the new file remains.
	sessions, err := ListSessions(dir)
	requireNoError(t, err)
	requireLen(t, sessions, 1)
	assertEqual(t, "new-session", sessions[0].ID)
}

// TestParser_PruneSessionsNothingToDelete_Good verifies the behaviour covered by this test case.
func TestParser_PruneSessionsNothingToDelete_Good(t *testing.T) {
	dir := t.TempDir()

	writeJSONL(t, dir, "recent.jsonl",
		userTextEntry(ts(0), "fresh"),
	)

	deleted, err := PruneSessions(dir, 24*time.Hour)
	requireNoError(t, err)
	assertEqual(t, 0, deleted)
}

// TestParser_PruneSessionsEmptyDir_Good verifies the behaviour covered by this test case.
func TestParser_PruneSessionsEmptyDir_Good(t *testing.T) {
	dir := t.TempDir()

	deleted, err := PruneSessions(dir, 1*time.Hour)
	requireNoError(t, err)
	assertEqual(t, 0, deleted)
}

// --- IsExpired tests ---

// TestParser_IsExpiredRecentSession_Good verifies the behaviour covered by this test case.
func TestParser_IsExpiredRecentSession_Good(t *testing.T) {
	sess := &Session{
		EndTime: time.Now().Add(-5 * time.Minute),
	}
	assertFalse(t, sess.IsExpired(1*time.Hour))
}

// TestParser_IsExpiredOldSession_Good verifies the behaviour covered by this test case.
func TestParser_IsExpiredOldSession_Good(t *testing.T) {
	sess := &Session{
		EndTime: time.Now().Add(-2 * time.Hour),
	}
	assertTrue(t, sess.IsExpired(1*time.Hour))
}

// TestParser_IsExpiredZeroEndTime_Bad verifies the behaviour covered by this test case.
func TestParser_IsExpiredZeroEndTime_Bad(t *testing.T) {
	sess := &Session{}
	expired := sess.IsExpired(1 * time.Hour)

	assertFalse(t, expired)
	assertTrue(t, sess.EndTime.IsZero())
}

// --- FetchSession tests ---

// TestParser_FetchSessionValidID_Good verifies the behaviour covered by this test case.
func TestParser_FetchSessionValidID_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "abc123.jsonl",
		userTextEntry(ts(0), "hello"),
	)

	sess, stats, err := FetchSession(dir, "abc123")
	requireNoError(t, err)
	requireNotNil(t, sess)
	requireNotNil(t, stats)
	assertEqual(t, "abc123", sess.ID)
	assertLen(t, sess.Events, 1)
}

// TestParser_FetchSessionPathTraversal_Ugly verifies the behaviour covered by this test case.
func TestParser_FetchSessionPathTraversal_Ugly(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, "../etc/passwd")
	requireError(t, err)
	assertContains(t, err.Error(), "invalid session id")
}

// TestParser_FetchSessionBackslashTraversal_Ugly verifies the behaviour covered by this test case.
func TestParser_FetchSessionBackslashTraversal_Ugly(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, `foo\bar`)
	requireError(t, err)
	assertContains(t, err.Error(), "invalid session id")
}

// TestParser_FetchSessionForwardSlash_Ugly verifies the behaviour covered by this test case.
func TestParser_FetchSessionForwardSlash_Ugly(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, "foo/bar")
	requireError(t, err)
	assertContains(t, err.Error(), "invalid session id")
}

// TestParser_FetchSessionURLEncodedTraversal_Ugly verifies the behaviour covered by this test case.
func TestParser_FetchSessionURLEncodedTraversal_Ugly(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, "%2e%2e%2fetc%2fpasswd")
	requireError(t, err)
	assertNotContains(t, err.Error(), "/etc/passwd")
}

// TestParser_FetchSessionSymlinkTraversal_Ugly verifies the behaviour covered by this test case.
func TestParser_FetchSessionSymlinkTraversal_Ugly(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	outsideFile := writeJSONL(t, outside, "secret.jsonl",
		userTextEntry(ts(0), "outside"),
	)
	linkPath := path.Join(dir, "linked.jsonl")
	if err := syscall.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, _, err := FetchSession(dir, "linked")
	requireError(t, err)
	assertContains(t, err.Error(), "invalid session path")
}

// TestParser_FetchSessionNotFound_Bad verifies the behaviour covered by this test case.
func TestParser_FetchSessionNotFound_Bad(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, "nonexistent")
	requireError(t, err)
	assertContains(t, err.Error(), "open transcript")
}

// --- ListSessions with truncated files ---

// TestParser_ListSessionsTruncatedFile_Good verifies the behaviour covered by this test case.
func TestParser_ListSessionsTruncatedFile_Good(t *testing.T) {
	dir := t.TempDir()
	// A .jsonl file where some lines are truncated — ListSessions should
	// still extract timestamps from valid lines.
	lines := []string{
		userTextEntry(ts(0), "start"),
		`{"type":"assistant","truncated`,
		userTextEntry(ts(5), "end"),
	}
	writeJSONL(t, dir, "partial.jsonl", lines...)

	sessions, err := ListSessions(dir)
	requireNoError(t, err)
	requireLen(t, sessions, 1)
	assertFalse(t, sessions[0].StartTime.IsZero())
	assertFalse(t, sessions[0].EndTime.IsZero())
	// End time should reflect the last valid timestamp.
	assertTrue(t, sessions[0].EndTime.After(sessions[0].StartTime))
}

// TestParser_ListSessionsOversizedLineSkipped_Ugly verifies the behaviour covered by this test case.
func TestParser_ListSessionsOversizedLineSkipped_Ugly(t *testing.T) {
	dir := t.TempDir()
	filePath := path.Join(dir, "oversized.jsonl")
	oversizedLine := string(bytes.Repeat([]byte("x"), maxScannerBuffer+1))
	requireTrue(t, hostFS.Write(filePath, userTextEntry(ts(0), "start")+"\n"+oversizedLine).OK)

	sessions, err := ListSessions(dir)
	requireNoError(t, err)
	assertEmpty(t, sessions)
}

// TestParser_ListSessionsSymlinkTraversal_Ugly verifies the behaviour covered by this test case.
func TestParser_ListSessionsSymlinkTraversal_Ugly(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	outsideFile := writeJSONL(t, outside, "secret.jsonl",
		userTextEntry(ts(0), "outside"),
	)
	if err := syscall.Symlink(outsideFile, path.Join(dir, "linked.jsonl")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	sessions, err := ListSessions(dir)
	requireNoError(t, err)
	assertEmpty(t, sessions)
}

// --- PruneSessions tests ---
