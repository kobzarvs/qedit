# Feature: Files Tree (Sidebar Mode)

## Overview
Режим файлового дерева для левого сайдбара.
Flat-list навигация по директориям с возможностью быстрого просмотра файлов.

**Зависимость**: требует `Left Sidebar` (feature__left-sidebar__plan.md). В коде сайдбар уже реализован (`internal/editor/sidebar.go`), поэтому FileTree должен быть реализован как `SidebarContent`.

---

## Ключевые решения
- **Хоткей**: `Cmd+O` открывает sidebar сразу в режиме FileTree. Нужен новый action (`open_file_tree`) и бинды в дефолтном keymap + `config.toml`.
- **Dotfiles**: по умолчанию скрыты, `a` или `.` показывает.
- **Gitignore**: в git-репо скрываем gitignored файлы, `h` показывает.
- **Preview mode**: `v` — auto-update при смене выделения. Так как навигация обрабатывается контейнером Sidebar, обновление превью нужно делать на уровне `Editor.handleSidebarKey` после `Sidebar.HandleKey`.
- **Esc vs menu**: в FileTree `esc` возвращает в меню (override), `q` закрывает сайдбар (поведение контейнера по умолчанию).

---

## 1. Data Structures

### SidebarFileTreeContent (new, implements SidebarContent)
```go
type SidebarFileTreeContent struct {
    dir            string
    projectRoot    string
    showHidden     bool
    showIgnored    bool
    gitRoot        string
    ignorePatterns []gitignorePattern
    previewMode    bool
    items          []SidebarItem
    index          int
}
```

### Editor integration
```go
sidebarOpenFilePath string // set on SidebarActionOpenFile
```
`ConsumeSidebarOpenFile() (string, bool)` — для app.go (аналогично `ConsumeSidebarBranchSelection`).

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

### EditorOptions (config.go / options.go)
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

### Где обновлять
- `internal/config/config.go`: поля + дефолты + `Load()` (через `md.IsDefined`).
- `internal/editor/options.go`: новые поля в `Options`.
- `internal/app/app.go`: прокинуть поля в `editor.New(...)`.

---

## 3. Key Bindings

### Global
| Key | Action |
|-----|--------|
| `Cmd+O` | Open sidebar → FileTree mode (new action `open_file_tree`) |
| `Alt+1` | Toggle sidebar menu (existing) |

### In FileTree Mode (sidebar focused)
| Key | Action |
|-----|--------|
| `up/k` | Move up (from Sidebar) |
| `down/j` | Move down (from Sidebar) |
| `home/g` | First item (Sidebar handles single `g`, no `gg`) |
| `end/G` | Last item (from Sidebar) |
| `pgup/pgdn` | Page navigation (from Sidebar) |
| `enter` | Open file or enter directory |
| `right/l` | Open file or enter directory (handled in FileTree content) |
| `backspace/left` | Go to parent directory |
| `Cmd+Home` | Go to project root |
| `v` | Toggle preview mode |
| `a` or `.` | Toggle dotfiles (hidden) |
| `h` | Toggle gitignored files (override sidebar default back) |
| `esc` | Back to sidebar menu (override sidebar close) |
| `q` | Close sidebar |

---

## 4. Implementation Steps

### Step 1: Config + Keymap
- **Files**: `internal/config/config.go`, `internal/editor/options.go`, `internal/app/app.go`, `config/config.toml`
- Add `file-tree-show-hidden/ignored` options with defaults.
- Add new action `open_file_tree` and bind `cmd+o` in default keymaps and in sample `config.toml`.

### Step 2: Gitignore Parsing
**File**: `internal/editor/gitignore.go` (new)
- `type gitignorePattern struct`
- `findGitRoot(dir string) string` - walk up to find `.git`
- `loadGitignore(gitRoot string) []gitignorePattern`
- `matchesGitignore(patterns, path, isDir) bool`
- Support: `*.ext`, `dir/`, `!negation`, `**/glob`

### Step 3: File Tree Content
**File**: `internal/editor/sidebar_filetree.go` (new)
- `type SidebarFileTreeContent struct` implements `SidebarContent`
- `NewSidebarFileTreeContent(dir, projectRoot string, showHidden, showIgnored bool)`
- `Items()`, `Title()`, `Index()`, `SetIndex()`, `Refresh()`
- `Mode()` returns `SidebarModeFileTree`

### Step 4: File Listing
**File**: `internal/editor/sidebar_filetree.go`
- `loadDir(dir string) error`
  - Read directory
  - Mark items: IsDir, IsHidden (starts with `.`), IsIgnored
  - Filter based on show flags
  - Sort: ".." first, then dirs, then files (case-insensitive)
  - Convert to `[]SidebarItem` and store in content
  - `.git` всегда скрыт

### Step 5: Path Truncation (Header)
Контейнер `Sidebar.Render` уже обрезает `Title()` по ширине простым cut.
Если нужен формат `"/first/.../last"`:
- Добавить `truncatePath(path, maxWidth)` в filetree
- При открытии/обновлении брать ширину `e.sidebar.CalculateWidth(e.viewWidth)` и сохранять в `content.title`
- На ресайзе потребуется `Refresh()` или повторное обновление заголовка

### Step 6: Navigation
**File**: `internal/editor/sidebar_filetree.go`
- `enter()` — открывает файл или заходит в директорию
- `goUp()` — parent directory
- `goToProjectRoot()` — Cmd+Home
- `toggleHidden()` — `a` or `.`
- `toggleIgnored()` — `h`
- `OnEnter()` возвращает `SidebarActionOpenFile` для файлов (path в `ActionData`)

### Step 7: Key Handling
**File**: `internal/editor/sidebar_filetree.go`
- `HandleKey(ev)` перехватывает только mode-specific keys и не трогает `j/k` (пусть скролл/selection остаются у контейнера):
  - `left/backspace` → `goUp()`
  - `right/l` → `enter()`
  - `a` / `.` → toggle hidden
  - `h` → toggle ignored
  - `v` → toggle preview
  - `Cmd+Home` → go to project root
  - `Esc` → `SidebarActionBackToMenu` (override стандартного close)

### Step 8: Preview Mode
Так как `Sidebar` сам двигает selection, контент не знает о каждом `MoveUp/Down`.
Обновление превью делается в `Editor.handleSidebarKey`:
- после `action := e.sidebar.HandleKey(...)`
- если текущий контент — FileTree и `previewMode == true` → `fileTreePreviewCurrent()`
- не вызывать при `SidebarActionClose/BackToMenu/SwitchMode`

### Step 9: Editor Integration
**Files**: `internal/editor/types.go`, `internal/editor/actions.go`, `internal/editor/input.go`
- Добавить action `open_file_tree` и команду `:tree`
- `switchSidebarMode` для `SidebarModeFileTree` вызывает `openSidebarFileTree()`
- `openSidebarFileTree(dir string)`:
  - закрывает refs picker (mutual exclusion)
  - создает `SidebarFileTreeContent` (dir = cwd или dir текущего файла)
  - `sidebar.MenuContent` обновляется для возврата в меню
  - `sidebar.Open(content)`
- `handleSidebarKey`: при `SidebarActionOpenFile` сохраняет `sidebarOpenFilePath` и закрывает/снимает фокус в зависимости от `sidebar.CloseOnSelect`

### Step 10: App Integration
**File**: `internal/app/app.go`
- Добавить `ed.ConsumeSidebarOpenFile()`:
  - открывать файл как при старте (OpenFile + LSP + highlight + watcher)
  - обновлять `gitPath`
- Определить `projectRoot` для FileTree:
  - приоритет: git root (если есть) → cwd → dir текущего файла

### Step 11: Commands
**File**: `internal/editor/actions.go`
- `:tree` — открыть FileTree от cwd/dir файла
- `:tree <path>` — открыть FileTree от указанного пути

---

## 5. File Listing Rules

1. `".."` всегда первым (если не `/`)
2. Директории сортируются по алфавиту (case-insensitive)
3. Файлы сортируются по алфавиту (case-insensitive)
4. Директории выше файлов
5. Dotfiles скрыты, если `showHidden = false`
6. Gitignored скрыты, если `showIgnored = false`
7. `.git` всегда скрыт

При показе hidden/ignored элементы рендерятся с `SidebarStyles.Hidden/Ignored`.

---

## 6. Path Truncation (Optional)

Если нужен формат `"/first/.../last"`, используем:
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
| `internal/config/config.go` | FileTreeShowHidden/Ignored, keymap default |
| `internal/editor/options.go` | новые поля опций |
| `internal/editor/gitignore.go` | **NEW** - gitignore parsing |
| `internal/editor/sidebar_filetree.go` | **NEW** - FileTree mode content |
| `internal/editor/input.go` | open/switch FileTree, handle SidebarActionOpenFile |
| `internal/editor/actions.go` | `open_file_tree`, `:tree` |
| `internal/editor/types.go` | action + command list |
| `internal/app/app.go` | ConsumeSidebarOpenFile |
| `config/config.toml` | дефолтные бинды |

---

## 8. Verification

1. `make build` - компиляция
2. `make test` - тесты для `gitignore.go`, `sidebar_filetree.go`
3. `make lint` - линтер
4. Manual testing:
   - `Cmd+O` opens FileTree mode
   - Navigate with j/k, enter directories
   - `v` enables preview, auto-updates on move
   - `a` shows/hides dotfiles
   - `h` shows/hides gitignored (in git repo)
   - `backspace/left` goes to parent
   - `esc` returns to menu, `q` closes
