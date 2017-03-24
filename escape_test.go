package jsonptr_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dolmen-go/jsonptr"
)

func ExampleEscapeString() {
	fmt.Println(jsonptr.EscapeString("a~b c/d"))
	// Output:
	// a~0b c~1d
}

func ExampleUnescapeString() {
	s, _ := jsonptr.UnescapeString("a~0b c~1d")
	fmt.Println(s)
	// Output:
	// a~b c/d
}

func ExampleUnescapeString_error() {
	_, err := jsonptr.UnescapeString("a~x")
	fmt.Println("jsonptr.ErrSyntax?", err == jsonptr.ErrSyntax)
	fmt.Println(err)
	// Output:
	// jsonptr.ErrSyntax? true
	// invalid JSON pointer
}

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
		got := jsonptr.EscapeString(tc.in)
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
	benchmarkEscape(b, jsonptr.EscapeString, escapeBenchmarkCases)
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
		{"~", "", jsonptr.ErrSyntax},
		{"~~", "", jsonptr.ErrSyntax},
		{"~a", "", jsonptr.ErrSyntax},
		{"~a ", "", jsonptr.ErrSyntax},
		{"a ~", "", jsonptr.ErrSyntax},
		{"a ~ x", "", jsonptr.ErrSyntax},
		{"a ~0 ~x", "", jsonptr.ErrSyntax},
	} {
		t.Logf("%s => %s", test.in, test.out)
		got, err := jsonptr.UnescapeString(test.in)
		if err != test.err {
			t.Logf("got: %s, expected: %s", err, test.err)
			t.Fail()
		} else if test.err != nil && got != test.out {
			t.Logf("got: %s, expected: %s", got, test.out)
			t.Fail()
		}
	}
}
