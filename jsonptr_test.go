// Copyright 2016-2026 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

package jsonptr_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dolmen-go/jsonptr"
)

var _ error = (*jsonptr.PtrError)(nil)

type getTester struct {
	t   *testing.T
	Get func(interface{}, string) (interface{}, error)
}

// docForms returns the same JSON document in all the forms accepted by Get:
// fully deserialized, raw, streamed, and partially deserialized
// (map[string]json.RawMessage or []json.RawMessage) when the document is an
// object or an array.
func docForms(t *testing.T, jsonData string) []interface{} {
	t.Helper()

	var data interface{}
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		t.Fatalf("Can't unmarshal %v: %s\n", jsonData, err)
	}
	docs := []interface{}{
		data,
		json.RawMessage(jsonData),
		json.NewDecoder(strings.NewReader(jsonData)),
	}
	switch data.(type) {
	case map[string]interface{}:
		var partial map[string]json.RawMessage
		if err := json.Unmarshal([]byte(jsonData), &partial); err != nil {
			t.Fatalf("Can't unmarshal %v: %s\n", jsonData, err)
		}
		docs = append(docs, partial)
	case []interface{}:
		var partial []json.RawMessage
		if err := json.Unmarshal([]byte(jsonData), &partial); err != nil {
			t.Fatalf("Can't unmarshal %v: %s\n", jsonData, err)
		}
		docs = append(docs, partial)
	}
	return docs
}

func (tester *getTester) checkGet(jsonData string, ptr string, expected interface{}) {
	t := tester.t
	t.Helper()
	t.Logf("%v => \"%v\"", jsonData, ptr)

	for _, doc := range docForms(t, jsonData) {
		got, err := tester.Get(doc, ptr)
		if err != nil {
			t.Errorf("  %T: unexpected error: %s", doc, err)
			continue
		}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("  %T: result error!\n  expected: %T %v\n       got: %T %v", doc, expected, expected, got, got)
		}
	}
}

// checkGetError checks that tester.Get fails with an error wrapping
// expectedErr located at expectedPtr, for the same document given in all the
// forms returned by docForms.
//
// The type of the error is expected from expectedErr: a BadPointerError for
// ErrSyntax, a DocumentError for nil, a PtrError otherwise.
func (tester *getTester) checkGetError(jsonData string, ptr string, expectedErr error, expectedPtr string) {
	t := tester.t
	t.Helper()
	t.Logf("%v => \"%v\" (error expected)", jsonData, ptr)

	for _, doc := range docForms(t, jsonData) {
		got, err := tester.Get(doc, ptr)
		if err == nil {
			t.Errorf("  %T: unexpected success: got %T %v", doc, got, got)
			continue
		}
		if expectedErr != nil && !errors.Is(err, expectedErr) {
			t.Errorf("  %T: got %T %q, want %q", doc, err, err, expectedErr)
			continue
		}
		var loc string
		var badErr *jsonptr.BadPointerError
		var docErr *jsonptr.DocumentError
		var ptrErr *jsonptr.PtrError
		switch {
		case expectedErr == nil:
			if !errors.As(err, &docErr) {
				t.Errorf("  %T: got %T %q, want *DocumentError", doc, err, err)
				continue
			}
			loc = docErr.Ptr
		case expectedErr == jsonptr.ErrSyntax:
			if !errors.As(err, &badErr) {
				t.Errorf("  %T: got %T %q, want *BadPointerError", doc, err, err)
				continue
			}
			loc = badErr.BadPtr
		default:
			if !errors.As(err, &ptrErr) {
				t.Errorf("  %T: got %T %q, want *PtrError", doc, err, err)
				continue
			}
			loc = ptrErr.Ptr
		}
		if loc != expectedPtr {
			t.Errorf("  %T: error located at %q, want %q", doc, loc, expectedPtr)
		}
	}
}

func (tester *getTester) runTest() {
	t := tester.t
	t.Parallel()

	for _, doc := range []interface{}{
		"x",
		1,
		1.0,
		true,
		false,
		nil,
		[]interface{}{},
		map[string]interface{}{},
	} {
		got, err := tester.Get(doc, "")
		if err != nil {
			t.Logf("%T: unexpected error: %T %q\n", doc, err, err)
			t.Fail()
			continue
		}
		if !reflect.DeepEqual(got, doc) {
			t.Logf("%T: different response\n", doc)
			t.Fail()
		}
	}

	tester.checkGet(`"x"`, ``, "x")
	tester.checkGet(`["x"]`, ``, []interface{}{"x"})
	// Nested containers at root: the partially deserialized forms are fully expanded
	tester.checkGet(`{"a":[1,{"b":null}],"c":{}}`, ``, map[string]interface{}{
		"a": []interface{}{float64(1), map[string]interface{}{"b": nil}},
		"c": map[string]interface{}{},
	})
	tester.checkGet(`[[],{"a":[true]}]`, ``, []interface{}{
		[]interface{}{},
		map[string]interface{}{"a": []interface{}{true}},
	})
	tester.checkGet(`{"a":{"b":[[1]]}}`, `/a`, map[string]interface{}{"b": []interface{}{[]interface{}{float64(1)}}})
	// Nil partially deserialized containers are expanded as nil containers
	// Note that a nil partially deserialized container is not something that is expected in input.
	// It it handled in getLeaf only for completeness.
	for _, test := range []struct {
		doc      interface{}
		ptr      string
		expected interface{}
	}{
		{[]json.RawMessage(nil), ``, []interface{}(nil)},
		{map[string]json.RawMessage(nil), ``, map[string]interface{}(nil)},
		{map[string]interface{}{"a": []json.RawMessage(nil)}, `/a`, []interface{}(nil)},
		{[]interface{}{map[string]json.RawMessage(nil)}, `/0`, map[string]interface{}(nil)},
	} {
		got, err := tester.Get(test.doc, test.ptr)
		if err != nil {
			t.Errorf("%#v => %q: unexpected error: %v", test.doc, test.ptr, err)
		} else if !reflect.DeepEqual(got, test.expected) {
			t.Errorf("%#v => %q: got %#v, want %#v", test.doc, test.ptr, got, test.expected)
		}
	}
	tester.checkGet(`["a","b"]`, `/0`, "a")
	tester.checkGet(`["a","b"]`, `/1`, "b")
	tester.checkGet(`{"a":"x"}`, `/a`, "x")
	tester.checkGet(`{"":"x"}`, `/`, "x")
	tester.checkGet(`{"":{"":{"":true}}}`, `///`, true)
	tester.checkGet(`{"~":"x"}`, `/~0`, "x")
	tester.checkGet(`{"/":"y"}`, `/~1`, "y")
	tester.checkGet(`{"~/":"z"}`, `/~0~1`, "z")
	tester.checkGet(`{"/~":"z"}`, `/~1~0`, "z")
	tester.checkGet(`{"~~~":"x"}`, `/~0~0~0`, "x")
	tester.checkGet(`{"~x~":"x"}`, `/~0x~0`, "x")
	tester.checkGet(`{"/~~/":"z"}`, `/~1~0~0~1`, "z")
	tester.checkGet(`{"1éé":"z"}`, `/1éé`, "z")
	tester.checkGet(`{"a":{}}`, `/a`, map[string]interface{}{})
	tester.checkGet(`{"a":[]}`, `/a`, []interface{}{})
	tester.checkGet(`{"a":[1,2]}`, `/a/0`, float64(1))
	tester.checkGet(`{"a":[1,2]}`, `/a/1`, float64(2))
	tester.checkGet(`{"b":null,"a":[1,2]}`, `/a/1`, float64(2))
	tester.checkGet(`{"a":[0,1,2,3,4,5,6,7,8,9,"x"]}`, `/a/10`, "x")

	// Property not found
	tester.checkGetError(`{}`, `/a`, jsonptr.ErrProperty, `/a`)
	tester.checkGetError(`{"b":1}`, `/a`, jsonptr.ErrProperty, `/a`)
	tester.checkGetError(`{"b":{"a":1},"c":[1,2],"d":null}`, `/a`, jsonptr.ErrProperty, `/a`)
	tester.checkGetError(`{"a":{"b":1}}`, `/a/c`, jsonptr.ErrProperty, `/a/c`)
	tester.checkGetError(`{"a":{"b":1}}`, `/b/a`, jsonptr.ErrProperty, `/b`)
	tester.checkGetError(`{"a":[{"b":1}]}`, `/a/0/c`, jsonptr.ErrProperty, `/a/0/c`)
	tester.checkGetError(`{"~":1}`, `/~1`, jsonptr.ErrProperty, `/~1`)
	// Index out of range
	tester.checkGetError(`[]`, `/0`, jsonptr.ErrIndex, `/0`)
	tester.checkGetError(`[1,2]`, `/2`, jsonptr.ErrIndex, `/2`)
	tester.checkGetError(`[1,2]`, `/-`, jsonptr.ErrIndex, `/-`)
	tester.checkGetError(`{"a":[[1],[2]]}`, `/a/1/1`, jsonptr.ErrIndex, `/a/1/1`)
	tester.checkGetError(`{"a":[[1],[2]]}`, `/a/2/0`, jsonptr.ErrIndex, `/a/2`)
	// Not an index: a navigation error, like a missing property
	tester.checkGetError(`[1]`, `/x`, jsonptr.ErrIndex, `/x`)
	tester.checkGetError(`[1]`, `/`, jsonptr.ErrIndex, `/`)
	tester.checkGetError(`[1]`, `/01`, jsonptr.ErrIndex, `/01`)
	tester.checkGetError(`[1]`, `/-1`, jsonptr.ErrIndex, `/-1`)
	tester.checkGetError(`[1]`, `/1e0`, jsonptr.ErrIndex, `/1e0`)
	tester.checkGetError(`[1]`, `/99999999999999999999`, jsonptr.ErrIndex, `/99999999999999999999`)
	tester.checkGetError(`{"a":[1]}`, `/a/x/y`, jsonptr.ErrIndex, `/a/x`)
	tester.checkGetError(`{"a":[[1]]}`, `/a/0/x`, jsonptr.ErrIndex, `/a/0/x`)
	// But a bad escape is a syntax error, whatever the container
	tester.checkGetError(`[1]`, `/~2`, jsonptr.ErrSyntax, `/~2`)
	tester.checkGetError(`{"a":[1]}`, `/a/~/x`, jsonptr.ErrSyntax, `/a/~`)
	// Invalid escape: the error is located at the bad token
	tester.checkGetError(`{"a":{"b":1}}`, `/a/~2`, jsonptr.ErrSyntax, `/a/~2`)
	tester.checkGetError(`{"a":{"b":1}}`, `/a/~2/x`, jsonptr.ErrSyntax, `/a/~2`)
	tester.checkGetError(`{"a":{"b":{"c":1}}}`, `/a/b/~/x`, jsonptr.ErrSyntax, `/a/b/~`)
	// Not an object or array: the error is located at the value which can't
	// be traversed
	tester.checkGetError(`1`, `/x`, nil, ``)
	tester.checkGetError(`"str"`, `/0/x`, nil, ``)
	tester.checkGetError(`{"s":1}`, `/s/x`, nil, `/s`)
	tester.checkGetError(`{"a":{"s":1}}`, `/a/s/x`, nil, `/a/s`)
	tester.checkGetError(`{"a":{"s":1}}`, `/a/s/x/y`, nil, `/a/s`)
	tester.checkGetError(`{"a":null}`, `/a/x`, nil, `/a`)
	tester.checkGetError(`{"a":[1]}`, `/a/0/x`, nil, `/a/0`)
	tester.checkGetError(`[{"s":"str"}]`, `/0/s/0`, nil, `/0/s`)
	tester.checkGetError(`{"a":[{"s":true}]}`, `/a/0/s/0`, nil, `/a/0/s`)

	// Partially deserialized containers mixed at various levels
	mixed := map[string]interface{}{
		"a": []json.RawMessage{
			json.RawMessage(`{"b":[1,2]}`),
			json.RawMessage(`"x"`),
		},
		"c": map[string]json.RawMessage{
			"d": json.RawMessage(`[true]`),
			"~": json.RawMessage(`null`),
		},
		"e": []interface{}{
			map[string]json.RawMessage{"f": json.RawMessage(`{"g":3}`)},
		},
	}
	for _, test := range []struct {
		ptr      string
		expected interface{}
	}{
		{`/a/0/b/1`, float64(2)},
		{`/a/1`, "x"},
		{`/c/d/0`, true},
		{`/c/~0`, nil},
		{`/e/0/f/g`, float64(3)},
		{`/e/0/f`, map[string]interface{}{"g": float64(3)}},
		// Partially deserialized containers are returned expanded when they are the leaf
		{`/a`, []interface{}{
			map[string]interface{}{"b": []interface{}{float64(1), float64(2)}},
			"x",
		}},
		{`/c`, map[string]interface{}{"d": []interface{}{true}, "~": nil}},
		{`/e/0`, map[string]interface{}{"f": map[string]interface{}{"g": float64(3)}}},
	} {
		t.Logf("mixed => %q", test.ptr)
		got, err := tester.Get(mixed, test.ptr)
		if err != nil {
			t.Errorf("  unexpected error: %s", err)
			continue
		}
		if !reflect.DeepEqual(got, test.expected) {
			t.Errorf("  expected: %T %v\n       got: %T %v", test.expected, test.expected, got, got)
		}
	}
	for _, test := range []struct {
		ptr string
		err error
		loc string
	}{
		{`/a/2`, jsonptr.ErrIndex, `/a/2`},
		{`/a/-`, jsonptr.ErrIndex, `/a/-`},
		{`/a/0/x`, jsonptr.ErrProperty, `/a/0/x`},
		{`/c/x`, jsonptr.ErrProperty, `/c/x`},
		{`/c/d/1`, jsonptr.ErrIndex, `/c/d/1`},
		{`/e/0/x`, jsonptr.ErrProperty, `/e/0/x`},
	} {
		t.Logf("mixed => %q (error expected)", test.ptr)
		_, err := tester.Get(mixed, test.ptr)
		var perr *jsonptr.PtrError
		if !errors.As(err, &perr) || !errors.Is(err, test.err) {
			t.Errorf("  got %T %v, want *PtrError %v", err, err, test.err)
			continue
		}
		if perr.Ptr != test.loc {
			t.Errorf("  error located at %q, want %q", perr.Ptr, test.loc)
		}
	}

	// Invalid JSON inside a partially deserialized container returned as the
	// leaf: the error is located at the invalid member
	for _, test := range []struct {
		doc interface{}
		ptr string
		loc string
	}{
		{[]json.RawMessage{json.RawMessage(`{`)}, ``, `/0`},
		{[]json.RawMessage{json.RawMessage(`1`), json.RawMessage(`[`)}, ``, `/1`},
		{map[string]json.RawMessage{"a": json.RawMessage(`x`)}, ``, `/a`},
		{map[string]json.RawMessage{"a/b~": json.RawMessage(``)}, ``, `/a~1b~0`},
		// Nested: the location is relative to the document
		{map[string]interface{}{"a": []json.RawMessage{json.RawMessage(`{`)}}, `/a`, `/a/0`},
		{[]json.RawMessage{json.RawMessage(`[{"b":1},[`)}, ``, `/0`},
		{[]interface{}{map[string]json.RawMessage{"b": json.RawMessage(`[1,`)}}, `/0`, `/0/b`},
	} {
		t.Logf("invalid member: %#v => %q", test.doc, test.ptr)
		_, err := tester.Get(test.doc, test.ptr)
		var docErr *jsonptr.DocumentError
		if !errors.As(err, &docErr) {
			t.Errorf("  got %T %v, want *DocumentError", err, err)
			continue
		}
		if docErr.Ptr != test.loc {
			t.Errorf("  error located at %q, want %q", docErr.Ptr, test.loc)
		}
		var synErr *json.SyntaxError
		if !errors.As(err, &synErr) {
			t.Errorf("  got %T %v, want a wrapped *json.SyntaxError", err, err)
		}
	}
}

func TestGet(t *testing.T) {
	(&getTester{
		t:   t,
		Get: jsonptr.Get,
	}).runTest()
}

func checkSet(t *testing.T, data interface{}, ptr string, value interface{}, jsonOut string) {
	t.Helper()

	if jsonIn, isString := data.(string); isString {
		// Same test with input converted to a RawMessage
		checkSet(t, json.RawMessage(jsonIn), ptr, value, jsonOut)

		t.Logf("%v + \"%v\" \"%v\"", jsonIn, ptr, value)
		if err := json.Unmarshal([]byte(jsonIn), &data); err != nil {
			t.Logf("Can't unmarshal %v: %s\n", jsonIn, err)
			t.Fail()
			return
		}
	} else {
		t.Logf("%#v + \"%v\" \"%v\"", data, ptr, value)
	}

	err := jsonptr.Set(&data, ptr, value)
	if err != nil {
		t.Logf("  unexpected error: %s\n", err)
		t.Fail()
		return
	}
	out, err := json.Marshal(data)
	if err != nil {
		t.Logf("  can't marshal output: %s\n", err)
		t.Fail()
		return
	}
	// Try exact matching
	if string(out) == jsonOut {
		return
	}
	// Else unmarshal and compare with DeepEqual
	var expectedData interface{}
	if err := json.Unmarshal([]byte(jsonOut), &expectedData); err != nil {
		t.Logf("Can't unmarshal %v: %s\n", expectedData, err)
		t.Fail()
		return
	}

	if !reflect.DeepEqual(data, expectedData) {
		t.Logf("Result error!\n  expected: %s\n       got: %s\n",
			jsonOut, string(out))
		t.Fail()
	}
}

func TestSet(t *testing.T) {
	t.Parallel()

	checkSet(t, `null`, ``, "x", `"x"`)
	checkSet(t, `null`, ``, 1, `1`)
	checkSet(t, `null`, ``, []interface{}{}, `[]`)
	checkSet(t, `null`, ``, map[string]interface{}{}, `{}`)
	checkSet(t, `[]`, ``, nil, `null`)
	checkSet(t, `{}`, ``, nil, `null`)
	// TODO more tests

	checkSet(t, `[null]`, `/0`, nil, `[null]`)
	checkSet(t, `[null]`, `/0`, true, `[true]`)
	// Appending
	checkSet(t, `[]`, `/-`, true, `[true]`)
	checkSet(t, []interface{}{}, `/-`, true, `[true]`)
	checkSet(t, []interface{}(nil), `/-`, true, `[true]`)
	checkSet(t, `[]`, `/0`, true, `[true]`)
	checkSet(t, `[]`, `/1`, true, `[null,true]`)
	checkSet(t, `[]`, `/2`, true, `[null,null,true]`)
	checkSet(t, []interface{}(nil), `/2`, true, `[null,null,true]`)
	checkSet(t, `[0,1]`, `/2`, true, `[0,1,true]`)
	checkSet(t, `[0,1]`, `/-`, true, `[0,1,true]`)
	checkSet(t, `{}`, `/ok`, true, `{"ok":true}`)
	checkSet(t, `{"x":[]}`, `/x/-`, true, `{"x":[true]}`)

	// json.RawMessage nested in the tree: it must be decoded and stored in
	// the tree before being modified
	checkSet(t, map[string]interface{}{"a": json.RawMessage(`{}`)}, `/a/b`, 1, `{"a":{"b":1}}`)
	checkSet(t, map[string]interface{}{"a": json.RawMessage(`{"x":{"y":[]}}`)}, `/a/x/y/-`, true, `{"a":{"x":{"y":[true]}}}`)
	checkSet(t, map[string]interface{}{"a": json.RawMessage(`[]`)}, `/a/-`, true, `{"a":[true]}`)
	checkSet(t, []interface{}{json.RawMessage(`[1]`)}, `/0/-`, 2, `[[1,2]]`)
	checkSet(t, []interface{}{json.RawMessage(`[1]`)}, `/0/0`, 2, `[[2]]`)
	checkSet(t, map[string]interface{}{"a": json.RawMessage(`{"b":{"c":1}}`)}, `/a/b/c`, 2, `{"a":{"b":{"c":2}}}`)
	// Replacing the RawMessage itself
	checkSet(t, map[string]interface{}{"a": json.RawMessage(`{}`)}, `/a`, 1, `{"a":1}`)
	// Nested streamed decoder
	checkSet(t, map[string]interface{}{"a": json.NewDecoder(strings.NewReader(`{"x":[1]}`))}, `/a/x/-`, 2, `{"a":{"x":[1,2]}}`)
	// Nested in a RawMessage root
	checkSet(t, `{"a":{"b":[]}}`, `/a/b/0`, true, `{"a":{"b":[true]}}`)

	// Invalid JSON in a nested RawMessage
	doc := interface{}(map[string]interface{}{"a": json.RawMessage(`{`)})
	err := jsonptr.Set(&doc, `/a/b`, 1)
	var docErr *jsonptr.DocumentError
	if !errors.As(err, &docErr) {
		t.Errorf("invalid nested JSON: got %T %v, want *DocumentError", err, err)
	}

	// Partially deserialized containers as direct parent: the value is stored
	// as a json.RawMessage
	checkSet(t, map[string]json.RawMessage{}, `/a`, 1, `{"a":1}`)
	checkSet(t, map[string]json.RawMessage(nil), `/a`, 1, `{"a":1}`)
	checkSet(t, map[string]json.RawMessage{"a": json.RawMessage(`1`)}, `/a`, "x", `{"a":"x"}`)
	checkSet(t, map[string]json.RawMessage{"a": json.RawMessage(`1`)}, `/~1`, []int{1}, `{"/":[1],"a":1}`)
	checkSet(t, map[string]json.RawMessage{}, `/a`, json.RawMessage(`{"x":1}`), `{"a":{"x":1}}`)
	checkSet(t, map[string]json.RawMessage{}, `/a`, json.NewDecoder(strings.NewReader(`[1]`)), `{"a":[1]}`)
	checkSet(t, []json.RawMessage{}, `/-`, true, `[true]`)
	checkSet(t, []json.RawMessage(nil), `/-`, true, `[true]`)
	checkSet(t, []json.RawMessage{json.RawMessage(`1`)}, `/0`, "x", `["x"]`)
	checkSet(t, []json.RawMessage{json.RawMessage(`1`)}, `/-`, 2, `[1,2]`)
	checkSet(t, []json.RawMessage{json.RawMessage(`1`)}, `/3`, 2, `[1,null,null,2]`)
	checkSet(t, []json.RawMessage{}, `/0`, json.RawMessage(`{}`), `[{}]`)
	checkSet(t, map[string]interface{}{"a": []json.RawMessage{}}, `/a/-`, true, `{"a":[true]}`)
	checkSet(t, map[string]interface{}{"a": map[string]json.RawMessage{}}, `/a/b`, nil, `{"a":{"b":null}}`)
	// Partially deserialized containers traversed on the path
	checkSet(t, map[string]json.RawMessage{"a": json.RawMessage(`{"b":1}`)}, `/a/b`, 2, `{"a":{"b":2}}`)
	checkSet(t, map[string]json.RawMessage{"a": json.RawMessage(`{"b":1}`), "c": json.RawMessage(`[]`)}, `/c/-`, 2, `{"a":{"b":1},"c":[2]}`)
	checkSet(t, []json.RawMessage{json.RawMessage(`[1]`), json.RawMessage(`{}`)}, `/0/-`, 2, `[[1,2],{}]`)
	checkSet(t, []json.RawMessage{json.RawMessage(`[1]`), json.RawMessage(`{}`)}, `/1/x`, 2, `[[1],{"x":2}]`)
	checkSet(t, map[string]json.RawMessage{"a": json.RawMessage(`[{"b":[]}]`)}, `/a/0/b/-`, 2, `{"a":[{"b":[2]}]}`)
	checkSet(t, map[string]interface{}{"a": map[string]json.RawMessage{"b": json.RawMessage(`[1]`)}}, `/a/b/-`, 2, `{"a":{"b":[1,2]}}`)
	checkSet(t, []interface{}{[]json.RawMessage{json.RawMessage(`{"x":1}`)}}, `/0/0/x`, 2, `[[{"x":2}]]`)

	// Shape of the tree after Set: every container on the path (including a
	// partially deserialized one, and the direct parent) ends up as a
	// map[string]interface{} or []interface{}, the untouched entries are kept
	// raw, and the value is stored as-is (a JSONDecoder value is drained into
	// a json.RawMessage)
	for _, test := range []struct {
		doc      interface{}
		ptr      string
		value    interface{}
		expected interface{}
	}{
		// Partially deserialized direct parent
		{
			map[string]json.RawMessage{"b": json.RawMessage(`2`)}, `/a`, 1,
			map[string]interface{}{"a": 1, "b": json.RawMessage(`2`)},
		},
		{
			[]json.RawMessage{json.RawMessage(`1`)}, `/-`, 2,
			[]interface{}{json.RawMessage(`1`), 2},
		},
		// Partially deserialized container traversed on the path
		{
			map[string]json.RawMessage{"a": json.RawMessage(`{}`), "b": json.RawMessage(`{"c":1}`)}, `/a/x`, 1,
			map[string]interface{}{
				"a": map[string]interface{}{"x": 1},
				"b": json.RawMessage(`{"c":1}`),
			},
		},
		// Lazy decoding of raw documents: only the containers on the path are
		// decoded, one layer each
		{
			json.RawMessage(`{"a":{"b":1},"c":{"d":2}}`), `/a/x`, 3,
			map[string]interface{}{
				"a": map[string]interface{}{"b": json.RawMessage(`1`), "x": 3},
				"c": json.RawMessage(`{"d":2}`),
			},
		},
		{
			json.RawMessage(`[[1],[2]]`), `/0/-`, 9,
			[]interface{}{
				[]interface{}{json.RawMessage(`1`), 9},
				json.RawMessage(`[2]`),
			},
		},
		{
			json.RawMessage(`{"a":[{"b":{"c":1},"d":2},{"e":3}],"f":4}`), `/a/0/b/x`, true,
			map[string]interface{}{
				"a": []interface{}{
					map[string]interface{}{
						"b": map[string]interface{}{"c": json.RawMessage(`1`), "x": true},
						"d": json.RawMessage(`2`),
					},
					json.RawMessage(`{"e":3}`),
				},
				"f": json.RawMessage(`4`),
			},
		},
		// Raw direct parent
		{
			json.RawMessage(`{"a":{"b":1}}`), `/x`, 2,
			map[string]interface{}{"a": json.RawMessage(`{"b":1}`), "x": 2},
		},
		{
			json.RawMessage(`[[1]]`), `/-`, 2,
			[]interface{}{json.RawMessage(`[1]`), 2},
		},
		// Leading whitespace
		{
			json.RawMessage(" \n\t{\"a\":1}"), `/b`, 2,
			map[string]interface{}{"a": json.RawMessage(`1`), "b": 2},
		},
		// Streamed document
		{
			json.NewDecoder(strings.NewReader(`{"a":{"b":1},"c":2}`)), `/a/x`, 3,
			map[string]interface{}{
				"a": map[string]interface{}{"b": json.RawMessage(`1`), "x": 3},
				"c": json.RawMessage(`2`),
			},
		},
		// Nested in a tree
		{
			map[string]interface{}{"a": json.RawMessage(`{"b":{},"c":1}`)}, `/a/b/x`, 2,
			map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{"x": 2},
					"c": json.RawMessage(`1`),
				},
			},
		},
		// Values: a json.RawMessage is stored as-is, a JSONDecoder is drained
		// into a json.RawMessage (both at root and deeper)
		{
			map[string]interface{}{}, `/a`, json.RawMessage(`{"x":1}`),
			map[string]interface{}{"a": json.RawMessage(`{"x":1}`)},
		},
		{
			map[string]interface{}{}, ``, json.RawMessage(`{"x":1}`),
			json.RawMessage(`{"x":1}`),
		},
		{
			map[string]interface{}{}, `/a`, json.NewDecoder(strings.NewReader(`{"x":1} 2`)),
			map[string]interface{}{"a": json.RawMessage(`{"x":1}`)},
		},
		{
			map[string]interface{}{}, ``, json.NewDecoder(strings.NewReader(`[1] 2`)),
			json.RawMessage(`[1]`),
		},
	} {
		t.Logf("shape: %s + %q", jsonptr.MustValue(json.Marshal(test.doc)), test.ptr)
		doc := test.doc
		if err := jsonptr.Set(&doc, test.ptr, test.value); err != nil {
			t.Errorf("  unexpected error: %v", err)
		} else if !reflect.DeepEqual(doc, test.expected) {
			t.Errorf("  got  %#v\n  want %#v", doc, test.expected)
		}
	}
	// A raw scalar can't be traversed; the document is left untouched on error
	doc = json.RawMessage(` 1`)
	if err := jsonptr.Set(&doc, `/a`, 1); !errors.As(err, &docErr) {
		t.Errorf("raw scalar: got %T %v, want *DocumentError", err, err)
	} else if !reflect.DeepEqual(doc, json.RawMessage(` 1`)) {
		t.Errorf("raw scalar: got %#v", doc)
	}
	// On error, a JSONDecoder on the path that has been read is replaced in
	// the tree by the raw value read, so the document stays usable
	for _, test := range []struct {
		doc      interface{}
		ptr      string
		expected interface{} // document after the failed Set
		get      string      // a pointer through the decoder, that must still resolve
	}{
		{
			map[string]interface{}{"a": json.NewDecoder(strings.NewReader(`{"b":1} "next"`))}, `/a/x/y`,
			map[string]interface{}{"a": json.RawMessage(`{"b":1}`)}, `/a/b`,
		},
		{
			[]interface{}{json.NewDecoder(strings.NewReader(`[1] "next"`))}, `/0/5/y`,
			[]interface{}{json.RawMessage(`[1]`)}, `/0/0`,
		},
		// Failure below the decoder, in a lazily decoded layer
		{
			map[string]interface{}{"a": json.NewDecoder(strings.NewReader(`{"b":{"c":1},"d":2}`))}, `/a/b/c/x`,
			map[string]interface{}{"a": json.RawMessage(`{"b":{"c":1},"d":2}`)}, `/a/b/c`,
		},
		// Decoder at the root
		{
			json.NewDecoder(strings.NewReader(`{"b":1}`)), `/x/y`,
			json.RawMessage(`{"b":1}`), `/b`,
		},
	} {
		t.Logf("failed Set through a JSONDecoder: %q", test.ptr)
		doc := test.doc
		if err := jsonptr.Set(&doc, test.ptr, 1); err == nil {
			t.Errorf("  expected error")
		} else if !reflect.DeepEqual(doc, test.expected) {
			t.Errorf("  got  %#v\n  want %#v", doc, test.expected)
		}
		// The document is still usable
		if _, err := jsonptr.Get(doc, test.get); err != nil {
			t.Errorf("  Get %q after failed Set: unexpected error: %v", test.get, err)
		}
	}

	// An invalid JSONDecoder value is rejected, the document is left untouched
	doc = map[string]interface{}{}
	if err := jsonptr.Set(&doc, `/a`, json.NewDecoder(strings.NewReader(`{`))); !errors.As(err, &docErr) {
		t.Errorf("invalid JSONDecoder value: got %T %v, want *DocumentError", err, err)
	} else if !reflect.DeepEqual(doc, map[string]interface{}{}) {
		t.Errorf("invalid JSONDecoder value: got %#v", doc)
	}

	// Navigation errors on the path to the parent
	for _, test := range []struct {
		doc   interface{}
		ptr   string
		value interface{}
		err   error
		loc   string // location reported in the error (not checked if empty)
	}{
		{map[string]interface{}{}, `a`, 1, jsonptr.ErrSyntax, `a`},
		{map[string]interface{}{}, `/~2/x`, 1, jsonptr.ErrSyntax, `/~2`},
		{map[string]interface{}{"a": map[string]interface{}{}}, `/a/~2/x`, 1, jsonptr.ErrSyntax, `/a/~2`},
		{map[string]interface{}{"a": map[string]interface{}{}}, `/a/~2`, 1, jsonptr.ErrSyntax, `/a/~2`},
		{map[string]interface{}{}, `/a/x`, 1, jsonptr.ErrProperty, `/a`},
		{map[string]interface{}{"a": 1}, `/a/x`, 1, nil, `/a`}, // DocumentError
		{[]interface{}{}, `/x/y`, 1, jsonptr.ErrIndex, `/x`},
		{[]interface{}{[]interface{}{}}, `/0/x/y`, 1, jsonptr.ErrIndex, `/0/x`},
		{[]interface{}{[]interface{}{}}, `/0/~2/y`, 1, jsonptr.ErrSyntax, `/0/~2`},
		{[]interface{}{}, `/~`, 1, jsonptr.ErrSyntax, `/~`},
		{[]interface{}{}, `/0/y`, 1, jsonptr.ErrIndex, `/0`},
		{[]interface{}{}, `/-/y`, 1, jsonptr.ErrIndex, `/-`},
		{[]interface{}{1}, `/0/y`, 1, nil, `/0`}, // DocumentError
		{map[string]json.RawMessage{}, `/~2/x`, 1, jsonptr.ErrSyntax, `/~2`},
		{map[string]json.RawMessage{"a": json.RawMessage(`{}`)}, `/a/~2/x`, 1, jsonptr.ErrSyntax, `/a/~2`},
		{[]json.RawMessage{}, `/x/y`, 1, jsonptr.ErrIndex, `/x`},
		{[]json.RawMessage{json.RawMessage(`[]`)}, `/0/x/y`, 1, jsonptr.ErrIndex, `/0/x`},
		{json.NewDecoder(strings.NewReader(`{`)), `/a`, 1, nil, ``},     // DocumentError
		{json.NewDecoder(strings.NewReader(`[x]`)), `/0/a`, 1, nil, ``}, // DocumentError
		// Errors with partially deserialized containers
		{map[string]json.RawMessage{}, `/~2`, 1, jsonptr.ErrSyntax, `/~2`},
		{map[string]json.RawMessage{}, `/a/b`, 1, jsonptr.ErrProperty, `/a`},
		{map[string]json.RawMessage{"a": json.RawMessage(`{`)}, `/a/b`, 1, nil, `/a`}, // DocumentError
		{map[string]json.RawMessage{"a": json.RawMessage(`[`)}, `/a/-`, 1, nil, `/a`}, // DocumentError
		{map[string]json.RawMessage{"a": json.RawMessage(`x`)}, `/a/b`, 1, nil, `/a`}, // DocumentError
		{map[string]json.RawMessage{"a": json.RawMessage(``)}, `/a/b`, 1, nil, `/a`},  // DocumentError
		{[]json.RawMessage{}, `/x`, 1, jsonptr.ErrIndex, `/x`},
		{[]json.RawMessage{}, `/0/b`, 1, jsonptr.ErrIndex, `/0`},
		{[]json.RawMessage{json.RawMessage(`1`)}, `/0/b`, 1, nil, `/0`}, // DocumentError
	} {
		doc := test.doc
		err := jsonptr.Set(&doc, test.ptr, test.value)
		if err == nil {
			t.Errorf("%#v + %q: expected error", test.doc, test.ptr)
			continue
		}
		var loc string
		var docErr *jsonptr.DocumentError
		var badErr *jsonptr.BadPointerError
		var ptrErr *jsonptr.PtrError
		if test.err == nil {
			if !errors.As(err, &docErr) {
				t.Errorf("%#v + %q: got %T %v, want *DocumentError", test.doc, test.ptr, err, err)
				continue
			}
			loc = docErr.Ptr
		} else {
			if !errors.Is(err, test.err) {
				t.Errorf("%#v + %q: got %T %v, want %v", test.doc, test.ptr, err, err, test.err)
				continue
			}
			switch {
			case errors.As(err, &badErr):
				loc = badErr.BadPtr
			case errors.As(err, &ptrErr):
				loc = ptrErr.Ptr
			}
		}
		if loc != test.loc {
			t.Errorf("%#v + %q: error located at %q, want %q", test.doc, test.ptr, loc, test.loc)
		}
	}
}

func checkDelete(t *testing.T, data interface{}, ptr string, expectedValue interface{}, jsonOut string) {
	t.Helper()

	t.Logf("%#v - \"%v\"", data, ptr)
	got, err := jsonptr.Delete(&data, ptr)
	if err != nil {
		t.Errorf("  unexpected error: %s", err)
		return
	}
	if !reflect.DeepEqual(got, expectedValue) {
		t.Errorf("  deleted value: got %T %v, want %T %v", got, got, expectedValue, expectedValue)
	}
	out, err := json.Marshal(data)
	if err != nil {
		t.Errorf("  can't marshal output: %s", err)
		return
	}
	if string(out) != jsonOut {
		t.Errorf("  got %s, want %s", out, jsonOut)
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	checkDelete(t, map[string]interface{}{"a": 1, "b": 2}, `/a`, 1, `{"b":2}`)
	checkDelete(t, []interface{}{1, 2, 3}, `/1`, 2, `[1,3]`)
	checkDelete(t, []interface{}{1, 2, 3}, `/2`, 3, `[1,2]`)
	checkDelete(t, []interface{}{1}, `/0`, 1, `[]`)
	checkDelete(t, map[string]interface{}{"a": []interface{}{1, 2}}, `/a/0`, 1, `{"a":[2]}`)

	// json.RawMessage in the tree, at root or nested: the container is decoded
	// lazily, so the deleted value is returned raw
	checkDelete(t, json.RawMessage(`{"a":1,"b":2}`), `/a`, json.RawMessage(`1`), `{"b":2}`)
	checkDelete(t, json.RawMessage(`[1,2,3]`), `/1`, json.RawMessage(`2`), `[1,3]`)
	checkDelete(t, map[string]interface{}{"a": json.RawMessage(`{"b":1,"c":2}`)}, `/a/b`, json.RawMessage(`1`), `{"a":{"c":2}}`)
	checkDelete(t, map[string]interface{}{"a": json.RawMessage(`[1,2,3]`)}, `/a/1`, json.RawMessage(`2`), `{"a":[1,3]}`)
	checkDelete(t, map[string]interface{}{"a": json.RawMessage(`{"b":{"c":[true]}}`)}, `/a/b/c/0`, json.RawMessage(`true`), `{"a":{"b":{"c":[]}}}`)
	checkDelete(t, map[string]interface{}{"a": json.NewDecoder(strings.NewReader(`{"b":1,"c":2}`))}, `/a/b`, json.RawMessage(`1`), `{"a":{"c":2}}`)

	// Lazy decoding: the tree shape after a Delete through a raw document
	doc := interface{}(json.RawMessage(`{"a":{"b":1,"c":2},"d":{"e":3}}`))
	if _, err := jsonptr.Delete(&doc, `/a/b`); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if !reflect.DeepEqual(doc, map[string]interface{}{
		"a": map[string]json.RawMessage{"c": json.RawMessage(`2`)},
		"d": json.RawMessage(`{"e":3}`),
	}) {
		t.Errorf("got %#v", doc)
	}

	// Partially deserialized containers as direct parent: the raw value is
	// returned and the container type is preserved
	checkDelete(t, map[string]json.RawMessage{"a": json.RawMessage(`1`), "b": json.RawMessage(`2`)}, `/a`, json.RawMessage(`1`), `{"b":2}`)
	checkDelete(t, map[string]json.RawMessage{"/": json.RawMessage(`1`), "b": json.RawMessage(`2`)}, `/~1`, json.RawMessage(`1`), `{"b":2}`)
	checkDelete(t, []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`), json.RawMessage(`3`)}, `/1`, json.RawMessage(`2`), `[1,3]`)
	checkDelete(t, []json.RawMessage{json.RawMessage(`1`)}, `/0`, json.RawMessage(`1`), `[]`)
	checkDelete(t, map[string]interface{}{"a": []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`)}}, `/a/0`, json.RawMessage(`1`), `{"a":[2]}`)
	checkDelete(t, []interface{}{map[string]json.RawMessage{"a": json.RawMessage(`1`)}}, `/0/a`, json.RawMessage(`1`), `[{}]`)
	// Partially deserialized containers traversed on the path
	checkDelete(t, map[string]json.RawMessage{"a": json.RawMessage(`[1,2]`), "b": json.RawMessage(`{}`)}, `/a/0`, json.RawMessage(`1`), `{"a":[2],"b":{}}`)
	checkDelete(t, map[string]json.RawMessage{"a": json.RawMessage(`{"b":1,"c":2}`)}, `/a/b`, json.RawMessage(`1`), `{"a":{"c":2}}`)
	checkDelete(t, []json.RawMessage{json.RawMessage(`[1,2]`), json.RawMessage(`{}`)}, `/0/1`, json.RawMessage(`2`), `[[1],{}]`)
	checkDelete(t, []json.RawMessage{json.RawMessage(`{"a":[true]}`)}, `/0/a/0`, json.RawMessage(`true`), `[{"a":[]}]`)

	doc = interface{}([]json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`)})
	if _, err := jsonptr.Delete(&doc, `/0`); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if !reflect.DeepEqual(doc, []json.RawMessage{json.RawMessage(`2`)}) {
		t.Errorf("got %#v", doc)
	}

	// On error, a JSONDecoder on the path that has been read is replaced in
	// the tree by the raw value read, so the document stays usable
	for _, test := range []struct {
		doc      interface{}
		ptr      string
		expected interface{} // document after the failed Delete
		get      string      // a pointer through the decoder, that must still resolve
	}{
		{
			map[string]interface{}{"a": json.NewDecoder(strings.NewReader(`{"b":1} "next"`))}, `/a/x/y`,
			map[string]interface{}{"a": json.RawMessage(`{"b":1}`)}, `/a/b`,
		},
		{
			[]interface{}{json.NewDecoder(strings.NewReader(`[1] "next"`))}, `/0/5`,
			[]interface{}{json.RawMessage(`[1]`)}, `/0/0`,
		},
		{
			map[string]interface{}{"a": json.NewDecoder(strings.NewReader(`{"b":{"c":1},"d":2}`))}, `/a/b/c/x`,
			map[string]interface{}{"a": json.RawMessage(`{"b":{"c":1},"d":2}`)}, `/a/b/c`,
		},
		{
			json.NewDecoder(strings.NewReader(`{"b":1}`)), `/x`,
			json.RawMessage(`{"b":1}`), `/b`,
		},
	} {
		t.Logf("failed Delete through a JSONDecoder: %q", test.ptr)
		doc := test.doc
		if _, err := jsonptr.Delete(&doc, test.ptr); err == nil {
			t.Errorf("  expected error")
		} else if !reflect.DeepEqual(doc, test.expected) {
			t.Errorf("  got  %#v\n  want %#v", doc, test.expected)
		}
		if _, err := jsonptr.Get(doc, test.get); err != nil {
			t.Errorf("  Get %q after failed Delete: unexpected error: %v", test.get, err)
		}
	}

	// Errors
	for _, test := range []struct {
		doc interface{}
		ptr string
		err error
		loc string // location reported in the error (not checked if empty)
	}{
		{map[string]interface{}{}, ``, jsonptr.ErrDeleteRoot, ``},
		{map[string]interface{}{}, `a`, jsonptr.ErrSyntax, `a`},
		{map[string]interface{}{}, `/a`, jsonptr.ErrProperty, `/a`},
		{map[string]interface{}{}, `/a/b`, jsonptr.ErrProperty, `/a`},
		{map[string]interface{}{"a": map[string]interface{}{}}, `/a/b/c`, jsonptr.ErrProperty, `/a/b`},
		{map[string]interface{}{"a": map[string]interface{}{}}, `/a/~2/c`, jsonptr.ErrSyntax, `/a/~2`},
		{map[string]interface{}{"a": 1}, `/a/b`, nil, `/a`}, // DocumentError
		{[]interface{}{1}, `/1`, jsonptr.ErrIndex, `/1`},
		{[]interface{}{1}, `/-`, jsonptr.ErrIndex, `/-`},
		{[]interface{}{1}, `/1/x`, jsonptr.ErrIndex, `/1`},
		{[]interface{}{1}, `/-/x`, jsonptr.ErrIndex, `/-`},
		{[]interface{}{1}, `/x`, jsonptr.ErrIndex, `/x`},
		{[]interface{}{[]interface{}{}}, `/0/x/y`, jsonptr.ErrIndex, `/0/x`},
		{[]interface{}{[]interface{}{}}, `/0/~2/y`, jsonptr.ErrSyntax, `/0/~2`},
		{[]interface{}{1}, `/~`, jsonptr.ErrSyntax, `/~`},
		{map[string]interface{}{"a": json.RawMessage(`{`)}, `/a/b`, nil, `/a`}, // DocumentError
		{json.RawMessage(`{`), `/a`, nil, ``},                                  // DocumentError
		{json.NewDecoder(strings.NewReader(`[x]`)), `/0/a`, nil, ``},           // DocumentError
		{map[string]json.RawMessage{}, `/a`, jsonptr.ErrProperty, `/a`},
		{map[string]json.RawMessage{}, `/~2`, jsonptr.ErrSyntax, `/~2`},
		{map[string]json.RawMessage{}, `/a/b`, jsonptr.ErrProperty, `/a`},
		{map[string]json.RawMessage{"a": json.RawMessage(`{}`)}, `/a/~2/c`, jsonptr.ErrSyntax, `/a/~2`},
		{map[string]json.RawMessage{"a": json.RawMessage(`{`)}, `/a/b`, nil, `/a`}, // DocumentError
		{[]json.RawMessage{json.RawMessage(`1`)}, `/1`, jsonptr.ErrIndex, `/1`},
		{[]json.RawMessage{json.RawMessage(`1`)}, `/-`, jsonptr.ErrIndex, `/-`},
		{[]json.RawMessage{json.RawMessage(`1`)}, `/x`, jsonptr.ErrIndex, `/x`},
		{[]json.RawMessage{json.RawMessage(`1`)}, `/1/x`, jsonptr.ErrIndex, `/1`},
		{[]json.RawMessage{json.RawMessage(`[]`)}, `/0/x/y`, jsonptr.ErrIndex, `/0/x`},
		{[]json.RawMessage{json.RawMessage(`1`)}, `/0/x`, nil, `/0`}, // DocumentError
	} {
		doc := test.doc
		_, err := jsonptr.Delete(&doc, test.ptr)
		if err == nil {
			t.Errorf("%#v - %q: expected error", test.doc, test.ptr)
			continue
		}
		var loc string
		var docErr *jsonptr.DocumentError
		var badErr *jsonptr.BadPointerError
		var ptrErr *jsonptr.PtrError
		if test.err == nil {
			if !errors.As(err, &docErr) {
				t.Errorf("%#v - %q: got %T %v, want *DocumentError", test.doc, test.ptr, err, err)
				continue
			}
			loc = docErr.Ptr
		} else {
			if !errors.Is(err, test.err) {
				t.Errorf("%#v - %q: got %T %v, want %v", test.doc, test.ptr, err, err, test.err)
				continue
			}
			// ErrSyntax and ErrDeleteRoot are reported by a BadPointerError,
			// ErrProperty and ErrIndex by a PtrError
			switch {
			case errors.As(err, &badErr):
				if test.err != jsonptr.ErrSyntax && test.err != jsonptr.ErrDeleteRoot {
					t.Errorf("%#v - %q: got %T %v, want *PtrError", test.doc, test.ptr, err, err)
					continue
				}
				loc = badErr.BadPtr
			case errors.As(err, &ptrErr):
				if test.err != jsonptr.ErrProperty && test.err != jsonptr.ErrIndex {
					t.Errorf("%#v - %q: got %T %v, want *BadPointerError", test.doc, test.ptr, err, err)
					continue
				}
				loc = ptrErr.Ptr
			}
		}
		if loc != test.loc {
			t.Errorf("%#v - %q: error located at %q, want %q", test.doc, test.ptr, loc, test.loc)
		}
	}
}
