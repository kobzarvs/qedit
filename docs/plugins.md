# Plugins

## Purpose
`qedit` now has an in-process Go plugin layer in [`internal/plugins`]. The
goal is to let new capabilities plug into the editor without editing the main
input/render/runtime dispatch every time.

## Current Shape
A plugin implements:

```go
type Plugin interface {
	ID() string
	Register(*editor.Editor) error
}
```

The runtime registry lives in [`internal/plugins/registry.go`]. Plugins are
applied during editor bootstrap from `internal/app`.

## Available Extension Points
The editor currently exposes these registration hooks:

- `RegisterCommand`
- `RegisterSidebarMode`
- `RegisterBehaviorProfile`
- `RegisterFormatter`
- `RegisterLanguageFeature`
- `RegisterGitFeature`

Useful runtime-safe editor methods for plugins:

- `OpenSidebarMode`
- `ShowSidebarContent`
- `CurrentSidebarMode`
- `SetBehaviorProfile`
- `BehaviorProfile`
- `SetStatusMessage`
- `Notify`

Behavior profile plugins own the top-level key dispatch for their profile.
Built-in `helix` and `vim` profiles keep modal state such as pending counts,
operator start positions, text-object state, repeat recording, visual/select
mode, multiple selections/cursors, registers, macro recording, jumplist state,
replace mode, and split/window focus inside editor-owned state so plugins do
not need to patch shared input handlers.

Command plugins can register multiple aliases for one handler. Built-in file
commands use that path for Vim-compatible aliases such as `:ls`, `:buffers`,
`:b`, `:b#`, `:bd`, and legacy `:bc`, plus Helix-style split aliases such as
`:vs` and `:hs`. Vim tutor file commands (`:r`, `:r !cmd`, visual
`:'<,'>w`) are also registered through the same command layer.

## Example
[`internal/plugins/profile_sidebar.go`] adds a real plugin:

- registers a custom sidebar mode
- registers a `:profiles` command
- switches between `basic`, `helix`, and `vim` from plugin-owned UI

This plugin is intentionally small, but it proves that:

- new sidebar capabilities can be added without editing the core sidebar switch
- new commands can be added without editing a central command switch
- in-process Go extensions work through a dedicated plugin layer

## Current Limits
This is still the first plugin iteration. Not implemented yet:

- dynamic `.so` loading
- plugin manifests/version negotiation
- dependency resolution
- isolated plugin lifecycle beyond registration time
- out-of-process RPC plugins

The current design is meant to stabilize the capability API first, then add a
broader plugin runtime on top of it.
