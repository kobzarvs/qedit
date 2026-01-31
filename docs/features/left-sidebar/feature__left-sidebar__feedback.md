# Feedback on Left Sidebar Plan

## Architectural Improvements

### 1. Interface for Sidebar Modes (Strategy Pattern)

Instead of a large `switch` statement in `Render`, `HandleKey`, etc., define an interface for sidebar content. This isolates the logic for each mode (Files, Git, History) into separate files/structs.

**Proposed Interface (`internal/editor/sidebar.go`):**

```go
// SidebarContent interface for any sidebar mode
type SidebarContent interface {
    // Title returns the mode title (e.g., "Files", "Branches")
    Title() string
    
    // Render draws the content into the specified area
    Render(screen tcell.Screen, styles SidebarStyles, x, y, width, height int)
    
    // HandleKey processes key events. Returns true if handled.
    // action returns a SidebarAction for the editor to perform (e.g., open file)
    HandleKey(ev *tcell.EventKey) (bool, SidebarAction)
}

type Sidebar struct {
    // ... base fields (Visible, Width, etc) ...
    
    // Current active content
    Content SidebarContent
    
    // Navigation stack (optional, for deeper navigation)
    HistoryStack []SidebarContent
}
```

**Benefits:**
*   **Code Cleanliness**: `sidebar.go` manages the container (resizing, borders, toggling), while `sidebar_files.go` handles file tree logic, etc.
*   **Extensibility**: Adding a new mode only requires implementing the interface and registering it in the menu.

### 2. Configuration Grouping

Group the 13 `Sidebar*` color fields in `Theme` into a nested structure to keep the config clean.

**Current:**
```go
type Theme struct {
    SidebarForeground string
    SidebarBackground string
    // ...
}
```

**Proposed:**
```go
type Theme struct {
    Sidebar SidebarTheme `toml:"sidebar"`
}

type SidebarTheme struct {
    Foreground       string `toml:"foreground"`
    Background       string `toml:"background"`
    DirForeground    string `toml:"dir-foreground"`
    // ...
}
```

**TOML:**
```toml
[theme.sidebar]
foreground = "#B3B1AD"
background = "#0A0E14"
```

## UX & Performance Considerations

1.  **Async Loading for History**: 
    *   Modes like `Recent History` (line history) or `Local Changes` can be slow for large files/repos. 
    *   Implement async loading or a spinner state to prevent UI freezing when switching to these modes.

2.  **Key Bindings (`Cmd` key)**:
    *   `Cmd` shortcuts often conflict with terminal emulators or the OS (MacOS). 
    *   Ensure fallback bindings or user-configurable keys are available (e.g., `Alt` or `Leader` keys).

3.  **Focus Management**:
    *   When selecting a file (`Enter`):
        *   Does focus move to the editor? (Standard behavior)
        *   Does the sidebar close? (Configurable behavior)
    *   **Suggestion**: Add a config option `sidebar-close-on-select = boolean` (default `false`).
