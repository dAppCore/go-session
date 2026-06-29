// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"

	core "dappco.re/go"
)

// raw marshals m into a rawjson value for extractToolInput tests.
func raw(t *testing.T, m map[string]any) rawjson {
	t.Helper()
	result := core.JSONMarshal(m)
	core.RequireTrue(t, result.OK)
	return rawjson(result.Value.([]byte))
}

// --- extractToolInput ---

func TestParserInternal_extractToolInput_Good(t *testing.T) {
	core.AssertEqual(t, "echo hi # greet",
		extractToolInput("Bash", raw(t, map[string]any{"command": "echo hi", "description": "greet"})))
	core.AssertEqual(t, "/etc/hosts",
		extractToolInput("Read", raw(t, map[string]any{"file_path": "/etc/hosts"})))
	core.AssertEqual(t, "/tmp/x.go (edit)",
		extractToolInput("Edit", raw(t, map[string]any{"file_path": "/tmp/x.go"})))
	core.AssertEqual(t, "/tmp/y.go (5 bytes)",
		extractToolInput("Write", raw(t, map[string]any{"file_path": "/tmp/y.go", "content": "hello"})))
}

func TestParserInternal_extractToolInput_Bad(t *testing.T) {
	core.AssertEqual(t, "", extractToolInput("Bash", nil))
	// A Bash command with no description carries no " # " suffix.
	core.AssertEqual(t, "ls",
		extractToolInput("Bash", raw(t, map[string]any{"command": "ls"})))
}

func TestParserInternal_extractToolInput_Ugly(t *testing.T) {
	core.AssertEqual(t, "/p/ in src",
		extractToolInput("Grep", raw(t, map[string]any{"pattern": "p", "path": "src"})))
	// Empty Grep path defaults to ".".
	core.AssertEqual(t, "/q/ in .",
		extractToolInput("Grep", raw(t, map[string]any{"pattern": "q"})))
	core.AssertEqual(t, "**/*.go",
		extractToolInput("Glob", raw(t, map[string]any{"pattern": "**/*.go"})))
	// Task with an explicit description.
	core.AssertEqual(t, "[explorer] map the repo",
		extractToolInput("Task", raw(t, map[string]any{"subagent_type": "explorer", "description": "map the repo"})))
	// Task without a description falls back to the (truncated) prompt.
	core.AssertEqual(t, "[explorer] do the thing",
		extractToolInput("Task", raw(t, map[string]any{"subagent_type": "explorer", "prompt": "do the thing"})))
	// An unknown tool falls back to sorted JSON keys.
	core.AssertEqual(t, "alpha, zeta",
		extractToolInput("Unknown", raw(t, map[string]any{"zeta": 1, "alpha": 2})))
}

// --- extractResultContent ---

func TestParserInternal_extractResultContent_Good(t *testing.T) {
	core.AssertEqual(t, "plain text", extractResultContent("plain text"))
}

func TestParserInternal_extractResultContent_Bad(t *testing.T) {
	// A map block with a text field returns the text.
	core.AssertEqual(t, "boxed", extractResultContent(map[string]any{"text": "boxed"}))
	// A map block without text falls through to Sprint.
	got := extractResultContent(map[string]any{"other": 1})
	core.AssertContains(t, got, "other")
}

func TestParserInternal_extractResultContent_Ugly(t *testing.T) {
	// A slice of content blocks joins their text fields with newlines.
	content := []any{
		map[string]any{"type": "text", "text": "line one"},
		map[string]any{"type": "text", "text": "line two"},
		map[string]any{"type": "image"},
	}
	core.AssertEqual(t, "line one\nline two", extractResultContent(content))
}

// --- truncate ---

func TestParserInternal_truncate_Good(t *testing.T) {
	core.AssertEqual(t, "abc...", truncate("abcdef", 3))
}

func TestParserInternal_truncate_Bad(t *testing.T) {
	core.AssertEqual(t, "abc", truncate("abc", 10))
}

func TestParserInternal_truncate_Ugly(t *testing.T) {
	core.AssertEqual(t, "abc", truncate("abc", 3))
	core.AssertEqual(t, "...", truncate("abc", 0))
}

// --- trimLineBreak ---

func TestParserInternal_trimLineBreak_Good(t *testing.T) {
	core.AssertEqual(t, "line", string(trimLineBreak([]byte("line\r"))))
}

func TestParserInternal_trimLineBreak_Bad(t *testing.T) {
	core.AssertEqual(t, "line", string(trimLineBreak([]byte("line"))))
}

func TestParserInternal_trimLineBreak_Ugly(t *testing.T) {
	core.AssertEqual(t, "", string(trimLineBreak([]byte(""))))
	// Only one trailing carriage return is stripped.
	core.AssertEqual(t, "a\r", string(trimLineBreak([]byte("a\r\r"))))
}

// --- transcriptPath ---

func TestParserInternal_transcriptPath_Good(t *testing.T) {
	core.AssertEqual(t, "/projects/abc.jsonl", transcriptPath("/projects", "abc.jsonl"))
}

func TestParserInternal_transcriptPath_Bad(t *testing.T) {
	core.AssertEqual(t, "abc.jsonl", transcriptPath("", "abc.jsonl"))
}

func TestParserInternal_transcriptPath_Ugly(t *testing.T) {
	// Redundant separators are cleaned.
	core.AssertEqual(t, "/projects/abc.jsonl", transcriptPath("/projects/", "abc.jsonl"))
}
