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
    behavior_profile.go    # Profile registry and mode/cursor presentation
    basic_profile.go       # Non-modal basic profile input engine
    vim_profile.go         # Vim profile MVP input engine
    types.go               # Mode enum, action constants, Editor struct, Cursor
    state.go               # Constructor (New), open/close, getters/setters
    input.go               # Top-level key dispatch into active profile
    edit.go                # Insert/delete/join/split, selection helpers
    command_registry.go    # Command capability registry
    sidebar_registry.go    # Sidebar mode capability registry
    formatter_registry.go  # Formatter capability registry
    language_feature_registry.go
    git_feature_registry.go
  ui/                      # tcell abstraction (styles, events, screen wrapper)
  config/                  # TOML config loading, language definitions
  lsp/                     # Language Server Protocol manager
  treesitter/              # Syntax highlighting engine
  gitinfo/                 # Git branch operations
  integrations/            # Clipboard, formatter, session store, terminal zoom
  plugins/                 # In-process Go plugin registry + example plugins
```

### Key Design Patterns

- **Single-threaded state ownership**: Main goroutine owns all editor state. Background workers (LSP, git) marshal results as events
- **Behavior profiles**: `basic`, `helix`, and `vim` each own their own input semantics
- **Capability registries**: commands, sidebar modes, formatters, language features, git features, and behavior profiles are registered instead of hardcoded into one switch
- **Runtime request/effect boundary**: editor enqueues runtime requests; `app` executes side effects
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
- Plugin integration tests live under `internal/plugins/`

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

## Ongoing Refactoring

The earlier editor-monolith split is largely complete. Current architectural
direction is documented in `docs/architecture.md` and `docs/plugins.md`.

When modifying editor code:
- avoid re-coupling profile-specific behavior back into shared input handlers
- prefer extending existing capability registries over adding new global dispatch
- use `internal/plugins/` for new in-process extensions when the feature should
  be pluggable rather than built directly into bootstrap
