// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
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
func jsonlLine(m map[string]interface{}) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// userTextEntry creates a JSONL line for a user text message.
func userTextEntry(timestamp string, text string) string {
	return jsonlLine(map[string]interface{}{
		"type":      "user",
		"timestamp": timestamp,
		"sessionId": "test-session",
		"message": map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": text},
			},
		},
	})
}

// assistantTextEntry creates a JSONL line for an assistant text message.
func assistantTextEntry(timestamp string, text string) string {
	return jsonlLine(map[string]interface{}{
		"type":      "assistant",
		"timestamp": timestamp,
		"sessionId": "test-session",
		"message": map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": text},
			},
		},
	})
}

// toolUseEntry creates a JSONL line for an assistant message containing a tool_use block.
func toolUseEntry(timestamp, toolName, toolID string, input map[string]interface{}) string {
	return jsonlLine(map[string]interface{}{
		"type":      "assistant",
		"timestamp": timestamp,
		"sessionId": "test-session",
		"message": map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
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
func toolResultEntry(timestamp, toolUseID string, content interface{}, isError bool) string {
	entry := map[string]interface{}{
		"type":      "user",
		"timestamp": timestamp,
		"sessionId": "test-session",
		"message": map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
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
		toolUseEntry(ts(1), "Bash", "tool-bash-1", map[string]interface{}{
			"command":     "ls -la",
			"description": "list files",
		}),
		toolResultEntry(ts(2), "tool-bash-1", "total 42\ndrwxr-xr-x 5 user staff 160 Feb 20 10:00 .", false),
		// Read tool_use
		toolUseEntry(ts(3), "Read", "tool-read-1", map[string]interface{}{
			"file_path": "/tmp/test.go",
		}),
		toolResultEntry(ts(4), "tool-read-1", "package main\n\nfunc main() {}", false),
		// Edit tool_use
		toolUseEntry(ts(5), "Edit", "tool-edit-1", map[string]interface{}{
			"file_path":  "/tmp/test.go",
			"old_string": "main",
			"new_string": "app",
		}),
		toolResultEntry(ts(6), "tool-edit-1", "ok", false),
		// Write tool_use
		toolUseEntry(ts(7), "Write", "tool-write-1", map[string]interface{}{
			"file_path": "/tmp/new.go",
			"content":   "package new\n",
		}),
		toolResultEntry(ts(8), "tool-write-1", "ok", false),
		// Grep tool_use
		toolUseEntry(ts(9), "Grep", "tool-grep-1", map[string]interface{}{
			"pattern": "TODO",
			"path":    "/tmp",
		}),
		toolResultEntry(ts(10), "tool-grep-1", "/tmp/test.go:3:// TODO fix this", false),
		// Glob tool_use
		toolUseEntry(ts(11), "Glob", "tool-glob-1", map[string]interface{}{
			"pattern": "**/*.go",
		}),
		toolResultEntry(ts(12), "tool-glob-1", "/tmp/test.go\n/tmp/new.go", false),
		// Task tool_use
		toolUseEntry(ts(13), "Task", "tool-task-1", map[string]interface{}{
			"prompt":       "Analyse the code",
			"description":  "Code analysis",
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
		toolUseEntry(ts(0), "Bash", "tool-err-1", map[string]interface{}{
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
	for i := 0; i < 1100; i++ {
		toolID := fmt.Sprintf("tool-%d", i)
		offset := (i * 2) + 1
		lines = append(lines,
			toolUseEntry(ts(offset), "Bash", toolID, map[string]interface{}{
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
	arrayContent := []map[string]interface{}{
		{"type": "text", "text": "First block"},
		{"type": "text", "text": "Second block"},
	}

	lines := []string{
		toolUseEntry(ts(0), "Bash", "tool-nested-1", map[string]interface{}{
			"command": "complex output",
		}),
		// Build the tool result with array content manually
		jsonlLine(map[string]interface{}{
			"type":      "user",
			"timestamp": ts(1),
			"sessionId": "test-session",
			"message": map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
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
		toolUseEntry(ts(0), "Read", "tool-map-1", map[string]interface{}{
			"file_path": "/tmp/data.json",
		}),
		// Build a tool result with map content
		jsonlLine(map[string]interface{}{
			"type":      "user",
			"timestamp": ts(1),
			"sessionId": "test-session",
			"message": map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": "tool-map-1",
						"content": map[string]interface{}{
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

func TestParseTranscript_MixedContentBlocks_Good(t *testing.T) {
	// Assistant message with both text and tool_use in the same message
	dir := t.TempDir()

	lines := []string{
		// An assistant message with text + tool_use in the same content array
		jsonlLine(map[string]interface{}{
			"type":      "assistant",
			"timestamp": ts(0),
			"sessionId": "test-session",
			"message": map[string]interface{}{
				"role": "assistant",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Let me check that file."},
					{
						"type":  "tool_use",
						"name":  "Read",
						"id":    "tool-mixed-1",
						"input": map[string]interface{}{"file_path": "/tmp/mix.go"},
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
	line := jsonlLine(map[string]interface{}{
		"type":      "user",
		"timestamp": "",
		"sessionId": "test-session",
		"message": map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
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
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "line one"},
		map[string]interface{}{"type": "text", "text": "line two"},
	}
	result := extractResultContent(content)
	assert.Equal(t, "line one\nline two", result)
}

func TestExtractResultContent_Map_Good(t *testing.T) {
	content := map[string]interface{}{"text": "from map"}
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
		toolUseEntry(ts(1), "Bash", "tool-1", map[string]interface{}{
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
		toolUseEntry(ts(0), "Bash", "orphan-1", map[string]interface{}{
			"command": "ls",
		}),
		toolUseEntry(ts(1), "Read", "orphan-2", map[string]interface{}{
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
