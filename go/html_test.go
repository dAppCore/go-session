// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"
	"time"

	core "dappco.re/go"
)

func TestHtml_RenderHTML_Good(t *testing.T) {
	out := core.PathJoin(t.TempDir(), "session.html")
	sess := &Session{ID: "abcdefghi", StartTime: time.Unix(0, 0), EndTime: time.Unix(60, 0), Events: []Event{{Type: "tool_use", Tool: "Bash", Input: "go test", Output: "PASS", Success: true}}}

	result := RenderHTML(sess, out)

	core.RequireTrue(t, result.OK)
	readResult := hostFS.Read(out)
	core.RequireTrue(t, readResult.OK)
	html := readResult.Value.(string)
	core.AssertContains(t, html, "abcdefg")
	core.AssertContains(t, html, "go test")
}

func TestHtml_RenderHTML_Bad(t *testing.T) {
	result := RenderHTML(&Session{}, core.PathJoin(t.TempDir(), "missing", "session.html"))

	core.AssertFalse(t, result.OK)
	core.AssertContains(t, result.Error(), "parent directory does not exist")
}

func TestHtml_RenderHTML_Ugly(t *testing.T) {
	out := core.PathJoin(t.TempDir(), "empty.html")

	result := RenderHTML(&Session{ID: "empty"}, out)

	core.AssertTrue(t, result.OK)
	readResult := hostFS.Read(out)
	core.RequireTrue(t, readResult.OK)
	core.AssertContains(t, readResult.Value.(string), "0 tool calls")
}

// TestHtml_RenderHTML_MixedEvents exercises every event-type render branch:
// user, assistant, Read/Edit labels, a failed tool_use (error span, output
// err class) and the errors summary span.
func TestHtml_RenderHTML_MixedEvents(t *testing.T) {
	out := core.PathJoin(t.TempDir(), "mixed.html")
	sess := &Session{ID: "mixedabc", StartTime: time.Unix(0, 0), EndTime: time.Unix(120, 0), Events: []Event{
		{Type: "user", Input: "do a thing", Timestamp: time.Unix(1, 0)},
		{Type: "assistant", Input: "on it", Timestamp: time.Unix(2, 0)},
		{Type: "tool_use", Tool: "Read", Input: "/etc/hosts", Timestamp: time.Unix(3, 0), Success: true},
		{Type: "tool_use", Tool: "Edit", Input: "/tmp/x.go", Timestamp: time.Unix(4, 0), Success: true},
		{Type: "tool_use", Tool: "Bash", Input: "boom", Output: "failure detail", Duration: time.Second, Success: false},
	}}

	result := RenderHTML(sess, out)

	core.RequireTrue(t, result.OK, result.Error())
	html := hostFS.Read(out).Value.(string)
	core.AssertContains(t, html, "1 errors")
	core.AssertContains(t, html, "Claude")
	core.AssertContains(t, html, "User")
	core.AssertContains(t, html, "Target")
	core.AssertContains(t, html, "File")
	core.AssertContains(t, html, "output err")
}
