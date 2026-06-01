//go:build unix

// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"io"
	"os"
	"syscall"
	"testing"

	core "dappco.re/go"
)

// --- openTranscriptNoFollow + nofollowfile.Read/Close ---

func TestParserUnix_openTranscriptNoFollow_Good(t *testing.T) {
	dir := t.TempDir()
	path := core.PathJoin(dir, "ok.jsonl")
	core.RequireTrue(t, hostFS.Write(path, "hello world\n").OK)

	result := openTranscriptNoFollow(path)

	core.RequireTrue(t, result.OK, result.Error())
	rc := result.Value.(io.ReadCloser)
	defer rc.Close()

	buf := make([]byte, 32)
	n, err := rc.Read(buf)
	core.RequireNoError(t, err)
	core.AssertEqual(t, "hello world\n", string(buf[:n]))
}

func TestParserUnix_openTranscriptNoFollow_Bad(t *testing.T) {
	// A symlink must be rejected by O_NOFOLLOW rather than followed.
	dir := t.TempDir()
	target := core.PathJoin(dir, "real.jsonl")
	core.RequireTrue(t, hostFS.Write(target, "secret\n").OK)
	link := core.PathJoin(dir, "link.jsonl")
	core.RequireNoError(t, os.Symlink(target, link))

	result := openTranscriptNoFollow(link)

	core.AssertFalse(t, result.OK)
	core.AssertContains(t, result.Error(), "open transcript without following symlinks")
}

func TestParserUnix_openTranscriptNoFollow_Ugly(t *testing.T) {
	// A directory opens but fails the regular-file check, exercising the
	// closeNoFollowFD cleanup path.
	dir := t.TempDir()

	result := openTranscriptNoFollow(dir)

	core.AssertFalse(t, result.OK)
	core.AssertContains(t, result.Error(), "not a regular file")
}

func TestParserUnix_nofollowfile_Read_Good(t *testing.T) {
	dir := t.TempDir()
	path := core.PathJoin(dir, "r.jsonl")
	core.RequireTrue(t, hostFS.Write(path, "abc").OK)
	rc := openTranscriptNoFollow(path).Value.(io.ReadCloser)
	defer rc.Close()

	buf := make([]byte, 8)
	n, err := rc.Read(buf)

	core.RequireNoError(t, err)
	core.AssertEqual(t, "abc", string(buf[:n]))
}

func TestParserUnix_nofollowfile_Read_Bad(t *testing.T) {
	// Reading past the end yields io.EOF.
	dir := t.TempDir()
	path := core.PathJoin(dir, "e.jsonl")
	core.RequireTrue(t, hostFS.Write(path, "").OK)
	rc := openTranscriptNoFollow(path).Value.(io.ReadCloser)
	defer rc.Close()

	_, err := rc.Read(make([]byte, 4))

	core.AssertErrorIs(t, err, io.EOF)
}

func TestParserUnix_nofollowfile_Close_Good(t *testing.T) {
	dir := t.TempDir()
	path := core.PathJoin(dir, "c.jsonl")
	core.RequireTrue(t, hostFS.Write(path, "x").OK)
	rc := openTranscriptNoFollow(path).Value.(io.ReadCloser)

	core.AssertNoError(t, rc.Close())
}

func TestParserUnix_nofollowfile_Close_Bad(t *testing.T) {
	// Closing twice surfaces the descriptor error from the second call.
	dir := t.TempDir()
	path := core.PathJoin(dir, "c2.jsonl")
	core.RequireTrue(t, hostFS.Write(path, "x").OK)
	f := openTranscriptNoFollow(path).Value.(*nofollowfile)

	core.RequireNoError(t, f.Close())
	err := f.Close()

	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "close transcript descriptor")
}

func TestParserUnix_nofollowfile_Read_Ugly(t *testing.T) {
	// Reading from a closed descriptor surfaces the wrapped read error.
	dir := t.TempDir()
	path := core.PathJoin(dir, "u.jsonl")
	core.RequireTrue(t, hostFS.Write(path, "data").OK)
	f := openTranscriptNoFollow(path).Value.(*nofollowfile)
	core.RequireNoError(t, f.Close())

	_, err := f.Read(make([]byte, 4))

	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "read transcript descriptor")
}

// --- isTranscriptMissing ---

func TestParserUnix_isTranscriptMissing_Good(t *testing.T) {
	err := core.E("op", "open", syscall.ENOENT)

	core.AssertTrue(t, isTranscriptMissing(err))
}

func TestParserUnix_isTranscriptMissing_Bad(t *testing.T) {
	core.AssertFalse(t, isTranscriptMissing(nil))
	core.AssertFalse(t, isTranscriptMissing(core.E("op", "other", syscall.EACCES)))
}

func TestParserUnix_isTranscriptMissing_Ugly(t *testing.T) {
	// A non-wrapping error that is not ENOENT stops the unwrap walk.
	core.AssertFalse(t, isTranscriptMissing(syscall.EPERM))
	// ENOENT presented directly (no wrapping) is still detected.
	core.AssertTrue(t, isTranscriptMissing(syscall.ENOENT))
}
