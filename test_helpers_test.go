// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"reflect"
	"testing"

	core "dappco.re/go/core"
)

func testContext(msgAndArgs []any) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	return core.Sprintf("%v: ", msgAndArgs[0])
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func isEmpty(v any) bool {
	if isNil(v) {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() == 0
	default:
		return rv.IsZero()
	}
}

func valueLen(v any) (int, bool) {
	if v == nil {
		return 0, true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len(), true
	default:
		return 0, false
	}
}

func requireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf("%sunexpected error: %v", testContext(msgAndArgs), err)
	}
}

func requireError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		t.Fatalf("%sexpected error, got nil", testContext(msgAndArgs))
	}
}

func requireEqual(t *testing.T, want, got any, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%swant %v, got %v", testContext(msgAndArgs), want, got)
	}
}

func requireTrue(t *testing.T, cond bool, msgAndArgs ...any) {
	t.Helper()
	if !cond {
		t.Fatalf("%sexpected true", testContext(msgAndArgs))
	}
}

func requireNotNil(t *testing.T, v any, msgAndArgs ...any) {
	t.Helper()
	if isNil(v) {
		t.Fatalf("%sexpected non-nil", testContext(msgAndArgs))
	}
}

func requireLen(t *testing.T, v any, want int, msgAndArgs ...any) {
	t.Helper()
	got, ok := valueLen(v)
	if !ok {
		t.Fatalf("%sexpected value with length, got %T", testContext(msgAndArgs), v)
	}
	if want != got {
		t.Fatalf("%swant length %v, got %v", testContext(msgAndArgs), want, got)
	}
}

func assertEqual(t *testing.T, want, got any, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%swant %v, got %v", testContext(msgAndArgs), want, got)
	}
}

func assertTrue(t *testing.T, cond bool, msgAndArgs ...any) {
	t.Helper()
	if !cond {
		t.Fatalf("%sexpected true", testContext(msgAndArgs))
	}
}

func assertFalse(t *testing.T, cond bool, msgAndArgs ...any) {
	t.Helper()
	if cond {
		t.Fatalf("%sexpected false", testContext(msgAndArgs))
	}
}

func assertNil(t *testing.T, v any, msgAndArgs ...any) {
	t.Helper()
	if !isNil(v) {
		t.Fatalf("%sexpected nil, got %v", testContext(msgAndArgs), v)
	}
}

func assertNotNil(t *testing.T, v any, msgAndArgs ...any) {
	t.Helper()
	if isNil(v) {
		t.Fatalf("%sexpected non-nil", testContext(msgAndArgs))
	}
}

func assertEmpty(t *testing.T, v any, msgAndArgs ...any) {
	t.Helper()
	if !isEmpty(v) {
		t.Fatalf("%sexpected empty, got %v", testContext(msgAndArgs), v)
	}
}

func assertLen(t *testing.T, v any, want int, msgAndArgs ...any) {
	t.Helper()
	got, ok := valueLen(v)
	if !ok {
		t.Fatalf("%sexpected value with length, got %T", testContext(msgAndArgs), v)
	}
	if want != got {
		t.Fatalf("%swant length %v, got %v", testContext(msgAndArgs), want, got)
	}
}

func assertContains(t *testing.T, s, substr string, msgAndArgs ...any) {
	t.Helper()
	if !core.Contains(s, substr) {
		t.Fatalf("%sexpected %q to contain %q", testContext(msgAndArgs), s, substr)
	}
}

func assertNotContains(t *testing.T, s, substr string, msgAndArgs ...any) {
	t.Helper()
	if core.Contains(s, substr) {
		t.Fatalf("%sexpected %q not to contain %q", testContext(msgAndArgs), s, substr)
	}
}

func assertInDelta(t *testing.T, want, got, delta float64, msgAndArgs ...any) {
	t.Helper()
	diff := want - got
	if diff < 0 {
		diff = -diff
	}
	if diff > delta {
		t.Fatalf("%swant %v within %v, got %v", testContext(msgAndArgs), want, delta, got)
	}
}
