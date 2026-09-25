---
name: changelog
description: Conventions for writing CHANGELOG.md in this repository, and how to verify it. Use this skill whenever a release is being prepared, a version is about to be tagged, or an entry is added to or edited in CHANGELOG.md — including when the user asks for "release notes", "what changed since the last version", or a summary of commits for users.
---

# Writing CHANGELOG.md

The audience is a developer deciding whether to upgrade, and what to fix in
their code if they do. Everything below serves that: the reader must be able to
tell in one pass whether a release can be taken blindly, and if not, exactly
what to change.

## Layout of the file

```markdown
# Changelog

All notable changes to [jsonptr][]
are documented here.

## v1.1.0 (2026-09-24)

<lead paragraph, only when the release needs one>

### Added
### Fixed
### Changed
### Internal

## v1.0.1 (2026-05-29)

* Short bullets, no commit links: old releases are history, not upgrade advice.
```

Releases come newest first, each as `## vX.Y.Z (YYYY-MM-DD)` with the release
date. Keep the four section names and that order: what is new, what was broken,
what behaves differently, then what only concerns contributors. Omit a section
which has nothing to say rather than writing "nothing".

Separate bullets with a blank line and wrap the prose; bullets are several
sentences long here, so a dense block would be unreadable.

## The lead paragraph

Write one when the release changes observable behaviour. Say in the first
sentence whether the exported API is unchanged, because that is the question
the reader has, then say what did change, and send them to *Changed*:

> The exported API is unchanged: no identifier has been added, removed or
> modified. However bugs which made `Get`, `Set` and `Delete` panic or silently
> do nothing on serialized documents are fixed […] Those fixes change what some
> calls return: see *Changed* below before upgrading.

A release with only additions and internal work does not need one.

## Entries describe observed behaviour, not commits

Do not paraphrase commit subjects. Build the entry list by running the previous
release against the new code and comparing the results, so every claim is
something a user could observe:

```sh
mkdir -p /tmp/old && git archive v1.0.1 | tar -x -C /tmp/old
# then a small program which calls the same inputs against both, with
# `replace github.com/dolmen-go/jsonptr => /tmp/old` in one module
```

Name the symptom the user would have seen — `panics (unexpected key type)`,
`silently do nothing`, `modifies the byte slice it is given` — not the internal
cause. A reader recognizes their own bug report in a symptom, never in
"refactor Set".

`Fixed` is for what was broken. `Changed` is for behaviour which was not a bug
but is now different: error types, returned types, reported locations. When a
change requires the reader to edit code, put that instruction in bold at the
end of the bullet:

> **Code testing `errors.Is(err, ErrSyntax)` for an invalid index must now test
> `ErrIndex`.**

## Commit references

Every bullet of *Fixed* and *Changed* ends with a line naming the commits which
implemented it, so a reader can see the actual change:

```markdown
  Commit: [887246f][].
  Commits: [4f7aba2][], [d88a3f0][].
```

Singular or plural accordingly. The label is the 7-character short hash; the
definition holds the full 40-character hash, which stays valid if the repository
ever grows ambiguous short hashes.

Find the commit by the code it introduced, not by its subject — a rewrite often
carries several fixes, and a promising subject often changed nothing relevant:

```sh
git log -S 'for decoder.More()' --oneline v1.0.1..HEAD
```

Citing the same commit from several bullets is normal and correct when one
commit fixed several things.

## Links to Go symbols

Mention a symbol as inline code, linked to its documentation, using a
reference-style link whose label is the short package name, a dot, and the
symbol:

```markdown
[`Get`][jsonptr.Get]  [`Pointer.In`][jsonptr.Pointer.In]  [`json.RawMessage`][json.RawMessage]
```

Dots are valid in Markdown link labels, and the prefix keeps this repository's
symbols distinct from the standard library's while reading the source of the
file.

Link an inline code span only when it is *exactly* a symbol. Leave alone:

| Not linked | Why |
|---|---|
| `errors.Is(err, ErrSyntax)` | an expression, not a symbol |
| `[]json.RawMessage`, `map[string]interface{}` | type expressions; only the element type is a symbol, and nesting a link inside reads badly |
| `"x"`, `{}`, `"/a/b"` | literals |
| `Ptr` | a struct field: pkg.go.dev has no anchor for it, so the link would 404 |

Link the first occurrence of a symbol in each bullet only. Every occurrence
would turn the prose into a wall of links; once per bullet keeps each bullet
self-sufficient for a reader who lands on it.

## Reference-style links everywhere

Long URLs inline make the source unreadable while editing. Keep every link to an
absolute URL reference-style, and use the collapsed form when the label equals
the text:

```markdown
Commit: [887246f][].
All notable changes to [jsonptr][]
```

A link to a file of the repository is short enough to stay inline, where it is
easier to read than a label resolved forty lines below:

```markdown
* Add this [CHANGELOG](CHANGELOG.md).
```

The definitions of a version go at the end of the section documenting that
version, not at the end of the file. A section then carries its own links: it
can be read while editing, or copied out whole — into a GitHub release, an
issue, a mail — without hunting for definitions at the other end of a file
which only grows.

```markdown
## v1.1.0 (2026-09-24)

### Internal

* Statement coverage of the testsuite raised from 77.9% to 96.0% […]

[json.RawMessage]: https://pkg.go.dev/encoding/json#RawMessage
[jsonptr.Get]: https://pkg.go.dev/github.com/dolmen-go/jsonptr#Get
[887246f]: https://github.com/dolmen-go/jsonptr/commit/887246f9642e76b9e78fb0154b83aa32f1cd184b

## v1.0.1 (2026-05-29)
```

Inside that block, symbols come first, sorted alphabetically, then commits in
order of first use. The links of the preamble, such as the package link, are
defined the same way at the end of the preamble, before the first version
section.

A section defines every label it uses, so the same label is expected to appear
again in another section. Markdown resolves a reference with the first matching
definition, which makes the repetition harmless — while the same label pointing
to two different URLs is always a mistake.

## The Internal section

For contributors, not users: test suite, refactorings, CI. Report test coverage
as the package-wide statement coverage before and after, measured rather than
estimated, and resist adding a per-file or per-function breakdown — the point is
the trend, and detail here ages badly:

> Statement coverage of the testsuite raised from 77.9% to 96.0% ; added some
> benchmarks and fuzzing tests.

Measure the old figure from the tag, not from memory:

```sh
(cd /tmp/old && go test -cover ./...)   # the previous release
go test -cover ./...                     # now
```

## Scripted edits

Bulk edits to this file (adding links to a whole section, converting link
styles) are worth scripting, and the author writes such throwaway scripts in Go
with `goeval`, not in Python:

`goeval -` reads the program on its standard input, so a quoted here-document
keeps it readable and leaves the shell out of its quoting:

```sh
goeval -i os -i log -i strings -i regexp -i fmt - <<'EOF'
data, err := os.ReadFile("CHANGELOG.md")
if err != nil {
	log.Fatal(err)
}
…
EOF
```

Go raw strings cannot hold the backticks of Markdown code spans, so key
replacements on backtick-free substrings, and let such a program fail loudly
(`log.Fatal`) on anything it did not expect rather than write a half-edited
file.

## Reflow the source, last, and only the latest release

The prose is wrapped to about 80 columns, so the Markdown source stays readable
in a terminal and its diffs stay small and reviewable.

Reflow only the section of the release being prepared. The preamble and the
older releases are published history: rewrapping them changes lines nobody is
reviewing, and hides the one section which matters in a diff spanning the whole
file. The tool below enforces this, so an old section keeps whatever width it
was written with, even past 80 columns.

Do not rewrap while the text is still moving. Editing a sentence in the middle
of a wrapped paragraph reshuffles every following line, which buries the actual
change in the diff, and the author edits these entries by hand. So write and
edit with whatever line breaks come naturally, and once the content is settled
— the entries are complete, the links verified, and the author has had the file
in front of them to review and edit — *offer* to reflow it:

> The entries look settled. Want me to reflow the source to 80 columns?

Reflow only on a yes, in a commit of its own, and check that nothing but the
line breaks moved:

```sh
cp CHANGELOG.md /tmp/before.md
go run ./.claude/skills/changelog/scripts/reflow CHANGELOG.md
# the sequence of words must be identical
diff <(tr -s ' \n' '\n\n' </tmp/before.md) <(tr -s ' \n' '\n\n' <CHANGELOG.md)
```

The tool rewraps the first `## ` section and copies the rest of the file
verbatim. Inside that section it keeps headings, link definitions and the
`Commit:` lines on their own line, and never breaks inside a link or an inline
code span: a newline between `]` and `[` would turn a reference into literal
text.

`reflow -c` reports the lines of the latest release which are too wide instead
of rewriting them. It is what the CI workflow runs
(`.github/workflows/changelog.yaml`), because the invariant worth enforcing is
the width a reader sees, not the exact wrapping of this tool: a paragraph
wrapped by hand at 78 columns is fine, and being nagged for it on every
work-in-progress commit would not be.

```sh
go run ./.claude/skills/changelog/scripts/reflow -c CHANGELOG.md
```

## Verify before committing

`scripts/check-refs` checks the link integrity of the file, section by section:
every reference resolves to a definition of its own section, no definition is
unused, no label is defined twice or points to two URLs, and no link to an
absolute URL stayed inline (a relative one is left alone).
Run it after any edit — a broken reference renders as literal brackets, which is
easy to miss in a long file. It exits non-zero on findings, so `go run` adds an
`exit status 1` line of its own:

```sh
go run ./.claude/skills/changelog/scripts/check-refs CHANGELOG.md
```

Then confirm that each pkg.go.dev anchor is a real identifier, which catches a
renamed or misspelled symbol:

```sh
grep -o 'jsonptr#[A-Za-z.]*' CHANGELOG.md | sed 's/.*#//' | sort -u |
  while read -r a; do go doc ".$a" >/dev/null 2>&1 || echo "MISSING: $a"; done
```

Reading the rendered file once is also worth it (`glow CHANGELOG.md`): unresolved
references and stray formatting are obvious there and invisible in the source.
