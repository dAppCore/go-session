// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"
	"time"

	core "dappco.re/go"
)

// --- formatDuration ---

func TestHtmlInternal_formatDuration_Good(t *testing.T) {
	core.AssertEqual(t, "1.5s", formatDuration(1500*time.Millisecond))
}

func TestHtmlInternal_formatDuration_Bad(t *testing.T) {
	core.AssertEqual(t, "250ms", formatDuration(250*time.Millisecond))
}

func TestHtmlInternal_formatDuration_Ugly(t *testing.T) {
	core.AssertEqual(t, "2m3s", formatDuration(2*time.Minute+3*time.Second))
	core.AssertEqual(t, "1h5m", formatDuration(time.Hour+5*time.Minute))
}

// --- shortID ---

func TestHtmlInternal_shortID_Good(t *testing.T) {
	core.AssertEqual(t, "abcdefgh", shortID("abcdefghijklmnop"))
}

func TestHtmlInternal_shortID_Bad(t *testing.T) {
	core.AssertEqual(t, "short", shortID("short"))
}

func TestHtmlInternal_shortID_Ugly(t *testing.T) {
	core.AssertEqual(t, "", shortID(""))
	core.AssertEqual(t, "12345678", shortID("12345678"))
}
