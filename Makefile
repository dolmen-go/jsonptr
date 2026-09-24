

go ?= GO111MODULE=on go
export go

go-version: go.mod $(shell $(go) list -f '{{$$Dir := .Dir}}{{range .GoFiles}}{{$$Dir}}/{{.}} {{end}}' ./...)
	@TZ=UTC git log -1 '--date=format-local:%Y%m%d%H%M%S' --abbrev=12 '--pretty=tformat:v0.0.0-%cd-%h' $^

go-get:
	@echo $(go) get $(shell $(go) list .)@$(shell $(MAKE) -f $(firstword $(MAKEFILE_LIST)) go-version)

# Tools of the 'changelog' skill, which documents their use
changelog_tools = ./.claude/skills/changelog/scripts
changelog = CHANGELOG.md

# Check the reference-style links: fails if a reference, a definition or a
# section is inconsistent
changelog-check-refs:
	$(go) run $(changelog_tools)/check-refs $(changelog)

# Rewrap the section of the latest release to 80 columns
changelog-reflow:
	$(go) run $(changelog_tools)/reflow $(changelog)

