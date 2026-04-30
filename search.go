// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"iter"   // Note: intrinsic — public lazy sequence API for search results; no core equivalent
	"path"   // Note: intrinsic — slash-separated transcript glob path construction; no core equivalent
	"slices" // Note: intrinsic — slices.Collect materialises search iterator results; no core equivalent
	"time"   // Note: intrinsic — search result timestamps mirror parsed transcript event times; no core equivalent

	core "dappco.re/go"
)

// SearchResult represents a match found in a session transcript.
//
// Example:
// result := session.SearchResult{SessionID: "abc123", Tool: "Bash"}
type SearchResult struct {
	SessionID string
	Timestamp time.Time
	Tool      string
	Match     string
}

// Search finds events matching the query across all sessions in the directory.
//
// Example:
// results, err := session.Search("/tmp/projects", "go test")
func Search(projectsDir, query string) ([]SearchResult, error) {
	return slices.Collect(SearchSeq(projectsDir, query)), nil
}

// SearchSeq returns an iterator over search results matching the query across all sessions.
//
// Example:
//
//	for result := range session.SearchSeq("/tmp/projects", "go test") {
//		_ = result
//	}
func SearchSeq(projectsDir, query string) iter.Seq[SearchResult] {
	return func(yield func(SearchResult) bool) {
		matches := core.PathGlob(path.Join(projectsDir, "*.jsonl"))

		query = core.Lower(query)

		for _, filePath := range matches {
			sess, _, err := ParseTranscript(filePath)
			if err != nil {
				continue
			}

			for evt := range sess.EventsSeq() {
				if evt.Type != "tool_use" {
					continue
				}
				text := core.Lower(core.Concat(evt.Input, " ", evt.Output))
				if core.Contains(text, query) {
					matchCtx := evt.Input
					if matchCtx == "" {
						matchCtx = truncate(evt.Output, 120)
					}
					res := SearchResult{
						SessionID: sess.ID,
						Timestamp: evt.Timestamp,
						Tool:      evt.Tool,
						Match:     matchCtx,
					}
					if !yield(res) {
						return
					}
				}
			}
		}
	}
}
