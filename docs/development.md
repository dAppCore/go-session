---
title: Development Guide
description: How to build, test, lint, and contribute to go-session.
---

# Development Guide

## Prerequisites

- **Go 1.26 or later** -- the module requires Go 1.26 (`go.mod`). The benchmark suite uses `b.Loop()`, introduced in Go 1.25.
- **`github.com/stretchr/testify`** -- test-only dependency, fetched automatically by `go test`.
- **`vhs`** (`github.com/charmbracelet/vhs`) -- optional, required only for `RenderMP4`. Install with `go install github.com/charmbracelet/vhs@latest`.
- **`golangci-lint`** -- optional, for running the full lint suite. Configuration is in `.golangci.yml`.

## Build and Test

```bash
# Run all tests
go test ./...

# Run a single test by name
go test -v -run TestParseTranscript_ToolCalls_Good

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Vet the package
go vet ./...

# Format code
gofmt -w .

# Lint (requires golangci-lint)
golangci-lint run ./...

# Check test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

If this module is part of the Go workspace at `~/Code/go.work`, you can also use the `core` CLI:

```bash
core go test
core go qa          # fmt + vet + lint + test
core go cov --open  # coverage with HTML report
```

## Test Structure

All tests are in the `session` package (white-box). Test files are co-located with their corresponding source files:

| Source file | Test file | What it covers |
|-------------|-----------|---------------|
| `parser.go` | `parser_test.go` | `ParseTranscript`, `ParseTranscriptReader`, `ListSessions`, `ListSessionsSeq`, `FetchSession`, `extractToolInput`, `extractResultContent`, `truncate`, `shortID`, `formatDuration`, edge cases (malformed JSON, truncated lines, binary garbage, null bytes, 5 MiB lines) |
| `analytics.go` | `analytics_test.go` | `Analyse` (nil, empty, single tool, mixed tools with errors, latency calculations, token estimation), `FormatAnalytics` |
| `html.go` | `html_test.go` | `RenderHTML` (basic session, empty session, error events, XSS protection, invalid path, label-per-tool-type) |
| `video.go` | `video_test.go` | `generateTape` (basic, skips non-tool events, failed commands, long output truncation, Task/Edit/Write events, empty session, empty command), `extractCommand`, `RenderMP4` error path |
| `search.go` | `search_test.go` | `Search` and `SearchSeq` (empty directory, no matches, single/multiple matches, case-insensitive, output matching, skips non-tool events, ignores non-JSONL files, skips malformed sessions) |
| -- | `bench_test.go` | Performance benchmarks for parsing, listing, and searching |

### Test Naming Convention

Tests use a `_Good`, `_Bad`, `_Ugly` suffix pattern:

- **`_Good`**: happy path; valid inputs, expected successful output.
- **`_Bad`**: expected error conditions or graceful degradation (malformed input, missing optional data).
- **`_Ugly`**: panic-inducing or extreme edge cases (missing files, nil input, binary garbage, path errors).

### Test Helpers

`parser_test.go` defines helpers for building synthetic JSONL content without external fixtures:

```go
// Fixed epoch: 2026-02-20 10:00:00 UTC, offset by seconds
ts(offsetSec int) string

// Marshal arbitrary map to a single JSONL line
jsonlLine(m map[string]any) string

// Convenience builders for specific entry types
userTextEntry(timestamp, text string) string
assistantTextEntry(timestamp, text string) string
toolUseEntry(timestamp, toolName, toolID string, input map[string]any) string
toolResultEntry(timestamp, toolUseID string, content any, isError bool) string

// Write lines to a temp .jsonl file, return the file path
writeJSONL(t *testing.T, dir string, name string, lines ...string) string
```

All test output uses `t.TempDir()`, which Go cleans up automatically after each test.

### Benchmarks

The benchmark suite generates synthetic JSONL files with a realistic distribution of tool types (Bash, Read, Edit, Grep, Glob):

| Benchmark | File size | Tool pairs |
|-----------|-----------|------------|
| `BenchmarkParseTranscript` | ~2.2 MB | 5,000 |
| `BenchmarkParseTranscript_Large` | ~11 MB | 25,000 |
| `BenchmarkListSessions` | 20 files, 100 pairs each | -- |
| `BenchmarkSearch` | 10 files, 500 pairs each | -- |

Run with:

```bash
go test -bench=. -benchmem ./...
```

### Coverage Targets

The current statement coverage is 90.9%. New contributions should maintain or improve this figure. When adding a function, add tests covering at minimum:

- The success path.
- Nil or zero-value inputs where applicable.
- At least one error path.

## Coding Standards

### Language

UK English throughout all source code comments, documentation, and commit messages. Examples: `colour`, `organisation`, `licence`, `initialise`, `centre`.

### Formatting and Lint

Code must be formatted with `gofmt`. The project uses `golangci-lint` with the following linters enabled (see `.golangci.yml`):

- `govet`, `errcheck`, `staticcheck`, `unused`, `gosimple`
- `ineffassign`, `typecheck`, `gocritic`, `gofmt`

Both `go vet ./...` and `golangci-lint run ./...` must be clean before committing.

### Types and Declarations

- Use explicit types on struct fields and function signatures.
- Avoid `interface{}` in public APIs; use typed parameters where possible.
- Handle all errors explicitly; do not use blank `_` for error returns in non-test code.

### File Headers

Source files should carry the SPDX licence identifier:

```go
// SPDX-Licence-Identifier: EUPL-1.2
package session
```

### Licence

EUPL-1.2. All new source files must include the SPDX header. By contributing, you agree that your contributions will be licensed under the European Union Public Licence 1.2.

## Commit Guidelines

Use conventional commits:

```
type(scope): description
```

Common types: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`.

Examples:

```
feat(parser): add ParseTranscriptReader for streaming parse
fix(html): escape data-text attribute value
test(analytics): add latency calculation edge cases
docs(architecture): update scanner buffer size
```

All commits must include the co-author trailer:

```
Co-Authored-By: Virgil <virgil@lethean.io>
```

`go test ./...` must pass before committing.

## Adding a New Tool Type

1. Define an input struct in `parser.go`:

   ```go
   type myToolInput struct {
       SomeField string `json:"some_field"`
   }
   ```

2. Add a `case "MyTool":` branch in `extractToolInput` that unmarshals the struct and returns a human-readable string.

3. Add a corresponding case in `html.go`'s input label logic (inside `RenderHTML`) if the label should differ from the default `"Command"`. For example, if MyTool targets a URL, use `"Target"`.

4. Add a case in `video.go`'s `generateTape` switch if the tool should appear in VHS tape output.

5. Add tests in `parser_test.go`:
   - A `TestExtractToolInput_MyTool_Good` test for `extractToolInput`.
   - An integration test using `toolUseEntry` + `toolResultEntry` to exercise the full parse pipeline.

## Adding Analytics Fields

`analytics.go` is a pure computation layer with no I/O. To add a new metric:

1. Add the field to the `SessionAnalytics` struct.
2. Populate it in the `Analyse` function's event iteration loop.
3. Add a row to `FormatAnalytics` if it should appear in CLI output.
4. Add a test case in `analytics_test.go`.

## Module Path and Go Workspace

The module path is `forge.lthn.ai/core/go-session`. If this package is used within a Go workspace, add it with:

```bash
go work use ./go-session
go work sync
```
