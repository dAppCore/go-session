# go-session

`dappco.re/go/core/session` -- Claude Code session parser and visualiser.

Reads JSONL transcript files produced by Claude Code, extracts structured events, and renders them as interactive HTML timelines or MP4 videos. Zero external dependencies (stdlib only).

## Installation

```bash
go get dappco.re/go/core/session@latest
```

## Core Types

```go
// Session holds parsed metadata and events from a transcript.
type Session struct {
    ID        string
    Path      string
    StartTime time.Time
    EndTime   time.Time
    Events    []Event
}

// Event represents a single action in the session timeline.
type Event struct {
    Timestamp time.Time
    Type      string        // "tool_use", "user", "assistant", "error"
    Tool      string        // "Bash", "Read", "Edit", "Write", "Grep", "Glob", etc.
    ToolID    string
    Input     string
    Output    string
    Duration  time.Duration
    Success   bool
    ErrorMsg  string
}
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "dappco.re/go/core/session"
)

func main() {
    // Parse a single transcript
    sess, err := session.ParseTranscript("~/.claude/projects/abc123.jsonl")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Session %s: %d events over %s\n",
        sess.ID, len(sess.Events), sess.EndTime.Sub(sess.StartTime))

    // Render to interactive HTML
    if err := session.RenderHTML(sess, "timeline.html"); err != nil {
        log.Fatal(err)
    }
}
```

## API Summary

| Function | Description |
|----------|-------------|
| `ListSessions(dir)` | List all `.jsonl` sessions in a directory, sorted newest first |
| `ParseTranscript(path)` | Parse a JSONL file into a structured `*Session` |
| `Search(dir, query)` | Search tool events across all sessions |
| `RenderHTML(sess, path)` | Generate self-contained HTML timeline |
| `RenderMP4(sess, path)` | Generate MP4 video via VHS (Charmbracelet) |

## Pages

- [[Session-Format]] -- JSONL structure, parsing logic, and event types
- [[Rendering]] -- HTML timeline and MP4 video output

## Licence

EUPL-1.2
