# Syntax Highlighting

## Current Status

Syntax highlighting is implemented in `internal/treesitter/engine.go` and is
driven by language detection from `config/languages.toml`.

Supported parser-backed languages:
- Go
- JavaScript and JSX
- TypeScript and TSX
- Vue
- Markdown, including fenced code blocks for supported embedded languages
- YAML
- TOML
- Bash

Regex-backed languages:
- JSON and JSONC
- `.gitignore` and `.dockerignore`
- Makefile variants

## Runtime Flow

For regular files, `internal/app` opens the file, detects the language, and then
parses it through the tree-sitter engine. Smaller synchronous languages apply
initial visible-range highlights immediately. JavaScript, TypeScript, TSX, JSX,
and Vue parse asynchronously because they are more likely to be large or slow.

Async parse requests are versioned. The engine keeps the latest pending text per
path, skips stale parse results, and emits parsed events with the parse version.
The runtime applies highlights only when the parsed event matches the latest
requested version for the active buffer.

During async reparsing after an edit, cached highlight spans for edited rows are
invalidated so stale syntax colors are not rendered while tree-sitter catches up.
Bulk edits without a precise `TextEdit` clear the visible highlight cache until
the next matching parsed event arrives.

Huge-file mode is different: files above the huge-file threshold or with very
long sampled lines use a lazy buffer. Async highlighting is available only for
clean huge buffers. Once a huge buffer has unsaved overlay edits, syntax
highlighting is cleared until the file is saved/reopened or overlay-aware
highlighting is implemented.

## Configuration

`config/config.toml` controls `editor.highlight-max-bytes`. Files larger than
that limit skip regular syntax highlighting. `0` disables the size limit.

Theme color keys live in `config/theme/ayu.toml`; YAML has dedicated keys:
- `syntax-yaml-key`
- `syntax-yaml-value`
- `syntax-yaml-list-item`

When adding a file type or theme key, update the repository config and the user
config under `~/.config/qedit/` if that local config is expected to mirror repo
defaults.

## Tests

Relevant coverage:
- `internal/treesitter/engine_test.go` for parsing, windowed highlights, long
  lines, markdown fences, and async parse coalescing.
- `internal/editor/highlight_state_test.go` for span normalization,
  invalidation, clipping, and rendering walkers.
- `internal/app/editor_highlight_runtime_test.go` for runtime behavior while
  async reparsing is pending.
