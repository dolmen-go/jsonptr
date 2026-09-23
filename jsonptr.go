// Copyright 2016-2026 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

// Package jsonptr implements JSON Pointer (RFC 6901). Fast, with strong testsuite.
//
// A JSON Pointer designates a value in a JSON document. [Get], [Set] and
// [Delete] apply one, in its text form, to a document. [Parse] builds a
// [Pointer], the parsed form, which can be moved around the document tree
// ([Pointer.Property], [Pointer.Index], [Pointer.Up]) before being applied
// with [Pointer.In], [Pointer.Set] or [Pointer.Delete].
//
// A document is a tree of []any, map[string]any and terminal values, as
// produced by [json.Unmarshal]. Any part of it may also be left
// undecoded, as a [json.RawMessage], a [JSONDecoder] or a partially
// deserialized container ([]json.RawMessage, map[string]json.RawMessage):
// those are traversed transparently and decoded only as far as needed.
//
// Errors tell which part of the pointer evaluation failed, and why: see
// [BadPointerError], [PtrError] and [DocumentError].
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
//
// It may be given to [Get], [Pointer.In], [Set] or [Delete] for streamed
// decoding, either as the document itself or as any value nested in it.
// [Set] also accepts it as the value to store.
//
// A JSONDecoder is consumed by those calls, so it can't be reused: [Get] and
// [Pointer.In] read from it up to the designated value, while [Set] and
// [Delete] read its next value as a [json.RawMessage] and store that
// raw value in the document in place of the decoder.
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
				return nil, badIndexError(ptr[:p], cur)
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
	case []json.RawMessage:
		// The nil case is not expected, but still handled for completeness
		if raw == nil {
			doc = []interface{}(nil)
		} else {
			arr := make([]interface{}, len(raw))
			for i, v := range raw {
				var perr ptrError
				arr[i], perr = getLeaf(v)
				if perr != nil {
					perr.rebase(fmt.Sprintf("/%d", i))
					return nil, perr
				}
			}
			doc = arr
		}
	case map[string]json.RawMessage:
		// The nil case is not expected, but still handled for completeness
		if raw == nil {
			doc = map[string]interface{}(nil)
		} else {
			m := make(map[string]interface{}, len(raw))
			for k, v := range raw {
				var perr ptrError
				m[k], perr = getLeaf(v)
				if perr != nil {
					perr.rebase("/" + EscapeString(k))
					return nil, perr
				}
			}
			doc = m
		}
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
// In the common case doc is a deserialized document, made of []any,
// map[string]any and terminal values. The value designated by ptr is then
// returned as it is stored in doc: a container is the very one held by the
// tree, so modifying it modifies doc.
//
// In case of error a [*BadPointerError] (invalid pointer), a [*PtrError]
// (the pointer doesn't match the document) or a [*DocumentError] (the document
// can't be traversed or decoded) is returned.
//
// # Undecoded values
//
// doc, or any value nested in it, may also be:
//   - a [encoding/json.RawMessage]
//   - a [JSONDecoder] (such as *[encoding/json.Decoder]) for streamed decoding
//   - a partially deserialized container: []json.RawMessage or
//     map[string]json.RawMessage
//
// Those may be mixed with the deserialized containers at any level of the
// tree and are traversed transparently.
//
// However, when such an undecoded value is the one designated by ptr, it is
// deserialized, deeply: the result is a copy, made of []any, map[string]any
// and terminal values, so modifying it does not modify doc. The original
// values are lost in the process: JSON numbers become float64, and a
// [JSONDecoder] is consumed.
//
// Members of a deserialized container are never decoded, so a
// [json.RawMessage] nested in the returned value is returned as is.
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
				return nil, badIndexError(ptr[:p], cur[:q])
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
				return nil, badIndexError(ptr[:p], cur[:q])
			}
			if n < 0 || n >= len(here) {
				return nil, indexError(ptr[:p])
			}
			doc = here[n]
		case JSONDecoder:
			v, err := getJSON(here, ptr[p-q-1:])
			if err != nil {
				err.rebase(ptr[:p-q-1])
				return nil, err
			}
			return v, nil
		case json.RawMessage:
			v, err := getRaw(here, ptr[p-q-1:])
			if err != nil {
				err.rebase(ptr[:p-q-1])
				return nil, err
			}
			return v, nil
		default:
			// Report the location of the value which can't be traversed
			return nil, docError(ptr[:p-q-1], doc)
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

// Set stores value at the location pointed by ptr in the document *pdoc.
//
// *pdoc may be any document accepted by [Get]. The root of the document
// (ptr == "") may be replaced. Otherwise the parent of the location must
// exist: only the leaf is created if it doesn't exist. In an array, index "-"
// appends the value, and an index beyond the end extends the array with
// nulls.
//
// value is stored as-is, except a [JSONDecoder] which is drained of its next
// value into a [json.RawMessage].
//
// Every container on the path to the value is rewritten in the tree as a
// map[string]any or a []any. A [json.RawMessage] or [JSONDecoder] on the
// path is decoded lazily, so only the containers on the path are decoded and
// the values outside the path are kept raw. A map[string]json.RawMessage or
// a []json.RawMessage on the path is converted, its members are kept raw.
//
// In case of error the document is left unchanged, except that a [JSONDecoder]
// on the path is replaced by the raw value read from it (a [JSONDecoder] given
// as value may also have been read).
func Set(pdoc *interface{}, ptr string, value interface{}) error {
	if dec, isDec := value.(JSONDecoder); isDec {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return &DocumentError{"", fmt.Errorf("invalid value to inject: %w", err)}
		}
		value = raw
	}
	if len(ptr) != 0 && ptr[0] != '/' {
		return syntaxError(ptr)
	}

	// Convert a nil ptrError to a nil error
	if err := set(pdoc, ptr, value); err != nil {
		return err
	}
	return nil
}

// set is the recursive implementation of [Set]: it follows the first token
// of ptr into *pdoc, calls itself on the child with the rest of the path, then
// stores the child back into *pdoc.
//
// ptr is either empty or starts with '/'. Errors are located relatively to
// *pdoc: the caller rebases them.
func set(pdoc *interface{}, ptr string, value interface{}) ptrError {
	if len(ptr) == 0 {
		*pdoc = value
		return nil
	}

	if raw, ok := (*pdoc).(JSONDecoder); ok {
		var r json.RawMessage
		if err := raw.Decode(&r); err != nil {
			return jsonError("", err)
		}
		*pdoc = r
	}

	parent := *pdoc
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
	prop := ptr[1 : 1+p] // first token, the child of *pdoc to follow
	curPtr := ptr[:1+p]  // "/" + prop: location of that child, relative to *pdoc
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
		if err := set(&tmp, nextPtr, value); err != nil {
			if _, isRaw := tmp.(json.RawMessage); isRaw {
				// A JSONDecoder child has been read: keep its raw bytes in the tree
				parent[key] = tmp
			}
			err.rebase(curPtr)
			return err
		}
		if parent != nil {
			parent[key] = tmp
			*pdoc = parent // for the case where parent was deserialized
		} else {
			*pdoc = map[string]interface{}{key: tmp}
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
		if err := set(&tmp, nextPtr, value); err != nil {
			err.rebase(curPtr)
			return err
		}
		if parent != nil {
			m := make(map[string]interface{}, len(parent))
			for k, v := range parent {
				m[k] = v
			}
			m[key] = tmp
			*pdoc = m
		} else {
			*pdoc = map[string]interface{}{key: tmp}
		}
	case []interface{}:
		n, err := arrayIndex(prop)
		if err != nil {
			return badIndexError(curPtr, prop)
		}
		var tmp interface{}
		if n >= 0 && n < len(parent) {
			tmp = parent[n]
		} else if nextPtr != "" {
			// Only the leaf may be created
			return indexError(curPtr)
		}
		if err := set(&tmp, nextPtr, value); err != nil {
			if _, isRaw := tmp.(json.RawMessage); isRaw {
				// A JSONDecoder child has been read: keep its raw bytes in the tree
				parent[n] = tmp
			}
			err.rebase(curPtr)
			return err
		}
		if n == -1 {
			n = len(parent)
		}
		if n >= len(parent) {
			// Pad with nulls up to n
			parent = append(parent, make([]interface{}, n-len(parent)+1)...)
		}
		*pdoc = parent // We do it in all cases (not just realloc) because parent might have originally been deserialized
		parent[n] = tmp
	case []json.RawMessage:
		n, err := arrayIndex(prop)
		if err != nil {
			return badIndexError(curPtr, prop)
		}
		var tmp interface{}
		if n >= 0 && n < len(parent) {
			tmp = parent[n]
		} else if nextPtr != "" {
			// Only the leaf may be created
			return indexError(curPtr)
		}
		if err := set(&tmp, nextPtr, value); err != nil {
			err.rebase(curPtr)
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
		*pdoc = arr
	default:
		// Report the location of parent itself: the caller rebases it
		return docError("", parent)
	}

	return nil
}

// Delete removes the value at the location pointed by ptr in the document
// *pdoc, and returns it. An array element is removed by shifting the remaining
// ones.
//
// *pdoc may be any document accepted by [Get]. The root of the document can't
// be deleted ([ErrDeleteRoot]). The value must exist ([ErrProperty], [ErrIndex]);
// index "-" is not accepted.
//
// The returned value is the one stored in the tree, so it is a
// [json.RawMessage] when the container of the value is a
// map[string]json.RawMessage or a []json.RawMessage (including when it results
// from the lazy decoding below).
//
// Every container on the path to the value, except the container of the value
// itself, is rewritten in the tree as a map[string]any or a []any. A
// [json.RawMessage] or [JSONDecoder] on the path is decoded lazily, so only
// the containers on the path are decoded and the values outside the path are
// kept raw. A map[string]json.RawMessage or a []json.RawMessage on the path
// is converted, its members are kept raw.
//
// In case of error the document is left unchanged, except that a [JSONDecoder]
// on the path is replaced by the raw value read from it.
func Delete(pdoc *interface{}, ptr string) (interface{}, error) {
	if len(ptr) == 0 {
		return nil, &BadPointerError{ptr, ErrDeleteRoot}
	}
	if ptr[0] != '/' {
		return nil, syntaxError(ptr)
	}

	// Convert a nil ptrError to a nil error
	v, err := del(pdoc, ptr)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// del is the recursive implementation of [Delete]: it follows the first token
// of ptr into *pdoc, then either removes the child (last token) or calls itself
// on the child with the rest of the path and stores the child back into *pdoc.
//
// ptr is not empty and starts with '/'. Errors are located relatively to
// *pdoc: the caller rebases them.
func del(pdoc *interface{}, ptr string) (interface{}, ptrError) {
	if raw, ok := (*pdoc).(JSONDecoder); ok {
		var r json.RawMessage
		if err := raw.Decode(&r); err != nil {
			return nil, jsonError("", err)
		}
		*pdoc = r
	}

	parent := *pdoc
	if raw, ok := parent.(json.RawMessage); ok {
		var err error
		parent, err = decodeLayer(raw)
		if err != nil {
			return nil, jsonError("", err)
		}
	}

	// Split ptr as "/" + prop + nextPtr
	p := strings.IndexByte(ptr[1:], '/')
	if p < 0 {
		p = len(ptr) - 1
	}
	prop := ptr[1 : 1+p] // first token, the child of *pdoc to follow
	curPtr := ptr[:1+p]  // "/" + prop: location of that child, relative to *pdoc
	nextPtr := ptr[1+p:] // rest of the path, relative to the child

	switch parent := (parent).(type) {
	case map[string]interface{}:
		key, err := UnescapeString(prop)
		if err != nil {
			return nil, &BadPointerError{curPtr, err}
		}
		tmp, found := parent[key]
		if !found {
			return nil, propertyError(curPtr)
		}
		if nextPtr == "" {
			delete(parent, key)
			return tmp, nil
		}
		v, perr := del(&tmp, nextPtr)
		if perr != nil {
			if _, isRaw := tmp.(json.RawMessage); isRaw {
				// A JSONDecoder child has been read: keep its raw bytes in the tree
				parent[key] = tmp
			}
			perr.rebase(curPtr)
			return nil, perr
		}
		parent[key] = tmp
		return v, nil
	case map[string]json.RawMessage:
		key, err := UnescapeString(prop)
		if err != nil {
			return nil, &BadPointerError{curPtr, err}
		}
		raw, found := parent[key]
		if !found {
			return nil, propertyError(curPtr)
		}
		if nextPtr == "" {
			delete(parent, key)
			*pdoc = parent // for the case where parent was deserialized
			return raw, nil
		}
		var tmp interface{} = raw
		v, perr := del(&tmp, nextPtr)
		if perr != nil {
			perr.rebase(curPtr)
			return nil, perr
		}
		// Convert to map[string]interface{} to store the decoded child
		m := make(map[string]interface{}, len(parent))
		for k, v := range parent {
			m[k] = v
		}
		m[key] = tmp
		*pdoc = m
		return v, nil
	case []interface{}:
		n, err := arrayIndex(prop)
		if err != nil {
			return nil, badIndexError(curPtr, prop)
		}
		if n < 0 || n >= len(parent) {
			return nil, indexError(curPtr)
		}
		tmp := parent[n]
		if nextPtr == "" {
			last := len(parent) - 1
			copy(parent[n:], parent[n+1:])
			parent[last] = nil // release the reference
			*pdoc = parent[:last]
			return tmp, nil
		}
		v, perr := del(&tmp, nextPtr)
		if perr != nil {
			if _, isRaw := tmp.(json.RawMessage); isRaw {
				// A JSONDecoder child has been read: keep its raw bytes in the tree
				parent[n] = tmp
			}
			perr.rebase(curPtr)
			return nil, perr
		}
		parent[n] = tmp
		return v, nil
	case []json.RawMessage:
		n, err := arrayIndex(prop)
		if err != nil {
			return nil, badIndexError(curPtr, prop)
		}
		if n < 0 || n >= len(parent) {
			return nil, indexError(curPtr)
		}
		raw := parent[n]
		if nextPtr == "" {
			last := len(parent) - 1
			copy(parent[n:], parent[n+1:])
			parent[last] = nil // release the reference
			*pdoc = parent[:last]
			return raw, nil
		}
		var tmp interface{} = raw
		v, perr := del(&tmp, nextPtr)
		if perr != nil {
			perr.rebase(curPtr)
			return nil, perr
		}
		// Convert to []interface{} to store the decoded child
		arr := make([]interface{}, len(parent))
		for i, v := range parent {
			arr[i] = v
		}
		arr[n] = tmp
		*pdoc = arr
		return v, nil
	default:
		// Report the location of parent itself: the caller rebases it
		return nil, docError("", parent)
	}
}
