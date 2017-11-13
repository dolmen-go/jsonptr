package jsonptr_test

import (
	"encoding/json"
	"fmt"

	"github.com/dolmen-go/jsonptr"
)

// Example from https://tools.ietf.org/html/rfc6901#section-5
func Example() {
	const JSON = `
{
   "foo": ["bar", "baz"],
   "": 0,
   "a/b": 1,
   "c%d": 2,
   "e^f": 3,
   "g|h": 4,
   "i\\j": 5,
   "k\"l": 6,
   " ": 7,
   "m~n": 8
}`
	var doc interface{}
	_ = json.Unmarshal([]byte(JSON), &doc)

	for _, ptr := range []string{
		"/foo",
		"/foo/0",
		"/",
		"/a~1b",
		"/c%d",
		"/e^f",
		"/g|h",
		"/i\\j",
		"/k\"l",
		"/ ",
		"/m~0n",
	} {
		result, _ := jsonptr.Get(doc, ptr)
		fmt.Printf("%-12q %#v\n", ptr, result)
	}
	// Output:
	// "/foo"       []interface {}{"bar", "baz"}
	// "/foo/0"     "bar"
	// "/"          0
	// "/a~1b"      1
	// "/c%d"       2
	// "/e^f"       3
	// "/g|h"       4
	// "/i\\j"      5
	// "/k\"l"      6
	// "/ "         7
	// "/m~0n"      8
}

func ExampleSet() {

	newArray := func() interface{} {
		return make([]interface{}, 0)
	}

	newObject := func() interface{} {
		return make(map[string]interface{})
	}

	var doc interface{}

	for _, step := range []struct {
		where string
		what  interface{}
	}{
		{"", newObject},
		{"/arr", newArray},
		{"/arr/-", 3},
		{"/arr/-", 2},
		{"/arr/-", 1},
		{"/obj", newObject},
		{"/obj/str", "hello"},
		{"/obj/bool", true},
		{"/arr/-", 0},
		{"/obj/", nil},
	} {
		what := step.what
		if f, isFunc := what.(func() interface{}); isFunc {
			what = f()
		}
		err := jsonptr.Set(&doc, step.where, what)
		if err != nil {
			panic(err)
		}
		//fmt.Printf("%#v\n", doc)
	}

	fmt.Println(func() string { x, _ := json.Marshal(doc); return string(x) }())
	// Output:
	// {"arr":[3,2,1,0],"obj":{"":null,"bool":true,"str":"hello"}}
}
