# Editor Refactoring Plan

## Context
`internal/editor/editor.go` has grown into a monolith that mixes core editing state,
input handling, UI rendering, pickers, LSP integration, formatting, clipboard,
undo/redo, and persistence. This makes the file hard to navigate, increases
coupling, and slows safe iteration.

## Goals
- Reduce file size and cognitive load by splitting responsibilities.
- Preserve behavior and public API in the first phase (pure refactor).
- Make future features (sidebar, file tree, history) easier to implement.
- Enable focused testing of isolated concerns.

## Non-goals
- No new features in Phase 1.
- No changes to event loop ownership (main goroutine still owns editor state).
- No concurrency model changes.

## Phase 0: Baseline Safety Checks
Before moving code, capture a baseline:
- `go test ./...`
- `go test -cover ./internal/editor` (record the coverage % in the PR).
- Identify test hotspots that protect critical behavior (undo/search/selection,
  key handling, rendering invariants).
- Add a Golden Master/snapshot test if possible: feed a deterministic key
  sequence and compare the rendered screen buffer to an approved snapshot.

## Phase 1: Mechanical Split (same package)
Purely move code into multiple files under `internal/editor` without changing
signatures or behavior.

Suggested file layout (first pass):
- `types.go`: consts, enums, structs (Mode, actions, Cursor, Editor fields).
- `state.go`: constructor (`New`), open/close, session state, getters/setters.
- `input.go`: `HandleKey`, `HandleMouse`, key routing, keymap helpers.
- `actions.go`: `execAction`, `execCommand`, action helpers.
- `edit.go`: insert/delete/join/split/indent, selection edit helpers.
- `undo.go`: undo/redo, changelog persistence.
- `search.go`: search state, fuzzy/regex matching and navigation.
- `render.go`: `Render`, statusline, commandline, popups, scroll indicator.
- `format.go`: `FormatGo`, `FormatMarkdownTables`, helpers.
- `pickers.go`: branch picker, refs picker, sidebar adapters.
- `lsp.go`: LSP goto/refs glue and types.
- `utils.go`: shared helpers (split/join lines, visual col, etc).

Rules for Phase 1:
- No logic changes; only move code.
- Keep tests and behavior identical.
- Keep package-level names and access unchanged.
- When possible, group functions to reduce incidental coupling on `Editor`.
- Make separate commits per block (input/render/edit/undo/search/etc.) to
  preserve `git blame` history.

Exit criteria:
- `go test ./...` passes.
- All editor features behave the same.
- `editor.go` is reduced to a small coordinator (or removed).
  - Coordinator responsibilities: type definitions (if not moved), and only
    top-level wiring like `HandleKey` and `Render` that delegate into helpers.

## Phase 2: Architectural Split (packages)
Align with `ARCHITECTURE.md` and split by layers:

- `internal/editor` (core, no tcell, no os/exec):
  - Buffer, selection, undo, search, format decision logic.
  - Exposes pure operations and state transitions.

- `internal/ui` (tcell rendering + input binding):
  - Screen render, statusline, pickers, sidebar, key routing.
  - Adapts UI events into core editor operations.

- `internal/integrations` (or existing packages):
  - `internal/lsp`, `internal/session`, `internal/gitinfo`, clipboard,
    formatters, etc.

Suggested interfaces to decouple core from integrations:
- `Clipboard` (Read/Write)
- `Formatter` (FormatGo/FormatMarkdown)
- `HighlightProvider` (HighlightRangeFunc)
- `GotoProvider` (LSP goto)
- `SessionStore` (load/save cursor/scroll)
- `Settings` / `Config` surface for keymaps, themes, UI options
  (owned by UI; injected into editor as needed).

Core depends only on interfaces, not implementations.

Exit criteria:
- UI renders identical output for the same editor state.
- Main loop still owns editor state; background workers marshal events.
- No performance regressions on key/render path.

## Phase 3: Internal Components (post-split)
After the package boundary is stable, introduce small internal structs to
reduce the core `Editor` surface area:
- `Buffer`: lines + cursor + insert/delete primitives.
- `Selection`: selection state + helpers.
- `UndoManager`: undo/redo stacks, grouping, persistence.
- `SearchState`: query, matches, navigation.

Note: render state should live in `internal/ui` after Phase 2.

Exit criteria:
- No behavior changes.
- Tests added around components where risk is higher (undo/search/selection).

## Sidebar vs Pickers
Sidebar is a UI construct and should remain in `internal/ui` alongside pickers.
If any sidebar logic is still in `internal/editor` after Phase 1, it becomes a
thin adapter in Phase 2 (UI handles view state; editor only exposes actions).

## Risk & Mitigation
- Risk: subtle behavior change during moves.
  - Mitigation: move-only commits; keep diff mechanical; re-run tests.
- Risk: hidden coupling between render and state.
  - Mitigation: add small tests or asserts around scroll/cursor invariants.
- Risk: performance regressions when adding layers.
  - Mitigation: keep hot path allocations stable; benchmark if needed.
- Risk: concurrency or async results (LSP/search) violate single-threaded
  ownership of editor state.
  - Mitigation: keep all state mutations in the main loop; marshal background
    results as events and apply them deterministically.

## Suggested PR slicing
1) Phase 1 split into files (no logic changes), commit by block (input, render,
   editing, undo, search, etc.).
2) Phase 2 UI/core split with interfaces (behind a compile-time adapter).
3) Phase 3 component extraction (`UndoManager`, `SearchState`, etc.) in small steps.

## Done Definition
- Clear module boundaries.
- `internal/editor` is small and focused.
- New contributors can locate code by concern quickly.
- Tests pass and behavior is unchanged from baseline.
