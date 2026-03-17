// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers to build synthetic JSONL ---

// ts returns a stable timestamp offset by the given seconds from a fixed epoch.
func ts(offsetSec int) string {
	base := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(offsetSec) * time.Second).Format(time.RFC3339Nano)
}

// jsonlLine marshals an arbitrary map to a single JSONL line.
func jsonlLine(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
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
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
	require.NoError(t, err)
	return path
}

// --- ParseTranscript tests ---

func TestParseTranscript_MinimalValid_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "minimal.jsonl",
		userTextEntry(ts(0), "Hello"),
		assistantTextEntry(ts(1), "Hi there!"),
	)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)
	require.NotNil(t, sess)

	assert.Equal(t, "minimal", sess.ID)
	assert.Equal(t, path, sess.Path)
	assert.False(t, sess.StartTime.IsZero(), "StartTime should be set")
	assert.False(t, sess.EndTime.IsZero(), "EndTime should be set")
	assert.True(t, sess.EndTime.After(sess.StartTime) || sess.EndTime.Equal(sess.StartTime))

	// Should have a user event and an assistant event
	require.Len(t, sess.Events, 2)
	assert.Equal(t, "user", sess.Events[0].Type)
	assert.Equal(t, "Hello", sess.Events[0].Input)
	assert.Equal(t, "assistant", sess.Events[1].Type)
	assert.Equal(t, "Hi there!", sess.Events[1].Input)
}

func TestParseTranscript_ToolCalls_Good(t *testing.T) {
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
	require.NoError(t, err)

	// Count tool_use events
	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	require.Len(t, toolEvents, 7, "should have 7 tool_use events")

	// Verify each tool was parsed correctly
	assert.Equal(t, "Bash", toolEvents[0].Tool)
	assert.Contains(t, toolEvents[0].Input, "ls -la")
	assert.Contains(t, toolEvents[0].Input, "# list files")
	assert.True(t, toolEvents[0].Success)
	assert.Equal(t, time.Second, toolEvents[0].Duration)

	assert.Equal(t, "Read", toolEvents[1].Tool)
	assert.Equal(t, "/tmp/test.go", toolEvents[1].Input)

	assert.Equal(t, "Edit", toolEvents[2].Tool)
	assert.Equal(t, "/tmp/test.go (edit)", toolEvents[2].Input)

	assert.Equal(t, "Write", toolEvents[3].Tool)
	assert.Equal(t, "/tmp/new.go (12 bytes)", toolEvents[3].Input)

	assert.Equal(t, "Grep", toolEvents[4].Tool)
	assert.Equal(t, "/TODO/ in /tmp", toolEvents[4].Input)

	assert.Equal(t, "Glob", toolEvents[5].Tool)
	assert.Equal(t, "**/*.go", toolEvents[5].Input)

	assert.Equal(t, "Task", toolEvents[6].Tool)
	assert.Equal(t, "[research] Code analysis", toolEvents[6].Input)
}

func TestParseTranscript_ToolError_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "error.jsonl",
		toolUseEntry(ts(0), "Bash", "tool-err-1", map[string]any{
			"command": "cat /nonexistent",
		}),
		toolResultEntry(ts(1), "tool-err-1", "cat: /nonexistent: No such file or directory", true),
	)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	require.Len(t, toolEvents, 1)
	assert.False(t, toolEvents[0].Success)
	assert.Contains(t, toolEvents[0].ErrorMsg, "No such file or directory")
}

func TestParseTranscript_EmptyFile_Bad(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "empty.jsonl")
	// Write a truly empty file
	require.NoError(t, os.WriteFile(path, []byte(""), 0644))

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.Empty(t, sess.Events)
	assert.True(t, sess.StartTime.IsZero())
}

func TestParseTranscript_MalformedJSON_Bad(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "malformed.jsonl",
		`{invalid json`,
		`{"type": "user", "timestamp": "`+ts(0)+`", not valid`,
		userTextEntry(ts(1), "This line is valid"),
		`}}}`,
		assistantTextEntry(ts(2), "This is also valid"),
	)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err, "malformed lines should be skipped, not cause an error")
	require.NotNil(t, sess)

	// Only the valid lines should produce events
	assert.Len(t, sess.Events, 2)
	assert.Equal(t, "user", sess.Events[0].Type)
	assert.Equal(t, "assistant", sess.Events[1].Type)
}

func TestParseTranscript_TruncatedJSONL_Bad(t *testing.T) {
	dir := t.TempDir()
	validLine := userTextEntry(ts(0), "Hello")
	// Truncated line: cut a valid JSON line in half
	truncated := assistantTextEntry(ts(1), "World")
	truncated = truncated[:len(truncated)/2]

	path := writeJSONL(t, dir, "truncated.jsonl", validLine, truncated)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err, "truncated last line should be skipped gracefully")
	require.NotNil(t, sess)

	// Only the first valid line should produce an event
	assert.Len(t, sess.Events, 1)
	assert.Equal(t, "user", sess.Events[0].Type)
}

func TestParseTranscript_LargeSession_Good(t *testing.T) {
	dir := t.TempDir()

	var lines []string
	lines = append(lines, userTextEntry(ts(0), "Start large session"))

	// Generate 1000+ tool call pairs
	for i := range 1100 {
		toolID := fmt.Sprintf("tool-%d", i)
		offset := (i * 2) + 1
		lines = append(lines,
			toolUseEntry(ts(offset), "Bash", toolID, map[string]any{
				"command": fmt.Sprintf("echo %d", i),
			}),
			toolResultEntry(ts(offset+1), toolID, fmt.Sprintf("output %d", i), false),
		)
	}
	lines = append(lines, assistantTextEntry(ts(2202), "Done"))

	path := writeJSONL(t, dir, "large.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)

	var toolCount int
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolCount++
		}
	}
	assert.Equal(t, 1100, toolCount, "all 1100 tool events should be parsed")
}

func TestParseTranscript_NestedToolResults_Good(t *testing.T) {
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
	require.NoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	require.Len(t, toolEvents, 1)
	assert.Contains(t, toolEvents[0].Output, "First block")
	assert.Contains(t, toolEvents[0].Output, "Second block")
}

func TestParseTranscript_NestedMapResult_Good(t *testing.T) {
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
	require.NoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	require.Len(t, toolEvents, 1)
	assert.Contains(t, toolEvents[0].Output, "file contents here")
}

func TestParseTranscript_FileNotFound_Ugly(t *testing.T) {
	_, _, err := ParseTranscript("/nonexistent/path/session.jsonl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open transcript")
}

func TestParseTranscript_SessionIDFromFilename_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "abc123def456.jsonl",
		userTextEntry(ts(0), "test"),
	)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)
	assert.Equal(t, "abc123def456", sess.ID)
}

func TestParseTranscript_TimestampsTracked_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "timestamps.jsonl",
		userTextEntry(ts(0), "start"),
		assistantTextEntry(ts(5), "middle"),
		userTextEntry(ts(10), "end"),
	)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)

	expectedStart, _ := time.Parse(time.RFC3339Nano, ts(0))
	expectedEnd, _ := time.Parse(time.RFC3339Nano, ts(10))

	assert.Equal(t, expectedStart, sess.StartTime)
	assert.Equal(t, expectedEnd, sess.EndTime)
}

func TestParseTranscript_TextTruncation_Good(t *testing.T) {
	dir := t.TempDir()
	longText := strings.Repeat("x", 600)
	path := writeJSONL(t, dir, "truncation.jsonl",
		userTextEntry(ts(0), longText),
	)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)

	require.Len(t, sess.Events, 1)
	// Input should be truncated to 500 + "..."
	assert.True(t, len(sess.Events[0].Input) <= 504, "input should be truncated")
	assert.True(t, strings.HasSuffix(sess.Events[0].Input, "..."), "truncated text should end with ...")
}

func TestSession_EventsSeq_Good(t *testing.T) {
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

	assert.Equal(t, sess.Events, events)
}

func TestParseTranscript_MixedContentBlocks_Good(t *testing.T) {
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
	require.NoError(t, err)

	// Should have an assistant text event + a tool_use event
	require.Len(t, sess.Events, 2)
	assert.Equal(t, "assistant", sess.Events[0].Type)
	assert.Equal(t, "tool_use", sess.Events[1].Type)
	assert.Equal(t, "Read", sess.Events[1].Tool)
}

func TestParseTranscript_UnmatchedToolResult_Bad(t *testing.T) {
	// A tool_result with no matching tool_use should be silently ignored
	dir := t.TempDir()
	path := writeJSONL(t, dir, "unmatched.jsonl",
		toolResultEntry(ts(0), "nonexistent-tool-id", "orphan result", false),
		userTextEntry(ts(1), "Normal message"),
	)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)

	// Only the user text event should appear; the orphan tool result is ignored
	require.Len(t, sess.Events, 1)
	assert.Equal(t, "user", sess.Events[0].Type)
}

func TestParseTranscript_EmptyTimestamp_Bad(t *testing.T) {
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
	require.NoError(t, err)

	// The event should still be parsed, but StartTime remains zero
	assert.True(t, sess.StartTime.IsZero())
}

// --- ListSessions tests ---

func TestListSessions_EmptyDir_Good(t *testing.T) {
	dir := t.TempDir()

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestListSessions_SingleSession_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "session-abc.jsonl",
		userTextEntry(ts(0), "Hello"),
		assistantTextEntry(ts(5), "World"),
	)

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	assert.Equal(t, "session-abc", sessions[0].ID)
	assert.False(t, sessions[0].StartTime.IsZero())
	assert.False(t, sessions[0].EndTime.IsZero())
}

func TestListSessions_MultipleSorted_Good(t *testing.T) {
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
	require.NoError(t, err)
	require.Len(t, sessions, 3)

	// Should be sorted newest first
	assert.Equal(t, "new", sessions[0].ID)
	assert.Equal(t, "mid", sessions[1].ID)
	assert.Equal(t, "old", sessions[2].ID)
}

func TestListSessions_NonJSONLIgnored_Good(t *testing.T) {
	dir := t.TempDir()

	writeJSONL(t, dir, "real-session.jsonl",
		userTextEntry(ts(0), "real"),
	)
	// Write non-JSONL files
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Hello"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("notes"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0644))

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "real-session", sessions[0].ID)
}

func TestListSessionsSeq_MultipleSorted_Good(t *testing.T) {
	dir := t.TempDir()

	// Create three sessions with different timestamps.
	writeJSONL(t, dir, "old.jsonl", userTextEntry(ts(0), "old"))
	writeJSONL(t, dir, "mid.jsonl", userTextEntry(ts(100), "mid"))
	writeJSONL(t, dir, "new.jsonl", userTextEntry(ts(200), "new"))

	var sessions []Session
	for s := range ListSessionsSeq(dir) {
		sessions = append(sessions, s)
	}

	require.Len(t, sessions, 3)
	// Should be sorted newest first
	assert.Equal(t, "new", sessions[0].ID)
	assert.Equal(t, "mid", sessions[1].ID)
	assert.Equal(t, "old", sessions[2].ID)
}

func TestListSessions_MalformedJSONLStillListed_Bad(t *testing.T) {
	dir := t.TempDir()

	// A .jsonl file with no valid timestamps — should still list with zero time or modtime
	writeJSONL(t, dir, "broken.jsonl",
		`{invalid json}`,
		`also not valid`,
	)

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "broken", sessions[0].ID)
	// StartTime should fall back to file modtime since no valid timestamps
	assert.False(t, sessions[0].StartTime.IsZero(), "should fall back to file modtime")
}

// --- extractToolInput tests ---

func TestExtractToolInput_Bash_Good(t *testing.T) {
	input := json.RawMessage(`{"command":"go test ./...","description":"run tests","timeout":120}`)
	result := extractToolInput("Bash", input)
	assert.Equal(t, "go test ./... # run tests", result)
}

func TestExtractToolInput_BashNoDescription_Good(t *testing.T) {
	input := json.RawMessage(`{"command":"ls -la"}`)
	result := extractToolInput("Bash", input)
	assert.Equal(t, "ls -la", result)
}

func TestExtractToolInput_Read_Good(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/Users/test/main.go","offset":10,"limit":50}`)
	result := extractToolInput("Read", input)
	assert.Equal(t, "/Users/test/main.go", result)
}

func TestExtractToolInput_Edit_Good(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/tmp/app.go","old_string":"foo","new_string":"bar"}`)
	result := extractToolInput("Edit", input)
	assert.Equal(t, "/tmp/app.go (edit)", result)
}

func TestExtractToolInput_Write_Good(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/tmp/out.txt","content":"hello world"}`)
	result := extractToolInput("Write", input)
	assert.Equal(t, "/tmp/out.txt (11 bytes)", result)
}

func TestExtractToolInput_Grep_Good(t *testing.T) {
	input := json.RawMessage(`{"pattern":"TODO","path":"/src"}`)
	result := extractToolInput("Grep", input)
	assert.Equal(t, "/TODO/ in /src", result)
}

func TestExtractToolInput_GrepNoPath_Good(t *testing.T) {
	input := json.RawMessage(`{"pattern":"FIXME"}`)
	result := extractToolInput("Grep", input)
	assert.Equal(t, "/FIXME/ in .", result)
}

func TestExtractToolInput_Glob_Good(t *testing.T) {
	input := json.RawMessage(`{"pattern":"**/*.go","path":"/src"}`)
	result := extractToolInput("Glob", input)
	assert.Equal(t, "**/*.go", result)
}

func TestExtractToolInput_Task_Good(t *testing.T) {
	input := json.RawMessage(`{"prompt":"Analyse the codebase","description":"Code review","subagent_type":"research"}`)
	result := extractToolInput("Task", input)
	assert.Equal(t, "[research] Code review", result)
}

func TestExtractToolInput_TaskNoDescription_Good(t *testing.T) {
	input := json.RawMessage(`{"prompt":"Short prompt","subagent_type":"codegen"}`)
	result := extractToolInput("Task", input)
	assert.Equal(t, "[codegen] Short prompt", result)
}

func TestExtractToolInput_UnknownTool_Good(t *testing.T) {
	input := json.RawMessage(`{"alpha":"one","beta":"two"}`)
	result := extractToolInput("CustomTool", input)
	// Fallback: sorted keys
	assert.Equal(t, "alpha, beta", result)
}

func TestExtractToolInput_NilInput_Bad(t *testing.T) {
	result := extractToolInput("Bash", nil)
	assert.Equal(t, "", result)
}

func TestExtractToolInput_InvalidJSON_Bad(t *testing.T) {
	input := json.RawMessage(`{broken`)
	result := extractToolInput("Bash", input)
	// All unmarshals fail, including the fallback map unmarshal
	assert.Equal(t, "", result)
}

// --- extractResultContent tests ---

func TestExtractResultContent_String_Good(t *testing.T) {
	result := extractResultContent("simple string")
	assert.Equal(t, "simple string", result)
}

func TestExtractResultContent_Array_Good(t *testing.T) {
	content := []any{
		map[string]any{"type": "text", "text": "line one"},
		map[string]any{"type": "text", "text": "line two"},
	}
	result := extractResultContent(content)
	assert.Equal(t, "line one\nline two", result)
}

func TestExtractResultContent_Map_Good(t *testing.T) {
	content := map[string]any{"text": "from map"}
	result := extractResultContent(content)
	assert.Equal(t, "from map", result)
}

func TestExtractResultContent_Other_Bad(t *testing.T) {
	result := extractResultContent(42)
	assert.Equal(t, "42", result)
}

// --- truncate tests ---

func TestTruncate_Short_Good(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
}

func TestTruncate_Exact_Good(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 5))
}

func TestTruncate_Long_Good(t *testing.T) {
	result := truncate("hello world", 5)
	assert.Equal(t, "hello...", result)
}

func TestTruncate_Empty_Good(t *testing.T) {
	assert.Equal(t, "", truncate("", 10))
}

// --- helper function tests ---

func TestShortID_Good(t *testing.T) {
	assert.Equal(t, "abcdefgh", shortID("abcdefghijklmnop"))
	assert.Equal(t, "short", shortID("short"))
	assert.Equal(t, "12345678", shortID("12345678"))
}

func TestFormatDuration_Good(t *testing.T) {
	assert.Equal(t, "500ms", formatDuration(500*time.Millisecond))
	assert.Equal(t, "1.5s", formatDuration(1500*time.Millisecond))
	assert.Equal(t, "2m30s", formatDuration(2*time.Minute+30*time.Second))
	assert.Equal(t, "1h5m", formatDuration(1*time.Hour+5*time.Minute))
}

// --- ParseStats tests ---

func TestParseStats_CleanJSONL_Good(t *testing.T) {
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
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 4, stats.TotalLines)
	assert.Equal(t, 0, stats.SkippedLines)
	assert.Equal(t, 0, stats.OrphanedToolCalls)
	assert.Empty(t, stats.Warnings)
}

func TestParseStats_MalformedLines_Good(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONL(t, dir, "malformed-stats.jsonl",
		`{bad json line one`,
		userTextEntry(ts(0), "Valid line"),
		`{another bad line}}}`,
		`not even close to json`,
		assistantTextEntry(ts(1), "Also valid"),
	)

	_, stats, err := ParseTranscript(path)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 5, stats.TotalLines)
	assert.Equal(t, 3, stats.SkippedLines)
	assert.Len(t, stats.Warnings, 3)

	// Each warning should contain line number and preview
	for _, w := range stats.Warnings {
		assert.Contains(t, w, "skipped (bad JSON)")
	}
}

func TestParseStats_OrphanedToolCalls_Good(t *testing.T) {
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
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 2, stats.OrphanedToolCalls)

	// Warnings should mention orphaned tool IDs
	var orphanWarnings int
	for _, w := range stats.Warnings {
		if strings.Contains(w, "orphaned tool call") {
			orphanWarnings++
		}
	}
	assert.Equal(t, 2, orphanWarnings)
}

func TestParseStats_TruncatedFinalLine_Good(t *testing.T) {
	dir := t.TempDir()
	validLine := userTextEntry(ts(0), "Hello")
	truncatedLine := `{"type":"assi`

	// Write without trailing newline after truncated line
	path := filepath.Join(dir, "truncfinal.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(validLine+"\n"+truncatedLine+"\n"), 0644))

	_, stats, err := ParseTranscript(path)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 1, stats.SkippedLines)

	// Should detect truncated final line
	var foundTruncated bool
	for _, w := range stats.Warnings {
		if strings.Contains(w, "truncated final line") {
			foundTruncated = true
		}
	}
	assert.True(t, foundTruncated, "should detect truncated final line")
}

func TestParseStats_FileEndingMidJSON_Good(t *testing.T) {
	dir := t.TempDir()
	validLine := userTextEntry(ts(0), "Hello")
	midJSON := `{"type":"assistant","timestamp":"2026-02-20T10:00:01Z","sessionId":"test","message":{"role":"assi`

	path := filepath.Join(dir, "midjson.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(validLine+"\n"+midJSON+"\n"), 0644))

	sess, stats, err := ParseTranscript(path)
	require.NoError(t, err)
	require.NotNil(t, sess)
	require.NotNil(t, stats)

	assert.Equal(t, 1, stats.SkippedLines)

	var foundTruncated bool
	for _, w := range stats.Warnings {
		if strings.Contains(w, "truncated final line") {
			foundTruncated = true
		}
	}
	assert.True(t, foundTruncated)
}

func TestParseStats_CompleteFileNoTrailingNewline_Good(t *testing.T) {
	dir := t.TempDir()
	line := userTextEntry(ts(0), "Hello")

	// Write without trailing newline — should still parse fine
	path := filepath.Join(dir, "nonewline.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(line), 0644))

	sess, stats, err := ParseTranscript(path)
	require.NoError(t, err)
	require.NotNil(t, sess)
	require.NotNil(t, stats)

	assert.Equal(t, 0, stats.SkippedLines)
	assert.Equal(t, 0, stats.OrphanedToolCalls)
	assert.Len(t, sess.Events, 1)

	// No truncation warning since the line parsed successfully
	var foundTruncated bool
	for _, w := range stats.Warnings {
		if strings.Contains(w, "truncated final line") {
			foundTruncated = true
		}
	}
	assert.False(t, foundTruncated)
}

func TestParseStats_WarningPreviewTruncated_Good(t *testing.T) {
	dir := t.TempDir()
	// A malformed line longer than 100 chars
	longBadLine := `{` + strings.Repeat("x", 200)
	path := writeJSONL(t, dir, "longbad.jsonl",
		longBadLine,
		userTextEntry(ts(0), "Valid"),
	)

	_, stats, err := ParseTranscript(path)
	require.NoError(t, err)

	require.Len(t, stats.Warnings, 1) // 1 skipped line (last line is valid, no truncation)
	// The preview in the warning should be at most ~100 chars of the bad line
	assert.True(t, len(stats.Warnings[0]) < 200,
		"warning preview should be truncated for long lines")
	assert.Contains(t, stats.Warnings[0], "line 1:")
}

// --- ParseTranscriptReader (streaming) tests ---

func TestParseTranscriptReader_MinimalValid_Good(t *testing.T) {
	// Parse directly from an in-memory reader.
	data := strings.Join([]string{
		userTextEntry(ts(0), "hello"),
		assistantTextEntry(ts(1), "world"),
	}, "\n") + "\n"
	reader := strings.NewReader(data)

	sess, stats, err := ParseTranscriptReader(reader, "stream-session")
	require.NoError(t, err)
	require.NotNil(t, sess)
	require.NotNil(t, stats)

	assert.Equal(t, "stream-session", sess.ID)
	assert.Empty(t, sess.Path, "reader-based parse should have empty path")
	assert.Len(t, sess.Events, 2)
	assert.Equal(t, "hello", sess.Events[0].Input)
	assert.Equal(t, "world", sess.Events[1].Input)
	assert.Equal(t, 2, stats.TotalLines)
	assert.Equal(t, 0, stats.SkippedLines)
}

func TestParseTranscriptReader_BytesBuffer_Good(t *testing.T) {
	// Parse from a bytes.Buffer (common streaming use case).
	data := strings.Join([]string{
		toolUseEntry(ts(0), "Bash", "tu-buf-1", map[string]any{
			"command":     "echo ok",
			"description": "test",
		}),
		toolResultEntry(ts(1), "tu-buf-1", "ok", false),
	}, "\n") + "\n"
	buf := bytes.NewBufferString(data)

	sess, _, err := ParseTranscriptReader(buf, "buf-session")
	require.NoError(t, err)
	require.Len(t, sess.Events, 1)
	assert.Equal(t, "Bash", sess.Events[0].Tool)
	assert.True(t, sess.Events[0].Success)
}

func TestParseTranscriptReader_EmptyReader_Good(t *testing.T) {
	reader := strings.NewReader("")

	sess, stats, err := ParseTranscriptReader(reader, "empty")
	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.Empty(t, sess.Events)
	assert.Equal(t, 0, stats.TotalLines)
}

func TestParseTranscriptReader_LargeLines_Good(t *testing.T) {
	// Verify the scanner handles very long lines (> 64KB).
	longText := strings.Repeat("x", 128*1024) // 128KB of text
	data := userTextEntry(ts(0), longText) + "\n"
	reader := strings.NewReader(data)

	sess, _, err := ParseTranscriptReader(reader, "big-session")
	require.NoError(t, err)
	require.Len(t, sess.Events, 1)
	// Input gets truncated to 500 chars by the truncate function.
	assert.Len(t, sess.Events[0].Input, 503) // 500 + "..."
}

func TestParseTranscriptReader_MalformedWithStats_Good(t *testing.T) {
	// Malformed lines in a reader should still produce correct stats.
	data := strings.Join([]string{
		`{bad json`,
		userTextEntry(ts(0), "valid"),
		`also bad`,
	}, "\n") + "\n"
	reader := strings.NewReader(data)

	sess, stats, err := ParseTranscriptReader(reader, "mixed")
	require.NoError(t, err)
	assert.Len(t, sess.Events, 1)
	assert.Equal(t, 3, stats.TotalLines)
	assert.Equal(t, 2, stats.SkippedLines)
}

func TestParseTranscriptReader_OrphanedTools_Good(t *testing.T) {
	// Tool calls without results should be tracked in stats.
	data := strings.Join([]string{
		toolUseEntry(ts(0), "Bash", "orphan-r1", map[string]any{
			"command": "ls",
		}),
		assistantTextEntry(ts(1), "No result arrived"),
	}, "\n") + "\n"
	reader := strings.NewReader(data)

	_, stats, err := ParseTranscriptReader(reader, "orphan-reader")
	require.NoError(t, err)
	assert.Equal(t, 1, stats.OrphanedToolCalls)
}

// --- Custom MCP tool tests ---

func TestParseTranscript_CustomMCPTool_Good(t *testing.T) {
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
	require.NoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	require.Len(t, toolEvents, 1)
	assert.Equal(t, "mcp__forge__create_issue", toolEvents[0].Tool)
	// Fallback should show sorted keys.
	assert.Contains(t, toolEvents[0].Input, "body")
	assert.Contains(t, toolEvents[0].Input, "repo")
	assert.Contains(t, toolEvents[0].Input, "title")
	assert.True(t, toolEvents[0].Success)
}

func TestParseTranscript_CustomMCPToolNestedInput_Good(t *testing.T) {
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
	require.NoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	require.Len(t, toolEvents, 1)
	assert.Contains(t, toolEvents[0].Input, "params")
	assert.Contains(t, toolEvents[0].Input, "query")
}

func TestParseTranscript_UnknownToolEmptyInput_Good(t *testing.T) {
	// A tool_use with an empty input object.
	dir := t.TempDir()
	lines := []string{
		toolUseEntry(ts(0), "SomeTool", "tu-empty-1", map[string]any{}),
		toolResultEntry(ts(1), "tu-empty-1", "done", false),
	}
	path := writeJSONL(t, dir, "empty_input.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)

	var toolEvents []Event
	for _, e := range sess.Events {
		if e.Type == "tool_use" {
			toolEvents = append(toolEvents, e)
		}
	}

	require.Len(t, toolEvents, 1)
	// Empty object should produce empty string from fallback.
	assert.Equal(t, "", toolEvents[0].Input)
}

// --- Edge case error recovery tests ---

func TestParseTranscript_BinaryGarbage_Ugly(t *testing.T) {
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
	require.NoError(t, err)

	// Only the valid line should produce an event.
	var userEvents []Event
	for _, e := range sess.Events {
		if e.Type == "user" {
			userEvents = append(userEvents, e)
		}
	}
	require.Len(t, userEvents, 1)
	assert.Equal(t, "survived", userEvents[0].Input)
	assert.Equal(t, 2, stats.SkippedLines)
}

func TestParseTranscript_NullBytes_Ugly(t *testing.T) {
	// Lines with embedded null bytes.
	dir := t.TempDir()
	lines := []string{
		`{"type":"user","timestamp":"` + ts(0) + `","sessionId":"n","message":` + string([]byte{0x00}) + `}`,
		userTextEntry(ts(1), "ok"),
	}
	path := writeJSONL(t, dir, "null_bytes.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)
	assert.Len(t, sess.Events, 1)
}

func TestParseTranscript_VeryLongLine_Ugly(t *testing.T) {
	// A single line that exceeds the default bufio.Scanner buffer.
	// The parser should handle this without error thanks to the enlarged buffer.
	dir := t.TempDir()
	huge := strings.Repeat("a", 5*1024*1024) // 5MB text
	path := writeJSONL(t, dir, "huge_line.jsonl",
		userTextEntry(ts(0), huge),
	)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)
	require.Len(t, sess.Events, 1)
}

func TestParseTranscript_MalformedMessageJSON_Bad(t *testing.T) {
	// Valid outer JSON but the message field is not valid message structure.
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(0) + `","sessionId":"b","message":"not an object"}`,
		userTextEntry(ts(1), "ok"),
	}
	path := writeJSONL(t, dir, "bad_msg.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)
	// First line's message is a string, not object — should be skipped.
	assert.Len(t, sess.Events, 1)
	assert.Equal(t, "ok", sess.Events[0].Input)
}

func TestParseTranscript_MalformedContentBlock_Bad(t *testing.T) {
	// Valid message structure but content blocks are malformed.
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(0) + `","sessionId":"c","message":{"role":"assistant","content":["not a block object"]}}`,
		userTextEntry(ts(1), "still ok"),
	}
	path := writeJSONL(t, dir, "bad_block.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)
	assert.Len(t, sess.Events, 1)
	assert.Equal(t, "still ok", sess.Events[0].Input)
}

func TestParseTranscript_TruncatedMissingBrace_Good(t *testing.T) {
	// Final line is missing its closing brace — should be skipped gracefully.
	dir := t.TempDir()
	lines := []string{
		userTextEntry(ts(0), "valid"),
		assistantTextEntry(ts(1), "also valid"),
		`{"type":"user","timestamp":"` + ts(2) + `","sessionId":"t","message":{"role":"user","content":[{"type":"text","text":"truncated"`,
	}
	path := writeJSONL(t, dir, "trunc_brace.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)
	// Only the two complete lines should produce events.
	assert.Len(t, sess.Events, 2)
	assert.Equal(t, "valid", sess.Events[0].Input)
	assert.Equal(t, "also valid", sess.Events[1].Input)
}

func TestParseTranscript_TruncatedMidKey_Good(t *testing.T) {
	// Line truncated in the middle of a JSON key.
	dir := t.TempDir()
	lines := []string{
		userTextEntry(ts(0), "first"),
		`{"type":"assis`,
	}
	path := writeJSONL(t, dir, "trunc_midkey.jsonl", lines...)

	sess, _, err := ParseTranscript(path)
	require.NoError(t, err)
	assert.Len(t, sess.Events, 1)
	assert.Equal(t, "first", sess.Events[0].Input)
}

func TestParseTranscript_AllBadLines_Good(t *testing.T) {
	// Every line is truncated/malformed — result should be empty, no error.
	dir := t.TempDir()
	lines := []string{
		`{"type":"user","timestamp`,
		`{"broken`,
		`not even json`,
	}
	path := writeJSONL(t, dir, "all_bad.jsonl", lines...)

	sess, stats, err := ParseTranscript(path)
	require.NoError(t, err)
	assert.Empty(t, sess.Events)
	assert.True(t, sess.StartTime.IsZero())
	assert.Equal(t, 3, stats.SkippedLines)
}

// --- ListSessions with truncated files ---

// --- PruneSessions tests ---

func TestPruneSessions_DeletesOldFiles_Good(t *testing.T) {
	dir := t.TempDir()

	// Create a session file with an old modification time.
	path := writeJSONL(t, dir, "old-session.jsonl",
		userTextEntry(ts(0), "old"),
	)
	// Backdate the file's mtime by 2 hours.
	oldTime := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(path, oldTime, oldTime))

	// Create a recent session file.
	writeJSONL(t, dir, "new-session.jsonl",
		userTextEntry(ts(0), "new"),
	)

	// Prune sessions older than 1 hour.
	deleted, err := PruneSessions(dir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	// Verify only the new file remains.
	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "new-session", sessions[0].ID)
}

func TestPruneSessions_NothingToDelete_Good(t *testing.T) {
	dir := t.TempDir()

	writeJSONL(t, dir, "recent.jsonl",
		userTextEntry(ts(0), "fresh"),
	)

	deleted, err := PruneSessions(dir, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestPruneSessions_EmptyDir_Good(t *testing.T) {
	dir := t.TempDir()

	deleted, err := PruneSessions(dir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

// --- IsExpired tests ---

func TestIsExpired_RecentSession_Good(t *testing.T) {
	sess := &Session{
		EndTime: time.Now().Add(-5 * time.Minute),
	}
	assert.False(t, sess.IsExpired(1*time.Hour))
}

func TestIsExpired_OldSession_Good(t *testing.T) {
	sess := &Session{
		EndTime: time.Now().Add(-2 * time.Hour),
	}
	assert.True(t, sess.IsExpired(1*time.Hour))
}

func TestIsExpired_ZeroEndTime_Bad(t *testing.T) {
	sess := &Session{}
	assert.False(t, sess.IsExpired(1*time.Hour))
}

// --- FetchSession tests ---

func TestFetchSession_ValidID_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "abc123.jsonl",
		userTextEntry(ts(0), "hello"),
	)

	sess, stats, err := FetchSession(dir, "abc123")
	require.NoError(t, err)
	require.NotNil(t, sess)
	require.NotNil(t, stats)
	assert.Equal(t, "abc123", sess.ID)
	assert.Len(t, sess.Events, 1)
}

func TestFetchSession_PathTraversal_Ugly(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, "../etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session id")
}

func TestFetchSession_BackslashTraversal_Ugly(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, `foo\bar`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session id")
}

func TestFetchSession_ForwardSlash_Ugly(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, "foo/bar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session id")
}

func TestFetchSession_NotFound_Bad(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open transcript")
}

// --- ListSessions with truncated files ---

func TestListSessions_TruncatedFile_Good(t *testing.T) {
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
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.False(t, sessions[0].StartTime.IsZero())
	assert.False(t, sessions[0].EndTime.IsZero())
	// End time should reflect the last valid timestamp.
	assert.True(t, sessions[0].EndTime.After(sessions[0].StartTime))
}

// --- PruneSessions tests ---

func TestPruneSessions_DeletesOld_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "old-session.jsonl", userTextEntry(ts(0), "old"))
	writeJSONL(t, dir, "new-session.jsonl", userTextEntry(ts(0), "new"))

	// Touch old-session to make it appear old (1 hour ago).
	oldPath := filepath.Join(dir, "old-session.jsonl")
	past := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(oldPath, past, past))

	deleted, err := PruneSessions(dir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	// Only new-session should remain.
	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "new-session", sessions[0].ID)
}

func TestPruneSessions_NoneExpired_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "fresh.jsonl", userTextEntry(ts(0), "fresh"))

	deleted, err := PruneSessions(dir, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
}

func TestPruneSessions_EmptyDir_Good(t *testing.T) {
	dir := t.TempDir()

	deleted, err := PruneSessions(dir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

// --- IsExpired tests ---

func TestIsExpired_Expired_Good(t *testing.T) {
	s := &Session{
		EndTime: time.Now().Add(-2 * time.Hour),
	}
	assert.True(t, s.IsExpired(1*time.Hour))
}

func TestIsExpired_NotExpired_Good(t *testing.T) {
	s := &Session{
		EndTime: time.Now().Add(-30 * time.Minute),
	}
	assert.False(t, s.IsExpired(1*time.Hour))
}

func TestIsExpired_ZeroEndTime_Bad(t *testing.T) {
	s := &Session{}
	assert.False(t, s.IsExpired(1*time.Hour))
}

// --- FetchSession tests ---

func TestFetchSession_ValidID_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "abc123.jsonl",
		userTextEntry(ts(0), "Hello"),
		assistantTextEntry(ts(1), "Hi"),
	)

	sess, stats, err := FetchSession(dir, "abc123")
	require.NoError(t, err)
	require.NotNil(t, sess)
	require.NotNil(t, stats)
	assert.Equal(t, "abc123", sess.ID)
	assert.Len(t, sess.Events, 2)
}

func TestFetchSession_PathTraversal_Bad(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, "../etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session id")
}

func TestFetchSession_BackslashTraversal_Bad(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, `..\\windows\\system32`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session id")
}

func TestFetchSession_SlashInID_Bad(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, "sub/dir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session id")
}

func TestFetchSession_NotFound_Bad(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FetchSession(dir, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open transcript")
}
