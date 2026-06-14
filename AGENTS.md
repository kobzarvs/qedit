# Repository Guidelines

## Project Structure & Module Organization
`cmd/qedit/` contains the main entry point for the CLI/TUI binary. Core
implementation lives in `internal/` (notable areas: `editor/`, `ui/`,
`lsp/`, `treesitter/`, `config/`, `session/`, `plugins/`, and `logger/`). Runtime
configuration is stored in `config/` (TOML files and themes under
`config/theme/`). Static assets (logos) are in `assets/`. Design notes and
planning docs live under `docs/`. Vendored dependencies are in `vendor/`.

## Build, Test, and Development Commands
- `make build` — build `./cmd/qedit` into the `qedit` binary at repo root.
- `make run` — run the app from source. Pass args after the target
  (example: `make run -- --help`).
- `make test` — run all Go tests (`go test ./...`).
- `make fmt` — format Go code with `go fmt ./...` (gofmt rules).
- `make lint` — run `golangci-lint run` using `.golangci.yml`.
- `make tidy` — sync module files (`go mod tidy`).

## Coding Style & Naming Conventions
Use standard Go formatting (tabs, gofmt) and keep lint clean per
`golangci-lint`. Follow Go naming: exported identifiers use `MixedCaps`,
unexported are lower camel case, and package names are short, lower-case
words (matching existing `internal/*` package names). Tests must live in
`*_test.go` files alongside the package they cover.

For editor architecture changes:
- add new commands, sidebar modes, formatters, language/git integrations, and
  behavior profiles through the existing registries instead of new central
  `switch` blocks
- put profile-specific editing semantics into the behavior profile layer
  (`basic`, `helix`, `vim`) instead of branching in shared input handlers
- prefer extending the in-process plugin layer in `internal/plugins/` when the
  feature is meant to be pluggable rather than hardwired into bootstrap

## Testing Guidelines
Tests use the Go `testing` package and live next to source in `internal/*`.
Name tests `TestXxx` and keep them deterministic (avoid external services or
environment coupling). Run the full suite with `make test` before opening a
PR; target specific packages with `go test ./internal/editor` or
`go test ./internal/plugins` when iterating.

## Commit & Pull Request Guidelines
Commit messages follow Conventional Commits with optional scopes, as seen in
history (examples: `feat(editor): ...`, `fix(sidebar): ...`, `docs: ...`,
`refactor(keyboard): ...`, `test(editor): ...`, `chore(config): ...`). Keep
subjects short and action-oriented. PRs should include a concise summary,
linked issues when applicable, and the commands you ran (e.g., `make test`,
`make lint`). For UI/rendering changes, include a brief before/after
description or screenshot from the terminal.

## Configuration & Assets
If you add or modify defaults, update the relevant TOML in `config/` and note
the change in your PR description. Keep binary assets in `assets/` and avoid
adding large, uncompressed files to the repo.
When introducing new theme color keys, update both `config/theme/ayu.toml` and
`~/.config/qedit/theme/ayu.toml`.
When adding new commands or shortcuts, update both `config/config.toml` and
`~/.config/qedit/config.toml`.
When adding support for new file types, update both `config/languages.toml` and
`~/.config/qedit/languages.toml`.
When changing profile behavior, plugin APIs, or capability registration rules,
update `docs/architecture.md` and `docs/plugins.md` too.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

When the user types `/graphify`, invoke the `skill` tool with `skill: "graphify"` before doing anything else.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- Dirty graphify-out/ files are expected after hooks or incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
