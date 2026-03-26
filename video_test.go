// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"
	"time"

	core "dappco.re/go/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTape_BasicSession_Good(t *testing.T) {
	sess := &Session{
		ID:        "tape-test-12345678",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events: []Event{
			{
				Type:    "tool_use",
				Tool:    "Bash",
				Input:   "go test ./...",
				Output:  "PASS",
				Success: true,
			},
			{
				Type:    "tool_use",
				Tool:    "Read",
				Input:   "/tmp/file.go",
				Output:  "package main",
				Success: true,
			},
		},
	}

	tape := generateTape(sess, "/tmp/output.mp4")

	assert.Contains(t, tape, "Output /tmp/output.mp4")
	assert.Contains(t, tape, "Set FontSize 16")
	assert.Contains(t, tape, "tape-tes") // shortID
	assert.Contains(t, tape, "2026-02-20 10:00")
	assert.Contains(t, tape, `"$ go test ./..."`)
	assert.Contains(t, tape, "PASS")
	assert.Contains(t, tape, `"# ✓ OK"`)
	assert.Contains(t, tape, "# Read: /tmp/file.go")
}

func TestGenerateTape_SkipsNonToolEvents_Good(t *testing.T) {
	sess := &Session{
		ID:        "skip-test",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events: []Event{
			{Type: "user", Input: "Hello"},
			{Type: "assistant", Input: "Hi there"},
			{Type: "tool_use", Tool: "Bash", Input: "echo hi", Output: "hi", Success: true},
		},
	}

	tape := generateTape(sess, "/tmp/out.mp4")

	// User and assistant events should NOT appear in the tape
	assert.NotContains(t, tape, "Hello")
	assert.NotContains(t, tape, "Hi there")
	// Bash command should appear
	assert.Contains(t, tape, "echo hi")
}

func TestGenerateTape_FailedCommand_Good(t *testing.T) {
	sess := &Session{
		ID:        "fail-test",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events: []Event{
			{
				Type:    "tool_use",
				Tool:    "Bash",
				Input:   "cat /missing",
				Output:  "No such file",
				Success: false,
			},
		},
	}

	tape := generateTape(sess, "/tmp/out.mp4")
	assert.Contains(t, tape, `"# ✗ FAILED"`)
}

func TestGenerateTape_LongOutput_Good(t *testing.T) {
	sess := &Session{
		ID:        "long-test",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events: []Event{
			{
				Type:    "tool_use",
				Tool:    "Bash",
				Input:   "cat huge.log",
				Output:  repeatString("x", 300),
				Success: true,
			},
		},
	}

	tape := generateTape(sess, "/tmp/out.mp4")
	// Output should be truncated to 200 chars + "..."
	assert.Contains(t, tape, "...")
}

func TestGenerateTape_TaskEvent_Good(t *testing.T) {
	sess := &Session{
		ID:        "task-test",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events: []Event{
			{
				Type:  "tool_use",
				Tool:  "Task",
				Input: "[research] Analyse code structure",
			},
		},
	}

	tape := generateTape(sess, "/tmp/out.mp4")
	assert.Contains(t, tape, "# Agent: [research] Analyse code structure")
}

func TestGenerateTape_EditWriteEvents_Good(t *testing.T) {
	sess := &Session{
		ID:        "edit-test",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events: []Event{
			{Type: "tool_use", Tool: "Edit", Input: "/tmp/app.go (edit)"},
			{Type: "tool_use", Tool: "Write", Input: "/tmp/new.go (50 bytes)"},
		},
	}

	tape := generateTape(sess, "/tmp/out.mp4")
	assert.Contains(t, tape, "# Edit: /tmp/app.go (edit)")
	assert.Contains(t, tape, "# Write: /tmp/new.go (50 bytes)")
}

func TestGenerateTape_EmptySession_Good(t *testing.T) {
	sess := &Session{
		ID:        "empty-test",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events:    nil,
	}

	tape := generateTape(sess, "/tmp/out.mp4")

	// Should still have the header and trailer
	assert.Contains(t, tape, "Output /tmp/out.mp4")
	assert.Contains(t, tape, "Sleep 3s")
	// No tool events
	lines := core.Split(tape, "\n")
	var toolLines int
	for _, line := range lines {
		if core.Contains(line, "$ ") || core.Contains(line, "# Read:") ||
			core.Contains(line, "# Edit:") || core.Contains(line, "# Write:") {
			toolLines++
		}
	}
	assert.Equal(t, 0, toolLines)
}

func TestGenerateTape_BashEmptyCommand_Bad(t *testing.T) {
	sess := &Session{
		ID:        "empty-cmd",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events: []Event{
			{Type: "tool_use", Tool: "Bash", Input: "", Output: "", Success: true},
		},
	}

	tape := generateTape(sess, "/tmp/out.mp4")
	// Empty command should be skipped (extractCommand returns "")
	assert.NotContains(t, tape, `"$ "`)
}

func TestExtractCommand_StripsDescriptionSuffix_Good(t *testing.T) {
	assert.Equal(t, "ls -la", extractCommand("ls -la # list files"))
	assert.Equal(t, "go test ./...", extractCommand("go test ./..."))
	assert.Equal(t, "echo hello", extractCommand("echo hello"))
}

func TestExtractCommand_NoDescription_Good(t *testing.T) {
	assert.Equal(t, "plain command", extractCommand("plain command"))
}

func TestExtractCommand_DescriptionAtStart_Good(t *testing.T) {
	// " # " at position 0 means idx <= 0, so it returns the whole input
	result := extractCommand(" # description only")
	assert.Equal(t, " # description only", result)
}

func TestRenderMP4_NoVHS_Ugly(t *testing.T) {
	// Skip if vhs is actually installed (this tests the error path)
	if lookupExecutable("vhs") != "" {
		t.Skip("vhs is installed; skipping missing-vhs test")
	}

	sess := &Session{
		ID:        "no-vhs",
		StartTime: time.Now(),
	}

	err := RenderMP4(sess, "/tmp/test.mp4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vhs not installed")
}
