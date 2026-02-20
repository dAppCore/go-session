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

The parser already streams (bufio.Scanner, 4MB buffer), skips malformed JSON lines, and handles unknown tools via field-name fallback. Phase 1 adds structured reporting and orphan detection.

### 1.1 Parse Stats

- [x] **Add `ParseStats` struct** — Track: `TotalLines int`, `SkippedLines int`, `OrphanedToolCalls int`, `Warnings []string`. Return alongside `*Session` from `ParseTranscript`. Signature becomes `ParseTranscript(path string) (*Session, *ParseStats, error)`. **Keep backward compat**: callers can ignore the stats. `a6fb934`
- [x] **Count skipped lines** — Increment `SkippedLines` when `json.Unmarshal` fails. Add the line number and first 100 chars to `Warnings`. `a6fb934`
- [x] **Track orphaned tool calls** — After scanning, any entries remaining in `pendingTools` map are orphaned (tool_use with no result). Set `OrphanedToolCalls = len(pendingTools)`. Include orphaned tool IDs in `Warnings`. `a6fb934`
- [x] **Tests** — Verify ParseStats counts with: (a) clean JSONL, (b) 3 malformed lines mixed in, (c) 2 orphaned tool calls, (d) truncated final line. `a6fb934`

### 1.2 Truncated JSONL Detection

- [x] **Detect incomplete final line** — After `scanner.Scan()` loop, check `scanner.Err()` for buffer errors. Also detect if last raw line was non-empty but failed `json.Unmarshal` — add to Warnings as "truncated final line". `a6fb934`
- [x] **Tests** — File ending without newline, file ending mid-JSON object `{"type":"assi`, file ending with complete line but no trailing newline. `a6fb934`

## Phase 2: Analytics

### 2.1 SessionAnalytics Struct

- [x] **Create `analytics.go`** — `type SessionAnalytics struct` with all fields. `a6fb934`

### 2.2 Analyse Function

- [x] **`Analyse(sess *Session) *SessionAnalytics`** — Iterate `sess.Events`, populate all fields. Pure function, no I/O. `a6fb934`
- [x] **`FormatAnalytics(a *SessionAnalytics) string`** — Tabular text output: duration, tool breakdown, error rates, latency stats. Suitable for CLI display. `a6fb934`
- [x] **Tests** — (a) Empty session, (b) single tool call, (c) mixed tools with errors, (d) verify latency calculations, (e) token estimation matches expected values. `a6fb934`

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
