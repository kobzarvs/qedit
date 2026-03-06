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

### `basic`
- non-modal text editing
- familiar movement/selection behavior
- command palette and sidebar remain available

### `helix`
- selection-first motions
- existing `g`, `m`, `z`, `space` command families
- command semantics stay close to current editor behavior

### `vim`
- normal/insert/visual/operator-pending/command modes
- counts and operators are profile semantics, not keymap aliases

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
