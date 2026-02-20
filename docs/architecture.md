# Architecture

Module: `forge.lthn.ai/core/go-session`

## Overview

go-session parses Claude Code JSONL transcript files into structured `Event` arrays, computes analytics over those events, and renders them as self-contained HTML timelines or MP4 video files via VHS. The package has no external runtime dependencies — only the standard library.

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

## Parsing Pipeline

### `ParseTranscript(path string) (*Session, *ParseStats, error)`

`ParseTranscript` streams the file line by line using `bufio.Scanner` with a 4 MB buffer, which handles large sessions without loading the entire file into memory.

The parser maintains a `pendingTools` map (keyed by tool ID) to correlate `tool_use` blocks with their corresponding `tool_result` blocks. When a result arrives, the parser computes the round-trip duration (`result.timestamp - toolUse.timestamp`), assembles an `Event`, and removes the entry from the map. Any tool IDs remaining in `pendingTools` after the scan completes are orphaned tool calls.

Malformed JSON lines are skipped without aborting the parse. The line number and a 100-character preview are appended to `ParseStats.Warnings`. A truncated final line is detected by checking whether the last scanned line failed to unmarshal.

The session's `ID` is derived from the filename (the `.jsonl` suffix is stripped). `StartTime` and `EndTime` are set from the first and last successfully parsed RFC3339Nano timestamps.

### `ListSessions(projectsDir string) ([]Session, error)`

`ListSessions` globs for `*.jsonl` files in the given directory and performs a lightweight scan of each file to extract the first and last timestamps. Sessions are returned sorted newest-first by `StartTime`. If no valid timestamps are found in a file (e.g. it is entirely malformed), `StartTime` falls back to the file's modification time.

## Event Types

The `Event` struct is the central data type:

```go
type Event struct {
    Timestamp time.Time
    Type      string        // "tool_use", "user", "assistant"
    Tool      string        // "Bash", "Read", "Edit", "Write", "Grep", "Glob", "Task"
    ToolID    string
    Input     string        // human-readable summary of the tool input
    Output    string        // truncated tool output (2000 chars max)
    Duration  time.Duration // time from tool_use to tool_result
    Success   bool
    ErrorMsg  string        // populated when Success is false
}
```

Non-tool events (`user` and `assistant`) carry only `Timestamp`, `Type`, and `Input` (the text content, truncated to 500 characters).

## Tool Input Extraction

`extractToolInput` converts the raw JSON input of a tool call into a readable string. Each recognised tool type has its own struct and formatting rule:

| Tool | Input format |
|------|-------------|
| Bash | `command # description` (description omitted if empty) |
| Read | `file_path` |
| Edit | `file_path (edit)` |
| Write | `file_path (N bytes)` |
| Grep | `/pattern/ in path` (path defaults to `.`) |
| Glob | `pattern` |
| Task | `[subagent_type] description` (falls back to truncated prompt) |

For unknown tools, the fallback extracts and sorts the JSON field names from the raw input object.

## Result Content Extraction

`extractResultContent` handles three forms that `tool_result` content can take:

- **String**: returned as-is.
- **Array of objects**: each object's `text` field is extracted and joined with newlines.
- **Map with a `text` key**: the text field is returned.
- **Anything else**: formatted with `fmt.Sprintf("%v", content)`.

## ParseStats

```go
type ParseStats struct {
    TotalLines        int
    SkippedLines      int
    OrphanedToolCalls int
    Warnings          []string
}
```

`ParseStats` is always returned alongside `*Session`. Callers may discard it with `_`. Warnings include per-line skipped-line notices (with line number and preview) and orphaned tool call IDs.

## Analytics

`Analyse(sess *Session) *SessionAnalytics` is a pure function (no I/O) that iterates events and computes:

- `Duration`: `EndTime - StartTime`
- `ActiveTime`: sum of all tool call durations
- `ToolCounts`, `ErrorCounts`: per-tool-name call and error tallies
- `SuccessRate`: `(totalCalls - totalErrors) / totalCalls`
- `AvgLatency`, `MaxLatency`: per-tool average and maximum durations
- `EstimatedInputTokens`, `EstimatedOutputTokens`: approximated at 1 token per 4 characters of input and output text respectively

`FormatAnalytics(a *SessionAnalytics) string` renders these fields as a tabular plain-text summary suitable for CLI display.

## Cross-Session Search

`Search(projectsDir, query string) ([]SearchResult, error)` iterates every `*.jsonl` file in the directory, parses each with `ParseTranscript`, and performs a case-insensitive substring match against the concatenation of each tool event's `Input` and `Output` fields. Only `tool_use` events are searched. Results carry the session ID, timestamp, tool name, and the matched input as context.

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

The arrow rotates 90 degrees (CSS `transform: rotate(90deg)`) when the panel is open. Output text in `.event-body` is capped at 400 px height with `overflow-y: auto`.

Input label semantics per tool:

| Tool | Label |
|------|-------|
| Bash | Command |
| Read, Glob, Grep | Target |
| Edit, Write | File |
| User | Message |
| Assistant | Response |

### Search and Filter

A sticky header contains a text input and a `<select>` filter. Both call `filterEvents()` on change. The filter adds or removes the `hidden` class from each `.event` element based on:

1. The type filter dropdown (`all`, `tool_use`, `errors`, `Bash`, `user`).
2. A case-insensitive substring match against the `data-text` attribute, which holds the lowercased concatenation of `Input` and `Output` for each event.

Pressing `/` focuses the search input (keyboard shortcut), unless an input element is already focused.

### XSS Protection

All user-controlled content is passed through `html.EscapeString` before being written into the HTML output. This covers event input text, output text, tool labels, and the `data-text` attribute used for client-side search. Raw strings are never interpolated directly into HTML.

## MP4 Video Rendering

`RenderMP4(sess *Session, outputPath string) error` generates a VHS tape script (`.tape` format for `github.com/charmbracelet/vhs`) from the session events and invokes the `vhs` binary to render it as an MP4.

`generateTape` constructs the tape script:
- Configuration: 1400x800, 16pt font, Catppuccin Mocha theme, 30 ms typing speed.
- Title frame: session short ID and start date, with a 2-second pause.
- Per Bash event: simulates typed command and abbreviated output (200 chars max), followed by a `# OK` or `# FAILED` comment and a 1-second pause.
- Read, Edit, Write events: a comment line showing tool name and truncated input, with a 500 ms pause.
- Task events: a comment line showing the agent description with a 1-second pause.
- Grep and Glob events are omitted from the tape (no visual output to simulate).

`extractCommand` strips the description suffix from a Bash input string by splitting on the first ` # ` separator.

`RenderMP4` is only usable when the `vhs` binary is present on `$PATH`; it returns an actionable error message if it is not.

## Truncation Limits

| Field | Limit |
|-------|-------|
| User / assistant text | 500 characters |
| Tool output | 2000 characters |
| Error message | 500 characters |
| HTML event header input | 120 characters |
| MP4 command output | 200 characters |
| Task description fallback | 80 characters |
| Malformed-line warning preview | 100 characters |

## Data Flow Diagram

```
JSONL file
    |
    v
bufio.Scanner (4 MB buffer)
    |
    +-- rawEntry (type, timestamp, sessionId, message)
    |       |
    |       +-- assistant/tool_use --> pendingTools[id]
    |       +-- user/tool_result   --> match pendingTools[id] --> Event
    |       +-- assistant/text     --> Event (assistant)
    |       +-- user/text          --> Event (user)
    |
    v
Session{ID, Path, StartTime, EndTime, Events[]}
    |
    +-- Analyse()      --> SessionAnalytics
    +-- RenderHTML()   --> .html (self-contained)
    +-- RenderMP4()    --> .mp4 (via vhs)
    +-- Search()       --> []SearchResult (cross-session)
```
