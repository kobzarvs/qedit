package editor

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/kobzarvs/qedit/internal/logger"
)

// SidebarMode represents different sidebar modes
type SidebarMode int

const (
	SidebarModeNone          SidebarMode = iota
	SidebarModeMenu                      // main menu for mode selection
	SidebarModeFileTree                  // file tree (future)
	SidebarModeBranches                  // git branch selection (v1)
	SidebarModeAI                        // AI providers/models
	SidebarModeRecentHistory             // line-by-line history (future)
	SidebarModeLocalChanges              // git changes
	SidebarModeWorktrees                 // git worktrees (future)
)

// SidebarAction represents actions returned from sidebar content
type SidebarAction int

const (
	SidebarActionNone           SidebarAction = iota
	SidebarActionClose                        // close sidebar
	SidebarActionBackToMenu                   // return to menu
	SidebarActionOpenFile                     // open file (path in Data)
	SidebarActionCheckoutBranch               // checkout git branch
	SidebarActionRefresh                      // refresh current mode
	SidebarActionFocusEditor                  // return focus to editor
	SidebarActionSwitchMode                   // switch to different mode
	SidebarActionOpenAIModels                 // open AI models list for provider
	SidebarActionSetAIModel                   // set AI model
)

// SidebarActionData contains action and associated data
type SidebarActionData struct {
	Action   SidebarAction
	Path     string      // for OpenFile
	Branch   string      // for CheckoutBranch
	Mode     SidebarMode // for SwitchMode
	Provider string      // for AI provider selection
	Model    string      // for AI model selection
}

// SidebarItem represents an item in the sidebar list
type SidebarItem struct {
	Label         string
	Path          string // optional, for file paths
	IsDir         bool
	IsHidden      bool
	IsIgnored     bool
	IsCurrent     bool // e.g., current branch
	Icon          rune // optional icon character
	IconStyle     Style
	Hotkey        string
	MatchIndices  []int
	RightSegments []SidebarTextSegment
	Available     bool
	Mode          SidebarMode // for menu items
	ShowStatus    bool
	Status        AIProviderStatus
}

// SidebarTextSegment represents a styled text segment (used for right-aligned info).
type SidebarTextSegment struct {
	Text  string
	Style Style
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
	HandleKey(ev EventKey) (bool, SidebarActionData)

	// OnEnter called when Enter pressed on current item
	OnEnter() SidebarActionData

	// Available returns false if mode unavailable (e.g., no git)
	Available() bool

	// Refresh reloads content (e.g., after directory change)
	Refresh() error
}

// SidebarStyles contains all sidebar styling
type SidebarStyles struct {
	Base     Style // default fg/bg
	Dir      Style // directories
	Selected Style // selected item
	// SelectedBackground is used for selection highlight without affecting text colors.
	SelectedBackground Color
	Header             Style // title bar
	Border             Style // vertical separator
	Hidden             Style // dimmed items
	Ignored            Style // gitignored
	Indicator          Style // ">" cursor
	Hotkey             Style // hotkey hints in menu
	Unavailable        Style // greyed out items
	Current            Style // current branch marker
	StatusOnline       Style // provider online
	StatusOffline      Style // provider offline
	DiffAdd            Style // diff additions (+)
	DiffDel            Style // diff deletions (-)
	SearchMatch        Style // match highlight
	SearchMatchFile    Style // match highlight for files
	SearchMatchDir     Style // match highlight for dirs
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

func runeWidth(r rune) int {
	if r == 0 {
		return 0
	}
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
		return 0
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

func isWideRune(r rune) bool {
	if unicode.In(r, unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana) {
		return true
	}
	switch {
	case r >= 0x1100 && r <= 0x115F: // Hangul Jamo
		return true
	case r == 0x2329 || r == 0x232A:
		return true
	case r >= 0x2E80 && r <= 0xA4CF: // CJK Radicals, Symbols, Yi
		return true
	case r >= 0xAC00 && r <= 0xD7A3: // Hangul Syllables
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK Compatibility Ideographs
		return true
	case r >= 0xFE10 && r <= 0xFE19: // Vertical forms
		return true
	case r >= 0xFE30 && r <= 0xFE6F: // CJK Compatibility Forms
		return true
	case r >= 0xFF00 && r <= 0xFF60: // Fullwidth Forms
		return true
	case r >= 0xFFE0 && r <= 0xFFE6: // Fullwidth symbol variants
		return true
	case r >= 0x1F300 && r <= 0x1FAFF: // Emoji ranges
		return true
	default:
		return false
	}
}

func stringWidth(s string) int {
	width := 0
	for _, r := range s {
		width += runeWidth(r)
	}
	return width
}

func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		w := runeWidth(r)
		if width+w > maxWidth {
			break
		}
		b.WriteRune(r)
		width += w
	}
	return b.String()
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
func (s *Sidebar) HandleKey(ev EventKey, viewHeight int) SidebarActionData {
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
	case key == KeyUp || r == 'k':
		s.MoveUp()
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == KeyDown || r == 'j':
		s.MoveDown()
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == KeyPgUp:
		s.PageUp(viewHeight)
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == KeyPgDn:
		s.PageDown(viewHeight)
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == KeyHome || (r == 'g' && ev.Modifiers() == 0):
		// Note: 'gg' motion would need state tracking
		s.MoveToFirst()
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == KeyEnd || r == 'G':
		s.MoveToLast()
		s.EnsureVisible(viewHeight)
		return SidebarActionData{Action: SidebarActionNone}

	case key == KeyEnter:
		return s.Content.OnEnter()

	case key == KeyRight || r == 'l':
		// Right/l only works in menu mode (to enter submenu), not on leaf items
		if s.Content.Mode() == SidebarModeMenu {
			return s.Content.OnEnter()
		}
		return SidebarActionData{Action: SidebarActionNone}

	case key == KeyLeft || r == 'h':
		// Left/h: back to menu (does nothing if already in menu)
		if s.Content.Mode() != SidebarModeMenu {
			return SidebarActionData{Action: SidebarActionBackToMenu}
		}
		return SidebarActionData{Action: SidebarActionNone}

	case key == KeyEscape || r == 'q':
		// Esc/q always closes sidebar
		return SidebarActionData{Action: SidebarActionClose}

	}

	return SidebarActionData{Action: SidebarActionNone}
}

// Render renders the sidebar
func (s *Sidebar) Render(screen Screen, styles SidebarStyles, x, y, w, h int) {
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
	title = truncateToWidth(title, contentWidth)
	col := x
	for _, r := range title {
		if col >= x+contentWidth {
			break
		}
		screen.SetContent(col, y, r, nil, styles.Header)
		col += runeWidth(r)
	}
	// Fill rest of header line
	for col < x+contentWidth {
		screen.SetContent(col, y, ' ', nil, styles.Header)
		col++
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
		isSelected := s.Focused && itemIdx == currentIdx

		// Get the selected background color
		selBg := styles.SelectedBackground
		if selBg == 0 && styles.Selected != nil {
			_, selBg, _ = styles.Selected.Decompose()
		}

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
			textStyle = textStyle.Background(selBg)
		}

		// Fill entire row with background first (edge to edge selection)
		bgStyle := styles.Base
		if isSelected {
			bgStyle = bgStyle.Background(selBg)
		}
		for c := x; c < x+contentWidth; c++ {
			screen.SetContent(c, row, ' ', nil, bgStyle)
		}

		// Draw indicator (status dot or current marker)
		col := x
		indicator := rune(0)
		indicatorStyle := textStyle
		if item.Icon != 0 {
			indicator = item.Icon
			if item.IconStyle != nil {
				indicatorStyle = item.IconStyle
			} else if item.ShowStatus {
				switch item.Status {
				case AIProviderStatusOnline:
					indicatorStyle = styles.StatusOnline
				default:
					indicatorStyle = styles.StatusOffline
				}
			}
		} else if item.IsCurrent {
			indicator = '*'
			indicatorStyle = styles.Current
		}
		if indicator != 0 {
			if isSelected {
				indicatorStyle = indicatorStyle.Background(selBg)
			}
			screen.SetContent(col, row, indicator, nil, indicatorStyle)
		}
		col++
		gapCols := 0
		if indicator != 0 && item.ShowStatus {
			gapCols = 1
			col++
		}

		// Draw label
		label := item.Label
		rightWidth := 0
		if len(item.RightSegments) > 0 {
			for _, seg := range item.RightSegments {
				rightWidth += stringWidth(seg.Text)
			}
		} else {
			rightWidth = stringWidth(item.Hotkey)
		}
		maxLabelWidth := contentWidth - 2 - gapCols // left margin + right margin + optional status gap
		if rightWidth > 0 {
			maxLabelWidth = contentWidth - 2 - gapCols - rightWidth - 1
		}
		if maxLabelWidth < 0 {
			maxLabelWidth = 0
		}
		label = truncateToWidth(label, maxLabelWidth)
		labelRunes := []rune(label)
		matchIdx := 0
		for i, r := range labelRunes {
			if col >= x+contentWidth-1 {
				break
			}
			runeStyle := textStyle
			if matchIdx < len(item.MatchIndices) && item.MatchIndices[matchIdx] == i {
				matchStyle := styles.SearchMatch
				if item.IsDir {
					if styles.SearchMatchDir != nil {
						matchStyle = styles.SearchMatchDir
					}
				} else if styles.SearchMatchFile != nil {
					matchStyle = styles.SearchMatchFile
				}
				if matchStyle != nil {
					runeStyle = matchStyle
				}
				matchIdx++
			}
			screen.SetContent(col, row, r, nil, runeStyle)
			col += runeWidth(r)
		}

		// Draw right-aligned info (hotkey or segments)
		if rightWidth > 0 {
			rightX := x + contentWidth - rightWidth - 1
			if rightX > col {
				if len(item.RightSegments) > 0 {
					rightCol := rightX
					for _, seg := range item.RightSegments {
						if rightCol >= x+contentWidth {
							break
						}
						segStyle := seg.Style
						if segStyle == nil {
							segStyle = styles.Hotkey
						}
						if !item.Available {
							segStyle = styles.Unavailable
						}
						if isSelected {
							segStyle = segStyle.Background(selBg)
						}
						for _, r := range seg.Text {
							if rightCol >= x+contentWidth {
								break
							}
							screen.SetContent(rightCol, row, r, nil, segStyle)
							rightCol += runeWidth(r)
						}
					}
				} else {
					// Hotkey style: keep hotkey color but use selected background if selected
					hotkeyStyle := styles.Hotkey
					if !item.Available {
						hotkeyStyle = styles.Unavailable
					}
					if isSelected {
						hotkeyStyle = hotkeyStyle.Background(selBg)
					}
					hotkeyCol := rightX
					for _, r := range item.Hotkey {
						if hotkeyCol >= x+contentWidth {
							break
						}
						screen.SetContent(hotkeyCol, row, r, nil, hotkeyStyle)
						hotkeyCol += runeWidth(r)
					}
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
