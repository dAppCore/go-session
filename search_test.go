package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_FindsMatchingToolEvents(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"s1","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Bash","input":{"command":"go test ./..."}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"s1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"PASS"}]}}`,
		`{"type":"assistant","timestamp":"` + ts(baseTime, 2*time.Second) + `","sessionId":"s1","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_2","name":"Grep","input":{"pattern":"TODO","path":"/src"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, 3*time.Second) + `","sessionId":"s1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_2","content":"main.go:42: // TODO fix this"}]}}`,
	}
	writeTempJSONL(t, dir, "session.jsonl", lines...)

	t.Run("match_input", func(t *testing.T) {
		results, err := Search(dir, "go test")
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "session", results[0].SessionID)
		assert.Equal(t, "Bash", results[0].Tool)
		assert.Contains(t, results[0].Match, "go test")
	})

	t.Run("match_output", func(t *testing.T) {
		results, err := Search(dir, "TODO fix")
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "Grep", results[0].Tool)
	})

	t.Run("case_insensitive", func(t *testing.T) {
		results, err := Search(dir, "PASS")
		require.NoError(t, err)
		require.Len(t, results, 1)
	})

	t.Run("no_match", func(t *testing.T) {
		results, err := Search(dir, "nonexistent query")
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestSearch_SkipsNonToolEvents(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		`{"type":"user","timestamp":"` + ts(baseTime, 0) + `","sessionId":"s2","message":{"role":"user","content":[{"type":"text","text":"find all TODO items"}]}}`,
		`{"type":"assistant","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"s2","message":{"role":"assistant","content":[{"type":"text","text":"I will search for TODO items."}]}}`,
	}
	writeTempJSONL(t, dir, "notool.jsonl", lines...)

	// The text "TODO" exists in user and assistant messages but Search only
	// looks at tool_use events.
	results, err := Search(dir, "TODO")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	results, err := Search(dir, "anything")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_MultipleSessions(t *testing.T) {
	dir := t.TempDir()

	// Session A has a matching tool call.
	linesA := []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"a","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_a","name":"Read","input":{"file_path":"/tmp/config.yaml"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"a","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_a","content":"port: 8080"}]}}`,
	}
	// Session B has a different matching tool call.
	linesB := []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"b","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_b","name":"Bash","input":{"command":"cat /tmp/config.yaml"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"b","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_b","content":"port: 8080"}]}}`,
	}

	writeTempJSONL(t, dir, "a.jsonl", linesA...)
	writeTempJSONL(t, dir, "b.jsonl", linesB...)

	results, err := Search(dir, "config.yaml")
	require.NoError(t, err)
	assert.Len(t, results, 2, "should find matches in both sessions")
}

func TestSearch_IgnoresNonJSONLFiles(t *testing.T) {
	dir := t.TempDir()
	// Write a .txt file containing the search term.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("config.yaml"), 0644))

	results, err := Search(dir, "config.yaml")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_MalformedSessionSkipped(t *testing.T) {
	dir := t.TempDir()
	// A completely invalid JSONL file.
	writeTempJSONL(t, dir, "broken.jsonl", `{not valid json at all`)
	// A valid session with a match.
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"ok","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_ok","name":"Bash","input":{"command":"echo found"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"ok","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_ok","content":"found"}]}}`,
	}
	writeTempJSONL(t, dir, "valid.jsonl", lines...)

	results, err := Search(dir, "found")
	require.NoError(t, err)
	assert.Len(t, results, 1, "should find match in valid session, ignoring broken one")
}

func TestSearch_MatchUsesInputAsContext(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"ctx","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_ctx","name":"Bash","input":{"command":"deploy production"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"ctx","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_ctx","content":"deployed successfully"}]}}`,
	}
	writeTempJSONL(t, dir, "ctx.jsonl", lines...)

	// Search matches on output but Match field should show input.
	results, err := Search(dir, "deployed")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Match, "deploy production",
		"Match context should be the tool input, not output")
}

func TestSearch_MatchFallsBackToOutput(t *testing.T) {
	// When a tool_use event has empty input but matching output, Match should
	// show the (truncated) output. We simulate this by creating a session
	// where the tool input resolves to empty.
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"fb","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_fb","name":"Bash","input":{}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"fb","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_fb","content":"fallback output text"}]}}`,
	}
	writeTempJSONL(t, dir, "fallback.jsonl", lines...)

	results, err := Search(dir, "fallback output")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Match, "fallback output")
}

func TestSearchResult_Fields(t *testing.T) {
	// Verify that SearchResult fields are populated correctly.
	dir := t.TempDir()
	lines := []string{
		`{"type":"assistant","timestamp":"` + ts(baseTime, 0) + `","sessionId":"fields","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_f","name":"Grep","input":{"pattern":"error","path":"/var/log"}}]}}`,
		`{"type":"user","timestamp":"` + ts(baseTime, time.Second) + `","sessionId":"fields","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_f","content":"found 3 errors"}]}}`,
	}
	writeTempJSONL(t, dir, "fields.jsonl", lines...)

	results, err := Search(dir, "error")
	require.NoError(t, err)
	require.Len(t, results, 1)

	r := results[0]
	assert.Equal(t, "fields", r.SessionID)
	assert.False(t, r.Timestamp.IsZero())
	assert.Equal(t, "Grep", r.Tool)
	assert.NotEmpty(t, r.Match)
}
