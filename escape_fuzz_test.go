// Copyright 2026 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

//go:build go1.18
// +build go1.18

package jsonptr_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dolmen-go/jsonptr"
)

// names is the seed corpus of property names, used unescaped as well as
// escaped (a name is a valid token only if it contains no '/' and no '~').
var names = [...]string{
	"",
	"a",
	"~",
	"/",
	"~0",
	"~1",
	"~2",
	"~01",
	"~~",
	"//",
	"a~1b",
	"a/b",
	"~~//",
	"a~",
	"~a",
	"é",
	"1éé",
	"\x00\xff",
}

// FuzzEscapeString checks that any property name can be escaped, that the
// result is a valid token, and that unescaping it gives back the name.
func FuzzEscapeString(f *testing.F) {
	for _, name := range names {
		f.Add(name)
	}

	f.Fuzz(func(t *testing.T, name string) {
		esc := jsonptr.EscapeString(name)

		// The escaped name is a single token: no '/', and every '~' is
		// followed by '0' or '1'
		for i := 0; i < len(esc); i++ {
			switch esc[i] {
			case '/':
				t.Fatalf("EscapeString(%q) = %q: contains '/'", name, esc)
			case '~':
				i++
				if i == len(esc) || (esc[i] != '0' && esc[i] != '1') {
					t.Fatalf("EscapeString(%q) = %q: invalid escape at %d", name, esc, i-1)
				}
			}
		}

		// Roundtrip
		back, err := jsonptr.UnescapeString(esc)
		if err != nil {
			t.Fatalf("UnescapeString(%q) (escape of %q): unexpected error: %v", esc, name, err)
		}
		if back != name {
			t.Fatalf("roundtrip of %q through %q gave %q", name, esc, back)
		}

		// Unescape must agree with UnescapeString
		b, err := jsonptr.Unescape([]byte(esc))
		if err != nil || string(b) != name {
			t.Fatalf("Unescape(%q) = %q, %v; want %q, <nil>", esc, b, err, name)
		}

		// A name which is already a valid token is left unchanged
		if _, err := unescape(name); err == nil && !strings.ContainsAny(name, "~/") && esc != name {
			t.Fatalf("EscapeString(%q) = %q: nothing to escape", name, esc)
		}
	})
}

// FuzzAppendEscape checks that AppendEscape appends what EscapeString
// returns, whatever the spare capacity of the destination.
func FuzzAppendEscape(f *testing.F) {
	for _, name := range names {
		f.Add("", name)
		f.Add("/prefix", name)
	}

	f.Fuzz(func(t *testing.T, prefix, name string) {
		want := prefix + jsonptr.EscapeString(name)

		// No spare capacity: AppendEscape has to allocate
		tight := make([]byte, len(prefix))
		copy(tight, prefix)
		if got := string(jsonptr.AppendEscape(tight, name)); got != want {
			t.Fatalf("AppendEscape(%q, %q) = %q, want %q", prefix, name, got, want)
		}

		// Enough spare capacity: AppendEscape writes in place
		large := make([]byte, len(prefix), len(prefix)+3*len(name)+1)
		copy(large, prefix)
		if got := string(jsonptr.AppendEscape(large, name)); got != want {
			t.Fatalf("AppendEscape(%q, %q) with spare cap = %q, want %q", prefix, name, got, want)
		}
	})
}

// FuzzUnescapeString checks UnescapeString and Unescape against the reference
// implementation, for any input: they must never panic, agree with each other,
// and report only ErrSyntax or ErrUsage.
func FuzzUnescapeString(f *testing.F) {
	for _, name := range names {
		f.Add(name)
		f.Add(jsonptr.EscapeString(name))
	}

	f.Fuzz(func(t *testing.T, token string) {
		want, wantErr := unescape(token)

		got, err := jsonptr.UnescapeString(token)
		if (err != nil) != (wantErr != nil) {
			t.Fatalf("UnescapeString(%q) = %q, %v; want %q, %v", token, got, err, want, wantErr)
		}
		if err != nil {
			if !errors.Is(err, jsonptr.ErrSyntax) && !errors.Is(err, jsonptr.ErrUsage) {
				t.Fatalf("UnescapeString(%q): got %v, want ErrSyntax or ErrUsage", token, err)
			}
		} else {
			if got != want {
				t.Fatalf("UnescapeString(%q) = %q, want %q", token, got, want)
			}
			// Escaping back gives the canonical form of the token
			if esc := jsonptr.EscapeString(got); esc != token {
				t.Fatalf("EscapeString(UnescapeString(%q)) = %q", token, esc)
			}
		}

		// Unescape works in place on a copy of the same input, and must
		// behave exactly like UnescapeString
		b, errB := jsonptr.Unescape([]byte(token))
		if (errB != nil) != (err != nil) {
			t.Fatalf("Unescape(%q) = %q, %v; UnescapeString gave %q, %v", token, b, errB, got, err)
		}
		if errB == nil && string(b) != got {
			t.Fatalf("Unescape(%q) = %q; UnescapeString gave %q", token, b, got)
		}
		if errB != nil && !errors.Is(errB, err) && !errors.Is(err, errB) {
			t.Fatalf("Unescape(%q) reported %v; UnescapeString reported %v", token, errB, err)
		}
	})
}
