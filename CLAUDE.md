# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Claude Code JSONL transcript parser, analytics engine, and HTML/video renderer. Module: `forge.lthn.ai/core/go-session`

## Commands

```bash
go test ./...                              # Run all tests
go test -v -run TestFunctionName_Context   # Run single test
go test -race ./...                        # Race detector
go test -bench=. -benchmem ./...           # Benchmarks
go vet ./...                               # Vet
golangci-lint run ./...                    # Lint (optional, config in .golangci.yml)
```

## Architecture

Single-package library (`package session`) with five source files forming a pipeline:

1. **parser.go** — Core JSONL parser. Reads Claude Code session files line-by-line (8 MiB scanner buffer), correlates `tool_use`/`tool_result` pairs via a `pendingTools` map keyed by tool ID, and produces `Session` with `[]Event`. Also handles session listing, fetching, and pruning.
2. **analytics.go** — Pure computation over `[]Event`. `Analyse()` returns `SessionAnalytics` (per-tool counts, error rates, latency stats, token estimates). No I/O.
3. **html.go** — `RenderHTML()` generates a self-contained HTML file (inline CSS/JS, dark theme, collapsible panels, client-side search). All user content is `html.EscapeString`-escaped.
4. **video.go** — `RenderMP4()` generates a VHS `.tape` script and shells out to `vhs`. Requires `vhs` on PATH.
5. **search.go** — `Search()`/`SearchSeq()` does cross-session case-insensitive substring search over tool event inputs and outputs.

Both slice-returning and `iter.Seq` variants exist for `ListSessions`, `Search`, and `Session.EventsSeq`.

### Adding a new tool type

Touch all layers: add input struct in `parser.go` → case in `extractToolInput` → label in `html.go` `RenderHTML` → tape entry in `video.go` `generateTape` → tests in `parser_test.go`.

## Testing

Tests are white-box (`package session`). Test helpers in `parser_test.go` build synthetic JSONL in-memory — no fixture files. Use `writeJSONL(t, dir, name, lines...)` and the entry builders (`toolUseEntry`, `toolResultEntry`, `userTextEntry`, `assistantTextEntry`).

Naming convention: `TestFunctionName_Context_Good/Bad/Ugly` (happy path / expected errors / extreme edge cases).

Coverage target: maintain ≥90.9%.

## Coding Standards

- UK English throughout (colour, licence, initialise)
- Explicit types on all function signatures and struct fields
- `go test ./...` and `go vet ./...` must pass before commit
- SPDX header on all source files: `// SPDX-Licence-Identifier: EUPL-1.2`
- Conventional commits: `type(scope): description`
- Co-Author trailer: `Co-Authored-By: Virgil <virgil@lethean.io>`
