# FINDINGS.md -- go-session

## 2026-02-19: Split from core/go (Virgil)

### Origin

Extracted from `forge.lthn.ai/core/go` `pkg/session/` on 19 Feb 2026.

### Architecture

- Parses Claude Code JSONL transcripts into an `Event` array
- Each event has type, timestamp, content, and optional tool metadata
- Supported tool types: Bash, Read, Edit, Write, Grep, Glob, Task

### Dependencies

- Zero external dependencies -- standard library only (`encoding/json`, `bufio`, `os`)

### Tests

- Test coverage for JSONL parsing and event type detection
