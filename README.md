# jsonptr - JSON Pointer ([RFC 6901](https://tools.ietf.org/html/rfc6901)) for Go

[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/dolmen-go/jsonptr)
[![Travis-CI](https://img.shields.io/travis/dolmen-go/jsonptr.svg)](https://travis-ci.org/dolmen-go/jsonptr)
[![Go Report Card](https://goreportcard.com/badge/github.com/dolmen-go/jsonptr)](https://goreportcard.com/report/github.com/dolmen-go/jsonptr)

## Features

Goals:

1. First-class interface
    * Idiomatic
    * Short
    * Complete
    * Structured errors, not just strings: [`BadPointerError`](https://godoc.org/github.com/dolmen-go/jsonptr#BadPointerError), [`PtrError`](https://godoc.org/github.com/dolmen-go/jsonptr#PtrError), [`DocumentError`](https://godoc.org/github.com/dolmen-go/jsonptr#DocumentError)
    * Working at JSON data model level (tree of `[]interface{}`, `map[string]interface{}`) as well as serialized JSON ([`json.RawMessage`](https://golang.org/pkg/encoding/json/#RawMessage), [`json.Decoder`](https://golang.org/pkg/encoding/json/#Decoder))
2. Correctness (most existing open source Go implementations have limitations in their interface or have implementation bugs)
    * Full testsuite (work in progress)
    * Reject invalid escapes (regexp `/~[^01]/`)
    * Allow any JSON value as leaf node
    * Allow any JSON value as root (not just a `map[string]interface{}`)
    * Allow to get/set the root of the document with the empty pointer `""`
3. Speed (see [benchmark](https://github.com/dolmen-go/jsonptr-benchmark))
    * No reflect
    * Optimised parsing

## Example

```go
package main

import (
    "fmt"
    "github.com/dolmen-go/jsonptr"
)

func main() {
    // JSON: { "a": [ 1 ] }
    document := map[string]interface{}{
        "a": []interface{}{
            1,
        }
    }

    val, err := jsonptr.Get(document, `/a/0`)
    if err != nil {
        fmt.Printf("Got: %v\n", val)
    } else {
        fmt.Printf("Error: %s\n", err)
    }
}
```

## Status

Production ready.

The aim is code coverage of 100%. Use go coverage tools and consider any
code not covered by the testsuite as never tested and full of bugs.

Todo:
* tests of error cases
* `Delete`

## License

Copyright 2017 Olivier Mengué

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
