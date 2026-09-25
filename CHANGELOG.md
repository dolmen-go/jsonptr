# Changelog

All notable changes to [jsonptr][]
are documented here.

[jsonptr]: https://pkg.go.dev/github.com/dolmen-go/jsonptr

## v1.1.0 (2026-09-25)

The exported API is unchanged: no identifier has been added, removed or
modified. However bugs which made [`Get`][jsonptr.Get], [`Set`][jsonptr.Set] and
[`Delete`][jsonptr.Delete] panic or silently do nothing on serialized documents
are fixed, and errors are now reported consistently whatever the representation
of the document. Those fixes change what some calls return: see *Changed* below
before upgrading.

### Added

* [`Set`][jsonptr.Set] and [`Delete`][jsonptr.Delete] decode a serialized part
  of a document lazily: only the containers on the path to the value are
  decoded, one layer at a time, and the values outside of that path are kept as
  [`json.RawMessage`][json.RawMessage]. `Set` and `Delete` may therefore leave
  `[]json.RawMessage` or `map[string]json.RawMessage` containers in the
  document, which [`Get`][jsonptr.Get] accepts back.

* Partially deserialized documents are now first-class: `[]json.RawMessage` and
  `map[string]json.RawMessage` are accepted by [`Get`][jsonptr.Get],
  [`Pointer.In`][jsonptr.Pointer.In], [`Set`][jsonptr.Set] and
  [`Delete`][jsonptr.Delete], at any level of the document, mixed with the other
  representations.

* Documentation: the package documentation now presents the whole API;
  [`Get`][jsonptr.Get] documents which values are returned as stored and which
  are returned as a deserialized copy; [`JSONDecoder`][jsonptr.JSONDecoder]
  documents that it is consumed.

### Fixed

* [`Get`][jsonptr.Get] no longer panics (`unexpected key type`) when a property
  is not found in a [`json.RawMessage`][json.RawMessage] or a
  [`JSONDecoder`][jsonptr.JSONDecoder] document. It reports
  [`ErrProperty`][jsonptr.ErrProperty].
  Commit: [887246f][].

* [`Set`][jsonptr.Set] and [`Delete`][jsonptr.Delete] no longer silently do
  nothing when the value to modify is inside a
  [`json.RawMessage`][json.RawMessage]: the decoded value is now stored back in
  the document. They also no longer panic when a property on the path is
  missing.
  Commit: [880a8c8][].

* [`Set`][jsonptr.Set] no longer stores the JSON encoding of the
  [`json.Decoder`][json.Decoder] struct itself (`{}`) when given a
  [`JSONDecoder`][jsonptr.JSONDecoder] as the value.
  Commit: [4a9615d][].

* [`Pointer.UnmarshalText`][jsonptr.Pointer.UnmarshalText] no longer modifies
  the byte slice it is given, and no longer panics on some invalid escapes. The
  location it reports for a syntax error is now the invalid token instead of the
  empty pointer.
  Commit: [4f3895b][].

* [`Pointer.In`][jsonptr.Pointer.In] no longer panics when the first token is
  applied to a value which is not an object or an array.
  Commit: [abe215f][].

* On error, a [`JSONDecoder`][jsonptr.JSONDecoder] which has been read is
  replaced in the document by the raw value read from it, so the document
  remains usable.
  Commits: [4f7aba2][], [d88a3f0][].

* [`DocumentError.Error`][jsonptr.DocumentError.Error] now includes the
  location, like the other error types, and stays consistent with the `Ptr`
  field when the error is rebased.
  Commit: [4fac752][].

* Add this [CHANGELOG](CHANGELOG.md). Drafted by AI, but carefully edited by its
  human master.

### Changed

These changes are source compatible, but code which inspects errors or uses the
value returned by [`Delete`][jsonptr.Delete] may need an update:

* An error reported because a value can't be traversed
  ([`DocumentError`][jsonptr.DocumentError]) now points to that value instead of
  the location below it: `Get(doc, "/a/b")` where `/a` is a number reports
  `"/a"` instead of `"/a/b"`.
  Commits: [1cf28c3][], [bbe5c28][].

* [`Get`][jsonptr.Get] deserializes deeply the value it returns when that value
  is a [`json.RawMessage`][json.RawMessage], a
  [`JSONDecoder`][jsonptr.JSONDecoder], a `[]json.RawMessage` or a
  `map[string]json.RawMessage`. The result is then a copy: modifying it does not
  modify the document. A value which is already deserialized is still returned
  as stored, so modifying it does modify the document.
  Commit: [bd5a895][].

* [`Set`][jsonptr.Set] rewrites every container on the path to the value as a
  `map[string]interface{}` or a `[]interface{}`.
  Commit: [4a9615d][].

* [`Delete`][jsonptr.Delete] returns the value as it is stored in the document.
  When the container of the value is a [`json.RawMessage`][json.RawMessage], a
  `[]json.RawMessage` or a `map[string]json.RawMessage`, the value is therefore
  returned as a `json.RawMessage` instead of a deserialized value. **Code
  type-asserting the result of `Delete` on a serialized document must now handle
  `json.RawMessage`** (`jsonptr.Get(v, "")` deserializes it).
  Commit: [d88a3f0][].

* [`Delete`][jsonptr.Delete] reports an index which is out of range, or `"-"`,
  as a [`*PtrError`][jsonptr.PtrError] wrapping [`ErrIndex`][jsonptr.ErrIndex]
  instead of a [`*BadPointerError`][jsonptr.BadPointerError], like
  [`Get`][jsonptr.Get] does.
  Commit: [d88a3f0][].

* A token which is not a valid array index (`"x"`, `"01"`, `"-1"`, …) is now
  reported as a [`*PtrError`][jsonptr.PtrError] wrapping
  [`ErrIndex`][jsonptr.ErrIndex] instead of a
  [`*BadPointerError`][jsonptr.BadPointerError] wrapping
  [`ErrSyntax`][jsonptr.ErrSyntax]. An invalid escape (`"~2"`) is still a
  `*BadPointerError` wrapping `ErrSyntax`, whatever the container. **Code
  testing `errors.Is(err, ErrSyntax)` for an invalid index must now test
  `ErrIndex`.**
  Commits: [58ab2dc][], [de73458][].

### Internal

* Statement coverage of the testsuite raised from 77.9% to 96.0% ; added some
  benchmarks and fuzzing tests.

* Tests run in parallel, and check the kind and the location of every error for
  each representation of the document.

* [`Set`][jsonptr.Set] and [`Delete`][jsonptr.Delete] are reimplemented as
  recursive functions which walk the document a single time.

[json.Decoder]: https://pkg.go.dev/encoding/json#Decoder
[json.RawMessage]: https://pkg.go.dev/encoding/json#RawMessage
[jsonptr.BadPointerError]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#BadPointerError
[jsonptr.Delete]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#Delete
[jsonptr.DocumentError]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#DocumentError
[jsonptr.DocumentError.Error]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#DocumentError.Error
[jsonptr.ErrIndex]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#ErrIndex
[jsonptr.ErrProperty]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#ErrProperty
[jsonptr.ErrSyntax]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#ErrSyntax
[jsonptr.Get]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#Get
[jsonptr.JSONDecoder]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#JSONDecoder
[jsonptr.Pointer.In]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#Pointer.In
[jsonptr.Pointer.UnmarshalText]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#Pointer.UnmarshalText
[jsonptr.PtrError]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#PtrError
[jsonptr.Set]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#Set
[887246f]: https://github.com/dolmen-go/jsonptr/commit/887246f9642e76b9e78fb0154b83aa32f1cd184b
[880a8c8]: https://github.com/dolmen-go/jsonptr/commit/880a8c8d4fb3fac1df146b04eb2cbe3e9fc1aa74
[4a9615d]: https://github.com/dolmen-go/jsonptr/commit/4a9615db4b6c14a4f1a2b6dce73fdb9d0e8a350c
[4f3895b]: https://github.com/dolmen-go/jsonptr/commit/4f3895b42e91817210417edbb73ce8c6321dedfc
[abe215f]: https://github.com/dolmen-go/jsonptr/commit/abe215fc8c79628273763af8f2362f65f3c7e44f
[4f7aba2]: https://github.com/dolmen-go/jsonptr/commit/4f7aba22488be2f5b809967b84261509cdd36b93
[d88a3f0]: https://github.com/dolmen-go/jsonptr/commit/d88a3f05266f82f2e8d7b9596a53a08001f73120
[4fac752]: https://github.com/dolmen-go/jsonptr/commit/4fac7523d80c6c542e27ea3e005b4f0ca7d92911
[58ab2dc]: https://github.com/dolmen-go/jsonptr/commit/58ab2dc5393905a927fef92359ab234c264415a4
[de73458]: https://github.com/dolmen-go/jsonptr/commit/de73458c9aa526e059619c5cd46d3d6286f18f67
[1cf28c3]: https://github.com/dolmen-go/jsonptr/commit/1cf28c3f67dd08b3643feff1b395e84232752589
[bbe5c28]: https://github.com/dolmen-go/jsonptr/commit/bbe5c28d116b4306d281bd25d4806b0289522f2e
[bd5a895]: https://github.com/dolmen-go/jsonptr/commit/bd5a895ca3e288a839d08029fe05fed379217a18

## v1.0.1 (2026-05-29)

Note: the version tag has been added late in September 2026. That version has been also available as v0.0.0-20260529085001-d6b11e72da90.

* [`Parse`][jsonptr.Parse] reports more precisely the location of a syntax error.
* CI: migrate from Travis-CI to GitHub Actions, add Codecov.

[jsonptr.Parse]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#Parse

## v1.0.0 (2022-09-04)

Note: the version tag has been added late in September 2026. That version has been also available as v0.0.0-20220904212016-e3f38a361346.

First stable release.
