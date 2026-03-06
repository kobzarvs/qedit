package editor

// SidebarMenuContent implements SidebarContent for the main menu
type SidebarMenuContent struct {
	items []SidebarMenuItem
	index int
}

// SidebarMenuItem represents a menu item
type SidebarMenuItem struct {
	Label     string
	Mode      SidebarMode
	Hotkey    string
	Available bool
}

// NewSidebarMenuContent creates a new menu content
func NewSidebarMenuContent(items []SidebarMenuItem) *SidebarMenuContent {
	m := &SidebarMenuContent{index: 0}
	m.SetItems(items)
	return m
}

// SetItems replaces the menu items while preserving a valid selection.
func (m *SidebarMenuContent) SetItems(items []SidebarMenuItem) {
	m.items = append(m.items[:0], items...)
	if len(m.items) == 0 {
		m.index = 0
		return
	}
	if m.index >= len(m.items) {
		m.index = len(m.items) - 1
	}
	if m.index < 0 {
		m.index = 0
	}
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
func (m *SidebarMenuContent) HandleKey(ev EventKey) (bool, SidebarActionData) {
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
	return nil
}
