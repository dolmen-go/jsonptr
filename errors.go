// Copyright 2016-2017 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

package jsonptr

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrSyntax   = errors.New("invalid JSON pointer")
	ErrIndex    = errors.New("invalid array index")
	ErrProperty = errors.New("property not found")

	ErrRoot = errors.New("can't go up from root")

	ErrUsage = errors.New("invalid use of jsonptr.UnescapeString on string with '/'")
)

type ptrError interface {
	error
	rebase(base string)
}

// BadPointerError signals JSON Pointer parsing errors
type BadPointerError struct {
	// Ptr is the substring of the original pointer where the error occurred
	BadPtr string
	// Err is ErrSyntax
	Err error
}

// Error implements the 'error' interface
func (e *BadPointerError) Error() string {
	return strconv.Quote(e.BadPtr) + ": " + e.Err.Error()
}

func (e *BadPointerError) rebase(base string) {
	if e != nil {
		e.BadPtr = base + e.BadPtr
	}
}

func syntaxError(ptr string) *BadPointerError {
	return &BadPointerError{ptr, ErrSyntax}
}

// PtrError signals JSON Pointer navigation errors
type PtrError struct {
	// Ptr is the substring of the original pointer where the error occurred
	Ptr string
	// Err is one of ErrIndex, ErrProperty
	Err error
}

// Error implements the 'error' interface
func (e *PtrError) Error() string {
	return strconv.Quote(e.Ptr) + ": " + e.Err.Error()
}

func (e *PtrError) rebase(base string) {
	if e != nil {
		e.Ptr = base + e.Ptr
	}
}

func indexError(ptr string) *PtrError {
	return &PtrError{ptr, ErrIndex}
}

func propertyError(ptr string) *PtrError {
	return &PtrError{ptr, ErrProperty}
}

// DocumentError signals a document that can't be processed by this library
type DocumentError struct {
	Ptr string
	Err error
}

// Error implements the 'error' interface
func (e *DocumentError) Error() string {
	return e.Err.Error()
}

func (e *DocumentError) rebase(base string) {
	if e != nil {
		e.Ptr = base + e.Ptr
	}
}

func docError(ptr string, doc interface{}) *DocumentError {
	return &DocumentError{ptr, fmt.Errorf("%q: not an object or array but %T", ptr, doc)}
}

func jsonError(ptr string, err error) *DocumentError {
	if e, ok := err.(*json.SyntaxError); ok {
		return &DocumentError{"", e}
	}
	return &DocumentError{ptr, err}
}
