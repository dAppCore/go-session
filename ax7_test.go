// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"bytes"
	"time"

	. "dappco.re/go"
)

// --- Analyse ---

func TestAX7_Analyse_Good(t *T) {
	sess := &Session{Events: []Event{{Type: "tool_use", Tool: "Bash", Duration: 2 * time.Second, Success: true}}}
	a := Analyse(sess)

	AssertEqual(t, 1, a.EventCount)
	AssertEqual(t, 1.0, a.SuccessRate)
	AssertEqual(t, 2*time.Second, a.AvgLatency["Bash"])
}

func TestAX7_Analyse_Bad(t *T) {
	a := Analyse(nil)

	AssertNotNil(t, a)
	AssertEqual(t, 0, a.EventCount)
	AssertEmpty(t, a.ToolCounts)
}

func TestAX7_Analyse_Ugly(t *T) {
	sess := &Session{Events: []Event{{Type: "tool_use", Tool: "Read", Input: "abcd", Output: "abcdefgh", Success: false}}}
	a := Analyse(sess)

	AssertEqual(t, 1, a.EventCount)
	AssertEqual(t, 0.0, a.SuccessRate)
	AssertEqual(t, 1, a.EstimatedInputTokens)
	AssertEqual(t, 2, a.EstimatedOutputTokens)
}

// --- FormatAnalytics ---

func TestAX7_FormatAnalytics_Good(t *T) {
	a := &SessionAnalytics{
		Duration:    2 * time.Minute,
		ActiveTime:  30 * time.Second,
		EventCount:  3,
		SuccessRate: 1,
		ToolCounts:  map[string]int{"Bash": 1},
		ErrorCounts: map[string]int{},
		AvgLatency:  map[string]time.Duration{"Bash": time.Second},
		MaxLatency:  map[string]time.Duration{"Bash": time.Second},
	}
	output := FormatAnalytics(a)

	AssertContains(t, output, "Session Analytics")
	AssertContains(t, output, "100.0%")
	AssertContains(t, output, "Bash")
}

func TestAX7_FormatAnalytics_Bad(t *T) {
	var a *SessionAnalytics

	AssertPanics(t, func() {
		_ = FormatAnalytics(a)
	})
	AssertNil(t, a)
}

func TestAX7_FormatAnalytics_Ugly(t *T) {
	a := &SessionAnalytics{}
	output := FormatAnalytics(a)

	AssertContains(t, output, "0.0%")
	AssertNotContains(t, output, "Tool Breakdown")
	AssertContains(t, output, "Events:")
}

// --- Session.EventsSeq ---

func TestAX7_Session_EventsSeq_Good(t *T) {
	sess := &Session{Events: []Event{{Type: "user", Input: "hello"}, {Type: "assistant", Input: "hi"}}}
	var got []string
	for evt := range sess.EventsSeq() {
		got = append(got, evt.Type)
	}

	AssertEqual(t, []string{"user", "assistant"}, got)
	AssertLen(t, sess.Events, 2)
}

func TestAX7_Session_EventsSeq_Bad(t *T) {
	sess := &Session{}
	var got []Event
	for evt := range sess.EventsSeq() {
		got = append(got, evt)
	}

	AssertEmpty(t, got)
	AssertEmpty(t, sess.Events)
}

func TestAX7_Session_EventsSeq_Ugly(t *T) {
	sess := &Session{Events: []Event{{Type: ""}}}
	var got []Event
	for evt := range sess.EventsSeq() {
		got = append(got, evt)
	}

	AssertLen(t, got, 1)
	AssertEqual(t, "", got[0].Type)
}

// --- Session.IsExpired ---

func TestAX7_Session_IsExpired_Good(t *T) {
	sess := &Session{EndTime: time.Now().Add(-2 * time.Hour)}
	expired := sess.IsExpired(time.Hour)

	AssertTrue(t, expired)
	AssertFalse(t, sess.EndTime.IsZero())
}

func TestAX7_Session_IsExpired_Bad(t *T) {
	sess := &Session{}
	expired := sess.IsExpired(time.Hour)

	AssertFalse(t, expired)
	AssertTrue(t, sess.EndTime.IsZero())
}

func TestAX7_Session_IsExpired_Ugly(t *T) {
	sess := &Session{EndTime: time.Now().Add(time.Hour)}
	expired := sess.IsExpired(time.Nanosecond)

	AssertFalse(t, expired)
	AssertTrue(t, sess.EndTime.After(time.Now()))
}

// --- ListSessions ---

func TestAX7_ListSessions_Good(t *T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "alpha.jsonl", userTextEntry(ts(0), "hello"))
	sessions, err := ListSessions(dir)

	RequireNoError(t, err)
	AssertLen(t, sessions, 1)
	AssertEqual(t, "alpha", sessions[0].ID)
}

func TestAX7_ListSessions_Bad(t *T) {
	dir := t.TempDir()
	sessions, err := ListSessions(dir)

	RequireNoError(t, err)
	AssertEmpty(t, sessions)
	AssertLen(t, sessions, 0)
}

func TestAX7_ListSessions_Ugly(t *T) {
	dir := t.TempDir()
	writeResult := hostFS.Write(Path(dir, "notes.txt"), "not a transcript")
	RequireTrue(t, writeResult.OK)
	sessions, err := ListSessions(dir)

	RequireNoError(t, err)
	AssertEmpty(t, sessions)
	AssertTrue(t, writeResult.OK)
}

// --- ListSessionsSeq ---

func TestAX7_ListSessionsSeq_Good(t *T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "alpha.jsonl", userTextEntry(ts(0), "hello"))
	var sessions []Session
	for sess := range ListSessionsSeq(dir) {
		sessions = append(sessions, sess)
	}

	AssertLen(t, sessions, 1)
	AssertEqual(t, "alpha", sessions[0].ID)
}

func TestAX7_ListSessionsSeq_Bad(t *T) {
	var sessions []Session
	for sess := range ListSessionsSeq(Path(t.TempDir(), "missing")) {
		sessions = append(sessions, sess)
	}

	AssertEmpty(t, sessions)
	AssertLen(t, sessions, 0)
}

func TestAX7_ListSessionsSeq_Ugly(t *T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "alpha.jsonl", userTextEntry(ts(0), "alpha"))
	writeJSONL(t, dir, "bravo.jsonl", userTextEntry(ts(1), "bravo"))
	var first Session
	for sess := range ListSessionsSeq(dir) {
		first = sess
		break
	}

	AssertNotEqual(t, "", first.ID)
	AssertTrue(t, first.StartTime.After(time.Time{}))
}

// --- PruneSessions ---

func TestAX7_PruneSessions_Good(t *T) {
	dir := t.TempDir()
	oldPath := writeJSONL(t, dir, "old.jsonl", userTextEntry(ts(0), "old"))
	oldTime := time.Now().Add(-2 * time.Hour)
	RequireNoError(t, setFileTimes(oldPath, oldTime, oldTime))
	deleted, err := PruneSessions(dir, time.Hour)

	RequireNoError(t, err)
	AssertEqual(t, 1, deleted)
	AssertFalse(t, hostFS.Stat(oldPath).OK)
}

func TestAX7_PruneSessions_Bad(t *T) {
	dir := t.TempDir()
	recentPath := writeJSONL(t, dir, "recent.jsonl", userTextEntry(ts(0), "recent"))
	deleted, err := PruneSessions(dir, time.Hour)

	RequireNoError(t, err)
	AssertEqual(t, 0, deleted)
	AssertTrue(t, hostFS.Stat(recentPath).OK)
}

func TestAX7_PruneSessions_Ugly(t *T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "now.jsonl", userTextEntry(ts(0), "now"))
	deleted, err := PruneSessions(dir, -time.Nanosecond)

	RequireNoError(t, err)
	AssertEqual(t, 1, deleted)
	sessions, err := ListSessions(dir)
	RequireNoError(t, err)
	AssertEmpty(t, sessions)
}

// --- FetchSession ---

func TestAX7_FetchSession_Good(t *T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "alpha.jsonl", userTextEntry(ts(0), "hello"))
	sess, stats, err := FetchSession(dir, "alpha")

	RequireNoError(t, err)
	AssertEqual(t, "alpha", sess.ID)
	AssertEqual(t, 1, stats.TotalLines)
}

func TestAX7_FetchSession_Bad(t *T) {
	dir := t.TempDir()
	sess, stats, err := FetchSession(dir, "missing")

	AssertError(t, err)
	AssertNil(t, sess)
	AssertNil(t, stats)
}

func TestAX7_FetchSession_Ugly(t *T) {
	dir := t.TempDir()
	sess, stats, err := FetchSession(dir, "../outside")

	AssertError(t, err)
	AssertNil(t, sess)
	AssertNil(t, stats)
	AssertContains(t, err.Error(), "invalid session id")
}

// --- ParseTranscript ---

func TestAX7_ParseTranscript_Good(t *T) {
	dir := t.TempDir()
	filePath := writeJSONL(t, dir, "alpha.jsonl", userTextEntry(ts(0), "hello"))
	sess, stats, err := ParseTranscript(filePath)

	RequireNoError(t, err)
	AssertEqual(t, "alpha", sess.ID)
	AssertEqual(t, 1, stats.TotalLines)
}

func TestAX7_ParseTranscript_Bad(t *T) {
	sess, stats, err := ParseTranscript(Path(t.TempDir(), "missing.jsonl"))

	AssertError(t, err)
	AssertNil(t, sess)
	AssertNil(t, stats)
}

func TestAX7_ParseTranscript_Ugly(t *T) {
	dir := t.TempDir()
	filePath := writeJSONL(t, dir, "mixed.jsonl", "{bad json", userTextEntry(ts(0), "good"))
	sess, stats, err := ParseTranscript(filePath)

	RequireNoError(t, err)
	AssertLen(t, sess.Events, 1)
	AssertEqual(t, 1, stats.SkippedLines)
}

// --- ParseTranscriptReader ---

func TestAX7_ParseTranscriptReader_Good(t *T) {
	reader := bytes.NewBufferString(userTextEntry(ts(0), "hello") + "\n")
	sess, stats, err := ParseTranscriptReader(reader, "reader")

	RequireNoError(t, err)
	AssertEqual(t, "reader", sess.ID)
	AssertEqual(t, 1, stats.TotalLines)
}

func TestAX7_ParseTranscriptReader_Bad(t *T) {
	reader := bytes.NewBufferString("{bad json\n")
	sess, stats, err := ParseTranscriptReader(reader, "bad")

	RequireNoError(t, err)
	AssertNotNil(t, sess)
	AssertEqual(t, 1, stats.SkippedLines)
}

func TestAX7_ParseTranscriptReader_Ugly(t *T) {
	reader := bytes.NewBufferString("")
	sess, stats, err := ParseTranscriptReader(reader, "")

	RequireNoError(t, err)
	AssertEqual(t, "", sess.ID)
	AssertEqual(t, 0, stats.TotalLines)
}

// --- Search ---

func TestAX7_Search_Good(t *T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "alpha.jsonl", toolUseEntry(ts(0), "Bash", "t1", map[string]any{"command": "go test"}), toolResultEntry(ts(1), "t1", "PASS", false))
	results, err := Search(dir, "go test")

	RequireNoError(t, err)
	AssertLen(t, results, 1)
	AssertEqual(t, "alpha", results[0].SessionID)
}

func TestAX7_Search_Bad(t *T) {
	dir := t.TempDir()
	results, err := Search(dir, "missing")

	RequireNoError(t, err)
	AssertEmpty(t, results)
	AssertLen(t, results, 0)
}

func TestAX7_Search_Ugly(t *T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "alpha.jsonl", toolUseEntry(ts(0), "Bash", "t1", map[string]any{"command": ""}), toolResultEntry(ts(1), "t1", "needle in output", false))
	results, err := Search(dir, "NEEDLE")

	RequireNoError(t, err)
	AssertLen(t, results, 1)
	AssertContains(t, results[0].Match, "needle")
}

// --- SearchSeq ---

func TestAX7_SearchSeq_Good(t *T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "alpha.jsonl", toolUseEntry(ts(0), "Bash", "t1", map[string]any{"command": "go vet"}), toolResultEntry(ts(1), "t1", "ok", false))
	var results []SearchResult
	for result := range SearchSeq(dir, "go vet") {
		results = append(results, result)
	}

	AssertLen(t, results, 1)
	AssertEqual(t, "Bash", results[0].Tool)
}

func TestAX7_SearchSeq_Bad(t *T) {
	var results []SearchResult
	for result := range SearchSeq(t.TempDir(), "absent") {
		results = append(results, result)
	}

	AssertEmpty(t, results)
	AssertLen(t, results, 0)
}

func TestAX7_SearchSeq_Ugly(t *T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "alpha.jsonl", toolUseEntry(ts(0), "Bash", "t1", map[string]any{"command": "go test"}), toolResultEntry(ts(1), "t1", "ok", false))
	var first SearchResult
	for result := range SearchSeq(dir, "") {
		first = result
		break
	}

	AssertEqual(t, "alpha", first.SessionID)
	AssertEqual(t, "Bash", first.Tool)
}

// --- RenderHTML ---

func TestAX7_RenderHTML_Good(t *T) {
	dir := t.TempDir()
	outputPath := Path(dir, "session.html")
	sess := &Session{ID: "alpha-session", StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 2, 20, 10, 0, 1, 0, time.UTC), Events: []Event{{Type: "tool_use", Tool: "Bash", Input: "echo ok", Output: "ok", Success: true}}}
	err := RenderHTML(sess, outputPath)

	RequireNoError(t, err)
	readResult := hostFS.Read(outputPath)
	AssertTrue(t, readResult.OK)
	AssertContains(t, readResult.Value.(string), "alpha-s")
}

func TestAX7_RenderHTML_Bad(t *T) {
	sess := &Session{ID: "bad"}
	err := RenderHTML(sess, Path(t.TempDir(), "missing", "session.html"))

	AssertError(t, err)
	AssertContains(t, err.Error(), "parent directory")
}

func TestAX7_RenderHTML_Ugly(t *T) {
	dir := t.TempDir()
	outputPath := Path(dir, "escaped.html")
	sess := &Session{ID: "ugly", Events: []Event{{Type: "tool_use", Tool: "Bash", Input: `<script>bad()</script>`, Output: `<script>bad()</script>`, Success: true}}}
	err := RenderHTML(sess, outputPath)

	RequireNoError(t, err)
	html := hostFS.Read(outputPath).Value.(string)
	AssertNotContains(t, html, "<script>bad")
	AssertContains(t, html, "&lt;script&gt;")
}

// --- RenderMP4 ---

func TestAX7_RenderMP4_Good(t *T) {
	dir := t.TempDir()
	vhsPath := Path(dir, "vhs")
	writeResult := hostFS.WriteMode(vhsPath, "#!/bin/sh\nexit 0\n", 0o755)
	RequireTrue(t, writeResult.OK)
	previousCore := hostCore
	c := New()
	c.Action("process.run", func(_ Context, opts Options) Result {
		AssertEqual(t, vhsPath, opts.String("command"))
		return Ok("rendered")
	})
	hostCore = c
	defer func() { hostCore = previousCore }()
	t.Setenv("PATH", dir)
	err := RenderMP4(&Session{ID: "alpha", StartTime: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)}, Path(dir, "out.mp4"))

	AssertNoError(t, err)
	AssertTrue(t, hostFS.Stat(vhsPath).OK)
}

func TestAX7_RenderMP4_Bad(t *T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	err := RenderMP4(&Session{ID: "missing-vhs"}, Path(dir, "out.mp4"))

	AssertError(t, err)
	AssertContains(t, err.Error(), "vhs not installed")
}

func TestAX7_RenderMP4_Ugly(t *T) {
	dir := t.TempDir()
	vhsPath := Path(dir, "vhs")
	writeResult := hostFS.WriteMode(vhsPath, "#!/bin/sh\nexit 7\n", 0o755)
	RequireTrue(t, writeResult.OK)
	previousCore := hostCore
	c := New()
	c.Action("process.run", func(_ Context, _ Options) Result {
		return Fail(NewError("render failed"))
	})
	hostCore = c
	defer func() { hostCore = previousCore }()
	t.Setenv("PATH", dir)
	err := RenderMP4(&Session{ID: "failing-vhs"}, Path(dir, "out.mp4"))

	AssertError(t, err)
	AssertContains(t, err.Error(), "vhs render")
}

// --- rawJSON JSON methods ---

func TestAX7_JSON_UnmarshalJSON_Good(t *T) {
	var raw rawJSON
	err := raw.UnmarshalJSON([]byte(`{"agent":"codex"}`))

	RequireNoError(t, err)
	AssertEqual(t, `{"agent":"codex"}`, string(raw))
}

func TestAX7_JSON_UnmarshalJSON_Bad(t *T) {
	var raw *rawJSON
	err := raw.UnmarshalJSON([]byte(`{}`))

	AssertError(t, err)
	AssertContains(t, err.Error(), "nil receiver")
}

func TestAX7_JSON_UnmarshalJSON_Ugly(t *T) {
	raw := rawJSON(`previous-value`)
	err := raw.UnmarshalJSON([]byte(`[1]`))

	RequireNoError(t, err)
	AssertEqual(t, `[1]`, string(raw))
	AssertLen(t, raw, 3)
}

func TestAX7_JSON_MarshalJSON_Good(t *T) {
	raw := rawJSON(`{"agent":"codex"}`)
	got, err := raw.MarshalJSON()

	RequireNoError(t, err)
	AssertEqual(t, `{"agent":"codex"}`, string(got))
}

func TestAX7_JSON_MarshalJSON_Bad(t *T) {
	var raw rawJSON
	got, err := raw.MarshalJSON()

	RequireNoError(t, err)
	AssertEqual(t, "null", string(got))
}

func TestAX7_JSON_MarshalJSON_Ugly(t *T) {
	raw := rawJSON{}
	got, err := raw.MarshalJSON()

	RequireNoError(t, err)
	AssertEqual(t, "", string(got))
	AssertLen(t, got, 0)
}
