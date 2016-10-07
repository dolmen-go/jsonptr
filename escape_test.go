package jsonptr

import (
	"testing"
)

func checkEscape(t *testing.T, in, expected string) {
	got := Escape(in)
	if got != expected {
		t.Errorf("in: %#v\n    got: %#v\nexpected: %#v\n", in, got, expected)
	} else {
		t.Logf("%#v => %#v\n", in, got)
	}
}

func TestEscape(t *testing.T) {
	checkEscape(t, "", "")
	checkEscape(t, "a", "a")
	checkEscape(t, "a~", "a~0")
	checkEscape(t, "~a", "~0a")
	checkEscape(t, "a~b", "a~0b")
	checkEscape(t, "a/", "a~1")
	checkEscape(t, "/a", "~1a")
	checkEscape(t, "a/b", "a~1b")
	checkEscape(t, "a/~b", "a~1~0b")
	checkEscape(t, "a~/b", "a~0~1b")
	checkEscape(t, "a~~~b", "a~0~0~0b")
	checkEscape(t, "a///b", "a~1~1~1b")
	checkEscape(t, "a/b/c/d", "a~1b~1c~1d")
	checkEscape(t, "é~é", "é~0é")
	checkEscape(t, "é~", "é~0")
	checkEscape(t, "~é", "~0é")
}

func TestUnescape(t *testing.T) {
	for _, test := range []struct {
		in, out string
		err     error
	}{
		{"", "", nil},
		{"x", "x", nil},
		{"~0", "~", nil},
		{"~1", "/", nil},
		{"~0~1", "~/", nil},
		{"~1~0", "/~", nil},
		{"x~1~0", "x/~", nil},
		{"x~1y~0", "x/y~", nil},
		{"~", "", ErrSyntax},
		{"~~", "", ErrSyntax},
		{"~a", "", ErrSyntax},
		{"~a ", "", ErrSyntax},
		{"a ~", "", ErrSyntax},
		{"a ~ x", "", ErrSyntax},
		{"a ~0 ~x", "", ErrSyntax},
	} {
		t.Logf("%s => %s", test.in, test.out)
		got, err := Unescape(test.in)
		if err != test.err {
			t.Logf("got: %s, expected: %s", err, test.err)
			t.Fail()
		} else if test.err != nil && got != test.out {
			t.Logf("got: %s, expected: %s", got, test.out)
			t.Fail()
		}
	}
}
