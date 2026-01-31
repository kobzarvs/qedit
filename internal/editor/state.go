package editor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/session"
)

func New(cfg config.Config) *Editor {
	normal := make(map[string]string, len(cfg.Keymap.Normal))
	for k, v := range cfg.Keymap.Normal {
		normal[k] = v
	}
	insert := make(map[string]string, len(cfg.Keymap.Insert))
	for k, v := range cfg.Keymap.Insert {
		insert[k] = v
	}
	tabWidth := cfg.Editor.TabWidth
	if tabWidth < 1 {
		tabWidth = 1
	}

	// Build color palette for reference resolution
	colors := make(map[string]tcell.Color)
	resolve := func(value string, fallback tcell.Color) tcell.Color {
		if value == "" {
			return fallback
		}
		if c, ok := colors[value]; ok {
			return c
		}
		return parseColor(value, fallback)
	}

	colors["foreground"] = parseColor(cfg.Theme.Foreground, tcell.ColorWhite)
	colors["background"] = parseColor(cfg.Theme.Background, tcell.ColorBlack)
	colors["statusline-foreground"] = resolve(cfg.Theme.StatuslineForeground, tcell.ColorBlack)
	colors["statusline-background"] = resolve(cfg.Theme.StatuslineBackground, tcell.ColorGray)
	colors["commandline-foreground"] = resolve(cfg.Theme.CommandlineForeground, colors["statusline-foreground"])
	colors["commandline-background"] = resolve(cfg.Theme.CommandlineBackground, colors["statusline-background"])
	colors["line-number-foreground"] = resolve(cfg.Theme.LineNumberForeground, tcell.ColorGray)
	colors["line-number-active-foreground"] = resolve(cfg.Theme.LineNumberActiveForeground, colors["foreground"])
	colors["selection-foreground"] = resolve(cfg.Theme.SelectionForeground, colors["foreground"])
	colors["selection-background"] = resolve(cfg.Theme.SelectionBackground, colors["background"])
	colors["search-foreground"] = resolve(cfg.Theme.SearchMatchForeground, tcell.ColorBlack)
	colors["search-background"] = resolve(cfg.Theme.SearchMatchBackground, tcell.ColorYellow)
	colors["syntax-keyword"] = resolve(cfg.Theme.SyntaxKeyword, colors["foreground"])
	colors["syntax-string"] = resolve(cfg.Theme.SyntaxString, colors["foreground"])
	colors["syntax-comment"] = resolve(cfg.Theme.SyntaxComment, colors["foreground"])
	colors["syntax-type"] = resolve(cfg.Theme.SyntaxType, colors["foreground"])
	colors["syntax-function"] = resolve(cfg.Theme.SyntaxFunction, colors["foreground"])
	colors["syntax-number"] = resolve(cfg.Theme.SyntaxNumber, colors["foreground"])
	colors["syntax-constant"] = resolve(cfg.Theme.SyntaxConstant, colors["foreground"])
	colors["syntax-operator"] = resolve(cfg.Theme.SyntaxOperator, colors["foreground"])
	colors["syntax-punctuation"] = resolve(cfg.Theme.SyntaxPunctuation, colors["foreground"])
	colors["syntax-field"] = resolve(cfg.Theme.SyntaxField, colors["foreground"])
	colors["syntax-builtin"] = resolve(cfg.Theme.SyntaxBuiltin, colors["foreground"])
	colors["syntax-unknown"] = resolve(cfg.Theme.SyntaxUnknown, tcell.ColorRed)
	colors["syntax-variable"] = resolve(cfg.Theme.SyntaxVariable, colors["foreground"])
	colors["syntax-parameter"] = resolve(cfg.Theme.SyntaxParameter, colors["foreground"])
	colors["branch-foreground"] = resolve(cfg.Theme.BranchForeground, colors["statusline-foreground"])
	colors["branch-background"] = resolve(cfg.Theme.BranchBackground, colors["statusline-background"])
	// Main branch has distinct default color (light green) to stand out
	mainBranchDefaultFg := tcell.NewRGBColor(144, 238, 144) // #90EE90 light green
	colors["main-branch-foreground"] = resolve(cfg.Theme.MainBranchForeground, mainBranchDefaultFg)
	colors["main-branch-background"] = resolve(cfg.Theme.MainBranchBackground, colors["statusline-background"])

	// Keyboard layout colors
	layoutUSFg := tcell.NewRGBColor(144, 238, 144) // #90EE90 light green
	layoutRUFg := tcell.NewRGBColor(135, 206, 250) // #87CEFA light sky blue
	colors["layout-us-foreground"] = layoutUSFg
	colors["layout-ru-foreground"] = layoutRUFg
	colors["layout-other-foreground"] = colors["statusline-foreground"]

	// Autocomplete colors
	colors["autocomplete-background"] = resolve(cfg.Theme.AutocompleteBackground, colors["commandline-background"])
	colors["autocomplete-hotkey"] = resolve(cfg.Theme.AutocompleteHotkey, tcell.ColorWhite)
	colors["autocomplete-description"] = resolve(cfg.Theme.AutocompleteDescription, colors["commandline-foreground"])
	colors["autocomplete-group"] = resolve(cfg.Theme.AutocompleteGroup, tcell.ColorGray)

	// Sidebar colors
	colors["sidebar-foreground"] = resolve(cfg.Theme.SidebarForeground, colors["foreground"])
	colors["sidebar-background"] = resolve(cfg.Theme.SidebarBackground, colors["background"])
	colors["sidebar-dir-foreground"] = resolve(cfg.Theme.SidebarDirForeground, tcell.ColorBlue)
	colors["sidebar-selected-foreground"] = resolve(cfg.Theme.SidebarSelectedForeground, colors["background"])
	colors["sidebar-selected-background"] = resolve(cfg.Theme.SidebarSelectedBackground, tcell.ColorYellow)
	colors["sidebar-header-foreground"] = resolve(cfg.Theme.SidebarHeaderForeground, colors["foreground"])
	colors["sidebar-header-background"] = resolve(cfg.Theme.SidebarHeaderBackground, colors["statusline-background"])
	colors["sidebar-border-foreground"] = resolve(cfg.Theme.SidebarBorderForeground, colors["line-number-foreground"])
	colors["sidebar-hidden-foreground"] = resolve(cfg.Theme.SidebarHiddenForeground, colors["line-number-foreground"])
	colors["sidebar-ignored-foreground"] = resolve(cfg.Theme.SidebarIgnoredForeground, colors["line-number-foreground"])
	colors["sidebar-indicator-foreground"] = resolve(cfg.Theme.SidebarIndicatorForeground, tcell.ColorYellow)
	colors["sidebar-hotkey-foreground"] = resolve(cfg.Theme.SidebarHotkeyForeground, tcell.ColorBlue)
	colors["sidebar-unavailable-foreground"] = resolve(cfg.Theme.SidebarUnavailableForeground, colors["line-number-foreground"])

	lineNumberMode := parseLineNumberMode(cfg.Editor.LineNumbers)
	gitBranchSymbol := strings.TrimSpace(cfg.Editor.GitBranchSymbol)

	// Initialize session manager (ignore error, session persistence is optional)
	sessionMgr, _ := session.NewManager()

	return &Editor{
		lines:                        [][]rune{[]rune{}},
		mode:                         ModeNormal,
		keymap:                       keymapSet{normal: normal, insert: insert},
		tabWidth:                     tabWidth,
		styleMain:                    tcell.StyleDefault.Foreground(colors["foreground"]).Background(colors["background"]),
		styleStatus:                  tcell.StyleDefault.Foreground(colors["statusline-foreground"]).Background(colors["statusline-background"]),
		styleCommand:                 tcell.StyleDefault.Foreground(colors["commandline-foreground"]).Background(colors["commandline-background"]),
		styleLineNumber:              tcell.StyleDefault.Foreground(colors["line-number-foreground"]).Background(colors["background"]),
		styleLineNumberActive:        tcell.StyleDefault.Foreground(colors["line-number-active-foreground"]).Background(colors["background"]),
		styleSelection:               tcell.StyleDefault.Foreground(colors["selection-foreground"]).Background(colors["selection-background"]),
		styleSearchMatch:             tcell.StyleDefault.Foreground(colors["search-foreground"]).Background(colors["search-background"]),
		styleSyntaxKeyword:           tcell.StyleDefault.Foreground(colors["syntax-keyword"]).Background(colors["background"]),
		styleSyntaxString:            tcell.StyleDefault.Foreground(colors["syntax-string"]).Background(colors["background"]),
		styleSyntaxComment:           tcell.StyleDefault.Foreground(colors["syntax-comment"]).Background(colors["background"]),
		styleSyntaxType:              tcell.StyleDefault.Foreground(colors["syntax-type"]).Background(colors["background"]),
		styleSyntaxFunction:          tcell.StyleDefault.Foreground(colors["syntax-function"]).Background(colors["background"]),
		styleSyntaxNumber:            tcell.StyleDefault.Foreground(colors["syntax-number"]).Background(colors["background"]),
		styleSyntaxConstant:          tcell.StyleDefault.Foreground(colors["syntax-constant"]).Background(colors["background"]),
		styleSyntaxOperator:          tcell.StyleDefault.Foreground(colors["syntax-operator"]).Background(colors["background"]),
		styleSyntaxPunctuation:       tcell.StyleDefault.Foreground(colors["syntax-punctuation"]).Background(colors["background"]),
		styleSyntaxField:             tcell.StyleDefault.Foreground(colors["syntax-field"]).Background(colors["background"]),
		styleSyntaxBuiltin:           tcell.StyleDefault.Foreground(colors["syntax-builtin"]).Background(colors["background"]),
		styleSyntaxUnknown:           tcell.StyleDefault.Foreground(colors["syntax-unknown"]).Background(colors["background"]),
		styleSyntaxVariable:          tcell.StyleDefault.Foreground(colors["syntax-variable"]).Background(colors["background"]),
		styleSyntaxParameter:         tcell.StyleDefault.Foreground(colors["syntax-parameter"]).Background(colors["background"]),
		styleTableBorder:             tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(colors["background"]),
		styleBranch:                  tcell.StyleDefault.Foreground(colors["branch-foreground"]).Background(colors["branch-background"]),
		styleMainBranch:              tcell.StyleDefault.Foreground(colors["main-branch-foreground"]).Background(colors["main-branch-background"]),
		styleLayoutUS:                tcell.StyleDefault.Foreground(colors["layout-us-foreground"]).Background(colors["statusline-background"]),
		styleLayoutRU:                tcell.StyleDefault.Foreground(colors["layout-ru-foreground"]).Background(colors["statusline-background"]),
		styleLayoutOther:             tcell.StyleDefault.Foreground(colors["layout-other-foreground"]).Background(colors["statusline-background"]),
		styleAutoComplete:            tcell.StyleDefault.Foreground(colors["autocomplete-description"]).Background(colors["autocomplete-background"]),
		styleAutoCompleteHotkey:      tcell.StyleDefault.Foreground(colors["autocomplete-hotkey"]).Background(colors["autocomplete-background"]),
		styleAutoCompleteDescription: tcell.StyleDefault.Foreground(colors["autocomplete-description"]).Background(colors["autocomplete-background"]),
		styleAutoCompleteGroup:       tcell.StyleDefault.Foreground(colors["autocomplete-group"]).Background(colors["autocomplete-background"]),
		lineNumberMode:               lineNumberMode,
		gitBranchSymbol:              gitBranchSymbol,
		highlightStart:               -1,
		highlightEnd:                 -1,
		sessionManager:               sessionMgr,
		sidebar: NewSidebar(
			cfg.Editor.SidebarWidth,
			cfg.Editor.SidebarMinWidth,
			cfg.Editor.SidebarMaxWidth,
			cfg.Editor.SidebarCloseOnSelect,
		),
		sidebarStyles: SidebarStyles{
			Base:        tcell.StyleDefault.Foreground(colors["sidebar-foreground"]).Background(colors["sidebar-background"]),
			Dir:         tcell.StyleDefault.Foreground(colors["sidebar-dir-foreground"]).Background(colors["sidebar-background"]),
			Selected:    tcell.StyleDefault.Foreground(colors["sidebar-selected-foreground"]).Background(colors["sidebar-selected-background"]),
			Header:      tcell.StyleDefault.Foreground(colors["sidebar-header-foreground"]).Background(colors["sidebar-header-background"]),
			Border:      tcell.StyleDefault.Foreground(colors["sidebar-border-foreground"]).Background(colors["sidebar-background"]),
			Hidden:      tcell.StyleDefault.Foreground(colors["sidebar-hidden-foreground"]).Background(colors["sidebar-background"]),
			Ignored:     tcell.StyleDefault.Foreground(colors["sidebar-ignored-foreground"]).Background(colors["sidebar-background"]),
			Indicator:   tcell.StyleDefault.Foreground(colors["sidebar-indicator-foreground"]).Background(colors["sidebar-background"]),
			Hotkey:      tcell.StyleDefault.Foreground(colors["sidebar-hotkey-foreground"]).Background(colors["sidebar-background"]),
			Unavailable: tcell.StyleDefault.Foreground(colors["sidebar-unavailable-foreground"]).Background(colors["sidebar-background"]),
			Current:     tcell.StyleDefault.Foreground(colors["sidebar-indicator-foreground"]).Background(colors["sidebar-background"]),
		},
	}
}
func (e *Editor) OpenFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	e.lines = splitLines(data)
	if len(e.lines) == 0 {
		e.lines = [][]rune{[]rune{}}
	}
	e.cursor = Cursor{}
	e.scroll = 0
	e.scrollX = 0
	e.mode = ModeNormal
	e.filename = path
	e.cmd = e.cmd[:0]
	e.statusMessage = ""
	e.undo = nil
	e.redo = nil
	e.savePoint = 0
	e.changeTick = 0
	e.lastEdit.Valid = false
	e.highlights = nil
	e.highlightStart = -1
	e.highlightEnd = -1
	e.selectionActive = false
	e.updateDirty()
	_ = e.LoadUndoHistory()

	// Restore session state
	e.restoreSessionState()

	return nil
}
func (e *Editor) restoreSessionState() {
	if e.sessionManager == nil || e.filename == "" {
		return
	}
	absPath, err := filepath.Abs(e.filename)
	if err != nil {
		return
	}
	state, ok := e.sessionManager.GetFileState(absPath)
	if !ok {
		return
	}

	// Restore cursor (clamped to valid range)
	e.cursor.Row = state.CursorRow
	if e.cursor.Row >= len(e.lines) {
		e.cursor.Row = len(e.lines) - 1
	}
	if e.cursor.Row < 0 {
		e.cursor.Row = 0
	}
	e.cursor.Col = state.CursorCol
	if e.cursor.Row < len(e.lines) && e.cursor.Col > len(e.lines[e.cursor.Row]) {
		e.cursor.Col = len(e.lines[e.cursor.Row])
	}
	if e.cursor.Col < 0 {
		e.cursor.Col = 0
	}

	// Restore scroll
	e.scroll = state.ScrollY
	if e.scroll < 0 {
		e.scroll = 0
	}
	e.scrollX = state.ScrollX
	if e.scrollX < 0 {
		e.scrollX = 0
	}

	// Restore mode
	switch state.Mode {
	case "insert":
		e.mode = ModeInsert
	default:
		e.mode = ModeNormal
	}

	// Restore selection with bounds validation
	if state.SelectionActive {
		// Validate selection is within file bounds
		if state.SelectionStartRow >= len(e.lines) || state.SelectionEndRow >= len(e.lines) {
			// File was shortened - reset selection
			e.selectionActive = false
		} else {
			e.selectionActive = true
			// Clamp columns to line lengths
			startCol := state.SelectionStartCol
			if startCol > len(e.lines[state.SelectionStartRow]) {
				startCol = len(e.lines[state.SelectionStartRow])
			}
			endCol := state.SelectionEndCol
			if endCol > len(e.lines[state.SelectionEndRow]) {
				endCol = len(e.lines[state.SelectionEndRow])
			}
			e.selectionStart = Cursor{Row: state.SelectionStartRow, Col: startCol}
			e.selectionEnd = Cursor{Row: state.SelectionEndRow, Col: endCol}
		}
	}
}
func (e *Editor) saveSessionState() {
	if e.sessionManager == nil || e.filename == "" {
		return
	}
	absPath, err := filepath.Abs(e.filename)
	if err != nil {
		return
	}

	mode := "normal"
	if e.mode == ModeInsert {
		mode = "insert"
	}

	state := session.FileState{
		CursorRow:         e.cursor.Row,
		CursorCol:         e.cursor.Col,
		ScrollY:           e.scroll,
		ScrollX:           e.scrollX,
		Mode:              mode,
		SelectionActive:   e.selectionActive,
		SelectionStartRow: e.selectionStart.Row,
		SelectionStartCol: e.selectionStart.Col,
		SelectionEndRow:   e.selectionEnd.Row,
		SelectionEndCol:   e.selectionEnd.Col,
	}
	e.sessionManager.SetFileState(absPath, state)
}

// Shutdown saves session state and stops background tasks
func (e *Editor) Shutdown() {
	e.saveSessionState()
	if e.sessionManager != nil {
		e.sessionManager.Stop()
	}
}
func (e *Editor) Content() string {
	return joinLines(e.lines)
}
func (e *Editor) SetKeyboardLayout(name string) {
	e.layoutName = strings.TrimSpace(name)
}
func (e *Editor) SetGitBranch(name string) {
	e.gitBranch = strings.TrimSpace(name)
}
func (e *Editor) SetGitMainBranch(name string) {
	e.gitMainBranch = strings.TrimSpace(name)
}
func (e *Editor) GetGitMainBranch() string {
	return e.gitMainBranch
}

// IsMainBranch returns true if the current branch is the main branch
func (e *Editor) IsMainBranch() bool {
	if e.gitBranch == "" || e.gitMainBranch == "" {
		return false
	}
	return e.gitBranch == e.gitMainBranch
}

// GetSessionManager returns the session manager for external use
func (e *Editor) GetSessionManager() *session.Manager {
	return e.sessionManager
}
func (e *Editor) SetNodeStackFunc(fn NodeStackFunc) {
	e.nodeStackFunc = fn
}
func (e *Editor) SetLSPGotoFunc(fn LSPGotoFunc) {
	e.lspGotoFunc = fn
}
func (e *Editor) SetHighlightRangeFunc(fn HighlightRangeFunc) {
	e.highlightRangeFunc = fn
}
func (e *Editor) SetStatusMessage(msg string) {
	e.setStatus(msg)
}
func (e *Editor) ChangeTick() uint64 {
	return e.changeTick
}
func (e *Editor) UpdateScroll() {
	if e.freeScroll {
		return
	}
	e.ensureCursorVisible(e.viewHeightCached())
}
func (e *Editor) ConsumeLastEdit() (TextEdit, bool) {
	if !e.lastEdit.Valid {
		return TextEdit{}, false
	}
	edit := e.lastEdit
	e.lastEdit.Valid = false
	return edit, true
}
func (e *Editor) LineCount() int {
	return len(e.lines)
}
func (e *Editor) VisibleRange() (int, int) {
	if len(e.lines) == 0 {
		return 0, 0
	}
	start := e.scroll
	if start < 0 {
		start = 0
	}
	end := start + e.viewHeight - 1
	if end < start {
		end = start
	}
	if end >= len(e.lines) {
		end = len(e.lines) - 1
	}
	return start, end
}
func (e *Editor) SetHighlights(startLine, endLine int, spans map[int][]HighlightSpan) {
	if spans == nil || startLine < 0 || endLine < startLine {
		e.highlights = nil
		e.highlightStart = -1
		e.highlightEnd = -1
		return
	}
	e.highlights = spans
	e.highlightStart = startLine
	e.highlightEnd = endLine
}
func (e *Editor) HasHighlights() bool {
	return e.highlights != nil && e.highlightStart >= 0 && e.highlightEnd >= e.highlightStart
}
