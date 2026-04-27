// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"reflect"
	"testing"

	core "dappco.re/go/core"
)

// testContext supports the session test suite.
func testContext(msgAndArgs []any) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	return core.Sprintf("%v: ", msgAndArgs[0])
}

// isNil supports the session test suite.
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

// isEmpty supports the session test suite.
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

// valueLen supports the session test suite.
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

// requireNoError stops the current test case when its condition is not met.
func requireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf("%sunexpected error: %v", testContext(msgAndArgs), err)
	}
}

// requireError stops the current test case when its condition is not met.
func requireError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		t.Fatalf("%sexpected error, got nil", testContext(msgAndArgs))
	}
}

// requireEqual stops the current test case when its condition is not met.
func requireEqual(t *testing.T, want, got any, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%swant %v, got %v", testContext(msgAndArgs), want, got)
	}
}

// requireTrue stops the current test case when its condition is not met.
func requireTrue(t *testing.T, cond bool, msgAndArgs ...any) {
	t.Helper()
	if !cond {
		t.Fatalf("%sexpected true", testContext(msgAndArgs))
	}
}

// requireNotNil stops the current test case when its condition is not met.
func requireNotNil(t *testing.T, v any, msgAndArgs ...any) {
	t.Helper()
	if isNil(v) {
		t.Fatalf("%sexpected non-nil", testContext(msgAndArgs))
	}
}

// requireLen stops the current test case when its condition is not met.
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

// assertEqual records a test failure when its condition is not met.
func assertEqual(t *testing.T, want, got any, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%swant %v, got %v", testContext(msgAndArgs), want, got)
	}
}

// assertTrue records a test failure when its condition is not met.
func assertTrue(t *testing.T, cond bool, msgAndArgs ...any) {
	t.Helper()
	if !cond {
		t.Errorf("%sexpected true", testContext(msgAndArgs))
	}
}

// assertFalse records a test failure when its condition is not met.
func assertFalse(t *testing.T, cond bool, msgAndArgs ...any) {
	t.Helper()
	if cond {
		t.Errorf("%sexpected false", testContext(msgAndArgs))
	}
}

// assertNil records a test failure when its condition is not met.
func assertNil(t *testing.T, v any, msgAndArgs ...any) {
	t.Helper()
	if !isNil(v) {
		t.Errorf("%sexpected nil, got %v", testContext(msgAndArgs), v)
	}
}

// assertNotNil records a test failure when its condition is not met.
func assertNotNil(t *testing.T, v any, msgAndArgs ...any) {
	t.Helper()
	if isNil(v) {
		t.Errorf("%sexpected non-nil", testContext(msgAndArgs))
	}
}

// assertEmpty records a test failure when its condition is not met.
func assertEmpty(t *testing.T, v any, msgAndArgs ...any) {
	t.Helper()
	if !isEmpty(v) {
		t.Errorf("%sexpected empty, got %v", testContext(msgAndArgs), v)
	}
}

// assertLen records a test failure when its condition is not met.
func assertLen(t *testing.T, v any, want int, msgAndArgs ...any) {
	t.Helper()
	got, ok := valueLen(v)
	if !ok {
		t.Errorf("%sexpected value with length, got %T", testContext(msgAndArgs), v)
		return
	}
	if want != got {
		t.Errorf("%swant length %v, got %v", testContext(msgAndArgs), want, got)
	}
}

// assertContains records a test failure when its condition is not met.
func assertContains(t *testing.T, s, substr string, msgAndArgs ...any) {
	t.Helper()
	if !core.Contains(s, substr) {
		t.Errorf("%sexpected %q to contain %q", testContext(msgAndArgs), s, substr)
	}
}

// assertNotContains records a test failure when its condition is not met.
func assertNotContains(t *testing.T, s, substr string, msgAndArgs ...any) {
	t.Helper()
	if core.Contains(s, substr) {
		t.Errorf("%sexpected %q not to contain %q", testContext(msgAndArgs), s, substr)
	}
}

// assertInDelta records a test failure when its condition is not met.
func assertInDelta(t *testing.T, want, got, delta float64, msgAndArgs ...any) {
	t.Helper()
	diff := want - got
	if diff < 0 {
		diff = -diff
	}
	if diff > delta {
		t.Errorf("%swant %v within %v, got %v", testContext(msgAndArgs), want, delta, got)
	}
}
