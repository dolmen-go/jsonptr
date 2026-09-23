// Copyright 2026 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

//go:build go1.27
// +build go1.27

package jsonptr_test

import (
	"strings"
	"testing"

	"github.com/dolmen-go/jsonptr"
)

// unescapeBenchmarks are representative tokens: without escape (the most
// common case), with escapes at various places, and invalid.
var unescapeBenchmarks = [...]struct {
	name  string
	token string
}{
	{"empty", ""},
	{"plain", "definitions"},
	{"plain-long", strings.Repeat("abcdefgh", 8)},
	{"escape-only", "~1"},
	{"escape-first", "~1home~1dolmen"},
	{"escape-last", strings.Repeat("a", 62) + "~1"},
	{"escape-many", strings.Repeat("~1", 32)},
	{"invalid-slash", "a/b"},
	{"invalid-escape", "a~2"},
}

// BenchmarkUnescapeString compares [jsonptr.UnescapeString] with the
// straightforward reference implementation used by the fuzz tests.
func BenchmarkUnescapeString(b *testing.B) {
	implementations := [...]struct {
		name     string
		unescape func(string) (string, error)
	}{
		{"jsonptr.UnescapeString", jsonptr.UnescapeString},
		{"reference", unescape},
	}
	for _, test := range unescapeBenchmarks {
		for _, impl := range implementations {
			b.Run(test.name+"/"+impl.name, func(b *testing.B) {
				f := impl.unescape
				token := test.token
				b.ReportAllocs()
				for b.Loop() {
					_, _ = f(token)
				}
			})
		}
	}
}
