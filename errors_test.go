package jsonptr

import "encoding/json"

// Wrapper is a copy of https://godoc.org/golang.org/x/exp/errors#Wrapper
type Wrapper interface {
	// Unwrap returns the next error in the error chain.
	// If there is no next error, Unwrap returns nil.
	Unwrap() error
}

var (
	_ = []Wrapper{
		indexError("/1"),
		propertyError("/a"),
		syntaxError("azerty"),
		docError("", complex(1, 2)),
		jsonError("", &json.SyntaxError{Offset: 0}),
	}
)
