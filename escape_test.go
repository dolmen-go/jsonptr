package jsonptr

import (
	"strings"
	"testing"
)

func TestEscape(t *testing.T) {
	for _, tc := range []struct {
		in, expected string
	}{
		{"", ""},
		{"a", "a"},
		{"a~", "a~0"},
		{"~a", "~0a"},
		{"a~b", "a~0b"},
		{"a/", "a~1"},
		{"/a", "~1a"},
		{"a/b", "a~1b"},
		{"a/~b", "a~1~0b"},
		{"a~/b", "a~0~1b"},
		{"a~~~b", "a~0~0~0b"},
		{"a///b", "a~1~1~1b"},
		{"a/b/c/d", "a~1b~1c~1d"},
		{"é~é", "é~0é"},
		{"é~", "é~0"},
		{"~é", "~0é"},
	} {
		t.Logf("%#v => %#v\n", tc.in, tc.expected)
		got := EscapeString(tc.in)
		if got != tc.expected {
			t.Errorf("got: %#v\n", got)
		}
	}
}

var jsonptrReplacer = strings.NewReplacer(
	"~", "~0",
	"/", "~1",
)

func EscapeWithReplacer(s string) string {
	return jsonptrReplacer.Replace(s)
}

func benchmarkEscape(b *testing.B, escapeFunc func(string) string, testCases []string) {
	for i := 0; i < b.N; i++ {
		for _, s := range testCases {
			_ = escapeFunc(s)
		}
	}
}

var escapeBenchmarkCases = []string{
	"property",
	"name",
	"id",
	"/usr/local/go/pkg/",
	"https://github.com/dolmen-go/jsonptr/",
	"~/.ssh/authorized_keys",
}

func BenchmarkEscape(b *testing.B) {
	benchmarkEscape(b, EscapeString, escapeBenchmarkCases)
}

func BenchmarkEscapeWithReplacer(b *testing.B) {
	benchmarkEscape(b, EscapeWithReplacer, escapeBenchmarkCases)
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
		got, err := UnescapeString(test.in)
		if err != test.err {
			t.Logf("got: %s, expected: %s", err, test.err)
			t.Fail()
		} else if test.err != nil && got != test.out {
			t.Logf("got: %s, expected: %s", got, test.out)
			t.Fail()
		}
	}
}
