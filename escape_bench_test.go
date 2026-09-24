// Copyright 2026 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

//go:build go1.27
// +build go1.27

package jsonptr_test

import (
	"encoding/json/jsontext"
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

// unescapeJSONText unescapes a token with [jsontext.Pointer.LastToken], by
// building the single token pointer which designates it.
//
// It reports no error: invalid tokens are out of its scope, so it is only
// comparable with the other implementations on valid input.
func unescapeJSONText(token string) (string, error) {
	return jsontext.Pointer("/" + token).LastToken(), nil
}

// BenchmarkUnescapeString compares [jsonptr.UnescapeString] with the
// reference implementation used by the fuzz tests and with the standard
// library.
func BenchmarkUnescapeString(b *testing.B) {
	implementations := [...]struct {
		name     string
		unescape func(string) (string, error)
	}{
		{"jsonptr.UnescapeString", jsonptr.UnescapeString},
		{"reference", unescape},
		{"jsontext.Pointer.LastToken", unescapeJSONText},
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
