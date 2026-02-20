# Development Guide

## Prerequisites

- Go 1.25 or later (the benchmark suite uses `b.Loop()`, introduced in Go 1.25).
- `github.com/stretchr/testify` — test-only dependency, fetched automatically by `go test`.
- `vhs` (`github.com/charmbracelet/vhs`) — optional, required only for `RenderMP4`. Install with `go install github.com/charmbracelet/vhs@latest`.

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

# Check test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

The benchmark suite generates synthetic JSONL files of varying sizes:

| Benchmark | File size | Tool pairs |
|-----------|-----------|------------|
| `BenchmarkParseTranscript` | ~2.2 MB | 5 000 |
| `BenchmarkParseTranscript_Large` | ~11 MB | 25 000 |
| `BenchmarkListSessions` | 20 files, 100 pairs each | — |
| `BenchmarkSearch` | 10 files, 500 pairs each | — |

## Test Patterns

All tests are in the `session` package (white-box). Test files are co-located with source files.

### Test Naming Convention

Tests use a `_Good`, `_Bad`, `_Ugly` suffix:

- `_Good`: happy path; valid inputs, expected successful output.
- `_Bad`: expected error conditions or graceful degradation (malformed input, missing optional data).
- `_Ugly`: panic-inducing or extreme edge cases (missing files, nil input, path errors).

### Test Helpers

`parser_test.go` defines a set of helpers for building synthetic JSONL content:

```go
// Fixed epoch: 2026-02-20 10:00:00 UTC, offset by seconds
ts(offsetSec int) string

// Marshal arbitrary map to a single JSONL line
jsonlLine(m map[string]interface{}) string

// Convenience builders
userTextEntry(timestamp, text string) string
assistantTextEntry(timestamp, text string) string
toolUseEntry(timestamp, toolName, toolID string, input map[string]interface{}) string
toolResultEntry(timestamp, toolUseID string, content interface{}, isError bool) string

// Write lines to a temp .jsonl file, return path
writeJSONL(t *testing.T, dir string, name string, lines ...string) string
```

Use `t.TempDir()` for all test output; Go cleans it up automatically after each test.

### Coverage Targets

The current statement coverage is 90.9%. New contributions should maintain or improve this figure. When adding a function, add corresponding tests covering at minimum:
- The success path.
- Nil or zero-value inputs where applicable.
- At least one error path.

## Coding Standards

### Language

UK English throughout all source code comments, documentation, and commit messages. Examples: `colour`, `organisation`, `licence`, `initialise`.

### Formatting and Lint

Code must be formatted with `gofmt` (or `goimports`). `go vet ./...` must be clean before committing.

### Types and Declarations

- Use explicit types on struct fields and function signatures.
- Avoid `interface{}` in public APIs; use typed parameters where possible.
- Handle all errors explicitly; do not use blank `_` for error returns in non-test code.

### File Headers

Source files that are part of the main package should carry the SPDX licence identifier:

```go
// SPDX-Licence-Identifier: EUPL-1.2
package session
```

### Licence

EUPL-1.2. All new source files must include the SPDX header.

## Commit Guidelines

Use conventional commits:

```
type(scope): description
```

Common types: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`.

Examples:

```
feat(parser): add ParseStats orphan detection
fix(html): escape data-text attribute value
test(analytics): add latency calculation edge cases
```

All commits must include the co-author trailer:

```
Co-Authored-By: Virgil <virgil@lethean.io>
```

`go test ./...` must pass before committing. The repository does not use a CI gate at present, so this is a manual requirement.

## Adding a New Tool Type

1. Define an input struct in `parser.go`:
   ```go
   type myToolInput struct {
       SomeField string `json:"some_field"`
   }
   ```

2. Add a `case "MyTool":` branch in `extractToolInput` that unmarshals the struct and returns a human-readable string.

3. Add a corresponding case in `html.go`'s input label logic if the label should differ from the default `"Command"`.

4. Add a case in `video.go`'s `generateTape` switch if the tool should appear in the VHS tape.

5. Add tests in `parser_test.go` for `extractToolInput` covering the new tool name, and an integration test using `toolUseEntry` + `toolResultEntry` to exercise the full parse path.

## Adding Analytics Fields

`analytics.go` is a pure computation layer. To add a new metric:

1. Add the field to `SessionAnalytics`.
2. Populate it in the `Analyse` function's event iteration loop.
3. Add a row to `FormatAnalytics` if it should appear in CLI output.
4. Add a test case in `analytics_test.go`.

## Module Path and Go Workspace

The module path is `forge.lthn.ai/core/go-session`. If this package is used within the Go workspace at `forge.lthn.ai/core/go`, add it to `go.work` with `go work use ./go-session` and run `go work sync`.
