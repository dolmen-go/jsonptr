// Copyright 2016 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

// Package jsonptr implements JSON Pointer (RFC 6901) lookup. Fast and strong testsuite.
//
// Any part of a data tree made of []interface{} or map[string]interface{}
// can be dereferenced with a JSON Pointer.
//
// Specification: https://tools.ietf.org/html/rfc6901
package jsonptr

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

func arrayIndex(token string) (int, error) {
	if len(token) == 0 {
		return -1, ErrSyntax
	}
	if len(token) == 1 {
		if token[0] == '0' {
			return 0, nil
		}
		if token[0] == '-' {
			return -1, nil
		}
	}
	if token[0] < '1' {
		return -1, ErrSyntax
	}
	var n int
	const maxInt = (1 << (strconv.IntSize - 1)) - 1
	const cutoff = maxInt/10 + 1
	for i := 0; i < len(token); i++ {
		c := token[i]
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

// jsonDecoder is a subset of the interface of encoding/json.Decoder.
type jsonDecoder interface {
	Token() (json.Token, error)
	More() bool
	Decode(interface{}) error
}

func getJSON(decoder jsonDecoder, ptr string) (interface{}, error) {
	//log.Println("[", ptr, "]")

	p := int(1)
	cur := ptr[1:]
	for {
		q := strings.IndexByte(cur, '/')
		if q != -1 {
			cur = cur[:q]
		} else {
			q = len(cur)
		}
		p += q

		tok, err := decoder.Token()
		if err != nil {
			// TODO wrap err
			return nil, err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil, docError(ptr[:p-q-1], tok)
		}
		switch delim {
		case '{':
			key, err := UnescapeString(cur)
			if err != nil {
				return nil, &PtrError{ptr[:p], err}
			}
			found := false
			for {
				tok, err := decoder.Token()
				if err != nil {
					// TODO wrap err
					return nil, err
				}
				k, ok := tok.(string)
				if !ok {
					// This should not happen
					panic("unexpected key type")
				}
				if !decoder.More() {
					panic("unexpected missing value in object")
				}
				if k == key {
					found = true
					break
				}
				// skip value
				var skip json.RawMessage
				err = decoder.Decode(&skip)
				if err != nil {
					return nil, err
				}
			}
			if !found {
				return nil, propertyError(ptr[:p])
			}
		case '[':
			n, err := arrayIndex(cur)
			if err != nil {
				return nil, &PtrError{ptr[:p], err}
			}
			if n < 0 {
				return nil, indexError(ptr[:p])
			}
			i := -1
			for decoder.More() {
				i++
				if i == n {
					// Continue deeper in the structure
					break
				}
				var skip json.RawMessage
				err = decoder.Decode(&skip)
				if err != nil {
					return nil, err
				}
			}
			if i < n {
				return nil, indexError(ptr[:p])
			}
		}

		p++
		if p > len(ptr) {
			break
		}
		cur = ptr[p:]
	}

	var value interface{}
	err := decoder.Decode(&value)
	return value, err
}

func getRaw(doc json.RawMessage, ptr string) (interface{}, error) {
	/*
		if len(ptr) == 0 {
			var value interface{}
			err := json.Unmarshal(doc, value)
			return value, err
		}
		if ptr[0] != '/' {
			return nil, ErrSyntax
		}
	*/

	return getJSON(json.NewDecoder(bytes.NewReader(doc)), ptr)
}

func getLeaf(doc interface{}) (interface{}, error) {
	if raw, ok := doc.(json.RawMessage); ok {
		err := json.Unmarshal(raw, &doc)
		return doc, err
	}
	return doc, nil
}

// Get extracts a value from a JSON-like data tree.
//
// doc may be a deserialized document, or a json.RawMessage.
// In case of error a PtrError is returned.
func Get(doc interface{}, ptr string) (interface{}, error) {
	if len(ptr) == 0 {
		return getLeaf(doc)
	}
	if ptr[0] != '/' {
		return nil, syntaxError(ptr)
	}
	cur := ptr[1:]
	p := int(1)
	for {
		q := strings.IndexByte(cur, '/')
		if q == -1 {
			q = len(cur)
		}
		p += q

		switch here := (doc).(type) {
		case map[string]interface{}:
			key, err := UnescapeString(cur[:q])
			if err != nil {
				return nil, &PtrError{ptr[:p], err}
			}
			var ok bool
			if doc, ok = here[key]; !ok {
				return nil, propertyError(ptr[:p])
			}
		case []interface{}:
			n, err := arrayIndex(cur[:q])
			if err != nil {
				return nil, &PtrError{ptr[:p], err}
			}
			if n < 0 || n >= len(here) {
				return nil, indexError(ptr[:p])
			}
			doc = here[n]
		case json.RawMessage:
			v, err := getRaw(here, ptr[p-q-1:])
			if perr, ok := err.(*PtrError); ok {
				perr.Ptr = ptr[:p-q-1+len(perr.Ptr)]
			}
			return v, err
		default:
			return nil, docError(ptr[:p], doc)
		}
		if p >= len(ptr) {
			break
		}
		p++
		cur = ptr[p:]
	}

	return getLeaf(doc)
}

// Set modifies a JSON-like data tree.
//
// In case of error a PtrError is returned.
func Set(doc *interface{}, ptr string, value interface{}) error {
	if len(ptr) == 0 {
		*doc = value
		return nil
	}
	p := strings.LastIndexByte(ptr, '/')
	if p < 0 {
		return syntaxError(ptr)
	}
	prop := ptr[p+1:]
	parentPtr := ptr[:p]

	parent, err := Get(*doc, parentPtr)
	if err != nil {
		return err
	}

	switch parent := (parent).(type) {
	case map[string]interface{}:
		key, err := UnescapeString(prop)
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
