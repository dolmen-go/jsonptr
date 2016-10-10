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
	return "/" + strings.Join(ptr[:], "/")
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
