// Copyright 2016-2017 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

package jsonptr_test

import (
	"encoding/json"
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

func (tester *getTester) checkGet(jsonData string, ptr string, expected interface{}) {
	t := tester.t
	t.Logf("%v => \"%v\"", jsonData, ptr)
	var data interface{}
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		t.Logf("Can't unmarshal %v: %s\n", jsonData, err)
		t.Fail()
		return
	}

	// Get from a deserialized structure
	got, err := tester.Get(data, ptr)
	if err != nil {
		t.Fatalf("  unexpected error: %s\n", err)
		return
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("Result error!\n  expected: %T %v\n       got: %T %v\n", expected, expected, got, got)
		return
	}

	// Get from the raw JSON document
	got, err = tester.Get(json.RawMessage(jsonData), ptr)
	if err != nil {
		t.Fatalf("  unexpected error: %s\n", err)
		return
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("Result error!\n  expected: %T %v\n       got: %T %v\n", expected, expected, got, got)
	}

	// Get from the raw JSON document
	got, err = tester.Get(json.NewDecoder(strings.NewReader(jsonData)), ptr)
	if err != nil {
		t.Fatalf("  unexpected error: %s\n", err)
		return
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("Result error!\n  expected: %T %v\n       got: %T %v\n", expected, expected, got, got)
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
}
