//go:build unix

// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"io"      // Note: intrinsic — io.ReadCloser contract and EOF signalling for descriptor-backed transcript reads; no core equivalent
	"syscall" // Note: intrinsic — O_NOFOLLOW descriptor opens and fstat checks are platform syscalls; no core equivalent

	coreerr "dappco.re/go/core/log"
)

type noFollowFile struct {
	fd int
}

// Read reads bytes from a descriptor opened without following symlinks.
func (f *noFollowFile) Read(p []byte) (int, error) {
	n, err := syscall.Read(f.fd, p)
	if err != nil {
		return n, coreerr.E("noFollowFile.Read", "read transcript descriptor", err)
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

// Close closes a descriptor opened without following symlinks.
func (f *noFollowFile) Close() error {
	if err := syscall.Close(f.fd); err != nil {
		return coreerr.E("noFollowFile.Close", "close transcript descriptor", err)
	}
	return nil
}

// openTranscriptNoFollow opens a regular transcript file without following symlinks.
func openTranscriptNoFollow(filePath string) (io.ReadCloser, error) {
	const op = "openTranscriptNoFollow"

	fd, err := syscall.Open(filePath, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, coreerr.E(op, "open transcript without following symlinks", err)
	}

	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		if closeErr := closeNoFollowFD(fd); closeErr != nil {
			return nil, closeErr
		}
		return nil, coreerr.E(op, "stat transcript descriptor", err)
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFREG {
		if closeErr := closeNoFollowFD(fd); closeErr != nil {
			return nil, closeErr
		}
		return nil, coreerr.E(op, "not a regular file", nil)
	}
	return &noFollowFile{fd: fd}, nil
}

// closeNoFollowFD closes a raw descriptor after a failed secure-open check.
func closeNoFollowFD(fd int) error {
	if err := syscall.Close(fd); err != nil {
		return coreerr.E("openTranscriptNoFollow", "close rejected transcript descriptor", err)
	}
	return nil
}

// isTranscriptMissing reports whether err wraps a missing transcript path error.
func isTranscriptMissing(err error) bool {
	for err != nil {
		if err == syscall.ENOENT {
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}
