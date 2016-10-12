package jsonptr

import (
	"fmt"
	"testing"
)

var _ fmt.Stringer = Pointer{}

func TestPointer(t *testing.T) {
	var p Pointer
	if !p.IsRoot() {
		t.Fatal("Empty must give a root pointer")
	}
	if p.String() != "" {
		t.Fatal("A root pointer is \"\"")
	}

	// Moves operations over p defined as functions
	var (
		up = p.Up

		property = func(name string) func() *Pointer {
			return func() *Pointer { return p.Property(name) }
		}

		index = func(i int) func() *Pointer {
			return func() *Pointer { return p.Index(i) }
		}

		chain = func(moves ...func() *Pointer) func() *Pointer {
			return func() *Pointer {
				q := &p
				for _, m := range moves {
					q = m()
				}
				return q
			}
		}
	)

	for _, test := range []struct {
		move     func() *Pointer
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

		r, err := Parse(test.expected)
		if err != nil {
			t.Errorf("Can't parse %s", test.expected)
		} else {
			got = r.String()
			if got != test.expected {
				t.Errorf("got: %s, expected: %s", got, test.expected)
			}
		}

		r = nil
		r = p.Clone()
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
