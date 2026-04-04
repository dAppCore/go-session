# Project History

## Origin

Extracted from `dappco.re/go/core` (`pkg/session/`) on 19 February 2026. The initial extraction provided a working parser that read Claude Code JSONL transcripts into `Event` arrays and identified the seven supported tool types: Bash, Read, Edit, Write, Grep, Glob, and Task.

## Completed Phases

### Phase 0 — Hardening and Test Coverage (`f40caaa`)

Established the test baseline after extraction.

- 67 tests across four test files: `parser_test.go`, `html_test.go`, `search_test.go`, `video_test.go`.
- Statement coverage: 90.9%.
- `parser_test.go` covers: `ParseTranscript` with minimal JSONL, all seven tool types, errors, truncated and malformed lines, large sessions (1 100+ tool pairs), nested array and map result content, `ListSessions` (empty directory, single session, multiple sessions sorted by timestamp, non-JSONL files ignored, malformed JSONL with mod-time fallback), `extractToolInput` for all seven tools plus nil input, invalid JSON, and unknown tool fallback, and all three paths of `extractResultContent`.
- Benchmarks use `b.Loop()` (Go 1.25+) for accurate timing: 2.2 MB file (5 000 tool pairs), 11 MB file (25 000 tool pairs), `ListSessions` over 20 files, and `Search` over 10 files.
- `go vet ./...` was clean from the outset.

### Phase 1 — Parser Robustness (`a6fb934`)

Added structured diagnostic reporting to `ParseTranscript`.

- `ParseTranscript` signature changed to `(*Session, *ParseStats, error)`. Existing callers are unaffected as Go allows ignoring the additional return value with `_`.
- `ParseStats` tracks `TotalLines`, `SkippedLines`, `OrphanedToolCalls`, and `Warnings`.
- Skipped lines append a warning containing the line number and a 100-character preview of the raw content.
- Orphaned tool calls (tool_use entries with no matching tool_result) are counted and their IDs recorded in warnings after the scan completes.
- Truncated final line detection: if the last scanned line failed to unmarshal JSON, a `"truncated final line"` warning is appended.
- Scanner buffer errors surface directly as a returned error rather than silent truncation.
- Tests added for: clean JSONL (zero counts), three malformed lines mixed with valid content, two orphaned tool calls, file ending mid-JSON object, file ending without a trailing newline, and warning preview truncation for long malformed lines.

### Phase 2 — Analytics (`a6fb934`)

Added `analytics.go` providing `Analyse` and `FormatAnalytics`.

- `SessionAnalytics` holds duration, active time (sum of tool durations), event count, per-tool call and error counts, success rate, per-tool average and maximum latencies, and estimated token counts (1 token per 4 characters of input and output text).
- `Analyse` is a pure function with no I/O. Safe to call on a nil `*Session`.
- `FormatAnalytics` produces a tabular plain-text summary with a tool breakdown table sorted alphabetically by tool name.
- Tests cover: empty session, nil session, single tool call, mixed tools with errors (success rate and active time calculations), latency averages across three Bash calls and one Read call, and token estimation with known character counts.

### Phase 3 — HTML Timeline (`9b32678`)

Added `html.go` with `RenderHTML` (257 lines of code).

- Self-contained dark-theme HTML file with all CSS and JavaScript inline.
- Sticky header showing session short ID (first 8 characters), start timestamp, duration, tool call count, and error count (omitted when zero).
- Events colour-coded by type: Bash (green), errors (red border), user messages (yellow), assistant messages (dim).
- Collapsible detail panels: `toggle(i)` toggles the `open` CSS class; arrow rotates 90 degrees using CSS transition. Output capped at 400 px with `overflow-y: auto`.
- Smart labels per event type: Command (Bash), Target (Read, Glob, Grep), File (Edit, Write), Message (user), Response (assistant).
- Text input and dropdown filter in the sticky header; `filterEvents()` applies both simultaneously via the `data-text` attribute.
- Keyboard shortcut: `/` focuses the search input.
- XSS protection: all user content passed through `html.EscapeString` including the `data-text` search attribute.
- Six tests: basic session structure, empty session, error events and status icons, special characters and XSS vectors, invalid output path error, and label-per-tool-type verification.

## Known Limitations

### `extractCommand` and Inline `#` Characters

`extractCommand` in `video.go` identifies the description suffix by splitting on the first occurrence of ` # ` in the input string. If a Bash command itself contains a literal ` # ` (for example inside a quoted string), the command will be truncated at that position in the MP4 tape output. This is a documented behaviour rather than a bug, since `extractToolInput` always constructs Bash input as `command + " # " + description`, making a collision possible only in contrived cases.

### `RenderMP4` Requires an External Binary

`RenderMP4` depends on the `vhs` binary (`github.com/charmbracelet/vhs`). There is no pure-Go fallback. The function returns a clear error message when `vhs` is absent. It is not testable in automated CI without the binary present; the test suite covers `generateTape` (the pure logic) and the error path only.

### Search is Sequential

`Search` parses every session file in the directory sequentially. For directories with many large session files this is the dominant cost. There is no indexing or caching.

### Token Estimation

`EstimatedInputTokens` and `EstimatedOutputTokens` use a fixed ratio of 4 characters per token. This approximation is suitable for rough comparisons but should not be treated as an accurate token count for billing or quota purposes.

## Future Considerations

The following have been identified as potential improvements but are not currently prioritised:

- **Parallel search**: fan out `ParseTranscript` calls across goroutines with a result channel to reduce wall time for large directories.
- **Persistent index**: a lightweight SQLite index or binary cache per session file to avoid re-parsing on every `Search` or `ListSessions` call.
- **Additional tool types**: the parser's `extractToolInput` fallback handles any unknown tool by listing its JSON keys. Dedicated handling could be added for `WebFetch`, `WebSearch`, `NotebookEdit`, and other tools that appear in Claude Code sessions.
- **HTML export options**: configurable truncation limits and optional full-output display remain open; per-event direct links are now available via `#evt-{i}` permalinks.
- **VHS alternative**: a pure-Go terminal animation renderer to eliminate the `vhs` dependency for MP4 output.
