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
