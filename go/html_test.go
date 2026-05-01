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
