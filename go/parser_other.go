//go:build !unix

// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"io" // Note: intrinsic — keeps the platform stub signature aligned with the Unix io.ReadCloser implementation; no core equivalent

	coreerr "dappco.re/go"
)

// openTranscriptNoFollow reports that secure no-follow opens are unavailable on this platform.
func openTranscriptNoFollow(filePath string) coreerr.Result {
	return coreerr.Fail(coreerr.E("openTranscriptNoFollow", "secure no-follow transcript opens are unsupported on this platform: "+filePath, nil))
}

// isTranscriptMissing reports whether err wraps a missing transcript path error.
func isTranscriptMissing(error) bool {
	return false
}
