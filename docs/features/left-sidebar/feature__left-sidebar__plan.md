# Feature: Left Sidebar (Reusable Component)

## Overview
Универсальный левый сайдбар - переиспользуемый компонент для различных режимов:
- Files tree
- Select git branch
- Recent history (построчная история изменений текущего файла)
- Local changes history
- Git worktree list

При открытии сайдбара показывается меню выбора режима.

---

## V1 Scope (Minimal)

**Первая итерация включает только:**
1. Sidebar container (width, visibility, focus, rendering)
2. Menu mode (выбор режима)
3. Branches mode (миграция существующего BranchPicker)

Остальные режимы (FileTree, History, etc.) реализуются как отдельные фичи после v1.

---

## 1. Концепция

### Sidebar Menu (главное меню)
При первом открытии или по хоткею показываем меню режимов:
```
┌─────────────────────┐
│ Sidebar             │
│ > Files             │
│   Branches          │
│   Recent History    │
│   Local Changes     │
│   Worktrees         │
└─────────────────────┘
```

### Режимы
| Mode                       | Description         | Hotkey  |
| -------------------------- | ------------------- | ------- |
| `SidebarModeMenu`          | Меню выбора режима  | `` ` `` |
| `SidebarModeFileTree`      | Дерево файлов       | `Cmd+O` |
| `SidebarModeBranches`      | Выбор git ветки     | `Cmd+B` |
| `SidebarModeRecentHistory` | История строк файла | -       |
| `SidebarModeLocalChanges`  | Локальная история   | -       |
| `SidebarModeWorktrees`     | Git worktrees       | -       |

---

## 2. Data Structures

### Strategy Pattern: SidebarContent Interface
```go
// SidebarContent interface - each mode implements this
type SidebarContent interface {
    // Mode returns the mode identifier
    Mode() SidebarMode

    // Title returns header text for the sidebar
    Title() string

    // Items returns the list to display
    Items() []SidebarItem

    // Index/SetIndex for selection
    Index() int
    SetIndex(i int)

    // HandleKey processes mode-specific keys
    // Returns: handled, action
    HandleKey(ev *tcell.EventKey) (bool, SidebarAction)

    // OnEnter called when Enter pressed on current item
    OnEnter() SidebarAction

    // Available returns false if mode unavailable (e.g., no git)
    Available() bool

    // Refresh reloads content (e.g., after directory change)
    Refresh() error
}
```

### SidebarAction Enum (explicit actions)
```go
type SidebarAction int

const (
    SidebarActionNone SidebarAction = iota
    SidebarActionClose           // close sidebar
    SidebarActionBackToMenu      // return to menu
    SidebarActionOpenFile        // open file (path in Data)
    SidebarActionCheckoutBranch  // checkout git branch
    SidebarActionRefresh         // refresh current mode
    SidebarActionFocusEditor     // return focus to editor
)

type SidebarActionData struct {
    Action SidebarAction
    Path   string // for OpenFile
    Branch string // for CheckoutBranch
}
```

### sidebar.go (new file)
```go
type SidebarMode int

const (
    SidebarModeNone SidebarMode = iota
    SidebarModeMenu       // главное меню выбора режима
    SidebarModeFileTree   // (future)
    SidebarModeBranches   // v1
    SidebarModeRecentHistory  // (future)
    SidebarModeLocalChanges   // (future)
    SidebarModeWorktrees      // (future)
)

type Sidebar struct {
    // Visibility & focus
    Visible     bool
    Focused     bool

    // Width config (from config)
    WidthConfig    string  // "30", "1/4", "25%"
    MinWidth       int
    MaxWidthConfig string

    // Current content (Strategy pattern)
    Content     SidebarContent
    MenuContent *SidebarMenuContent // always available for returning

    // Scroll state (managed by container)
    Scroll      int
}

type SidebarItem struct {
    Label     string
    Path      string      // optional, for file paths
    IsDir     bool
    IsHidden  bool
    IsIgnored bool
    IsCurrent bool        // e.g., current branch
    Icon      rune        // optional icon character
}
```

### Menu Content (sidebar_menu.go)
```go
type SidebarMenuContent struct {
    items     []SidebarMenuItem
    index     int
    gitAvail  bool
}

type SidebarMenuItem struct {
    Label     string
    Mode      SidebarMode
    Hotkey    string  // display hint (e.g., "Cmd+O")
    Available bool
}

func (m *SidebarMenuContent) Mode() SidebarMode { return SidebarModeMenu }
func (m *SidebarMenuContent) Title() string { return "Sidebar" }
func (m *SidebarMenuContent) Items() []SidebarItem { /* convert menu items */ }
func (m *SidebarMenuContent) Available() bool { return true }
```

### Branches Content (sidebar_branches.go) - V1
```go
type SidebarBranchesContent struct {
    branches []string
    current  string
    index    int
}

func (b *SidebarBranchesContent) Mode() SidebarMode { return SidebarModeBranches }
func (b *SidebarBranchesContent) Title() string { return "Branches" }
func (b *SidebarBranchesContent) OnEnter() SidebarAction {
    return SidebarActionCheckoutBranch // with branch name
}
func (b *SidebarBranchesContent) Available() bool {
    return gitinfo.IsGitRepo() // check if git available
}
```

### Editor integration
```go
// Editor struct
sidebar       Sidebar
sidebarStyles SidebarStyles

// SidebarStyles
type SidebarStyles struct {
    Base       tcell.Style  // default fg/bg
    Dir        tcell.Style  // directories
    Selected   tcell.Style  // selected item
    Header     tcell.Style  // title bar
    Border     tcell.Style  // vertical separator
    Hidden     tcell.Style  // dimmed items
    Ignored    tcell.Style  // gitignored
    Indicator  tcell.Style  // ">" cursor
    Hotkey     tcell.Style  // hotkey hints in menu
    Unavailable tcell.Style // greyed out items
}
```

---

## 3. Configuration

### EditorOptions (config.go)
```go
type EditorOptions struct {
    // Sidebar
    SidebarWidth         string `toml:"sidebar-width"`           // default "30"
    SidebarMinWidth      int    `toml:"sidebar-min-width"`       // default 15
    SidebarMaxWidth      string `toml:"sidebar-max-width"`       // default "50"
    SidebarCloseOnSelect bool   `toml:"sidebar-close-on-select"` // default false
}
```

### Theme colors (config.go)
```go
// Sidebar colors (flat fields - no breaking change)
// NOTE: Future cleanup may move to nested [theme.sidebar] section
//       with backward compatibility support
SidebarForeground          string `toml:"sidebar-foreground"`
SidebarBackground          string `toml:"sidebar-background"`
SidebarDirForeground       string `toml:"sidebar-dir-foreground"`
SidebarSelectedForeground  string `toml:"sidebar-selected-foreground"`
SidebarSelectedBackground  string `toml:"sidebar-selected-background"`
SidebarHeaderForeground    string `toml:"sidebar-header-foreground"`
SidebarHeaderBackground    string `toml:"sidebar-header-background"`
SidebarBorderForeground    string `toml:"sidebar-border-foreground"`
SidebarHiddenForeground    string `toml:"sidebar-hidden-foreground"`
SidebarIgnoredForeground   string `toml:"sidebar-ignored-foreground"`
SidebarIndicatorForeground string `toml:"sidebar-indicator-foreground"`
SidebarHotkeyForeground    string `toml:"sidebar-hotkey-foreground"`
SidebarUnavailableForeground string `toml:"sidebar-unavailable-foreground"`
```

### Default values
```go
SidebarForeground:           "#B3B1AD",
SidebarBackground:           "#0A0E14",
SidebarDirForeground:        "#59C2FF",
SidebarSelectedForeground:   "#0A0E14",
SidebarSelectedBackground:   "#E6B450",
SidebarHeaderForeground:     "#B3B1AD",
SidebarHeaderBackground:     "#0F1419",
SidebarBorderForeground:     "#3E4B59",
SidebarHiddenForeground:     "#3E4B59",
SidebarIgnoredForeground:    "#3E4B59",
SidebarIndicatorForeground:  "#E6B450",
SidebarHotkeyForeground:     "#59C2FF",
SidebarUnavailableForeground: "#3E4B59",

SidebarWidth:         "30",
SidebarMinWidth:      15,
SidebarMaxWidth:      "50",
SidebarCloseOnSelect: false,
```

### Example config.toml
```toml
[editor]
sidebar-width = "30"            # "30", "1/4", "25%"
sidebar-min-width = 15
sidebar-max-width = "50"
sidebar-close-on-select = false # close sidebar when selecting item

[theme]
sidebar-foreground = "#B3B1AD"
sidebar-background = "#0A0E14"
sidebar-dir-foreground = "#59C2FF"
sidebar-selected-foreground = "#0A0E14"
sidebar-selected-background = "#E6B450"
sidebar-header-foreground = "#B3B1AD"
sidebar-header-background = "#0F1419"
sidebar-border-foreground = "#3E4B59"
sidebar-hidden-foreground = "#3E4B59"
sidebar-ignored-foreground = "#3E4B59"
sidebar-indicator-foreground = "#E6B450"
sidebar-hotkey-foreground = "#59C2FF"
sidebar-unavailable-foreground = "#3E4B59"
```

---

## 4. Key Bindings

### Keybinding Conflicts Note
`Cmd` shortcuts often conflict with terminal emulators or macOS.
All bindings must be user-configurable via `[keymap.normal]`.
Provide sensible defaults with fallbacks.

### Global (normal mode)
| Key     | Fallback  | Action                                         |
| ------- | --------- | ---------------------------------------------- |
| `` ` `` | -         | Toggle sidebar menu (or focus if already open) |
| `Cmd+O` | `Space e` | Open sidebar → FileTree mode (future)          |
| `Cmd+B` | `Space b` | Open sidebar → Branches mode                   |

**Migration Note**: `Cmd+B` currently opens BranchPicker modal.
In v1, we replace it with sidebar Branches mode. Old modal code removed.

### Sidebar Menu (when in menu mode)
| Key      | Action        |
| -------- | ------------- |
| `up/k`   | Move up       |
| `down/j` | Move down     |
| `enter`  | Select mode   |
| `esc/q`  | Close sidebar |
| `` ` ``  | Close sidebar |

### Focus Behavior on Select
When user selects item (Enter):
- Focus moves to editor
- If `sidebar-close-on-select = true`: sidebar closes
- If `sidebar-close-on-select = false`: sidebar stays open (default)

### Common Sidebar (all modes)
| Key             | Action                             |
| --------------- | ---------------------------------- |
| `up/k`          | Move up                            |
| `down/j`        | Move down                          |
| `home/gg`       | First item                         |
| `end/G`         | Last item                          |
| `pgup/pgdn`     | Page navigation                    |
| `esc`           | Back to menu (or close if in menu) |
| `q`             | Close sidebar                      |
| `` ` `` / `tab` | Return focus to editor             |

---

## 5. Interaction with Existing Components

### RefsPicker Sidebar
Currently `renderRefsSidebar` renders LSP references in left sidebar.

**Strategy for v1**: Mutual exclusion
- When new Sidebar opens → close RefsPicker
- When RefsPicker opens → close new Sidebar
- Future: migrate RefsPicker to `SidebarModeRefs`

```go
// In editor.go when opening sidebar:
if e.refsPickerActive {
    e.closeRefsPicker()
}
e.sidebar.Visible = true
```

### BranchPicker Modal
Currently `branchPickerActive` renders centered modal.

**Migration in v1**:
- Remove `branchPickerActive`, `branchPickerItems`, etc.
- Replace with `SidebarBranchesContent`
- `Cmd+B` opens sidebar in Branches mode
- Same functionality, different UI location

---

## 6. Implementation Steps

### Step 1: Config
**File**: `internal/config/config.go`
- Add `SidebarWidth`, `SidebarMinWidth`, `SidebarMaxWidth`, `SidebarCloseOnSelect`
- Add 13 `Sidebar*` color fields to Theme (flat, no nesting)
- Add defaults and merge logic

### Step 2: Sidebar Types & Interface
**File**: `internal/editor/sidebar.go` (new)
- Define `SidebarMode` enum
- Define `SidebarAction` enum
- Define `SidebarContent` interface
- Define `Sidebar` struct (container)
- Define `SidebarItem`, `SidebarStyles` structs

### Step 3: Width Calculation
**File**: `internal/editor/sidebar.go`
- `parseWidthValue(value, screenWidth) int`
- `(s *Sidebar) CalculateWidth(screenWidth) int`

### Step 4: Container Navigation
**File**: `internal/editor/sidebar.go`
Container manages scroll, delegates index to Content:
- `(s *Sidebar) MoveUp()` → `s.Content.SetIndex(...)`
- `(s *Sidebar) MoveDown()`
- `(s *Sidebar) PageUp(height int)`
- `(s *Sidebar) PageDown(height int)`
- `(s *Sidebar) EnsureVisible(height int)`

### Step 5: Menu Content
**File**: `internal/editor/sidebar_menu.go` (new)
- Implement `SidebarContent` interface
- `NewSidebarMenuContent(gitAvailable bool)`
- Menu items with availability check
- `OnEnter()` returns action to switch mode

### Step 6: Branches Content (V1)
**File**: `internal/editor/sidebar_branches.go` (new)
- Implement `SidebarContent` interface
- `NewSidebarBranchesContent(branches, current)`
- `OnEnter()` returns `SidebarActionCheckoutBranch`
- `Available()` checks git repo

### Step 7: Rendering
**File**: `internal/editor/sidebar.go`
- `(s *Sidebar) Render(screen, styles, x, y, w, h)`
- Get title and items from `s.Content`
- Draw: border, header, items, selection indicator
- Style unavailable items with `Unavailable` style

### Step 8: Key Handling
**File**: `internal/editor/sidebar.go`
- `(s *Sidebar) HandleKey(ev) SidebarActionData`
- Common keys: up/down/pgup/pgdn/home/end/esc/q
- Delegate to `s.Content.HandleKey()` for mode-specific
- Return action for editor to execute

### Step 9: Editor Integration
**File**: `internal/editor/editor.go`
- Add `sidebar Sidebar` and `sidebarStyles SidebarStyles`
- Remove `branchPicker*` fields (migration)
- In `New()`: init sidebar, parse styles
- In `Render()`: if sidebar visible, render and offset editor
- In `HandleKey()`: route to sidebar when focused
- Execute `SidebarAction` results (checkout branch, open file, etc.)

### Step 10: App Integration
**File**: `internal/app/app.go`
- Remove `ConsumeBranchPickerRequest()` usage
- Handle branch checkout via sidebar action
- Detect git availability for menu

### Step 11: Commands
**File**: `internal/editor/editor.go`
- `:sidebar` - toggle sidebar menu
- `:sidew` / `:sidew 30` / `:sidew 25%` - show/set width

---

## 7. Width Calculation

```go
func parseWidthValue(value string, screenWidth int) int {
    value = strings.TrimSpace(value)

    // Percentage: "25%"
    if strings.HasSuffix(value, "%") {
        pct, _ := strconv.Atoi(strings.TrimSuffix(value, "%"))
        return screenWidth * pct / 100
    }

    // Fraction: "1/4"
    if strings.Contains(value, "/") {
        parts := strings.Split(value, "/")
        if len(parts) == 2 {
            num, _ := strconv.Atoi(parts[0])
            den, _ := strconv.Atoi(parts[1])
            if den > 0 {
                return screenWidth * num / den
            }
        }
    }

    // Absolute: "30"
    n, _ := strconv.Atoi(value)
    return n
}
```

---

## 8. Rendering Layout

### Menu Mode
```
┌─────────────────────┬───────────────────────────────┐
│ Sidebar             │                               │
│ > Files        Cmd+O│  Editor content               │
│   Branches     Cmd+B│                               │
│   Recent History    │                               │
│   Local Changes     │                               │
│   Worktrees         │                               │
│                     │                               │
├─────────────────────┴───────────────────────────────┤
│ [status line]                                       │
└─────────────────────────────────────────────────────┘
```

### List Mode (e.g., FileTree)
```
┌─────────────────────┬───────────────────────────────┐
│ /Users/.../project  │                               │
│ > ..                │  Editor content               │
│   /src              │                               │
│   /tests            │                               │
│   main.go           │                               │
│   README.md         │                               │
│                     │                               │
├─────────────────────┴───────────────────────────────┤
│ [status line]                                       │
└─────────────────────────────────────────────────────┘
```

---

## 9. Critical Files

| File                                  | Changes                                       |
| ------------------------------------- | --------------------------------------------- |
| `internal/config/config.go`           | Sidebar config + theme colors                 |
| `internal/editor/sidebar.go`          | **NEW** - Sidebar container, interface, types |
| `internal/editor/sidebar_menu.go`     | **NEW** - Menu mode content                   |
| `internal/editor/sidebar_branches.go` | **NEW** - Branches mode content               |
| `internal/editor/editor.go`           | Integration, remove old branchPicker          |
| `internal/app/app.go`                 | Remove old branch picker handling             |
| `config.example.toml`                 | Sidebar config examples                       |

---

## 10. Verification

1. `make build` - компиляция
2. `make test` - тесты для sidebar*.go
3. `make lint` - линтер
4. Manual testing:
   - `` ` `` opens sidebar menu
   - Navigate menu with j/k
   - Git-dependent modes greyed out when not in git repo
   - Enter on "Branches" opens branches list
   - Select branch → checkout works
   - `Cmd+B` directly opens Branches mode
   - Esc returns to menu, q closes sidebar
   - `:sidew 40` changes width
   - `:sidew 1/4` works with fractions
   - Width respects min/max bounds
   - `sidebar-close-on-select = true` closes on select
   - RefsPicker and Sidebar are mutually exclusive

---

## 11. Future Modes (out of V1 scope)

Каждый режим реализуется как отдельная фича после v1:
- `SidebarModeFileTree` → feature__files-tree__plan.md
- `SidebarModeRecentHistory` → line-by-line history
- `SidebarModeLocalChanges` → local changes history
- `SidebarModeWorktrees` → git worktree list

В меню эти пункты показываются, но при выборе:
```go
e.setStatus("FileTree: not implemented yet")
```

---

## 12. Decisions Made

| Question                           | Decision                                       |
| ---------------------------------- | ---------------------------------------------- |
| Replace BranchPicker or keep both? | Replace. Old modal removed in v1               |
| Persist sidebar width?             | In-memory only. Config sets default            |
| Sidebar vs RefsPicker?             | Mutual exclusion. Close one when opening other |
| Theme nesting?                     | Deferred. Keep flat fields for v1              |
| Interface vs simple struct?        | Interface (Strategy pattern) for extensibility |
