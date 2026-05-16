# Qedit Architecture

## Purpose
This document fixes the target architecture for the ongoing refactor so that
mechanical code splits and future feature work move toward the same end state.

## Product Principles
- Modularity: editor state, UI, runtime orchestration, and integrations should
  evolve independently.
- Extensibility: built-in features and future plugins should use the same
  capability model.
- Behavioral profiles: `basic`, `helix`, and `vim` must be first-class runtime
  profiles, not just different keymaps over one hardcoded behavior engine.

## Current Direction
- AI integration is removed from the product surface and is not part of the
  target architecture.
- Refactoring should preserve single-threaded ownership of editor state in the
  main loop until a stronger event/effect boundary is introduced.
- Early phases prefer mechanical splits inside existing packages over large
  cross-package rewrites.
- Capability registries now exist for commands, sidebar modes, formatters,
  language support, git integration, and behavior profiles.
- `editor.profile` is now explicit runtime configuration and can also be
  changed from inside the editor via `:profile basic|helix|vim`.

## Current Implementation

The current codebase has not been physically split into the target packages
below. The actual package layout is:
- `cmd/qedit`: binary entry point.
- `internal/app`: runtime orchestration, screen setup, file monitoring, git,
  LSP, tree-sitter, and editor runtime requests.
- `internal/editor`: editor state, input/profile behavior, rendering helpers,
  buffer management, command/sidebar/language/git registries, and runtime
  request creation.
- `internal/ui`: tcell style and event adapters.
- `internal/treesitter`: parser/highlight engine. Async parse requests are
  versioned and coalesced by active path so the runtime can ignore stale parsed
  events.
- `internal/lsp`, `internal/gitinfo`, `internal/integrations`,
  `internal/config`, `internal/session`, and `internal/plugins`: external
  service and persistence boundaries.

`internal/editor` still contains UI-adjacent rendering and input code. Treat the
layering below as the desired direction, not the current directory structure.

## Target Layers

### `internal/editor/core`
Owns pure editor state and deterministic transitions:
- buffer text and cursor state
- selection and modal state
- undo/redo
- search state
- profile-driven command execution

Rules:
- no `tcell`
- no direct filesystem/process/network calls
- no direct config persistence

### `internal/ui`
Owns presentation and input adaptation:
- TUI rendering
- sidebar and picker presentation
- key and mouse event translation
- profile-aware key dispatch into core intents

### `internal/runtime`
Owns side effects and orchestration:
- file open/save/reload
- session persistence
- git/LSP/tree-sitter coordination
- background watchers

### `internal/plugins`
Owns plugin discovery and registration:
- manifests
- capability registry
- lifecycle hooks
- built-in and in-process Go extensions

Current implementation:
- `internal/plugins` provides an in-process registry for Go plugins.
- Example plugin: a profile sidebar plugin that registers a custom sidebar mode
  and command without editing core dispatch.

## Plugin Capability Model
The registry should expose explicit capabilities instead of ad-hoc editor hooks.
Initial capability groups:
- commands
- sidebar views
- formatters
- language support
- git integrations
- behavior profiles

Non-goal for now:
- dynamic `.so` loading is not required for the first plugin system iteration

## Behavioral Profiles
Profiles define semantics and default bindings together:

Users can open an interactive tutorial inside the editor with `:tutor`,
`:tutor helix`, or `:tutor vim`. Tutorial buffers are embedded scratch buffers:
they start clean, can be edited while following the lessons, and are not tied
to an on-disk path unless explicitly saved with `:w <path>`.

Profile support claims are test-scoped. `internal/editor/profile_conformance_test.go`
lists the advertised Vim and Helix profile commands and fails if that advertised
set diverges from the exercised conformance set. Vim core editing scenarios are
also compared against a real `vim -Nu NONE -N` process when `vim` is available
on the test machine. This lets release notes claim conformance for the declared
profile command set, not unbounded compatibility with every upstream Vim or
Helix feature.

The current support matrix lives in `docs/modes-compatibility.md`. Any new
profile command should land with a full-key simulation test and an update to
that matrix.

### `basic`
- non-modal text editing
- familiar movement/selection behavior
- command palette and sidebar remain available
- implemented as a first-class profile in the input router

### `helix`
- selection-first motions
- numeric counts for motions and line extension (`2w`, `3e`, `2x`)
- linewise `x` selection so `xd` removes whole lines and `xyp` duplicates
  selected lines in Helix order
- conformance-covered editing/navigation includes `P`, `b`, `f/F/t/T`, `gh`,
  `%`, `;`, uppercase WORD motions (`W/B/E`), selection indentation with
  `>`/`<`, replace-with-yanked-text (`R`), number increment/decrement
  (`Ctrl-a`, `Ctrl-x`), regex selection/splitting (`s`, `S`, `Alt-s`),
  alignment (`&`), primary selection cycling/removal, duplicate cursors
  (`C`, `Alt-C`), content cycling (`Alt-(`/`Alt-)`), selection case transforms
  (`~`, backtick, `Alt+backtick`), search-register selection expansion (`*`,
  `n/N` in select mode), syntax-node selection expansion (`Alt-o`, `Alt-i`),
  match-mode pair selection/surrounds (`mi`, `ma`, `ms`, `md`, `mr`), line
  comments (`Ctrl-c`), and jumplist navigation (`Ctrl-s`, `Ctrl-o`, `Ctrl-i`)
- buffer navigation is exposed through the Helix-style goto family (`gn`,
  `gp`, `ga`) and the `space b` buffer picker
- Helix window mode is backed by an editor split tree and covers new/current
  buffer splits (`Ctrl-w nv/ns`, `Ctrl-w v/s`), focus (`Ctrl-w hjkl`,
  `Ctrl-w w`), close/only (`Ctrl-w q/o`), swap/transpose (`Ctrl-w HJKL`,
  `Ctrl-w t`), and command splits (`:vs`, `:hs`)
- existing `g`, `m`, `z`, `space` command families
- command semantics stay close to current editor behavior
- implemented as the extracted legacy engine

### `vim`
- normal/insert/visual/visual-line/operator-pending/command modes
- counts and operators are profile semantics, not keymap aliases
- current MVP supports distinct normal/insert/visual behavior, operator
  pending, counts, linewise yank/paste, replace mode, undo-line, substitute
  command basics, and profile switching
- operator-pending state preserves the operator start position and accepts
  Vim-style counts before and after operators, for example `2d3d`, `d10j`,
  `dgg`, `D`, `C`, `s`, `S`, and `X`
- conformance-covered Vim additions include `cw`, `Y`, `%`, `>>`, `<<`, `~`,
  `.`, common text objects (`iw/aw`, `ip/ap`, quotes, brackets), uppercase
  WORD motions (`W/B/E`), backward word-end motions (`ge/gE`), case operators
  (`gu`, `gU`, `g~`), marks (`m`, `'`, backtick), sentence/paragraph motions,
  named and blackhole registers for covered yank/delete/paste flows, macro
  recording/replay (`q`, `@`, `@@`), jumplist navigation (`Ctrl-o`,
  `Ctrl-i`), number increment/decrement (`Ctrl-a`, `Ctrl-x`), and native
  non-selecting `f/F/t/T`
- Vim-style buffer commands include `:ls`, `:buffers`, `:b`, `:b#`, `:bn`,
  `:bp`, `:bnext`, `:bprevious`, `:bd`, and `:bd!`
- Vim tutor command coverage includes file info (`Ctrl-g`), read commands
  (`:r`, `:r !cmd`), shell commands (`:!cmd`), visual-range writes
  (`:'<,'>w`), and shared split/window commands (`Ctrl-w v/s`, `Ctrl-w w`,
  `:vs`, `:hs`)
- Vim tutor search options include `:set ic`, `:set noic`, `:set invic`,
  `:set hls is`, and `:nohlsearch`

## Refactor Order
1. Mechanical split inside `internal/editor`.
2. Stabilize architecture docs and acceptance criteria.
3. Move UI-specific logic out of core candidates.
4. Introduce `intent -> effect` boundary between core and runtime.
5. Add plugin registry and migrate built-in extensions onto capabilities.
6. Introduce behavior profile engine.

## Acceptance Criteria
- `go test ./...` remains green after each step.
- New sidebar modes do not require editing a central hardcoded menu switch.
- Runtime profile selection is explicit in config and not limited to keymap
  overrides.
- Core editing logic can be tested without `tcell` and without touching disk.
