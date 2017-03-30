package jsonptr

import (
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
		return Pointer(nil), nil
	}
	if pointer[0] != '/' {
		return Pointer(nil), ErrSyntax
	}
	ptr := strings.Split(pointer[1:], "/")
	for i, part := range ptr {
		var err error
		if ptr[i], err = UnescapeString(part); err != nil {
			return Pointer(nil), err
		}
	}
	return Pointer(ptr), nil
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

// Clone returns a new, independant, copy of the pointer.
func (ptr Pointer) Clone() Pointer {
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

// In returns the value from doc pointed by ptr.
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
		default:
			// We report the error at the upper level
			return nil, docError(ptr[:i-1].String(), doc)
		}
	}
	return doc, nil
}

// Set changes a value in document pdoc at location pointed by ptr.
func (ptr Pointer) Set(pdoc *interface{}, value interface{}) error {
	// TODO Make an optimised implementation
	return Set(pdoc, ptr.String(), value)
}
