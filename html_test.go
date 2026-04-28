// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"
	"time"

	core "dappco.re/go"
)

// TestHTML_RenderHTMLBasicSession_Good verifies the behaviour covered by this test case.
func TestHTML_RenderHTMLBasicSession_Good(t *testing.T) {
	dir := t.TempDir()
	outputPath := dir + "/output.html"

	sess := &Session{
		ID:        "test-session-12345678",
		Path:      "/tmp/test.jsonl",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 20, 10, 5, 30, 0, time.UTC),
		Events: []Event{
			{
				Timestamp: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
				Type:      "user",
				Input:     "Hello, please help me",
			},
			{
				Timestamp: time.Date(2026, 2, 20, 10, 0, 1, 0, time.UTC),
				Type:      "assistant",
				Input:     "Sure, let me check.",
			},
			{
				Timestamp: time.Date(2026, 2, 20, 10, 0, 2, 0, time.UTC),
				Type:      "tool_use",
				Tool:      "Bash",
				ToolID:    "t1",
				Input:     "ls -la",
				Output:    "total 42",
				Duration:  time.Second,
				Success:   true,
			},
			{
				Timestamp: time.Date(2026, 2, 20, 10, 0, 4, 0, time.UTC),
				Type:      "tool_use",
				Tool:      "Read",
				ToolID:    "t2",
				Input:     "/tmp/file.go",
				Output:    "package main",
				Duration:  500 * time.Millisecond,
				Success:   true,
			},
		},
	}

	err := RenderHTML(sess, outputPath)
	requireNoError(t, err)

	readResult := hostFS.Read(outputPath)
	requireTrue(t, readResult.OK)
	html := readResult.Value.(string)

	// Basic structure checks
	assertContains(t, html, "<!DOCTYPE html>")
	assertContains(t, html, "test-ses") // shortID of "test-session-12345678"
	assertContains(t, html, "2026-02-20 10:00:00")
	assertContains(t, html, "5m30s") // duration
	assertContains(t, html, "2 tool calls")
	assertContains(t, html, "ls -la")
	assertContains(t, html, "total 42")
	assertContains(t, html, "/tmp/file.go")
	assertContains(t, html, "User")   // user event label
	assertContains(t, html, "Claude") // assistant event label
	assertContains(t, html, "Bash")
	assertContains(t, html, "Read")
	assertContains(t, html, `href="#evt-0"`)
	assertContains(t, html, "openHashEvent")
	// Should contain JS for toggle and filter
	assertContains(t, html, "function toggle")
	assertContains(t, html, "function filterEvents")
}

// TestHTML_RenderHTMLEmptySession_Good verifies the behaviour covered by this test case.
func TestHTML_RenderHTMLEmptySession_Good(t *testing.T) {
	dir := t.TempDir()
	outputPath := dir + "/empty.html"

	sess := &Session{
		ID:        "empty",
		Path:      "/tmp/empty.jsonl",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		Events:    nil,
	}

	err := RenderHTML(sess, outputPath)
	requireNoError(t, err)

	readResult := hostFS.Read(outputPath)
	requireTrue(t, readResult.OK)
	html := readResult.Value.(string)
	assertContains(t, html, "<!DOCTYPE html>")
	assertContains(t, html, "0 tool calls")
	// Should NOT contain error span
	assertNotContains(t, html, "errors</span>")
}

// TestHTML_RenderHTMLWithErrors_Good verifies the behaviour covered by this test case.
func TestHTML_RenderHTMLWithErrors_Good(t *testing.T) {
	dir := t.TempDir()
	outputPath := dir + "/errors.html"

	sess := &Session{
		ID:        "err-session",
		Path:      "/tmp/err.jsonl",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 20, 10, 1, 0, 0, time.UTC),
		Events: []Event{
			{
				Timestamp: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
				Type:      "tool_use",
				Tool:      "Bash",
				Input:     "cat /nonexistent",
				Output:    "No such file",
				Duration:  100 * time.Millisecond,
				Success:   false,
				ErrorMsg:  "No such file",
			},
			{
				Timestamp: time.Date(2026, 2, 20, 10, 0, 30, 0, time.UTC),
				Type:      "tool_use",
				Tool:      "Bash",
				Input:     "echo ok",
				Output:    "ok",
				Duration:  50 * time.Millisecond,
				Success:   true,
			},
		},
	}

	err := RenderHTML(sess, outputPath)
	requireNoError(t, err)

	readResult := hostFS.Read(outputPath)
	requireTrue(t, readResult.OK)
	html := readResult.Value.(string)
	assertContains(t, html, "1 errors")
	assertContains(t, html, `class="event error"`)
	assertContains(t, html, "&#10007;") // cross mark for failed
	assertContains(t, html, "&#10003;") // check mark for success
}

// TestHTML_RenderHTMLSpecialCharacters_Good verifies the behaviour covered by this test case.
func TestHTML_RenderHTMLSpecialCharacters_Good(t *testing.T) {
	dir := t.TempDir()
	outputPath := dir + "/special.html"

	sess := &Session{
		ID:        "special",
		Path:      "/tmp/special.jsonl",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 20, 10, 0, 1, 0, time.UTC),
		Events: []Event{
			{
				Timestamp: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
				Type:      "tool_use",
				Tool:      "Bash",
				Input:     `echo "<script>alert('xss')</script>"`,
				Output:    `<script>alert('xss')</script>`,
				Duration:  time.Second,
				Success:   true,
			},
			{
				Timestamp: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
				Type:      "user",
				Input:     `User says: "quotes & <brackets>"`,
			},
		},
	}

	err := RenderHTML(sess, outputPath)
	requireNoError(t, err)

	readResult := hostFS.Read(outputPath)
	requireTrue(t, readResult.OK)
	html := readResult.Value.(string)

	// Script tags should be escaped, never raw
	assertNotContains(t, html, "<script>alert")
	assertContains(t, html, "&lt;script&gt;")
	assertContains(t, html, "&amp;")
}

// TestHTML_RenderHTMLInvalidPath_Ugly verifies the behaviour covered by this test case.
func TestHTML_RenderHTMLInvalidPath_Ugly(t *testing.T) {
	sess := &Session{
		ID:     "test",
		Events: nil,
	}

	err := RenderHTML(sess, "/nonexistent/dir/output.html")
	requireError(t, err)
	assertContains(t, err.Error(), "parent directory does not exist")
}

// TestHTML_RenderHTMLLabelsByToolType_Good verifies the behaviour covered by this test case.
func TestHTML_RenderHTMLLabelsByToolType_Good(t *testing.T) {
	dir := t.TempDir()
	outputPath := dir + "/labels.html"

	sess := &Session{
		ID:        "labels",
		Path:      "/tmp/labels.jsonl",
		StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 20, 10, 0, 5, 0, time.UTC),
		Events: []Event{
			{Type: "tool_use", Tool: "Bash", Input: "ls", Timestamp: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC), Success: true},
			{Type: "tool_use", Tool: "Read", Input: "/file", Timestamp: time.Date(2026, 2, 20, 10, 0, 1, 0, time.UTC), Success: true},
			{Type: "tool_use", Tool: "Glob", Input: "**/*.go", Timestamp: time.Date(2026, 2, 20, 10, 0, 2, 0, time.UTC), Success: true},
			{Type: "tool_use", Tool: "Grep", Input: "/TODO/ in .", Timestamp: time.Date(2026, 2, 20, 10, 0, 3, 0, time.UTC), Success: true},
			{Type: "tool_use", Tool: "Edit", Input: "/file (edit)", Timestamp: time.Date(2026, 2, 20, 10, 0, 4, 0, time.UTC), Success: true},
			{Type: "tool_use", Tool: "Write", Input: "/file (100 bytes)", Timestamp: time.Date(2026, 2, 20, 10, 0, 5, 0, time.UTC), Success: true},
		},
	}

	err := RenderHTML(sess, outputPath)
	requireNoError(t, err)

	readResult := hostFS.Read(outputPath)
	requireTrue(t, readResult.OK)
	html := readResult.Value.(string)

	// Bash gets "Command" label
	assertTrue(t, core.Contains(html, "Command"), "Bash events should use 'Command' label")
	// Read, Glob, Grep get "Target" label
	assertTrue(t, core.Contains(html, "Target"), "Read/Glob/Grep events should use 'Target' label")
	// Edit, Write get "File" label
	assertTrue(t, core.Contains(html, "File"), "Edit/Write events should use 'File' label")
}
