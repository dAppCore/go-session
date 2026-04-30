//go:build unix

// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"io"
	"syscall"

	. "dappco.re/go"
)

// --- noFollowFile.Read ---

func TestAX7_FollowFile_Read_Good(t *T) {
	filePath := Path(t.TempDir(), "reader.txt")
	writeResult := hostFS.WriteMode(filePath, "abc", 0o600)
	RequireTrue(t, writeResult.OK)
	fd, err := syscall.Open(filePath, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	RequireNoError(t, err)
	f := &noFollowFile{fd: fd}
	buf := make([]byte, 3)
	n, err := f.Read(buf)
	closeErr := f.Close()

	RequireNoError(t, closeErr)
	AssertNoError(t, err)
	AssertEqual(t, 3, n)
	AssertEqual(t, "abc", string(buf))
}

func TestAX7_FollowFile_Read_Bad(t *T) {
	f := &noFollowFile{fd: -1}
	buf := make([]byte, 1)
	n, err := f.Read(buf)

	AssertError(t, err)
	AssertEqual(t, -1, n)
	AssertEqual(t, byte(0), buf[0])
}

func TestAX7_FollowFile_Read_Ugly(t *T) {
	filePath := Path(t.TempDir(), "empty.txt")
	writeResult := hostFS.WriteMode(filePath, "", 0o600)
	RequireTrue(t, writeResult.OK)
	fd, err := syscall.Open(filePath, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	RequireNoError(t, err)
	f := &noFollowFile{fd: fd}
	buf := make([]byte, 1)
	n, err := f.Read(buf)
	closeErr := f.Close()

	RequireNoError(t, closeErr)
	AssertErrorIs(t, err, io.EOF)
	AssertEqual(t, 0, n)
}

// --- noFollowFile.Close ---

func TestAX7_FollowFile_Close_Good(t *T) {
	filePath := Path(t.TempDir(), "closer.txt")
	writeResult := hostFS.WriteMode(filePath, "abc", 0o600)
	RequireTrue(t, writeResult.OK)
	fd, err := syscall.Open(filePath, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	RequireNoError(t, err)
	f := &noFollowFile{fd: fd}
	err = f.Close()

	AssertNoError(t, err)
	AssertEqual(t, fd, f.fd)
}

func TestAX7_FollowFile_Close_Bad(t *T) {
	f := &noFollowFile{fd: -1}
	err := f.Close()

	AssertError(t, err)
	AssertContains(t, err.Error(), "close transcript descriptor")
}

func TestAX7_FollowFile_Close_Ugly(t *T) {
	filePath := Path(t.TempDir(), "twice.txt")
	writeResult := hostFS.WriteMode(filePath, "abc", 0o600)
	RequireTrue(t, writeResult.OK)
	fd, err := syscall.Open(filePath, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	RequireNoError(t, err)
	f := &noFollowFile{fd: fd}
	firstErr := f.Close()
	secondErr := f.Close()

	AssertNoError(t, firstErr)
	AssertError(t, secondErr)
	AssertContains(t, secondErr.Error(), "close transcript descriptor")
}
