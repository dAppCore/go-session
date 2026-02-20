// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderHTML_BasicSession_Good(t *testing.T) {
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
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	html := string(content)

	// Basic structure checks
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "test-ses") // shortID of "test-session-12345678"
	assert.Contains(t, html, "2026-02-20 10:00:00")
	assert.Contains(t, html, "5m30s") // duration
	assert.Contains(t, html, "2 tool calls")
	assert.Contains(t, html, "ls -la")
	assert.Contains(t, html, "total 42")
	assert.Contains(t, html, "/tmp/file.go")
	assert.Contains(t, html, "User")   // user event label
	assert.Contains(t, html, "Claude") // assistant event label
	assert.Contains(t, html, "Bash")
	assert.Contains(t, html, "Read")
	// Should contain JS for toggle and filter
	assert.Contains(t, html, "function toggle")
	assert.Contains(t, html, "function filterEvents")
}

func TestRenderHTML_EmptySession_Good(t *testing.T) {
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
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	html := string(content)
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "0 tool calls")
	// Should NOT contain error span
	assert.NotContains(t, html, "errors</span>")
}

func TestRenderHTML_WithErrors_Good(t *testing.T) {
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
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	html := string(content)
	assert.Contains(t, html, "1 errors")
	assert.Contains(t, html, `class="event error"`)
	assert.Contains(t, html, "&#10007;") // cross mark for failed
	assert.Contains(t, html, "&#10003;") // check mark for success
}

func TestRenderHTML_SpecialCharacters_Good(t *testing.T) {
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
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	html := string(content)

	// Script tags should be escaped, never raw
	assert.NotContains(t, html, "<script>alert")
	assert.Contains(t, html, "&lt;script&gt;")
	assert.Contains(t, html, "&amp;")
}

func TestRenderHTML_InvalidPath_Ugly(t *testing.T) {
	sess := &Session{
		ID:     "test",
		Events: nil,
	}

	err := RenderHTML(sess, "/nonexistent/dir/output.html")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create html")
}

func TestRenderHTML_LabelsByToolType_Good(t *testing.T) {
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
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	html := string(content)

	// Bash gets "Command" label
	assert.True(t, strings.Contains(html, "Command"), "Bash events should use 'Command' label")
	// Read, Glob, Grep get "Target" label
	assert.True(t, strings.Contains(html, "Target"), "Read/Glob/Grep events should use 'Target' label")
	// Edit, Write get "File" label
	assert.True(t, strings.Contains(html, "File"), "Edit/Write events should use 'File' label")
}
