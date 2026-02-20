package session

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: RenderMP4 requires the external `vhs` binary and writes to temp files
// then calls exec. We test generateTape (the pure logic) directly, and verify
// RenderMP4 returns a sensible error when vhs is absent.

func TestRenderMP4_VHSNotInstalled(t *testing.T) {
	sess := &Session{
		ID:        "video-test",
		StartTime: baseTime,
		EndTime:   baseTime.Add(time.Minute),
	}

	err := RenderMP4(sess, "/tmp/out.mp4")
	if err == nil {
		t.Skip("vhs is installed on this system; skipping missing-binary test")
	}
	assert.Contains(t, err.Error(), "vhs not installed")
}

func TestGenerateTape_EmptySession(t *testing.T) {
	sess := &Session{
		ID:        "empty-tape",
		StartTime: baseTime,
		EndTime:   baseTime,
		Events:    nil,
	}

	tape := generateTape(sess, "/tmp/empty.mp4")

	assert.Contains(t, tape, "Output /tmp/empty.mp4")
	assert.Contains(t, tape, "Set FontSize 16")
	assert.Contains(t, tape, "Set Width 1400")
	assert.Contains(t, tape, "Set Height 800")
	assert.Contains(t, tape, "Set Theme")
	assert.Contains(t, tape, "# Session empty-ta")
	assert.Contains(t, tape, "Sleep 3s") // Final sleep
}

func TestGenerateTape_BashEvents(t *testing.T) {
	sess := &Session{
		ID:        "bash-tape-session-long-id",
		StartTime: baseTime,
		EndTime:   baseTime.Add(10 * time.Second),
		Events: []Event{
			{
				Type:    "tool_use",
				Tool:    "Bash",
				Input:   "ls -la # list files",
				Output:  "total 10\nfile1.go\nfile2.go",
				Success: true,
			},
			{
				Type:     "tool_use",
				Tool:     "Bash",
				Input:    "false",
				Output:   "exit 1",
				Success:  false,
				ErrorMsg: "exit 1",
			},
		},
	}

	tape := generateTape(sess, "/tmp/bash.mp4")

	t.Run("title_uses_short_id", func(t *testing.T) {
		assert.Contains(t, tape, "# Session bash-tap")
	})

	t.Run("bash_command_shown", func(t *testing.T) {
		assert.Contains(t, tape, `"$ ls -la"`)
	})

	t.Run("bash_output_shown", func(t *testing.T) {
		assert.Contains(t, tape, "file1.go")
	})

	t.Run("success_indicator", func(t *testing.T) {
		assert.Contains(t, tape, "OK")
	})

	t.Run("failure_indicator", func(t *testing.T) {
		assert.Contains(t, tape, "FAILED")
	})
}

func TestGenerateTape_ReadEditWriteEvents(t *testing.T) {
	sess := &Session{
		ID:        "file-ops",
		StartTime: baseTime,
		EndTime:   baseTime.Add(5 * time.Second),
		Events: []Event{
			{Type: "tool_use", Tool: "Read", Input: "/tmp/foo.go", Success: true},
			{Type: "tool_use", Tool: "Edit", Input: "/tmp/foo.go (edit)", Success: true},
			{Type: "tool_use", Tool: "Write", Input: "/tmp/bar.go (100 bytes)", Success: true},
		},
	}

	tape := generateTape(sess, "/tmp/files.mp4")

	assert.Contains(t, tape, "# Read: /tmp/foo.go")
	assert.Contains(t, tape, "# Edit: /tmp/foo.go (edit)")
	assert.Contains(t, tape, "# Write: /tmp/bar.go (100 bytes)")
}

func TestGenerateTape_TaskEvents(t *testing.T) {
	sess := &Session{
		ID:        "task-session",
		StartTime: baseTime,
		EndTime:   baseTime.Add(5 * time.Second),
		Events: []Event{
			{Type: "tool_use", Tool: "Task", Input: "[research] summarise the codebase", Success: true},
		},
	}

	tape := generateTape(sess, "/tmp/task.mp4")
	assert.Contains(t, tape, "# Agent: [research] summarise the codebase")
}

func TestGenerateTape_SkipsNonToolEvents(t *testing.T) {
	sess := &Session{
		ID:        "skip-test",
		StartTime: baseTime,
		EndTime:   baseTime.Add(5 * time.Second),
		Events: []Event{
			{Type: "user", Input: "user message"},
			{Type: "assistant", Input: "assistant message"},
			{Type: "tool_use", Tool: "Bash", Input: "echo ok", Output: "ok", Success: true},
		},
	}

	tape := generateTape(sess, "/tmp/skip.mp4")

	// User and assistant messages should not appear as typed commands.
	assert.NotContains(t, tape, "user message")
	assert.NotContains(t, tape, "assistant message")
	assert.Contains(t, tape, "$ echo ok")
}

func TestGenerateTape_LongOutputTruncated(t *testing.T) {
	longOutput := strings.Repeat("x", 500)
	sess := &Session{
		ID:        "trunc-out",
		StartTime: baseTime,
		EndTime:   baseTime.Add(5 * time.Second),
		Events: []Event{
			{Type: "tool_use", Tool: "Bash", Input: "cmd", Output: longOutput, Success: true},
		},
	}

	tape := generateTape(sess, "/tmp/trunc.mp4")
	// Output in the tape should be truncated at 200 chars + "...".
	assert.Contains(t, tape, "...")
	// The full 500-char string should not appear.
	assert.NotContains(t, tape, longOutput)
}

func TestGenerateTape_EmptyBashCommand(t *testing.T) {
	sess := &Session{
		ID:        "empty-cmd",
		StartTime: baseTime,
		EndTime:   baseTime.Add(time.Second),
		Events: []Event{
			{Type: "tool_use", Tool: "Bash", Input: "", Success: true},
		},
	}

	tape := generateTape(sess, "/tmp/empty-cmd.mp4")
	// An empty command should be skipped (no "$ " line).
	lines := strings.Split(tape, "\n")
	for _, line := range lines {
		assert.NotContains(t, line, `"$ "`)
	}
}

func TestGenerateTape_SkipsGrepGlob(t *testing.T) {
	// Grep and Glob tool_use events are not handled in the switch,
	// so they should produce no typed output in the tape.
	sess := &Session{
		ID:        "grep-glob",
		StartTime: baseTime,
		EndTime:   baseTime.Add(5 * time.Second),
		Events: []Event{
			{Type: "tool_use", Tool: "Grep", Input: "/TODO/ in .", Success: true},
			{Type: "tool_use", Tool: "Glob", Input: "**/*.go", Success: true},
		},
	}

	tape := generateTape(sess, "/tmp/gg.mp4")
	// Title and settings should exist, but no Grep/Glob content.
	assert.Contains(t, tape, "Output /tmp/gg.mp4")
	assert.NotContains(t, tape, "TODO")
	assert.NotContains(t, tape, "*.go")
}

// -- extractCommand tests --

func TestExtractCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"with description", "ls -la # list files", "ls -la"},
		{"without description", "pwd", "pwd"},
		// extractCommand naively splits on first " # " so embedded hashes are truncated.
		{"hash in command", "echo 'hello # world'", "echo 'hello"},
		{"description at start", " # desc", " # desc"}, // idx == 0, not > 0
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCommand(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}
