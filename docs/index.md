---
title: go-session
description: Claude Code JSONL transcript parser, analytics engine, and HTML timeline renderer for Go.
---

# go-session

`go-session` parses Claude Code JSONL session transcripts into structured event arrays, computes per-tool analytics, renders self-contained HTML timelines with client-side search, and generates VHS tape scripts for MP4 video output. It has no external runtime dependencies -- stdlib only.

**Module path:** `dappco.re/go/core/session`
**Go version:** 1.26
**Licence:** EUPL-1.2

## Quick Start

```go
import "dappco.re/go/core/session"

// Parse a single session file
sess, stats, err := session.ParseTranscript("/path/to/session.jsonl")

// Or parse from any io.Reader (streaming, in-memory, HTTP body, etc.)
sess, stats, err := session.ParseTranscriptReader(reader, "my-session-id")

// Compute analytics
analytics := session.Analyse(sess)
fmt.Println(session.FormatAnalytics(analytics))

// Render an interactive HTML timeline
err = session.RenderHTML(sess, "timeline.html")

// Search across all sessions in a directory
results, err := session.Search("~/.claude/projects/my-project", "git commit")

// List sessions (newest first)
sessions, err := session.ListSessions("~/.claude/projects/my-project")

// Prune old sessions
deleted, err := session.PruneSessions("~/.claude/projects/my-project", 30*24*time.Hour)
```

## Package Layout

The entire package lives in a single Go package (`session`) with five source files:

| File | Purpose |
|------|---------|
| `parser.go` | Core types (`Event`, `Session`, `ParseStats`), JSONL parsing (`ParseTranscript`, `ParseTranscriptReader`), session listing (`ListSessions`, `ListSessionsSeq`), pruning (`PruneSessions`), fetching (`FetchSession`), tool input extraction |
| `analytics.go` | `SessionAnalytics` type, `Analyse` (pure computation), `FormatAnalytics` (CLI-friendly text output) |
| `html.go` | `RenderHTML` -- self-contained dark-theme HTML timeline with collapsible panels, search, and XSS protection |
| `video.go` | `RenderMP4` -- VHS tape script generation and invocation for MP4 video output |
| `search.go` | `Search` and `SearchSeq` -- case-insensitive cross-session search over tool call inputs and outputs |

Test files mirror the source files (`parser_test.go`, `analytics_test.go`, `html_test.go`, `video_test.go`, `search_test.go`) plus `bench_test.go` for benchmarks.

## Dependencies

| Dependency | Scope | Purpose |
|------------|-------|---------|
| Go standard library | Runtime | All parsing, HTML rendering, file I/O, JSON decoding |
| `github.com/stretchr/testify` | Test only | Assertions and requirements in test files |
| `vhs` (charmbracelet) | Optional external binary | Required only by `RenderMP4` for MP4 video generation |

The package has **zero runtime dependencies** beyond the Go standard library. `testify` is fetched automatically by `go test` and is never imported outside test files.

## Supported Tool Types

The parser recognises the following Claude Code tool types and formats their input for human readability:

| Tool | Input format | Example |
|------|-------------|---------|
| Bash | `command # description` | `ls -la # list files` |
| Read | `file_path` | `/tmp/main.go` |
| Edit | `file_path (edit)` | `/tmp/main.go (edit)` |
| Write | `file_path (N bytes)` | `/tmp/out.txt (42 bytes)` |
| Grep | `/pattern/ in path` | `/TODO/ in /src` |
| Glob | `pattern` | `**/*.go` |
| Task | `[subagent_type] description` | `[research] Code review` |
| Any other (MCP tools, etc.) | Sorted top-level JSON keys | `body, repo, title` |

Unknown tools (including MCP tools like `mcp__forge__create_issue`) are handled gracefully by extracting and sorting the JSON field names from the raw input.

## Iterator Support

Several functions offer both slice-returning and iterator-based variants, using Go's `iter.Seq` type:

| Slice variant | Iterator variant | Description |
|---------------|-----------------|-------------|
| `ListSessions()` | `ListSessionsSeq()` | Enumerate sessions in a directory |
| `Search()` | `SearchSeq()` | Search across sessions |
| -- | `Session.EventsSeq()` | Iterate over events in a session |

The iterator variants avoid allocating the full result slice upfront and support early termination via `break` or `return` in `range` loops.

## Further Reading

- [Architecture](architecture.md) -- JSONL format, parsing pipeline, event model, analytics, HTML rendering, XSS protection, data flow
- [Development Guide](development.md) -- Prerequisites, build commands, test patterns, coding standards, how to add new tool types
- [Project History](history.md) -- Completed phases, known limitations, future considerations
