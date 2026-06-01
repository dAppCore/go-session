// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"os"
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

// --- generateTape branch coverage ---

func TestVideo_generateTape_Good(t *testing.T) {
	sess := &Session{ID: "long-session-id-truncated", StartTime: time.Unix(0, 0), Events: []Event{
		{Type: "tool_use", Tool: "Read", Input: "/etc/hosts"},
		{Type: "tool_use", Tool: "Edit", Input: "/tmp/x.go"},
		{Type: "tool_use", Tool: "Write", Input: "/tmp/y.go"},
		{Type: "tool_use", Tool: "Task", Input: "investigate the failing build"},
	}}

	tape := generateTape(sess, "/tmp/out.mp4")

	core.AssertContains(t, tape, "# Read: /etc/hosts")
	core.AssertContains(t, tape, "# Edit: /tmp/x.go")
	core.AssertContains(t, tape, "# Write: /tmp/y.go")
	core.AssertContains(t, tape, "# Agent: investigate the failing build")
	// Session id is clipped to the first 8 runes in the title frame.
	core.AssertContains(t, tape, "# Session long-ses")
}

func TestVideo_generateTape_Bad(t *testing.T) {
	// Non-tool_use events and an empty-command Bash event produce no body.
	sess := &Session{ID: "x", Events: []Event{
		{Type: "assistant", Input: "thinking out loud"},
		{Type: "tool_use", Tool: "Bash", Input: "", Success: true},
	}}

	tape := generateTape(sess, "/tmp/out.mp4")

	core.AssertNotContains(t, tape, "thinking out loud")
	core.AssertNotContains(t, tape, "# ✓ OK")
	core.AssertContains(t, tape, "Sleep 3s")
}

func TestVideo_generateTape_Ugly(t *testing.T) {
	// A failed Bash event with >200 bytes of output: output is truncated and
	// the failure marker is emitted.
	longOutput := repeatString("z", 250)
	sess := &Session{ID: "x", Events: []Event{
		{Type: "tool_use", Tool: "Bash", Input: "do thing", Output: longOutput, Success: false},
	}}

	tape := generateTape(sess, "/tmp/out.mp4")

	core.AssertContains(t, tape, "# ✗ FAILED")
	core.AssertContains(t, tape, "...")
	core.AssertNotContains(t, tape, repeatString("z", 250))
}

// --- extractCommand ---

func TestVideo_extractCommand_Good(t *testing.T) {
	core.AssertEqual(t, "echo hi", extractCommand("echo hi # greet the user"))
}

func TestVideo_extractCommand_Bad(t *testing.T) {
	core.AssertEqual(t, "echo hi", extractCommand("echo hi"))
}

func TestVideo_extractCommand_Ugly(t *testing.T) {
	// A leading " # " (idx 0) is not treated as a description separator.
	core.AssertEqual(t, " # hash-first", extractCommand(" # hash-first"))
}

// --- lookupExecutable / isExecutablePath ---

func TestVideo_lookupExecutable_Good(t *testing.T) {
	dir := t.TempDir()
	exe := core.PathJoin(dir, "tool")
	core.RequireTrue(t, hostFS.Write(exe, "#!/bin/sh\n").OK)
	core.RequireNoError(t, os.Chmod(exe, 0o755))
	t.Setenv("PATH", dir)

	core.AssertEqual(t, exe, lookupExecutable("tool"))
}

func TestVideo_lookupExecutable_Bad(t *testing.T) {
	core.AssertEqual(t, "", lookupExecutable(""))
	core.AssertEqual(t, "", lookupExecutable("definitely-not-on-path-xyzzy"))
}

func TestVideo_lookupExecutable_Ugly(t *testing.T) {
	// An explicit path (contains a slash) bypasses PATH; a non-executable
	// direct path resolves to empty.
	dir := t.TempDir()
	plain := core.PathJoin(dir, "plain.txt")
	core.RequireTrue(t, hostFS.Write(plain, "data").OK)

	core.AssertEqual(t, "", lookupExecutable(plain))

	exe := core.PathJoin(dir, "direct")
	core.RequireTrue(t, hostFS.Write(exe, "#!/bin/sh\n").OK)
	core.RequireNoError(t, os.Chmod(exe, 0o755))
	core.AssertEqual(t, exe, lookupExecutable(exe))
}

func TestVideo_isExecutablePath_Good(t *testing.T) {
	dir := t.TempDir()
	exe := core.PathJoin(dir, "run")
	core.RequireTrue(t, hostFS.Write(exe, "#!/bin/sh\n").OK)
	core.RequireNoError(t, os.Chmod(exe, 0o755))

	core.AssertTrue(t, isExecutablePath(exe))
}

func TestVideo_isExecutablePath_Bad(t *testing.T) {
	core.AssertFalse(t, isExecutablePath(core.PathJoin(t.TempDir(), "missing")))
}

func TestVideo_isExecutablePath_Ugly(t *testing.T) {
	// A directory and a non-executable regular file are both rejected.
	dir := t.TempDir()
	core.AssertFalse(t, isExecutablePath(dir))

	plain := core.PathJoin(dir, "data.txt")
	core.RequireTrue(t, hostFS.Write(plain, "x").OK)
	core.RequireNoError(t, os.Chmod(plain, 0o644))
	core.AssertFalse(t, isExecutablePath(plain))
}
