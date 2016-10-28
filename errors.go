package jsonptr

import (
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

// PtrError is the structured error for JSON Pointer parsing or navigation
// errors
type PtrError struct {
	// Ptr is the substring of the original pointer where the error occurred
	Ptr string
	// Err is one of ErrSyntax, ErrIndex, ErrProperty
	Err error
}

// Error implements the 'error' interface
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
