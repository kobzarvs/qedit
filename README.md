# qedit

Fast TUI IDE for macOS/Linux. Baseline goal: full feature parity with Helix, then extend.

## Goals
- Very low input-to-render latency
- Selection-first editing model
- LSP + tree-sitter for language features
- Plugin system: in-process Go, out-of-process for other runtimes
- Optimized for Ghostty, compatible with other terminals

## Non-goals (for now)
- Windows support
- GUI frontend

## Build
Requires Go 1.25.6.

```
go mod tidy

go build ./cmd/qedit
./qedit
```

## Usage (current)
- Normal: `h/j/k/l`, arrows, `i` to insert, `:` for command, `u` undo, `Ctrl+r` redo, `q` to quit
- Insert: type to insert, `Esc` to normal
- Commands: `:w`, `:w <path>`, `:q`, `:q!`, `:wq`/`:x`, `:fmt`, `:ln abs|rel|off`
- Open file: `./qedit path/to/file` or `make run path/to/file`

## Config (planned)
- `~/.config/qedit/config.toml`
- `~/.config/qedit/languages.toml`

### Keymap config (current)
`~/.config/qedit/config.toml`

```
[editor]
tab-width = 4
line-numbers = "absolute"
git-branch-symbol = "git:"

[theme]
theme = "ayu"

[keymap.normal]
h = "move_left"
j = "move_down"
k = "move_up"
l = "move_right"
i = "enter_insert"
":" = "enter_command"
q = "quit"
u = "undo"
U = "redo"
home = "line_start"
end = "line_end"
"cmd+home" = "file_start"
"cmd+end" = "file_end"
"cmd+l" = "toggle_line_numbers"
"ctrl+home" = "file_start"
"ctrl+end" = "file_end"
"ctrl+a" = "file_start"
"ctrl+e" = "file_end"
pgup = "page_up"
pgdn = "page_down"
up = "move_up"
down = "move_down"
left = "move_left"
right = "move_right"
"ctrl+c" = "quit"
"ctrl+r" = "redo"

[keymap.insert]
esc = "enter_normal"
left = "move_left"
right = "move_right"
up = "move_up"
down = "move_down"
home = "line_start"
end = "line_end"
"cmd+home" = "file_start"
"cmd+end" = "file_end"
"cmd+l" = "toggle_line_numbers"
"ctrl+home" = "file_start"
"ctrl+end" = "file_end"
"ctrl+a" = "file_start"
"ctrl+e" = "file_end"
pgup = "page_up"
pgdn = "page_down"
backspace = "backspace"
enter = "newline"
tab = "insert_tab"
```

See `ARCHITECTURE.md` and `HELIX_PARITY.md` for the plan.

### Themes (current)
Themes live in `~/.config/qedit/theme/<name>.toml`.

Example: `~/.config/qedit/theme/ayu.toml`

```
foreground = "#B3B1AD"
background = "#0A0E14"
statusline-foreground = "#B3B1AD"
statusline-background = "#0F1419"
commandline-foreground = "#B3B1AD"
commandline-background = "#0F1419"
line-number-foreground = "#3E4B59"
line-number-active-foreground = "#B3B1AD"
```

### Languages config (current)
`~/.config/qedit/languages.toml`

```
[[language]]
name = "go"
file-types = ["go"]
roots = ["go.mod", ".git"]
language-servers = ["gopls"]

[language-server.gopls]
command = "gopls"
args = []
```

Tree-sitter is wired for Go only for now; other languages will be added as grammars are integrated.
