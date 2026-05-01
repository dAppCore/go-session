// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"
	"time"

	core "dappco.re/go"
)

func TestVideo_RenderMP4_Good(t *testing.T) {
	if lookupExecutable("vhs") == "" {
		t.Skip("RenderMP4 success branch requires vhs")
	}
	sess := &Session{ID: "video", StartTime: time.Unix(0, 0), Events: []Event{{Type: "tool_use", Tool: "Bash", Input: "echo ok", Output: "ok", Success: true}}}

	result := RenderMP4(sess, core.PathJoin(t.TempDir(), "session.mp4"))
	tape := generateTape(sess, "/tmp/session.mp4")

	core.AssertTrue(t, result.OK)
	core.AssertContains(t, tape, "Output /tmp/session.mp4")
	core.AssertContains(t, tape, "echo ok")
}

func TestVideo_RenderMP4_Bad(t *testing.T) {
	if lookupExecutable("vhs") != "" {
		t.Skip("RenderMP4 missing-vhs branch requires vhs absent")
	}
	sess := &Session{ID: "video"}

	result := RenderMP4(sess, "/tmp/session.mp4")

	core.AssertFalse(t, result.OK)
	core.AssertContains(t, result.Error(), "vhs not installed")
}

func TestVideo_RenderMP4_Ugly(t *testing.T) {
	sess := &Session{ID: "video", Events: []Event{{Type: "tool_use", Tool: "Bash", Input: "", Success: true}}}

	result := RenderMP4(sess, "/tmp/session.mp4")
	tape := generateTape(sess, "/tmp/session.mp4")

	if lookupExecutable("vhs") == "" {
		core.AssertFalse(t, result.OK)
	}
	core.AssertNotContains(t, tape, "\"$ \"")
	core.AssertContains(t, tape, "Sleep 3s")
}
