// Copyright 2026 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

// check-refs checks the integrity of the reference-style links of a Markdown
// file, following the convention that each section ("## ...", plus the preamble
// before the first one) defines every label it uses, at its end:
//
//   - a reference must have a definition in its own section, else it renders as
//     literal brackets, or silently borrows the definition of another section
//   - a definition must be used in its own section
//   - a label must not be defined twice in the same section
//   - a label repeated in several sections must always point to the same URL
//   - no link to an absolute URL must have stayed inline; a relative link, to a
//     file of the repository, is short enough to stay inline
//
// Usage: check-refs [file...]      (default: CHANGELOG.md)
//
// It exits with 1 if a file has findings, 2 if a file can't be read.
package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

var (
	definition = regexp.MustCompile(`^\[([^]]+)\]: +(\S+)`)
	reference  = regexp.MustCompile(`\[[^]]*\]\[[^]]*\]`)
	inline     = regexp.MustCompile(`\]\(https?://`)
)

// A label is used and defined per section, the preamble being section 0.
type key struct {
	section int
	label   string
}

type finding struct {
	line int
	msg  string
}

// check returns the findings of one file, ordered by line.
func check(source string) []finding {
	var findings []finding
	report := func(line int, format string, args ...interface{}) {
		findings = append(findings, finding{line, fmt.Sprintf(format, args...)})
	}

	section := 0
	title := []string{"preamble"}
	defined := make(map[key]int)   // where a label is defined
	used := make(map[key]int)      // how many times a label is referenced
	usedAt := make(map[key]int)    // where it is referenced first
	url := make(map[string]string) // the URL of a label, across sections

	for n, line := range strings.Split(source, "\n") {
		n++ // line numbers start at 1

		// A version section starts at "## ..." ("### ..." stays in the same one)
		if strings.HasPrefix(line, "## ") {
			section++
			title = append(title, line[3:])
			continue
		}

		if m := definition.FindStringSubmatch(line); m != nil {
			label, target := m[1], m[2]
			k := key{section, label}
			if _, ok := defined[k]; ok {
				report(n, "[%s] defined twice in %s", label, title[section])
			}
			defined[k] = n
			if previous, ok := url[label]; ok && previous != target {
				report(n, "[%s] points to %s and to %s", label, previous, target)
			}
			url[label] = target
			continue
		}

		for _, ref := range reference.FindAllString(line, -1) {
			// [text][label], or [text][] where the label is the text
			sep := strings.Index(ref, "][")
			label := ref[sep+2 : len(ref)-1]
			if label == "" {
				label = ref[1:sep]
			}
			k := key{section, label}
			if used[k]++; used[k] == 1 {
				usedAt[k] = n
			}
		}

		if inline.MatchString(line) {
			report(n, "inline link")
		}
	}

	for k, n := range usedAt {
		if _, ok := defined[k]; !ok {
			report(n, "[%s] used in %s but defined nowhere in it", k.label, title[k.section])
		}
	}
	for k, n := range defined {
		if used[k] == 0 {
			report(n, "[%s] defined in %s but not used in it", k.label, title[k.section])
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].line != findings[j].line {
			return findings[i].line < findings[j].line
		}
		return findings[i].msg < findings[j].msg
	})
	return findings
}

func main() {
	files := os.Args[1:]
	if len(files) == 0 {
		files = []string{"CHANGELOG.md"}
	}

	status := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		findings := check(string(data))
		for _, f := range findings {
			fmt.Printf("%s:%d: %s\n", file, f.line, f.msg)
		}
		if len(findings) > 0 {
			status = 1
		} else {
			fmt.Println(file+":", "links OK")
		}
	}
	os.Exit(status)
}
