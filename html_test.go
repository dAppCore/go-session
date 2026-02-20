package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderHTML_BasicSession(t *testing.T) {
	sess := &Session{
		ID:        "test-session-abcdef12",
		Path:      "/tmp/test.jsonl",
		StartTime: baseTime,
		EndTime:   baseTime.Add(5 * time.Minute),
		Events: []Event{
			{
				Timestamp: baseTime,
				Type:      "user",
				Input:     "Please list files",
			},
			{
				Timestamp: baseTime.Add(time.Second),
				Type:      "tool_use",
				Tool:      "Bash",
				ToolID:    "tu_1",
				Input:     "ls -la # list files",
				Output:    "total 42\ndrwxr-xr-x  3 user user  4096 Feb 19 .",
				Duration:  2 * time.Second,
				Success:   true,
			},
			{
				Timestamp: baseTime.Add(3 * time.Second),
				Type:      "assistant",
				Input:     "The directory contains 42 items.",
			},
		},
	}

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "out.html")
	err := RenderHTML(sess, outputPath)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(data)

	t.Run("contains_doctype", func(t *testing.T) {
		assert.True(t, strings.HasPrefix(html, "<!DOCTYPE html>"))
	})

	t.Run("contains_session_id", func(t *testing.T) {
		assert.Contains(t, html, "test-ses")
	})

	t.Run("contains_timestamp", func(t *testing.T) {
		assert.Contains(t, html, "2026-02-19 10:00:00")
	})

	t.Run("contains_tool_count", func(t *testing.T) {
		assert.Contains(t, html, "1 tool calls")
	})

	t.Run("contains_user_event", func(t *testing.T) {
		assert.Contains(t, html, "User")
		assert.Contains(t, html, "Please list files")
	})

	t.Run("contains_bash_event", func(t *testing.T) {
		assert.Contains(t, html, "Bash")
		assert.Contains(t, html, "ls -la")
	})

	t.Run("contains_assistant_event", func(t *testing.T) {
		assert.Contains(t, html, "Claude")
	})

	t.Run("contains_js_functions", func(t *testing.T) {
		assert.Contains(t, html, "function toggle(")
		assert.Contains(t, html, "function filterEvents(")
	})

	t.Run("contains_success_icon", func(t *testing.T) {
		assert.Contains(t, html, "&#10003;") // Tick mark
	})

	t.Run("html_ends_properly", func(t *testing.T) {
		assert.True(t, strings.HasSuffix(strings.TrimSpace(html), "</html>"))
	})
}

func TestRenderHTML_EmptySession(t *testing.T) {
	sess := &Session{
		ID:        "empty-session",
		Path:      "/tmp/empty.jsonl",
		StartTime: baseTime,
		EndTime:   baseTime,
		Events:    nil,
	}

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "empty.html")
	err := RenderHTML(sess, outputPath)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(data)

	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "0 tool calls")
	// No error count span should appear in the stats.
	assert.NotContains(t, html, `class="err">`)
}

func TestRenderHTML_WithErrors(t *testing.T) {
	sess := &Session{
		ID:        "error-session",
		Path:      "/tmp/err.jsonl",
		StartTime: baseTime,
		EndTime:   baseTime.Add(10 * time.Second),
		Events: []Event{
			{
				Timestamp: baseTime,
				Type:      "tool_use",
				Tool:      "Bash",
				ToolID:    "tu_ok",
				Input:     "echo ok",
				Output:    "ok",
				Duration:  time.Second,
				Success:   true,
			},
			{
				Timestamp: baseTime.Add(2 * time.Second),
				Type:      "tool_use",
				Tool:      "Bash",
				ToolID:    "tu_err",
				Input:     "false",
				Output:    "exit 1",
				Duration:  time.Second,
				Success:   false,
				ErrorMsg:  "exit 1",
			},
		},
	}

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "errors.html")
	err := RenderHTML(sess, outputPath)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(data)

	assert.Contains(t, html, "2 tool calls")
	assert.Contains(t, html, "1 errors")
	assert.Contains(t, html, `class="err"`)
	assert.Contains(t, html, "&#10007;") // Cross mark for error
}

func TestRenderHTML_DurationFormatting(t *testing.T) {
	sess := &Session{
		ID:        "dur-test",
		Path:      "/tmp/dur.jsonl",
		StartTime: baseTime,
		EndTime:   baseTime.Add(2 * time.Hour),
		Events: []Event{
			{
				Timestamp: baseTime,
				Type:      "tool_use",
				Tool:      "Bash",
				ToolID:    "tu_ms",
				Input:     "fast cmd",
				Duration:  500 * time.Millisecond,
				Success:   true,
			},
			{
				Timestamp: baseTime.Add(time.Second),
				Type:      "tool_use",
				Tool:      "Bash",
				ToolID:    "tu_sec",
				Input:     "slow cmd",
				Duration:  45 * time.Second,
				Success:   true,
			},
			{
				Timestamp: baseTime.Add(time.Minute),
				Type:      "tool_use",
				Tool:      "Bash",
				ToolID:    "tu_min",
				Input:     "very slow cmd",
				Duration:  3*time.Minute + 30*time.Second,
				Success:   true,
			},
		},
	}

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "dur.html")
	err := RenderHTML(sess, outputPath)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(data)

	assert.Contains(t, html, "500ms")
	assert.Contains(t, html, "45.0s")
	assert.Contains(t, html, "3m30s")
	assert.Contains(t, html, "2h0m") // Header duration
}

func TestRenderHTML_HTMLEscaping(t *testing.T) {
	sess := &Session{
		ID:        "escape-test",
		Path:      "/tmp/esc.jsonl",
		StartTime: baseTime,
		EndTime:   baseTime.Add(time.Second),
		Events: []Event{
			{
				Timestamp: baseTime,
				Type:      "user",
				Input:     `<script>alert("xss")</script>`,
			},
			{
				Timestamp: baseTime,
				Type:      "tool_use",
				Tool:      "Bash",
				ToolID:    "tu_esc",
				Input:     `echo "<b>bold</b>"`,
				Output:    `<b>bold</b>`,
				Duration:  time.Second,
				Success:   true,
			},
		},
	}

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "esc.html")
	err := RenderHTML(sess, outputPath)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(data)

	// Raw angle brackets must be escaped.
	assert.NotContains(t, html, `<script>alert`)
	assert.Contains(t, html, "&lt;script&gt;")
}

func TestRenderHTML_InvalidPath(t *testing.T) {
	sess := &Session{ID: "x", StartTime: baseTime, EndTime: baseTime}
	err := RenderHTML(sess, "/nonexistent/dir/out.html")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create html")
}

func TestRenderHTML_AllEventTypes(t *testing.T) {
	// Verify that the label logic covers all event types.
	sess := &Session{
		ID:        "labels",
		Path:      "/tmp/labels.jsonl",
		StartTime: baseTime,
		EndTime:   baseTime.Add(10 * time.Second),
		Events: []Event{
			{Timestamp: baseTime, Type: "user", Input: "user msg"},
			{Timestamp: baseTime, Type: "assistant", Input: "assistant msg"},
			{Timestamp: baseTime, Type: "tool_use", Tool: "Bash", Input: "cmd", Success: true, Duration: time.Second},
			{Timestamp: baseTime, Type: "tool_use", Tool: "Read", Input: "/path", Success: true, Duration: time.Second},
			{Timestamp: baseTime, Type: "tool_use", Tool: "Edit", Input: "/path (edit)", Success: true, Duration: time.Second},
			{Timestamp: baseTime, Type: "tool_use", Tool: "Write", Input: "/path (10 bytes)", Success: true, Duration: time.Second},
			{Timestamp: baseTime, Type: "tool_use", Tool: "Grep", Input: "/TODO/ in .", Success: true, Duration: time.Second},
			{Timestamp: baseTime, Type: "tool_use", Tool: "Glob", Input: "**/*.go", Success: true, Duration: time.Second},
		},
	}

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "labels.html")
	err := RenderHTML(sess, outputPath)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(data)

	// Check label assignments.
	assert.Contains(t, html, "Message") // User event label
	assert.Contains(t, html, "Response") // Assistant event label
	assert.Contains(t, html, "Command")  // Bash event label
	assert.Contains(t, html, "Target")   // Read/Grep/Glob event label
	assert.Contains(t, html, "File")     // Edit/Write event label
}

// -- shortID tests --

func TestShortID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"long id", "abcdef1234567890", "abcdef12"},
		{"short id", "abc", "abc"},
		{"exactly 8", "abcdefgh", "abcdefgh"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shortID(tt.id))
		})
	}
}

// -- formatDuration tests --

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"milliseconds", 500 * time.Millisecond, "500ms"},
		{"seconds", 45 * time.Second, "45.0s"},
		{"minutes", 3*time.Minute + 30*time.Second, "3m30s"},
		{"hours", 2*time.Hour + 15*time.Minute, "2h15m"},
		{"zero", 0, "0ms"},
		{"sub-millisecond", 500 * time.Microsecond, "0ms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatDuration(tt.d))
		})
	}
}
