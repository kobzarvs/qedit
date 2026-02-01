# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

qedit is a terminal-based text editor written in Go that combines vim/Helix-style modal editing with modern IDE features (LSP, tree-sitter syntax highlighting, git integration).

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

```
cmd/qedit/main.go          # Entry point: initializes logger, config, app
internal/
  app/app.go               # Main event loop, coordinates editor + UI + integrations
  editor/                  # Core editor (largest module, ~13K LOC)
    types.go               # Mode enum, action constants, Editor struct, Cursor
    state.go               # Constructor (New), open/close, getters/setters
    input.go               # HandleKey, HandleMouse, key routing
    render.go              # Render, statusline, commandline, popups
    edit.go                # Insert/delete/join/split, selection helpers
    undo.go                # Undo/redo stacks, changelog persistence
    search.go              # Search state, fuzzy/regex matching
    move.go                # Cursor motion commands
    actions.go             # execAction, execCommand
    sidebar.go             # Sidebar rendering and navigation
  ui/                      # tcell abstraction (styles, events, screen wrapper)
  config/                  # TOML config loading, language definitions
  lsp/                     # Language Server Protocol manager
  treesitter/              # Syntax highlighting engine
  gitinfo/                 # Git branch operations
  integrations/            # Clipboard, formatter, session store, terminal zoom
```

### Key Design Patterns

- **Single-threaded state ownership**: Main goroutine owns all editor state. Background workers (LSP, git) marshal results as events
- **Modal editing**: ModeNormal, ModeInsert, ModeCommand, ModeBranchPicker, ModeSearch
- **Action dispatch**: Key bindings map to action strings (e.g., "move_left"), dispatched through `execAction`
- **Embedded components**: Editor struct embeds Buffer, Selection, UndoManager, SearchState

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

## Configuration

Config directory: `~/.config/qedit/` (or `$XDG_CONFIG_HOME/qedit/` or `$QEDIT_CONFIG_HOME`)

Files:
- `config.toml` - Main config (editor settings, keymaps, theme)
- `theme/<name>.toml` - Theme definitions
- `history`, `search_history` - Command/search history

## Ongoing Refactoring

The editor module is being refactored per `docs/refactoring/editor.md`:
- **Phase 1** (mostly complete): Split monolith into focused files
- **Phase 2** (planned): Separate UI layer from core logic with interfaces
- **Phase 3** (planned): Extract internal components (Buffer, UndoManager, etc.)

When modifying editor code, maintain the file organization and avoid re-coupling split concerns.
