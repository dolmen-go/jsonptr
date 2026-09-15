// Copyright 2016-2020 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

// Package jsonptr implements JSON Pointer (RFC 6901) lookup. Fast, with strong testsuite.
//
// Any part of a data tree made of []interface{} or map[string]interface{}
// may be dereferenced with a JSON Pointer.
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

// JSONDecoder is a subset of the interface of [encoding/json.Decoder].
// It can be used as an input to Get().
type JSONDecoder interface {
	Token() (json.Token, error)
	More() bool
	Decode(interface{}) error
}

func getJSON(decoder JSONDecoder, ptr string) (interface{}, ptrError) {
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
			return nil, jsonError(ptr[:p], err)
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil, docError(ptr[:p-q-1], tok)
		}
		switch delim {
		case '{':
			key, err := UnescapeString(cur)
			if err != nil {
				return nil, &BadPointerError{ptr[:p], err}
			}
			found := false
			// More() is false once the closing '}' is reached
			for decoder.More() {
				tok, err := decoder.Token()
				if err != nil {
					return nil, jsonError(ptr[:p], err)
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
					return nil, jsonError(ptr[:p], err)
				}
			}
			if !found {
				return nil, propertyError(ptr[:p])
			}
		case '[':
			n, err := arrayIndex(cur)
			if err != nil {
				return nil, &BadPointerError{ptr[:p], err}
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
					return nil, jsonError(ptr[:p], err)
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
	if err := decoder.Decode(&value); err != nil {
		return nil, jsonError(ptr, err)
	}
	return value, nil
}

func getRaw(doc json.RawMessage, ptr string) (interface{}, ptrError) {
	/*
		if len(ptr) == 0 {
			var value interface{}
			if err := json.Unmarshal(doc, &value); err != nil {
				return nil, jsonError(ptr, err)
			}
			return value, nil
		}
		if ptr[0] != '/' {
			return nil, syntaxError(ptr)
		}
	*/

	return getJSON(json.NewDecoder(bytes.NewReader(doc)), ptr)
}

func getLeaf(doc interface{}) (interface{}, ptrError) {
	var err error

	switch raw := doc.(type) {
	case json.RawMessage:
		doc = nil
		err = json.Unmarshal(raw, &doc)
	case JSONDecoder:
		doc = nil
		err = raw.Decode(&doc)
	default:
		return doc, nil
	}
	if err != nil {
		return nil, jsonError("", err)
	}
	return doc, nil
}

// Get extracts a value from a JSON-like data tree.
//
// doc may be:
//   - a deserialized document made of []interface{}, map[string]interface{} or any terminal value
//   - a [encoding/json.RawMessage]
//   - a JSONDecoder (such as *[encoding/json.Decoder]) for streamed decoding
//   - a partially deserialized document: []json.RawMessage, map[string]json.RawMessage
//
// Those containers may be mixed at any level of the tree.
//
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
				return nil, &BadPointerError{ptr[:p], err}
			}
			var ok bool
			if doc, ok = here[key]; !ok {
				return nil, propertyError(ptr[:p])
			}
		case []interface{}:
			n, err := arrayIndex(cur[:q])
			if err != nil {
				return nil, &BadPointerError{ptr[:p], err}
			}
			if n < 0 || n >= len(here) {
				return nil, indexError(ptr[:p])
			}
			doc = here[n]
		case map[string]json.RawMessage:
			key, err := UnescapeString(cur[:q])
			if err != nil {
				return nil, &BadPointerError{ptr[:p], err}
			}
			var ok bool
			if doc, ok = here[key]; !ok {
				return nil, propertyError(ptr[:p])
			}
		case []json.RawMessage:
			n, err := arrayIndex(cur[:q])
			if err != nil {
				return nil, &BadPointerError{ptr[:p], err}
			}
			if n < 0 || n >= len(here) {
				return nil, indexError(ptr[:p])
			}
			doc = here[n]
		case JSONDecoder:
			v, err := getJSON(here, ptr[p-q-1:])
			if perr, ok := err.(*PtrError); ok {
				perr.Ptr = ptr[:p-q-1+len(perr.Ptr)]
			}
			return v, err
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

	doc, err := getLeaf(doc)
	if err != nil {
		err.rebase(ptr)
	}
	return doc, err
}

// materialize walks *pdoc along ptr[p:] (ptr[:p] is the already walked part)
// and replaces in place any [encoding/json.RawMessage] or JSONDecoder found on
// the way (including at the end of the path) by its decoded value.
// Once done, the value at ptr (if any) is attached to the tree at *pdoc and
// can be modified in place.
//
// A map[string]json.RawMessage or []json.RawMessage traversed on the way is
// converted to map[string]interface{} or []interface{} (only the traversed
// element is decoded, the others are kept raw), as the decoded element must be
// stored in it. One at the end of the path is left untouched.
//
// Only JSON decoding errors are reported: navigation errors are left for Get
// to report.
func materialize(pdoc *interface{}, ptr string, p int) error {
	switch raw := (*pdoc).(type) {
	case json.RawMessage:
		var v interface{}
		if err := json.Unmarshal(raw, &v); err != nil {
			return jsonError(ptr[:p], err)
		}
		*pdoc = v
	case JSONDecoder:
		var v interface{}
		if err := raw.Decode(&v); err != nil {
			return jsonError(ptr[:p], err)
		}
		*pdoc = v
	}

	if p >= len(ptr) {
		return nil
	}
	// ptr[p] == '/'
	p++
	q := strings.IndexByte(ptr[p:], '/')
	if q == -1 {
		q = len(ptr) - p
	}
	token := ptr[p : p+q]
	p += q

	switch here := (*pdoc).(type) {
	case map[string]interface{}:
		key, err := UnescapeString(token)
		if err != nil {
			return nil
		}
		v, ok := here[key]
		if !ok {
			return nil
		}
		err = materialize(&v, ptr, p)
		here[key] = v
		return err
	case []interface{}:
		n, err := arrayIndex(token)
		if err != nil || n < 0 || n >= len(here) {
			return nil
		}
		return materialize(&here[n], ptr, p)
	case map[string]json.RawMessage:
		key, err := UnescapeString(token)
		if err != nil {
			return nil
		}
		raw, ok := here[key]
		if !ok {
			return nil
		}
		m := make(map[string]interface{}, len(here))
		for k, v := range here {
			m[k] = v
		}
		*pdoc = m
		var v interface{} = raw
		err = materialize(&v, ptr, p)
		m[key] = v
		return err
	case []json.RawMessage:
		n, err := arrayIndex(token)
		if err != nil || n < 0 || n >= len(here) {
			return nil
		}
		s := make([]interface{}, len(here))
		for i, v := range here {
			s[i] = v
		}
		*pdoc = s
		return materialize(&s[n], ptr, p)
	}
	return nil
}

// rawValue returns value as a [encoding/json.RawMessage] for storing it into
// a map[string]json.RawMessage or a []json.RawMessage: value itself if it is
// already one, the next value of a JSONDecoder, or the JSON encoding of any
// other value.
func rawValue(value interface{}) (json.RawMessage, error) {
	switch v := value.(type) {
	case json.RawMessage:
		return v, nil
	case JSONDecoder:
		var raw json.RawMessage
		err := v.Decode(&raw)
		return raw, err
	}
	return json.Marshal(value)
}

// Set modifies a JSON-like data tree.
//
// Any [encoding/json.RawMessage] or JSONDecoder on the path to the value
// is replaced in the tree by its decoded value.
//
// If the container of the value is a map[string]json.RawMessage or a
// []json.RawMessage, value is stored as a json.RawMessage: as-is if it is
// already one, else its JSON encoding.
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

	if err := materialize(doc, parentPtr, 0); err != nil {
		return err
	}
	parent, err := Get(*doc, parentPtr)
	if err != nil {
		return err
	}

	switch parent := (parent).(type) {
	case map[string]interface{}:
		key, err := UnescapeString(prop)
		if err != nil {
			return &BadPointerError{ptr, err}
		}
		if parent != nil {
			parent[key] = value
		} else {
			return Set(doc, parentPtr, map[string]interface{}{key: value})
		}
	case []interface{}:
		n, err := arrayIndex(prop)
		if err != nil {
			return &BadPointerError{ptr, err}
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

		// Pad with nulls up to n
		parent = append(parent, make([]interface{}, n-len(parent)+1)...)
		parent[n] = value
		// We appended beyond original len, so the slice changed so we have to
		// store the new one at the old place
		// No error can happen as we already parsed the pointer
		_ = Set(doc, parentPtr, parent)
	case map[string]json.RawMessage:
		key, err := UnescapeString(prop)
		if err != nil {
			return &BadPointerError{ptr, err}
		}
		raw, err := rawValue(value)
		if err != nil {
			return &DocumentError{ptr, err}
		}
		if parent != nil {
			parent[key] = raw
		} else {
			return Set(doc, parentPtr, map[string]json.RawMessage{key: raw})
		}
	case []json.RawMessage:
		n, err := arrayIndex(prop)
		if err != nil {
			return &BadPointerError{ptr, err}
		}
		raw, err := rawValue(value)
		if err != nil {
			return &DocumentError{ptr, err}
		}
		if n == -1 {
			n = len(parent)
		} else if n < len(parent) {
			parent[n] = raw
			return nil
		}
		// Pad with nulls up to n (a nil json.RawMessage is encoded as null)
		parent = append(parent, make([]json.RawMessage, n-len(parent)+1)...)
		parent[n] = raw
		_ = Set(doc, parentPtr, parent)
	default:
		return docError(parentPtr, parent)
	}

	return nil
}

// Delete removes an object property or an array element (and shifts remaining ones).
// It can't be applied on root.
//
// Any [encoding/json.RawMessage] or JSONDecoder on the path to the value
// is replaced in the tree by its decoded value.
func Delete(pdoc *interface{}, ptr string) (interface{}, error) {
	if len(ptr) == 0 {
		return nil, &BadPointerError{ptr, ErrDeleteRoot}
	}

	p := strings.LastIndexByte(ptr, '/')
	if p < 0 {
		return nil, syntaxError(ptr)
	}
	prop := ptr[p+1:]
	parentPtr := ptr[:p]

	if err := materialize(pdoc, parentPtr, 0); err != nil {
		return nil, err
	}
	parent, err := Get(*pdoc, parentPtr)
	if err != nil {
		return nil, err
	}

	switch parent := (parent).(type) {
	case map[string]interface{}:
		key, err := UnescapeString(prop)
		if err != nil {
			return nil, &BadPointerError{ptr, err}
		}
		v, found := parent[key]
		if !found {
			return nil, propertyError(ptr)
		}
		delete(parent, key)
		return v, nil
	case []interface{}:
		n, err := arrayIndex(prop)
		if err != nil {
			return nil, &BadPointerError{ptr, err}
		}
		/*
			// FIXME what should be the baviour for '-'?
			if n == -1 {
				n = len(parent)-1
			}
		*/
		if n < 0 {
			return nil, &BadPointerError{ptr, ErrIndex}
		} else if n >= len(parent) {
			return nil, &BadPointerError{ptr, ErrIndex}
		}
		v := parent[n]
		copy(parent[n:], parent[n+1:])
		return v, Set(pdoc, parentPtr, parent[:len(parent)-1])
	case map[string]json.RawMessage:
		key, err := UnescapeString(prop)
		if err != nil {
			return nil, &BadPointerError{ptr, err}
		}
		v, found := parent[key]
		if !found {
			return nil, propertyError(ptr)
		}
		delete(parent, key)
		return v, nil
	case []json.RawMessage:
		n, err := arrayIndex(prop)
		if err != nil {
			return nil, &BadPointerError{ptr, err}
		}
		if n < 0 || n >= len(parent) {
			return nil, &BadPointerError{ptr, ErrIndex}
		}
		v := parent[n]
		copy(parent[n:], parent[n+1:])
		return v, Set(pdoc, parentPtr, parent[:len(parent)-1])
	default:
		return nil, docError(parentPtr, parent)
	}
}
