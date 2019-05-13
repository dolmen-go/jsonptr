package jsonptr_test

import (
	"testing"

	"github.com/dolmen-go/jsonptr"
)

func TestMustValue(t *testing.T) {
	// Should not raise exception
	_ = jsonptr.MustValue(jsonptr.Get([]interface{}{42}, "/0"))

	defer func() {
		if e := recover(); e == nil || e == error(nil) {
			t.Fatalf("ErrSyntax expected as panic but got %v", e)
		} else {
			t.Logf("%T %[1]v", e)
		}
	}()
	// Should raise exception
	_ = jsonptr.MustValue(jsonptr.Get(nil, "z"))
}
