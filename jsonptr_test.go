// Copyright 2016-2020 Olivier Mengué. All rights reserved.
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
//
// The partially deserialized forms are omitted for the root pointer as Get
// would return them as-is, which is not comparable with the expected value.
func docForms(t *testing.T, jsonData string, ptr string) []interface{} {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		t.Fatalf("Can't unmarshal %v: %s\n", jsonData, err)
	}
	docs := []interface{}{
		data,
		json.RawMessage(jsonData),
		json.NewDecoder(strings.NewReader(jsonData)),
	}
	if ptr == "" {
		return docs
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
	t.Logf("%v => \"%v\"", jsonData, ptr)

	for _, doc := range docForms(t, jsonData, ptr) {
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

// checkGetError checks that tester.Get fails with a PtrError wrapping
// expectedErr located at expectedPtr, for the same document given in all the
// forms returned by docForms.
func (tester *getTester) checkGetError(jsonData string, ptr string, expectedErr error, expectedPtr string) {
	t := tester.t
	t.Logf("%v => \"%v\" (error expected)", jsonData, ptr)

	for _, doc := range docForms(t, jsonData, ptr) {
		got, err := tester.Get(doc, ptr)
		if err == nil {
			t.Errorf("  %T: unexpected success: got %T %v", doc, got, got)
			continue
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("  %T: got %T %q, want %q", doc, err, err, expectedErr)
			continue
		}
		var perr *jsonptr.PtrError
		if !errors.As(err, &perr) {
			t.Errorf("  %T: got %T %q, want *PtrError", doc, err, err)
			continue
		}
		if perr.Ptr != expectedPtr {
			t.Errorf("  %T: error located at %q, want %q", doc, perr.Ptr, expectedPtr)
		}
	}
}

func (tester *getTester) runTest() {
	t := tester.t
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
		// Partially deserialized containers are returned as-is when they are the leaf
		{`/a`, mixed["a"]},
		{`/c`, mixed["c"]},
		{`/e/0`, mixed["e"].([]interface{})[0]},
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
}

func TestGet(t *testing.T) {
	(&getTester{
		t:   t,
		Get: jsonptr.Get,
	}).runTest()
}

func checkSet(t *testing.T, data interface{}, ptr string, value interface{}, jsonOut string) {
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
}

func checkDelete(t *testing.T, data interface{}, ptr string, expectedValue interface{}, jsonOut string) {
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
	checkDelete(t, map[string]interface{}{"a": 1, "b": 2}, `/a`, 1, `{"b":2}`)
	checkDelete(t, []interface{}{1, 2, 3}, `/1`, 2, `[1,3]`)
	checkDelete(t, []interface{}{1, 2, 3}, `/2`, 3, `[1,2]`)
	checkDelete(t, []interface{}{1}, `/0`, 1, `[]`)
	checkDelete(t, map[string]interface{}{"a": []interface{}{1, 2}}, `/a/0`, 1, `{"a":[2]}`)

	// json.RawMessage in the tree, at root or nested
	checkDelete(t, json.RawMessage(`{"a":1,"b":2}`), `/a`, float64(1), `{"b":2}`)
	checkDelete(t, json.RawMessage(`[1,2,3]`), `/1`, float64(2), `[1,3]`)
	checkDelete(t, map[string]interface{}{"a": json.RawMessage(`{"b":1,"c":2}`)}, `/a/b`, float64(1), `{"a":{"c":2}}`)
	checkDelete(t, map[string]interface{}{"a": json.RawMessage(`[1,2,3]`)}, `/a/1`, float64(2), `{"a":[1,3]}`)
	checkDelete(t, map[string]interface{}{"a": json.RawMessage(`{"b":{"c":[true]}}`)}, `/a/b/c/0`, true, `{"a":{"b":{"c":[]}}}`)
	checkDelete(t, map[string]interface{}{"a": json.NewDecoder(strings.NewReader(`{"b":1,"c":2}`))}, `/a/b`, float64(1), `{"a":{"c":2}}`)

	// Errors
	for _, test := range []struct {
		doc interface{}
		ptr string
		err error
	}{
		{map[string]interface{}{}, ``, jsonptr.ErrDeleteRoot},
		{map[string]interface{}{}, `a`, jsonptr.ErrSyntax},
		{map[string]interface{}{}, `/a`, jsonptr.ErrProperty},
		{map[string]interface{}{"a": 1}, `/a/b`, nil}, // DocumentError
		{[]interface{}{1}, `/1`, jsonptr.ErrIndex},
		{[]interface{}{1}, `/-`, jsonptr.ErrIndex},
		{[]interface{}{1}, `/x`, jsonptr.ErrSyntax},
		{map[string]interface{}{"a": json.RawMessage(`{`)}, `/a/b`, nil}, // DocumentError
	} {
		doc := test.doc
		_, err := jsonptr.Delete(&doc, test.ptr)
		if err == nil {
			t.Errorf("%#v - %q: expected error", test.doc, test.ptr)
			continue
		}
		if test.err == nil {
			var docErr *jsonptr.DocumentError
			if !errors.As(err, &docErr) {
				t.Errorf("%#v - %q: got %T %v, want *DocumentError", test.doc, test.ptr, err, err)
			}
		} else if !errors.Is(err, test.err) {
			t.Errorf("%#v - %q: got %T %v, want %v", test.doc, test.ptr, err, err, test.err)
		}
	}
}
