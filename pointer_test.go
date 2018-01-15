// Copyright 2016-2017 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

package jsonptr_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dolmen-go/jsonptr"
)

var _ fmt.Stringer = jsonptr.Pointer{}

var parseTests = [...]struct {
	in  string
	out jsonptr.Pointer
	err error
}{
	{"", nil, nil},
	{"a", nil, jsonptr.ErrSyntax},
	{"~", nil, jsonptr.ErrSyntax},
	{"/", jsonptr.Pointer{""}, nil},
	{"////", jsonptr.Pointer{"", "", "", ""}, nil},
	{"/a", jsonptr.Pointer{"a"}, nil},
	{"/~", nil, &jsonptr.BadPointerError{"/~", jsonptr.ErrSyntax}},
	{"/~x", nil, &jsonptr.BadPointerError{"/~x", jsonptr.ErrSyntax}},
	{"/a~", nil, &jsonptr.BadPointerError{"/a~", jsonptr.ErrSyntax}},
	{"/a~x", nil, &jsonptr.BadPointerError{"/a~x", jsonptr.ErrSyntax}},
	{"/abc/~", nil, &jsonptr.BadPointerError{"/abc/~", jsonptr.ErrSyntax}},
	{"/abc/~/b", nil, &jsonptr.BadPointerError{"/abc/~", jsonptr.ErrSyntax}},
	{"/abc/~x", nil, &jsonptr.BadPointerError{"/abc/~x", jsonptr.ErrSyntax}},
	{"/abc/a~", nil, &jsonptr.BadPointerError{"/abc/a~", jsonptr.ErrSyntax}},
	{"/~0", jsonptr.Pointer{"~"}, nil},
	{"/~1", jsonptr.Pointer{"/"}, nil},
	{"/~0~0", jsonptr.Pointer{"~~"}, nil},
	{"/~0~1", jsonptr.Pointer{"~/"}, nil},
	{"/~1~0", jsonptr.Pointer{"/~"}, nil},
	{"/~1~1", jsonptr.Pointer{"//"}, nil},
	{"/abc/def~0/ghi", jsonptr.Pointer{"abc", "def~", "ghi"}, nil},
	{"/abc/def~0/g~1hi", jsonptr.Pointer{"abc", "def~", "g/hi"}, nil},
	// Real test cases
	{"/definitions/Location", jsonptr.Pointer{"definitions", "Location"}, nil},
	{"/paths/~1home~1dolmen", jsonptr.Pointer{"paths", "/home/dolmen"}, nil},
	{"/paths/~0dolmen", jsonptr.Pointer{"paths", "~dolmen"}, nil},
}

func TestPointerParse(t *testing.T) {
	targets := [...]struct {
		Parse     func(string) (jsonptr.Pointer, error)
		Stringify func(jsonptr.Pointer) string
	}{
		{
			Parse:     jsonptr.Parse,
			Stringify: func(p jsonptr.Pointer) string { return p.String() },
		},
		{
			Parse: func(s string) (jsonptr.Pointer, error) {
				var p jsonptr.Pointer
				err := p.UnmarshalText([]byte(s))
				return p, err
			},
			Stringify: func(p jsonptr.Pointer) string {
				s, err := p.MarshalText()
				if err != nil {
					panic(fmt.Sprintf("unexpected error %q", err))
				}
				return string(s)
			},
		},
	}

	for _, test := range parseTests {
		if test.err != nil {
			t.Logf("%q => %s", test.in, test.err)
		} else {
			t.Logf("%q => %#v", test.in, test.out)
		}

		for _, target := range targets {
			out, err := target.Parse(test.in)
			if (err == nil) != (test.err == nil) {
				t.Errorf("got error: %q", err)
			} else if (out == nil) != (test.out == nil) || len(out) != len(test.out) {
				t.Errorf("got %#v", out)
			} else if err == nil {
				for i, part := range out {
					if part != test.out[i] {
						t.Errorf("got %#v", out)
						break
					}
				}
				ptr := target.Stringify(out)
				if ptr != test.in {
					t.Errorf("roundtrip failure: got %q != %q", ptr, test.in)
				}
			} else if !reflect.DeepEqual(err, test.err) {
				// TODO fix jsonptr.Parse to match UnmarshalText
				t.Logf("error mismatch: want %T %q, got %T %q",
					test.err, test.err,
					err, err,
				)
			}
		}
	}
}

// AltParse is another implementation of Parse that aimed
// to be faster, but isn't for the most common case (no escape)
// See BenchmarkParse for comparison.
func AltParse(pointer string) (jsonptr.Pointer, error) {
	if pointer == "" {
		return nil, nil
	}
	if pointer[0] != '/' {
		return nil, jsonptr.ErrSyntax
	}
	ptr := strings.Split(pointer[1:], "/")
	p := 1
	i := 0
	for {
		q := p
		for q < len(pointer) {
			if pointer[q] == '~' {
				break
			}
			q++
		}
		if q == len(pointer) {
			break
		}
		for p+len(ptr[i]) < q {
			i++
			p += len(ptr) + 1
		}
		var err error
		ptr[i], err = jsonptr.UnescapeString(ptr[i])
		if err != nil {
			return nil, err
		}
		i++
		if i == len(ptr) {
			break
		}
	}
	return ptr, nil
}

func BenchmarkParse(b *testing.B) {
	implementations := [...]struct {
		name  string
		parse func(string) (jsonptr.Pointer, error)
	}{
		{"jsonptr.Parse", jsonptr.Parse},
		{"AltParse", AltParse},
	}
	for _, test := range parseTests {
		for _, impl := range implementations {
			b.Run(fmt.Sprintf("%q/%s", test.in, impl.name), func(b *testing.B) {
				parse := impl.parse
				ptr := test.in
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _ = parse(ptr)
				}
			})
		}
	}
}

func TestPointer(t *testing.T) {
	var p jsonptr.Pointer
	if !p.IsRoot() {
		t.Fatal("Empty must give a root pointer")
	}
	if p.String() != "" {
		t.Fatal("A root pointer is \"\"")
	}

	// Moves operations over p defined as functions
	var (
		up = p.Up

		property = func(name string) func() *jsonptr.Pointer {
			return func() *jsonptr.Pointer {
				q := p.Property(name)
				if q.LeafName() != name {
					panic("LeafName() mismatch")
				}
				return q
			}
		}

		index = func(i int) func() *jsonptr.Pointer {
			return func() *jsonptr.Pointer {
				q := p.Index(i)
				if j, err := q.LeafIndex(); err != nil || j != i {
					panic("LeafIndex() mismatch")
				}
				return q
			}
		}

		chain = func(moves ...func() *jsonptr.Pointer) func() *jsonptr.Pointer {
			return func() *jsonptr.Pointer {
				q := &p
				for _, m := range moves {
					q = m()
				}
				return q
			}
		}
	)

	for _, test := range []struct {
		move     func() *jsonptr.Pointer
		expected string
	}{
		{property("foo"), "/foo"},
		{property("bar"), "/foo/bar"},
		{index(0), "/foo/bar/0"},
		{up, "/foo/bar"},
		{up, "/foo"},
		{index(4), "/foo/4"},
		{up, "/foo"},
		{property("a/a"), "/foo/a~1a"},
		{chain(up, property("a~a")), "/foo/a~0a"},
		{chain(up, property("~~//")), "/foo/~0~0~1~1"},
		{chain(up, property("~01")), "/foo/~001"},
		{chain(up, up), ""},
	} {
		t.Logf("Moving to \"%s\"", test.expected)

		q := test.move()
		if q != &p {
			t.Errorf("Chaining failure")
		}

		got := p.String()
		if got != test.expected {
			t.Fatalf("got: %s, expected: %s", got, test.expected)
		}
		if (got == "") != p.IsRoot() {
			t.Error("IsRoot failure")
		}

		if !p.IsRoot() {
			got = p.Property(p.Pop()).String()
			if got != test.expected {
				t.Fatalf("Pop+Property => got: %s, expected: %s", got, test.expected)
			}
		}

		r, err := jsonptr.Parse(test.expected)
		if err != nil {
			t.Errorf("Can't parse %s", test.expected)
		} else {
			got = r.String()
			if got != test.expected {
				t.Errorf("got: %s, expected: %s", got, test.expected)
			}
		}

		r = nil
		r = p.Copy()
		if r.String() != test.expected {
			t.Errorf("Clone error: got %s, expected %s", r.String(), test.expected)
		} else {
			r.Property("baz")
			if r.String() != test.expected+"/baz" {
				t.Errorf("Property() failure in clone")
			}
			// Check that p has not changed
			if p.String() != test.expected {
				t.Errorf("Cloning failure: p is altered")
			}
		}
	}
	if !p.IsRoot() {
		t.Fatal("IsRoot failure")
	}
}

func ExamplePointer_conversion() {
	fmt.Printf("%q\n\n", jsonptr.Pointer{"foo", "bar", "a/b", "x~y"})

	for _, ptr := range []jsonptr.Pointer{
		nil,
		{},
		{"a", "b"},
		{"a~/b"},
	} {
		fmt.Printf("%q\n", ptr.String())
	}
	// Output:
	// "/foo/bar/a~1b/x~0y"
	//
	// ""
	// ""
	// "/a/b"
	// "/a~0~1b"
}

func ExamplePointer_navigation() {
	ptr := jsonptr.Pointer{}
	fmt.Printf("%q\n", ptr)

	ptr.Property("foo").Index(3).Property("a/b")
	fmt.Printf("%q\n", ptr.String())

	ptr.Up()
	fmt.Printf("%q\n", ptr)

	ptr.Property("c~d")
	fmt.Printf("%q\n", ptr)

	// Output:
	// ""
	// "/foo/3/a~1b"
	// "/foo/3"
	// "/foo/3/c~0d"
}

func TestPointerClone(t *testing.T) {
	orig := jsonptr.Pointer{"foo"}
	clone := orig.Copy()
	if clone.String() != "/foo" {
		t.Errorf("Failure!")
	}
	orig.Up().Property("bar")
	if clone.String() != "/foo" {
		t.Errorf("Failure!")
	}
}

// TestPointerIn tests Parse() and Pointer.In()
func TestPointerIn(t *testing.T) {
	(&getTester{
		t: t,
		Get: func(doc interface{}, pointer string) (interface{}, error) {
			ptr, err := jsonptr.Parse(pointer)
			if err != nil {
				return nil, err
			}
			return ptr.In(doc)
		},
	}).runTest()
}
