// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"
	"time"

	core "dappco.re/go/core"
)

func TestVideo_GenerateTapeBasicSession_Good(t *testing.T) {
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

	assertContains(t, tape, "Output /tmp/output.mp4")
	assertContains(t, tape, "Set FontSize 16")
	assertContains(t, tape, "tape-tes") // shortID
	assertContains(t, tape, "2026-02-20 10:00")
	assertContains(t, tape, `"$ go test ./..."`)
	assertContains(t, tape, "PASS")
	assertContains(t, tape, `"# ✓ OK"`)
	assertContains(t, tape, "# Read: /tmp/file.go")
}

func TestVideo_GenerateTapeSkipsNonToolEvents_Good(t *testing.T) {
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
	assertNotContains(t, tape, "Hello")
	assertNotContains(t, tape, "Hi there")
	// Bash command should appear
	assertContains(t, tape, "echo hi")
}

func TestVideo_GenerateTapeFailedCommand_Good(t *testing.T) {
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
	assertContains(t, tape, `"# ✗ FAILED"`)
}

func TestVideo_GenerateTapeLongOutput_Good(t *testing.T) {
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
	assertContains(t, tape, "...")
}

func TestVideo_GenerateTapeTaskEvent_Good(t *testing.T) {
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
	assertContains(t, tape, "# Agent: [research] Analyse code structure")
}

func TestVideo_GenerateTapeEditWriteEvents_Good(t *testing.T) {
	sess := &Session{
		ID:        "edit-test",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events: []Event{
			{Type: "tool_use", Tool: "Edit", Input: "/tmp/app.go (edit)"},
			{Type: "tool_use", Tool: "Write", Input: "/tmp/new.go (50 bytes)"},
		},
	}

	tape := generateTape(sess, "/tmp/out.mp4")
	assertContains(t, tape, "# Edit: /tmp/app.go (edit)")
	assertContains(t, tape, "# Write: /tmp/new.go (50 bytes)")
}

func TestVideo_GenerateTapeEmptySession_Good(t *testing.T) {
	sess := &Session{
		ID:        "empty-test",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events:    nil,
	}

	tape := generateTape(sess, "/tmp/out.mp4")

	// Should still have the header and trailer
	assertContains(t, tape, "Output /tmp/out.mp4")
	assertContains(t, tape, "Sleep 3s")
	// No tool events
	lines := core.Split(tape, "\n")
	var toolLines int
	for _, line := range lines {
		if core.Contains(line, "$ ") || core.Contains(line, "# Read:") ||
			core.Contains(line, "# Edit:") || core.Contains(line, "# Write:") {
			toolLines++
		}
	}
	assertEqual(t, 0, toolLines)
}

func TestVideo_GenerateTapeBashEmptyCommand_Bad(t *testing.T) {
	sess := &Session{
		ID:        "empty-cmd",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events: []Event{
			{Type: "tool_use", Tool: "Bash", Input: "", Output: "", Success: true},
		},
	}

	tape := generateTape(sess, "/tmp/out.mp4")
	// Empty command should be skipped (extractCommand returns "")
	assertNotContains(t, tape, `"$ "`)
}

func TestVideo_ExtractCommandStripsDescriptionSuffix_Good(t *testing.T) {
	assertEqual(t, "ls -la", extractCommand("ls -la # list files"))
	assertEqual(t, "go test ./...", extractCommand("go test ./..."))
	assertEqual(t, "echo hello", extractCommand("echo hello"))
}

func TestVideo_ExtractCommandNoDescription_Good(t *testing.T) {
	assertEqual(t, "plain command", extractCommand("plain command"))
}

func TestVideo_ExtractCommandDescriptionAtStart_Good(t *testing.T) {
	// " # " at position 0 means idx <= 0, so it returns the whole input
	result := extractCommand(" # description only")
	assertEqual(t, " # description only", result)
}

func TestVideo_RenderMP4NoVHS_Ugly(t *testing.T) {
	// Skip if vhs is actually installed (this tests the error path)
	if lookupExecutable("vhs") != "" {
		t.Skip("vhs is installed; skipping missing-vhs test")
	}

	sess := &Session{
		ID:        "no-vhs",
		StartTime: time.Now(),
	}

	err := RenderMP4(sess, "/tmp/test.mp4")
	requireError(t, err)
	assertContains(t, err.Error(), "vhs not installed")
}
