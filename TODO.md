# TODO.md — go-session

Dispatched from core/go orchestration. Pick up tasks in order.

---

## Phase 0: Hardening & Test Coverage

- [x] **Add parser tests** — 67 tests, 90.9% coverage. ParseTranscript with minimal JSONL, all 7 tool types, errors, truncated/malformed, large sessions (1100+), nested results. `f40caaa`
- [x] **Add ListSessions tests** — Empty dir, single/multi sorted, non-JSONL ignored, malformed JSONL modtime fallback. `f40caaa`
- [x] **Tool extraction coverage** — All 7 tool types + nil, invalid JSON, unknown tool fallback. `f40caaa`
- [x] **Benchmark parsing** — 2.2MB (5K tools) and 11MB (25K tools) files. Plus ListSessions and Search benchmarks. `b.Loop()` (Go 1.25+). `f40caaa`
- [x] **`go vet ./...` clean** — No warnings. `f40caaa`

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
