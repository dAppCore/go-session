// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkParseTranscript benchmarks parsing a ~1MB+ JSONL file.
func BenchmarkParseTranscript(b *testing.B) {
	dir := b.TempDir()
	path := generateBenchJSONL(b, dir, 5000) // ~1MB+ of JSONL

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		sess, _, err := ParseTranscript(path)
		if err != nil {
			b.Fatal(err)
		}
		if len(sess.Events) == 0 {
			b.Fatal("expected events")
		}
	}
}

// BenchmarkParseTranscript_Large benchmarks a larger ~5MB file.
func BenchmarkParseTranscript_Large(b *testing.B) {
	dir := b.TempDir()
	path := generateBenchJSONL(b, dir, 25000) // ~5MB

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _, err := ParseTranscript(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkListSessions benchmarks listing sessions in a directory.
func BenchmarkListSessions(b *testing.B) {
	dir := b.TempDir()

	// Create 20 session files
	for range 20 {
		generateBenchJSONL(b, dir, 100)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		sessions, err := ListSessions(dir)
		if err != nil {
			b.Fatal(err)
		}
		if len(sessions) == 0 {
			b.Fatal("expected sessions")
		}
	}
}

// BenchmarkSearch benchmarks searching across multiple sessions.
func BenchmarkSearch(b *testing.B) {
	dir := b.TempDir()

	// Create 10 session files with varied content
	for range 10 {
		generateBenchJSONL(b, dir, 500)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := Search(dir, "echo")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// generateBenchJSONL creates a synthetic JSONL file with the given number of tool pairs.
// Returns the file path.
func generateBenchJSONL(b testing.TB, dir string, numTools int) string {
	b.Helper()

	var sb strings.Builder
	baseTS := "2026-02-20T10:00:00Z"

	// Opening user message
	sb.WriteString(fmt.Sprintf(`{"type":"user","timestamp":"%s","sessionId":"bench","message":{"role":"user","content":[{"type":"text","text":"Start benchmark session"}]}}`, baseTS))
	sb.WriteByte('\n')

	for i := range numTools {
		toolID := fmt.Sprintf("tool-%d", i)
		offset := i * 2

		// Alternate between different tool types for realistic distribution
		var toolUse, toolResult string
		switch i % 5 {
		case 0: // Bash
			toolUse = fmt.Sprintf(`{"type":"assistant","timestamp":"2026-02-20T10:%02d:%02dZ","sessionId":"bench","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","id":"%s","input":{"command":"echo iteration %d","description":"echo test"}}]}}`,
				offset/60, offset%60, toolID, i)
			toolResult = fmt.Sprintf(`{"type":"user","timestamp":"2026-02-20T10:%02d:%02dZ","sessionId":"bench","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"%s","content":"iteration %d output line one\niteration %d output line two","is_error":false}]}}`,
				(offset+1)/60, (offset+1)%60, toolID, i, i)
		case 1: // Read
			toolUse = fmt.Sprintf(`{"type":"assistant","timestamp":"2026-02-20T10:%02d:%02dZ","sessionId":"bench","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","id":"%s","input":{"file_path":"/tmp/bench/file-%d.go"}}]}}`,
				offset/60, offset%60, toolID, i)
			toolResult = fmt.Sprintf(`{"type":"user","timestamp":"2026-02-20T10:%02d:%02dZ","sessionId":"bench","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"%s","content":"package main\n\nfunc main() {\n\tfmt.Println(%d)\n}","is_error":false}]}}`,
				(offset+1)/60, (offset+1)%60, toolID, i)
		case 2: // Edit
			toolUse = fmt.Sprintf(`{"type":"assistant","timestamp":"2026-02-20T10:%02d:%02dZ","sessionId":"bench","message":{"role":"assistant","content":[{"type":"tool_use","name":"Edit","id":"%s","input":{"file_path":"/tmp/bench/file-%d.go","old_string":"old","new_string":"new"}}]}}`,
				offset/60, offset%60, toolID, i)
			toolResult = fmt.Sprintf(`{"type":"user","timestamp":"2026-02-20T10:%02d:%02dZ","sessionId":"bench","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"%s","content":"ok","is_error":false}]}}`,
				(offset+1)/60, (offset+1)%60, toolID)
		case 3: // Grep
			toolUse = fmt.Sprintf(`{"type":"assistant","timestamp":"2026-02-20T10:%02d:%02dZ","sessionId":"bench","message":{"role":"assistant","content":[{"type":"tool_use","name":"Grep","id":"%s","input":{"pattern":"TODO","path":"/tmp/bench"}}]}}`,
				offset/60, offset%60, toolID)
			toolResult = fmt.Sprintf(`{"type":"user","timestamp":"2026-02-20T10:%02d:%02dZ","sessionId":"bench","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"%s","content":"/tmp/bench/file.go:10: // TODO fix this","is_error":false}]}}`,
				(offset+1)/60, (offset+1)%60, toolID)
		case 4: // Glob
			toolUse = fmt.Sprintf(`{"type":"assistant","timestamp":"2026-02-20T10:%02d:%02dZ","sessionId":"bench","message":{"role":"assistant","content":[{"type":"tool_use","name":"Glob","id":"%s","input":{"pattern":"**/*.go"}}]}}`,
				offset/60, offset%60, toolID)
			toolResult = fmt.Sprintf(`{"type":"user","timestamp":"2026-02-20T10:%02d:%02dZ","sessionId":"bench","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"%s","content":"/tmp/a.go\n/tmp/b.go\n/tmp/c.go","is_error":false}]}}`,
				(offset+1)/60, (offset+1)%60, toolID)
		}

		sb.WriteString(toolUse)
		sb.WriteByte('\n')
		sb.WriteString(toolResult)
		sb.WriteByte('\n')
	}

	// Closing assistant message
	sb.WriteString(fmt.Sprintf(`{"type":"assistant","timestamp":"2026-02-20T12:00:00Z","sessionId":"bench","message":{"role":"assistant","content":[{"type":"text","text":"Benchmark session complete."}]}}%s`, "\n"))

	name := fmt.Sprintf("bench-%d.jsonl", numTools)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		b.Fatal(err)
	}

	info, _ := os.Stat(path)
	b.Logf("Generated %s: %d bytes, %d tool pairs", name, info.Size(), numTools)

	return path
}
