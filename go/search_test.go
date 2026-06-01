// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"

	core "dappco.re/go"
)

func TestSearch_Search_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "one.jsonl", toolUseEntry("Bash", "tool-1", map[string]any{"command": "go test"}), toolResultEntry("tool-1", "PASS", false))

	result := Search(dir, "go test")

	core.RequireTrue(t, result.OK)
	matches := result.Value.([]SearchResult)
	core.AssertLen(t, matches, 1)
	core.AssertEqual(t, "one", matches[0].SessionID)
}

func TestSearch_Search_Bad(t *testing.T) {
	result := Search(t.TempDir(), "absent")

	core.RequireTrue(t, result.OK)
	core.AssertEmpty(t, result.Value.([]SearchResult))
}

func TestSearch_Search_Ugly(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "broken.jsonl", "{")
	writeJSONL(t, dir, "valid.jsonl", toolUseEntry("Bash", "tool-1", map[string]any{"command": "GO TEST"}), toolResultEntry("tool-1", "PASS", false))

	result := Search(dir, "go test")

	core.RequireTrue(t, result.OK)
	core.AssertLen(t, result.Value.([]SearchResult), 1)
}

func TestSearch_SearchSeq_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "one.jsonl", toolUseEntry("Bash", "tool-1", map[string]any{"command": "go vet"}), toolResultEntry("tool-1", "PASS", false))

	var matches []SearchResult
	for item := range SearchSeq(dir, "go vet") {
		matches = append(matches, item)
	}

	core.AssertLen(t, matches, 1)
}

func TestSearch_SearchSeq_Bad(t *testing.T) {
	var matches []SearchResult
	for item := range SearchSeq(t.TempDir(), "nothing") {
		matches = append(matches, item)
	}

	core.AssertEmpty(t, matches)
}

func TestSearch_SearchSeq_Ugly(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "text.jsonl", userTextEntry("please run go test"))

	var matches []SearchResult
	for item := range SearchSeq(dir, "go test") {
		matches = append(matches, item)
	}

	core.AssertEmpty(t, matches)
}

// TestSearch_SearchSeq_EarlyBreak stops after the first match, exercising
// the yield-false return path inside the iterator.
func TestSearch_SearchSeq_EarlyBreak(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "a.jsonl",
		toolUseEntry("Bash", "t1", map[string]any{"command": "go test ./a"}), toolResultEntry("t1", "PASS", false),
		toolUseEntry("Bash", "t2", map[string]any{"command": "go test ./b"}), toolResultEntry("t2", "PASS", false),
	)

	count := 0
	for range SearchSeq(dir, "go test") {
		count++
		break
	}

	core.AssertEqual(t, 1, count)
}

// TestSearch_SearchSeq_OutputMatch matches on event output when the query is
// absent from the input, returning a truncated output context.
func TestSearch_SearchSeq_OutputMatch(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "out.jsonl",
		toolUseEntry("Read", "r1", map[string]any{"file_path": "/tmp/x"}),
		toolResultEntry("r1", "the needle is in the output", false),
	)

	var matches []SearchResult
	for item := range SearchSeq(dir, "needle") {
		matches = append(matches, item)
	}

	core.AssertLen(t, matches, 1)
	core.AssertContains(t, matches[0].Match, "/tmp/x")
}
