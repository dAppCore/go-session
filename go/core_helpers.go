// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"context"

	core "dappco.re/go"
)

var hostCore = core.New()
var hostFS = (&core.Fs{}).NewUnrestricted()

// sessionCore returns the shared core instance, initialising it if needed.
func sessionCore(c *core.Core) *core.Core {
	if c == nil {
		c = hostCore
	}
	if c == nil {
		c = core.New()
		hostCore = c
	}
	return c
}

// hostContext returns the context associated with the shared core instance.
func hostContext(c *core.Core) context.Context {
	c = sessionCore(c)
	return c.Context()
}

// hostProcess returns the process runner associated with the shared core instance.
func hostProcess(c *core.Core) *core.Process {
	return sessionCore(c).Process()
}

type rawjson []byte

// UnmarshalJSON stores raw JSON bytes without decoding their nested structure.
func (m *rawjson) UnmarshalJSON(data []byte) (
	err error,
) {
	if m == nil {
		return core.E("rawjson.UnmarshalJSON", "nil receiver", nil)
	}
	*m = append((*m)[:0], data...)
	return nil
}

// MarshalJSON returns the stored raw JSON bytes or null for a nil value.
func (m rawjson) MarshalJSON() (
	[]byte,
	error,
) {
	if m == nil {
		return []byte("null"), nil
	}
	return m, nil
}

// resultError extracts an error from a failed core result.
func resultError(result core.Result) (
	err error,
) {
	if result.OK {
		return nil
	}
	if err, ok := result.Value.(error); ok && err != nil {
		return err
	}
	return core.E("resultError", "unexpected core result failure", nil)
}

// repeatString repeats a string without importing strings.
func repeatString(s string, count int) string {
	if s == "" || count <= 0 {
		return ""
	}
	b := core.NewBuilder()
	for range count {
		b.WriteString(s)
	}
	return b.String()
}

// containsAny reports whether s contains any rune from chars.
func containsAny(s, chars string) bool {
	for _, ch := range chars {
		for _, candidate := range s {
			if candidate == ch {
				return true
			}
		}
	}
	return false
}

// indexOf returns the byte index of substr within s.
func indexOf(s, substr string) int {
	if substr == "" {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	limit := len(s) - len(substr)
	for i := 0; i <= limit; i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// trimQuotes removes matching single-token quote delimiters from s.
func trimQuotes(s string) string {
	if len(s) < 2 {
		return s
	}
	if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '`' && s[len(s)-1] == '`') {
		return s[1 : len(s)-1]
	}
	return s
}
