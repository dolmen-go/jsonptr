// Copyright 2019-2026 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

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
