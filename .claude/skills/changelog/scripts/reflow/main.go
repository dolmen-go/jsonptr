// Copyright 2026 Olivier Mengué. All rights reserved.
// Use of this source code is governed by the Apache 2.0 license that
// can be found in the LICENSE file.

// reflow rewraps the prose of the section of the latest release of a changelog
// to a maximum line width, to keep the source readable and its diffs small.
//
// Only that section, the first one introduced by "## ", is touched: the
// preamble and the older releases are already published and reviewed, so
// rewrapping them would add noise to the diff without helping anyone.
//
// Inside that section, headings, link definitions and the "Commit:" lines are
// left on their own line. A link ([text][label]) and an inline code span are
// never split: a newline between "]" and "[" would turn a reference into
// literal text, and a code span reads badly across two lines.
//
// Usage: reflow [-w width] [file...]
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	heading    = regexp.MustCompile(`^#{1,6} `)
	definition = regexp.MustCompile(`^\[[^]]+\]: `)
	commit     = regexp.MustCompile(`^\s*Commits?: `)
	bullet     = regexp.MustCompile(`^([-*+] |\d+\. )`)
	// A link or an inline code span must not be split
	atom = regexp.MustCompile("\\[[^]]*\\]\\[[^]]*\\]|`[^`]*`")
)

// nbsp marks, while wrapping, a space which must not become a line break.
const nbsp = "\x00"

// wrap renders the words of text, the first line prefixed with first and the
// next ones with next, without exceeding width when it can be avoided.
//
// A word is a run of non-space characters, so the punctuation which follows a
// link stays attached to it; the spaces inside an atom are protected so that
// the atom stays in a single word.
func wrap(text, first, next string, width int) []string {
	text = atom.ReplaceAllStringFunc(text, func(a string) string {
		return strings.ReplaceAll(a, " ", nbsp)
	})
	words := strings.Fields(text)
	for i, word := range words {
		words[i] = strings.ReplaceAll(word, nbsp, " ")
	}
	if len(words) == 0 {
		return nil
	}
	var lines []string
	line := first + words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = next + word
			continue
		}
		line += " " + word
	}
	return append(lines, line)
}

// reflow rewraps the section of the latest release, and returns the file
// unchanged if it has no release section yet.
func reflow(source string, width int) string {
	lines := strings.Split(source, "\n")

	// The latest release is the first "## " section; it ends at the next one
	start, end := -1, len(lines)
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		if start < 0 {
			start = i
		} else {
			end = i
			break
		}
	}
	if start < 0 {
		return source
	}

	out := append([]string{}, lines[:start]...)
	out = append(out, rewrap(lines[start:end], width)...)
	return strings.Join(append(out, lines[end:]...), "\n")
}

// rewrap reflows the paragraphs of a single section.
func rewrap(lines []string, width int) []string {
	var out []string

	// The paragraph being accumulated
	var pending []string
	var first, next string
	flush := func() {
		if len(pending) > 0 {
			out = append(out, wrap(strings.Join(pending, " "), first, next, width)...)
			pending = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		// A blank line, a heading or a definition ends the paragraph and is
		// kept as it is
		case trimmed == "", heading.MatchString(line), definition.MatchString(line):
			flush()
			out = append(out, line)

		// The commit reference closes a bullet and stays on its own line
		case commit.MatchString(line):
			flush()
			out = append(out, line)

		case bullet.MatchString(trimmed):
			flush()
			marker := bullet.FindString(trimmed)
			indent := line[:len(line)-len(trimmed)]
			first, next = indent+marker, indent+strings.Repeat(" ", len(marker))
			pending = []string{strings.TrimPrefix(trimmed, marker)}

		// Any other line continues the current paragraph, or starts one
		default:
			if pending == nil {
				indent := line[:len(line)-len(trimmed)]
				first, next = indent, indent
			}
			pending = append(pending, trimmed)
		}
	}
	flush()

	return out
}

// tooWide reports the lines of the latest release which are wider than width
// and could be wrapped, which is what a reader of the source notices. It does
// not require the exact wrapping this tool produces: the author wraps by hand
// while writing, and only that section is ever reflowed.
func tooWide(source string, width int) []string {
	lines := strings.Split(source, "\n")
	start, end := -1, len(lines)
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		if start < 0 {
			start = i
		} else {
			end = i
			break
		}
	}
	if start < 0 {
		return nil
	}

	var wide []string
	for i, line := range lines[start:end] {
		switch {
		case len(line) <= width,
			definition.MatchString(line),
			// A single word cannot be wrapped: a long URL or link stays
			!strings.Contains(strings.TrimSpace(line), " "):
			continue
		}
		wide = append(wide, fmt.Sprintf("%d: %d columns", start+i+1, len(line)))
	}
	return wide
}

func main() {
	width := flag.Int("w", 80, "maximum line width")
	check := flag.Bool("c", false, "report the lines which are too wide, without rewriting")
	flag.Parse()

	for _, file := range flag.Args() {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if *check {
			wide := tooWide(string(data), *width)
			for _, w := range wide {
				fmt.Printf("%s:%s\n", file, w)
			}
			if len(wide) > 0 {
				os.Exit(1)
			}
			fmt.Printf("%s: no line wider than %d columns in the latest release\n", file, *width)
			continue
		}

		out := reflow(string(data), *width)
		if out == string(data) {
			fmt.Println(file, "unchanged")
			continue
		}
		if err := os.WriteFile(file, []byte(out), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(file, "reflowed")
	}
}
