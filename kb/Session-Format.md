# Session Format

Claude Code writes session transcripts as JSONL (one JSON object per line) to `~/.claude/projects/`. Each line has a consistent top-level structure that the parser decodes into structured `Event` values.

## JSONL Line Structure

Every line in a transcript file follows this schema:

```json
{
  "type": "assistant" | "user",
  "timestamp": "2026-02-19T14:30:00.000Z",
  "sessionId": "abc123...",
  "message": { ... }
}
```

The `message` field contains a `role` and an array of `content` blocks. The parser handles two entry types:

- **`assistant`** entries contain `text` blocks (Claude's prose) and `tool_use` blocks (tool invocations)
- **`user`** entries contain `text` blocks (human messages) and `tool_result` blocks (tool outputs)

## Event Types

The parser produces four event types:

| Type | Source | Description |
|------|--------|-------------|
| `tool_use` | assistant + user | A tool call paired with its result |
| `user` | user text block | A human message |
| `assistant` | assistant text block | Claude's reasoning or response |
| `error` | tool_result with `is_error: true` | A failed tool invocation |

## Parsing Pipeline

`ParseTranscript` processes the JSONL file in a single pass:

1. **Scan** each line into a `rawEntry` struct
2. For **assistant** entries, extract `tool_use` blocks and store them in a pending map keyed by tool ID
3. For **user** entries, match `tool_result` blocks against pending tool uses by `tool_use_id`
4. **Pair** the tool invocation with its result to compute duration and success/failure
5. Extract text blocks as `user` or `assistant` events

```go
sess, err := session.ParseTranscript("/path/to/session.jsonl")
if err != nil {
    log.Fatal(err)
}

for _, evt := range sess.Events {
    if evt.Type == "tool_use" && !evt.Success {
        fmt.Printf("FAILED: %s %s -- %s\n", evt.Tool, evt.Input, evt.ErrorMsg)
    }
}
```

## Tool Input Extraction

Each tool type has its input decoded differently:

| Tool | Extracted Input |
|------|----------------|
| `Bash` | Command string (with optional `# description` suffix) |
| `Read` | File path |
| `Edit` | File path with `(edit)` suffix |
| `Write` | File path with byte count |
| `Grep` | `/pattern/ in path` |
| `Glob` | Glob pattern |
| `Task` | `[subagent_type] description` |

Unknown tools fall back to listing the JSON keys from the input object.

## Listing Sessions

`ListSessions` scans a directory for `.jsonl` files and performs a fast two-pass read (first and last timestamp) without fully parsing every event:

```go
sessions, err := session.ListSessions("~/.claude/projects/")
if err != nil {
    log.Fatal(err)
}
for _, s := range sessions {
    fmt.Printf("%s  %s  %s\n", s.ID[:8], s.StartTime.Format("02 Jan 15:04"), s.EndTime.Sub(s.StartTime))
}
```

Results are sorted newest first.

## Cross-Session Search

`Search` parses all sessions and finds `tool_use` events matching a case-insensitive query against both input and output text:

```go
results, err := session.Search("~/.claude/projects/", "migration")
for _, r := range results {
    fmt.Printf("[%s] %s: %s\n", r.SessionID[:8], r.Tool, r.Match)
}
```

Returns `[]SearchResult` with session ID, timestamp, tool name, and matching context.

See also: [[Home]] | [[Rendering]]
