package session

import (
	"iter"
	"path/filepath"
	"slices"
	"strings"
	"time"
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
		matches, err := filepath.Glob(filepath.Join(projectsDir, "*.jsonl"))
		if err != nil {
			return
		}

		query = strings.ToLower(query)

		for _, path := range matches {
			sess, _, err := ParseTranscript(path)
			if err != nil {
				continue
			}

			for evt := range sess.EventsSeq() {
				if evt.Type != "tool_use" {
					continue
				}
				text := strings.ToLower(evt.Input + " " + evt.Output)
				if strings.Contains(text, query) {
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
