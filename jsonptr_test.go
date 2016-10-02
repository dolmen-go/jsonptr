// Copyright 2016 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

package jsonptr

import (
	"encoding/json"
	"reflect"
	"testing"
)

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
	checkGet(t, `{"a":{}}`, `/a`, map[string]interface{}{})
	checkGet(t, `{"a":[]}`, `/a`, []interface{}{})
	checkGet(t, `{"a":[1,2]}`, `/a/0`, float64(1))
	checkGet(t, `{"a":[1,2]}`, `/a/1`, float64(2))
	checkGet(t, `{"a":[0,1,2,3,4,5,6,7,8,9,"x"]}`, `/a/10`, "x")
}
