# Rendering

go-session provides two output formats for visualising parsed sessions: a self-contained HTML timeline and an MP4 video rendered via Charmbracelet VHS.

## HTML Timeline

`RenderHTML` generates a single HTML file with no external dependencies. The output includes:

- **Sticky header** with session ID, start time, duration, tool call count, and error count
- **Search bar** with real-time filtering (press `/` to focus)
- **Type filter** dropdown: All events, Tool calls only, Errors only, Bash only, User messages
- **Collapsible event cards** colour-coded by tool type:
  - Green: Bash commands
  - Blue (accent): Other tools (Read, Edit, Write, Grep, Glob)
  - Yellow: User messages
  - Grey: Assistant responses
  - Red border: Failed tool calls
- **Permalinks** on each event card for direct `#evt-N` links

### Usage

```go
sess, err := session.ParseTranscript("session.jsonl")
if err != nil {
    log.Fatal(err)
}

if err := session.RenderHTML(sess, "output/timeline.html"); err != nil {
    log.Fatal(err)
}
// Open output/timeline.html in any browser
```

### Event Card Layout

Each card displays:

| Element | Description |
|---------|-------------|
| Timestamp | `HH:MM:SS` of the event |
| Tool badge | Colour-coded tool name |
| Input summary | Truncated to 120 characters |
| Duration | Formatted as ms/s/min/hr |
| Status icon | Green tick or red cross for tool calls |

Clicking a card expands it to show the full input (labelled contextually as Command, Message, File, or Target) and the complete output.

### Labels

The input label adapts to the tool type:

- **Bash**: "Command"
- **Read, Glob, Grep**: "Target"
- **Edit, Write**: "File"
- **User messages**: "Message"
- **Assistant**: "Response"

## MP4 Video

`RenderMP4` generates a terminal recording using [VHS](https://github.com/charmbracelet/vhs). VHS must be installed separately.

### Prerequisites

```bash
go install github.com/charmbracelet/vhs@latest
```

### Usage

```go
sess, err := session.ParseTranscript("session.jsonl")
if err != nil {
    log.Fatal(err)
}

if err := session.RenderMP4(sess, "output/session.mp4"); err != nil {
    log.Fatal(err)
}
```

### How It Works

1. A VHS `.tape` script is generated from the session events
2. The tape uses the Catppuccin Mocha theme at 1400x800 resolution
3. Only `tool_use` events are rendered:
   - **Bash**: Shows the command being typed, abbreviated output, and a status indicator
   - **Read/Edit/Write**: Shows a comment line with the file path
   - **Task**: Shows an "Agent:" comment with the task description
4. Each event includes a brief pause for readability
5. VHS renders the tape to the specified MP4 path

### Tape Configuration

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
