// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"bytes"

	core "dappco.re/go/core"
)

var hostFS = (&core.Fs{}).NewUnrestricted()

type rawJSON []byte

func (m *rawJSON) UnmarshalJSON(data []byte) error {
	if m == nil {
		return core.E("rawJSON.UnmarshalJSON", "nil receiver", nil)
	}
	*m = append((*m)[:0], data...)
	return nil
}

func (m rawJSON) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return m, nil
}

func resultError(result core.Result) error {
	if result.OK {
		return nil
	}
	if err, ok := result.Value.(error); ok && err != nil {
		return err
	}
	return core.E("resultError", "unexpected core result failure", nil)
}

func repeatString(s string, count int) string {
	if s == "" || count <= 0 {
		return ""
	}
	return string(bytes.Repeat([]byte(s), count))
}

func containsAny(s, chars string) bool {
	for _, ch := range chars {
		if bytes.IndexRune([]byte(s), ch) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, substr string) int {
	return bytes.Index([]byte(s), []byte(substr))
}

func trimQuotes(s string) string {
	if len(s) < 2 {
		return s
	}
	if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '`' && s[len(s)-1] == '`') {
		return s[1 : len(s)-1]
	}
	return s
}
