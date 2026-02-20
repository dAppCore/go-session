package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -- helpers --

// writeTempJSONL writes lines to a temp .jsonl file and returns the path.
func writeTempJSONL(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := strings.Join(lines, "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// ts builds an RFC3339Nano timestamp string offset from a base.
func ts(base time.Time, offset time.Duration) string {
	return base.Add(offset).Format(time.RFC3339Nano)
}

// jsonLine builds a raw JSONL line from an entry struct.
func jsonLine(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

// -- fixtures --

var baseTime = time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC)

// fixtureMinimalSession returns JSONL lines for a user message followed by
// an assistant text reply.
func fixtureMinimalSession() []string {
	return []string{
		`{"type":"user","timestamp":"` + ts(baseTime, 0) + `","sessionId":"abc123","message":{"role":"user","content":[{"type":"text","text":"hello"}]}}`,
		`{"type":"assistant","timestamp":"` + ts(baseTime, 2*time.Second) + `","sessionId":"abc123","message":{"role":"assistant","content":[{"type":"text","text":"Hi there."}]}}`,
	}
}

// fixtureToolCallSession returns a session where the assistant invokes a Bash
// tool and the user entry carries the tool_result.
func fixtureToolCallSession() []string {
	return []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"tool1","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Bash","input":{"command":"ls -la","description":"list files"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, 3*time.Second) + `","sessionId":"tool1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"total 42\ndrwxr-xr-x 3 user user 4096 Feb 19 10:00 ."}]}}`,
	}
}

// fixtureAllToolTypes returns lines exercising every supported tool type.
func fixtureAllToolTypes() []string {
	tools := []struct {
		id, name string
		input    string
	}{
		{"tu_bash", "Bash", `{"command":"echo ok","description":"test echo"}`},
		{"tu_read", "Read", `{"file_path":"/tmp/foo.go","offset":0,"limit":100}`},
		{"tu_edit", "Edit", `{"file_path":"/tmp/foo.go","old_string":"old","new_string":"new"}`},
		{"tu_write", "Write", `{"file_path":"/tmp/bar.go","content":"package bar"}`},
		{"tu_grep", "Grep", `{"pattern":"TODO","path":"/src"}`},
		{"tu_glob", "Glob", `{"pattern":"**/*.go","path":"/src"}`},
		{"tu_task", "Task", `{"prompt":"summarise","description":"summarise code","subagent_type":"research"}`},
	}

	var lines []string
	for i, tool := range tools {
		off := time.Duration(i*2) * time.Second
		// Assistant makes the tool_use call.
		lines = append(lines, `{"type":"assistant","timestamp":"`+ts(baseTime, off)+`","sessionId":"all_tools","message":{"role":"assistant","content":[{"type":"tool_use","id":"`+tool.id+`","name":"`+tool.name+`","input":`+tool.input+`}]}}`)
		// User entry carries the tool_result.
		lines = append(lines, `{"type":"user","timestamp":"`+ts(baseTime, off+time.Second)+`","sessionId":"all_tools","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"`+tool.id+`","content":"result ok"}]}}`)
	}
	return lines
}

// fixtureErrorToolResult returns a session with a failing tool call.
func fixtureErrorToolResult() []string {
	return []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"err1","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_err","name":"Bash","input":{"command":"bad-cmd"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"err1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_err","is_error":true,"content":"command not found: bad-cmd"}]}}`,
	}
}

// -- ParseTranscript tests --

func TestParseTranscript_MinimalValid(t *testing.T) {
	dir := t.TempDir()
	path := writeTempJSONL(t, dir, "minimal.jsonl", fixtureMinimalSession()...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err)

	assert.Equal(t, "minimal", sess.ID)
	assert.Equal(t, path, sess.Path)
	assert.False(t, sess.StartTime.IsZero(), "StartTime should be set")
	assert.False(t, sess.EndTime.IsZero(), "EndTime should be set")
	assert.True(t, sess.EndTime.After(sess.StartTime), "EndTime should be after StartTime")

	require.Len(t, sess.Events, 2)

	// First event: user message
	assert.Equal(t, "user", sess.Events[0].Type)
	assert.Equal(t, "hello", sess.Events[0].Input)

	// Second event: assistant message
	assert.Equal(t, "assistant", sess.Events[1].Type)
	assert.Equal(t, "Hi there.", sess.Events[1].Input)
}

func TestParseTranscript_ToolCallBash(t *testing.T) {
	dir := t.TempDir()
	path := writeTempJSONL(t, dir, "toolcall.jsonl", fixtureToolCallSession()...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err)

	// Should have one tool_use event (the assistant text block is empty,
	// so only the completed tool call appears).
	require.Len(t, sess.Events, 1)
	evt := sess.Events[0]

	assert.Equal(t, "tool_use", evt.Type)
	assert.Equal(t, "Bash", evt.Tool)
	assert.Equal(t, "tu_1", evt.ToolID)
	assert.Contains(t, evt.Input, "ls -la")
	assert.Contains(t, evt.Input, "# list files")
	assert.Contains(t, evt.Output, "total 42")
	assert.True(t, evt.Success)
	assert.Equal(t, 3*time.Second, evt.Duration)
}

func TestParseTranscript_AllToolTypes(t *testing.T) {
	dir := t.TempDir()
	path := writeTempJSONL(t, dir, "alltools.jsonl", fixtureAllToolTypes()...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err)

	expectedTools := []string{"Bash", "Read", "Edit", "Write", "Grep", "Glob", "Task"}
	var gotTools []string
	for _, evt := range sess.Events {
		if evt.Type == "tool_use" {
			gotTools = append(gotTools, evt.Tool)
		}
	}
	assert.Equal(t, expectedTools, gotTools)
}

func TestParseTranscript_ErrorToolResult(t *testing.T) {
	dir := t.TempDir()
	path := writeTempJSONL(t, dir, "error.jsonl", fixtureErrorToolResult()...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err)

	require.Len(t, sess.Events, 1)
	evt := sess.Events[0]
	assert.False(t, evt.Success)
	assert.Contains(t, evt.ErrorMsg, "command not found")
}

func TestParseTranscript_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeTempJSONL(t, dir, "empty.jsonl")

	sess, err := ParseTranscript(path)
	require.NoError(t, err)
	assert.Empty(t, sess.Events)
	assert.True(t, sess.StartTime.IsZero())
}

func TestParseTranscript_MalformedLines(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		`{not valid json`,
		`{"type":"user","timestamp":"` + ts(baseTime, 0) + `","sessionId":"x","message":{"role":"user","content":[{"type":"text","text":"ok"}]}}`,
		`{also bad`,
		`{"type":"assistant","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"x","message":{"role":"assistant","content":[{"type":"text","text":"reply"}]}}`,
	}
	path := writeTempJSONL(t, dir, "malformed.jsonl", lines...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err, "malformed lines should be skipped, not cause an error")
	assert.Len(t, sess.Events, 2, "only the two valid lines should produce events")
}

func TestParseTranscript_TruncatedInput(t *testing.T) {
	dir := t.TempDir()
	// Final line is incomplete JSON (no closing brace).
	lines := []string{
		`{"type":"user","timestamp":"` + ts(baseTime, 0) + `","sessionId":"trunc","message":{"role":"user","content":[{"type":"text","text":"start"}]}}`,
		`{"type":"assistant","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"trunc","message":{"role":"assistant","content":[{"type":"text","text":"partial`,
	}
	path := writeTempJSONL(t, dir, "truncated.jsonl", lines...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err)
	// Only the first valid line should produce an event.
	assert.Len(t, sess.Events, 1)
	assert.Equal(t, "start", sess.Events[0].Input)
}

func TestParseTranscript_FileNotFound(t *testing.T) {
	_, err := ParseTranscript("/nonexistent/path.jsonl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open transcript")
}

func TestParseTranscript_LargeSession(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	// Generate 500 user+assistant pairs = 1000 lines.
	for i := 0; i < 500; i++ {
		off := time.Duration(i*2) * time.Second
		lines = append(lines,
			`{"type":"user","timestamp":"`+ts(baseTime, off)+`","sessionId":"large","message":{"role":"user","content":[{"type":"text","text":"msg `+strings.Repeat("x", 50)+`"}]}}`,
			`{"type":"assistant","timestamp":"`+ts(baseTime, off+time.Second)+`","sessionId":"large","message":{"role":"assistant","content":[{"type":"text","text":"reply `+strings.Repeat("y", 50)+`"}]}}`,
		)
	}
	path := writeTempJSONL(t, dir, "large.jsonl", lines...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err)
	assert.Len(t, sess.Events, 1000)
}

func TestParseTranscript_ToolResultArrayContent(t *testing.T) {
	// tool_result whose content is an array of text blocks.
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"arr","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_arr","name":"Bash","input":{"command":"echo hi"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"arr","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_arr","content":[{"type":"text","text":"line1"},{"type":"text","text":"line2"}]}]}}`,
	}
	path := writeTempJSONL(t, dir, "array_content.jsonl", lines...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err)
	require.Len(t, sess.Events, 1)
	assert.Contains(t, sess.Events[0].Output, "line1")
	assert.Contains(t, sess.Events[0].Output, "line2")
}

func TestParseTranscript_ToolResultMapContent(t *testing.T) {
	// tool_result whose content is a map with a text key.
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"map","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_map","name":"Read","input":{"file_path":"/tmp/x"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"map","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_map","content":{"text":"file contents here"}}]}}`,
	}
	path := writeTempJSONL(t, dir, "map_content.jsonl", lines...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err)
	require.Len(t, sess.Events, 1)
	assert.Equal(t, "file contents here", sess.Events[0].Output)
}

func TestParseTranscript_UnmatchedToolResult(t *testing.T) {
	// A tool_result whose tool_use_id doesn't match any pending tool_use.
	dir := t.TempDir()
	lines := []string{
		`{"type":"user","timestamp":"` + ts(baseTime, 0) + `","sessionId":"unmatched","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"nonexistent","content":"orphan result"}]}}`,
	}
	path := writeTempJSONL(t, dir, "unmatched.jsonl", lines...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err)
	// Unmatched tool_result should be silently ignored.
	assert.Empty(t, sess.Events)
}

func TestParseTranscript_MixedContentBlocks(t *testing.T) {
	// An assistant message with both text and tool_use blocks.
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"mixed","message":{"role":"assistant","content":[{"type":"text","text":"I will read the file."},{"type":"tool_use","id":"tu_mix","name":"Read","input":{"file_path":"/etc/hosts"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"mixed","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_mix","content":"127.0.0.1 localhost"}]}}`,
	}
	path := writeTempJSONL(t, dir, "mixed.jsonl", lines...)

	sess, err := ParseTranscript(path)
	require.NoError(t, err)
	// Should get: 1 assistant text + 1 tool_use
	require.Len(t, sess.Events, 2)
	assert.Equal(t, "assistant", sess.Events[0].Type)
	assert.Equal(t, "I will read the file.", sess.Events[0].Input)
	assert.Equal(t, "tool_use", sess.Events[1].Type)
	assert.Equal(t, "Read", sess.Events[1].Tool)
}

// -- extractToolInput tests --

func TestExtractToolInput(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    string
		want     string
	}{
		{
			name:     "Bash with description",
			toolName: "Bash",
			input:    `{"command":"ls -la","description":"list files"}`,
			want:     "ls -la # list files",
		},
		{
			name:     "Bash without description",
			toolName: "Bash",
			input:    `{"command":"pwd"}`,
			want:     "pwd",
		},
		{
			name:     "Read",
			toolName: "Read",
			input:    `{"file_path":"/home/user/main.go","offset":10,"limit":50}`,
			want:     "/home/user/main.go",
		},
		{
			name:     "Edit",
			toolName: "Edit",
			input:    `{"file_path":"/tmp/test.go","old_string":"foo","new_string":"bar"}`,
			want:     "/tmp/test.go (edit)",
		},
		{
			name:     "Write",
			toolName: "Write",
			input:    `{"file_path":"/tmp/out.txt","content":"hello world"}`,
			want:     "/tmp/out.txt (11 bytes)",
		},
		{
			name:     "Grep with path",
			toolName: "Grep",
			input:    `{"pattern":"TODO","path":"/src"}`,
			want:     "/TODO/ in /src",
		},
		{
			name:     "Grep without path",
			toolName: "Grep",
			input:    `{"pattern":"FIXME"}`,
			want:     "/FIXME/ in .",
		},
		{
			name:     "Glob",
			toolName: "Glob",
			input:    `{"pattern":"**/*.go","path":"/src"}`,
			want:     "**/*.go",
		},
		{
			name:     "Task with description",
			toolName: "Task",
			input:    `{"prompt":"do something long","description":"short desc","subagent_type":"research"}`,
			want:     "[research] short desc",
		},
		{
			name:     "Task without description falls back to prompt",
			toolName: "Task",
			input:    `{"prompt":"analyse this code","subagent_type":"codegen"}`,
			want:     "[codegen] analyse this code",
		},
		{
			name:     "Unknown tool shows sorted keys",
			toolName: "CustomMCP",
			input:    `{"beta":"b","alpha":"a"}`,
			want:     "alpha, beta",
		},
		{
			name:     "Nil input",
			toolName: "Bash",
			input:    "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw json.RawMessage
			if tt.input != "" {
				raw = json.RawMessage(tt.input)
			}
			got := extractToolInput(tt.toolName, raw)
			assert.Equal(t, tt.want, got)
		})
	}
}

// -- extractResultContent tests --

func TestExtractResultContent(t *testing.T) {
	tests := []struct {
		name    string
		content interface{}
		want    string
	}{
		{
			name:    "string content",
			content: "simple output",
			want:    "simple output",
		},
		{
			name: "array of text blocks",
			content: []interface{}{
				map[string]interface{}{"type": "text", "text": "line1"},
				map[string]interface{}{"type": "text", "text": "line2"},
			},
			want: "line1\nline2",
		},
		{
			name:    "map with text key",
			content: map[string]interface{}{"text": "map value"},
			want:    "map value",
		},
		{
			name:    "map without text key",
			content: map[string]interface{}{"data": 42},
			want:    "map[data:42]",
		},
		{
			name:    "nil content",
			content: nil,
			want:    "<nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractResultContent(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

// -- truncate tests --

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hello..."},
		{"empty string", "", 10, ""},
		{"zero max", "hello", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, truncate(tt.s, tt.max))
		})
	}
}

// -- ListSessions tests --

func TestListSessions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestListSessions_SingleSession(t *testing.T) {
	dir := t.TempDir()
	writeTempJSONL(t, dir, "sess-001.jsonl", fixtureMinimalSession()...)

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "sess-001", sessions[0].ID)
	assert.False(t, sessions[0].StartTime.IsZero())
	assert.False(t, sessions[0].EndTime.IsZero())
}

func TestListSessions_MultipleSorted(t *testing.T) {
	dir := t.TempDir()

	// Create sessions with different timestamps.
	early := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	late := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)

	earlyLines := []string{
		`{"type":"user","timestamp":"` + early.Format(time.RFC3339Nano) + `","sessionId":"early","message":{"role":"user","content":[{"type":"text","text":"old"}]}}`,
	}
	lateLines := []string{
		`{"type":"user","timestamp":"` + late.Format(time.RFC3339Nano) + `","sessionId":"late","message":{"role":"user","content":[{"type":"text","text":"new"}]}}`,
	}

	writeTempJSONL(t, dir, "early.jsonl", earlyLines...)
	writeTempJSONL(t, dir, "late.jsonl", lateLines...)

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	// Should be sorted newest first.
	assert.Equal(t, "late", sessions[0].ID)
	assert.Equal(t, "early", sessions[1].ID)
}

func TestListSessions_IgnoresNonJSONL(t *testing.T) {
	dir := t.TempDir()
	writeTempJSONL(t, dir, "session.jsonl", fixtureMinimalSession()...)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a session"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0644))

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
}

func TestListSessions_MalformedContent(t *testing.T) {
	dir := t.TempDir()
	// A .jsonl file with no valid timestamps at all — should still appear
	// with a fallback start time from file mod time.
	writeTempJSONL(t, dir, "bad.jsonl", `{not json}`, `{also bad}`)

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	// StartTime should fall back to the file's mod time.
	assert.False(t, sessions[0].StartTime.IsZero())
}

// -- Benchmark --

func BenchmarkParseTranscript(b *testing.B) {
	dir := b.TempDir()
	var lines []string
	for i := 0; i < 2000; i++ {
		off := time.Duration(i*2) * time.Second
		lines = append(lines,
			`{"type":"assistant","timestamp":"`+ts(baseTime, off)+`","sessionId":"bench","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_`+strings.Repeat("a", 5)+`_`+string(rune('0'+i%10))+`","name":"Bash","input":{"command":"echo hello"}}]}}`,
			`{"type":"user","timestamp":"`+ts(baseTime, off+time.Second)+`","sessionId":"bench","message":{"role":"user","content":[{"type":"text","text":"msg"}]}}`,
		)
	}
	path := writeTempJSONL(&testing.T{}, dir, "bench.jsonl", lines...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseTranscript(path)
	}
}
