# TODO.md — go-session

Dispatched from core/go orchestration. Pick up tasks in order.

---

## Phase 0: Hardening & Test Coverage

- [x] **Add parser tests** — Test `ParseTranscript()` with: minimal valid JSONL (1 user + 1 assistant message), tool call events (Bash, Read, Edit, Write, Grep, Glob, Task), truncated JSONL (incomplete last line), empty file, malformed JSON lines (should skip gracefully), very large session (1000+ events), nested tool results with arrays and maps. Also: HTML renderer tests, video/tape generator tests, search tests. 51 tests total, 90.9% coverage.
- [x] **Add ListSessions tests** — Test with: empty directory, single session, multiple sessions sorted by date, non-.jsonl files ignored, malformed content fallback.
- [x] **Tool extraction coverage** — Test `extractToolInput()` for all 7 supported tool types plus unknown tool fallback and nil input. Test `extractResultContent()` for string, array, map, and nil content.
- [x] **Benchmark parsing** — `BenchmarkParseTranscript` with 4000-line JSONL (2000 assistant + 2000 user entries).
- [x] **`go vet ./...` clean** — No warnings.

## Phase 1: Parser Robustness

- [ ] Handle truncated JSONL (incomplete final line, missing closing brace)
- [ ] Handle very large sessions (streaming parse, avoid loading entire file into memory)
- [ ] Handle non-standard tool formats (custom MCP tools, unknown tool names)
- [ ] Add graceful error recovery — skip malformed lines, log warnings

## Phase 2: Analytics

- [ ] Session duration stats (start time, end time, wall clock, active time)
- [ ] Tool usage frequency (count per tool type, percentage breakdown)
- [ ] Error rate tracking (failed tool calls, retries, panics)
- [ ] Token usage estimation from assistant message lengths

## Phase 3: Timeline UI

- [ ] Feed parsed events into go-html for visual session timeline
- [ ] Colour-code events by type (tool call, assistant message, user message)
- [ ] Add collapsible detail panels for long tool outputs
- [ ] Export timeline as standalone HTML file

---

## Workflow

1. Virgil in core/go writes tasks here after research
2. This repo's dedicated session picks up tasks in phase order
3. Mark `[x]` when done, note commit hash
