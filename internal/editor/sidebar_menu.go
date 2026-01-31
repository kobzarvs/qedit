package editor

import "github.com/gdamore/tcell/v2"

// SidebarMenuContent implements SidebarContent for the main menu
type SidebarMenuContent struct {
	items    []SidebarMenuItem
	index    int
	gitAvail bool
}

// SidebarMenuItem represents a menu item
type SidebarMenuItem struct {
	Label     string
	Mode      SidebarMode
	Hotkey    string
	Available bool
}

// NewSidebarMenuContent creates a new menu content
func NewSidebarMenuContent(gitAvailable bool) *SidebarMenuContent {
	m := &SidebarMenuContent{
		gitAvail: gitAvailable,
		index:    0,
	}
	m.buildItems()
	return m
}

// buildItems populates the menu items
func (m *SidebarMenuContent) buildItems() {
	m.items = []SidebarMenuItem{
		{Label: "Files", Mode: SidebarModeFileTree, Hotkey: "Cmd+O", Available: true},
		{Label: "Branches", Mode: SidebarModeBranches, Hotkey: "Cmd+B", Available: m.gitAvail},
		{Label: "Recent History", Mode: SidebarModeRecentHistory, Hotkey: "", Available: false},
		{Label: "Local Changes", Mode: SidebarModeLocalChanges, Hotkey: "", Available: false},
		{Label: "Worktrees", Mode: SidebarModeWorktrees, Hotkey: "", Available: m.gitAvail},
	}
}

// SetGitAvailable updates git availability and rebuilds items
func (m *SidebarMenuContent) SetGitAvailable(avail bool) {
	m.gitAvail = avail
	m.buildItems()
}

// Mode returns the mode identifier
func (m *SidebarMenuContent) Mode() SidebarMode {
	return SidebarModeMenu
}

// Title returns header text
func (m *SidebarMenuContent) Title() string {
	return "Sidebar"
}

// Items returns the list to display
func (m *SidebarMenuContent) Items() []SidebarItem {
	result := make([]SidebarItem, len(m.items))
	for i, item := range m.items {
		result[i] = SidebarItem{
			Label:     item.Label,
			Hotkey:    item.Hotkey,
			Available: item.Available,
			Mode:      item.Mode,
		}
	}
	return result
}

// Index returns current selection index
func (m *SidebarMenuContent) Index() int {
	return m.index
}

// SetIndex sets the selection index
func (m *SidebarMenuContent) SetIndex(i int) {
	if i >= 0 && i < len(m.items) {
		m.index = i
	}
}

// HandleKey processes mode-specific keys
func (m *SidebarMenuContent) HandleKey(ev *tcell.EventKey) (bool, SidebarActionData) {
	// No special keys for menu, navigation is handled by container
	return false, SidebarActionData{Action: SidebarActionNone}
}

// OnEnter called when Enter pressed
func (m *SidebarMenuContent) OnEnter() SidebarActionData {
	if m.index < 0 || m.index >= len(m.items) {
		return SidebarActionData{Action: SidebarActionNone}
	}

	item := m.items[m.index]
	if !item.Available {
		// Item not available - do nothing (or show status message)
		return SidebarActionData{Action: SidebarActionNone}
	}

	return SidebarActionData{
		Action: SidebarActionSwitchMode,
		Mode:   item.Mode,
	}
}

// Available returns true (menu is always available)
func (m *SidebarMenuContent) Available() bool {
	return true
}

// Refresh reloads content
func (m *SidebarMenuContent) Refresh() error {
	m.buildItems()
	return nil
}
