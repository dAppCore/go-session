---
title: Architecture
description: Internals of go-session -- JSONL format, parsing pipeline, event model, analytics, HTML rendering, video output, and data flow.
---

# Architecture

Module: `dappco.re/go/core/session`

## Overview

go-session parses Claude Code JSONL transcript files into structured `Event` arrays, computes analytics over those events, renders them as self-contained HTML timelines, and optionally generates MP4 video files via VHS. The package has no external runtime dependencies -- only the Go standard library.

## JSONL Transcript Format

Claude Code stores sessions as newline-delimited JSON (JSONL) files. Each line is a top-level entry:

```json
{"type":"assistant","timestamp":"2026-02-20T10:00:01Z","sessionId":"abc123","message":{...}}
{"type":"user","timestamp":"2026-02-20T10:00:02Z","sessionId":"abc123","message":{...}}
```

The `type` field is either `"assistant"` or `"user"`. The `message` field contains a `role` and a `content` array of content blocks. Content blocks carry a `type` of `"text"`, `"tool_use"`, or `"tool_result"`.

Tool calls appear as a two-entry sequence:

1. An `assistant` entry with a `tool_use` content block (carries the tool name, unique ID, and input parameters).
2. A subsequent `user` entry with a `tool_result` content block referencing the same tool ID (carries the output and an `is_error` flag).

## Key Types

### Event

The central data type representing a single action in a session timeline:

```go
type Event struct {
    Timestamp time.Time
    Type      string        // "tool_use", "user", "assistant", "error"
    Tool      string        // "Bash", "Read", "Edit", "Write", "Grep", "Glob", "Task", etc.
    ToolID    string
    Input     string        // Human-readable summary of the tool input
    Output    string        // Truncated tool output (max 2000 chars)
    Duration  time.Duration // Round-trip time from tool_use to tool_result
    Success   bool
    ErrorMsg  string        // Populated when Success is false
}
```

Non-tool events (`user` and `assistant`) carry only `Timestamp`, `Type`, and `Input` (text content, truncated to 500 characters).

### Session

Holds parsed session metadata and events:

```go
type Session struct {
    ID        string
    Path      string
    StartTime time.Time
    EndTime   time.Time
    Events    []Event
}
```

`ID` is derived from the filename (the `.jsonl` suffix is stripped). `Path` is set for file-based parses; it remains empty for reader-based parses. `EventsSeq()` returns an `iter.Seq[Event]` iterator over the events.

### ParseStats

Diagnostic information returned alongside every parse:

```go
type ParseStats struct {
    TotalLines        int
    SkippedLines      int
    OrphanedToolCalls int
    Warnings          []string
}
```

Warnings include per-line skipped-line notices (with line number and a 100-character preview), orphaned tool call IDs, and truncated final line detection. Callers may discard `ParseStats` with `_`.

### SessionAnalytics

Computed metrics for a parsed session:

```go
type SessionAnalytics struct {
    Duration              time.Duration
    ActiveTime            time.Duration
    EventCount            int
    ToolCounts            map[string]int
    ErrorCounts           map[string]int
    SuccessRate           float64
    AvgLatency            map[string]time.Duration
    MaxLatency            map[string]time.Duration
    EstimatedInputTokens  int
    EstimatedOutputTokens int
}
```

### SearchResult

A match found during cross-session search:

```go
type SearchResult struct {
    SessionID string
    Timestamp time.Time
    Tool      string
    Match     string
}
```

## Parsing Pipeline

### ParseTranscript / ParseTranscriptReader

Both functions share a common internal implementation (`parseFromReader`). The file-based variant opens the file and derives the session ID from the filename; the reader-based variant accepts any `io.Reader` and an explicit ID parameter.

The parser streams line by line using `bufio.Scanner` with an **8 MiB buffer** (defined as the `maxScannerBuffer` constant), which handles very large tool outputs without truncation.

Each line is unmarshalled into a `rawEntry` struct:

```go
type rawEntry struct {
    Type      string          `json:"type"`
    Timestamp string          `json:"timestamp"`
    SessionID string          `json:"sessionId"`
    Message   json.RawMessage `json:"message"`
    UserType  string          `json:"userType"`
}
```

The parser maintains a `pendingTools` map (keyed by tool ID) to correlate `tool_use` blocks with their corresponding `tool_result` blocks. When a result arrives, the parser computes the round-trip duration (`result.timestamp - toolUse.timestamp`), assembles an `Event`, and removes the entry from the map.

Malformed JSON lines are skipped without aborting the parse. The line number and a 100-character preview are appended to `ParseStats.Warnings`. A truncated final line is detected by checking whether the last scanned line failed to unmarshal.

Any tool IDs remaining in `pendingTools` after the scan completes are counted as orphaned tool calls and recorded in the stats.

### ListSessions / ListSessionsSeq

Globs for `*.jsonl` files in the given directory and performs a lightweight scan of each file (using a 1 MiB buffer) to extract the first and last timestamps. Sessions are returned sorted newest-first by `StartTime`. If no valid timestamps are found (e.g. entirely malformed files), `StartTime` falls back to the file's modification time.

`ListSessionsSeq` returns an `iter.Seq[Session]` for lazy consumption.

### FetchSession

Retrieves a single session by ID. Guards against path traversal by rejecting IDs containing `..`, `/`, or `\`. Delegates to `ParseTranscript`.

### PruneSessions

Deletes session files in a directory whose modification time exceeds the given `maxAge`. Returns the count of deleted files.

## Tool Input Extraction

`extractToolInput` converts the raw JSON input of a tool call into a readable string. Each recognised tool type has its own input struct and formatting rule:

| Tool | Input struct | Formatted output |
|------|-------------|-----------------|
| Bash | `bashInput{Command, Description, Timeout}` | `command # description` (description omitted if empty) |
| Read | `readInput{FilePath, Offset, Limit}` | `file_path` |
| Edit | `editInput{FilePath, OldString, NewString}` | `file_path (edit)` |
| Write | `writeInput{FilePath, Content}` | `file_path (N bytes)` |
| Grep | `grepInput{Pattern, Path}` | `/pattern/ in path` (path defaults to `.`) |
| Glob | `globInput{Pattern, Path}` | `pattern` |
| Task | `taskInput{Prompt, Description, SubagentType}` | `[subagent_type] description` (falls back to truncated prompt) |

For unknown tools (including MCP tools), the fallback extracts the top-level JSON keys, sorts them alphabetically, and joins them with commas. If the input is nil or completely unparseable, an empty string is returned.

## Result Content Extraction

`extractResultContent` handles the three forms that `tool_result` content can take:

- **String**: returned as-is.
- **Array of objects**: each object's `text` field is extracted and joined with newlines.
- **Map with a `text` key**: the text value is returned.
- **Anything else**: formatted with `fmt.Sprintf("%v", content)`.

## Analytics

`Analyse(sess *Session) *SessionAnalytics` is a pure function (no I/O) that iterates events once and computes:

- **Duration**: `EndTime - StartTime`
- **ActiveTime**: sum of all tool call durations
- **ToolCounts**, **ErrorCounts**: per-tool-name call and error tallies
- **SuccessRate**: `(totalCalls - totalErrors) / totalCalls`
- **AvgLatency**, **MaxLatency**: per-tool average and maximum durations
- **EstimatedInputTokens**, **EstimatedOutputTokens**: approximated at 1 token per 4 characters of input and output text respectively

`Analyse` is safe to call on a nil `*Session` (returns zeroed analytics).

`FormatAnalytics(a *SessionAnalytics) string` renders the analytics as a tabular plain-text summary with a tool breakdown table sorted alphabetically by tool name, suitable for CLI display.

## Cross-Session Search

`Search(projectsDir, query string) ([]SearchResult, error)` iterates every `*.jsonl` file in the directory, parses each with `ParseTranscript`, and performs a case-insensitive substring match against the concatenation of each tool event's `Input` and `Output` fields. Only `tool_use` events are searched; user and assistant text messages are excluded from results. The `Match` field in each result carries the tool input as context, or a truncated portion of the output if the input is empty.

`SearchSeq` returns an `iter.Seq[SearchResult]` for lazy consumption and early termination.

## HTML Timeline Rendering

`RenderHTML(sess *Session, outputPath string) error` generates a fully self-contained HTML file. All CSS and JavaScript are written inline; the output has no external dependencies and can be opened directly in any browser.

### Visual Design

The timeline uses a dark theme with CSS custom properties:

| Variable | Value | Purpose |
|----------|-------|---------|
| `--bg` | `#0d1117` | Page background |
| `--bg2` | `#161b22` | Header and event header background |
| `--bg3` | `#21262d` | Hovered elements, input fields |
| `--fg` | `#c9d1d9` | Primary text |
| `--dim` | `#8b949e` | Secondary text (timestamps, durations) |
| `--accent` | `#58a6ff` | Tool names, links, focused borders |
| `--green` | `#3fb950` | Bash tool label, success icons |
| `--red` | `#f85149` | Error tool label, error event border, failure icons |
| `--yellow` | `#d29922` | User message label |
| `--border` | `#30363d` | Default card borders |

The font stack is `'SF Mono', 'Cascadia Code', 'JetBrains Mono', monospace` at 13px.

### Event Colour Coding

Tool type determines the CSS class applied to the `.tool` label span:

| Event type / tool | CSS class | Colour |
|------------------|-----------|--------|
| Bash tool | `.bash` | green |
| Failed tool | `.error` (on `.event`) | red border |
| User message | `.user` | yellow |
| Assistant message | `.assistant` | dim |
| All other tools | lowercase tool name | accent (default) |

Success or failure of a `tool_use` event is indicated by a Unicode check mark (U+2713) in green or cross (U+2717) in red.

### Collapsible Panels

Each event is rendered as a `<div class="event">` containing:

- `.event-header`: always visible; shows timestamp, tool label, truncated input (120 chars), duration, and status icon.
- `.event-body`: hidden by default; shown on click via the `toggle(i)` JavaScript function which toggles the `open` class.

The arrow indicator rotates 90 degrees (CSS `transform: rotate(90deg)`) when the panel is open. Output text in `.event-body` is capped at 400px height with `overflow-y: auto`.

Input label semantics vary per tool:

| Tool | Label |
|------|-------|
| Bash | Command |
| Read, Glob, Grep | Target |
| Edit, Write | File |
| User | Message |
| Assistant | Response |

### Search and Filter

A sticky header contains a text input and a `<select>` filter dropdown. Both call `filterEvents()` on change. The filter adds or removes the `hidden` class from each `.event` element based on:

1. The type filter dropdown (`all`, `tool_use`, `errors`, `Bash`, `user`).
2. A case-insensitive substring match against the `data-text` attribute, which holds the lowercased concatenation of `Input` and `Output` for each event.

Pressing `/` focuses the search input (keyboard shortcut), unless an input element is already focused.

### XSS Protection

All user-controlled content is passed through `html.EscapeString` before being written into the HTML output. This covers event input text, output text, tool labels, and the `data-text` attribute used for client-side search. Raw strings are never interpolated directly into HTML.

## MP4 Video Rendering

`RenderMP4(sess *Session, outputPath string) error` generates a VHS tape script (`.tape` format for `github.com/charmbracelet/vhs`) from the session events and invokes the `vhs` binary to render it as an MP4.

`generateTape` constructs the tape script:

- **Configuration**: 1400x800, 16pt font, Catppuccin Mocha theme, 30ms typing speed, bash shell.
- **Title frame**: session short ID (first 8 characters) and start date, followed by a 2-second pause.
- **Bash events**: simulated typed command with `$ ` prefix, abbreviated output (200 chars max), and a status comment (`# OK` or `# FAILED`) with a 1-second pause.
- **Read, Edit, Write events**: a comment line showing tool name and truncated input (80 chars), with a 500ms pause.
- **Task events**: a comment line showing the agent description, with a 1-second pause.
- **Grep and Glob events**: omitted from the tape (no visual output to simulate).
- **Trailer**: 3-second pause at the end.

`extractCommand` strips the description suffix from a Bash input string by splitting on the first ` # ` separator. If the separator appears at position 0, the full input is returned unchanged.

`RenderMP4` requires the `vhs` binary on `$PATH` and returns an actionable error message if it is absent.

## Truncation Limits

| Field | Limit |
|-------|-------|
| User / assistant text input | 500 characters |
| Tool output | 2,000 characters |
| Error message | 500 characters |
| HTML event header input | 120 characters |
| MP4 command output | 200 characters |
| Task description fallback | 80 characters |
| Malformed-line warning preview | 100 characters |
| Scanner buffer | 8 MiB |

## Data Flow Diagram

```
JSONL file / io.Reader
    |
    v
bufio.Scanner (8 MiB buffer)
    |
    +-- rawEntry (type, timestamp, sessionId, message)
    |       |
    |       +-- assistant message
    |       |       +-- tool_use block  --> pendingTools[id]
    |       |       +-- text block      --> Event{Type: "assistant"}
    |       |
    |       +-- user message
    |               +-- tool_result block --> match pendingTools[id] --> Event{Type: "tool_use"}
    |               +-- text block        --> Event{Type: "user"}
    |
    v
Session{ID, Path, StartTime, EndTime, Events[]}
    |
    +-- Analyse()       --> SessionAnalytics
    |                        +-- FormatAnalytics() --> plain text
    |
    +-- RenderHTML()    --> .html (self-contained, dark theme)
    |
    +-- RenderMP4()     --> .tape --> vhs --> .mp4
    |
    +-- Search()        --> []SearchResult (cross-session)
```
