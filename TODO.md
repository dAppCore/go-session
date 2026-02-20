# TODO.md — go-session

Dispatched from core/go orchestration. Pick up tasks in order.

---

## Phase 0: Hardening & Test Coverage

- [x] **Add parser tests** — 51 tests, 90.9% coverage. `7771e64`
- [x] **Add ListSessions tests** — 5 tests (empty, single, sorted, non-jsonl, malformed). `7771e64`
- [x] **Tool extraction coverage** — All 7 tool types + unknown fallback + nil. `7771e64`
- [x] **Benchmark parsing** — `BenchmarkParseTranscript` with 4000-line JSONL. `7771e64`
- [x] **`go vet ./...` clean** — No warnings. `7771e64`

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
