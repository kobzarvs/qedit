# Syntax Highlighting Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add syntax highlighting for .gitignore and Makefile files, and improve YAML highlighting with independent color control for keys, list items, and values.

**Architecture:** Extend the existing treesitter/engine.go with regex-based highlighting for Makefile (since no tree-sitter grammar exists), enhance gitignore regex patterns, update YAML tree-sitter query, and add new YAML-specific theme fields throughout the config/styles pipeline.

**Tech Stack:** Go, tree-sitter (YAML), regex (gitignore, Makefile), tcell (terminal rendering)

---

## Task 1: Add Makefile Language Definition

**Files:**
- Modify: `config/languages.toml`

**Step 1: Add Makefile language entry**

Add after the gitignore entry in `config/languages.toml`:

```toml
[[language]]
name = "makefile"
file-types = ["Makefile", "makefile", "GNUmakefile", "mk"]
roots = []
```

**Step 2: Verify with manual test**

Open qedit and check that a file named `Makefile` is detected as "makefile" language (will show in statusline once highlighting is implemented).

**Step 3: Commit**

```bash
git add config/languages.toml
git commit -m "$(cat <<'EOF'
feat(config): add makefile language definition

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add YAML-specific Theme Fields to Config

**Files:**
- Modify: `internal/config/config.go:32-97` (Theme struct)

**Step 1: Add new fields to Theme struct**

Add these fields after `SyntaxParameter` (around line 65):

```go
SyntaxYAMLKey      string `toml:"syntax-yaml-key"`
SyntaxYAMLValue    string `toml:"syntax-yaml-value"`
SyntaxYAMLListItem string `toml:"syntax-yaml-list-item"`
```

**Step 2: Run tests to verify no breaking changes**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "$(cat <<'EOF'
feat(config): add YAML-specific theme fields

Adds syntax-yaml-key, syntax-yaml-value, syntax-yaml-list-item
for independent color customization of YAML elements.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Add YAML-specific Colors to Theme File

**Files:**
- Modify: `config/theme/ayu.toml`
- Modify: `~/.config/qedit/theme/ayu.toml`

**Step 1: Add YAML colors after syntax-parameter**

Add after `syntax-parameter` line:

```toml
# YAML-specific
syntax-yaml-key = "#E6B673"       # Same as field by default (gold)
syntax-yaml-value = "#B3B1AD"     # Same as foreground (plain text)
syntax-yaml-list-item = "#77B9E2" # Same as parameter (light blue)
```

**Step 2: Commit**

```bash
git add config/theme/ayu.toml
git commit -m "$(cat <<'EOF'
feat(theme): add YAML-specific syntax colors

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Add YAML Styles to UI Styles Resolution

**Files:**
- Modify: `internal/ui/styles.go:46-59` (color resolution)
- Modify: `internal/ui/styles.go:113-127` (style creation)
- Modify: `internal/ui/styles.go:161-188` (EditorStyles return)

**Step 1: Add color resolution after syntax-parameter (around line 59)**

```go
colors["syntax-yaml-key"] = resolve(theme.SyntaxYAMLKey, colors["syntax-field"])
colors["syntax-yaml-value"] = resolve(theme.SyntaxYAMLValue, colors["foreground"])
colors["syntax-yaml-list-item"] = resolve(theme.SyntaxYAMLListItem, colors["syntax-parameter"])
```

**Step 2: Add style creation after syntaxParameter (around line 126)**

```go
syntaxYAMLKey := style(colors["syntax-yaml-key"], colors["background"])
syntaxYAMLValue := style(colors["syntax-yaml-value"], colors["background"])
syntaxYAMLListItem := style(colors["syntax-yaml-list-item"], colors["background"])
```

**Step 3: Add to EditorStyles return (around line 188)**

```go
SyntaxYAMLKey:       syntaxYAMLKey,
SyntaxYAMLValue:     syntaxYAMLValue,
SyntaxYAMLListItem:  syntaxYAMLListItem,
```

**Step 4: Run build to verify**

Run: `go build ./...`
Expected: Build error (EditorStyles doesn't have these fields yet)

**Step 5: Commit (partial - will complete with editor types)**

Do NOT commit yet - continue to Task 5.

---

## Task 5: Add YAML Style Fields to Editor Types

**Files:**
- Modify: `internal/editor/ui_types.go` (EditorStyles struct)

**Step 1: Find EditorStyles struct and add fields**

Add after `SyntaxParameter` field:

```go
SyntaxYAMLKey      Style
SyntaxYAMLValue    Style
SyntaxYAMLListItem Style
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build error (Editor struct doesn't have these fields yet)

**Step 3: Do NOT commit yet**

---

## Task 6: Add YAML Style Fields to Editor Struct

**Files:**
- Modify: `internal/editor/types.go` (Editor struct - style fields section)

**Step 1: Find style fields section (styleSyntaxXxx) and add**

Add after `styleSyntaxParameter`:

```go
styleSyntaxYAMLKey      Style
styleSyntaxYAMLValue    Style
styleSyntaxYAMLListItem Style
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build error (fields not assigned in applyStyles)

---

## Task 7: Apply YAML Styles in Editor

**Files:**
- Modify: `internal/editor/style_default.go` (applyStyles function)

**Step 1: Find applyStyles function and add assignments**

Add after `e.styleSyntaxParameter = styles.SyntaxParameter`:

```go
e.styleSyntaxYAMLKey = styles.SyntaxYAMLKey
e.styleSyntaxYAMLValue = styles.SyntaxYAMLValue
e.styleSyntaxYAMLListItem = styles.SyntaxYAMLListItem
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: PASS

**Step 3: Commit all YAML theme changes together**

```bash
git add internal/ui/styles.go internal/editor/ui_types.go internal/editor/types.go internal/editor/style_default.go
git commit -m "$(cat <<'EOF'
feat(editor): add YAML-specific style support

Adds style fields and resolution for syntax-yaml-key,
syntax-yaml-value, and syntax-yaml-list-item.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Add YAML Kind Mappings to styleForHighlight

**Files:**
- Modify: `internal/editor/utils.go:162-195` (styleForHighlight function)

**Step 1: Add new cases in styleForHighlight switch**

Add after `case "parameter":` case:

```go
case "yaml-key":
    return e.styleSyntaxYAMLKey, true
case "yaml-value":
    return e.styleSyntaxYAMLValue, true
case "yaml-list-item":
    return e.styleSyntaxYAMLListItem, true
```

**Step 2: Add highlight priorities**

In `highlightPriority` function (around line 196), add cases:

```go
case "yaml-key":
    return 4
case "yaml-value":
    return 2
case "yaml-list-item":
    return 3
```

**Step 3: Run tests**

Run: `go test ./internal/editor/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/editor/utils.go
git commit -m "$(cat <<'EOF'
feat(editor): add YAML highlight kind mappings

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Update YAML Tree-Sitter Query

**Files:**
- Modify: `internal/treesitter/engine.go:939-954` (yamlHighlightQuery)

**Step 1: Replace yamlHighlightQuery constant**

Replace the existing query with:

```go
const yamlHighlightQuery = `
((comment) @comment)
((string_scalar) @yaml-value)
((double_quote_scalar) @yaml-value)
((single_quote_scalar) @yaml-value)
((integer_scalar) @number)
((float_scalar) @number)
((null_scalar) @constant)
((boolean_scalar) @constant)
((block_mapping_pair key: (_) @yaml-key))
((flow_pair key: (_) @yaml-key))
((block_sequence (block_sequence_item) @yaml-list-item))
((anchor_name) @keyword)
((alias_name) @keyword)
((tag) @type)
["," ":" "[" "]" "{" "}" ">" "|" "*" "&"] @punctuation
("-") @punctuation
`
```

**Step 2: Run build and test**

Run: `go build ./... && go test ./internal/treesitter/... -v`
Expected: PASS

**Step 3: Manual test**

Open a YAML file in qedit and verify:
- Keys are colored with yaml-key color (gold)
- String values are colored with yaml-value color
- List items have distinct coloring

**Step 4: Commit**

```bash
git add internal/treesitter/engine.go
git commit -m "$(cat <<'EOF'
feat(treesitter): update YAML query for distinct key/value/list colors

- Keys now use @yaml-key capture
- String values use @yaml-value capture
- List items use @yaml-list-item capture

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Enhance Gitignore Highlighting

**Files:**
- Modify: `internal/treesitter/engine.go:1042-1147` (gitignore patterns and function)

**Step 1: Update regex patterns**

Replace existing gitignore patterns (around line 1042) with:

```go
// Gitignore patterns
gitComment       = regexp.MustCompile(`^#.*`)
gitNegate        = regexp.MustCompile(`^!`)
gitDoubleGlob    = regexp.MustCompile(`\*\*`)
gitGlob          = regexp.MustCompile(`[*?]`)
gitCharRange     = regexp.MustCompile(`\[[^\]]+\]`)
gitEscape        = regexp.MustCompile(`\\[!#*?\[\]\\]`)
```

**Step 2: Replace highlightGitignoreLine function**

Replace the function (around line 1121) with:

```go
func (e *Engine) highlightGitignoreLine(line string) []HighlightSpan {
	lineRunes := []rune(line)
	lineLen := len(lineRunes)
	if lineLen == 0 {
		return nil
	}

	// Comments take precedence - entire line is comment
	if gitComment.MatchString(line) {
		return []HighlightSpan{{StartCol: 0, EndCol: lineLen, Kind: "comment"}}
	}

	var spans []HighlightSpan

	// Negation prefix (!)
	if loc := gitNegate.FindStringIndex(line); loc != nil {
		spans = append(spans, HighlightSpan{StartCol: 0, EndCol: 1, Kind: "keyword"})
	}

	// Rooted path prefix (/) - after potential negation
	offset := 0
	if len(line) > 0 && line[0] == '!' {
		offset = 1
	}
	if offset < len(line) && line[offset] == '/' {
		startRune := len([]rune(line[:offset]))
		spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: startRune + 1, Kind: "type"})
	}

	// Escape sequences (must be processed before globs)
	for _, loc := range gitEscape.FindAllStringIndex(line, -1) {
		startRune := len([]rune(line[:loc[0]]))
		endRune := len([]rune(line[:loc[1]]))
		spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "string"})
	}

	// Character ranges [...]
	for _, loc := range gitCharRange.FindAllStringIndex(line, -1) {
		startRune := len([]rune(line[:loc[0]]))
		endRune := len([]rune(line[:loc[1]]))
		spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "string"})
	}

	// Double glob patterns (**) - must be before single glob
	for _, loc := range gitDoubleGlob.FindAllStringIndex(line, -1) {
		startRune := len([]rune(line[:loc[0]]))
		endRune := len([]rune(line[:loc[1]]))
		spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "constant"})
	}

	// Single glob patterns (*, ?)
	for _, loc := range gitGlob.FindAllStringIndex(line, -1) {
		startRune := len([]rune(line[:loc[0]]))
		endRune := len([]rune(line[:loc[1]]))
		// Skip if this is part of ** (already highlighted as constant)
		isPartOfDouble := false
		for _, span := range spans {
			if span.Kind == "constant" && startRune >= span.StartCol && endRune <= span.EndCol {
				isPartOfDouble = true
				break
			}
		}
		if !isPartOfDouble {
			spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "operator"})
		}
	}

	// Directory marker (trailing /)
	if len(line) > 0 && line[len(line)-1] == '/' {
		spans = append(spans, HighlightSpan{StartCol: lineLen - 1, EndCol: lineLen, Kind: "type"})
	}

	return spans
}
```

**Step 3: Run tests**

Run: `go test ./internal/treesitter/... -v`
Expected: PASS

**Step 4: Manual test**

Open a `.gitignore` file and verify:
- `# comments` are gray
- `!negation` has keyword color
- `*.txt` has operator color on `*`
- `**/recursive` has constant color on `**`
- `[a-z]` ranges have string color
- `/rooted` and `dir/` have type color on `/`

**Step 5: Commit**

```bash
git add internal/treesitter/engine.go
git commit -m "$(cat <<'EOF'
feat(treesitter): enhance gitignore syntax highlighting

- Double globs (**) highlighted as constants
- Character ranges [...] highlighted as strings
- Escape sequences highlighted as strings
- Rooted paths (/) and directory markers highlighted as types
- Single globs (*, ?) highlighted as operators

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Add Makefile Regex Patterns

**Files:**
- Modify: `internal/treesitter/engine.go` (add patterns after gitignore patterns)

**Step 1: Add Makefile regex patterns**

Add after gitignore patterns (around line 1048):

```go
// Makefile patterns
makeComment   = regexp.MustCompile(`#.*$`)
makeTarget    = regexp.MustCompile(`^([a-zA-Z0-9_][a-zA-Z0-9_.-]*)\s*:(?!=)`)
makeVarAssign = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\s*(:=|\?=|\+=|::=|!=|=)`)
makeVarRef    = regexp.MustCompile(`\$\(([^)]+)\)|\$\{([^}]+)\}`)
makeAutoVar   = regexp.MustCompile(`\$[@<^?*%+]|\$\([@<^?*%+][DF]?\)|\$\{[@<^?*%+][DF]?\}`)
makeDirective = regexp.MustCompile(`^\s*(-?include|sinclude|vpath|override|export|unexport|define|endef|undefine|ifdef|ifndef|ifeq|ifneq|else|endif|\.PHONY|\.SUFFIXES|\.DEFAULT|\.PRECIOUS|\.INTERMEDIATE|\.SECONDARY|\.SECONDEXPANSION|\.DELETE_ON_ERROR|\.IGNORE|\.LOW_RESOLUTION_TIME|\.SILENT|\.EXPORT_ALL_VARIABLES|\.NOTPARALLEL|\.ONESHELL|\.POSIX)\b`)
```

**Step 2: Run build**

Run: `go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/treesitter/engine.go
git commit -m "$(cat <<'EOF'
feat(treesitter): add Makefile regex patterns

Patterns for targets, variables, directives, and automatic variables.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Add Makefile to Regex Language Switches

**Files:**
- Modify: `internal/treesitter/engine.go` (multiple switch statements)

**Step 1: Find and update OpenFile switch (around line 121)**

Change:
```go
case "json", "gitignore":
```
To:
```go
case "json", "gitignore", "makefile":
```

**Step 2: Find and update parseSync switch (around line 189)**

Change:
```go
case "json", "gitignore":
```
To:
```go
case "json", "gitignore", "makefile":
```

**Step 3: Find and update Highlights switch (around line 248)**

Change:
```go
case "json", "gitignore":
```
To:
```go
case "json", "gitignore", "makefile":
```

**Step 4: Run build**

Run: `go build ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/treesitter/engine.go
git commit -m "$(cat <<'EOF'
feat(treesitter): add makefile to regex-based language switches

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Implement Makefile Highlighting Function

**Files:**
- Modify: `internal/treesitter/engine.go` (add function and update regexHighlights)

**Step 1: Update regexHighlights switch (around line 1055)**

Add case:
```go
case "makefile":
    out[row] = e.highlightMakefileLine(line)
```

**Step 2: Add highlightMakefileLine function**

Add after highlightGitignoreLine function:

```go
func (e *Engine) highlightMakefileLine(line string) []HighlightSpan {
	lineRunes := []rune(line)
	lineLen := len(lineRunes)
	if lineLen == 0 {
		return nil
	}

	var spans []HighlightSpan

	// Comments (can appear anywhere, take rest of line)
	if loc := makeComment.FindStringIndex(line); loc != nil {
		startRune := len([]rune(line[:loc[0]]))
		spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: lineLen, Kind: "comment"})
		// Process only the part before comment
		line = line[:loc[0]]
		lineLen = len([]rune(line))
	}

	if lineLen == 0 {
		return spans
	}

	// Directives (.PHONY, include, ifdef, etc.)
	if matches := makeDirective.FindStringSubmatchIndex(line); matches != nil && len(matches) >= 4 {
		startRune := len([]rune(line[:matches[2]]))
		endRune := len([]rune(line[:matches[3]]))
		spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "keyword"})
	}

	// Variable assignment (VAR = value)
	if matches := makeVarAssign.FindStringSubmatchIndex(line); matches != nil && len(matches) >= 6 {
		// Variable name
		startRune := len([]rune(line[:matches[2]]))
		endRune := len([]rune(line[:matches[3]]))
		spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "variable"})
		// Assignment operator
		opStart := len([]rune(line[:matches[4]]))
		opEnd := len([]rune(line[:matches[5]]))
		spans = append(spans, HighlightSpan{StartCol: opStart, EndCol: opEnd, Kind: "operator"})
	} else if matches := makeTarget.FindStringSubmatchIndex(line); matches != nil && len(matches) >= 4 {
		// Target name (before colon)
		startRune := len([]rune(line[:matches[2]]))
		endRune := len([]rune(line[:matches[3]]))
		spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "function"})
	}

	// Recipe line indicator (tab at start)
	if len(line) > 0 && line[0] == '\t' {
		spans = append(spans, HighlightSpan{StartCol: 0, EndCol: 1, Kind: "punctuation"})
	}

	// Automatic variables ($@, $<, $^, etc.) - highlight before regular vars
	for _, loc := range makeAutoVar.FindAllStringIndex(line, -1) {
		startRune := len([]rune(line[:loc[0]]))
		endRune := len([]rune(line[:loc[1]]))
		spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "constant"})
	}

	// Variable references $(VAR) or ${VAR}
	for _, loc := range makeVarRef.FindAllStringIndex(line, -1) {
		startRune := len([]rune(line[:loc[0]]))
		endRune := len([]rune(line[:loc[1]]))
		// Skip if already highlighted as automatic variable
		isAuto := false
		for _, span := range spans {
			if span.Kind == "constant" && startRune >= span.StartCol && startRune < span.EndCol {
				isAuto = true
				break
			}
		}
		if !isAuto {
			spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "variable"})
		}
	}

	return spans
}
```

**Step 3: Run build and tests**

Run: `go build ./... && go test ./internal/treesitter/... -v`
Expected: PASS

**Step 4: Manual test**

Create a test Makefile and open in qedit:
```makefile
# Comment
.PHONY: all clean

CC = gcc
CFLAGS := -Wall -O2

all: $(TARGET)
	$(CC) $(CFLAGS) -o $@ $<

clean:
	rm -f $(TARGET)
```

Verify:
- `# Comment` is gray
- `.PHONY` is keyword color
- `CC`, `CFLAGS` are variable color
- `=`, `:=` are operator color
- `all`, `clean` are function color
- `$(CC)`, `$(CFLAGS)`, `$(TARGET)` are variable color
- `$@`, `$<` are constant color

**Step 5: Commit**

```bash
git add internal/treesitter/engine.go
git commit -m "$(cat <<'EOF'
feat(treesitter): implement Makefile syntax highlighting

Highlights:
- Comments (#...)
- Directives (.PHONY, include, ifdef, etc.)
- Targets (name before colon)
- Variable assignments and references
- Automatic variables ($@, $<, $^, etc.)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 14: Final Integration Test

**Step 1: Build the project**

Run: `make build`
Expected: PASS

**Step 2: Run all tests**

Run: `make test`
Expected: PASS

**Step 3: Manual integration test**

Test all three file types:

1. Open a `.gitignore` file - verify enhanced highlighting
2. Open a `Makefile` - verify new highlighting works
3. Open a `.yaml` file - verify keys, values, list items have distinct colors

**Step 4: Final commit if any fixes needed**

If everything works:
```bash
git log --oneline -10  # Review commits
```

---

## Verification Checklist

- [ ] `make build` passes
- [ ] `make test` passes
- [ ] `.gitignore` files show enhanced highlighting (comments, globs, double globs, ranges)
- [ ] `Makefile` files show syntax highlighting (targets, variables, directives, comments)
- [ ] `.yaml` files show distinct colors for keys, values, and list items
- [ ] Theme file `config/theme/ayu.toml` has YAML-specific color entries
- [ ] User theme file `~/.config/qedit/theme/ayu.toml` updated with YAML-specific colors
- [ ] No regressions in existing syntax highlighting (Go, bash, etc.)

---

## Files Modified Summary

| File | Changes |
|------|---------|
| `config/languages.toml` | Add makefile language |
| `config/theme/ayu.toml` | Add YAML-specific colors |
| `~/.config/qedit/theme/ayu.toml` | Add YAML-specific colors (user config) |
| `internal/config/config.go` | Add YAML theme fields |
| `internal/ui/styles.go` | Add YAML style resolution |
| `internal/editor/ui_types.go` | Add YAML style fields to EditorStyles |
| `internal/editor/types.go` | Add YAML style fields to Editor |
| `internal/editor/style_default.go` | Add YAML style assignments |
| `internal/editor/utils.go` | Add YAML kind mappings |
| `internal/treesitter/engine.go` | Update YAML query, enhance gitignore, add Makefile |
