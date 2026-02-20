# go-session

Claude Code JSONL transcript parser, analytics engine, and HTML timeline renderer. Parses Claude Code session files into structured event arrays (tool calls with round-trip durations, user and assistant messages), computes per-tool analytics (call counts, error rates, average and peak latency, estimated token usage), renders self-contained HTML timelines with collapsible panels and client-side search, and generates VHS tape scripts for MP4 video output. No external runtime dependencies — stdlib only.

**Module**: `forge.lthn.ai/core/go-session`
**Licence**: EUPL-1.2
**Language**: Go 1.25

## Quick Start

```go
import "forge.lthn.ai/core/go-session"

sess, stats, err := session.ParseTranscript("/path/to/session.jsonl")
analytics := session.Analyse(sess)
fmt.Println(session.FormatAnalytics(analytics))

// Render interactive HTML timeline
err = session.RenderHTML(sess, "timeline.html")

// Search across all sessions in a directory
results, err := session.Search("~/.claude/projects/my-project", "git commit")
```

## Documentation

- [Architecture](docs/architecture.md) — JSONL format, parsing pipeline, event types, analytics, HTML rendering, XSS protection
- [Development Guide](docs/development.md) — prerequisites, build, test patterns, coding standards
- [Project History](docs/history.md) — completed phases, known limitations, future considerations

## Build & Test

```bash
go test ./...
go vet ./...
go build ./...
```

## Licence

European Union Public Licence 1.2 — see [LICENCE](LICENCE) for details.
