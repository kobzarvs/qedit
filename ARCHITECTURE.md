# Architecture

## Overview
qedit is a single-process core with a tight event loop for input, editing, and rendering.
Background workers handle parsing and language servers.

```
Input (tcell) -> Keymap -> Commands -> Editor Core -> Render -> Screen
                               ^
                               | (LSP + Tree-sitter events)
```

## Planned Modules
- `internal/app`: bootstrap, event loop, lifecycle
- `internal/editor`: buffers, selections, undo/redo, registers
- `internal/ui`: layout, statusline, popups, renderer
- `internal/lsp`: JSON-RPC client, requests, diagnostics
- `internal/treesitter`: incremental parsing, queries
- `internal/plugin`: plugin host and API surface
- `internal/config`: config discovery and TOML parsing

## Concurrency Model
- Main goroutine owns the TUI and editor state.
- Background goroutines handle LSP, tree-sitter parsing, IO.
- All background results are marshaled into the main loop as events.

## Plugin Model
- In-process Go plugins using `plugin` (built with `-buildmode=plugin`).
  - Requires the same Go toolchain and compatible module versions.
- Out-of-process plugins for Python/Node/C/WASM using JSON-RPC over stdio.
  - Unified RPC API for commands, events, and diagnostics.

## Config
Config is stored under `~/.config/qedit/` and mirrors Helix structure where useful.
- `config.toml`: editor options, keymaps, themes
- `languages.toml`: language definitions, LSP server commands

## Performance Notes
- Render uses a minimal cell-diff against the previous frame.
- Avoid allocations on the hot path (input -> command -> render).
