// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"testing"

	core "dappco.re/go"
)

// --- sessionCore / hostContext / hostProcess ---

func TestCoreHelpers_sessionCore_Good(t *testing.T) {
	c := core.New()

	got := sessionCore(c)

	core.AssertNotNil(t, got)
	core.AssertEqual(t, c, got)
}

func TestCoreHelpers_sessionCore_Bad(t *testing.T) {
	got := sessionCore(nil)

	core.AssertNotNil(t, got)
}

func TestCoreHelpers_sessionCore_Ugly(t *testing.T) {
	first := sessionCore(nil)
	second := sessionCore(nil)

	core.AssertEqual(t, first, second)
}

func TestCoreHelpers_hostContext_Good(t *testing.T) {
	ctx := hostContext(core.New())

	core.AssertNotNil(t, ctx)
}

func TestCoreHelpers_hostContext_Bad(t *testing.T) {
	ctx := hostContext(nil)

	core.AssertNotNil(t, ctx)
}

func TestCoreHelpers_hostContext_Ugly(t *testing.T) {
	a := hostContext(nil)
	b := hostContext(nil)

	core.AssertEqual(t, a, b)
}

func TestCoreHelpers_hostProcess_Good(t *testing.T) {
	p := hostProcess(core.New())

	core.AssertNotNil(t, p)
}

func TestCoreHelpers_hostProcess_Bad(t *testing.T) {
	p := hostProcess(nil)

	core.AssertNotNil(t, p)
}

func TestCoreHelpers_hostProcess_Ugly(t *testing.T) {
	a := hostProcess(nil)
	b := hostProcess(nil)

	core.AssertEqual(t, a, b)
}

// --- rawjson.UnmarshalJSON / MarshalJSON ---

func TestCoreHelpers_rawjson_UnmarshalJSON_Good(t *testing.T) {
	var m rawjson

	err := m.UnmarshalJSON([]byte(`{"a":1}`))

	core.RequireNoError(t, err)
	core.AssertEqual(t, `{"a":1}`, string(m))
}

func TestCoreHelpers_rawjson_UnmarshalJSON_Bad(t *testing.T) {
	var m *rawjson

	err := m.UnmarshalJSON([]byte(`{}`))

	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "nil receiver")
}

func TestCoreHelpers_rawjson_UnmarshalJSON_Ugly(t *testing.T) {
	m := rawjson("old-content-to-be-overwritten")

	err := m.UnmarshalJSON([]byte(`[]`))

	core.RequireNoError(t, err)
	core.AssertEqual(t, `[]`, string(m))
}

func TestCoreHelpers_rawjson_MarshalJSON_Good(t *testing.T) {
	m := rawjson(`{"k":"v"}`)

	out, err := m.MarshalJSON()

	core.RequireNoError(t, err)
	core.AssertEqual(t, `{"k":"v"}`, string(out))
}

func TestCoreHelpers_rawjson_MarshalJSON_Bad(t *testing.T) {
	var m rawjson

	out, err := m.MarshalJSON()

	core.RequireNoError(t, err)
	core.AssertEqual(t, "null", string(out))
}

func TestCoreHelpers_rawjson_MarshalJSON_Ugly(t *testing.T) {
	m := rawjson{}

	out, err := m.MarshalJSON()

	core.RequireNoError(t, err)
	// A non-nil but empty slice round-trips as empty bytes, not null.
	core.AssertEqual(t, "", string(out))
}

// --- resultError ---

func TestCoreHelpers_resultError_Good(t *testing.T) {
	err := resultError(core.Ok(nil))

	core.AssertNoError(t, err)
}

func TestCoreHelpers_resultError_Bad(t *testing.T) {
	cause := core.E("scope", "boom", nil)

	err := resultError(core.Fail(cause))

	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "boom")
}

func TestCoreHelpers_resultError_Ugly(t *testing.T) {
	// A failed result whose Value is not an error must still produce one.
	err := resultError(core.Result{Value: "not-an-error", OK: false})

	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "unexpected core result failure")
}

// --- repeatString ---

func TestCoreHelpers_repeatString_Good(t *testing.T) {
	core.AssertEqual(t, "ababab", repeatString("ab", 3))
}

func TestCoreHelpers_repeatString_Bad(t *testing.T) {
	core.AssertEqual(t, "", repeatString("", 5))
}

func TestCoreHelpers_repeatString_Ugly(t *testing.T) {
	core.AssertEqual(t, "", repeatString("x", 0))
	core.AssertEqual(t, "", repeatString("x", -3))
}

// --- containsAny ---

func TestCoreHelpers_containsAny_Good(t *testing.T) {
	core.AssertTrue(t, containsAny("a/b", "/\\"))
}

func TestCoreHelpers_containsAny_Bad(t *testing.T) {
	core.AssertFalse(t, containsAny("abc", "/\\"))
}

func TestCoreHelpers_containsAny_Ugly(t *testing.T) {
	core.AssertFalse(t, containsAny("", "/\\"))
	core.AssertFalse(t, containsAny("abc", ""))
}

// --- indexOf ---

func TestCoreHelpers_indexOf_Good(t *testing.T) {
	core.AssertEqual(t, 7, indexOf("echo a # note", "# note"))
}

func TestCoreHelpers_indexOf_Bad(t *testing.T) {
	core.AssertEqual(t, -1, indexOf("abc", "xyz"))
}

func TestCoreHelpers_indexOf_Ugly(t *testing.T) {
	core.AssertEqual(t, 0, indexOf("abc", ""))
	core.AssertEqual(t, -1, indexOf("ab", "abc"))
}

// --- trimQuotes ---

func TestCoreHelpers_trimQuotes_Good(t *testing.T) {
	core.AssertEqual(t, "value", trimQuotes(`"value"`))
	core.AssertEqual(t, "value", trimQuotes("`value`"))
}

func TestCoreHelpers_trimQuotes_Bad(t *testing.T) {
	core.AssertEqual(t, "value", trimQuotes("value"))
}

func TestCoreHelpers_trimQuotes_Ugly(t *testing.T) {
	core.AssertEqual(t, "x", trimQuotes("x"))
	core.AssertEqual(t, `"mismatched`, trimQuotes(`"mismatched`))
	core.AssertEqual(t, "", trimQuotes(""))
}
