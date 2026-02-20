# FINDINGS.md -- go-session

## 2026-02-19: Split from core/go (Virgil)

### Origin

Extracted from `forge.lthn.ai/core/go` `pkg/session/` on 19 Feb 2026.

### Architecture

- Parses Claude Code JSONL transcripts into an `Event` array
- Each event has type, timestamp, content, and optional tool metadata
- Supported tool types: Bash, Read, Edit, Write, Grep, Glob, Task

### Dependencies

- Zero external dependencies at runtime -- standard library only (`encoding/json`, `bufio`, `os`)
- Test dependency: `github.com/stretchr/testify` (assert/require)

### Tests

- Test coverage for JSONL parsing and event type detection

## 2026-02-20: Phase 0 Hardening (Charon)

### Test Coverage

- 51 tests across 4 test files, 90.9% statement coverage
- `parser_test.go` — 13 tests + 12 extractToolInput subtests + 5 extractResultContent subtests + 5 truncate subtests + 5 ListSessions tests + benchmark
- `html_test.go` — 7 RenderHTML tests + 4 shortID subtests + 6 formatDuration subtests
- `search_test.go` — 9 Search tests covering cross-session matching, case insensitivity, empty dirs, malformed sessions
- `video_test.go` — 8 generateTape tests + 1 RenderMP4 error test + 5 extractCommand subtests

### Observations

- `extractCommand()` naively splits on first ` # ` — commands containing literal ` # ` (e.g. inside quotes) get truncated. Documented in test, not a bug per se since the parser always constructs `command + " # " + description`.
- `RenderMP4()` is untestable without the external `vhs` binary. Tests cover `generateTape()` (the pure logic) and verify `RenderMP4` returns a clear error when vhs is absent.
- `extractResultContent()` handles string, `[]interface{}`, and `map[string]interface{}` content types. All three paths plus nil are tested.
- `ListSessions` falls back to file mod time when no valid timestamps are found in a JSONL file.
- `go vet ./...` was clean from the start — no fixes needed.

## 2026-02-20: Phase 1 Streaming & Edge Cases

### New API surface

- `ParseTranscriptReader(r io.Reader, id string) (*Session, *ParseStats, error)` — streaming parse from any `io.Reader` (pipes, network, in-memory buffers). No file on disc required.
- `parseFromReader(r io.Reader, id string)` — internal shared implementation, eliminating code duplication between `ParseTranscript` and `ParseTranscriptReader`.
- `maxScannerBuffer` constant: 8 MiB (bumped from 4 MiB), handles very large tool outputs.

### Tests added (18 new, 104 total)

- **Streaming io.Reader (6 tests):** strings.Reader, bytes.Buffer, empty reader, large lines (128KB), malformed lines with stats, orphaned tool calls.
- **Custom MCP tools (3 tests):** `mcp__forge__create_issue` with flat input, nested JSON input, empty input object.
- **Edge case recovery (5 tests):** binary garbage, null bytes, 5MB lines, malformed message JSON, malformed content blocks.
- **Additional truncated JSONL (3 tests):** missing closing brace, mid-key truncation, all-bad-lines file.
- **ListSessions truncated files (1 test):** partially truncated file still extracts valid timestamps.

### Coverage

- 93.0% statement coverage (up from 90.9%).
- `ParseTranscript`, `ParseTranscriptReader`, `extractToolInput`, `extractResultContent`, `truncate` all at 100%.
