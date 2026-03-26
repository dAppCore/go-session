// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"iter"
	"path"
	"slices"
	"time"

	core "dappco.re/go/core"
)

// SearchResult represents a match found in a session transcript.
type SearchResult struct {
	SessionID string
	Timestamp time.Time
	Tool      string
	Match     string
}

// Search finds events matching the query across all sessions in the directory.
func Search(projectsDir, query string) ([]SearchResult, error) {
	return slices.Collect(SearchSeq(projectsDir, query)), nil
}

// SearchSeq returns an iterator over search results matching the query across all sessions.
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
