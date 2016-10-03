// Copyright 2016 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

// Package jsonptr implements JSON Pointer (RFC 6901) lookup
package jsonptr

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrSyntax   = errors.New("invalid JSON pointer")
	ErrIndex    = errors.New("invalid array index")
	ErrProperty = errors.New("property not found")
)

// PtrError is the structured error for JSON Pointer parsing or navigation
// errors
type PtrError struct {
	// Ptr is the substring of the original pointer where the error occurred
	Ptr string
	// Err is one of ErrSyntax, ErrIndex, ErrProperty
	Err error
}

// Error implement the 'error' interface
func (e *PtrError) Error() string {
	return strconv.Quote(e.Ptr) + ": " + e.Err.Error()
}

func syntaxError(ptr string) *PtrError {
	return &PtrError{ptr, ErrSyntax}
}

func indexError(ptr string) *PtrError {
	return &PtrError{ptr, ErrIndex}
}

func propertyError(ptr string) *PtrError {
	return &PtrError{ptr, ErrProperty}
}

func docError(ptr string, doc interface{}) *PtrError {
	return &PtrError{ptr, fmt.Errorf("not an object or array but %T", doc)}
}

func arrayIndex(b []byte) (int, error) {
	if len(b) == 0 {
		return -1, ErrSyntax
	}
	if len(b) == 1 {
		if b[0] == '0' {
			return 0, nil
		}
		if b[0] == '-' {
			return -1, nil
		}
	}
	if b[0] < '1' {
		return -1, ErrSyntax
	}
	var n int
	const maxInt = (1 << (strconv.IntSize - 1)) - 1
	const cutoff = maxInt/10 + 1
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c < '0' || c > '9' {
			return -1, ErrSyntax
		}
		if n >= cutoff {
			// Overflow
			return -1, ErrSyntax
		}
		n *= 10
		n1 := n + int(c-'0')
		if n1 < n || n1 > maxInt {
			// Overflow
			return -1, ErrSyntax
		}
		n = n1
	}
	return n, nil
}

func propertyName(b []byte) (string, error) {
	// FIXME reject '~' followed by something else than '0', '1'
	return string(
		bytes.Replace(
			bytes.Replace(b,
				[]byte(`~1`), []byte(`/`), -1),
			[]byte(`~0`), []byte(`~`), -1),
	), nil
}

// Get extracts a value from a JSON-like data tree
//
// In case of error a PtrError is returned
func Get(doc interface{}, ptr string) (interface{}, error) {
	if len(ptr) == 0 {
		return doc, nil
	}
	if ptr[0] != '/' {
		return nil, syntaxError(ptr)
	}
	bptr := []byte(ptr)
	cur := bptr[1:]
	p := int(1)
	for {
		q := bytes.IndexByte(cur, `/`[0])
		if q == -1 {
			q = len(cur)
		}
		p += q

		switch here := (doc).(type) {
		case map[string]interface{}:
			key, err := propertyName(cur[:q])
			if err != nil {
				return nil, &PtrError{string(bptr[:p]), err}
			}
			var ok bool
			if doc, ok = here[key]; !ok {
				return nil, propertyError(string(bptr[:p]))
			}
		case []interface{}:
			n, err := arrayIndex(cur[:q])
			if err != nil {
				return nil, &PtrError{string(bptr[:p]), err}
			}
			if n < 0 || n >= len(here) {
				return nil, indexError(string(bptr[:p]))
			}
			doc = here[n]
		default:
			return nil, docError(string(bptr[:p]), doc)
		}
		if p >= len(bptr) {
			break
		}
		p++
		cur = bptr[p:]
	}

	return doc, nil
}

// Set modifies a JSON-like data tree
//
// In case of error a PtrError is returned
func Set(doc *interface{}, ptr string, value interface{}) error {
	if len(ptr) == 0 {
		*doc = value
		return nil
	}
	bptr := []byte(ptr)
	p := bytes.LastIndexByte(bptr, '/')
	if p < 0 {
		return syntaxError(ptr)
	}
	prop := bptr[p+1:]
	parentPtr := string(bptr[:p])

	parent, err := Get(*doc, parentPtr)
	if err != nil {
		return err
	}

	switch parent := (parent).(type) {
	case map[string]interface{}:
		key, err := propertyName(prop)
		if err != nil {
			return &PtrError{ptr, err}
		}
		parent[key] = value
	case []interface{}:
		n, err := arrayIndex(prop)
		if err != nil {
			return &PtrError{ptr, err}

		}
		if n == -1 {
			n = len(parent)
		} else if n < len(parent) {
			parent[n] = value
			return nil
		}

		// if n > len(parent) {
		//	return &PtrError{ptr, ErrIndex}
		//}

		// TODO make+copy
		for i := n - len(parent) - 1; i > 0; i-- {
			parent = append(parent, nil)
		}
		parent = append(parent, value)
		// We appended beyond original len, so the slice changed so we have to
		// store the new one at the old place
		// No error can happen as we already parsed the pointer
		_ = Set(doc, parentPtr, parent)
	default:
		return docError(parentPtr, parent)
	}

	return nil
}
