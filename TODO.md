# TODO.md — go-session

Dispatched from core/go orchestration. Pick up tasks in order.

---

## Phase 0: Hardening & Test Coverage

- [ ] **Add parser tests** — Test `ParseTranscript()` with: minimal valid JSONL (1 user + 1 assistant message), tool call events (Bash, Read, Edit, Write, Grep, Glob, Task), truncated JSONL (incomplete last line), empty file, malformed JSON lines (should skip gracefully), very large session (1000+ events), nested tool results with arrays and maps.
- [ ] **Add ListSessions tests** — Test with: empty directory, single session, multiple sessions sorted by date, non-.jsonl files ignored.
- [ ] **Tool extraction coverage** — Test `extractToolInput()` for each supported tool type. Verify correct field extraction from JSON input.
- [ ] **Benchmark parsing** — `BenchmarkParseTranscript` with a 10MB JSONL file. Measure memory and time.
- [ ] **`go vet ./...` clean** — Fix any warnings.

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
