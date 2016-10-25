package jsonptr

import (
	"errors"
	"strconv"
	"strings"
)

// Pointer represents a mutable parsed JSON Pointer
type Pointer []string

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

func (ptr Pointer) String() string {
	if len(ptr) == 0 {
		return ""
	}

	for i, part := range ptr {
		if strings.ContainsAny(part, "~/") {
			escaped := make([]string, len(ptr))
			copy(escaped, ptr[:i])
			for j := i; j < len(ptr); j++ {
				escaped[j] = EscapeString(ptr[j])
			}
			return "/" + strings.Join(escaped, "/")
		}
	}
	return "/" + strings.Join(ptr, "/")
}

func (ptr Pointer) Clone() Pointer {
	return ptr[:]
}

func (ptr Pointer) IsRoot() bool {
	return len(ptr) == 0
}

var ErrRoot = errors.New("Can't go up from root")

func (ptr *Pointer) Up() *Pointer {
	if ptr.IsRoot() {
		panic(ErrRoot)
	}
	*ptr = (*ptr)[:len(*ptr)-1]
	return ptr
}

func (ptr *Pointer) Pop() string {
	last := len(*ptr) - 1
	if last < 0 {
		panic(ErrRoot)
	}
	prop := (*ptr)[last]
	*ptr = (*ptr)[:last]
	return prop
}

func (ptr *Pointer) Property(name string) *Pointer {
	*ptr = append(*ptr, name)
	return ptr
}

func (ptr *Pointer) Index(index int) *Pointer {
	var prop string
	if index < 0 {
		prop = "-"
	} else {
		prop = strconv.Itoa(index)
	}
	return ptr.Property(prop)
}

// In returns the value from doc pointed by ptr
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

func (ptr Pointer) Set(pdoc *interface{}, value interface{}) error {
	// TODO Make an optimised implementation
	return Set(pdoc, ptr.String(), value)
}
