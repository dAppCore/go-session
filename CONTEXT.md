# Context — go-session

> Relevant knowledge from OpenBrain.

## 1. go-session [convention] (score: 0.636)

Documentation

- `/Users/snider/Code/go-session/docs/architecture.md` — JSONL format, parsing pipeline, event types, analytics, HTML rendering, XSS protection
- `/Users/snider/Code/go-session/docs/development.md` — prerequisites, build/test commands, test patterns, coding standards
- `/Users/snider/Code/go-session/docs/history.md` — completed phases, known limitations, future considerations

## 2. go-session [service] (score: 0.604)

[go-session] Pages

- [[Session-Format]] -- JSONL structure, parsing logic, and event types
- [[Rendering]] -- HTML timeline and MP4 video output

## 3. go-session [service] (score: 0.563)

[go-session] Core Types

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

## 4. go-session [service] (score: 0.560)

[go-session] Installation

```bash
go get forge.lthn.ai/core/go-session@latest
```

## 5. go-session [service] (score: 0.557)

[go-session] API Summary

| Function | Description |
|----------|-------------|
| `ListSessions(dir)` | List all `.jsonl` sessions in a directory, sorted newest first |
| `ParseTranscript(path)` | Parse a JSONL file into a structured `*Session` |
| `Search(dir, query)` | Search tool events across all sessions |
| `RenderHTML(sess, path)` | Generate self-contained HTML timeline |
| `RenderMP4(sess, path)` | Generate MP4 video via VHS (Charmbracelet) |

## 6. go-session [service] (score: 0.536)

[go-session] Prerequisites

```bash
go install github.com/charmbracelet/vhs@latest
```

## 7. go-session [service] (score: 0.524)

[go-session] Quick Start

```go
package main

import (
    "fmt"
    "log"

    "forge.lthn.ai/core/go-session"
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

## 8. go-session [service] (score: 0.523)

[go-session] Usage

```go
sess, err := session.ParseTranscript("session.jsonl")
if err != nil {
    log.Fatal(err)
}

if err := session.RenderMP4(sess, "output/session.mp4"); err != nil {
    log.Fatal(err)
}
```

## 9. go-session [service] (score: 0.520)

[go-session] Tape Configuration

The generated tape uses these defaults:

```
FontSize 16
Width 1400
Height 800
TypingSpeed 30ms
Theme "Catppuccin Mocha"
Shell bash
```

See also: [[Home]] | [[Session-Format]]

## 10. go-session [service] (score: 0.509)

[go-session] Rendering

go-session provides two output formats for visualising parsed sessions: a self-contained HTML timeline and an MP4 video rendered via Charmbracelet VHS.

