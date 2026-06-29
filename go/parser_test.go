// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"errors"
	"syscall"
	"testing"
	"time"

	core "dappco.re/go"
)

// failingReader returns a fixed error on first read, used to exercise the
// scan-failure propagation path.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("synthetic read failure")
}

func ts() string {
	return time.Unix(1, 0).UTC().Format(time.RFC3339Nano)
}

func jsonLine(t *testing.T, m map[string]any) string {
	t.Helper()
	result := core.JSONMarshal(m)
	core.RequireTrue(t, result.OK)
	return string(result.Value.([]byte))
}

func userTextEntry(text string) string {
	result := core.JSONMarshal(map[string]any{
		"type":      "user",
		"timestamp": ts(),
		"sessionId": "test-session",
		"message":   map[string]any{"role": "user", "content": []map[string]any{{"type": "text", "text": text}}},
	})
	return string(result.Value.([]byte))
}

func toolUseEntry(tool, id string, input map[string]any) string {
	result := core.JSONMarshal(map[string]any{
		"type":      "assistant",
		"timestamp": ts(),
		"sessionId": "test-session",
		"message":   map[string]any{"role": "assistant", "content": []map[string]any{{"type": "tool_use", "name": tool, "id": id, "input": input}}},
	})
	return string(result.Value.([]byte))
}

func toolResultEntry(id string, content any, failed bool) string {
	result := core.JSONMarshal(map[string]any{
		"type":      "user",
		"timestamp": ts(),
		"sessionId": "test-session",
		"message":   map[string]any{"role": "user", "content": []map[string]any{{"type": "tool_result", "tool_use_id": id, "content": content, "is_error": failed}}},
	})
	return string(result.Value.([]byte))
}

func writeJSONL(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	file := core.PathJoin(dir, name)
	result := hostFS.Write(file, core.Concat(core.Join("\n", lines...), "\n"))
	core.RequireTrue(t, result.OK)
	return file
}

func parsedValue(t *testing.T, result core.Result) ParsedSession {
	t.Helper()
	core.RequireTrue(t, result.OK, result.Error())
	return result.Value.(ParsedSession)
}

func TestParser_Session_EventsSeq_Good(t *testing.T) {
	sess := &Session{Events: []Event{{Type: "user"}, {Type: "assistant"}}}

	count := 0
	for range sess.EventsSeq() {
		count++
	}

	core.AssertEqual(t, 2, count)
}

func TestParser_Session_EventsSeq_Bad(t *testing.T) {
	sess := &Session{}

	for range sess.EventsSeq() {
		t.Fatal("empty EventsSeq yielded an event")
	}
}

func TestParser_Session_EventsSeq_Ugly(t *testing.T) {
	sess := &Session{Events: []Event{{Type: "tool_use", Tool: "Bash"}}}

	var last Event
	for item := range sess.EventsSeq() {
		last = item
	}

	core.AssertEqual(t, "Bash", last.Tool)
}

func TestParser_ListSessions_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "b.jsonl", userTextEntry("second"))
	writeJSONL(t, dir, "a.jsonl", userTextEntry("first"))

	result := ListSessions(dir)

	core.RequireTrue(t, result.OK)
	core.AssertLen(t, result.Value.([]Session), 2)
}

func TestParser_ListSessions_Bad(t *testing.T) {
	result := ListSessions(t.TempDir())

	core.RequireTrue(t, result.OK)
	core.AssertEmpty(t, result.Value.([]Session))
}

func TestParser_ListSessions_Ugly(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "bad.jsonl", "{")

	result := ListSessions(dir)

	core.RequireTrue(t, result.OK)
	core.AssertLen(t, result.Value.([]Session), 1)
}

func TestParser_ListSessionsSeq_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "one.jsonl", userTextEntry("hi"))

	var sessions []Session
	for sess := range ListSessionsSeq(dir) {
		sessions = append(sessions, sess)
	}

	core.AssertLen(t, sessions, 1)
}

func TestParser_ListSessionsSeq_Bad(t *testing.T) {
	for range ListSessionsSeq(t.TempDir()) {
		t.Fatal("empty ListSessionsSeq yielded a session")
	}
}

func TestParser_ListSessionsSeq_Ugly(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "one.jsonl", "")

	var sessions []Session
	for sess := range ListSessionsSeq(dir) {
		sessions = append(sessions, sess)
	}

	core.AssertLen(t, sessions, 1)
}

func TestParser_PruneSessions_Good(t *testing.T) {
	dir := t.TempDir()
	file := writeJSONL(t, dir, "old.jsonl", userTextEntry("old"))
	past := time.Now().Add(-48 * time.Hour)
	core.RequireNoError(t, syscall.UtimesNano(file, []syscall.Timespec{
		syscall.NsecToTimespec(past.UnixNano()),
		syscall.NsecToTimespec(past.UnixNano()),
	}))

	result := PruneSessions(dir, time.Hour)

	core.RequireTrue(t, result.OK)
	core.AssertEqual(t, 1, result.Value.(int))
}

func TestParser_PruneSessions_Bad(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "new.jsonl", userTextEntry("new"))

	result := PruneSessions(dir, 48*time.Hour)

	core.RequireTrue(t, result.OK)
	core.AssertEqual(t, 0, result.Value.(int))
}

func TestParser_PruneSessions_Ugly(t *testing.T) {
	result := PruneSessions(t.TempDir(), -time.Hour)

	core.RequireTrue(t, result.OK)
	core.AssertEqual(t, 0, result.Value.(int))
}

func TestParser_Session_IsExpired_Good(t *testing.T) {
	sess := &Session{EndTime: time.Now().Add(-2 * time.Hour)}
	expired := sess.IsExpired(time.Hour)

	core.AssertTrue(t, expired)
	core.AssertFalse(t, sess.EndTime.IsZero())
}

func TestParser_Session_IsExpired_Bad(t *testing.T) {
	sess := &Session{EndTime: time.Now()}
	expired := sess.IsExpired(time.Hour)

	core.AssertFalse(t, expired)
	core.AssertFalse(t, sess.EndTime.IsZero())
}

func TestParser_Session_IsExpired_Ugly(t *testing.T) {
	sess := &Session{}
	expired := sess.IsExpired(time.Hour)

	core.AssertFalse(t, expired)
	core.AssertTrue(t, sess.EndTime.IsZero())
}

func TestParser_FetchSession_Good(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "abc.jsonl", userTextEntry("hello"))

	parsed := parsedValue(t, FetchSession(dir, "abc"))

	core.AssertEqual(t, "abc", parsed.Session.ID)
	core.AssertEqual(t, 1, parsed.Stats.TotalLines)
}

func TestParser_FetchSession_Bad(t *testing.T) {
	result := FetchSession(t.TempDir(), "missing")

	core.AssertFalse(t, result.OK)
	core.AssertContains(t, result.Error(), "open transcript")
}

func TestParser_FetchSession_Ugly(t *testing.T) {
	result := FetchSession(t.TempDir(), "../escape")

	core.AssertFalse(t, result.OK)
	core.AssertContains(t, result.Error(), "invalid session id")
}

func TestParser_ParseTranscript_Good(t *testing.T) {
	file := writeJSONL(t, t.TempDir(), "ok.jsonl", userTextEntry("hello"))

	parsed := parsedValue(t, ParseTranscript(file))

	core.AssertEqual(t, "ok", parsed.Session.ID)
	core.AssertLen(t, parsed.Session.Events, 1)
}

func TestParser_ParseTranscript_Bad(t *testing.T) {
	result := ParseTranscript(core.PathJoin(t.TempDir(), "missing.jsonl"))

	core.AssertFalse(t, result.OK)
	core.AssertContains(t, result.Error(), "open transcript")
}

func TestParser_ParseTranscript_Ugly(t *testing.T) {
	file := writeJSONL(t, t.TempDir(), "bad.jsonl", "{")

	parsed := parsedValue(t, ParseTranscript(file))

	core.AssertEqual(t, 1, parsed.Stats.SkippedLines)
	core.AssertContains(t, core.Join("\n", parsed.Stats.Warnings...), "bad JSON")
}

func TestParser_ParseTranscriptReader_Good(t *testing.T) {
	parsed := parsedValue(t, ParseTranscriptReader(core.NewReader(userTextEntry("hi")), "reader"))

	core.AssertEqual(t, "reader", parsed.Session.ID)
	core.AssertLen(t, parsed.Session.Events, 1)
}

func TestParser_ParseTranscriptReader_Bad(t *testing.T) {
	parsed := parsedValue(t, ParseTranscriptReader(core.NewReader("{"), "reader"))

	core.AssertEqual(t, 1, parsed.Stats.SkippedLines)
}

func TestParser_ParseTranscriptReader_Ugly(t *testing.T) {
	parsed := parsedValue(t, ParseTranscriptReader(core.NewReader(""), "empty"))

	core.AssertEqual(t, "empty", parsed.Session.ID)
	core.AssertEmpty(t, parsed.Session.Events)
}

// assistantTextEntry builds an assistant message carrying a plain text block.
func assistantTextEntry(text string) string {
	result := core.JSONMarshal(map[string]any{
		"type":      "assistant",
		"timestamp": ts(),
		"sessionId": "test-session",
		"message":   map[string]any{"role": "assistant", "content": []map[string]any{{"type": "text", "text": text}}},
	})
	return string(result.Value.([]byte))
}

// TestParser_parseFromReader_BadTimestamp records a warning and skips the
// event when an entry carries an unparseable timestamp.
func TestParser_parseFromReader_BadTimestamp(t *testing.T) {
	line := jsonLine(t, map[string]any{
		"type":      "user",
		"timestamp": "not-a-timestamp",
		"message":   map[string]any{"content": []map[string]any{{"type": "text", "text": "hi"}}},
	})

	parsed := parsedValue(t, ParseTranscriptReader(core.NewReader(line), "ts"))

	core.AssertEmpty(t, parsed.Session.Events)
	core.AssertContains(t, core.Join("\n", parsed.Stats.Warnings...), "bad timestamp")
}

// TestParser_parseFromReader_AssistantText captures assistant text blocks as
// assistant events.
func TestParser_parseFromReader_AssistantText(t *testing.T) {
	parsed := parsedValue(t, ParseTranscriptReader(core.NewReader(assistantTextEntry("thinking")), "asst"))

	core.AssertLen(t, parsed.Session.Events, 1)
	core.AssertEqual(t, "assistant", parsed.Session.Events[0].Type)
	core.AssertEqual(t, "thinking", parsed.Session.Events[0].Input)
}

// TestParser_parseFromReader_OrphanedToolCall reports a tool_use with no
// matching tool_result as orphaned.
func TestParser_parseFromReader_OrphanedToolCall(t *testing.T) {
	parsed := parsedValue(t, ParseTranscriptReader(
		core.NewReader(toolUseEntry("Bash", "orphan-1", map[string]any{"command": "ls"})), "orph"))

	core.AssertEqual(t, 1, parsed.Stats.OrphanedToolCalls)
	core.AssertContains(t, core.Join("\n", parsed.Stats.Warnings...), "orphaned tool call: orphan-1")
	core.AssertEmpty(t, parsed.Session.Events)
}

// TestParser_parseFromReader_TruncatedFinalLine flags a trailing malformed
// line as a truncated final line.
func TestParser_parseFromReader_TruncatedFinalLine(t *testing.T) {
	input := core.Concat(userTextEntry("ok"), "\n", "{partial")

	parsed := parsedValue(t, ParseTranscriptReader(core.NewReader(input), "trunc"))

	core.AssertContains(t, core.Join("\n", parsed.Stats.Warnings...), "truncated final line")
}

// TestParser_ParseTranscriptReader_ScanError propagates a reader failure as
// a wrapped parse error rather than silently returning an empty session.
func TestParser_ParseTranscriptReader_ScanError(t *testing.T) {
	result := ParseTranscriptReader(failingReader{}, "boom")

	core.AssertFalse(t, result.OK)
	core.AssertContains(t, result.Error(), "parse transcript")
}

// TestParser_ListSessionsSeq_EarlyBreak stops the iterator after the first
// session, exercising the yield-false return path.
func TestParser_ListSessionsSeq_EarlyBreak(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "a.jsonl", userTextEntry("a"))
	writeJSONL(t, dir, "b.jsonl", userTextEntry("b"))

	count := 0
	for range ListSessionsSeq(dir) {
		count++
		break
	}

	core.AssertEqual(t, 1, count)
}
