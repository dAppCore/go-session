# Context — go-session

> Relevant knowledge from OpenBrain.

### 1. go-session [service] (score: 0.080)

[go-session] Licence

EUPL-1.2

### 2. go-session [convention] (score: 0.021)

Coding Standards

- UK English throughout (colour, licence, initialise)
- `declare(strict_types=1)` equivalent: explicit types on all signatures
- `go test ./...` must pass before commit
- `go vet ./...` must be clean before commit
- SPDX-Licence-Identifier: EUPL-1.2 header on all source files
- Conventional commits: `type(scope): description`
- Co-Author: `Co-Authored-By: Virgil <virgil@lethean.io>`
- Test naming: `TestFunctionName_Context_Good/Bad/Ugly`
- New tool types: add struct in `parser.go`, case in `extractToolInput`, label in `html.go`, tape entry in `video.go`, and tests in `parser_test.go`

### 3. go-session [service] (score: 0.006)

[go-session] Labels

The input label adapts to the tool type:

- **Bash**: "Command"
- **Read, Glob, Grep**: "Target"
- **Edit, Write**: "File"
- **User messages**: "Message"
- **Assistant**: "Response"

### 4. go-session [service] (score: -0.002)

[go-session] Installation

```bash
go get dappco.re/go/core/session@latest
```

### 5. go-session [convention] (score: -0.004)

Commands

```bash
go test ./...          # Run all tests
go test -v -run Name   # Run single test
go vet ./...           # Vet the package
```

### 6. go-session [service] (score: -0.023)

[go-session] Event Card Layout

Each card displays:

| Element | Description |
|---------|-------------|
| Timestamp | `HH:MM:SS` of the event |
| Tool badge | Colour-coded tool name |
| Input summary | Truncated to 120 characters |
| Duration | Formatted as ms/s/min/hr |
| Status icon | Green tick or red cross for tool calls |

Clicking a card expands it to show the full input (labelled contextually as Command, Message, File, or Target) and the complete output.

### 7. go-session [service] (score: -0.024)

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

### 8. go-session [service] (score: -0.031)

[go-session] Prerequisites

```bash
go install github.com/charmbracelet/vhs@latest
```

### 9. go-session [service] (score: -0.040)

[go-session] How It Works

1. A VHS `.tape` script is generated from the session events
2. The tape uses the Catppuccin Mocha theme at 1400x800 resolution
3. Only `tool_use` events are rendered:
   - **Bash**: Shows the command being typed, abbreviated output, and a status indicator
   - **Read/Edit/Write**: Shows a comment line with the file path
   - **Task**: Shows an "Agent:" comment with the task description
4. Each event includes a brief pause for readability
5. VHS renders the tape to the specified MP4 path

### 10. go-session [service] (score: -0.044)

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

