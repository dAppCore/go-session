// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"io"     // Note: intrinsic — Reader, ReadCloser, and EOF contracts for transcript streams and hostFS handles; no core equivalent
	"io/fs"  // Note: intrinsic — fs.FileInfo metadata returned from hostFS.Stat; no core equivalent
	"iter"   // Note: intrinsic — public lazy sequence API for sessions and events; no core equivalent
	"slices" // Note: intrinsic — iterator collection, sorted keys, and session ordering; no core equivalent
	"time"   // Note: intrinsic — RFC3339 transcript timestamps and session age calculations; no core equivalent

	core "dappco.re/go"
	coreerr "dappco.re/go"
)

// maxScannerBuffer is the maximum line length the scanner will accept.
// Set to 8 MiB to handle very large tool outputs without truncation.
const maxScannerBuffer = 8 * 1024 * 1024

// maxPendingToolCalls bounds unmatched tool_use entries retained while parsing.
const maxPendingToolCalls = 4096

// Event represents a single action in a session timeline.
//
// Example:
// evt := session.Event{Type: "tool_use", Tool: "Bash"}
type Event struct {
	Timestamp time.Time
	Type      string // "tool_use", "user", "assistant", "error"
	Tool      string // "Bash", "Read", "Edit", "Write", "Grep", "Glob", etc.
	ToolID    string
	Input     string // Command, file path, or message text
	Output    string // Result text
	Duration  time.Duration
	Success   bool
	ErrorMsg  string
}

// Session holds parsed session metadata and events.
//
// Example:
// sess := &session.Session{ID: "abc123", Events: []session.Event{}}
type Session struct {
	ID        string
	Path      string
	StartTime time.Time
	EndTime   time.Time
	Events    []Event
}

// EventsSeq returns an iterator over the session's events.
//
// Example:
//
//	for evt := range sess.EventsSeq() {
//		_ = evt
//	}
func (s *Session) EventsSeq() iter.Seq[Event] {
	return slices.Values(s.Events)
}

// rawEntry is the top-level structure of a Claude Code JSONL line.
type rawEntry struct {
	Type      string  `json:"type"`
	Timestamp string  `json:"timestamp"`
	SessionID string  `json:"sessionId"`
	Message   rawjson `json:"message"`
	UserType  string  `json:"userType"`
}

type rawMessage struct {
	Role    string    `json:"role"`
	Content []rawjson `json:"content"`
}

type contentBlock struct {
	Type      string  `json:"type"`
	Name      string  `json:"name,omitempty"`
	ID        string  `json:"id,omitempty"`
	Text      string  `json:"text,omitempty"`
	Input     rawjson `json:"input,omitempty"`
	ToolUseID string  `json:"tool_use_id,omitempty"`
	Content   any     `json:"content,omitempty"`
	IsError   *bool   `json:"is_error,omitempty"`
}

type bashInput struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Timeout     int    `json:"timeout"`
}

type readInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type editInput struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type writeInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type grepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

type taskInput struct {
	Prompt       string `json:"prompt"`
	Description  string `json:"description"`
	SubagentType string `json:"subagent_type"`
}

// ParseStats reports diagnostic information from a parse run.
//
// Example:
// stats := &session.ParseStats{TotalLines: 42}
type ParseStats struct {
	TotalLines        int
	SkippedLines      int
	OrphanedToolCalls int
	Warnings          []string
}

type ParsedSession struct {
	Session *Session
	Stats   *ParseStats
}

// ListSessions returns all sessions found in the Claude projects directory.
//
// Example:
// result := session.ListSessions("/tmp/projects")
func ListSessions(projectsDir string) core.Result {
	return core.Ok(slices.Collect(ListSessionsSeq(projectsDir)))
}

// ListSessionsSeq returns an iterator over all sessions found in the Claude projects directory.
//
// Example:
//
//	for sess := range session.ListSessionsSeq("/tmp/projects") {
//		_ = sess
//	}
func ListSessionsSeq(projectsDir string) iter.Seq[Session] {
	return func(yield func(Session) bool) {
		const op = "ListSessionsSeq"

		matches := core.PathGlob(transcriptPath(projectsDir, "*.jsonl"))

		var sessions []Session
		for _, filePath := range matches {
			base := core.PathBase(filePath)
			id := core.TrimSuffix(base, ".jsonl")

			openResult := openTranscriptNoFollow(filePath)
			if !openResult.OK {
				coreerr.Warn("skip unreadable transcript", "op", op, "file", filePath, "err", openResult.Error())
				continue
			}
			f := openResult.Value.(io.ReadCloser)

			s := Session{
				ID:   id,
				Path: filePath,
			}

			// Quick scan for first and last timestamps
			var firstTS, lastTS string
			scanResult := scanTranscriptLines(f, maxScannerBuffer, func(line []byte) bool {
				var entry rawEntry
				if !core.JSONUnmarshal(line, &entry).OK {
					return true
				}
				if entry.Timestamp == "" {
					return true
				}
				if firstTS == "" {
					firstTS = entry.Timestamp
				}
				lastTS = entry.Timestamp
				return true
			})
			closeErr := f.Close()
			if !scanResult.OK {
				coreerr.Warn("skip unreadable transcript", "op", op, "file", filePath, "err", scanResult.Error())
				continue
			}
			if closeErr != nil {
				coreerr.Warn("skip unreadable transcript", "op", op, "file", filePath, "err", closeErr)
				continue
			}

			if firstTS != "" {
				if t, err := time.Parse(time.RFC3339Nano, firstTS); err == nil {
					s.StartTime = t
				}
			}
			if lastTS != "" {
				if t, err := time.Parse(time.RFC3339Nano, lastTS); err == nil {
					s.EndTime = t
				}
			}
			if s.StartTime.IsZero() {
				infoResult := hostFS.Stat(filePath)
				if infoResult.OK {
					if info, ok := infoResult.Value.(fs.FileInfo); ok {
						s.StartTime = info.ModTime()
					}
				}
			}

			sessions = append(sessions, s)
		}

		slices.SortFunc(sessions, func(i, j Session) int {
			return j.StartTime.Compare(i.StartTime)
		})

		for _, s := range sessions {
			if !yield(s) {
				return
			}
		}
	}
}

// PruneSessions deletes session files in the projects directory that were last
// modified more than maxAge ago. Returns the number of files deleted.
//
// Example:
// result := session.PruneSessions("/tmp/projects", 24*time.Hour)
func PruneSessions(projectsDir string, maxAge time.Duration) core.Result {
	matches := core.PathGlob(transcriptPath(projectsDir, "*.jsonl"))

	var deleted int
	now := time.Now()
	for _, filePath := range matches {
		infoResult := hostFS.Stat(filePath)
		if !infoResult.OK {
			continue
		}
		info, ok := infoResult.Value.(fs.FileInfo)
		if !ok {
			continue
		}

		if now.Sub(info.ModTime()) > maxAge {
			if deleteResult := hostFS.Delete(filePath); deleteResult.OK {
				deleted++
			}
		}
	}
	return core.Ok(deleted)
}

// IsExpired returns true if the session's end time is older than the given maxAge
// relative to now.
//
// Example:
// expired := sess.IsExpired(24 * time.Hour)
func (s *Session) IsExpired(maxAge time.Duration) bool {
	if s.EndTime.IsZero() {
		return false
	}
	return time.Since(s.EndTime) > maxAge
}

// FetchSession retrieves a session by ID from the projects directory.
// It ensures the ID does not contain path traversal characters.
//
// Example:
// result := session.FetchSession("/tmp/projects", "abc123")
func FetchSession(projectsDir, id string) core.Result {
	if core.Contains(id, "..") || containsAny(id, `/\`) {
		return core.Fail(coreerr.E("FetchSession", "invalid session id", nil))
	}

	filePath := transcriptPath(projectsDir, id+".jsonl")
	openResult := openTranscriptNoFollow(filePath)
	if !openResult.OK {
		err := resultError(openResult)
		if isTranscriptMissing(err) {
			return core.Fail(coreerr.E("FetchSession", "open transcript", err))
		}
		return core.Fail(coreerr.E("FetchSession", "invalid session path", err))
	}
	f := openResult.Value.(io.ReadCloser)
	defer func() {
		if err := f.Close(); err != nil {
			coreerr.Warn("close transcript", "op", "FetchSession", "file", filePath, "err", err)
		}
	}()
	return parseTranscriptFile(filePath, f)
}

// ParseTranscript reads a JSONL session file and returns structured events.
// Malformed or truncated lines are skipped; diagnostics are reported in ParseStats.
//
// Example:
// result := session.ParseTranscript("/tmp/projects/abc123.jsonl")
func ParseTranscript(filePath string) core.Result {
	openResult := hostFS.Open(filePath)
	if !openResult.OK {
		return core.Fail(coreerr.E("ParseTranscript", "open transcript", resultError(openResult)))
	}
	f, ok := openResult.Value.(io.ReadCloser)
	if !ok {
		return core.Fail(coreerr.E("ParseTranscript", "unexpected file handle type", nil))
	}
	defer func() {
		if err := f.Close(); err != nil {
			coreerr.Warn("close transcript", "op", "ParseTranscript", "file", filePath, "err", err)
		}
	}()

	return parseTranscriptFile(filePath, f)
}

// parseTranscriptFile parses an already-open transcript reader and assigns path metadata.
func parseTranscriptFile(filePath string, r io.Reader) core.Result {
	base := core.PathBase(filePath)
	id := core.TrimSuffix(base, ".jsonl")

	parseResult := parseFromReader(r, id)
	if !parseResult.OK {
		return core.Fail(coreerr.E("ParseTranscript", "parse transcript", resultError(parseResult)))
	}
	parsed := parseResult.Value.(ParsedSession)
	if parsed.Session != nil {
		parsed.Session.Path = filePath
	}
	return core.Ok(parsed)
}

// ParseTranscriptReader parses a JSONL session from an io.Reader, enabling
// streaming parse without needing a file on disc. The id parameter sets
// the session ID (since there is no file name to derive it from).
//
// Example:
// result := session.ParseTranscriptReader(reader, "abc123")
func ParseTranscriptReader(r io.Reader, id string) core.Result {
	parseResult := parseFromReader(r, id)
	if !parseResult.OK {
		return core.Fail(coreerr.E("ParseTranscriptReader", "parse transcript", resultError(parseResult)))
	}
	return parseResult
}

// parseFromReader is the shared implementation for both file-based and
// reader-based parsing. It scans line-by-line with an 8 MiB buffer,
// gracefully skipping malformed lines.
func parseFromReader(r io.Reader, id string) core.Result {
	sess := &Session{
		ID: id,
	}

	stats := &ParseStats{}

	// Collect tool_use entries keyed by ID.
	type toolUse struct {
		timestamp time.Time
		tool      string
		input     string
	}
	pendingTools := make(map[string]toolUse)

	var lineNum int
	var lastRaw string
	var lastLineFailed bool

	scanResult := scanTranscriptLines(r, maxScannerBuffer, func(line []byte) bool {
		lineNum++
		stats.TotalLines++

		raw := string(line)
		if core.Trim(raw) == "" {
			return true
		}

		lastRaw = raw
		lastLineFailed = false

		var entry rawEntry
		if !core.JSONUnmarshalString(raw, &entry).OK {
			stats.SkippedLines++
			preview := raw
			if len(preview) > 100 {
				preview = preview[:100]
			}
			stats.Warnings = append(stats.Warnings,
				core.Sprintf("line %d: skipped (bad JSON): %s", lineNum, preview))
			lastLineFailed = true
			return true
		}

		ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err != nil {
			stats.Warnings = append(stats.Warnings, core.Sprintf("line %d: bad timestamp %q: %v", lineNum, entry.Timestamp, err))
			return true
		}

		if sess.StartTime.IsZero() && !ts.IsZero() {
			sess.StartTime = ts
		}
		if !ts.IsZero() {
			sess.EndTime = ts
		}

		switch entry.Type {
		case "assistant":
			var msg rawMessage
			if !core.JSONUnmarshal(entry.Message, &msg).OK {
				stats.Warnings = append(stats.Warnings, core.Sprintf("line %d: failed to unmarshal assistant message", lineNum))
				return true
			}
			for i, raw := range msg.Content {
				var block contentBlock
				if !core.JSONUnmarshal(raw, &block).OK {
					stats.Warnings = append(stats.Warnings, core.Sprintf("line %d block %d: failed to unmarshal content", lineNum, i))
					continue
				}

				switch block.Type {
				case "text":
					if text := core.Trim(block.Text); text != "" {
						sess.Events = append(sess.Events, Event{
							Timestamp: ts,
							Type:      "assistant",
							Input:     truncate(text, 500),
						})
					}

				case "tool_use":
					if block.ID == "" {
						continue
					}
					if _, exists := pendingTools[block.ID]; !exists && len(pendingTools) >= maxPendingToolCalls {
						stats.Warnings = append(stats.Warnings,
							core.Sprintf("line %d: skipped tool_use %q (pending tool limit reached)", lineNum, block.ID))
						continue
					}
					inputStr := extractToolInput(block.Name, block.Input)
					pendingTools[block.ID] = toolUse{
						timestamp: ts,
						tool:      block.Name,
						input:     truncate(inputStr, 500),
					}
				}
			}

		case "user":
			var msg rawMessage
			if !core.JSONUnmarshal(entry.Message, &msg).OK {
				stats.Warnings = append(stats.Warnings, core.Sprintf("line %d: failed to unmarshal user message", lineNum))
				return true
			}
			for i, raw := range msg.Content {
				var block contentBlock
				if !core.JSONUnmarshal(raw, &block).OK {
					stats.Warnings = append(stats.Warnings, core.Sprintf("line %d block %d: failed to unmarshal content", lineNum, i))
					continue
				}

				switch block.Type {
				case "tool_result":
					if tu, ok := pendingTools[block.ToolUseID]; ok {
						output := extractResultContent(block.Content)
						isError := block.IsError != nil && *block.IsError
						evt := Event{
							Timestamp: tu.timestamp,
							Type:      "tool_use",
							Tool:      tu.tool,
							ToolID:    block.ToolUseID,
							Input:     tu.input,
							Output:    truncate(output, 2000),
							Duration:  ts.Sub(tu.timestamp),
							Success:   !isError,
						}
						if isError {
							evt.ErrorMsg = truncate(output, 500)
						}
						sess.Events = append(sess.Events, evt)
						delete(pendingTools, block.ToolUseID)
					}

				case "text":
					if text := core.Trim(block.Text); text != "" {
						sess.Events = append(sess.Events, Event{
							Timestamp: ts,
							Type:      "user",
							Input:     truncate(text, 500),
						})
					}
				}
			}
		}
		return true
	})

	// Detect truncated final line.
	if lastLineFailed && lastRaw != "" {
		stats.Warnings = append(stats.Warnings, "truncated final line")
	}

	if !scanResult.OK {
		return core.Fail(resultError(scanResult))
	}

	// Track orphaned tool calls (tool_use with no matching result).
	stats.OrphanedToolCalls = len(pendingTools)
	if stats.OrphanedToolCalls > 0 {
		for id := range pendingTools {
			stats.Warnings = append(stats.Warnings,
				core.Sprintf("orphaned tool call: %s", id))
		}
	}

	return core.Ok(ParsedSession{Session: sess, Stats: stats})
}

// extractToolInput converts raw Claude tool input into a concise display string.
func extractToolInput(toolName string, raw rawjson) string {
	if raw == nil {
		return ""
	}

	switch toolName {
	case "Bash":
		var inp bashInput
		if core.JSONUnmarshal(raw, &inp).OK {
			desc := inp.Description
			if desc != "" {
				desc = " # " + desc
			}
			return inp.Command + desc
		}
	case "Read":
		var inp readInput
		if core.JSONUnmarshal(raw, &inp).OK {
			return inp.FilePath
		}
	case "Edit":
		var inp editInput
		if core.JSONUnmarshal(raw, &inp).OK {
			return core.Sprintf("%s (edit)", inp.FilePath)
		}
	case "Write":
		var inp writeInput
		if core.JSONUnmarshal(raw, &inp).OK {
			return core.Sprintf("%s (%d bytes)", inp.FilePath, len(inp.Content))
		}
	case "Grep":
		var inp grepInput
		if core.JSONUnmarshal(raw, &inp).OK {
			path := inp.Path
			if path == "" {
				path = "."
			}
			return core.Sprintf("/%s/ in %s", inp.Pattern, path)
		}
	case "Glob":
		var inp globInput
		if core.JSONUnmarshal(raw, &inp).OK {
			return inp.Pattern
		}
	case "Task":
		var inp taskInput
		if core.JSONUnmarshal(raw, &inp).OK {
			desc := inp.Description
			if desc == "" {
				desc = truncate(inp.Prompt, 80)
			}
			return core.Sprintf("[%s] %s", inp.SubagentType, desc)
		}
	}

	// Fallback: show raw JSON keys
	var m map[string]any
	if core.JSONUnmarshal(raw, &m).OK {
		parts := make([]string, 0, len(m))
		for key := range m {
			parts = append(parts, key)
		}
		slices.Sort(parts)
		return core.Join(", ", parts...)
	}

	return ""
}

// extractResultContent converts Claude tool_result content into plain text.
func extractResultContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if text, ok := m["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return core.Join("\n", parts...)
	case map[string]any:
		if text, ok := v["text"].(string); ok {
			return text
		}
	}
	return core.Sprint(content)
}

// truncate returns s capped to max bytes with an ellipsis marker.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// scanTranscriptLines streams newline-delimited records with a per-line size limit.
func scanTranscriptLines(r io.Reader, maxLineSize int, handle func([]byte) bool) core.Result {
	const op = "scanTranscriptLines"

	if maxLineSize <= 0 {
		maxLineSize = maxScannerBuffer
	}

	readBuffer := make([]byte, 64*1024)
	line := make([]byte, 0, 64*1024)

	for {
		n, readErr := r.Read(readBuffer)
		if n > 0 {
			chunk := readBuffer[:n]
			start := 0
			for i, b := range chunk {
				if b != '\n' {
					continue
				}
				if len(line)+i-start > maxLineSize {
					return core.Fail(coreerr.E(op, core.Sprintf("line exceeds %d bytes", maxLineSize), nil))
				}
				line = append(line, chunk[start:i]...)
				if !handle(trimLineBreak(line)) {
					return core.Ok(nil)
				}
				line = line[:0]
				start = i + 1
			}
			if start < len(chunk) {
				if len(line)+len(chunk)-start > maxLineSize {
					return core.Fail(coreerr.E(op, core.Sprintf("line exceeds %d bytes", maxLineSize), nil))
				}
				line = append(line, chunk[start:]...)
			}
		}

		if readErr == io.EOF {
			if len(line) > 0 {
				if !handle(trimLineBreak(line)) {
					return core.Ok(nil)
				}
			}
			return core.Ok(nil)
		}
		if readErr != nil {
			return core.Fail(coreerr.E(op, "read error", readErr))
		}
	}
}

// trimLineBreak removes a trailing carriage return from a scanned line.
func trimLineBreak(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		return line[:len(line)-1]
	}
	return line
}

// transcriptPath joins a projects directory and transcript file name.
func transcriptPath(projectsDir, name string) string {
	if projectsDir == "" {
		return core.CleanPath(name, "/")
	}
	return core.CleanPath(core.JoinPath(projectsDir, name), "/")
}
