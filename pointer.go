// Copyright 2016-2017 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

package jsonptr

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// Pointer represents a mutable parsed JSON Pointer.
//
// The Go representation is a array of non-encoded path elements. This
// allows to use type conversion from/to a []string.
type Pointer []string

// Parse parses a JSON pointer from its text representation.
func Parse(pointer string) (Pointer, error) {
	if pointer == "" {
		return nil, nil
	}
	if pointer[0] != '/' {
		return nil, ErrSyntax
	}
	ptr := strings.Split(pointer[1:], "/")
	// Optimize for the common case
	if strings.IndexByte(pointer, '~') == -1 {
		return ptr, nil
	}
	for i, part := range ptr {
		var err error
		if ptr[i], err = UnescapeString(part); err != nil {
			// TODO return the full prefix
			return nil, err
		}
	}
	return ptr, nil
}

// MarshalText() implements encoding.TextUnmarshaler
func (ptr *Pointer) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*ptr = nil
		return nil
	}
	if text[0] != '/' {
		return ErrSyntax
	}

	var p Pointer
	t := text[1:]
	for {
		i := bytes.IndexByte(t, '/')
		if i < 0 {
			break
		}
		part, err := Unescape(t[:i])
		if err != nil {
			return syntaxError(string(text[:len(text)-len(t)-i-1]))
		}
		p = append(p, string(part))
		t = t[i+1:]
	}
	part, err := Unescape(t)
	if err != nil {
		return syntaxError(string(text))
	}
	*ptr = append(p, string(part))
	return nil
}

// String returns a JSON Pointer string, escaping components when necessary:
// '~' is replaced by "~0", '/' by "~1"
func (ptr Pointer) String() string {
	if len(ptr) == 0 {
		return ""
	}

	dst := make([]byte, 0, 8*len(ptr))
	for _, part := range ptr {
		dst = AppendEscape(append(dst, '/'), part)
	}
	return string(dst)
}

// MarshalText() implements encoding.TextMarshaler
func (ptr Pointer) MarshalText() (text []byte, err error) {
	if len(ptr) == 0 {
		return nil, nil
	}
	dst := make([]byte, 0, 8*len(ptr))
	for _, part := range ptr {
		dst = AppendEscape(append(dst, '/'), part)
	}
	return dst, nil
}

// Copy returns a new, independant, copy of the pointer.
func (ptr Pointer) Copy() Pointer {
	return append(Pointer{}, ptr...)
}

// IsRoot returns true if the pointer is at root (empty).
func (ptr Pointer) IsRoot() bool {
	return len(ptr) == 0
}

// Up removes the last element of the pointer.
// The pointer is returned for chaining.
//
// Panics if already at root.
func (ptr *Pointer) Up() *Pointer {
	if ptr.IsRoot() {
		panic(ErrRoot)
	}
	*ptr = (*ptr)[:len(*ptr)-1]
	return ptr
}

// Pop removes the last element of the pointer and returns it.
//
// Panics if already at root.
func (ptr *Pointer) Pop() string {
	last := len(*ptr) - 1
	if last < 0 {
		panic(ErrRoot)
	}
	prop := (*ptr)[last]
	*ptr = (*ptr)[:last]
	return prop
}

// Property moves the pointer deeper, following a property name.
// The pointer is returned for chaining.
func (ptr *Pointer) Property(name string) *Pointer {
	*ptr = append(*ptr, name)
	return ptr
}

// Index moves the pointer deeper, following an array index.
// The pointer is returned for chaining.
func (ptr *Pointer) Index(index int) *Pointer {
	var prop string
	if index < 0 {
		prop = "-"
	} else {
		prop = strconv.Itoa(index)
	}
	return ptr.Property(prop)
}

// LeafName returns the name of the last part of the pointer.
func (ptr Pointer) LeafName() string {
	return ptr[len(ptr)-1]
}

// LeafIndex returns the last part of the pointer as an array index.
// -1 is returned for "-".
func (ptr Pointer) LeafIndex() (int, error) {
	return arrayIndex(ptr.LeafName())
}

// In returns the value from doc pointed by ptr.
//
// doc may be a deserialized document, or a json.RawMessage.
func (ptr Pointer) In(doc interface{}) (interface{}, error) {
	for i, key := range ptr {
		switch here := (doc).(type) {
		case map[string]interface{}:
			var ok bool
			if doc, ok = here[key]; !ok {
				return nil, propertyError(ptr[:i].String())
			}
		case []interface{}:
			n, err := arrayIndex(key)
			if err != nil {
				return nil, &PtrError{ptr[:i].String(), err}
			}
			if n < 0 || n >= len(here) {
				return nil, indexError(ptr[:i].String())
			}
			doc = here[n]
		case JSONDecoder:
			v, err := getJSON(here, ptr[i:].String())
			if err != nil {
				err.rebase(ptr[:i].String())
			}
			return v, err
		case json.RawMessage:
			v, err := getRaw(here, ptr[i:].String())
			if err != nil {
				err.rebase(ptr[:i].String())
			}
			return v, err
		default:
			// We report the error at the upper level
			return nil, docError(ptr[:i-1].String(), doc)
		}
	}

	doc, err := getLeaf(doc)
	if err != nil {
		err.rebase(ptr.String())
	}
	return doc, err
}

// Set changes a value in document pdoc at location pointed by ptr.
func (ptr Pointer) Set(pdoc *interface{}, value interface{}) error {
	// TODO Make an optimised implementation
	return Set(pdoc, ptr.String(), value)
}
