# Feature: Files Tree (Sidebar Mode)

## Overview
Режим файлового дерева для левого сайдбара.
Flat-list навигация по директориям с возможностью быстрого просмотра файлов.

**Зависимость**: требует реализации `Left Sidebar` (feature__left-sidebar__plan.md)

---

## Ключевые решения
- **Хоткей**: `Cmd+O` - открыть sidebar в режиме FileTree
- **Dotfiles**: по умолчанию скрыты, `a` или `.` показывает
- **Gitignore**: в git-репо скрываем gitignored файлы, `h` показывает
- **Preview mode**: `v` - auto-update при движении курсора

---

## 1. Data Structures

### File Tree State (Editor struct)
```go
// File tree mode state
fileTreeDir            string
fileTreeProjectRoot    string
fileTreeShowHidden     bool   // toggle with 'a' or '.'
fileTreeShowIgnored    bool   // toggle with 'h'
fileTreeGitRoot        string
fileTreeIgnorePatterns []gitignorePattern
fileTreePreviewMode    bool
fileTreeSelectedPath   string // for app.go to consume
```

### gitignore.go (new file)
```go
type gitignorePattern struct {
    Pattern  string
    IsDir    bool  // pattern ends with /
    Negation bool  // pattern starts with !
}
```

---

## 2. Configuration

### EditorOptions (config.go)
```go
// File tree specific (sidebar width is in Left Sidebar feature)
FileTreeShowHidden  bool `toml:"file-tree-show-hidden"`  // default false
FileTreeShowIgnored bool `toml:"file-tree-show-ignored"` // default false
```

### Example config.toml
```toml
[editor]
file-tree-show-hidden = false   # dotfiles
file-tree-show-ignored = false  # gitignored files
```

---

## 3. Key Bindings

### Global
| Key | Action |
|-----|--------|
| `Cmd+O` | Open sidebar → FileTree mode |

### In FileTree Mode (sidebar focused)
| Key | Action |
|-----|--------|
| `up/k` | Move up (from Sidebar) |
| `down/j` | Move down (from Sidebar) |
| `home/gg` | First item (from Sidebar) |
| `end/G` | Last item (from Sidebar) |
| `pgup/pgdn` | Page navigation (from Sidebar) |
| `enter/l/right` | Open file or enter directory |
| `backspace/left` | Go to parent directory |
| `Cmd+Home` | Go to project root |
| `v` | Toggle preview mode |
| `a` or `.` | Toggle dotfiles (hidden) |
| `h` | Toggle gitignored files |
| `esc` | Back to sidebar menu |
| `q` | Close sidebar |

---

## 4. Implementation Steps

### Step 1: Config
**File**: `internal/config/config.go`
- Add `FileTreeShowHidden bool` (default false)
- Add `FileTreeShowIgnored bool` (default false)
- Add merge logic in `Load()`

### Step 2: Gitignore Parsing
**File**: `internal/editor/gitignore.go` (new)
- `type gitignorePattern struct`
- `findGitRoot(dir string) string` - walk up to find .git
- `loadGitignore(gitRoot string) []gitignorePattern`
- `matchesGitignore(patterns, path, isDir) bool`
- Support: `*.ext`, `dir/`, `!negation`, `**/glob`

### Step 3: File Tree State
**File**: `internal/editor/editor.go`
- Add file tree state fields to Editor struct
- Initialize `fileTreeShowHidden/Ignored` from config in `New()`

### Step 4: File Listing
**File**: `internal/editor/filetree.go` (new)
- `(e *Editor) fileTreeOpen()` - activate FileTree mode
- `(e *Editor) fileTreeLoadDir(dir string) error`
  - Read directory
  - Mark items: IsDir, IsHidden (starts with `.`), IsIgnored
  - Filter based on show flags
  - Sort: ".." first, then dirs, then files (case-insensitive)
  - Convert to `[]SidebarItem`
  - Set `sidebar.Items`, `sidebar.Title` (truncated path)

### Step 5: Path Truncation
**File**: `internal/editor/filetree.go`
- `truncatePath(path, maxWidth) string`
- Format: `"/first/.../last"` when too long

### Step 6: Navigation
**File**: `internal/editor/filetree.go`
- `(e *Editor) fileTreeEnter()` - open file or enter dir
- `(e *Editor) fileTreeGoUp()` - parent directory
- `(e *Editor) fileTreeGoToProjectRoot()` - Cmd+Home
- `(e *Editor) fileTreeToggleHidden()` - 'a' or '.'
- `(e *Editor) fileTreeToggleIgnored()` - 'h'

### Step 7: Key Handling
**File**: `internal/editor/filetree.go`
- `(e *Editor) handleFileTreeKey(ev) bool`
- Handle mode-specific keys: enter, backspace, v, a, h, Cmd+Home
- Return false for unhandled → falls through to sidebar common

### Step 8: Preview Mode
**File**: `internal/editor/filetree.go`
- `(e *Editor) fileTreeTogglePreview()`
- `(e *Editor) fileTreePreviewCurrent()`
- Auto-update on cursor move when preview active
- Render preview in editor area (read-only, with syntax)

### Step 9: Editor Integration
**File**: `internal/editor/editor.go`
- In `handleNormal()`: `Cmd+O` → `fileTreeOpen()`
- In sidebar key routing: if mode == FileTree → `handleFileTreeKey()`
- On sidebar Enter action: call `fileTreeEnter()`
- `ConsumeFileTreeSelection() (string, bool)` for app.go

### Step 10: App Integration
**File**: `internal/app/app.go`
- Set `fileTreeProjectRoot` when opening file
- On `ConsumeFileTreeSelection()`: open selected file

### Step 11: Commands
**File**: `internal/editor/editor.go`
- `:tree` - toggle FileTree mode
- `:tree path` - open at specific path

---

## 5. File Listing Rules

1. `".."` always first (unless at "/")
2. Directories sorted alphabetically (case-insensitive)
3. Files sorted alphabetically (case-insensitive)
4. Directories before files
5. Dotfiles filtered when `fileTreeShowHidden = false`
6. Gitignored filtered when `fileTreeShowIgnored = false`
7. `.git` always hidden

---

## 6. Path Truncation

When path > sidebarWidth - 2:
```
/Users/diver/projects/myproject/src/components
→ /Users/.../components
```

```go
func truncatePath(path string, maxWidth int) string {
    if len(path) <= maxWidth {
        return path
    }
    parts := strings.Split(path, string(os.PathSeparator))
    if len(parts) <= 2 {
        return path[:maxWidth-3] + "..."
    }
    first := parts[0]
    if first == "" {
        first = "/"
    }
    last := parts[len(parts)-1]
    result := first + "/.../" + last
    if len(result) > maxWidth {
        // truncate last
        avail := maxWidth - len(first) - 5
        if avail > 3 {
            result = first + "/.../" + last[:avail-3] + "..."
        } else {
            result = path[:maxWidth-3] + "..."
        }
    }
    return result
}
```

---

## 7. Critical Files

| File | Changes |
|------|---------|
| `internal/config/config.go` | FileTreeShowHidden/Ignored |
| `internal/editor/gitignore.go` | **NEW** - gitignore parsing |
| `internal/editor/filetree.go` | **NEW** - FileTree mode logic |
| `internal/editor/editor.go` | FileTree state, key routing |
| `internal/app/app.go` | Project root, file open |

---

## 8. Verification

1. `make build` - компиляция
2. `make test` - тесты для gitignore.go, filetree.go
3. `make lint` - линтер
4. Manual testing:
   - `Cmd+O` opens FileTree mode
   - Navigate with j/k, enter directories
   - `v` enables preview, auto-updates on move
   - `a` shows/hides dotfiles
   - `h` shows/hides gitignored (in git repo)
   - Dimmed style for hidden/ignored when shown
   - `backspace` goes to parent
   - `Cmd+Home` returns to project root
   - Path truncation works for long paths
   - `:tree /path` opens at path
   - Select file → opens in editor
