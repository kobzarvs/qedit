# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

qedit is a terminal-based text editor written in Go with first-class behavior
profiles for `basic`, `helix`, and `vim`, plus modern IDE features (LSP,
tree-sitter syntax highlighting, git integration). AI integration has been
removed from the product surface.

## Build & Development Commands

```bash
make build    # Build binary → ./qedit
make run      # Run with args: make run <file>
make test     # Run all tests: go test ./...
make lint     # Run golangci-lint
make fmt      # Format code: go fmt ./...
make tidy     # go mod tidy
```

Run a single test:
```bash
go test ./internal/editor -run TestFunctionName -v
```

Debug mode (enables zap logging):
```bash
QEDIT_DEBUG=1 ./qedit <file>
```

## Architecture

```text
cmd/qedit/main.go          # Entry point: initializes logger, config, app
internal/
  app/                     # Runtime orchestration, file/watch/git/LSP wiring
  editor/                  # Editor state machine and capability registries
  ui/                      # tcell abstraction (styles, events, screen wrapper)
  config/                  # TOML config loading, language definitions
  lsp/                     # Language Server Protocol manager
  treesitter/              # Syntax highlighting engine (async background worker)
  gitinfo/                 # Git branch operations
  integrations/            # Clipboard, formatter, session store, terminal zoom
  plugins/                 # In-process Go plugin registry + example plugins
```

### Key Design Patterns

- **Single-threaded state ownership**: Main goroutine owns all editor state. Background workers (LSP, git, tree-sitter) marshal results as events back into the main loop
- **Behavior profiles**: `basic`, `helix`, and `vim` each own their own input semantics via `BehaviorProfile.HandleKey`
- **Capability registries**: commands, sidebar modes, formatters, language features, git features, and behavior profiles are registered via `RegisterCommand`, `RegisterSidebarMode`, etc. — never hardcoded into a central switch
- **Runtime request/effect boundary**: editor enqueues `RuntimeRequest` objects (save, open, clipboard, format, etc.) via `enqueueRuntimeRequest()`; `app` drains and executes side effects via `ConsumeRuntimeRequest()` in the event loop
- **Embedded components**: Editor struct embeds Buffer, Selection, UndoManager, SearchState

### Runtime Request Flow

The editor never performs I/O directly. Side effects flow as:
1. Editor action calls `e.enqueueRuntimeRequest(kind, payload)`
2. App event loop calls `ed.ConsumeRuntimeRequest()` each tick
3. `editorRuntimeController` executes the side effect (file I/O, clipboard, LSP, persist config)
4. Results fed back as events on the next tick

Request kinds include: `RuntimeRequestSaveFile`, `RuntimeRequestOpenFile`,
`RuntimeRequestWriteClipboard`, `RuntimeRequestReadClipboard`,
`RuntimeRequestFormatBuffer`, `RuntimeRequestBufferSwitched`,
`RuntimeRequestPersistProfile`, `RuntimeRequestSaveHugeFile`.

### Input Dispatch Flow

1. `Editor.HandleKey(ev)` → `handleGlobalFocusHotkeys()` (Alt+ shortcuts)
2. If sidebar focused → `handleSidebarKey()`
3. Otherwise → `handleProfileKey()` → active profile's `HandleKey` function
4. Profile handler returns `bool` (true = consumed)
5. Common overlays handled in `handleCommonProfileOverlays()` (zoom, refs picker, keybindings help)

### Huge File Mode

Files ≥ 64 MB or with any line ≥ 128 KB activate huge mode. Detection happens
in `openRuntimeFile()` by sampling the first 1 MB.

Key differences in huge mode:
- `Editor.text` is **nil**; all line access goes through `HugeFileBuffer` (lazy line span indexing, multi-layer caches, async byte anchor seeding)
- Editing is limited: tracked as overlays in `editorHugeFileState.edits` and `.patches`, not in the main buffer
- Undo/redo not supported
- Rendering uses `prefetchHugeViewport()` and `hugeLineSegment()` to avoid loading full lines
- Session restore sets `deferInitialViewportWarm = true` to skip expensive first paint
- Saves use `RuntimeRequestSaveHugeFile` (separate from regular save)

### Buffer Management (Multi-File)

`BufferManager` tracks multiple `BufferState` snapshots. Switching files:
1. `snapshotBufferState()` — captures all per-file state (text, cursor, undo, selection, scroll, highlights)
2. Update buffer manager index
3. `restoreBufferState()` — writes fields back from snapshot
4. Enqueue `RuntimeRequestBufferSwitched`

Huge file buffers need explicit `Close()` when removing.

### Dependencies

- `tcell/v2` - Terminal rendering
- `go-tree-sitter` - Syntax parsing
- `BurntSushi/toml` - Config parsing
- `uber-go/zap` - Logging

## Testing Conventions

- Test files: `*_test.go` alongside source
- Test helpers in `internal/editor/test_helpers_test.go` for creating test editors
- Snapshot tests in `render_snapshot_test.go` for rendering verification
- Hotkey coverage tests in `hotkeys_*_test.go`
- Plugin integration tests live under `internal/plugins/`

## Commit Conventions

Conventional Commits with optional scopes:
`feat(editor): ...`, `fix(sidebar): ...`, `perf(editor): ...`,
`refactor(keyboard): ...`, `test(editor): ...`, `docs: ...`, `chore(config): ...`

## Configuration

Config directory: `~/.config/qedit/` (or `$XDG_CONFIG_HOME/qedit/` or `$QEDIT_CONFIG_HOME`)

Files:
- `config.toml` - Main config (editor settings, keymaps, theme)
- `theme/<name>.toml` - Theme definitions
- `history`, `search_history` - Command/search history

Important runtime settings:
- `editor.profile = "basic" | "helix" | "vim"`
- `:profile basic|helix|vim` switches and persists the active profile

When adding new theme color keys, update both `config/theme/ayu.toml` and
`~/.config/qedit/theme/ayu.toml`.
When adding new commands or shortcuts, update both `config/config.toml` and
`~/.config/qedit/config.toml`.
When adding support for new file types, update both `config/languages.toml` and
`~/.config/qedit/languages.toml`.

## Tree-sitter Gotchas

- The `go-tree-sitter` runtime uses ABI version 14 (min 13) — v15 grammars silently fail
- `ensureQuery()` caches `nil` on failure — once a query fails, it won't retry; validate queries before shipping
- TS and TSX use **separate** grammars: `LanguageTypescript()` for ts/js, `LanguageTSX()` for tsx/jsx
- Highlight queries must use node symbols (`(super)`, `(this)`) not string literals (`"super"`)

## Ongoing Refactoring

The earlier editor-monolith split is largely complete. Current architectural
direction is documented in `docs/architecture.md` and `docs/plugins.md`.

When modifying editor code:
- avoid re-coupling profile-specific behavior back into shared input handlers
- prefer extending existing capability registries over adding new global dispatch
- use `internal/plugins/` for new in-process extensions when the feature should
  be pluggable rather than built directly into bootstrap
