// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"time"

	core "dappco.re/go"
)

func ExampleSession_EventsSeq() {
	sess := &Session{Events: []Event{{Type: "user"}}}
	for event := range sess.EventsSeq() {
		_ = event
	}
}

func ExampleListSessions() {
	_ = ListSessions("/tmp/claude-projects")
}

func ExampleListSessionsSeq() {
	for sess := range ListSessionsSeq("/tmp/claude-projects") {
		_ = sess
	}
}

func ExamplePruneSessions() {
	_ = PruneSessions("/tmp/claude-projects", 24*time.Hour)
}

func ExampleSession_IsExpired() {
	sess := &Session{EndTime: time.Now().Add(-48 * time.Hour)}
	_ = sess.IsExpired(24 * time.Hour)
}

func ExampleFetchSession() {
	_ = FetchSession("/tmp/claude-projects", "abc123")
}

func ExampleParseTranscript() {
	_ = ParseTranscript("/tmp/claude-projects/abc123.jsonl")
}

func ExampleParseTranscriptReader() {
	_ = ParseTranscriptReader(core.NewReader(""), "abc123")
}
