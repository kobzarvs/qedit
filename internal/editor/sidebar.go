package editor

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kobzarvs/qedit/internal/logger"
)

// SidebarMode represents different sidebar modes
type SidebarMode int

const (
	SidebarModeNone SidebarMode = iota
	SidebarModeMenu          // main menu for mode selection
	SidebarModeFileTree      // file tree (future)
	SidebarModeBranches      // git branch selection (v1)
	SidebarModeRecentHistory // line-by-line history (future)
	SidebarModeLocalChanges  // local changes history (future)
	SidebarModeWorktrees     // git worktrees (future)
)

// SidebarAction represents actions returned from sidebar content
type SidebarAction int

const (
	SidebarActionNone SidebarAction = iota
	SidebarActionClose          // close sidebar
	SidebarActionBackToMenu     // return to menu
	SidebarActionOpenFile       // open file (path in Data)
	SidebarActionCheckoutBranch // checkout git branch
	SidebarActionRefresh        // refresh current mode
	SidebarActionFocusEditor    // return focus to editor
	SidebarActionSwitchMode     // switch to different mode
)

// SidebarActionData contains action and associated data
type SidebarActionData struct {
	Action SidebarAction
	Path   string      // for OpenFile
	Branch string      // for CheckoutBranch
	Mode   SidebarMode // for SwitchMode
}

// SidebarItem represents an item in the sidebar list
type SidebarItem struct {
	Label     string
	Path      string // optional, for file paths
	IsDir     bool
	IsHidden  bool
	IsIgnored bool
	IsCurrent bool // e.g., current branch
	Icon      rune // optional icon character
	Hotkey    string
	Available bool
	Mode      SidebarMode // for menu items
}

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
	HandleKey(ev *tcell.EventKey) (bool, SidebarActionData)

	// OnEnter called when Enter pressed on current item
	OnEnter() SidebarActionData

	// Available returns false if mode unavailable (e.g., no git)
	Available() bool

	// Refresh reloads content (e.g., after directory change)
	Refresh() error
}

// SidebarStyles contains all sidebar styling
type SidebarStyles struct {
	Base        tcell.Style // default fg/bg
	Dir         tcell.Style // directories
	Selected    tcell.Style // selected item
	Header      tcell.Style // title bar
	Border      tcell.Style // vertical separator
	Hidden      tcell.Style // dimmed items
	Ignored     tcell.Style // gitignored
	Indicator   tcell.Style // ">" cursor
	Hotkey      tcell.Style // hotkey hints in menu
	Unavailable tcell.Style // greyed out items
	Current     tcell.Style // current branch marker
}

// Sidebar is the main sidebar container
type Sidebar struct {
	// Visibility & focus
	Visible bool
	Focused bool

	// Width config (from config)
	WidthConfig    string // "30", "1/4", "25%"
	MinWidth       int
	MaxWidthConfig string

	// Current content (Strategy pattern)
	Content     SidebarContent
	MenuContent *SidebarMenuContent // always available for returning

	// Scroll state (managed by container)
	Scroll int

	// Close on select behavior
	CloseOnSelect bool
}

// NewSidebar creates a new sidebar with config
func NewSidebar(widthConfig string, minWidth int, maxWidthConfig string, closeOnSelect bool) *Sidebar {
	return &Sidebar{
		Visible:        false,
		Focused:        false,
		WidthConfig:    widthConfig,
		MinWidth:       minWidth,
		MaxWidthConfig: maxWidthConfig,
		CloseOnSelect:  closeOnSelect,
		Scroll:         0,
	}
}

// CalculateWidth returns the sidebar width based on config and screen width
func (s *Sidebar) CalculateWidth(screenWidth int) int {
	width := parseWidthValue(s.WidthConfig, screenWidth)
	maxWidth := parseWidthValue(s.MaxWidthConfig, screenWidth)

	if width < s.MinWidth {
		width = s.MinWidth
	}
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
	}
	// Don't exceed half the screen
	if width > screenWidth/2 {
		width = screenWidth / 2
	}

	return width
}

// parseWidthValue parses width value: "30", "1/4", "25%"
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

// Navigation methods

// MoveUp moves selection up
func (s *Sidebar) MoveUp() {
	if s.Content == nil {
		return
	}
	idx := s.Content.Index()
	if idx > 0 {
		s.Content.SetIndex(idx - 1)
	}
}

// MoveDown moves selection down
func (s *Sidebar) MoveDown() {
	if s.Content == nil {
		return
	}
	items := s.Content.Items()
	idx := s.Content.Index()
	if idx < len(items)-1 {
		s.Content.SetIndex(idx + 1)
	}
}

// MoveToFirst moves to the first item
func (s *Sidebar) MoveToFirst() {
	if s.Content == nil {
		return
	}
	s.Content.SetIndex(0)
}

// MoveToLast moves to the last item
func (s *Sidebar) MoveToLast() {
	if s.Content == nil {
		return
	}
	items := s.Content.Items()
	if len(items) > 0 {
		s.Content.SetIndex(len(items) - 1)
	}
}

// PageUp moves selection up by a page
func (s *Sidebar) PageUp(height int) {
	if s.Content == nil {
		return
	}
	idx := s.Content.Index()
	idx -= height
	if idx < 0 {
		idx = 0
	}
	s.Content.SetIndex(idx)
}

// PageDown moves selection down by a page
func (s *Sidebar) PageDown(height int) {
	if s.Content == nil {
		return
	}
	items := s.Content.Items()
	idx := s.Content.Index()
	idx += height
	if idx >= len(items) {
		idx = len(items) - 1
		if idx < 0 {
			idx = 0
		}
	}
	s.Content.SetIndex(idx)
}

// EnsureVisible adjusts scroll to make current item visible
func (s *Sidebar) EnsureVisible(height int) {
	if s.Content == nil || height <= 0 {
		return
	}
	idx := s.Content.Index()

	// Ensure scroll doesn't go past the current item
	if idx < s.Scroll {
		s.Scroll = idx
	}
	// Ensure current item is visible
	if idx >= s.Scroll+height {
		s.Scroll = idx - height + 1
	}
}

// HandleKey processes common sidebar keys and delegates to content
func (s *Sidebar) HandleKey(ev *tcell.EventKey, viewHeight int) SidebarActionData {
	if s.Content == nil {
		logger.Debug("Sidebar.HandleKey: content is nil")
		return SidebarActionData{Action: SidebarActionNone}
	}

	key := ev.Key()
	r := ev.Rune()
	logger.Debug("Sidebar.HandleKey", "key", key, "rune", string(r), "mode", s.Content.Mode())

	// First let content handle mode-specific keys
	handled, action := s.Content.HandleKey(ev)
	if handled {
		logger.Debug("Sidebar.HandleKey: handled by content", "action", action.Action)
		return action
	}

	// Common navigation
	switch {
	case key == tcell.KeyUp || r == 'k':
		s.MoveUp()
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == tcell.KeyDown || r == 'j':
		s.MoveDown()
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == tcell.KeyPgUp:
		s.PageUp(viewHeight)
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == tcell.KeyPgDn:
		s.PageDown(viewHeight)
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == tcell.KeyHome || (r == 'g' && ev.Modifiers() == 0):
		// Note: 'gg' motion would need state tracking
		s.MoveToFirst()
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == tcell.KeyEnd || r == 'G':
		s.MoveToLast()
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == tcell.KeyEnter:
		return s.Content.OnEnter()

	case key == tcell.KeyRight || r == 'l':
		// Right/l only works in menu mode (to enter submenu), not on leaf items
		if s.Content.Mode() == SidebarModeMenu {
			return s.Content.OnEnter()
		}
		return SidebarActionData{Action: SidebarActionNone}

	case key == tcell.KeyLeft || r == 'h':
		// Left/h: back to menu (does nothing if already in menu)
		if s.Content.Mode() != SidebarModeMenu {
			return SidebarActionData{Action: SidebarActionBackToMenu}
		}
		return SidebarActionData{Action: SidebarActionNone}

	case key == tcell.KeyEscape || r == 'q':
		// Esc/q always closes sidebar
		return SidebarActionData{Action: SidebarActionClose}

	case r == '`':
		return SidebarActionData{Action: SidebarActionFocusEditor}
	}

	return SidebarActionData{Action: SidebarActionNone}
}

// Render renders the sidebar
func (s *Sidebar) Render(screen tcell.Screen, styles SidebarStyles, x, y, w, h int) {
	if s.Content == nil || w <= 0 || h <= 0 {
		return
	}

	// Fill background
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			screen.SetContent(col, row, ' ', nil, styles.Base)
		}
	}

	// Draw border on the right side
	for row := y; row < y+h; row++ {
		screen.SetContent(x+w-1, row, '│', nil, styles.Border)
	}

	contentWidth := w - 1 // exclude border

	// Draw header
	title := s.Content.Title()
	if len(title) > contentWidth {
		title = title[:contentWidth]
	}
	for i, r := range title {
		if i < contentWidth {
			screen.SetContent(x+i, y, r, nil, styles.Header)
		}
	}
	// Fill rest of header line
	for i := len(title); i < contentWidth; i++ {
		screen.SetContent(x+i, y, ' ', nil, styles.Header)
	}

	// Draw items
	items := s.Content.Items()
	currentIdx := s.Content.Index()
	listHeight := h - 1 // minus header

	s.EnsureVisible(listHeight)

	for i := 0; i < listHeight; i++ {
		itemIdx := s.Scroll + i
		row := y + 1 + i

		if itemIdx >= len(items) {
			// Empty line
			for col := x; col < x+contentWidth; col++ {
				screen.SetContent(col, row, ' ', nil, styles.Base)
			}
			continue
		}

		item := items[itemIdx]
		isSelected := itemIdx == currentIdx

		// Get the selected background color
		_, selBg, _ := styles.Selected.Decompose()

		// Determine text style (foreground color based on item type)
		textStyle := styles.Base
		if !item.Available {
			textStyle = styles.Unavailable
		} else if item.IsHidden {
			textStyle = styles.Hidden
		} else if item.IsIgnored {
			textStyle = styles.Ignored
		} else if item.IsDir {
			textStyle = styles.Dir
		} else if item.IsCurrent {
			// Current item (e.g., current branch) keeps its special color
			textStyle = styles.Current
		}

		// If selected, apply selected background but keep text foreground
		if isSelected {
			fg, _, _ := textStyle.Decompose()
			textStyle = textStyle.Foreground(fg).Background(selBg)
		}

		// Fill entire row with background first (edge to edge selection)
		bgStyle := styles.Base
		if isSelected {
			bgStyle = bgStyle.Background(selBg)
		}
		for c := x; c < x+contentWidth; c++ {
			screen.SetContent(c, row, ' ', nil, bgStyle)
		}

		// Draw indicator (only '*' for current item, e.g. current branch)
		col := x
		if item.IsCurrent {
			screen.SetContent(col, row, '*', nil, textStyle)
		}
		col++

		// Draw label
		label := item.Label
		maxLabelWidth := contentWidth - 2 // indicator + space for hotkey
		if item.Hotkey != "" {
			maxLabelWidth = contentWidth - 2 - len(item.Hotkey) - 1
		}
		if len(label) > maxLabelWidth && maxLabelWidth > 0 {
			label = label[:maxLabelWidth]
		}

		for _, r := range label {
			if col < x+contentWidth-1 {
				screen.SetContent(col, row, r, nil, textStyle)
				col++
			}
		}

		// Draw hotkey (right-aligned)
		if item.Hotkey != "" {
			hotkeyX := x + contentWidth - len(item.Hotkey) - 1
			if hotkeyX > col {
				// Hotkey style: keep hotkey color but use selected background if selected
				hotkeyStyle := styles.Hotkey
				if !item.Available {
					hotkeyStyle = styles.Unavailable
				}
				if isSelected {
					fg, _, _ := hotkeyStyle.Decompose()
					hotkeyStyle = hotkeyStyle.Foreground(fg).Background(selBg)
				}
				for i, r := range item.Hotkey {
					screen.SetContent(hotkeyX+i, row, r, nil, hotkeyStyle)
				}
			}
		}
	}
}

// SetContent sets the sidebar content and resets scroll
func (s *Sidebar) SetContent(content SidebarContent) {
	s.Content = content
	s.Scroll = 0
}

// Open opens the sidebar with the given content
func (s *Sidebar) Open(content SidebarContent) {
	s.SetContent(content)
	s.Visible = true
	s.Focused = true
}

// Close closes the sidebar
func (s *Sidebar) Close() {
	s.Visible = false
	s.Focused = false
}

// Toggle toggles sidebar visibility
func (s *Sidebar) Toggle() {
	if s.Visible {
		s.Close()
	} else if s.MenuContent != nil {
		s.Open(s.MenuContent)
	}
}
