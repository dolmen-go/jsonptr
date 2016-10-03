// Copyright 2016 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

package jsonptr

import (
	"encoding/json"
	"reflect"
	"testing"
)

var _ error = (*PtrError)(nil)

func checkGet(t *testing.T, jsonData string, ptr string, expected interface{}) {
	t.Logf("%v => \"%v\"", jsonData, ptr)
	var data interface{}
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		t.Logf("Can't unmarshal %v: %s\n", jsonData, err)
		t.Fail()
		return
	}

	got, err := Get(data, ptr)
	if err != nil {
		t.Logf("  unexpected error: %s\n", err)
		t.Fail()
		return
	}
	if !reflect.DeepEqual(got, expected) {
		t.Logf("Result error!\n  expected: %T %v\n       got: %T %v\n", expected, expected, got, got)
		t.Fail()
	}
}

func TestGet(t *testing.T) {
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
		got, err := Get(doc, "")
		if err != nil {
			t.Logf("%T: unexpected error: %s\n", doc, err)
			t.Fail()
			continue
		}
		if !reflect.DeepEqual(got, doc) {
			t.Logf("%T: different response\n", doc)
			t.Fail()
		}
	}

	checkGet(t, `"x"`, ``, "x")
	checkGet(t, `["x"]`, ``, []interface{}{"x"})
	checkGet(t, `["a","b"]`, `/0`, "a")
	checkGet(t, `["a","b"]`, `/1`, "b")
	checkGet(t, `{"a":"x"}`, `/a`, "x")
	checkGet(t, `{"":"x"}`, `/`, "x")
	checkGet(t, `{"~":"x"}`, `/~0`, "x")
	checkGet(t, `{"/":"y"}`, `/~1`, "y")
	checkGet(t, `{"~/":"z"}`, `/~0~1`, "z")
	checkGet(t, `{"/~":"z"}`, `/~1~0`, "z")
	checkGet(t, `{"~~~":"x"}`, `/~0~0~0`, "x")
	checkGet(t, `{"~x~":"x"}`, `/~0x~0`, "x")
	checkGet(t, `{"a":{}}`, `/a`, map[string]interface{}{})
	checkGet(t, `{"a":[]}`, `/a`, []interface{}{})
	checkGet(t, `{"a":[1,2]}`, `/a/0`, float64(1))
	checkGet(t, `{"a":[1,2]}`, `/a/1`, float64(2))
	checkGet(t, `{"a":[0,1,2,3,4,5,6,7,8,9,"x"]}`, `/a/10`, "x")
}

func checkSet(t *testing.T, jsonIn string, ptr string, value interface{}, jsonOut string) {
	t.Logf("%v + \"%v\" \"%v\"", jsonIn, ptr, value)
	var data interface{}
	if err := json.Unmarshal([]byte(jsonIn), &data); err != nil {
		t.Logf("Can't unmarshal %v: %s\n", jsonIn, err)
		t.Fail()
		return
	}

	err := Set(&data, ptr, value)
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
	checkSet(t, `[]`, `/0`, true, `[true]`)
	checkSet(t, `{}`, `/ok`, true, `{"ok":true}`)
	checkSet(t, `{"x":[]}`, `/x/-`, true, `{"x":[true]}`)
}
