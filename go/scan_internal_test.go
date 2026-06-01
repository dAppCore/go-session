// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"

	core "dappco.re/go"
)

// collectLines runs scanTranscriptLines over s and returns the lines handed
// to the callback plus the scan result.
func collectLines(s string, maxLineSize int) ([]string, core.Result) {
	var lines []string
	result := scanTranscriptLines(core.NewReader(s), maxLineSize, func(line []byte) bool {
		lines = append(lines, string(line))
		return true
	})
	return lines, result
}

func TestScanInternal_scanTranscriptLines_Good(t *testing.T) {
	lines, result := collectLines("alpha\nbeta\ngamma", 1024)

	core.RequireTrue(t, result.OK, result.Error())
	core.AssertLen(t, lines, 3)
	core.AssertEqual(t, "gamma", lines[2])
}

func TestScanInternal_scanTranscriptLines_Bad(t *testing.T) {
	// A line longer than the limit fails before the newline is reached.
	_, result := collectLines(repeatString("x", 50)+"\n", 8)

	core.AssertFalse(t, result.OK)
	core.AssertContains(t, result.Error(), "line exceeds 8 bytes")
}

func TestScanInternal_scanTranscriptLines_Ugly(t *testing.T) {
	// A trailing line with no newline that overflows the limit fails on the
	// end-of-chunk accumulation path.
	_, result := collectLines(repeatString("y", 50), 8)

	core.AssertFalse(t, result.OK)
	core.AssertContains(t, result.Error(), "line exceeds 8 bytes")
}

func TestScanInternal_scanTranscriptLines_DefaultLimit(t *testing.T) {
	// A non-positive maxLineSize falls back to the 8 MiB default.
	lines, result := collectLines("one\ntwo", 0)

	core.RequireTrue(t, result.OK, result.Error())
	core.AssertLen(t, lines, 2)
}

func TestScanInternal_scanTranscriptLines_EarlyStop(t *testing.T) {
	// The handler returning false stops the scan mid-stream.
	count := 0
	result := scanTranscriptLines(core.NewReader("a\nb\nc\n"), 1024, func([]byte) bool {
		count++
		return false
	})

	core.RequireTrue(t, result.OK, result.Error())
	core.AssertEqual(t, 1, count)
}

func TestScanInternal_scanTranscriptLines_EarlyStopFinalLine(t *testing.T) {
	// Early stop on the trailing newline-less line still returns ok.
	result := scanTranscriptLines(core.NewReader("only-line"), 1024, func([]byte) bool {
		return false
	})

	core.RequireTrue(t, result.OK, result.Error())
}
