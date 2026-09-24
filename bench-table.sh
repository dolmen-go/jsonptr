#!/bin/sh
# Copyright 2026 Olivier Mengué. All rights reserved.
# Use of this source code is governed by the Apache 2.0 license that
# can be found in the LICENSE file.

# bench-table.sh runs a benchmark which compares implementations over a set of
# cases, and renders the result as a table: one row per case, one column per
# implementation.
#
# The benchmark must report its results as "<Benchmark>/<case>/<implementation>",
# which is what a benchmark looping over cases and implementations with b.Run
# produces (see BenchmarkUnescapeString).
#
# The table is formatted for the terminal (with glow, else column) when stdout
# is a terminal, else it is left as Markdown. --md forces Markdown.
#
# Usage:
#   ./bench-table.sh [--md] [benchmark] [go test flags...]
#
# Examples:
#   ./bench-table.sh
#   ./bench-table.sh BenchmarkUnescapeString -benchtime 5s
#   ./bench-table.sh --md BenchmarkParse -count 6 > bench.md

set -eu

# Extract --md, keep the other arguments for go test
md=0
n=$#
i=0
while [ $i -lt $n ]; do
	arg=$1
	shift
	case $arg in
	--md) md=1 ;;
	*) set -- "$@" "$arg" ;;
	esac
	i=$((i + 1))
done

bench=${1:-BenchmarkUnescapeString}
[ $# -gt 0 ] && shift

# render formats a Markdown table for a terminal.
render() {
	if command -v glow >/dev/null 2>&1; then
		# Renders the borders, the alignment and the bold.
		# -w 0 disables the wrapping, which would truncate the columns to
		# the width of the terminal.
		glow -w 0 -
	elif command -v column >/dev/null 2>&1; then
		# Only aligns the columns: the bold markers are left as they are
		body=$(cat)
		printf '%s\n' "$body" |
			grep '^|' |
			sed '2d; s/^| //; s/ *|$//' |
			column -t -s '|' -o ' │ '
		# Whatever follows the table (the legend)
		printf '%s\n' "$body" | grep -v '^|' | sed '/^[[:space:]]*$/d' |
			while IFS= read -r line; do printf '\n%s\n' "$line"; done
	else
		cat
	fi
}

# -run '^$' skips the tests, keeping only the benchmarks
table=$(
	go test -run '^$' -bench "^${bench}\$" -benchmem "$@" . |
		awk -v bench="$bench" '
	# Collect "<Benchmark>/<case>/<impl>-<GOMAXPROCS> <iter> <v> ns/op <v> B/op <v> allocs/op"
	$1 ~ ("^" bench "/") {
		name = $1
		sub(/-[0-9]+$/, "", name)         # drop the -GOMAXPROCS suffix
		sub("^" bench "/", "", name)      # drop the benchmark name
		i = length(name)
		while (i > 0 && substr(name, i, 1) != "/") i--
		if (i == 0) next                  # not a <case>/<impl> benchmark
		case_ = substr(name, 1, i - 1)
		impl = substr(name, i + 1)

		ns = bytes = allocs = "?"
		for (f = 2; f <= NF; f++) {
			if ($f == "ns/op") ns = $(f - 1)
			else if ($f == "B/op") bytes = $(f - 1)
			else if ($f == "allocs/op") allocs = $(f - 1)
		}

		if (!(case_ in caseSeen)) { caseSeen[case_] = 1; cases[++nCases] = case_ }
		if (!(impl in implSeen)) { implSeen[impl] = 1; impls[++nImpls] = impl }
		nsOf[case_, impl] = ns
		cell[case_, impl] = ns " / " bytes " / " allocs
		next
	}

	# Report anything unexpected (build errors, FAIL, ...) on stderr
	/^(FAIL|ok|---|#|cannot|no such)/ { print > "/dev/stderr"; next }

	END {
		if (nCases == 0) {
			print "no results for " bench ": is it defined, and does it use b.Run(case/impl)?" > "/dev/stderr"
			exit 1
		}

		printf "| case |"
		for (j = 1; j <= nImpls; j++) printf " %s |", impls[j]
		printf "\n|---|"
		for (j = 1; j <= nImpls; j++) printf "---|"
		printf "\n"

		for (i = 1; i <= nCases; i++) {
			c = cases[i]
			# Find the fastest implementation of the row
			best = ""
			for (j = 1; j <= nImpls; j++) {
				v = nsOf[c, impls[j]]
				if (v == "" || v == "?") continue
				if (best == "" || v + 0 < best + 0) { best = v; bestImpl = impls[j] }
			}
			printf "| %s |", c
			for (j = 1; j <= nImpls; j++) {
				v = cell[c, impls[j]]
				if (v == "") v = "-"
				else if (impls[j] == bestImpl) v = "**" v "**"
				printf " %s |", v
			}
			printf "\n"
		}
		printf "\nns/op / B/op / allocs/op, fastest of each row in bold.\n"
	}
	'
)

if [ "$md" = 1 ] || [ ! -t 1 ]; then
	printf '%s\n' "$table"
else
	printf '%s\n' "$table" | render
fi
