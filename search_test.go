// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_EmptyDir_Good(t *testing.T) {
	dir := t.TempDir()

	results, err := Search(dir, "anything")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_NoMatches_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "session.jsonl",
		toolUseEntry(ts(0), "Bash", "tool-1", map[string]any{
			"command": "ls -la",
		}),
		toolResultEntry(ts(1), "tool-1", "total 42", false),
	)

	results, err := Search(dir, "nonexistent-query")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_SingleMatch_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "session.jsonl",
		toolUseEntry(ts(0), "Bash", "tool-1", map[string]any{
			"command": "go test ./...",
		}),
		toolResultEntry(ts(1), "tool-1", "PASS ok mypackage 0.5s", false),
	)

	results, err := Search(dir, "go test")
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, "session", results[0].SessionID)
	assert.Equal(t, "Bash", results[0].Tool)
	assert.Contains(t, results[0].Match, "go test")
}

func TestSearchSeq_SingleMatch_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "session.jsonl",
		toolUseEntry(ts(0), "Bash", "tool-1", map[string]any{
			"command": "go test ./...",
		}),
		toolResultEntry(ts(1), "tool-1", "PASS ok mypackage 0.5s", false),
	)

	var results []SearchResult
	for r := range SearchSeq(dir, "go test") {
		results = append(results, r)
	}

	require.Len(t, results, 1)
	assert.Equal(t, "session", results[0].SessionID)
	assert.Equal(t, "Bash", results[0].Tool)
}

func TestSearch_MultipleMatches_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "session1.jsonl",
		toolUseEntry(ts(0), "Bash", "t1", map[string]any{
			"command": "go test ./...",
		}),
		toolResultEntry(ts(1), "t1", "PASS", false),
		toolUseEntry(ts(2), "Bash", "t2", map[string]any{
			"command": "go test -race ./...",
		}),
		toolResultEntry(ts(3), "t2", "PASS", false),
	)
	writeJSONL(t, dir, "session2.jsonl",
		toolUseEntry(ts(0), "Bash", "t3", map[string]any{
			"command": "go test -bench=.",
		}),
		toolResultEntry(ts(1), "t3", "PASS", false),
	)

	results, err := Search(dir, "go test")
	require.NoError(t, err)
	assert.Len(t, results, 3, "should find matches across both sessions")
}

func TestSearch_CaseInsensitive_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "session.jsonl",
		toolUseEntry(ts(0), "Bash", "t1", map[string]any{
			"command": "GO TEST ./...",
		}),
		toolResultEntry(ts(1), "t1", "PASS", false),
	)

	results, err := Search(dir, "go test")
	require.NoError(t, err)
	assert.Len(t, results, 1, "search should be case-insensitive")
}

func TestSearch_MatchesInOutput_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "session.jsonl",
		toolUseEntry(ts(0), "Bash", "t1", map[string]any{
			"command": "cat log.txt",
		}),
		toolResultEntry(ts(1), "t1", "ERROR: connection refused to database", false),
	)

	results, err := Search(dir, "connection refused")
	require.NoError(t, err)
	require.Len(t, results, 1, "should match against output text")
	// Match field should contain the input (command) since it's non-empty
	assert.Contains(t, results[0].Match, "cat log.txt")
}

func TestSearch_SkipsNonToolEvents_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "session.jsonl",
		userTextEntry(ts(0), "Please search for something"),
		assistantTextEntry(ts(1), "I will search for something"),
	)

	// "search" appears in user and assistant text, but Search only checks tool_use events
	results, err := Search(dir, "search")
	require.NoError(t, err)
	assert.Empty(t, results, "should only match tool_use events, not user/assistant text")
}

func TestSearch_NonJSONLIgnored_Good(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte("go test"), 0644))

	results, err := Search(dir, "go test")
	require.NoError(t, err)
	assert.Empty(t, results, "non-JSONL files should be ignored")
}

func TestSearch_MalformedSessionSkipped_Bad(t *testing.T) {
	dir := t.TempDir()

	// One broken session and one valid session
	writeJSONL(t, dir, "broken.jsonl",
		`{not valid json at all`,
	)
	writeJSONL(t, dir, "valid.jsonl",
		toolUseEntry(ts(0), "Bash", "t1", map[string]any{
			"command": "go test ./...",
		}),
		toolResultEntry(ts(1), "t1", "PASS", false),
	)

	results, err := Search(dir, "go test")
	require.NoError(t, err)
	assert.Len(t, results, 1, "should still find matches in valid sessions")
}
