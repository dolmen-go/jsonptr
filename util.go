// Copyright 2019-2026 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

package jsonptr

// MustValue allows to wrap a call to some jsonptr function which returns a value to transform any error into a panic.
func MustValue(v interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return v
}
