// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"bufio"
	"io"
	"io/fs"
	"iter"
	"maps"
	"path"
	"slices"
	"time"

	core "dappco.re/go/core"
)

// maxScannerBuffer is the maximum line length the scanner will accept.
// Set to 8 MiB to handle very large tool outputs without truncation.
const maxScannerBuffer = 8 * 1024 * 1024

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
	Message   rawJSON `json:"message"`
	UserType  string  `json:"userType"`
}

type rawMessage struct {
	Role    string    `json:"role"`
	Content []rawJSON `json:"content"`
}

type contentBlock struct {
	Type      string  `json:"type"`
	Name      string  `json:"name,omitempty"`
	ID        string  `json:"id,omitempty"`
	Text      string  `json:"text,omitempty"`
	Input     rawJSON `json:"input,omitempty"`
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
	Path    string `json:"path"`
}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
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

// ListSessions returns all sessions found in the Claude projects directory.
//
// Example:
// sessions, err := session.ListSessions("/tmp/projects")
func ListSessions(projectsDir string) ([]Session, error) {
	return slices.Collect(ListSessionsSeq(projectsDir)), nil
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
		matches := core.PathGlob(path.Join(projectsDir, "*.jsonl"))

		var sessions []Session
		for _, filePath := range matches {
			base := path.Base(filePath)
			id := core.TrimSuffix(base, ".jsonl")

			infoResult := hostFS.Stat(filePath)
			if !infoResult.OK {
				continue
			}
			info, ok := infoResult.Value.(fs.FileInfo)
			if !ok {
				continue
			}

			s := Session{
				ID:   id,
				Path: filePath,
			}

			// Quick scan for first and last timestamps
			openResult := hostFS.Open(filePath)
			if !openResult.OK {
				continue
			}
			f, ok := openResult.Value.(io.ReadCloser)
			if !ok {
				continue
			}

			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
			var firstTS, lastTS string
			for scanner.Scan() {
				var entry rawEntry
				if !core.JSONUnmarshal(scanner.Bytes(), &entry).OK {
					continue
				}
				if entry.Timestamp == "" {
					continue
				}
				if firstTS == "" {
					firstTS = entry.Timestamp
				}
				lastTS = entry.Timestamp
			}
			f.Close()

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
				s.StartTime = info.ModTime()
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
// deleted, err := session.PruneSessions("/tmp/projects", 24*time.Hour)
func PruneSessions(projectsDir string, maxAge time.Duration) (int, error) {
	matches := core.PathGlob(path.Join(projectsDir, "*.jsonl"))

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
	return deleted, nil
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
// sess, stats, err := session.FetchSession("/tmp/projects", "abc123")
func FetchSession(projectsDir, id string) (*Session, *ParseStats, error) {
	if core.Contains(id, "..") || containsAny(id, `/\`) {
		return nil, nil, core.E("FetchSession", "invalid session id", nil)
	}

	filePath := path.Join(projectsDir, id+".jsonl")
	return ParseTranscript(filePath)
}

// ParseTranscript reads a JSONL session file and returns structured events.
// Malformed or truncated lines are skipped; diagnostics are reported in ParseStats.
//
// Example:
// sess, stats, err := session.ParseTranscript("/tmp/projects/abc123.jsonl")
func ParseTranscript(filePath string) (*Session, *ParseStats, error) {
	openResult := hostFS.Open(filePath)
	if !openResult.OK {
		return nil, nil, core.E("ParseTranscript", "open transcript", resultError(openResult))
	}
	f, ok := openResult.Value.(io.ReadCloser)
	if !ok {
		return nil, nil, core.E("ParseTranscript", "unexpected file handle type", nil)
	}
	defer f.Close()

	base := path.Base(filePath)
	id := core.TrimSuffix(base, ".jsonl")

	sess, stats, err := parseFromReader(f, id)
	if sess != nil {
		sess.Path = filePath
	}
	if err != nil {
		return sess, stats, core.E("ParseTranscript", "parse transcript", err)
	}
	return sess, stats, nil
}

// ParseTranscriptReader parses a JSONL session from an io.Reader, enabling
// streaming parse without needing a file on disc. The id parameter sets
// the session ID (since there is no file name to derive it from).
//
// Example:
// sess, stats, err := session.ParseTranscriptReader(reader, "abc123")
func ParseTranscriptReader(r io.Reader, id string) (*Session, *ParseStats, error) {
	sess, stats, err := parseFromReader(r, id)
	if err != nil {
		return sess, stats, core.E("ParseTranscriptReader", "parse transcript", err)
	}
	return sess, stats, nil
}

// parseFromReader is the shared implementation for both file-based and
// reader-based parsing. It scans line-by-line using bufio.Scanner with
// an 8 MiB buffer, gracefully skipping malformed lines.
func parseFromReader(r io.Reader, id string) (*Session, *ParseStats, error) {
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

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, maxScannerBuffer), maxScannerBuffer)

	var lineNum int
	var lastRaw string
	var lastLineFailed bool

	for scanner.Scan() {
		lineNum++
		stats.TotalLines++

		raw := scanner.Text()
		if core.Trim(raw) == "" {
			continue
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
			continue
		}

		ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err != nil {
			stats.Warnings = append(stats.Warnings, core.Sprintf("line %d: bad timestamp %q: %v", lineNum, entry.Timestamp, err))
			continue
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
				continue
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
					inputStr := extractToolInput(block.Name, block.Input)
					pendingTools[block.ID] = toolUse{
						timestamp: ts,
						tool:      block.Name,
						input:     inputStr,
					}
				}
			}

		case "user":
			var msg rawMessage
			if !core.JSONUnmarshal(entry.Message, &msg).OK {
				stats.Warnings = append(stats.Warnings, core.Sprintf("line %d: failed to unmarshal user message", lineNum))
				continue
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
	}

	// Detect truncated final line.
	if lastLineFailed && lastRaw != "" {
		stats.Warnings = append(stats.Warnings, "truncated final line")
	}

	// Check for scanner buffer errors.
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, stats, scanErr
	}

	// Track orphaned tool calls (tool_use with no matching result).
	stats.OrphanedToolCalls = len(pendingTools)
	if stats.OrphanedToolCalls > 0 {
		for id := range pendingTools {
			stats.Warnings = append(stats.Warnings,
				core.Sprintf("orphaned tool call: %s", id))
		}
	}

	return sess, stats, nil
}

func extractToolInput(toolName string, raw rawJSON) string {
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
		parts := slices.Sorted(maps.Keys(m))
		return core.Join(", ", parts...)
	}

	return ""
}

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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
