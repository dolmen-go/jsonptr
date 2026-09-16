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
	"fmt"
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

// decodeLayer decodes only the outer layer of a raw JSON value: an object is
// decoded as a map[string]json.RawMessage, an array as a []json.RawMessage
// (the members are kept raw), and any other value is fully decoded.
func decodeLayer(raw json.RawMessage) (interface{}, error) {
	// Skip leading JSON whitespace to find the kind of value
	i := 0
	for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t' || raw[i] == '\n' || raw[i] == '\r') {
		i++
	}
	if i < len(raw) {
		switch raw[i] {
		case '{':
			var m map[string]json.RawMessage
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil, err
			}
			return m, nil
		case '[':
			var s []json.RawMessage
			if err := json.Unmarshal(raw, &s); err != nil {
				return nil, err
			}
			return s, nil
		}
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// materialize walks *pdoc along ptr[p:] (ptr[:p] is the already walked part)
// and replaces in place any [encoding/json.RawMessage] or JSONDecoder found on
// the way (including at the end of the path) by its decoded value.
// The value at ptr is returned: it is attached to the tree at *pdoc and
// can be modified in place.
//
// Decoding is layered (see decodeLayer): a raw object or array is decoded as a
// map[string]json.RawMessage or []json.RawMessage, so only the containers on
// the path are decoded and the values outside the path are kept raw.
//
// A map[string]json.RawMessage or []json.RawMessage traversed on the way is
// converted to map[string]interface{} or []interface{} (only the traversed
// element is decoded, the others are kept raw), as the decoded element must be
// stored in it. One at the end of the path is left untouched.
//
// Errors are reported like Get does.
func materialize(pdoc *interface{}, ptr string, p int) (interface{}, error) {
	switch raw := (*pdoc).(type) {
	case json.RawMessage:
		v, err := decodeLayer(raw)
		if err != nil {
			return nil, jsonError(ptr[:p], err)
		}
		*pdoc = v
	case JSONDecoder:
		var r json.RawMessage
		if err := raw.Decode(&r); err != nil {
			return nil, jsonError(ptr[:p], err)
		}
		v, err := decodeLayer(r)
		if err != nil {
			return nil, jsonError(ptr[:p], err)
		}
		*pdoc = v
	}

	if p >= len(ptr) {
		return *pdoc, nil
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
			return nil, &BadPointerError{ptr[:p], err}
		}
		child, ok := here[key]
		if !ok {
			return nil, propertyError(ptr[:p])
		}
		v, err := materialize(&child, ptr, p)
		here[key] = child
		return v, err
	case []interface{}:
		n, err := arrayIndex(token)
		if err != nil {
			return nil, &BadPointerError{ptr[:p], err}
		}
		if n < 0 || n >= len(here) {
			return nil, indexError(ptr[:p])
		}
		return materialize(&here[n], ptr, p)
	case map[string]json.RawMessage:
		key, err := UnescapeString(token)
		if err != nil {
			return nil, &BadPointerError{ptr[:p], err}
		}
		raw, ok := here[key]
		if !ok {
			return nil, propertyError(ptr[:p])
		}
		m := make(map[string]interface{}, len(here))
		for k, v := range here {
			m[k] = v
		}
		*pdoc = m
		var child interface{} = raw
		v, err := materialize(&child, ptr, p)
		m[key] = child
		return v, err
	case []json.RawMessage:
		n, err := arrayIndex(token)
		if err != nil {
			return nil, &BadPointerError{ptr[:p], err}
		}
		if n < 0 || n >= len(here) {
			return nil, indexError(ptr[:p])
		}
		s := make([]interface{}, len(here))
		for i, v := range here {
			s[i] = v
		}
		*pdoc = s
		return materialize(&s[n], ptr, p)
	default:
		return nil, docError(ptr[:p], *pdoc)
	}
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

// Set stores value at the location pointed by ptr in the document *doc.
//
// *doc may be any document accepted by [Get]. The root of the document
// (ptr == "") may be replaced. Otherwise the parent of the location must
// exist: only the leaf is created if it doesn't exist. In an array, index "-"
// appends the value, and an index beyond the end extends the array with
// nulls.
//
// value is stored as-is, except a JSONDecoder which is drained of its next
// value into a [encoding/json.RawMessage].
//
// Every container on the path to the value is rewritten in the tree as a
// map[string]interface{} or a []interface{}. A [encoding/json.RawMessage] or
// JSONDecoder on the path is decoded lazily, so only the containers on the
// path are decoded and the values outside the path are kept raw. A
// map[string]json.RawMessage or a []json.RawMessage on the path is converted,
// its members are kept raw.
//
// In case of error the document is left unchanged, except that a JSONDecoder
// on the path (or given as value) may have been read.
func Set(doc *interface{}, ptr string, value interface{}) error {
	if dec, isDec := value.(JSONDecoder); isDec {
		var raw json.RawMessage
		err := dec.Decode(&raw)
		if err != nil {
			return fmt.Errorf("invalid value to inject: %w", err)
		}
		value = raw
	}

	if len(ptr) == 0 {
		*doc = value
		return nil
	}
	if ptr[0] != '/' {
		return syntaxError(ptr)
	}

	if raw, ok := (*doc).(JSONDecoder); ok {
		var r json.RawMessage
		if err := raw.Decode(&r); err != nil {
			return jsonError("", err)
		}
		*doc = r
	}

	parent := *doc
	if raw, ok := parent.(json.RawMessage); ok {
		var err error
		parent, err = decodeLayer(raw)
		if err != nil {
			return jsonError("", err)
		}
	}

	// Split ptr as "/" + prop + nextPtr
	p := strings.IndexByte(ptr[1:], '/')
	if p < 0 {
		p = len(ptr) - 1
	}
	prop := ptr[1 : 1+p] // first token, the child of *doc to follow
	curPtr := ptr[:1+p]  // "/" + prop: location of that child, relative to *doc
	nextPtr := ptr[1+p:] // rest of the path, relative to the child

	switch parent := (parent).(type) {
	case map[string]interface{}:
		key, err := UnescapeString(prop)
		if err != nil {
			return &BadPointerError{curPtr, err}
		}
		tmp, found := parent[key]
		if !found && nextPtr != "" {
			// Only the leaf may be created
			return propertyError(curPtr)
		}
		if err := Set(&tmp, nextPtr, value); err != nil {
			err.(ptrError).rebase(curPtr)
			return err
		}
		if parent != nil {
			parent[key] = tmp
			*doc = parent // for the case where parent was deserialized
		} else {
			*doc = map[string]interface{}{key: tmp}
		}
	case map[string]json.RawMessage:
		key, err := UnescapeString(prop)
		if err != nil {
			return &BadPointerError{curPtr, err}
		}
		var tmp interface{}
		if v, found := parent[key]; found {
			tmp = v
		} else if nextPtr != "" {
			// Only the leaf may be created
			return propertyError(curPtr)
		}
		if err := Set(&tmp, nextPtr, value); err != nil {
			err.(ptrError).rebase(curPtr)
			return err
		}
		if parent != nil {
			m := make(map[string]interface{}, len(parent))
			for k, v := range parent {
				m[k] = v
			}
			m[key] = tmp
			*doc = m
		} else {
			*doc = map[string]interface{}{key: tmp}
		}
	case []interface{}:
		n, err := arrayIndex(prop)
		if err != nil {
			return &BadPointerError{curPtr, err}
		}
		var tmp interface{}
		if n >= 0 && n < len(parent) {
			tmp = parent[n]
		} else if nextPtr != "" {
			// Only the leaf may be created
			return indexError(curPtr)
		}
		if err := Set(&tmp, nextPtr, value); err != nil {
			err.(ptrError).rebase(curPtr)
			return err
		}
		if n == -1 {
			n = len(parent)
		}
		if n >= len(parent) {
			// Pad with nulls up to n
			parent = append(parent, make([]interface{}, n-len(parent)+1)...)
		}
		*doc = parent // We do it in all cases (not just realloc) because parent might have originally been deserialized
		parent[n] = tmp
	case []json.RawMessage:
		n, err := arrayIndex(prop)
		if err != nil {
			return &BadPointerError{curPtr, err}
		}
		var tmp interface{}
		if n >= 0 && n < len(parent) {
			tmp = parent[n]
		} else if nextPtr != "" {
			// Only the leaf may be created
			return indexError(curPtr)
		}
		if err := Set(&tmp, nextPtr, value); err != nil {
			err.(ptrError).rebase(curPtr)
			return err
		}
		if n == -1 {
			n = len(parent)
		}
		// Convert to []interface{}, padded with nulls up to n if necessary
		l := len(parent)
		if n >= l {
			l = n + 1
		}
		arr := make([]interface{}, l)
		for i, v := range parent {
			arr[i] = v
		}
		arr[n] = tmp
		*doc = arr
	default:
		return docError(curPtr, parent)
	}

	return nil
}

// Delete removes an object property or an array element (and shifts remaining ones).
// It can't be applied on root.
//
// Any [encoding/json.RawMessage] or JSONDecoder on the path to the value
// is replaced in the tree by its decoded value, lazily as in [Set].
// A value deleted from a map[string]json.RawMessage or []json.RawMessage
// (including one resulting from that lazy decoding) is returned as the
// json.RawMessage stored in it.
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

	parent, err := materialize(pdoc, parentPtr, 0)
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
