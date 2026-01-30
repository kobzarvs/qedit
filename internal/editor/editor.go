package editor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeCommand
	ModeBranchPicker
	ModeSearch
)

const (
	actionMoveLeft          = "move_left"
	actionMoveRight         = "move_right"
	actionMoveUp            = "move_up"
	actionMoveDown          = "move_down"
	actionWordLeft          = "word_left"
	actionWordRight         = "word_right"
	actionLineStart         = "line_start"
	actionLineEnd           = "line_end"
	actionFileStart         = "file_start"
	actionFileEnd           = "file_end"
	actionPageUp            = "page_up"
	actionPageDown          = "page_down"
	actionMoveLineUp        = "move_line_up"
	actionMoveLineDown      = "move_line_down"
	actionToggleLineNumbers = "toggle_line_numbers"
	actionBranchPicker      = "branch_picker"
	actionEnterInsert       = "enter_insert"
	actionEnterNormal       = "enter_normal"
	actionEnterCommand      = "enter_command"
	actionQuit              = "quit"
	actionBackspace         = "backspace"
	actionNewline           = "newline"
	actionInsertTab         = "insert_tab"
	actionUndo              = "undo"
	actionRedo              = "redo"
	actionDeleteLine        = "delete_line"
	actionDeleteChar        = "delete_char"
	actionDeleteWordLeft    = "delete_word_left"
	actionInsertLineBelow   = "insert_line_below"
	actionUndoLine          = "undo_line"
	actionScrollUp          = "scroll_up"
	actionScrollDown        = "scroll_down"
	actionIndent            = "indent"
	actionUnindent          = "unindent"
	actionSelectAll         = "select_all"

	// Helix-style motions
	actionWordForward       = "word_forward"        // w - move to next word start
	actionWordBackward      = "word_backward"       // b - move to previous word start
	actionWordEnd           = "word_end"            // e - move to word end
	actionGotoMode          = "goto_mode"           // g - enter goto mode
	actionGotoLine          = "goto_line"           // G - go to last line (or specific line)
	actionGotoFirstLine     = "goto_first_line"     // gg - go to first line
	actionGotoFileEnd       = "goto_file_end"       // ge - go to end of file
	actionFindChar          = "find_char"           // f - find char forward
	actionFindCharBackward  = "find_char_backward"  // F - find char backward
	actionTillChar          = "till_char"           // t - till char forward
	actionTillCharBackward  = "till_char_backward"  // T - till char backward

	// Helix-style editing
	actionDelete            = "delete"              // d - delete selection
	actionChange            = "change"              // c - change (delete + insert)
	actionYank              = "yank"                // y - yank/copy
	actionPaste             = "paste"               // p - paste after
	actionPasteBefore       = "paste_before"        // P - paste before
	actionOpenBelow         = "open_below"          // o - open line below
	actionOpenAbove         = "open_above"          // O - open line above
	actionAppend            = "append"              // a - append (insert after cursor)
	actionAppendLineEnd     = "append_line_end"     // A - insert at line end
	actionInsertLineStart   = "insert_line_start"   // I - insert at first non-whitespace
	actionReplaceChar       = "replace_char"        // r - replace with single char
	actionJoinLines         = "join_lines"          // J - join lines

	// Helix-style selection
	actionToggleSelect      = "toggle_select"       // v - toggle selection mode
	actionExtendLine        = "extend_line"         // x - extend to full line
	actionCollapseSelection = "collapse_selection"  // ; - collapse selection to cursor
	actionFlipSelection     = "flip_selection"      // Alt+; - flip selection anchor

	// Space mode
	actionSpaceMode         = "space_mode"          // Space - open space menu

	// Match mode
	actionMatchMode         = "match_mode"          // m - enter match mode

	// View mode
	actionViewMode          = "view_mode"           // z - enter view mode

	// Search
	actionSearchForward     = "search_forward"      // / - exact search forward
	actionSearchBackward    = "search_backward"     // ? - exact search backward
	actionSearchFuzzy       = "search_fuzzy"        // Cmd+F - fuzzy search forward
	actionSearchRegex       = "search_regex"        // Cmd+E - regex search forward
	actionSearchNext        = "search_next"         // n - next match
	actionSearchPrev        = "search_prev"         // N - previous match

	// Special
	actionInsertLineAbove   = "insert_line_above"   // Shift+Enter - insert indented line above cursor

	// Terminal zoom
	actionTerminalZoomIn    = "terminal_zoom_in"    // Cmd+= - zoom in terminal 5x

	// Selection scope
	actionExpandSelection   = "expand_selection"    // Alt+Shift+Up - expand selection to parent node
	actionShrinkSelection   = "shrink_selection"    // Alt+Shift+Down - shrink selection to child node

	// File operations
	actionSave              = "save"                // Cmd+S - save file
)

// SpaceMenuItem represents an item in the space menu
type SpaceMenuItem struct {
	Key         rune
	Label       string
	Action      string
	Implemented bool // whether this action is implemented
}

// SpaceMenuItems defines the space menu structure
var SpaceMenuItems = []SpaceMenuItem{
	{'f', "Open file picker", "file_picker", false},
	{'F', "Open file picker at cwd", "file_picker_cwd", false},
	{'e', "Open file explorer", "file_explorer", false},
	{'E', "Open file explorer at buffer dir", "file_explorer_buffer", false},
	{'b', "Open buffer picker", "buffer_picker", false},
	{'j', "Open jumplist picker", "jumplist_picker", false},
	{'s', "Open symbol picker", "symbol_picker", false},
	{'S', "Open workspace symbol picker", "workspace_symbol_picker", false},
	{'d', "Open diagnostic picker", "diagnostic_picker", false},
	{'D', "Open workspace diagnostic picker", "workspace_diagnostic_picker", false},
	{'g', "Open changed file picker", "changed_file_picker", false},
	{'a', "Perform code action", "code_action", false},
	{'\'', "Open last picker", "last_picker", false},
	{'G', "Debug (experimental)", "debug", false},
	{'w', "Window mode", "window_mode", true},
	{'y', "Yank to clipboard", "yank_clipboard", true},
	{'Y', "Yank main to clipboard", "yank_main_clipboard", true},
	{'p', "Paste from clipboard", "paste_clipboard", true},
	{'P', "Paste before from clipboard", "paste_clipboard_before", true},
	{'R', "Replace with clipboard", "replace_clipboard", false},
	{'/', "Global search", "global_search", false},
	{'k', "Show docs for item", "show_docs", false},
	{'r', "Rename symbol", "rename_symbol", false},
	{'h', "Select symbol references", "select_references", false},
	{'c', "Comment/uncomment", "toggle_comment", true},
	{'C', "Block comment/uncomment", "toggle_block_comment", false},
	{'?', "Show all keybindings", "show_keybindings", true},
}

// GotoMenuItems defines the goto mode menu (g prefix)
var GotoMenuItems = []SpaceMenuItem{
	{'g', "Go to file start", "goto_first_line", true},
	{'e', "Go to file end", "goto_file_end", true},
	{'h', "Go to line start", "line_start", true},
	{'l', "Go to line end", "line_end", true},
	{'s', "Go to first non-whitespace", "goto_first_nonblank", true},
	{'d', "Go to definition", "goto_definition", false},
	{'D', "Go to declaration", "goto_declaration", false},
	{'y', "Go to type definition", "goto_type_definition", false},
	{'r', "Go to references", "goto_references", false},
	{'i', "Go to implementation", "goto_implementation", false},
	{'t', "Go to window top", "goto_window_top", true},
	{'c', "Go to window center", "goto_window_center", true},
	{'b', "Go to window bottom", "goto_window_bottom", true},
	{'a', "Go to last accessed file", "goto_last_accessed", false},
	{'m', "Go to last modified file", "goto_last_modified", false},
	{'n', "Go to next buffer", "goto_next_buffer", false},
	{'p', "Go to previous buffer", "goto_prev_buffer", false},
	{'.', "Go to last change", "goto_last_change", false},
}

// MatchMenuItems defines the match mode menu (m prefix)
var MatchMenuItems = []SpaceMenuItem{
	{'m', "Go to matching bracket", "match_bracket", true},
	{'s', "Surround add", "surround_add", false},
	{'r', "Surround replace", "surround_replace", false},
	{'d', "Surround delete", "surround_delete", false},
	{'a', "Select around object", "select_around", false},
	{'i', "Select inside object", "select_inside", false},
}

// ViewMenuItems defines the view/scroll mode menu (z prefix)
var ViewMenuItems = []SpaceMenuItem{
	{'c', "Center cursor line", "view_center", true},
	{'t', "Scroll cursor to top", "view_top", true},
	{'b', "Scroll cursor to bottom", "view_bottom", true},
	{'k', "Scroll up", "scroll_up", true},
	{'j', "Scroll down", "scroll_down", true},
}

// WindowMenuItems defines the window mode menu (space-w prefix)
var WindowMenuItems = []SpaceMenuItem{
	{'w', "Switch to next window", "window_next", false},
	{'v', "Vertical split", "window_vsplit", false},
	{'s', "Horizontal split", "window_hsplit", false},
	{'h', "Move to left window", "window_left", false},
	{'j', "Move to window below", "window_down", false},
	{'k', "Move to window above", "window_up", false},
	{'l', "Move to right window", "window_right", false},
	{'q', "Close window", "window_close", false},
	{'o', "Close other windows", "window_only", false},
}

type actionKind int

const (
	actionInsertRune actionKind = iota
	actionDeleteRune
	actionSplitLine
	actionJoinLine
	actionMoveLine
	actionInsertText // Bulk insert of multiple lines
	actionDeleteText // Bulk delete of multiple lines
)

type action struct {
	kind           actionKind
	pos            Cursor
	r              rune
	rowFrom        int
	rowTo          int
	group          uint64
	text           [][]rune // For bulk text operations
	endPos         Cursor   // End position for bulk delete
	selectionStart Cursor   // Selection to restore on undo
	selectionEnd   Cursor   // Selection to restore on undo
	hasSelection   bool     // Whether to restore selection on undo
}

// actionJSON is used for serializing actions to changelog files
type actionJSON struct {
	Kind           int        `json:"k"`
	PosRow         int        `json:"pr"`
	PosCol         int        `json:"pc"`
	R              rune       `json:"r,omitempty"`
	RowFrom        int        `json:"rf,omitempty"`
	RowTo          int        `json:"rt,omitempty"`
	Group          uint64     `json:"g"`
	Text           []string   `json:"t,omitempty"`
	EndPosRow      int        `json:"er,omitempty"`
	EndPosCol      int        `json:"ec,omitempty"`
	SelectionStart [2]int     `json:"ss,omitempty"`
	SelectionEnd   [2]int     `json:"se,omitempty"`
	HasSelection   bool       `json:"hs,omitempty"`
}

type Cursor struct {
	Row int
	Col int
}

type TextEdit struct {
	Valid          bool
	StartByte      int
	OldEndByte     int
	NewEndByte     int
	StartRow       int
	StartColBytes  int
	OldEndRow      int
	OldEndColBytes int
	NewEndRow      int
	NewEndColBytes int
}

type HighlightSpan struct {
	StartCol int
	EndCol   int
	Kind     string
}

type keymapSet struct {
	normal map[string]string
	insert map[string]string
}

// NodeRange represents a syntax node's position range
type NodeRange struct {
	StartRow int
	StartCol int
	EndRow   int
	EndCol   int
}

// NodeStackFunc is a callback to get syntax node stack at a position
type NodeStackFunc func(path string, row, col int) []NodeRange

type Editor struct {
	lines                  [][]rune
	cursor                 Cursor
	scroll                 int
	mode                   Mode
	filename               string
	dirty                  bool
	keymap                 keymapSet
	cmd                    []rune
	cmdCursor              int      // cursor position within cmd
	cmdHistory             []string // command history
	cmdHistoryIndex        int      // current position in history (-1 = not browsing)
	cmdHistoryPrefix       string   // prefix for filtered history search
	statusMessage          string
	undo                   []action
	redo                   []action
	savePoint              int
	tabWidth               int
	viewHeight             int
	styleMain              tcell.Style
	styleStatus            tcell.Style
	styleCommand           tcell.Style
	styleLineNumber        tcell.Style
	styleLineNumberActive  tcell.Style
	styleSelection         tcell.Style
	styleSearchMatch       tcell.Style
	styleSyntaxKeyword     tcell.Style
	styleSyntaxString      tcell.Style
	styleSyntaxComment     tcell.Style
	styleSyntaxType        tcell.Style
	styleSyntaxFunction    tcell.Style
	styleSyntaxNumber      tcell.Style
	styleSyntaxConstant    tcell.Style
	styleSyntaxOperator    tcell.Style
	styleSyntaxPunctuation tcell.Style
	styleSyntaxField       tcell.Style
	styleSyntaxBuiltin     tcell.Style
	styleSyntaxUnknown     tcell.Style
	styleSyntaxVariable    tcell.Style
	styleSyntaxParameter   tcell.Style
	lineNumberMode         LineNumberMode
	layoutName             string
	gitBranch              string
	gitBranchSymbol        string
	selectionActive        bool
	selectionStart         Cursor
	selectionEnd           Cursor
	highlights             map[int][]HighlightSpan
	highlightStart         int
	highlightEnd           int
	changeTick             uint64
	lastEdit               TextEdit
	branchPickerActive     bool
	branchPickerItems      []string
	branchPickerIndex      int
	branchPickerRequested  bool
	branchPickerSelection  string
	lineUndoRow            int
	lineUndoContent        []rune
	lineUndoValid          bool
	lastKeyCombo           string
	freeScroll             bool
	lastScrollTime         time.Time
	undoGroup              uint64

	// Helix-style state
	clipboard              [][]rune               // yanked text (lines)
	pendingAction          string                 // pending action waiting for char input (f/F/t/T/r)
	selectMode             bool                   // whether in visual/select mode
	lastFindChar           rune                   // last char used in f/F/t/T
	lastFindForward        bool                   // direction of last find
	lastFindTill           bool                   // whether last find was till (t/T)
	gotoMode               bool                   // whether in goto mode (g prefix)
	matchMode              bool                   // whether in match mode (m prefix)
	viewMode               bool                   // whether in view mode (z prefix)
	windowMode             bool                   // whether in window mode (space-w prefix)
	pendingKeys            string                 // keys typed so far in a sequence (e.g., "g" waiting for second key)
	lastCommand            string                 // last executed command for display (e.g., "gg", "ge", "fw")
	spaceMenuActive        bool                   // whether space menu is open
	keybindingsHelpActive  bool                   // whether keybindings help popup is open
	keybindingsHelpScroll  int                    // scroll position in keybindings help

	// Search state
	searchQuery            []rune                 // current search query
	searchCursor           int                    // cursor position within search query
	searchMatches          []SearchMatch          // all matches in the file
	searchMatchIndex       int                    // current match index
	searchForward          bool                   // search direction
	searchFuzzy            bool                   // true = fuzzy search (cmd+f), false = exact (/)
	searchRegex            bool                   // true = regex search (cmd+e)
	lastSearchQuery        string                 // last search query for n/N
	searchHistory          []string               // search history (prefixed with /: F: or E:)
	searchHistoryIndex     int                    // current position in search history (-1 = not browsing)
	searchHistoryPrefix    string                 // prefix for filtered search history

	// Terminal zoom state
	zoomPendingRestore     bool                   // true = waiting for space to restore zoom

	// Copied message state
	copiedMessageTime      time.Time              // when "copied" was shown

	// Selection scope (expand/shrink)
	nodeStackFunc          NodeStackFunc          // callback to get syntax node stack
	selectionScopeStack    []NodeRange            // stack of selection scopes for shrinking
	selectionScopeIndex    int                    // current index in scope stack
}

// SearchMatch represents a match location
type SearchMatch struct {
	Row         int
	Col         int
	Length      int
	Score       int   // fuzzy match score (higher = better)
	MatchedCols []int // columns of matched chars within the word (for fuzzy highlight)
}

type LineNumberMode int

const (
	LineNumberOff LineNumberMode = iota
	LineNumberAbsolute
	LineNumberRelative
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
	mainFg := parseColor(cfg.Theme.Foreground, tcell.ColorWhite)
	mainBg := parseColor(cfg.Theme.Background, tcell.ColorBlack)
	statusFg := parseColor(cfg.Theme.StatuslineForeground, tcell.ColorBlack)
	statusBg := parseColor(cfg.Theme.StatuslineBackground, tcell.ColorGray)
	commandFg := parseColor(cfg.Theme.CommandlineForeground, statusFg)
	commandBg := parseColor(cfg.Theme.CommandlineBackground, statusBg)
	lineNumberFg := parseColor(cfg.Theme.LineNumberForeground, tcell.ColorGray)
	lineNumberActiveFg := parseColor(cfg.Theme.LineNumberActiveForeground, mainFg)
	selectionFg := parseColor(cfg.Theme.SelectionForeground, mainFg)
	selectionBg := parseColor(cfg.Theme.SelectionBackground, mainBg)
	searchMatchFg := parseColor(cfg.Theme.SearchMatchForeground, tcell.ColorBlack)
	searchMatchBg := parseColor(cfg.Theme.SearchMatchBackground, tcell.ColorYellow)
	syntaxKeyword := parseColor(cfg.Theme.SyntaxKeyword, mainFg)
	syntaxString := parseColor(cfg.Theme.SyntaxString, mainFg)
	syntaxComment := parseColor(cfg.Theme.SyntaxComment, mainFg)
	syntaxType := parseColor(cfg.Theme.SyntaxType, mainFg)
	syntaxFunction := parseColor(cfg.Theme.SyntaxFunction, mainFg)
	syntaxNumber := parseColor(cfg.Theme.SyntaxNumber, mainFg)
	syntaxConstant := parseColor(cfg.Theme.SyntaxConstant, mainFg)
	syntaxOperator := parseColor(cfg.Theme.SyntaxOperator, mainFg)
	syntaxPunctuation := parseColor(cfg.Theme.SyntaxPunctuation, mainFg)
	syntaxField := parseColor(cfg.Theme.SyntaxField, mainFg)
	syntaxBuiltin := parseColor(cfg.Theme.SyntaxBuiltin, mainFg)
	syntaxUnknown := parseColor(cfg.Theme.SyntaxUnknown, tcell.ColorRed)
	syntaxVariable := parseColor(cfg.Theme.SyntaxVariable, mainFg)
	syntaxParameter := parseColor(cfg.Theme.SyntaxParameter, mainFg)
	lineNumber := tcell.StyleDefault.Foreground(lineNumberFg).Background(mainBg)
	lineNumberActive := tcell.StyleDefault.Foreground(lineNumberActiveFg).Background(mainBg)
	selection := tcell.StyleDefault.Foreground(selectionFg).Background(selectionBg)
	searchMatch := tcell.StyleDefault.Foreground(searchMatchFg).Background(searchMatchBg)
	syntaxKeywordStyle := tcell.StyleDefault.Foreground(syntaxKeyword).Background(mainBg)
	syntaxStringStyle := tcell.StyleDefault.Foreground(syntaxString).Background(mainBg)
	syntaxCommentStyle := tcell.StyleDefault.Foreground(syntaxComment).Background(mainBg)
	syntaxTypeStyle := tcell.StyleDefault.Foreground(syntaxType).Background(mainBg)
	syntaxFunctionStyle := tcell.StyleDefault.Foreground(syntaxFunction).Background(mainBg)
	syntaxNumberStyle := tcell.StyleDefault.Foreground(syntaxNumber).Background(mainBg)
	syntaxConstantStyle := tcell.StyleDefault.Foreground(syntaxConstant).Background(mainBg)
	syntaxOperatorStyle := tcell.StyleDefault.Foreground(syntaxOperator).Background(mainBg)
	syntaxPunctuationStyle := tcell.StyleDefault.Foreground(syntaxPunctuation).Background(mainBg)
	syntaxFieldStyle := tcell.StyleDefault.Foreground(syntaxField).Background(mainBg)
	syntaxBuiltinStyle := tcell.StyleDefault.Foreground(syntaxBuiltin).Background(mainBg)
	syntaxUnknownStyle := tcell.StyleDefault.Foreground(syntaxUnknown).Background(mainBg)
	syntaxVariableStyle := tcell.StyleDefault.Foreground(syntaxVariable).Background(mainBg)
	syntaxParameterStyle := tcell.StyleDefault.Foreground(syntaxParameter).Background(mainBg)
	lineNumberMode := parseLineNumberMode(cfg.Editor.LineNumbers)
	gitBranchSymbol := strings.TrimSpace(cfg.Editor.GitBranchSymbol)
	return &Editor{
		lines:                  [][]rune{[]rune{}},
		mode:                   ModeNormal,
		keymap:                 keymapSet{normal: normal, insert: insert},
		tabWidth:               tabWidth,
		styleMain:              tcell.StyleDefault.Foreground(mainFg).Background(mainBg),
		styleStatus:            tcell.StyleDefault.Foreground(statusFg).Background(statusBg),
		styleCommand:           tcell.StyleDefault.Foreground(commandFg).Background(commandBg),
		styleLineNumber:        lineNumber,
		styleLineNumberActive:  lineNumberActive,
		styleSelection:         selection,
		styleSearchMatch:       searchMatch,
		styleSyntaxKeyword:     syntaxKeywordStyle,
		styleSyntaxString:      syntaxStringStyle,
		styleSyntaxComment:     syntaxCommentStyle,
		styleSyntaxType:        syntaxTypeStyle,
		styleSyntaxFunction:    syntaxFunctionStyle,
		styleSyntaxNumber:      syntaxNumberStyle,
		styleSyntaxConstant:    syntaxConstantStyle,
		styleSyntaxOperator:    syntaxOperatorStyle,
		styleSyntaxPunctuation: syntaxPunctuationStyle,
		styleSyntaxField:       syntaxFieldStyle,
		styleSyntaxBuiltin:     syntaxBuiltinStyle,
		styleSyntaxUnknown:     syntaxUnknownStyle,
		styleSyntaxVariable:    syntaxVariableStyle,
		styleSyntaxParameter:   syntaxParameterStyle,
		lineNumberMode:         lineNumberMode,
		gitBranchSymbol:        gitBranchSymbol,
		highlightStart:         -1,
		highlightEnd:           -1,
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
	e.updateDirty()
	_ = e.LoadUndoHistory()
	return nil
}

func (e *Editor) HandleKey(ev *tcell.EventKey) bool {
	e.freeScroll = false
	if e.mode != ModeCommand && e.mode != ModeSearch && e.statusMessage != "" {
		e.statusMessage = ""
	}
	// Track last key combination for display
	e.lastKeyCombo = keyStringDisplay(ev)
	switch e.mode {
	case ModeInsert:
		return e.handleInsert(ev)
	case ModeCommand:
		return e.handleCommand(ev)
	case ModeBranchPicker:
		return e.handleBranchPicker(ev)
	case ModeSearch:
		return e.handleSearch(ev)
	default:
		return e.handleNormal(ev)
	}
}

func (e *Editor) HandleMouse(ev *tcell.EventMouse) {
	if ev.Buttons() == tcell.WheelUp {
		e.scrollUp(3)
		e.freeScroll = true
		e.lastScrollTime = time.Now()
	} else if ev.Buttons() == tcell.WheelDown {
		e.scrollDown(3)
		e.freeScroll = true
		e.lastScrollTime = time.Now()
	} else if ev.Buttons() == tcell.Button1 {
		e.handleMouseClick(ev)
	}
}

func (e *Editor) handleMouseClick(ev *tcell.EventMouse) {
	x, y := ev.Position()

	// Convert screen Y to line number
	row := y + e.scroll
	if row < 0 {
		row = 0
	}
	if row >= len(e.lines) {
		row = len(e.lines) - 1
	}
	if row < 0 {
		return // empty file
	}

	// Convert screen X to column (accounting for gutter)
	gutterW := e.gutterWidth()
	visualX := x - gutterW
	if visualX < 0 {
		visualX = 0
	}

	// Convert visual column to logical column
	col := visualToLogicalCol(e.lines[row], visualX, e.tabWidth)

	// Set cursor position
	e.cursor.Row = row
	e.cursor.Col = col
	e.clampCursorCol()

	// Clear selection and free scroll mode
	e.selectionActive = false
	e.freeScroll = false
}

func (e *Editor) scrollUp(lines int) {
	e.scroll -= lines
	if e.scroll < 0 {
		e.scroll = 0
	}
}

func (e *Editor) scrollDown(lines int) {
	// Keep last line at least 5 lines above status line
	viewHeight := e.viewHeightCached()
	maxScroll := len(e.lines) - viewHeight + 5
	if maxScroll < 0 {
		maxScroll = 0
	}
	e.scroll += lines
	if e.scroll > maxScroll {
		e.scroll = maxScroll
	}
}

// scrollViewUp scrolls the view up (shows earlier lines), keeping cursor visible
func (e *Editor) scrollViewUp() {
	if e.scroll <= 0 {
		return
	}
	e.scroll--
	e.lastScrollTime = time.Now()
	// If cursor is now below visible area, move it up
	viewHeight := e.viewHeightCached()
	if e.cursor.Row >= e.scroll+viewHeight {
		e.cursor.Row = e.scroll + viewHeight - 1
		e.clampCursorCol()
	}
}

// scrollViewDown scrolls the view down (shows later lines), keeping cursor visible
func (e *Editor) scrollViewDown() {
	// Keep last line at least 5 lines above status line
	viewHeight := e.viewHeightCached()
	maxScroll := len(e.lines) - viewHeight + 5
	if maxScroll < 0 {
		maxScroll = 0
	}
	if e.scroll >= maxScroll {
		return
	}
	e.scroll++
	e.lastScrollTime = time.Now()
	// If cursor is now above visible area, move it down
	if e.cursor.Row < e.scroll {
		e.cursor.Row = e.scroll
		e.clampCursorCol()
	}
}

func (e *Editor) Render(s tcell.Screen) {
	w, h := s.Size()
	if w <= 0 || h <= 0 {
		return
	}

	statusY := h - 2
	cmdY := h - 1
	viewHeight := h - 2
	if h < 2 {
		statusY = h - 1
		cmdY = h - 1
	}
	if viewHeight < 0 {
		viewHeight = 0
	}
	e.viewHeight = viewHeight
	if !e.freeScroll {
		e.ensureCursorVisible(viewHeight)
	}

	s.SetStyle(e.styleMain)
	s.Clear()

	gutterWidth := e.gutterWidth()
	for y := 0; y < viewHeight; y++ {
		lineIdx := e.scroll + y
		if lineIdx >= len(e.lines) {
			clearLine(s, y, w, e.styleMain)
			continue
		}
		e.drawLineWithGutter(s, y, w, gutterWidth, lineIdx)
	}

	// Draw scroll indicator if recently scrolled
	e.renderScrollIndicator(s, w, viewHeight)

	var cx, cy int
	if statusY >= 0 && !e.zoomPendingRestore {
		e.renderStatusline(s, w, statusY)
	}
	if cmdY >= 0 && !e.zoomPendingRestore {
		cmdCursor := e.renderCommandline(s, w, cmdY)
		if e.mode == ModeCommand || e.mode == ModeSearch {
			cx = cmdCursor
			cy = cmdY
		}
	}
	cursorVisible := true
	if e.mode != ModeCommand && e.mode != ModeSearch && e.mode != ModeBranchPicker {
		cy = e.cursor.Row - e.scroll
		if cy < 0 || cy >= viewHeight {
			cursorVisible = false
		}
		if e.cursor.Row >= 0 && e.cursor.Row < len(e.lines) {
			cx = gutterWidth + visualCol(e.lines[e.cursor.Row], e.cursor.Col, e.tabWidth)
		}
		if cx >= w {
			cx = w - 1
		}
	}

	if e.branchPickerActive {
		e.renderBranchPicker(s, w, viewHeight)
	}
	if e.spaceMenuActive {
		e.renderSpaceMenu(s, w, viewHeight)
	}
	if e.gotoMode {
		e.renderMenu(s, w, viewHeight, "Goto", GotoMenuItems)
	}
	if e.matchMode {
		e.renderMenu(s, w, viewHeight, "Match", MatchMenuItems)
	}
	if e.viewMode {
		e.renderMenu(s, w, viewHeight, "View", ViewMenuItems)
	}
	if e.windowMode {
		e.renderMenu(s, w, viewHeight, "Window", WindowMenuItems)
	}
	if e.keybindingsHelpActive {
		e.renderKeybindingsHelp(s, w, viewHeight)
	}
	if e.mode == ModeBranchPicker || e.spaceMenuActive || e.keybindingsHelpActive || !cursorVisible {
		s.HideCursor()
		s.Show()
		return
	}
	cursorStyle := tcell.CursorStyleSteadyBlock
	if e.mode == ModeInsert || e.mode == ModeSearch || e.mode == ModeCommand {
		cursorStyle = tcell.CursorStyleSteadyBar
	}
	s.SetCursorStyle(cursorStyle)
	s.ShowCursor(cx, cy)
	s.Show()
}

func (e *Editor) handleNormal(ev *tcell.EventKey) bool {
	// Handle zoom mode - only allow = (more zoom) or space (restore)
	if e.zoomPendingRestore {
		if ev.Key() == tcell.KeyRune {
			switch ev.Rune() {
			case ' ':
				e.sendTerminalZoom(false, 20) // zoom out 20 times
				e.zoomPendingRestore = false
				return false
			case '=':
				e.sendTerminalZoom(true, 20) // zoom in more
				return false
			}
		}
		// Block all other keys during zoom mode
		return false
	}

	// Handle space menu
	if e.spaceMenuActive {
		return e.handleSpaceMenu(ev)
	}

	// Handle keybindings help popup
	if e.keybindingsHelpActive {
		return e.handleKeybindingsHelp(ev)
	}

	// Handle goto mode (g prefix)
	if e.gotoMode {
		e.gotoMode = false
		e.pendingKeys = ""
		if ev.Key() == tcell.KeyEscape {
			return false
		}
		if ev.Key() == tcell.KeyRune {
			return e.handleGotoKey(ev.Rune())
		}
		return false
	}

	// Handle match mode (m prefix)
	if e.matchMode {
		e.matchMode = false
		e.pendingKeys = ""
		if ev.Key() == tcell.KeyEscape {
			return false
		}
		if ev.Key() == tcell.KeyRune {
			return e.handleMatchKey(ev.Rune())
		}
		return false
	}

	// Handle view mode (z prefix)
	if e.viewMode {
		e.viewMode = false
		e.pendingKeys = ""
		if ev.Key() == tcell.KeyEscape {
			return false
		}
		if ev.Key() == tcell.KeyRune {
			return e.handleViewKey(ev.Rune())
		}
		return false
	}

	// Handle window mode (space-w prefix)
	if e.windowMode {
		e.windowMode = false
		e.pendingKeys = ""
		if ev.Key() == tcell.KeyEscape {
			return false
		}
		if ev.Key() == tcell.KeyRune {
			return e.handleWindowKey(ev.Rune())
		}
		return false
	}

	// Handle pending char input (f/F/t/T/r)
	if e.pendingAction != "" {
		pendingKey := e.pendingKeys
		e.pendingKeys = ""
		if ev.Key() == tcell.KeyEscape {
			e.pendingAction = ""
			return false
		}
		if ev.Key() == tcell.KeyRune {
			e.handlePendingChar(ev.Rune())
			e.lastCommand = pendingKey + string(ev.Rune())
			return false
		}
		// Ignore other keys while waiting for char
		return false
	}

	if e.handleSelectionMove(ev) {
		return false
	}
	key := keyStringForMap(ev, e.keymap.normal)
	if key == "" {
		return false
	}
	action, ok := e.keymap.normal[key]
	if !ok {
		return false
	}

	// Helix-style: w, b, e, f, F, t, T - anchor moves to old cursor, cursor moves to target
	// Selection covers what was "jumped over"
	if isHelixSelectingMotion(action) {
		// Anchor = where cursor WAS
		anchor := e.cursor
		result := e.execAction(action)
		if anchor != e.cursor {
			// Selection from old position to new position
			e.selectionActive = true
			e.selectionStart = anchor
			e.selectionEnd = e.cursor
			e.selectMode = true
		}
		return result
	}

	// In select mode, extend selection for other motion commands
	if e.selectMode && isMotionAction(action) {
		before := e.cursor
		result := e.execAction(action)
		if before != e.cursor {
			e.selectionEnd = e.cursor
		}
		return result
	}

	return e.execAction(action)
}

// handleGotoKey handles the second key after 'g' prefix
func (e *Editor) handleGotoKey(ch rune) bool {
	var action string
	switch ch {
	case 'g':
		action = actionGotoFirstLine
	case 'e':
		action = actionGotoFileEnd
	case 'h':
		action = actionLineStart
	case 'l':
		action = actionLineEnd
	case 's':
		action = actionFileStart // same as gg
	default:
		return false
	}

	// Record the executed command
	e.lastCommand = "g" + string(ch)

	// In select mode, extend selection
	if e.selectMode && isMotionAction(action) {
		before := e.cursor
		result := e.execAction(action)
		if before != e.cursor {
			e.selectionEnd = e.cursor
		}
		return result
	}

	return e.execAction(action)
}

// handleMatchKey handles the second key after 'm' prefix
func (e *Editor) handleMatchKey(ch rune) bool {
	e.lastCommand = "m" + string(ch)

	switch ch {
	case 'm':
		e.goToMatchingBracket()
	case 'a':
		e.setStatus("select around (not implemented)")
	case 'i':
		e.setStatus("select inside (not implemented)")
	case 's':
		e.setStatus("surround add (not implemented)")
	case 'r':
		e.setStatus("surround replace (not implemented)")
	case 'd':
		e.setStatus("surround delete (not implemented)")
	default:
		return false
	}
	return false
}

// handleViewKey handles the second key after 'z' prefix
func (e *Editor) handleViewKey(ch rune) bool {
	e.lastCommand = "z" + string(ch)

	switch ch {
	case 'c':
		e.centerCursorLine()
	case 't':
		e.scrollCursorToTop()
	case 'b':
		e.scrollCursorToBottom()
	case 'k':
		e.scrollUp(1)
	case 'j':
		e.scrollDown(1)
	default:
		return false
	}
	return false
}

// handleWindowKey handles the second key after 'space-w' prefix
func (e *Editor) handleWindowKey(ch rune) bool {
	e.lastCommand = "SPC w" + string(ch)
	e.setStatus("window mode (not implemented)")
	return false
}

// handleKeybindingsHelp handles key input in keybindings help popup
func (e *Editor) handleKeybindingsHelp(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyEnter:
		e.keybindingsHelpActive = false
		return false
	case tcell.KeyUp, tcell.KeyCtrlP:
		if e.keybindingsHelpScroll > 0 {
			e.keybindingsHelpScroll--
		}
	case tcell.KeyDown, tcell.KeyCtrlN:
		e.keybindingsHelpScroll++
	case tcell.KeyPgUp:
		e.keybindingsHelpScroll -= 10
		if e.keybindingsHelpScroll < 0 {
			e.keybindingsHelpScroll = 0
		}
	case tcell.KeyPgDn:
		e.keybindingsHelpScroll += 10
	case tcell.KeyRune:
		if ev.Rune() == 'q' {
			e.keybindingsHelpActive = false
		} else if ev.Rune() == 'j' {
			e.keybindingsHelpScroll++
		} else if ev.Rune() == 'k' && e.keybindingsHelpScroll > 0 {
			e.keybindingsHelpScroll--
		}
	}
	return false
}

// goToMatchingBracket jumps to the matching bracket or quote
func (e *Editor) goToMatchingBracket() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}
	line := e.lines[e.cursor.Row]
	if e.cursor.Col < 0 || e.cursor.Col >= len(line) {
		return
	}

	ch := line[e.cursor.Col]

	// Handle quotes/backticks (same char for open/close)
	if ch == '"' || ch == '\'' || ch == '`' {
		e.goToMatchingQuote(ch)
		return
	}

	// Handle brackets (different chars for open/close)
	var match rune
	var forward bool

	switch ch {
	case '(':
		match, forward = ')', true
	case ')':
		match, forward = '(', false
	case '[':
		match, forward = ']', true
	case ']':
		match, forward = '[', false
	case '{':
		match, forward = '}', true
	case '}':
		match, forward = '{', false
	case '<':
		match, forward = '>', true
	case '>':
		match, forward = '<', false
	default:
		e.setStatus("no bracket or quote under cursor")
		return
	}

	depth := 1
	row, col := e.cursor.Row, e.cursor.Col

	if forward {
		col++
		for row < len(e.lines) {
			line := e.lines[row]
			for col < len(line) {
				if line[col] == ch {
					depth++
				} else if line[col] == match {
					depth--
					if depth == 0 {
						e.cursor.Row = row
						e.cursor.Col = col
						return
					}
				}
				col++
			}
			row++
			col = 0
		}
	} else {
		col--
		for row >= 0 {
			line := e.lines[row]
			if col < 0 {
				row--
				if row >= 0 {
					col = len(e.lines[row]) - 1
				}
				continue
			}
			for col >= 0 {
				if line[col] == ch {
					depth++
				} else if line[col] == match {
					depth--
					if depth == 0 {
						e.cursor.Row = row
						e.cursor.Col = col
						return
					}
				}
				col--
			}
			row--
			if row >= 0 {
				col = len(e.lines[row]) - 1
			}
		}
	}
	e.setStatus("no matching bracket found")
}

// goToMatchingQuote jumps to the matching quote character
// For quotes, we determine if it's opening or closing by counting quotes before cursor
func (e *Editor) goToMatchingQuote(quoteChar rune) {
	row, col := e.cursor.Row, e.cursor.Col

	// Count quotes of this type before cursor position to determine if opening/closing
	// Even count = opening quote (search forward)
	// Odd count = closing quote (search backward)
	count := 0
	for r := 0; r <= row; r++ {
		line := e.lines[r]
		endCol := len(line)
		if r == row {
			endCol = col
		}
		for c := 0; c < endCol; c++ {
			if line[c] == quoteChar {
				// Skip escaped quotes (check for backslash before)
				if c > 0 && line[c-1] == '\\' {
					continue
				}
				count++
			}
		}
	}

	if count%2 == 0 {
		// Opening quote - search forward for closing
		e.findMatchingQuoteForward(quoteChar)
	} else {
		// Closing quote - search backward for opening
		e.findMatchingQuoteBackward(quoteChar)
	}
}

// findMatchingQuoteForward finds the closing quote
func (e *Editor) findMatchingQuoteForward(quoteChar rune) {
	row, col := e.cursor.Row, e.cursor.Col+1

	for row < len(e.lines) {
		line := e.lines[row]
		for col < len(line) {
			if line[col] == quoteChar {
				// Check if escaped
				escaped := false
				if col > 0 && line[col-1] == '\\' {
					// Count consecutive backslashes
					bs := 0
					for i := col - 1; i >= 0 && line[i] == '\\'; i-- {
						bs++
					}
					escaped = bs%2 == 1
				}
				if !escaped {
					e.cursor.Row = row
					e.cursor.Col = col
					return
				}
			}
			col++
		}
		row++
		col = 0
	}
	e.setStatus("no matching quote found")
}

// findMatchingQuoteBackward finds the opening quote
func (e *Editor) findMatchingQuoteBackward(quoteChar rune) {
	row, col := e.cursor.Row, e.cursor.Col-1

	for row >= 0 {
		if col < 0 {
			row--
			if row >= 0 {
				col = len(e.lines[row]) - 1
			}
			continue
		}
		line := e.lines[row]
		for col >= 0 {
			if line[col] == quoteChar {
				// Check if escaped
				escaped := false
				if col > 0 && line[col-1] == '\\' {
					// Count consecutive backslashes
					bs := 0
					for i := col - 1; i >= 0 && line[i] == '\\'; i-- {
						bs++
					}
					escaped = bs%2 == 1
				}
				if !escaped {
					e.cursor.Row = row
					e.cursor.Col = col
					return
				}
			}
			col--
		}
		row--
		if row >= 0 {
			col = len(e.lines[row]) - 1
		}
	}
	e.setStatus("no matching quote found")
}

// centerCursorLine scrolls to center cursor line on screen
func (e *Editor) centerCursorLine() {
	viewHeight := e.viewHeightCached()
	e.scroll = e.cursor.Row - viewHeight/2
	if e.scroll < 0 {
		e.scroll = 0
	}
}

// scrollCursorToTop scrolls to put cursor line at top
func (e *Editor) scrollCursorToTop() {
	e.scroll = e.cursor.Row
}

// scrollCursorToBottom scrolls to put cursor line at bottom
func (e *Editor) scrollCursorToBottom() {
	viewHeight := e.viewHeightCached()
	e.scroll = e.cursor.Row - viewHeight + 1
	if e.scroll < 0 {
		e.scroll = 0
	}
}

// toggleLineComment toggles comment on current line or selection
func (e *Editor) toggleLineComment() {
	// Detect comment prefix based on file extension
	ext := filepath.Ext(e.filename)
	var prefix string
	switch ext {
	case ".go", ".c", ".cpp", ".h", ".java", ".js", ".ts", ".rs", ".swift":
		prefix = "//"
	case ".py", ".sh", ".bash", ".zsh", ".yaml", ".yml", ".toml", ".rb":
		prefix = "#"
	case ".lua", ".sql":
		prefix = "--"
	case ".vim":
		prefix = "\""
	case ".html", ".xml":
		prefix = "<!--"
	default:
		prefix = "//"
	}

	start, end := e.cursor.Row, e.cursor.Row
	if s, en, ok := e.selectionRange(); ok {
		start, end = s.Row, en.Row
		// If selection ends at column 0, don't include that line
		// (common when selecting full lines - cursor ends up at start of next line)
		if en.Col == 0 && en.Row > s.Row {
			end = en.Row - 1
		}
	}

	// Validate range
	if start < 0 {
		start = 0
	}
	if end >= len(e.lines) {
		end = len(e.lines) - 1
	}
	if start > end {
		return // nothing to do
	}

	// Find minimum indentation (only count non-empty lines)
	minIndent := -1
	for row := start; row <= end; row++ {
		line := e.lines[row]
		if len(line) == 0 {
			continue // skip empty lines for indent calculation
		}
		indent := 0
		for _, r := range line {
			if r == ' ' || r == '\t' {
				indent++
			} else {
				break
			}
		}
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent < 0 {
		minIndent = 0
	}

	// Check if all non-empty lines are already commented at minIndent position
	allCommented := true
	for row := start; row <= end; row++ {
		line := string(e.lines[row])
		if len(line) == 0 {
			continue // skip empty lines
		}
		// Check if comment prefix exists at or after minIndent
		rest := line
		if minIndent < len(line) {
			rest = line[minIndent:]
		}
		trimmed := strings.TrimLeft(rest, " \t")
		if !strings.HasPrefix(trimmed, prefix) {
			allCommented = false
			break
		}
	}

	e.startUndoGroup()

	for row := start; row <= end; row++ {
		line := e.lines[row]
		lineStr := string(line)

		// Skip empty lines
		if len(lineStr) == 0 {
			continue
		}

		if allCommented {
			// Remove comment - find the prefix and remove it
			idx := strings.Index(lineStr, prefix)
			if idx >= 0 {
				// Remove prefix and one space if present
				removeLen := len(prefix)
				if idx+removeLen < len(lineStr) && lineStr[idx+removeLen] == ' ' {
					removeLen++
				}
				newLine := lineStr[:idx] + lineStr[idx+removeLen:]
				e.lines[row] = []rune(newLine)
			}
		} else {
			// Add comment at minIndent position
			insertAt := minIndent
			if insertAt > len(lineStr) {
				insertAt = len(lineStr)
			}
			newLine := lineStr[:insertAt] + prefix + " " + lineStr[insertAt:]
			e.lines[row] = []rune(newLine)
		}
	}

	e.finishUndoGroup()
	e.dirty = true
	e.lastEdit.Valid = false
}

// handleSpaceMenu handles key input when space menu is active
func (e *Editor) handleSpaceMenu(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyEscape {
		e.spaceMenuActive = false
		e.pendingKeys = ""
		return false
	}

	if ev.Key() == tcell.KeyRune {
		ch := ev.Rune()
		for _, item := range SpaceMenuItems {
			if item.Key == ch {
				e.spaceMenuActive = false
				e.pendingKeys = ""
				e.lastCommand = "SPC " + string(ch)
				return e.executeSpaceAction(item)
			}
		}
	}

	// Unknown key - close menu
	e.spaceMenuActive = false
	e.pendingKeys = ""
	return false
}

// executeSpaceAction executes the action from space menu
func (e *Editor) executeSpaceAction(item SpaceMenuItem) bool {
	if !item.Implemented {
		e.setStatus(item.Label + " (not implemented)")
		return false
	}

	switch item.Action {
	case "yank_clipboard":
		e.yankToSystemClipboard()
	case "yank_main_clipboard":
		e.yankToSystemClipboard()
	case "paste_clipboard":
		e.pasteFromSystemClipboard(false)
	case "paste_clipboard_before":
		e.pasteFromSystemClipboard(true)
	case "window_mode":
		e.windowMode = true
		e.pendingKeys = "SPC w"
		return false
	case "toggle_comment":
		e.toggleLineComment()
	case "show_keybindings":
		e.keybindingsHelpActive = true
		e.keybindingsHelpScroll = 0
	default:
		e.setStatus(item.Label + " (not implemented)")
	}
	return false
}

// yankToSystemClipboard copies selection to system clipboard
func (e *Editor) yankToSystemClipboard() {
	// First yank to internal clipboard
	e.yankSelection()

	// Then copy to system clipboard if available
	if len(e.clipboard) == 0 {
		return
	}

	// Build text from clipboard
	var sb strings.Builder
	for i, line := range e.clipboard {
		if i > 0 {
			sb.WriteRune('\n')
		}
		sb.WriteString(string(line))
	}

	// Try to copy to system clipboard using pbcopy on macOS
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(sb.String())
	if err := cmd.Run(); err != nil {
		e.setStatus("yanked (clipboard unavailable)")
		return
	}
	e.setStatus("yanked to clipboard")
}

// pasteFromSystemClipboard pastes from system clipboard
func (e *Editor) pasteFromSystemClipboard(before bool) {
	// Try to get from system clipboard using pbpaste on macOS
	cmd := exec.Command("pbpaste")
	output, err := cmd.Output()
	if err != nil {
		e.setStatus("clipboard unavailable")
		return
	}

	text := string(output)
	if text == "" {
		e.setStatus("clipboard empty")
		return
	}

	// Parse into lines
	lines := strings.Split(text, "\n")
	e.clipboard = make([][]rune, len(lines))
	for i, line := range lines {
		e.clipboard[i] = []rune(line)
	}

	// Paste
	if before {
		e.pasteBefore()
	} else {
		e.pasteAfter()
	}
	e.setStatus("pasted from clipboard")
}

// isMotionAction returns true if the action is a motion that should extend selection
func isMotionAction(action string) bool {
	switch action {
	case actionMoveLeft, actionMoveRight, actionMoveUp, actionMoveDown,
		actionWordLeft, actionWordRight, actionLineStart, actionLineEnd,
		actionFileStart, actionFileEnd, actionPageUp, actionPageDown,
		actionWordForward, actionWordBackward, actionWordEnd,
		actionGotoLine, actionGotoFirstLine, actionGotoFileEnd,
		actionFindChar, actionFindCharBackward, actionTillChar, actionTillCharBackward:
		return true
	}
	return false
}

// isHelixSelectingMotion returns true if motion should auto-start selection (Helix style)
// These motions extend selection from current position to target
func isHelixSelectingMotion(action string) bool {
	switch action {
	case actionWordForward, actionWordBackward, actionWordEnd,
		actionFindChar, actionFindCharBackward, actionTillChar, actionTillCharBackward:
		return true
	}
	return false
}


func (e *Editor) handleInsert(ev *tcell.EventKey) bool {
	if e.handleSelectionMove(ev) {
		return false
	}
	key := keyStringForMap(ev, e.keymap.insert)
	if key != "" {
		if action, ok := e.keymap.insert[key]; ok {
			return e.execAction(action)
		}
	}
	if ev.Key() == tcell.KeyRune {
		e.clearSelection()
		e.insertRune(ev.Rune())
	}
	return false
}

func (e *Editor) handleCommand(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		e.mode = ModeNormal
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	case tcell.KeyCtrlC:
		e.mode = ModeNormal
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	case tcell.KeyEnter:
		cmd := strings.TrimSpace(string(e.cmd))
		e.mode = ModeNormal
		// Add to history if not empty and different from last
		if cmd != "" && (len(e.cmdHistory) == 0 || e.cmdHistory[len(e.cmdHistory)-1] != cmd) {
			e.cmdHistory = append(e.cmdHistory, cmd)
			e.saveCmdHistory()
		}
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return e.execCommand(cmd)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if e.cmdCursor > 0 && len(e.cmd) > 0 {
			// Delete char before cursor
			e.cmd = append(e.cmd[:e.cmdCursor-1], e.cmd[e.cmdCursor:]...)
			e.cmdCursor--
			e.cmdHistoryIndex = -1
		}
		return false
	case tcell.KeyDelete:
		if e.cmdCursor < len(e.cmd) {
			// Delete char at cursor
			e.cmd = append(e.cmd[:e.cmdCursor], e.cmd[e.cmdCursor+1:]...)
			e.cmdHistoryIndex = -1
		}
		return false
	case tcell.KeyLeft, tcell.KeyCtrlB: // Ctrl+B = back (readline)
		if e.cmdCursor > 0 {
			e.cmdCursor--
		}
		return false
	case tcell.KeyRight, tcell.KeyCtrlF: // Ctrl+F = forward (readline)
		if e.cmdCursor < len(e.cmd) {
			e.cmdCursor++
		}
		return false
	case tcell.KeyHome, tcell.KeyCtrlA: // Ctrl+A = beginning of line
		e.cmdCursor = 0
		return false
	case tcell.KeyEnd, tcell.KeyCtrlE: // Ctrl+E = end of line
		e.cmdCursor = len(e.cmd)
		return false
	case tcell.KeyUp, tcell.KeyCtrlP: // Ctrl+P = previous
		e.cmdHistoryUp()
		return false
	case tcell.KeyDown, tcell.KeyCtrlN: // Ctrl+N = next
		e.cmdHistoryDown()
		return false
	case tcell.KeyCtrlU: // Ctrl+U = clear line
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	case tcell.KeyCtrlK: // Ctrl+K = kill to end of line
		e.cmd = e.cmd[:e.cmdCursor]
		e.cmdHistoryIndex = -1
		return false
	case tcell.KeyCtrlW: // Ctrl+W = delete word backward
		if e.cmdCursor > 0 {
			// Find start of previous word
			i := e.cmdCursor - 1
			for i > 0 && e.cmd[i-1] == ' ' {
				i--
			}
			for i > 0 && e.cmd[i-1] != ' ' {
				i--
			}
			e.cmd = append(e.cmd[:i], e.cmd[e.cmdCursor:]...)
			e.cmdCursor = i
			e.cmdHistoryIndex = -1
		}
		return false
	case tcell.KeyRune:
		// Insert char at cursor position
		e.cmd = append(e.cmd[:e.cmdCursor], append([]rune{ev.Rune()}, e.cmd[e.cmdCursor:]...)...)
		e.cmdCursor++
		e.cmdHistoryIndex = -1
		return false
	}
	return false
}

// cmdHistoryUp navigates to older command in history
func (e *Editor) cmdHistoryUp() {
	if len(e.cmdHistory) == 0 {
		return
	}

	// First time pressing up - save current prefix for filtering
	if e.cmdHistoryIndex == -1 {
		e.cmdHistoryPrefix = string(e.cmd)
		e.cmdHistoryIndex = len(e.cmdHistory)
	}

	// Find previous matching command
	for i := e.cmdHistoryIndex - 1; i >= 0; i-- {
		if strings.HasPrefix(e.cmdHistory[i], e.cmdHistoryPrefix) {
			e.cmdHistoryIndex = i
			e.cmd = []rune(e.cmdHistory[i])
			e.cmdCursor = len(e.cmd)
			return
		}
	}
}

// cmdHistoryDown navigates to newer command in history
func (e *Editor) cmdHistoryDown() {
	if e.cmdHistoryIndex == -1 {
		return
	}

	// Find next matching command
	for i := e.cmdHistoryIndex + 1; i < len(e.cmdHistory); i++ {
		if strings.HasPrefix(e.cmdHistory[i], e.cmdHistoryPrefix) {
			e.cmdHistoryIndex = i
			e.cmd = []rune(e.cmdHistory[i])
			e.cmdCursor = len(e.cmd)
			return
		}
	}

	// No more matches - restore original prefix
	e.cmdHistoryIndex = -1
	e.cmd = []rune(e.cmdHistoryPrefix)
	e.cmdCursor = len(e.cmd)
}

// historyFilePath returns the path to the command history file
func historyFilePath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history"), nil
}

// LoadCmdHistory loads command history from file
func (e *Editor) LoadCmdHistory() {
	path, err := historyFilePath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return // File doesn't exist yet, that's ok
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line != "" {
			e.cmdHistory = append(e.cmdHistory, line)
		}
	}
}

// saveCmdHistory saves command history to file
func (e *Editor) saveCmdHistory() {
	path, err := historyFilePath()
	if err != nil {
		return
	}
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	// Keep only last 1000 commands
	history := e.cmdHistory
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}
	data := strings.Join(history, "\n")
	_ = os.WriteFile(path, []byte(data), 0644)
}

// searchHistoryFilePath returns the path to the search history file
func searchHistoryFilePath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "search_history"), nil
}

// LoadSearchHistory loads search history from file
func (e *Editor) LoadSearchHistory() {
	path, err := searchHistoryFilePath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return // File doesn't exist yet, that's ok
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" {
			e.searchHistory = append(e.searchHistory, line)
		}
	}
}

// saveSearchHistory saves search history to file
func (e *Editor) saveSearchHistory() {
	path, err := searchHistoryFilePath()
	if err != nil {
		return
	}
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	// Keep only last 1000 searches
	history := e.searchHistory
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}
	data := strings.Join(history, "\n")
	_ = os.WriteFile(path, []byte(data), 0644)
}

// addSearchToHistory adds a search query to history with type prefix
// Prefix: "/:" for exact, "F:" for fuzzy, "E:" for regex
func (e *Editor) addSearchToHistory(query string) {
	if query == "" {
		return
	}
	var prefix string
	if e.searchRegex {
		prefix = "E:"
	} else if e.searchFuzzy {
		prefix = "F:"
	} else {
		prefix = "/:"
	}
	entry := prefix + query
	// Don't add duplicates consecutively
	if len(e.searchHistory) > 0 && e.searchHistory[len(e.searchHistory)-1] == entry {
		return
	}
	e.searchHistory = append(e.searchHistory, entry)
	e.saveSearchHistory()
}

// currentSearchPrefix returns the prefix for current search type
func (e *Editor) currentSearchPrefix() string {
	if e.searchRegex {
		return "E:"
	} else if e.searchFuzzy {
		return "F:"
	}
	return "/:"
}

// navigateSearchHistory navigates search history (direction: -1 for older, 1 for newer)
func (e *Editor) navigateSearchHistory(direction int) {
	if len(e.searchHistory) == 0 {
		return
	}

	prefix := e.currentSearchPrefix()

	// Save current query as prefix when starting history navigation
	if e.searchHistoryIndex == -1 && direction < 0 {
		e.searchHistoryPrefix = string(e.searchQuery)
	}

	// Find matching entries in history (filter by search type prefix)
	startIdx := e.searchHistoryIndex
	if startIdx == -1 {
		startIdx = len(e.searchHistory)
	}

	if direction < 0 {
		// Going back in history
		for i := startIdx - 1; i >= 0; i-- {
			entry := e.searchHistory[i]
			if strings.HasPrefix(entry, prefix) {
				query := strings.TrimPrefix(entry, prefix)
				// If we have a prefix filter, apply it
				if e.searchHistoryPrefix == "" || strings.HasPrefix(query, e.searchHistoryPrefix) {
					e.searchHistoryIndex = i
					e.searchQuery = []rune(query)
					e.searchCursor = len(e.searchQuery)
					e.updateSearchMatches()
					return
				}
			}
		}
	} else {
		// Going forward in history
		for i := startIdx + 1; i < len(e.searchHistory); i++ {
			entry := e.searchHistory[i]
			if strings.HasPrefix(entry, prefix) {
				query := strings.TrimPrefix(entry, prefix)
				if e.searchHistoryPrefix == "" || strings.HasPrefix(query, e.searchHistoryPrefix) {
					e.searchHistoryIndex = i
					e.searchQuery = []rune(query)
					e.searchCursor = len(e.searchQuery)
					e.updateSearchMatches()
					return
				}
			}
		}
		// No more forward - restore original prefix
		e.searchHistoryIndex = -1
		e.searchQuery = []rune(e.searchHistoryPrefix)
		e.searchCursor = len(e.searchQuery)
		e.updateSearchMatches()
	}
}

func (e *Editor) handleSearch(ev *tcell.EventKey) bool {
	// Handle Cmd+Up/Down for navigating matches in file
	if ev.Modifiers()&tcell.ModMeta != 0 {
		switch ev.Key() {
		case tcell.KeyUp:
			// Navigate to previous match
			if len(e.searchMatches) > 0 {
				e.searchMatchIndex--
				if e.searchMatchIndex < 0 {
					e.searchMatchIndex = len(e.searchMatches) - 1
				}
				e.jumpToCurrentMatch()
			}
			return false
		case tcell.KeyDown:
			// Navigate to next match
			if len(e.searchMatches) > 0 {
				e.searchMatchIndex++
				if e.searchMatchIndex >= len(e.searchMatches) {
					e.searchMatchIndex = 0
				}
				e.jumpToCurrentMatch()
			}
			return false
		}
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		e.mode = ModeNormal
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.searchMatches = nil
		e.searchHistoryIndex = -1
		return false
	case tcell.KeyCtrlC:
		e.mode = ModeNormal
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.searchMatches = nil
		e.searchHistoryIndex = -1
		return false
	case tcell.KeyEnter:
		// Confirm search and go to first/current match
		query := string(e.searchQuery)
		if query != "" {
			e.addSearchToHistory(query)
			e.lastSearchQuery = query
		}
		if len(e.searchMatches) > 0 {
			match := e.searchMatches[e.searchMatchIndex]
			e.cursor.Row = match.Row
			e.cursor.Col = match.Col
		}
		e.mode = ModeNormal
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.searchHistoryIndex = -1
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if e.searchCursor > 0 && len(e.searchQuery) > 0 {
			e.searchQuery = append(e.searchQuery[:e.searchCursor-1], e.searchQuery[e.searchCursor:]...)
			e.searchCursor--
			e.updateSearchMatches()
		}
		return false
	case tcell.KeyDelete:
		if e.searchCursor < len(e.searchQuery) {
			e.searchQuery = append(e.searchQuery[:e.searchCursor], e.searchQuery[e.searchCursor+1:]...)
			e.updateSearchMatches()
		}
		return false
	case tcell.KeyLeft, tcell.KeyCtrlB:
		if e.searchCursor > 0 {
			e.searchCursor--
		}
		return false
	case tcell.KeyRight, tcell.KeyCtrlF:
		if e.searchCursor < len(e.searchQuery) {
			e.searchCursor++
		}
		return false
	case tcell.KeyHome, tcell.KeyCtrlA:
		e.searchCursor = 0
		return false
	case tcell.KeyEnd, tcell.KeyCtrlE:
		e.searchCursor = len(e.searchQuery)
		return false
	case tcell.KeyUp, tcell.KeyCtrlP:
		// Navigate history (older)
		e.navigateSearchHistory(-1)
		return false
	case tcell.KeyDown, tcell.KeyCtrlN:
		// Navigate history (newer)
		e.navigateSearchHistory(1)
		return false
	case tcell.KeyCtrlU:
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.updateSearchMatches()
		return false
	case tcell.KeyCtrlW:
		if e.searchCursor > 0 {
			i := e.searchCursor - 1
			for i > 0 && e.searchQuery[i-1] == ' ' {
				i--
			}
			for i > 0 && e.searchQuery[i-1] != ' ' {
				i--
			}
			e.searchQuery = append(e.searchQuery[:i], e.searchQuery[e.searchCursor:]...)
			e.searchCursor = i
			e.updateSearchMatches()
		}
		return false
	case tcell.KeyRune:
		e.searchQuery = append(e.searchQuery[:e.searchCursor], append([]rune{ev.Rune()}, e.searchQuery[e.searchCursor:]...)...)
		e.searchCursor++
		e.updateSearchMatches()
		return false
	}
	return false
}

// updateSearchMatches performs fuzzy search and updates matches
func (e *Editor) updateSearchMatches() {
	e.searchMatches = nil
	e.searchMatchIndex = 0

	query := string(e.searchQuery)
	if query == "" {
		return
	}

	// Regex search mode
	if e.searchRegex {
		re, err := regexp.Compile("(?i)" + query) // case-insensitive
		if err != nil {
			// Invalid regex, show error in status
			e.setStatus("regex error: " + err.Error())
			return
		}
		for row, line := range e.lines {
			lineStr := string(line)
			matches := re.FindAllStringIndex(lineStr, -1)
			for _, m := range matches {
				// Convert byte positions to rune positions
				col := utf8.RuneCountInString(lineStr[:m[0]])
				length := utf8.RuneCountInString(lineStr[m[0]:m[1]])
				e.searchMatches = append(e.searchMatches, SearchMatch{
					Row:    row,
					Col:    col,
					Length: length,
					Score:  1000,
				})
			}
		}
	} else {
		queryLower := strings.ToLower(query)

		// Search through all lines
		for row, line := range e.lines {
			lineStr := string(line)
			lineLower := strings.ToLower(lineStr)

			// Find all exact substring matches in this line first
			offset := 0
			for {
				col := strings.Index(lineLower[offset:], queryLower)
				if col < 0 {
					break
				}
				e.searchMatches = append(e.searchMatches, SearchMatch{
					Row:    row,
					Col:    offset + col,
					Length: len(query),
					Score:  1000, // Exact match gets high score
				})
				offset += col + 1
				if offset >= len(lineLower) {
					break
				}
			}

			// In fuzzy mode, find words containing all query letters
			if e.searchFuzzy {
				words := extractWords(line)
				for _, w := range words {
					// Skip if this word position is already covered by an exact match
					alreadyMatched := false
					for _, m := range e.searchMatches {
						if m.Row == row && w.start >= m.Col && w.start < m.Col+m.Length {
							alreadyMatched = true
							break
						}
					}
					if alreadyMatched {
						continue
					}

					// Check fuzzy match (sequential or chunk-based)
					if matchedPositions := fuzzyMatchWord(w.word, query); matchedPositions != nil {
						e.searchMatches = append(e.searchMatches, SearchMatch{
							Row:         row,
							Col:         w.start,
							Length:      len([]rune(w.word)),
							Score:       500, // Fuzzy match score
							MatchedCols: matchedPositions,
						})
					}
				}
			}
		}
	}

	// Sort by row, then by column
	sortSearchMatches(e.searchMatches)

	// Find match closest to cursor
	if len(e.searchMatches) > 0 {
		e.searchMatchIndex = 0
		for i, match := range e.searchMatches {
			if match.Row >= e.cursor.Row {
				e.searchMatchIndex = i
				break
			}
		}
		e.jumpToCurrentMatch()
	}
}

// sequentialMatch checks if all query chars appear in word in order
// e.g., "anwl" matches "actionsWorld" as [a]ctio[n][W]or[l]d
// Returns matched positions (rune indices) or nil if no match
func sequentialMatch(word, query string) []int {
	wordRunes := []rune(strings.ToLower(word))
	queryLower := strings.ToLower(query)

	var positions []int
	wi := 0
	for _, qc := range queryLower {
		found := false
		for wi < len(wordRunes) {
			if wordRunes[wi] == qc {
				positions = append(positions, wi)
				wi++
				found = true
				break
			}
			wi++
		}
		if !found {
			return nil
		}
	}
	return positions
}

// chunkMatch checks if query can be split into 2 chunks that both exist in word
// e.g., "lidra" -> "li" + "dra" both found in "drawLine"
// Returns matched positions (rune indices) or nil if no match
func chunkMatch(word, query string) []int {
	if len(query) < 2 {
		return nil
	}
	wordLower := strings.ToLower(word)
	wordRunes := []rune(wordLower)
	queryLower := strings.ToLower(query)

	// Try all possible 2-chunk splits
	for i := 1; i < len(queryLower); i++ {
		chunk1 := queryLower[:i]
		chunk2 := queryLower[i:]

		idx1 := strings.Index(wordLower, chunk1)
		idx2 := strings.Index(wordLower, chunk2)

		// Both chunks must exist in word
		if idx1 >= 0 && idx2 >= 0 {
			var positions []int
			// Convert byte positions to rune positions and collect all matched runes
			runeIdx1 := utf8.RuneCountInString(wordLower[:idx1])
			runeIdx2 := utf8.RuneCountInString(wordLower[:idx2])
			chunk1Runes := []rune(chunk1)
			chunk2Runes := []rune(chunk2)

			for j := 0; j < len(chunk1Runes); j++ {
				positions = append(positions, runeIdx1+j)
			}
			for j := 0; j < len(chunk2Runes); j++ {
				pos := runeIdx2 + j
				// Avoid duplicates if chunks overlap
				duplicate := false
				for _, p := range positions {
					if p == pos {
						duplicate = true
						break
					}
				}
				if !duplicate {
					positions = append(positions, pos)
				}
			}

			// Sort positions
			for i := 0; i < len(positions); i++ {
				for j := i + 1; j < len(positions); j++ {
					if positions[j] < positions[i] {
						positions[i], positions[j] = positions[j], positions[i]
					}
				}
			}

			// Validate: matched positions should equal query length (ignoring overlaps)
			if len(positions) >= len(wordRunes) || len(positions) < len([]rune(query)) {
				continue
			}
			return positions
		}
	}
	return nil
}

// fuzzyMatchWord checks if word matches query using fuzzy algorithms
// Returns matched positions (rune indices) or nil if no match
func fuzzyMatchWord(word, query string) []int {
	// Try sequential match first (letters in order)
	if positions := sequentialMatch(word, query); positions != nil {
		return positions
	}
	// Try chunk match (query split into 2 parts, both found in word)
	if positions := chunkMatch(word, query); positions != nil {
		return positions
	}
	return nil
}

// isWordChar returns true if the rune is part of a word/identifier
func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// wordMatch holds a word and its position in a line
type wordMatch struct {
	word  string
	start int
	end   int
}

// extractWords extracts all words/identifiers from a line with their positions
func extractWords(line []rune) []wordMatch {
	var words []wordMatch
	i := 0
	for i < len(line) {
		// Skip non-word characters
		for i < len(line) && !isWordChar(line[i]) {
			i++
		}
		if i >= len(line) {
			break
		}
		// Collect word
		start := i
		for i < len(line) && isWordChar(line[i]) {
			i++
		}
		words = append(words, wordMatch{
			word:  string(line[start:i]),
			start: start,
			end:   i,
		})
	}
	return words
}

// sortSearchMatches sorts matches by row (for navigation)
func sortSearchMatches(matches []SearchMatch) {
	// Simple bubble sort (matches are usually small)
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Row < matches[i].Row ||
				(matches[j].Row == matches[i].Row && matches[j].Col < matches[i].Col) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
}

// jumpToCurrentMatch moves cursor to the current search match
func (e *Editor) jumpToCurrentMatch() {
	if e.searchMatchIndex < 0 || e.searchMatchIndex >= len(e.searchMatches) {
		return
	}
	match := e.searchMatches[e.searchMatchIndex]
	e.cursor.Row = match.Row
	e.cursor.Col = match.Col + match.Length // cursor at end of word
	e.ensureCursorVisible(e.viewHeightCached())

	// Select the whole matched word for editing (d/c/r/DEL)
	if match.Length > 0 {
		e.selectionActive = true
		e.selectionStart = Cursor{Row: match.Row, Col: match.Col}
		e.selectionEnd = Cursor{Row: match.Row, Col: match.Col + match.Length}
	}
}

// enterSearchMode enters search mode
func (e *Editor) enterSearchMode(forward bool, fuzzy bool, regex bool) {
	e.mode = ModeSearch
	e.searchQuery = e.searchQuery[:0]
	e.searchCursor = 0
	e.searchMatches = nil
	e.searchMatchIndex = 0
	e.searchForward = forward
	e.searchFuzzy = fuzzy
	e.searchRegex = regex
	if regex {
		e.pendingKeys = "E"
	} else if fuzzy {
		e.pendingKeys = "F"
	} else {
		e.pendingKeys = "/"
	}
}

// searchNext goes to next match
func (e *Editor) searchNext() {
	if e.lastSearchQuery == "" {
		e.setStatus("no previous search")
		return
	}

	// Re-run search if matches are empty
	if len(e.searchMatches) == 0 {
		e.searchQuery = []rune(e.lastSearchQuery)
		e.updateSearchMatches()
	}

	if len(e.searchMatches) == 0 {
		e.setStatus("no matches")
		return
	}

	// Find next match after cursor
	found := false
	for i, match := range e.searchMatches {
		if match.Row > e.cursor.Row || (match.Row == e.cursor.Row && match.Col > e.cursor.Col) {
			e.searchMatchIndex = i
			found = true
			break
		}
	}
	if !found {
		e.searchMatchIndex = 0 // Wrap around
	}

	e.jumpToCurrentMatch()
	e.setStatus(fmt.Sprintf("[%d/%d] %s", e.searchMatchIndex+1, len(e.searchMatches), e.lastSearchQuery))
}

// searchPrev goes to previous match
func (e *Editor) searchPrev() {
	if e.lastSearchQuery == "" {
		e.setStatus("no previous search")
		return
	}

	// Re-run search if matches are empty
	if len(e.searchMatches) == 0 {
		e.searchQuery = []rune(e.lastSearchQuery)
		e.updateSearchMatches()
	}

	if len(e.searchMatches) == 0 {
		e.setStatus("no matches")
		return
	}

	// Find previous match before cursor
	found := false
	for i := len(e.searchMatches) - 1; i >= 0; i-- {
		match := e.searchMatches[i]
		if match.Row < e.cursor.Row || (match.Row == e.cursor.Row && match.Col < e.cursor.Col) {
			e.searchMatchIndex = i
			found = true
			break
		}
	}
	if !found {
		e.searchMatchIndex = len(e.searchMatches) - 1 // Wrap around
	}

	e.jumpToCurrentMatch()
	e.setStatus(fmt.Sprintf("[%d/%d] %s", e.searchMatchIndex+1, len(e.searchMatches), e.lastSearchQuery))
}

func (e *Editor) handleSelectionMove(ev *tcell.EventKey) bool {
	if ev.Modifiers()&tcell.ModShift == 0 {
		return false
	}
	// Don't handle if Alt is pressed - let keymap handle alt+shift combinations
	if ev.Modifiers()&tcell.ModAlt != 0 {
		return false
	}
	switch ev.Key() {
	case tcell.KeyLeft:
		if ev.Modifiers()&tcell.ModMeta != 0 {
			e.extendSelection(e.moveWordLeft)
		} else {
			e.extendSelection(e.moveLeft)
		}
		return true
	case tcell.KeyRight:
		if ev.Modifiers()&tcell.ModMeta != 0 {
			e.extendSelection(e.moveWordRight)
		} else {
			e.extendSelection(e.moveRight)
		}
		return true
	case tcell.KeyUp:
		e.extendSelection(e.moveUp)
		return true
	case tcell.KeyDown:
		e.extendSelection(e.moveDown)
		return true
	case tcell.KeyPgUp:
		e.extendSelection(e.pageUp)
		return true
	case tcell.KeyPgDn:
		e.extendSelection(e.pageDown)
		return true
	case tcell.KeyHome:
		if ev.Modifiers()&tcell.ModMeta != 0 {
			e.extendSelection(e.moveFileStart)
			return true
		}
		e.extendSelection(e.moveLineStart)
		return true
	case tcell.KeyEnd:
		if ev.Modifiers()&tcell.ModMeta != 0 {
			e.extendSelection(e.moveFileEnd)
			return true
		}
		e.extendSelection(e.moveLineEnd)
		return true
	}
	return false
}

func (e *Editor) handleBranchPicker(ev *tcell.EventKey) bool {
	switch keyString(ev) {
	case "esc", "ctrl+c":
		e.closeBranchPicker("")
		return false
	case "enter":
		if len(e.branchPickerItems) == 0 {
			e.closeBranchPicker("")
			return false
		}
		selection := e.branchPickerItems[e.branchPickerIndex]
		e.closeBranchPicker(selection)
		return false
	case "up", "k":
		e.branchPickerIndex--
	case "down", "j":
		e.branchPickerIndex++
	case "pgup":
		e.branchPickerIndex -= e.branchPickerPageSize()
	case "pgdn":
		e.branchPickerIndex += e.branchPickerPageSize()
	case "home":
		e.branchPickerIndex = 0
	case "end":
		e.branchPickerIndex = len(e.branchPickerItems) - 1
	default:
		return false
	}
	if e.branchPickerIndex < 0 {
		e.branchPickerIndex = 0
	}
	if e.branchPickerIndex >= len(e.branchPickerItems) {
		e.branchPickerIndex = len(e.branchPickerItems) - 1
		if e.branchPickerIndex < 0 {
			e.branchPickerIndex = 0
		}
	}
	return false
}

func (e *Editor) execAction(action string) bool {
	switch action {
	case actionMoveLeft:
		e.moveLeft()
	case actionMoveRight:
		e.moveRight()
	case actionMoveUp:
		e.moveUp()
	case actionMoveDown:
		e.moveDown()
	case actionWordLeft:
		e.moveWordLeft()
	case actionWordRight:
		e.moveWordRight()
	case actionLineStart:
		e.moveLineStart()
	case actionLineEnd:
		e.moveLineEnd()
	case actionFileStart:
		e.moveFileStart()
	case actionFileEnd:
		e.moveFileEnd()
	case actionPageUp:
		e.pageUp()
	case actionPageDown:
		e.pageDown()
	case actionMoveLineUp:
		e.moveLineUp()
	case actionMoveLineDown:
		e.moveLineDown()
	case actionToggleLineNumbers:
		e.toggleLineNumbers()
	case actionBranchPicker:
		e.branchPickerRequested = true
	case actionEnterInsert:
		e.mode = ModeInsert
		e.saveLineState()
	case actionEnterNormal:
		e.mode = ModeNormal
	case actionEnterCommand:
		e.mode = ModeCommand
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
	case actionQuit:
		return true
	case actionBackspace:
		e.backspace()
	case actionNewline:
		e.insertNewline()
	case actionInsertTab:
		e.insertTab()
	case actionUndo:
		e.Undo()
		return false // Don't clear selection - undo may restore it
	case actionRedo:
		e.Redo()
		return false // Don't clear selection
	case actionDeleteLine:
		e.deleteLine()
	case actionDeleteChar:
		e.deleteChar()
	case actionDeleteWordLeft:
		e.deleteWordLeft()
	case actionInsertLineBelow:
		e.insertLineBelow()
	case actionUndoLine:
		e.undoLine()
	case actionScrollUp:
		e.scrollViewUp()
	case actionScrollDown:
		e.scrollViewDown()
	case actionIndent:
		e.indentSelection()
		return false // Don't clear selection
	case actionUnindent:
		e.unindentSelection()
		return false // Don't clear selection
	case actionSelectAll:
		e.selectAll()
		return false // Don't clear selection

	// Helix-style motions
	case actionWordForward:
		e.wordForward()
	case actionWordBackward:
		e.wordBackward()
	case actionWordEnd:
		e.wordEnd()
	case actionGotoMode:
		e.gotoMode = true
		e.pendingKeys = "g"
		return false // Don't clear selection, wait for next key
	case actionGotoLine:
		e.gotoLastLine()
	case actionGotoFirstLine:
		e.gotoFirstLine()
	case actionGotoFileEnd:
		e.gotoFileEnd()
	case actionFindChar:
		e.setPendingFindChar(action)
		e.pendingKeys = "f"
		return false // Don't clear selection, wait for char input
	case actionFindCharBackward:
		e.setPendingFindChar(action)
		e.pendingKeys = "F"
		return false
	case actionTillChar:
		e.setPendingFindChar(action)
		e.pendingKeys = "t"
		return false
	case actionTillCharBackward:
		e.setPendingFindChar(action)
		e.pendingKeys = "T"
		return false

	// Helix-style editing
	case actionDelete:
		e.helixDelete()
	case actionChange:
		e.helixChange()
		return false // Don't clear selection (entering insert mode)
	case actionYank:
		e.yankSelection()
		return false // Don't clear selection yet (yank preserves for visual feedback)
	case actionPaste:
		e.pasteAfter()
	case actionPasteBefore:
		e.pasteBefore()
	case actionOpenBelow:
		e.openBelow()
		return false // Entering insert mode
	case actionOpenAbove:
		e.openAbove()
		return false // Entering insert mode
	case actionAppend:
		e.appendMode()
		return false // Entering insert mode
	case actionAppendLineEnd:
		e.appendLineEnd()
		return false // Entering insert mode
	case actionInsertLineStart:
		e.insertLineStart()
		return false // Entering insert mode
	case actionReplaceChar:
		e.setPendingFindChar(action)
		e.pendingKeys = "r"
		return false // Wait for char input
	case actionJoinLines:
		e.joinLinesCmd()

	// Helix-style selection
	case actionToggleSelect:
		e.toggleSelectMode()
		return false // Don't clear selection
	case actionExtendLine:
		e.extendLine()
		return false // Don't clear selection
	case actionCollapseSelection:
		e.collapseSelection()
	case actionFlipSelection:
		e.flipSelection()
		return false // Don't clear selection

	// Space mode
	case actionSpaceMode:
		e.spaceMenuActive = true
		e.pendingKeys = "SPC"
		return false

	// Match mode
	case actionMatchMode:
		e.matchMode = true
		e.pendingKeys = "m"
		return false

	// View mode
	case actionViewMode:
		e.viewMode = true
		e.pendingKeys = "z"
		return false

	// Search
	case actionSearchForward:
		e.enterSearchMode(true, false, false) // exact search
		return false
	case actionSearchBackward:
		e.enterSearchMode(false, false, false) // exact search
		return false
	case actionSearchFuzzy:
		e.enterSearchMode(true, true, false) // fuzzy search
		return false
	case actionSearchRegex:
		e.enterSearchMode(true, false, true) // regex search
		return false
	case actionSearchNext:
		e.searchNext()
	case actionSearchPrev:
		e.searchPrev()

	// Special
	case actionInsertLineAbove:
		e.insertLineAboveCursor()

	// Terminal zoom
	case actionTerminalZoomIn:
		e.sendTerminalZoom(true, 20) // zoom in 20 times
		e.zoomPendingRestore = true
		return false

	// Selection scope
	case actionExpandSelection:
		e.expandSelection()
		return false
	case actionShrinkSelection:
		e.shrinkSelection()
		return false

	// File operations
	case actionSave:
		if err := e.Save(""); err != nil {
			e.setStatus(err.Error())
		} else {
			e.setStatus("saved " + e.filename)
		}
		return false
	}
	if !e.selectMode {
		e.clearSelection()
	}
	return false
}

func (e *Editor) execCommand(cmd string) bool {
	if cmd == "" {
		return false
	}
	fields := strings.Fields(cmd)
	name := fields[0]
	args := fields[1:]

	switch name {
	case "w":
		path := ""
		if len(args) > 0 {
			path = strings.Join(args, " ")
		}
		if err := e.Save(path); err != nil {
			e.setStatus(err.Error())
			return false
		}
		e.setStatus("written")
		return false
	case "q":
		if e.dirty {
			e.setStatus("unsaved changes (use :q!)")
			return false
		}
		return true
	case "q!":
		return true
	case "wq", "x":
		path := ""
		if len(args) > 0 {
			path = strings.Join(args, " ")
		}
		if err := e.Save(path); err != nil {
			e.setStatus(err.Error())
			return false
		}
		return true
	case "ln":
		if len(args) == 0 {
			e.toggleLineNumbers()
			return false
		}
		switch strings.ToLower(args[0]) {
		case "off":
			e.lineNumberMode = LineNumberOff
			e.setStatus("line numbers off")
		case "abs", "absolute":
			e.lineNumberMode = LineNumberAbsolute
			e.setStatus("line numbers absolute")
		case "rel", "relative":
			e.lineNumberMode = LineNumberRelative
			e.setStatus("line numbers relative")
		default:
			e.setStatus("unknown line number mode")
		}
		return false
	case "fmt":
		if err := e.FormatGo(); err != nil {
			e.setStatus(err.Error())
			return false
		}
		e.setStatus("formatted")
		return false
	default:
		e.setStatus("unknown command: " + name)
		return false
	}
}

func (e *Editor) Save(path string) error {
	if path == "" {
		if e.filename == "" {
			return errors.New("no file name")
		}
		path = e.filename
	}
	data := []byte(joinLines(e.lines))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	e.filename = path
	e.savePoint = len(e.undo)
	e.updateDirty()
	_ = e.SaveUndoHistory()
	return nil
}

func (e *Editor) FormatGo() error {
	src := e.Content()
	cmd := exec.Command("gofmt")
	cmd.Stdin = strings.NewReader(src)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return errors.New(msg)
		}
		return err
	}
	formatted := out.String()
	if formatted == src {
		return nil
	}
	e.replaceBuffer(formatted, true)
	return nil
}

func (e *Editor) replaceBuffer(text string, markDirty bool) {
	lines := splitLines([]byte(text))
	if len(lines) == 0 {
		lines = [][]rune{[]rune{}}
	}
	e.lines = lines
	if e.cursor.Row >= len(e.lines) {
		e.cursor.Row = len(e.lines) - 1
		if e.cursor.Row < 0 {
			e.cursor.Row = 0
		}
	}
	e.clampCursorCol()
	if e.scroll >= len(e.lines) {
		e.scroll = len(e.lines) - 1
		if e.scroll < 0 {
			e.scroll = 0
		}
	}
	e.undo = nil
	e.redo = nil
	if markDirty {
		e.savePoint = -1
	} else {
		e.savePoint = 0
	}
	e.lastEdit.Valid = false
	e.changeTick++
	e.updateDirty()
}

func (e *Editor) Undo() {
	if len(e.undo) == 0 {
		e.setStatus("nothing to undo")
		return
	}

	// Get the group of the last action
	group := e.undo[len(e.undo)-1].group

	// Undo all actions in this group
	for len(e.undo) > 0 && e.undo[len(e.undo)-1].group == group {
		idx := len(e.undo) - 1
		act := e.undo[idx]
		e.undo = e.undo[:idx]
		inv, ok := e.applyAction(act)
		if !ok {
			e.setStatus("undo failed")
			return
		}
		inv.group = act.group
		e.redo = append(e.redo, inv)
	}
	e.changeTick++
	e.updateDirty()
	// Invalidate lastEdit to force full reparse for syntax highlighting
	e.lastEdit.Valid = false
}

func (e *Editor) Redo() {
	if len(e.redo) == 0 {
		e.setStatus("nothing to redo")
		return
	}

	// Get the group of the last action
	group := e.redo[len(e.redo)-1].group

	// Redo all actions in this group
	for len(e.redo) > 0 && e.redo[len(e.redo)-1].group == group {
		idx := len(e.redo) - 1
		act := e.redo[idx]
		e.redo = e.redo[:idx]
		inv, ok := e.applyAction(act)
		if !ok {
			e.setStatus("redo failed")
			return
		}
		inv.group = act.group
		e.undo = append(e.undo, inv)
	}
	e.changeTick++
	e.updateDirty()
	// Invalidate lastEdit to force full reparse for syntax highlighting
	e.lastEdit.Valid = false
}

func (e *Editor) applyAction(act action) (action, bool) {
	switch act.kind {
	case actionInsertRune:
		if !e.insertRuneAt(act.pos, act.r) {
			return action{}, false
		}
		return action{kind: actionDeleteRune, pos: act.pos, r: act.r}, true
	case actionDeleteRune:
		if !e.deleteRuneAt(act.pos) {
			return action{}, false
		}
		return action{kind: actionInsertRune, pos: act.pos, r: act.r}, true
	case actionSplitLine:
		if !e.splitLineAt(act.pos) {
			return action{}, false
		}
		return action{kind: actionJoinLine, pos: act.pos}, true
	case actionJoinLine:
		if !e.joinLineAt(act.pos) {
			return action{}, false
		}
		return action{kind: actionSplitLine, pos: act.pos}, true
	case actionMoveLine:
		if !e.swapLines(act.rowFrom, act.rowTo) {
			return action{}, false
		}
		e.cursor.Row = act.rowTo
		if e.cursor.Row < 0 {
			e.cursor.Row = 0
		}
		if e.cursor.Row >= len(e.lines) {
			e.cursor.Row = len(e.lines) - 1
		}
		e.clampCursorCol()
		return action{kind: actionMoveLine, rowFrom: act.rowTo, rowTo: act.rowFrom}, true
	case actionInsertText:
		endPos := e.insertTextAt(act.pos, act.text)
		// Restore selection if this action has one
		if act.hasSelection {
			e.selectionActive = true
			e.selectionStart = act.selectionStart
			e.selectionEnd = act.selectionEnd
		}
		// Preserve selection info for redo→undo cycle
		return action{
			kind:           actionDeleteText,
			pos:            act.pos,
			endPos:         endPos,
			text:           act.text,
			selectionStart: act.selectionStart,
			selectionEnd:   act.selectionEnd,
			hasSelection:   act.hasSelection,
		}, true
	case actionDeleteText:
		deleted := e.deleteTextRange(act.pos, act.endPos)
		// Clear selection - after delete there's no selection
		e.selectionActive = false
		// Preserve selection info for undo cycle
		return action{
			kind:           actionInsertText,
			pos:            act.pos,
			text:           deleted,
			selectionStart: act.selectionStart,
			selectionEnd:   act.selectionEnd,
			hasSelection:   act.hasSelection,
		}, true
	default:
		return action{}, false
	}
}

func (e *Editor) recordUndo(act action) {
	e.undoGroup++
	act.group = e.undoGroup
	e.undo = append(e.undo, act)
	e.redo = e.redo[:0]
	e.changeTick++
	e.updateDirty()
}

// startUndoGroup starts a new undo group. All subsequent appendUndo calls will use this group.
// Call this before a series of appendUndo calls, then call finishUndoGroup at the end.
func (e *Editor) startUndoGroup() {
	e.undoGroup++
}

// appendUndo adds an action to undo stack with the current group.
// Use this when recording multiple actions as part of a single logical operation.
func (e *Editor) appendUndo(act action) {
	act.group = e.undoGroup
	e.undo = append(e.undo, act)
}

// finishUndoGroup clears redo and updates state after a group of undo actions.
func (e *Editor) finishUndoGroup() {
	e.redo = e.redo[:0]
	e.changeTick++
	e.updateDirty()
}

func (e *Editor) updateDirty() {
	e.dirty = len(e.undo) != e.savePoint
}

// changelogFilePath returns the path for the changelog file for the given file path.
// Format: ~/.config/qedit/changelog/<encoded-path>.log
func changelogFilePath(filePath string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".config", "qedit", "changelog")

	// Get absolute path and encode it
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	// Replace path separators with underscores and other special chars
	encoded := strings.ReplaceAll(absPath, string(filepath.Separator), "_")
	encoded = strings.ReplaceAll(encoded, ":", "_")
	encoded = strings.ReplaceAll(encoded, " ", "_")

	return filepath.Join(dir, encoded+".log")
}

// actionToJSON converts an action to its JSON-serializable form
func actionToJSON(a action) actionJSON {
	var textStrings []string
	if len(a.text) > 0 {
		textStrings = make([]string, len(a.text))
		for i, line := range a.text {
			textStrings[i] = string(line)
		}
	}
	return actionJSON{
		Kind:           int(a.kind),
		PosRow:         a.pos.Row,
		PosCol:         a.pos.Col,
		R:              a.r,
		RowFrom:        a.rowFrom,
		RowTo:          a.rowTo,
		Group:          a.group,
		Text:           textStrings,
		EndPosRow:      a.endPos.Row,
		EndPosCol:      a.endPos.Col,
		SelectionStart: [2]int{a.selectionStart.Row, a.selectionStart.Col},
		SelectionEnd:   [2]int{a.selectionEnd.Row, a.selectionEnd.Col},
		HasSelection:   a.hasSelection,
	}
}

// jsonToAction converts a JSON-serializable action back to an action
func jsonToAction(j actionJSON) action {
	var text [][]rune
	if len(j.Text) > 0 {
		text = make([][]rune, len(j.Text))
		for i, s := range j.Text {
			text[i] = []rune(s)
		}
	}
	return action{
		kind:           actionKind(j.Kind),
		pos:            Cursor{Row: j.PosRow, Col: j.PosCol},
		r:              j.R,
		rowFrom:        j.RowFrom,
		rowTo:          j.RowTo,
		group:          j.Group,
		text:           text,
		endPos:         Cursor{Row: j.EndPosRow, Col: j.EndPosCol},
		selectionStart: Cursor{Row: j.SelectionStart[0], Col: j.SelectionStart[1]},
		selectionEnd:   Cursor{Row: j.SelectionEnd[0], Col: j.SelectionEnd[1]},
		hasSelection:   j.HasSelection,
	}
}

// SaveUndoHistory saves the undo history to the changelog file
func (e *Editor) SaveUndoHistory() error {
	if e.filename == "" {
		return nil // No file path, nothing to save
	}

	logPath := changelogFilePath(e.filename)
	if logPath == "" {
		return nil
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Open file for writing
	f, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	encoder := json.NewEncoder(writer)

	// Write each action as a JSON line
	for _, a := range e.undo {
		if err := encoder.Encode(actionToJSON(a)); err != nil {
			return err
		}
	}

	return writer.Flush()
}

// LoadUndoHistory loads the undo history from the changelog file
func (e *Editor) LoadUndoHistory() error {
	if e.filename == "" {
		return nil
	}

	logPath := changelogFilePath(e.filename)
	if logPath == "" {
		return nil
	}

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No history file, that's ok
		}
		return err
	}
	defer f.Close()

	e.undo = nil
	scanner := bufio.NewScanner(f)
	// Increase buffer size for large actions
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		var j actionJSON
		if err := json.Unmarshal(scanner.Bytes(), &j); err != nil {
			continue // Skip malformed lines
		}
		e.undo = append(e.undo, jsonToAction(j))
	}

	return scanner.Err()
}

// ClearUndoHistory removes the changelog file for the current file
func (e *Editor) ClearUndoHistory() error {
	if e.filename == "" {
		return nil
	}

	logPath := changelogFilePath(e.filename)
	if logPath == "" {
		return nil
	}

	err := os.Remove(logPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (e *Editor) setStatus(msg string) {
	e.statusMessage = msg
}

// sendTerminalZoom sends zoom commands to the terminal via AppleScript.
// zoomIn=true sends Cmd++, zoomIn=false sends Cmd+-.
func (e *Editor) sendTerminalZoom(zoomIn bool, times int) {
	key := "+"
	if !zoomIn {
		key = "-"
	}

	script := fmt.Sprintf(`
		tell application "System Events"
			repeat %d times
				keystroke "%s" using command down
			end repeat
		end tell
	`, times, key)

	_ = exec.Command("osascript", "-e", script).Run()
}

func (e *Editor) insertRune(r rune) {
	pos := e.cursor
	line := e.lines[pos.Row]
	if pos.Col > len(line) {
		pos.Col = len(line)
	}
	if !e.insertRuneAt(pos, r) {
		return
	}
	e.recordUndo(action{kind: actionDeleteRune, pos: pos, r: r})
}

func (e *Editor) insertTab() {
	e.insertRune('\t')
}

func (e *Editor) insertRuneAt(pos Cursor, r rune) bool {
	if pos.Row < 0 || pos.Row >= len(e.lines) {
		return false
	}
	line := e.lines[pos.Row]
	if pos.Col < 0 {
		pos.Col = 0
	}
	if pos.Col > len(line) {
		pos.Col = len(line)
	}
	e.recordTextEdit(pos, pos, Cursor{Row: pos.Row, Col: pos.Col + 1}, runeByteLen(r))
	line = append(line, 0)
	copy(line[pos.Col+1:], line[pos.Col:])
	line[pos.Col] = r
	e.lines[pos.Row] = line
	e.cursor = Cursor{Row: pos.Row, Col: pos.Col + 1}
	return true
}

// insertTextAt inserts multiple lines at the given position and returns the end position.
// This is a bulk operation for efficiency with large text blocks.
func (e *Editor) insertTextAt(pos Cursor, text [][]rune) Cursor {
	if len(text) == 0 || pos.Row < 0 || pos.Row >= len(e.lines) {
		return pos
	}

	if len(text) == 1 {
		// Single line: just insert the runes into the current line
		line := e.lines[pos.Row]
		if pos.Col < 0 {
			pos.Col = 0
		}
		if pos.Col > len(line) {
			pos.Col = len(line)
		}
		newLine := make([]rune, 0, len(line)+len(text[0]))
		newLine = append(newLine, line[:pos.Col]...)
		newLine = append(newLine, text[0]...)
		newLine = append(newLine, line[pos.Col:]...)
		e.lines[pos.Row] = newLine
		return Cursor{Row: pos.Row, Col: pos.Col + len(text[0])}
	}

	// Multi-line insertion
	line := e.lines[pos.Row]
	if pos.Col < 0 {
		pos.Col = 0
	}
	if pos.Col > len(line) {
		pos.Col = len(line)
	}

	// First line: prefix from original + first inserted line
	firstLine := make([]rune, 0, pos.Col+len(text[0]))
	firstLine = append(firstLine, line[:pos.Col]...)
	firstLine = append(firstLine, text[0]...)

	// Last line: last inserted line + suffix from original
	suffix := line[pos.Col:]
	lastLine := make([]rune, 0, len(text[len(text)-1])+len(suffix))
	lastLine = append(lastLine, text[len(text)-1]...)
	lastLine = append(lastLine, suffix...)

	// Build new lines slice
	newLines := make([][]rune, 0, len(e.lines)+len(text)-1)
	newLines = append(newLines, e.lines[:pos.Row]...)
	newLines = append(newLines, firstLine)
	for i := 1; i < len(text)-1; i++ {
		newLines = append(newLines, text[i])
	}
	newLines = append(newLines, lastLine)
	newLines = append(newLines, e.lines[pos.Row+1:]...)
	e.lines = newLines

	return Cursor{Row: pos.Row + len(text) - 1, Col: len(text[len(text)-1])}
}

// deleteTextRange deletes text from start to end position and returns the deleted text.
// This is a bulk operation for efficiency with large text blocks.
func (e *Editor) deleteTextRange(start, end Cursor) [][]rune {
	if start.Row < 0 || end.Row >= len(e.lines) || start.Row > end.Row {
		return nil
	}
	if start.Row == end.Row && start.Col >= end.Col {
		return nil
	}

	if start.Row == end.Row {
		// Single line deletion
		line := e.lines[start.Row]
		if start.Col < 0 {
			start.Col = 0
		}
		if end.Col > len(line) {
			end.Col = len(line)
		}
		deleted := make([]rune, end.Col-start.Col)
		copy(deleted, line[start.Col:end.Col])
		newLine := make([]rune, 0, len(line)-(end.Col-start.Col))
		newLine = append(newLine, line[:start.Col]...)
		newLine = append(newLine, line[end.Col:]...)
		e.lines[start.Row] = newLine
		e.cursor = start
		return [][]rune{deleted}
	}

	// Multi-line deletion
	// Collect deleted text
	deleted := make([][]rune, end.Row-start.Row+1)

	// First line partial
	firstLine := e.lines[start.Row]
	if start.Col < 0 {
		start.Col = 0
	}
	if start.Col > len(firstLine) {
		start.Col = len(firstLine)
	}
	deleted[0] = make([]rune, len(firstLine)-start.Col)
	copy(deleted[0], firstLine[start.Col:])

	// Middle lines (complete)
	for i := start.Row + 1; i < end.Row; i++ {
		deleted[i-start.Row] = make([]rune, len(e.lines[i]))
		copy(deleted[i-start.Row], e.lines[i])
	}

	// Last line partial
	lastLine := e.lines[end.Row]
	if end.Col < 0 {
		end.Col = 0
	}
	if end.Col > len(lastLine) {
		end.Col = len(lastLine)
	}
	deleted[len(deleted)-1] = make([]rune, end.Col)
	copy(deleted[len(deleted)-1], lastLine[:end.Col])

	// Merge first and last lines
	mergedLine := make([]rune, 0, start.Col+len(lastLine)-end.Col)
	mergedLine = append(mergedLine, firstLine[:start.Col]...)
	mergedLine = append(mergedLine, lastLine[end.Col:]...)

	// Build new lines slice
	newLines := make([][]rune, 0, len(e.lines)-(end.Row-start.Row))
	newLines = append(newLines, e.lines[:start.Row]...)
	newLines = append(newLines, mergedLine)
	newLines = append(newLines, e.lines[end.Row+1:]...)
	e.lines = newLines

	e.cursor = start
	return deleted
}

func (e *Editor) insertNewline() {
	pos := e.cursor
	line := e.lines[pos.Row]
	if pos.Col > len(line) {
		pos.Col = len(line)
	}
	if !e.splitLineAt(pos) {
		return
	}
	e.recordUndo(action{kind: actionJoinLine, pos: pos})
}

func (e *Editor) splitLineAt(pos Cursor) bool {
	if pos.Row < 0 || pos.Row >= len(e.lines) {
		return false
	}
	line := e.lines[pos.Row]
	if pos.Col < 0 {
		pos.Col = 0
	}
	if pos.Col > len(line) {
		pos.Col = len(line)
	}
	e.recordTextEdit(pos, pos, Cursor{Row: pos.Row + 1, Col: 0}, 1)
	left := append([]rune(nil), line[:pos.Col]...)
	right := append([]rune(nil), line[pos.Col:]...)

	newLines := make([][]rune, 0, len(e.lines)+1)
	newLines = append(newLines, e.lines[:pos.Row]...)
	newLines = append(newLines, left, right)
	newLines = append(newLines, e.lines[pos.Row+1:]...)
	e.lines = newLines

	e.cursor = Cursor{Row: pos.Row + 1, Col: 0}
	return true
}

func (e *Editor) backspace() {
	if e.cursor.Col > 0 {
		pos := Cursor{Row: e.cursor.Row, Col: e.cursor.Col - 1}
		line := e.lines[pos.Row]
		if pos.Col >= len(line) {
			pos.Col = len(line) - 1
		}
		if pos.Col < 0 {
			return
		}
		r := line[pos.Col]
		if !e.deleteRuneAt(pos) {
			return
		}
		e.recordUndo(action{kind: actionInsertRune, pos: pos, r: r})
		return
	}
	if e.cursor.Row == 0 {
		return
	}
	pos := Cursor{Row: e.cursor.Row - 1, Col: len(e.lines[e.cursor.Row-1])}
	if !e.joinLineAt(pos) {
		return
	}
	e.recordUndo(action{kind: actionSplitLine, pos: pos})
}

func (e *Editor) deleteLine() {
	// If there's a selection, delete the selected text (same as 'd' key)
	if start, end, ok := e.selectionRange(); ok {
		e.deleteSelection(start, end, true) // Restore selection on undo
		e.clearSelection()
		e.selectMode = false
		return
	}

	if len(e.lines) == 0 {
		return
	}
	row := e.cursor.Row
	if row < 0 || row >= len(e.lines) {
		return
	}

	line := e.lines[row]

	if len(e.lines) == 1 {
		// Only one line - just clear it
		if len(line) == 0 {
			return
		}
		// Use deleteSelection for consistency, no selection restore
		e.deleteSelection(Cursor{Row: 0, Col: 0}, Cursor{Row: 0, Col: len(line)}, false)
		return
	}

	// Delete entire line including newline using deleteSelection
	var start, end Cursor
	if row < len(e.lines)-1 {
		// Not the last line: select from start of this line to start of next
		start = Cursor{Row: row, Col: 0}
		end = Cursor{Row: row + 1, Col: 0}
	} else {
		// Last line: select from end of previous line to end of this line
		start = Cursor{Row: row - 1, Col: len(e.lines[row-1])}
		end = Cursor{Row: row, Col: len(line)}
	}

	e.deleteSelection(start, end, false) // No selection restore for line delete

	// Adjust cursor position
	if e.cursor.Row >= len(e.lines) {
		e.cursor.Row = len(e.lines) - 1
		if e.cursor.Row < 0 {
			e.cursor.Row = 0
		}
	}
	e.cursor.Col = 0
	e.clampCursorCol()
}

func (e *Editor) deleteChar() {
	// If there's a selection, delete the selected text
	if start, end, ok := e.selectionRange(); ok {
		e.deleteSelection(start, end, true) // Restore selection on undo
		return
	}

	// No selection - delete character to the right
	row := e.cursor.Row
	col := e.cursor.Col
	if row < 0 || row >= len(e.lines) {
		return
	}
	line := e.lines[row]

	if col < len(line) {
		// Delete character at cursor position
		pos := Cursor{Row: row, Col: col}
		r := line[col]
		if e.deleteRuneAt(pos) {
			e.recordUndo(action{kind: actionInsertRune, pos: pos, r: r})
		}
	} else if row < len(e.lines)-1 {
		// At end of line, join with next line
		pos := Cursor{Row: row, Col: len(line)}
		if e.joinLineAt(pos) {
			e.recordUndo(action{kind: actionSplitLine, pos: pos})
		}
	}
}

func (e *Editor) deleteSelection(start, end Cursor, restoreSelectionOnUndo bool) {
	if start.Row < 0 || end.Row >= len(e.lines) {
		return
	}

	// Calculate byte offsets BEFORE making changes
	startByte, startColBytes := e.byteOffset(start)
	oldEndByte, oldEndColBytes := e.byteOffset(end)

	// Collect deleted content for undo
	// Use bulk operation for efficiency with large selections
	deleted := e.collectDeletedText(start, end)

	e.startUndoGroup()
	// Record as a single bulk insert action for undo
	e.appendUndo(action{
		kind:           actionInsertText,
		pos:            start,
		text:           deleted,
		selectionStart: start,
		selectionEnd:   end,
		hasSelection:   restoreSelectionOnUndo,
	})
	e.finishUndoGroup()

	// Record text edit for tree-sitter
	e.lastEdit = TextEdit{
		Valid:          true,
		StartByte:      startByte,
		OldEndByte:     oldEndByte,
		NewEndByte:     startByte, // Nothing inserted
		StartRow:       start.Row,
		StartColBytes:  startColBytes,
		OldEndRow:      end.Row,
		OldEndColBytes: oldEndColBytes,
		NewEndRow:      start.Row,
		NewEndColBytes: startColBytes,
	}

	// Actually delete the selection
	if start.Row == end.Row {
		// Single line deletion
		line := e.lines[start.Row]
		newLine := append([]rune(nil), line[:start.Col]...)
		newLine = append(newLine, line[end.Col:]...)
		e.lines[start.Row] = newLine
	} else {
		// Multi-line deletion
		firstLine := e.lines[start.Row][:start.Col]
		lastLine := e.lines[end.Row][end.Col:]
		mergedLine := append([]rune(nil), firstLine...)
		mergedLine = append(mergedLine, lastLine...)

		newLines := make([][]rune, 0, len(e.lines)-(end.Row-start.Row))
		newLines = append(newLines, e.lines[:start.Row]...)
		newLines = append(newLines, mergedLine)
		newLines = append(newLines, e.lines[end.Row+1:]...)
		e.lines = newLines
	}

	e.cursor = start
	e.clearSelection()
	e.changeTick++
	e.updateDirty()
}

// collectDeletedText collects text from start to end position without modifying the buffer.
func (e *Editor) collectDeletedText(start, end Cursor) [][]rune {
	if start.Row == end.Row {
		// Single line
		line := e.lines[start.Row]
		deleted := make([]rune, end.Col-start.Col)
		copy(deleted, line[start.Col:end.Col])
		return [][]rune{deleted}
	}

	// Multi-line
	deleted := make([][]rune, end.Row-start.Row+1)

	// First line partial
	firstLine := e.lines[start.Row]
	deleted[0] = make([]rune, len(firstLine)-start.Col)
	copy(deleted[0], firstLine[start.Col:])

	// Middle lines (complete)
	for i := start.Row + 1; i < end.Row; i++ {
		deleted[i-start.Row] = make([]rune, len(e.lines[i]))
		copy(deleted[i-start.Row], e.lines[i])
	}

	// Last line partial
	lastLine := e.lines[end.Row]
	deleted[len(deleted)-1] = make([]rune, end.Col)
	copy(deleted[len(deleted)-1], lastLine[:end.Col])

	return deleted
}

func (e *Editor) deleteWordLeft() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}

	if e.cursor.Col == 0 {
		// At start of line - join with previous line
		if e.cursor.Row > 0 {
			// Calculate byte offset BEFORE change
			pos := Cursor{Row: e.cursor.Row - 1, Col: len(e.lines[e.cursor.Row-1])}
			startByte, startColBytes := e.byteOffset(pos)
			oldEndByte := startByte + 1 // +1 for newline

			e.lastEdit = TextEdit{
				Valid:          true,
				StartByte:      startByte,
				OldEndByte:     oldEndByte,
				NewEndByte:     startByte,
				StartRow:       pos.Row,
				StartColBytes:  startColBytes,
				OldEndRow:      e.cursor.Row,
				OldEndColBytes: 0,
				NewEndRow:      pos.Row,
				NewEndColBytes: startColBytes,
			}

			if e.joinLineAt(pos) {
				e.recordUndo(action{kind: actionSplitLine, pos: pos})
			}
		}
		return
	}

	line := e.lines[e.cursor.Row]
	endCol := e.cursor.Col
	idx := endCol - 1

	if idx >= len(line) {
		idx = len(line) - 1
	}
	if idx < 0 {
		return
	}

	// Skip spaces first
	for idx > 0 && isSpaceRune(line[idx]) {
		idx--
	}

	// Then skip word characters or non-word/non-space characters
	if idx >= 0 && isWordRune(line[idx]) {
		for idx > 0 && isWordRune(line[idx-1]) {
			idx--
		}
	} else if idx >= 0 && !isSpaceRune(line[idx]) {
		for idx > 0 && !isSpaceRune(line[idx-1]) && !isWordRune(line[idx-1]) {
			idx--
		}
	}

	startCol := idx
	if startCol >= endCol {
		return
	}

	// Calculate byte offsets BEFORE making changes
	startByte, startColBytes := e.byteOffset(Cursor{Row: e.cursor.Row, Col: startCol})
	oldEndByte, oldEndColBytes := e.byteOffset(Cursor{Row: e.cursor.Row, Col: endCol})

	// Record text edit for tree-sitter
	e.lastEdit = TextEdit{
		Valid:          true,
		StartByte:      startByte,
		OldEndByte:     oldEndByte,
		NewEndByte:     startByte,
		StartRow:       e.cursor.Row,
		StartColBytes:  startColBytes,
		OldEndRow:      e.cursor.Row,
		OldEndColBytes: oldEndColBytes,
		NewEndRow:      e.cursor.Row,
		NewEndColBytes: startColBytes,
	}

	// Record undo for each character (backwards) as a group
	e.startUndoGroup()
	for col := endCol - 1; col >= startCol; col-- {
		if col >= 0 && col < len(line) {
			e.appendUndo(action{kind: actionInsertRune, pos: Cursor{Row: e.cursor.Row, Col: col}, r: line[col]})
		}
	}
	e.finishUndoGroup()

	// Actually delete the range
	newLine := append([]rune(nil), line[:startCol]...)
	newLine = append(newLine, line[endCol:]...)
	e.lines[e.cursor.Row] = newLine

	e.cursor.Col = startCol
}

func (e *Editor) insertLineBelow() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}

	// Get current line's indentation
	line := e.lines[e.cursor.Row]
	indent := make([]rune, 0)
	for _, r := range line {
		if r == ' ' || r == '\t' {
			indent = append(indent, r)
		} else {
			break
		}
	}

	// Move cursor to end of line
	e.cursor.Col = len(line)

	// Split line (creates new line below) and insert indentation as a group
	e.startUndoGroup()
	pos := e.cursor
	if !e.splitLineAt(pos) {
		return
	}
	e.appendUndo(action{kind: actionJoinLine, pos: pos})

	// Insert indentation on the new line
	for _, r := range indent {
		insertPos := e.cursor
		if e.insertRuneAt(insertPos, r) {
			e.appendUndo(action{kind: actionDeleteRune, pos: insertPos, r: r})
		}
	}
	e.finishUndoGroup()
}

func (e *Editor) indentSelection() {
	start, end, ok := e.selectionRange()
	if !ok {
		// No selection - behavior depends on mode
		if e.mode == ModeNormal {
			// In Normal mode, indent the current line (tab at beginning)
			e.indentCurrentLine()
		} else {
			// In Insert mode, insert tab at cursor position
			e.insertTab()
		}
		return
	}

	// Calculate actual end row - if end.Col == 0, don't include that row
	endRow := end.Row
	if end.Col == 0 && end.Row > start.Row {
		endRow = end.Row - 1
	}

	// Indent all lines in selection as a group
	e.startUndoGroup()
	for row := start.Row; row <= endRow; row++ {
		if row < 0 || row >= len(e.lines) {
			continue
		}
		// Insert tab at beginning of line
		line := e.lines[row]
		newLine := make([]rune, len(line)+1)
		newLine[0] = '\t'
		copy(newLine[1:], line)
		e.lines[row] = newLine
		e.appendUndo(action{kind: actionDeleteRune, pos: Cursor{Row: row, Col: 0}, r: '\t'})
	}
	e.finishUndoGroup()
	e.lastEdit.Valid = false

	// Adjust cursor and selection columns - they shift by 1 for affected lines
	if e.cursor.Row >= start.Row && e.cursor.Row <= endRow {
		e.cursor.Col++
	}
	if e.selectionStart.Row >= start.Row && e.selectionStart.Row <= endRow {
		e.selectionStart.Col++
	}
	if e.selectionEnd.Row >= start.Row && e.selectionEnd.Row <= endRow && end.Col > 0 {
		e.selectionEnd.Col++
	}
}

// indentCurrentLine adds a tab at the beginning of the current line (for Normal mode)
func (e *Editor) indentCurrentLine() {
	row := e.cursor.Row
	if row < 0 || row >= len(e.lines) {
		return
	}
	line := e.lines[row]
	newLine := make([]rune, len(line)+1)
	newLine[0] = '\t'
	copy(newLine[1:], line)
	e.lines[row] = newLine
	e.recordUndo(action{kind: actionDeleteRune, pos: Cursor{Row: row, Col: 0}, r: '\t'})
	e.cursor.Col++
	e.lastEdit.Valid = false
}

func (e *Editor) unindentSelection() {
	start, end, hasSelection := e.selectionRange()
	if !hasSelection {
		// No selection - unindent current line only
		start = e.cursor
		end = e.cursor
	}

	// Calculate actual end row - if end.Col == 0, don't include that row
	endRow := end.Row
	if hasSelection && end.Col == 0 && end.Row > start.Row {
		endRow = end.Row - 1
	}

	// Track how many chars removed from each relevant line
	cursorLineRemoved := 0
	startLineRemoved := 0
	endLineRemoved := 0

	// Unindent all lines in selection as a group
	e.startUndoGroup()
	for row := start.Row; row <= endRow; row++ {
		if row < 0 || row >= len(e.lines) {
			continue
		}
		line := e.lines[row]
		if len(line) == 0 {
			continue
		}

		removed := 0
		// Remove leading tab or spaces (up to tabWidth)
		if line[0] == '\t' {
			e.appendUndo(action{kind: actionInsertRune, pos: Cursor{Row: row, Col: 0}, r: '\t'})
			e.lines[row] = line[1:]
			removed = 1
		} else if line[0] == ' ' {
			// Count spaces to remove (up to tabWidth)
			for i := 0; i < e.tabWidth && i < len(line) && line[i] == ' '; i++ {
				removed++
			}
			// Record undo for each space (backwards)
			for i := removed - 1; i >= 0; i-- {
				e.appendUndo(action{kind: actionInsertRune, pos: Cursor{Row: row, Col: i}, r: ' '})
			}
			e.lines[row] = line[removed:]
		}

		if row == e.cursor.Row {
			cursorLineRemoved = removed
		}
		if row == e.selectionStart.Row {
			startLineRemoved = removed
		}
		if row == e.selectionEnd.Row {
			endLineRemoved = removed
		}
	}
	e.finishUndoGroup()
	e.lastEdit.Valid = false

	// Adjust cursor column
	if cursorLineRemoved > 0 {
		e.cursor.Col -= cursorLineRemoved
		if e.cursor.Col < 0 {
			e.cursor.Col = 0
		}
	}

	// Adjust selection columns if there was a selection
	if hasSelection {
		if startLineRemoved > 0 {
			e.selectionStart.Col -= startLineRemoved
			if e.selectionStart.Col < 0 {
				e.selectionStart.Col = 0
			}
		}
		// Only adjust selectionEnd.Col if it's on an affected line
		if endLineRemoved > 0 && end.Col > 0 {
			e.selectionEnd.Col -= endLineRemoved
			if e.selectionEnd.Col < 0 {
				e.selectionEnd.Col = 0
			}
		}
	}
}

func (e *Editor) saveLineState() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		e.lineUndoValid = false
		return
	}
	if e.lineUndoValid && e.lineUndoRow == e.cursor.Row {
		return // Already tracking this line
	}
	e.lineUndoRow = e.cursor.Row
	e.lineUndoContent = append([]rune(nil), e.lines[e.cursor.Row]...)
	e.lineUndoValid = true
}

func (e *Editor) undoLine() {
	if !e.lineUndoValid {
		e.setStatus("no line changes to undo")
		return
	}
	row := e.lineUndoRow
	if row < 0 || row >= len(e.lines) {
		e.setStatus("line no longer exists")
		e.lineUndoValid = false
		return
	}

	currentLine := e.lines[row]
	originalLine := e.lineUndoContent

	// If line hasn't changed, nothing to do
	if string(currentLine) == string(originalLine) {
		e.setStatus("no changes on this line")
		return
	}

	e.startUndoGroup()

	// Delete current line content (backwards for proper undo)
	for i := len(currentLine) - 1; i >= 0; i-- {
		pos := Cursor{Row: row, Col: i}
		r := currentLine[i]
		if e.deleteRuneAt(pos) {
			e.appendUndo(action{kind: actionInsertRune, pos: pos, r: r})
		}
	}

	// Insert original content
	for i, r := range originalLine {
		pos := Cursor{Row: row, Col: i}
		if e.insertRuneAt(pos, r) {
			e.appendUndo(action{kind: actionDeleteRune, pos: pos, r: r})
		}
	}

	e.finishUndoGroup()

	// Position cursor at start of line
	e.cursor.Row = row
	e.cursor.Col = 0
	if e.cursor.Col > len(e.lines[row]) {
		e.cursor.Col = len(e.lines[row])
	}

	// Invalidate line undo since we've restored it
	e.lineUndoValid = false
	e.setStatus("line restored")
}

func (e *Editor) deleteRuneAt(pos Cursor) bool {
	if pos.Row < 0 || pos.Row >= len(e.lines) {
		return false
	}
	line := e.lines[pos.Row]
	if pos.Col < 0 || pos.Col >= len(line) {
		return false
	}
	e.recordTextEdit(pos, Cursor{Row: pos.Row, Col: pos.Col + 1}, pos, 0)
	copy(line[pos.Col:], line[pos.Col+1:])
	line = line[:len(line)-1]
	e.lines[pos.Row] = line
	e.cursor = Cursor{Row: pos.Row, Col: pos.Col}
	return true
}

func (e *Editor) joinLineAt(pos Cursor) bool {
	if pos.Row < 0 || pos.Row+1 >= len(e.lines) {
		return false
	}
	left := e.lines[pos.Row]
	right := e.lines[pos.Row+1]
	if pos.Col < 0 {
		pos.Col = 0
	}
	if pos.Col > len(left) {
		pos.Col = len(left)
	}
	e.recordTextEdit(pos, Cursor{Row: pos.Row + 1, Col: 0}, pos, 0)
	merged := append(left, right...)

	newLines := make([][]rune, 0, len(e.lines)-1)
	newLines = append(newLines, e.lines[:pos.Row]...)
	newLines = append(newLines, merged)
	newLines = append(newLines, e.lines[pos.Row+2:]...)
	e.lines = newLines

	e.cursor = Cursor{Row: pos.Row, Col: pos.Col}
	return true
}

func (e *Editor) moveLeft() {
	if e.cursor.Col > 0 {
		e.cursor.Col--
		return
	}
	if e.cursor.Row == 0 {
		return
	}
	e.cursor.Row--
	e.cursor.Col = len(e.lines[e.cursor.Row])
}

func (e *Editor) moveRight() {
	lineLen := len(e.lines[e.cursor.Row])
	if e.cursor.Col < lineLen {
		e.cursor.Col++
		return
	}
	if e.cursor.Row >= len(e.lines)-1 {
		return
	}
	e.cursor.Row++
	e.cursor.Col = 0
}

func (e *Editor) moveUp() {
	if e.cursor.Row == 0 {
		return
	}
	e.cursor.Row--
	e.clampCursorCol()
	if e.mode == ModeInsert {
		e.saveLineState()
	}
}

func (e *Editor) moveDown() {
	if e.cursor.Row >= len(e.lines)-1 {
		return
	}
	e.cursor.Row++
	e.clampCursorCol()
	if e.mode == ModeInsert {
		e.saveLineState()
	}
}

func (e *Editor) moveWordLeft() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}
	if e.cursor.Col <= 0 {
		if e.cursor.Row == 0 {
			return
		}
		e.cursor.Row--
		e.cursor.Col = len(e.lines[e.cursor.Row])
		return
	}
	line := e.lines[e.cursor.Row]
	idx := e.cursor.Col - 1
	if idx >= len(line) {
		idx = len(line) - 1
	}
	for idx > 0 && isSpaceRune(line[idx]) {
		idx--
	}
	if idx < 0 {
		e.cursor.Col = 0
		return
	}
	if isWordRune(line[idx]) {
		for idx > 0 && isWordRune(line[idx-1]) {
			idx--
		}
		e.cursor.Col = idx
		return
	}
	for idx > 0 && !isSpaceRune(line[idx-1]) && !isWordRune(line[idx-1]) {
		idx--
	}
	e.cursor.Col = idx
}

func (e *Editor) moveWordRight() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}
	line := e.lines[e.cursor.Row]
	if e.cursor.Col >= len(line) {
		if e.cursor.Row >= len(e.lines)-1 {
			return
		}
		e.cursor.Row++
		e.cursor.Col = 0
		return
	}
	idx := e.cursor.Col
	if idx < 0 {
		idx = 0
	}
	if idx >= len(line) {
		e.cursor.Col = len(line)
		return
	}
	if isSpaceRune(line[idx]) {
		for idx < len(line) && isSpaceRune(line[idx]) {
			idx++
		}
		e.cursor.Col = idx
		return
	}
	if isWordRune(line[idx]) {
		for idx < len(line) && isWordRune(line[idx]) {
			idx++
		}
	} else {
		for idx < len(line) && !isSpaceRune(line[idx]) && !isWordRune(line[idx]) {
			idx++
		}
	}
	for idx < len(line) && isSpaceRune(line[idx]) {
		idx++
	}
	e.cursor.Col = idx
}

func (e *Editor) moveLineStart() {
	e.cursor.Col = 0
}

func (e *Editor) moveLineEnd() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		e.cursor.Col = 0
		return
	}
	e.cursor.Col = len(e.lines[e.cursor.Row])
}

func (e *Editor) moveFileStart() {
	prevRow := e.cursor.Row
	e.cursor.Row = 0
	e.cursor.Col = 0
	if e.mode == ModeInsert && e.cursor.Row != prevRow {
		e.saveLineState()
	}
}

func (e *Editor) moveFileEnd() {
	if len(e.lines) == 0 {
		e.cursor.Row = 0
		e.cursor.Col = 0
		return
	}
	prevRow := e.cursor.Row
	e.cursor.Row = len(e.lines) - 1
	e.cursor.Col = len(e.lines[e.cursor.Row])
	if e.mode == ModeInsert && e.cursor.Row != prevRow {
		e.saveLineState()
	}
}

func (e *Editor) moveLineUp() {
	if e.cursor.Row <= 0 || e.cursor.Row >= len(e.lines) {
		return
	}
	from := e.cursor.Row
	to := e.cursor.Row - 1
	if !e.swapLines(from, to) {
		return
	}
	e.cursor.Row = to
	e.recordUndo(action{kind: actionMoveLine, rowFrom: from, rowTo: to})
	if e.mode == ModeInsert {
		e.saveLineState()
	}
}

func (e *Editor) moveLineDown() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines)-1 {
		return
	}
	from := e.cursor.Row
	to := e.cursor.Row + 1
	if !e.swapLines(from, to) {
		return
	}
	e.cursor.Row = to
	e.recordUndo(action{kind: actionMoveLine, rowFrom: from, rowTo: to})
	if e.mode == ModeInsert {
		e.saveLineState()
	}
}

func (e *Editor) pageUp() {
	height := e.viewHeightCached()
	if height < 1 {
		height = 1
	}
	prevRow := e.cursor.Row
	e.cursor.Row -= height
	if e.cursor.Row < 0 {
		e.cursor.Row = 0
	}
	e.clampCursorCol()
	if e.mode == ModeInsert && e.cursor.Row != prevRow {
		e.saveLineState()
	}
}

func (e *Editor) pageDown() {
	height := e.viewHeightCached()
	if height < 1 {
		height = 1
	}
	prevRow := e.cursor.Row
	e.cursor.Row += height
	if e.cursor.Row >= len(e.lines) {
		e.cursor.Row = len(e.lines) - 1
		if e.cursor.Row < 0 {
			e.cursor.Row = 0
		}
	}
	e.clampCursorCol()
	if e.mode == ModeInsert && e.cursor.Row != prevRow {
		e.saveLineState()
	}
}

// Helix-style word forward (w) - move to next word start
func (e *Editor) wordForward() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}
	line := e.lines[e.cursor.Row]
	idx := e.cursor.Col

	// If at end of line, move to next line
	if idx >= len(line) {
		if e.cursor.Row >= len(e.lines)-1 {
			return
		}
		e.cursor.Row++
		e.cursor.Col = 0
		// Skip to first non-space
		line = e.lines[e.cursor.Row]
		for e.cursor.Col < len(line) && isSpaceRune(line[e.cursor.Col]) {
			e.cursor.Col++
		}
		return
	}

	// Remember if we started on a word (not punctuation)
	startedOnWord := isWordRune(line[idx])
	wordEndIdx := idx

	// Skip current word or punctuation
	if isWordRune(line[idx]) {
		for idx < len(line) && isWordRune(line[idx]) {
			idx++
		}
		wordEndIdx = idx - 1 // Last char of word
	} else if !isSpaceRune(line[idx]) {
		for idx < len(line) && !isSpaceRune(line[idx]) && !isWordRune(line[idx]) {
			idx++
		}
	}

	// Check if there's whitespace before next word
	hasWhitespace := idx < len(line) && isSpaceRune(line[idx])

	// Edge case: started on word, no whitespace, next char is punctuation
	// In this case, behave like 'e' - stop at end of current word
	if startedOnWord && !hasWhitespace && idx < len(line) && !isWordRune(line[idx]) {
		e.cursor.Col = wordEndIdx
		return
	}

	// Skip whitespace to next word
	for idx < len(line) && isSpaceRune(line[idx]) {
		idx++
	}

	// If reached end of line, move to next line
	if idx >= len(line) && e.cursor.Row < len(e.lines)-1 {
		e.cursor.Row++
		e.cursor.Col = 0
		line = e.lines[e.cursor.Row]
		for e.cursor.Col < len(line) && isSpaceRune(line[e.cursor.Col]) {
			e.cursor.Col++
		}
		return
	}

	e.cursor.Col = idx
}

// Helix-style word backward (b) - move to previous word start
func (e *Editor) wordBackward() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}
	line := e.lines[e.cursor.Row]
	idx := e.cursor.Col

	// If at start of line, move to previous line
	if idx <= 0 {
		if e.cursor.Row <= 0 {
			return
		}
		e.cursor.Row--
		line = e.lines[e.cursor.Row]
		e.cursor.Col = len(line)
		// Recursively find previous word start
		e.wordBackward()
		return
	}

	// Move back one char to get off current position
	idx--

	// Skip whitespace backwards
	for idx > 0 && isSpaceRune(line[idx]) {
		idx--
	}

	// If reached start of line
	if idx <= 0 {
		if isSpaceRune(line[0]) && e.cursor.Row > 0 {
			e.cursor.Row--
			line = e.lines[e.cursor.Row]
			e.cursor.Col = len(line)
			e.wordBackward()
			return
		}
		e.cursor.Col = 0
		return
	}

	// Find start of current word
	if isWordRune(line[idx]) {
		for idx > 0 && isWordRune(line[idx-1]) {
			idx--
		}
	} else {
		for idx > 0 && !isSpaceRune(line[idx-1]) && !isWordRune(line[idx-1]) {
			idx--
		}
	}

	e.cursor.Col = idx
}

// Helix-style word end (e) - move to end of word
func (e *Editor) wordEnd() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}
	line := e.lines[e.cursor.Row]
	idx := e.cursor.Col

	// Move forward one position to get off current word end
	idx++

	// Skip whitespace
	for idx < len(line) && isSpaceRune(line[idx]) {
		idx++
	}

	// If reached end of line, move to next line
	if idx >= len(line) {
		if e.cursor.Row >= len(e.lines)-1 {
			e.cursor.Col = len(line)
			return
		}
		e.cursor.Row++
		line = e.lines[e.cursor.Row]
		idx = 0
		// Skip whitespace on new line
		for idx < len(line) && isSpaceRune(line[idx]) {
			idx++
		}
	}

	// Find end of word
	if idx < len(line) {
		if isWordRune(line[idx]) {
			for idx < len(line)-1 && isWordRune(line[idx+1]) {
				idx++
			}
		} else if !isSpaceRune(line[idx]) {
			for idx < len(line)-1 && !isSpaceRune(line[idx+1]) && !isWordRune(line[idx+1]) {
				idx++
			}
		}
	}

	e.cursor.Col = idx
}

// Helix-style goto line (G) - go to last line
func (e *Editor) gotoLastLine() {
	if len(e.lines) == 0 {
		return
	}
	e.cursor.Row = len(e.lines) - 1
	e.cursor.Col = 0
}

// Helix-style goto first line (gg)
func (e *Editor) gotoFirstLine() {
	e.cursor.Row = 0
	e.cursor.Col = 0
}

// Helix-style goto file end (ge) - go to end of file
func (e *Editor) gotoFileEnd() {
	if len(e.lines) == 0 {
		e.cursor.Row = 0
		e.cursor.Col = 0
		return
	}
	e.cursor.Row = len(e.lines) - 1
	e.cursor.Col = len(e.lines[e.cursor.Row])
}

// findCharForward finds next occurrence of char on current line
// isBracketOrQuote returns true if char is a bracket or quote that should search across lines
func isBracketOrQuote(ch rune) bool {
	switch ch {
	case '(', ')', '[', ']', '{', '}', '<', '>', '\'', '"', '`':
		return true
	}
	return false
}

func (e *Editor) findCharForward(ch rune, till bool) bool {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return false
	}

	// For brackets/quotes, search across lines
	if isBracketOrQuote(ch) {
		startRow := e.cursor.Row
		startCol := e.cursor.Col + 1

		// For till mode: if char at cursor+1 is the target, skip it
		// (we're already at the "till" position from previous search)
		if till && startCol < len(e.lines[startRow]) && e.lines[startRow][startCol] == ch {
			startCol++
		}

		for row := startRow; row < len(e.lines); row++ {
			line := e.lines[row]
			fromCol := 0
			if row == startRow {
				fromCol = startCol
			}
			for col := fromCol; col < len(line); col++ {
				if line[col] == ch {
					e.cursor.Row = row
					if till {
						// For till, stop one char before
						if col > 0 {
							e.cursor.Col = col - 1
						} else if row > startRow {
							// If at start of line, go to end of previous line
							e.cursor.Row = row - 1
							e.cursor.Col = len(e.lines[row-1])
						} else {
							e.cursor.Col = col
						}
					} else {
						e.cursor.Col = col
					}
					return true
				}
			}
		}
		return false
	}

	// For regular chars, search only on current line
	line := e.lines[e.cursor.Row]
	startIdx := e.cursor.Col + 1

	// For till mode: skip immediate target
	if till && startIdx < len(line) && line[startIdx] == ch {
		startIdx++
	}

	for i := startIdx; i < len(line); i++ {
		if line[i] == ch {
			if till {
				e.cursor.Col = i - 1
			} else {
				e.cursor.Col = i
			}
			return true
		}
	}
	return false
}

// findCharBackward finds previous occurrence of char
func (e *Editor) findCharBackward(ch rune, till bool) bool {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return false
	}

	// For brackets/quotes, search across lines backwards
	if isBracketOrQuote(ch) {
		startRow := e.cursor.Row
		startCol := e.cursor.Col - 1

		// For till mode: if char at cursor-1 is the target, skip it
		// (we're already at the "till" position from previous search)
		if till && startCol >= 0 && e.lines[startRow][startCol] == ch {
			startCol--
		}

		for row := startRow; row >= 0; row-- {
			line := e.lines[row]
			toCol := len(line) - 1
			if row == startRow {
				toCol = startCol
			}
			for col := toCol; col >= 0; col-- {
				if line[col] == ch {
					e.cursor.Row = row
					if till {
						// For till, stop one char after
						if col < len(line)-1 {
							e.cursor.Col = col + 1
						} else if row < len(e.lines)-1 {
							// If at end of line, go to start of next line
							e.cursor.Row = row + 1
							e.cursor.Col = 0
						} else {
							e.cursor.Col = col
						}
					} else {
						e.cursor.Col = col
					}
					return true
				}
			}
		}
		return false
	}

	// For regular chars, search only on current line
	line := e.lines[e.cursor.Row]
	startIdx := e.cursor.Col - 1

	// For till mode: skip immediate target
	if till && startIdx >= 0 && line[startIdx] == ch {
		startIdx--
	}

	for i := startIdx; i >= 0; i-- {
		if line[i] == ch {
			if till {
				e.cursor.Col = i + 1
			} else {
				e.cursor.Col = i
			}
			return true
		}
	}
	return false
}

// setPendingFindChar sets up pending char find (f/F/t/T)
func (e *Editor) setPendingFindChar(action string) {
	e.pendingAction = action
}

// handlePendingChar processes char input for pending action
func (e *Editor) handlePendingChar(ch rune) bool {
	action := e.pendingAction
	e.pendingAction = ""

	// For f, F, t, T - Helix style: anchor moves to old cursor, selection covers jump
	isSelectingAction := action == actionFindChar || action == actionFindCharBackward ||
		action == actionTillChar || action == actionTillCharBackward

	anchor := e.cursor

	var result bool
	switch action {
	case actionFindChar:
		e.lastFindChar = ch
		e.lastFindForward = true
		e.lastFindTill = false
		result = e.findCharForward(ch, false)
	case actionFindCharBackward:
		e.lastFindChar = ch
		e.lastFindForward = false
		e.lastFindTill = false
		result = e.findCharBackward(ch, false)
	case actionTillChar:
		e.lastFindChar = ch
		e.lastFindForward = true
		e.lastFindTill = true
		result = e.findCharForward(ch, true)
	case actionTillCharBackward:
		e.lastFindChar = ch
		e.lastFindForward = false
		e.lastFindTill = true
		result = e.findCharBackward(ch, true)
	case actionReplaceChar:
		return e.replaceCharAtCursor(ch)
	default:
		return false
	}

	// Set selection from anchor to new cursor position (inclusive of cursor char)
	if isSelectingAction && anchor != e.cursor {
		e.selectionActive = true
		e.selectionStart = anchor
		// Selection end is exclusive, so add 1 to include the character at cursor
		e.selectionEnd = Cursor{Row: e.cursor.Row, Col: e.cursor.Col + 1}
		e.selectMode = true
	}

	return result
}

// Helix-style delete (d) - delete selection or char
func (e *Editor) helixDelete() {
	if start, end, ok := e.selectionRange(); ok {
		e.deleteSelection(start, end, true) // Restore selection on undo
		e.clearSelection()
		e.selectMode = false
		return
	}
	// No selection - delete char at cursor
	e.deleteChar()
}

// Helix-style change (c) - delete selection and enter insert mode
func (e *Editor) helixChange() {
	if start, end, ok := e.selectionRange(); ok {
		e.deleteSelection(start, end, true) // Restore selection on undo
		e.clearSelection()
		e.selectMode = false
	}
	e.mode = ModeInsert
	e.saveLineState()
}

// copyToSystemClipboard copies text to macOS clipboard using pbcopy
func (e *Editor) copyToSystemClipboard() {
	if len(e.clipboard) == 0 {
		return
	}
	// Join clipboard lines with newlines
	var lines []string
	for _, line := range e.clipboard {
		lines = append(lines, string(line))
	}
	text := strings.Join(lines, "\n")

	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}

// Helix-style yank (y) - copy selection to clipboard
func (e *Editor) yankSelection() {
	start, end, ok := e.selectionRange()
	if !ok {
		// No selection - yank current line
		if e.cursor.Row >= 0 && e.cursor.Row < len(e.lines) {
			e.clipboard = [][]rune{append([]rune(nil), e.lines[e.cursor.Row]...)}
		}
		e.copyToSystemClipboard()
		e.lastCommand = "y"
		e.copiedMessageTime = time.Now()
		return
	}

	// Copy selection to clipboard
	e.clipboard = nil
	for row := start.Row; row <= end.Row; row++ {
		if row < 0 || row >= len(e.lines) {
			continue
		}
		line := e.lines[row]
		startCol := 0
		endCol := len(line)
		if row == start.Row {
			startCol = start.Col
		}
		if row == end.Row {
			endCol = end.Col
		}
		if startCol < 0 {
			startCol = 0
		}
		if endCol > len(line) {
			endCol = len(line)
		}
		e.clipboard = append(e.clipboard, append([]rune(nil), line[startCol:endCol]...))
	}
	e.copyToSystemClipboard()
	e.lastCommand = "y"
	e.copiedMessageTime = time.Now()
	e.clearSelection()
	e.selectMode = false
}

// Helix-style paste (p) - paste after cursor
func (e *Editor) pasteAfter() {
	if len(e.clipboard) == 0 {
		return
	}

	e.startUndoGroup()
	defer e.finishUndoGroup()

	if len(e.clipboard) == 1 {
		// Single line - paste inline after cursor
		line := e.clipboard[0]
		pos := Cursor{Row: e.cursor.Row, Col: e.cursor.Col + 1}
		if pos.Col > len(e.lines[e.cursor.Row]) {
			pos.Col = len(e.lines[e.cursor.Row])
		}
		for _, r := range line {
			if e.insertRuneAt(pos, r) {
				e.appendUndo(action{kind: actionDeleteRune, pos: pos, r: r})
				pos.Col++
			}
		}
		e.cursor.Col = pos.Col - 1
		if e.cursor.Col < 0 {
			e.cursor.Col = 0
		}
	} else {
		// Multi-line - paste lines below
		for i, line := range e.clipboard {
			newRow := e.cursor.Row + 1 + i
			// Insert new line
			if newRow > len(e.lines) {
				newRow = len(e.lines)
			}
			newLines := make([][]rune, len(e.lines)+1)
			copy(newLines, e.lines[:newRow])
			newLines[newRow] = append([]rune(nil), line...)
			copy(newLines[newRow+1:], e.lines[newRow:])
			e.lines = newLines
		}
		e.cursor.Row++
		e.cursor.Col = 0
		e.lastEdit.Valid = false
	}
}

// Helix-style paste before (P) - paste before cursor
func (e *Editor) pasteBefore() {
	if len(e.clipboard) == 0 {
		return
	}

	e.startUndoGroup()
	defer e.finishUndoGroup()

	if len(e.clipboard) == 1 {
		// Single line - paste inline at cursor
		line := e.clipboard[0]
		pos := e.cursor
		for _, r := range line {
			if e.insertRuneAt(pos, r) {
				e.appendUndo(action{kind: actionDeleteRune, pos: pos, r: r})
				pos.Col++
			}
		}
	} else {
		// Multi-line - paste lines above
		for i, line := range e.clipboard {
			newRow := e.cursor.Row + i
			newLines := make([][]rune, len(e.lines)+1)
			copy(newLines, e.lines[:newRow])
			newLines[newRow] = append([]rune(nil), line...)
			copy(newLines[newRow+1:], e.lines[newRow:])
			e.lines = newLines
		}
		e.cursor.Col = 0
		e.lastEdit.Valid = false
	}
}

// Helix-style open below (o) - open line below and enter insert
func (e *Editor) openBelow() {
	e.insertLineBelow()
	e.mode = ModeInsert
	e.saveLineState()
}

// Helix-style open above (O) - open line above and enter insert
func (e *Editor) openAbove() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}

	// Get current line's indentation
	line := e.lines[e.cursor.Row]
	indent := make([]rune, 0)
	for _, r := range line {
		if r == ' ' || r == '\t' {
			indent = append(indent, r)
		} else {
			break
		}
	}

	// Insert new line above
	newLines := make([][]rune, len(e.lines)+1)
	copy(newLines, e.lines[:e.cursor.Row])
	newLines[e.cursor.Row] = append([]rune(nil), indent...)
	copy(newLines[e.cursor.Row+1:], e.lines[e.cursor.Row:])
	e.lines = newLines

	e.cursor.Col = len(indent)
	e.mode = ModeInsert
	e.saveLineState()
	e.lastEdit.Valid = false
}

// insertLineAboveCursor inserts an empty line at cursor position,
// pushing current line down. The new line is indented with tabs/spaces
// up to the cursor's visual column. Cursor stays at same position.
func (e *Editor) insertLineAboveCursor() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}

	line := e.lines[e.cursor.Row]
	tabWidth := e.tabWidth
	if tabWidth < 1 {
		tabWidth = 4
	}

	// Calculate cursor's visual column
	visualX := visualCol(line, e.cursor.Col, tabWidth)

	// Build indentation: fill with tabs, then spaces for remainder
	indent := make([]rune, 0)
	col := 0
	for col+tabWidth <= visualX {
		indent = append(indent, '\t')
		col += tabWidth
	}
	// Add spaces for remaining 1-3 positions
	for col < visualX {
		indent = append(indent, ' ')
		col++
	}

	// Insert new line at cursor position, push current line down
	newLines := make([][]rune, len(e.lines)+1)
	copy(newLines, e.lines[:e.cursor.Row])
	newLines[e.cursor.Row] = indent
	copy(newLines[e.cursor.Row+1:], e.lines[e.cursor.Row:])
	e.lines = newLines

	// Record undo: to undo this, we need to join the line we created
	// The position is at the end of the new line (which has the indent)
	e.recordUndo(action{kind: actionJoinLine, pos: Cursor{Row: e.cursor.Row, Col: len(indent)}})

	// Cursor stays at same row (now on the new indented line)
	e.cursor.Col = len(indent)
	e.lastEdit.Valid = false
}

// Helix-style append (a) - move right and enter insert
func (e *Editor) appendMode() {
	e.moveRight()
	e.mode = ModeInsert
	e.saveLineState()
}

// Helix-style append line end (A) - go to line end and enter insert
func (e *Editor) appendLineEnd() {
	e.moveLineEnd()
	e.mode = ModeInsert
	e.saveLineState()
}

// Helix-style insert line start (I) - go to first non-whitespace and insert
func (e *Editor) insertLineStart() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}
	line := e.lines[e.cursor.Row]
	// Find first non-whitespace
	col := 0
	for col < len(line) && (line[col] == ' ' || line[col] == '\t') {
		col++
	}
	e.cursor.Col = col
	e.mode = ModeInsert
	e.saveLineState()
}

// Helix-style replace char (r) - replace char at cursor
func (e *Editor) replaceCharAtCursor(ch rune) bool {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return false
	}
	line := e.lines[e.cursor.Row]
	if e.cursor.Col < 0 || e.cursor.Col >= len(line) {
		return false
	}

	oldChar := line[e.cursor.Col]
	e.startUndoGroup()
	// Delete old char
	if e.deleteRuneAt(e.cursor) {
		e.appendUndo(action{kind: actionInsertRune, pos: e.cursor, r: oldChar})
	}
	// Insert new char
	if e.insertRuneAt(e.cursor, ch) {
		e.appendUndo(action{kind: actionDeleteRune, pos: e.cursor, r: ch})
	}
	e.finishUndoGroup()
	return true
}

// Helix-style join lines (J) - join current line with next
func (e *Editor) joinLinesCmd() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines)-1 {
		return
	}

	// Position at end of current line
	pos := Cursor{Row: e.cursor.Row, Col: len(e.lines[e.cursor.Row])}

	// Add a space before joining (unless line ends with space or next line starts with space)
	currentLine := e.lines[e.cursor.Row]
	nextLine := e.lines[e.cursor.Row+1]
	needSpace := len(currentLine) > 0 && len(nextLine) > 0 &&
		!isSpaceRune(currentLine[len(currentLine)-1]) &&
		!isSpaceRune(nextLine[0])

	if needSpace {
		if e.insertRuneAt(pos, ' ') {
			e.recordUndo(action{kind: actionDeleteRune, pos: pos, r: ' '})
			pos.Col++
		}
	}

	// Join lines
	if e.joinLineAt(pos) {
		e.recordUndo(action{kind: actionSplitLine, pos: pos})
	}

	e.cursor = pos
}

// Helix-style toggle select (v) - toggle selection mode
func (e *Editor) toggleSelectMode() {
	e.selectMode = !e.selectMode
	if e.selectMode {
		// Start selection at cursor
		e.selectionStart = e.cursor
		e.selectionEnd = e.cursor
		e.selectionActive = true
	} else {
		e.clearSelection()
	}
}

// Helix-style extend line (x) - select current line
func (e *Editor) extendLine() {
	if e.cursor.Row < 0 || e.cursor.Row >= len(e.lines) {
		return
	}

	// Select entire current line including newline
	e.selectionStart = Cursor{Row: e.cursor.Row, Col: 0}
	if e.cursor.Row < len(e.lines)-1 {
		e.selectionEnd = Cursor{Row: e.cursor.Row + 1, Col: 0}
	} else {
		e.selectionEnd = Cursor{Row: e.cursor.Row, Col: len(e.lines[e.cursor.Row])}
	}
	e.selectionActive = true
	e.selectMode = true
}

// Helix-style collapse selection (;) - collapse selection to cursor
func (e *Editor) collapseSelection() {
	e.clearSelection()
	e.selectMode = false
}

// Helix-style flip selection (Alt+;) - swap anchor and cursor
func (e *Editor) flipSelection() {
	if !e.selectionActive {
		return
	}
	e.selectionStart, e.selectionEnd = e.selectionEnd, e.selectionStart
	e.cursor = e.selectionEnd
}

func (e *Editor) clampCursorCol() {
	lineLen := len(e.lines[e.cursor.Row])
	if e.cursor.Col > lineLen {
		e.cursor.Col = lineLen
	}
}

func (e *Editor) ensureCursorVisible(viewHeight int) {
	if viewHeight <= 0 {
		return
	}
	// If cursor is far outside visible area, center it
	if e.cursor.Row < e.scroll-1 || e.cursor.Row >= e.scroll+viewHeight+1 {
		e.scroll = e.cursor.Row - viewHeight/2
		if e.scroll < 0 {
			e.scroll = 0
		}
		return
	}
	// Otherwise just make it visible at the edge
	if e.cursor.Row < e.scroll {
		e.scroll = e.cursor.Row
		return
	}
	if e.cursor.Row >= e.scroll+viewHeight {
		e.scroll = e.cursor.Row - viewHeight + 1
	}
}

func (e *Editor) renderStatusline(s tcell.Screen, w, y int) {
	mode := "NORMAL"
	if e.mode == ModeInsert {
		mode = "INSERT"
	} else if e.mode == ModeCommand {
		mode = "COMMAND"
	} else if e.mode == ModeBranchPicker {
		mode = "BRANCHES"
	} else if e.mode == ModeSearch {
		mode = "SEARCH"
	}
	name := e.filename
	if name == "" {
		name = "[No Name]"
	} else {
		name = filepath.Base(name)
	}
	dirty := ""
	if e.dirty {
		dirty = "*"
	}

	status := fmt.Sprintf(" %s | %s%s ", mode, name, dirty)
	if e.statusMessage != "" {
		status = fmt.Sprintf(" %s | %s%s | %s ", mode, name, dirty, e.statusMessage)
	}
	row := e.cursor.Row + 1
	col := 1
	if e.cursor.Row >= 0 && e.cursor.Row < len(e.lines) {
		col = visualCol(e.lines[e.cursor.Row], e.cursor.Col, e.tabWidth) + 1
	}
	right := fmt.Sprintf(" Ln %d, Col %d", row, col)
	if e.gitBranch != "" {
		right += " | " + formatGitBranch(e.gitBranchSymbol, e.gitBranch)
	}
	if e.layoutName != "" {
		right = right + " | " + e.layoutName
	}

	line := composeStatusLine(status, right, w)
	for x, r := range line {
		if x >= w {
			break
		}
		s.SetContent(x, y, r, nil, e.styleStatus)
	}
}

func (e *Editor) renderCommandline(s tcell.Screen, w, y int) int {
	var cmdRunes []rune
	var rightText string

	if e.mode == ModeSearch {
		// Search mode: show /query with match count
		prefix := '/'
		if !e.searchForward {
			prefix = '?'
		}
		cmdRunes = append([]rune{prefix}, e.searchQuery...)

		// Show match count on the right
		if len(e.searchMatches) > 0 {
			rightText = fmt.Sprintf(" [%d/%d] ", e.searchMatchIndex+1, len(e.searchMatches))
		} else if len(e.searchQuery) > 0 {
			rightText = " [no matches] "
		}
	} else if e.mode == ModeCommand {
		cmdRunes = append([]rune{':'}, e.cmd...)
	} else {
		cmdRunes = e.cmd
	}

	// Prepare right side: pending keys or last command (if not in search mode)
	// Check if "copied" message should be shown (within 2 seconds)
	const copiedMessageDuration = 2 * time.Second
	showCopiedMessage := time.Since(e.copiedMessageTime) < copiedMessageDuration && e.lastCommand == "y"
	checkmarkPos := -1 // position of ✓ in rightRunes for green coloring

	if rightText == "" {
		if e.pendingKeys != "" {
			// Show pending keys while waiting for next key (e.g., "g", "f")
			rightText = " " + e.pendingKeys + "_ "
		} else if showCopiedMessage {
			// Show "copied [✓] | y"
			rightText = " copied [✓] | y "
			checkmarkPos = 9 // position of ✓ in " copied [✓] | y "
		} else if e.lastCommand != "" {
			// Show last executed command (e.g., "gg", "fw")
			rightText = " " + e.lastCommand + " "
		} else if e.lastKeyCombo != "" {
			// Fallback to last key combo
			rightText = " " + e.lastKeyCombo + " "
		}
	}

	rightRunes := []rune(rightText)
	rightStart := w - len(rightRunes)
	if rightStart < 0 {
		rightStart = 0
		rightRunes = rightRunes[:w]
		// Adjust checkmark position if truncated
		if checkmarkPos >= len(rightRunes) {
			checkmarkPos = -1
		}
	}

	// Calculate available width for command
	availableWidth := rightStart
	if availableWidth < 0 {
		availableWidth = 0
	}

	// Calculate cursor position
	var cursorX int
	if e.mode == ModeCommand {
		cursorX = e.cmdCursor + 1 // +1 for ':' prefix
	} else if e.mode == ModeSearch {
		cursorX = e.searchCursor + 1 // +1 for '/' or '?' prefix
	} else {
		cursorX = len(cmdRunes)
	}

	// Handle scrolling if command is too long
	if len(cmdRunes) > availableWidth {
		// Ensure cursor is visible
		start := 0
		if cursorX > availableWidth-1 {
			start = cursorX - availableWidth + 1
		}
		if start > len(cmdRunes)-availableWidth {
			start = len(cmdRunes) - availableWidth
		}
		if start < 0 {
			start = 0
		}
		cmdRunes = cmdRunes[start:]
		cursorX = cursorX - start
	}

	// Style for green checkmark
	styleGreen := e.styleCommand.Foreground(tcell.ColorGreen)

	// Draw command line content
	for x := 0; x < w; x++ {
		if x < len(cmdRunes) {
			s.SetContent(x, y, cmdRunes[x], nil, e.styleCommand)
		} else if x >= rightStart && x-rightStart < len(rightRunes) {
			idx := x - rightStart
			style := e.styleCommand
			if idx == checkmarkPos {
				style = styleGreen
			}
			s.SetContent(x, y, rightRunes[idx], nil, style)
		} else {
			s.SetContent(x, y, ' ', nil, e.styleCommand)
		}
	}

	if cursorX < 0 {
		cursorX = 0
	}
	if cursorX >= w {
		cursorX = w - 1
	}
	return cursorX
}

const scrollIndicatorDuration = 1500 * time.Millisecond

func (e *Editor) renderScrollIndicator(s tcell.Screen, w, viewHeight int) {
	if viewHeight < 1 || w < 1 {
		return
	}

	// Check if scroll indicator should be visible
	elapsed := time.Since(e.lastScrollTime)
	if elapsed >= scrollIndicatorDuration {
		return
	}

	totalLines := len(e.lines)
	if totalLines <= viewHeight {
		return // No need for scroll indicator if all content fits
	}

	// Calculate thumb size (minimum 1 row)
	thumbSize := viewHeight * viewHeight / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}

	// Calculate thumb position
	maxScroll := totalLines - viewHeight
	if maxScroll < 1 {
		maxScroll = 1
	}
	thumbPos := e.scroll * (viewHeight - thumbSize) / maxScroll
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos+thumbSize > viewHeight {
		thumbPos = viewHeight - thumbSize
	}

	// Calculate opacity based on time elapsed (fade out effect)
	// 0-1000ms: full opacity, 1000-1500ms: fade out
	var thumbChar rune
	var trackChar rune
	fadeStart := 1000 * time.Millisecond
	if elapsed < fadeStart {
		thumbChar = '█'
		trackChar = '░'
	} else {
		// Fade out: use lighter characters
		fadeProgress := float64(elapsed-fadeStart) / float64(scrollIndicatorDuration-fadeStart)
		if fadeProgress < 0.33 {
			thumbChar = '▓'
			trackChar = '░'
		} else if fadeProgress < 0.66 {
			thumbChar = '▒'
			trackChar = ' '
		} else {
			thumbChar = '░'
			trackChar = ' '
		}
	}

	// Draw scroll indicator in the rightmost column
	x := w - 1
	style := tcell.StyleDefault.Foreground(tcell.ColorGray)
	for y := 0; y < viewHeight; y++ {
		var ch rune
		if y >= thumbPos && y < thumbPos+thumbSize {
			ch = thumbChar
		} else {
			ch = trackChar
		}
		if ch != ' ' {
			s.SetContent(x, y, ch, nil, style)
		}
	}
}

func (e *Editor) viewHeightCached() int {
	if e.viewHeight < 1 {
		return 1
	}
	return e.viewHeight
}

func splitLines(data []byte) [][]rune {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	parts := strings.Split(text, "\n")
	lines := make([][]rune, len(parts))
	for i, p := range parts {
		lines[i] = []rune(p)
	}
	return lines
}

func joinLines(lines [][]rune) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(string(line))
	}
	return b.String()
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

func (e *Editor) SetNodeStackFunc(fn NodeStackFunc) {
	e.nodeStackFunc = fn
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

func (e *Editor) clearSelection() {
	e.selectionActive = false
	e.selectionStart = Cursor{}
	e.selectionEnd = Cursor{}
}

func (e *Editor) selectAll() {
	if len(e.lines) == 0 {
		return
	}
	e.selectionStart = Cursor{Row: 0, Col: 0}
	lastRow := len(e.lines) - 1
	e.selectionEnd = Cursor{Row: lastRow, Col: len(e.lines[lastRow])}
	e.selectionActive = true
}

// expandSelection expands selection to the next larger syntax node
func (e *Editor) expandSelection() {
	if e.nodeStackFunc == nil || e.filename == "" {
		e.setStatus("syntax tree not available")
		return
	}

	// Get node stack at cursor position
	stack := e.nodeStackFunc(e.filename, e.cursor.Row, e.cursor.Col)
	if len(stack) == 0 {
		e.setStatus("no syntax node at cursor")
		return
	}

	// If no selection or selection changed, rebuild scope stack
	if !e.selectionActive || len(e.selectionScopeStack) == 0 {
		e.selectionScopeStack = stack
		e.selectionScopeIndex = 0
	}

	// Find next larger scope
	if e.selectionScopeIndex < len(e.selectionScopeStack) {
		nr := e.selectionScopeStack[e.selectionScopeIndex]
		e.selectionStart = Cursor{Row: nr.StartRow, Col: nr.StartCol}
		e.selectionEnd = Cursor{Row: nr.EndRow, Col: nr.EndCol}
		e.selectionActive = true
		e.selectMode = true
		e.selectionScopeIndex++
	}
}

// shrinkSelection shrinks selection to the next smaller syntax node
func (e *Editor) shrinkSelection() {
	if !e.selectionActive || len(e.selectionScopeStack) == 0 {
		return
	}

	// Go back to previous scope
	if e.selectionScopeIndex > 1 {
		e.selectionScopeIndex--
		nr := e.selectionScopeStack[e.selectionScopeIndex-1]
		e.selectionStart = Cursor{Row: nr.StartRow, Col: nr.StartCol}
		e.selectionEnd = Cursor{Row: nr.EndRow, Col: nr.EndCol}
	} else {
		// Can't shrink further, clear selection
		e.clearSelection()
		e.selectMode = false
		e.selectionScopeStack = nil
		e.selectionScopeIndex = 0
	}
}

func (e *Editor) extendSelection(move func()) {
	before := e.cursor
	if !e.selectionActive {
		e.selectionStart = before
	}
	move()
	if before == e.cursor && !e.selectionActive {
		return
	}
	e.selectionActive = true
	e.selectionEnd = e.cursor
}

func (e *Editor) selectionRange() (Cursor, Cursor, bool) {
	if !e.selectionActive {
		return Cursor{}, Cursor{}, false
	}
	if e.selectionStart == e.selectionEnd {
		return Cursor{}, Cursor{}, false
	}
	start := e.selectionStart
	end := e.selectionEnd
	if cursorLess(end, start) {
		start, end = end, start
	}
	return start, end, true
}

func cursorLess(a, b Cursor) bool {
	if a.Row != b.Row {
		return a.Row < b.Row
	}
	return a.Col < b.Col
}

func (e *Editor) selectionRangeForLine(lineIdx int) (int, int, bool) {
	start, end, ok := e.selectionRange()
	if !ok {
		return 0, 0, false
	}
	if lineIdx < start.Row || lineIdx > end.Row {
		return 0, 0, false
	}
	lineLen := 0
	if lineIdx >= 0 && lineIdx < len(e.lines) {
		lineLen = len(e.lines[lineIdx])
	}
	startCol := 0
	endCol := lineLen
	if start.Row == end.Row {
		startCol = clampRange(start.Col, 0, lineLen)
		endCol = clampRange(end.Col, 0, lineLen)
	} else if lineIdx == start.Row {
		startCol = clampRange(start.Col, 0, lineLen)
		endCol = lineLen
	} else if lineIdx == end.Row {
		startCol = 0
		endCol = clampRange(end.Col, 0, lineLen)
	}
	if endCol <= startCol {
		return 0, 0, false
	}
	return startCol, endCol, true
}

func (e *Editor) styleForHighlight(kind string) (tcell.Style, bool) {
	switch kind {
	case "keyword":
		return e.styleSyntaxKeyword, true
	case "string":
		return e.styleSyntaxString, true
	case "comment":
		return e.styleSyntaxComment, true
	case "type":
		return e.styleSyntaxType, true
	case "function":
		return e.styleSyntaxFunction, true
	case "number":
		return e.styleSyntaxNumber, true
	case "constant":
		return e.styleSyntaxConstant, true
	case "operator":
		return e.styleSyntaxOperator, true
	case "punctuation":
		return e.styleSyntaxPunctuation, true
	case "field":
		return e.styleSyntaxField, true
	case "builtin":
		return e.styleSyntaxBuiltin, true
	case "variable":
		return e.styleSyntaxVariable, true
	case "parameter":
		return e.styleSyntaxParameter, true
	default:
		return e.styleMain, false
	}
}

func highlightPriority(kind string) int {
	switch kind {
	case "comment":
		return 7
	case "string":
		return 6
	case "keyword":
		return 5
	case "constant":
		return 4
	case "builtin":
		return 4
	case "parameter":
		return 3
	case "type", "function", "number":
		return 3
	case "field":
		return 2
	case "variable":
		return 2
	case "operator":
		return 1
	case "punctuation":
		return 1
	default:
		return 0
	}
}

func highlightKindAt(spans []HighlightSpan, col int) (string, bool) {
	bestKind := ""
	bestPriority := 0
	for _, span := range spans {
		if col < span.StartCol || col >= span.EndCol {
			continue
		}
		priority := highlightPriority(span.Kind)
		if priority > bestPriority {
			bestPriority = priority
			bestKind = span.Kind
		}
	}
	if bestKind == "" {
		return "", false
	}
	return bestKind, true
}

func clampRange(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (e *Editor) ConsumeBranchPickerRequest() bool {
	if !e.branchPickerRequested {
		return false
	}
	e.branchPickerRequested = false
	return true
}

func (e *Editor) ShowBranchPicker(branches []string, current string) {
	if len(branches) == 0 {
		e.setStatus("no branches")
		return
	}
	items := make([]string, len(branches))
	copy(items, branches)
	e.branchPickerItems = items
	e.branchPickerIndex = 0
	if current != "" {
		for i, name := range items {
			if name == current {
				e.branchPickerIndex = i
				break
			}
		}
	}
	e.branchPickerActive = true
	e.mode = ModeBranchPicker
}

func (e *Editor) ConsumeBranchSelection() (string, bool) {
	if e.branchPickerSelection == "" {
		return "", false
	}
	selection := e.branchPickerSelection
	e.branchPickerSelection = ""
	return selection, true
}

func (e *Editor) branchPickerPageSize() int {
	size := e.viewHeightCached() - 4
	if size < 1 {
		return 1
	}
	return size
}

func (e *Editor) closeBranchPicker(selection string) {
	e.branchPickerActive = false
	e.branchPickerItems = nil
	e.branchPickerIndex = 0
	e.mode = ModeNormal
	e.branchPickerSelection = selection
}

func (e *Editor) swapLines(a, b int) bool {
	if a < 0 || b < 0 || a >= len(e.lines) || b >= len(e.lines) {
		return false
	}
	e.lines[a], e.lines[b] = e.lines[b], e.lines[a]
	e.lastEdit.Valid = false
	return true
}

func runeByteLen(r rune) int {
	n := utf8.RuneLen(r)
	if n < 1 {
		return 1
	}
	return n
}

func runeSliceByteLen(rs []rune) int {
	n := 0
	for _, r := range rs {
		n += runeByteLen(r)
	}
	return n
}

func (e *Editor) byteOffset(pos Cursor) (int, int) {
	row := pos.Row
	if row < 0 {
		row = 0
	}
	if row > len(e.lines) {
		row = len(e.lines)
	}
	offset := 0
	for i := 0; i < row && i < len(e.lines); i++ {
		offset += runeSliceByteLen(e.lines[i]) + 1
	}
	if row >= len(e.lines) {
		return offset, 0
	}
	line := e.lines[row]
	col := pos.Col
	if col < 0 {
		col = 0
	}
	if col > len(line) {
		col = len(line)
	}
	colBytes := runeSliceByteLen(line[:col])
	offset += colBytes
	return offset, colBytes
}

func (e *Editor) recordTextEdit(start, oldEnd, newEnd Cursor, insertedBytes int) {
	startByte, startColBytes := e.byteOffset(start)
	oldEndByte, oldEndColBytes := e.byteOffset(oldEnd)
	newEndByte := startByte + insertedBytes
	newEndColBytes := 0
	if newEnd.Row == start.Row {
		newEndColBytes = startColBytes + insertedBytes
	}
	e.lastEdit = TextEdit{
		Valid:          true,
		StartByte:      startByte,
		OldEndByte:     oldEndByte,
		NewEndByte:     newEndByte,
		StartRow:       start.Row,
		StartColBytes:  startColBytes,
		OldEndRow:      oldEnd.Row,
		OldEndColBytes: oldEndColBytes,
		NewEndRow:      newEnd.Row,
		NewEndColBytes: newEndColBytes,
	}
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isSpaceRune(r rune) bool {
	return unicode.IsSpace(r)
}

func (e *Editor) drawLine(s tcell.Screen, y, w, startX int, line []rune, tabWidth int, selStart, selEnd int, spans []HighlightSpan, highlightActive bool, searchMatches []SearchMatch, lineIdx int, currentMatchIdx int) {
	x := startX
	col := 0
	if tabWidth < 1 {
		tabWidth = 1
	}
	fallbackStyle := e.styleMain
	if highlightActive {
		fallbackStyle = e.styleSyntaxUnknown
	}


	for idx, r := range line {
		if x >= w {
			break
		}
		// First determine the syntax-highlighted style
		activeStyle := fallbackStyle
		if kind, ok := highlightKindAt(spans, idx); ok {
			if style, ok := e.styleForHighlight(kind); ok {
				activeStyle = style
			}
		} else if highlightActive && !isWordRune(r) {
			activeStyle = e.styleMain
		}

		// Check for search match highlight
		isInMatch := false
		isCurrentMatch := false
		isMatchedChar := false // true if this char is one of the fuzzy-matched letters
		for i, match := range searchMatches {
			if match.Row == lineIdx && match.Length > 0 && idx >= match.Col && idx < match.Col+match.Length {
				isInMatch = true
				if i == currentMatchIdx {
					isCurrentMatch = true
					// Check if this char is in MatchedCols (relative to word start)
					relIdx := idx - match.Col
					for _, mc := range match.MatchedCols {
						if mc == relIdx {
							isMatchedChar = true
							break
						}
					}
					// If no MatchedCols, all chars are matched (exact match)
					if len(match.MatchedCols) == 0 {
						isMatchedChar = true
					}
				}
				break
			}
		}

		// Apply overlays: search match or selection
		if isCurrentMatch && isMatchedChar {
			// Current match, matched letter: yellow highlight
			activeStyle = e.styleSearchMatch
		} else if isCurrentMatch {
			// Current match, non-matched letter: selection background
			_, selBg, _ := e.styleSelection.Decompose()
			fg, _, _ := activeStyle.Decompose()
			activeStyle = activeStyle.Foreground(fg).Background(selBg)
		} else if isInMatch {
			// Other matches: selection background
			_, selBg, _ := e.styleSelection.Decompose()
			fg, _, _ := activeStyle.Decompose()
			activeStyle = activeStyle.Foreground(fg).Background(selBg)
		} else if selStart >= 0 && selEnd > selStart && idx >= selStart && idx < selEnd {
			// Selection: only change background, keep syntax foreground
			_, selBg, _ := e.styleSelection.Decompose()
			fg, _, _ := activeStyle.Decompose()
			activeStyle = activeStyle.Foreground(fg).Background(selBg)
		}
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			for i := 0; i < spaces && x < w; i++ {
				s.SetContent(x, y, ' ', nil, activeStyle)
				x++
				col++
			}
			continue
		}
		s.SetContent(x, y, r, nil, activeStyle)
		x++
		col++
	}
	for x < w {
		s.SetContent(x, y, ' ', nil, fallbackStyle)
		x++
	}
}

func clearLine(s tcell.Screen, y, w int, style tcell.Style) {
	for x := 0; x < w; x++ {
		s.SetContent(x, y, ' ', nil, style)
	}
}

func composeStatusLine(left, right string, width int) []rune {
	if width <= 0 {
		return nil
	}
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if len(leftRunes)+len(rightRunes) > width {
		if len(rightRunes) >= width {
			rightRunes = rightRunes[len(rightRunes)-width:]
			leftRunes = nil
		} else {
			leftRunes = leftRunes[:width-len(rightRunes)]
		}
	}
	spaceCount := width - len(leftRunes) - len(rightRunes)
	if spaceCount < 0 {
		spaceCount = 0
	}
	line := make([]rune, 0, width)
	line = append(line, leftRunes...)
	for i := 0; i < spaceCount; i++ {
		line = append(line, ' ')
	}
	line = append(line, rightRunes...)
	return line
}

func formatGitBranch(symbol, branch string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		symbol = "git:"
	}
	if strings.HasSuffix(symbol, ":") || strings.HasSuffix(symbol, " ") {
		return symbol + branch
	}
	return symbol + " " + branch
}

func parseColor(name string, fallback tcell.Color) tcell.Color {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}
	if strings.HasPrefix(name, "#") && len(name) == 7 {
		r, err1 := strconv.ParseInt(name[1:3], 16, 32)
		g, err2 := strconv.ParseInt(name[3:5], 16, 32)
		b, err3 := strconv.ParseInt(name[5:7], 16, 32)
		if err1 == nil && err2 == nil && err3 == nil {
			return tcell.NewRGBColor(int32(r), int32(g), int32(b))
		}
		return fallback
	}
	name = strings.ToLower(name)
	if name == "default" {
		return tcell.ColorDefault
	}
	c := tcell.GetColor(name)
	if c == tcell.ColorDefault {
		return fallback
	}
	return c
}

func visualCol(line []rune, logicalCol int, tabWidth int) int {
	if tabWidth < 1 {
		tabWidth = 1
	}
	if logicalCol < 0 {
		logicalCol = 0
	}
	if logicalCol > len(line) {
		logicalCol = len(line)
	}
	col := 0
	for i := 0; i < logicalCol; i++ {
		if line[i] == '\t' {
			col += tabWidth - (col % tabWidth)
			continue
		}
		col++
	}
	return col
}

func visualToLogicalCol(line []rune, visualX int, tabWidth int) int {
	if tabWidth < 1 {
		tabWidth = 1
	}
	if visualX <= 0 {
		return 0
	}
	col := 0
	for i, r := range line {
		var advance int
		if r == '\t' {
			advance = tabWidth - (col % tabWidth)
		} else {
			advance = 1
		}
		if col+advance > visualX {
			// Click is within this character - return current position
			return i
		}
		col += advance
		if col >= visualX {
			return i + 1
		}
	}
	// Click is past end of line
	return len(line)
}

func parseLineNumberMode(value string) LineNumberMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "relative", "rel":
		return LineNumberRelative
	case "off", "none", "false":
		return LineNumberOff
	default:
		return LineNumberAbsolute
	}
}

func (e *Editor) toggleLineNumbers() {
	switch e.lineNumberMode {
	case LineNumberAbsolute:
		e.lineNumberMode = LineNumberRelative
		e.setStatus("line numbers relative")
	case LineNumberRelative:
		e.lineNumberMode = LineNumberAbsolute
		e.setStatus("line numbers absolute")
	default:
		e.lineNumberMode = LineNumberAbsolute
		e.setStatus("line numbers absolute")
	}
}

func (e *Editor) gutterWidth() int {
	if e.lineNumberMode == LineNumberOff {
		return 0
	}
	maxLine := len(e.lines)
	if maxLine < 1 {
		maxLine = 1
	}
	digits := len(strconv.Itoa(maxLine))
	if digits < 2 {
		digits = 2
	}
	// Format: " " + digits + " " (leading space + number + trailing space)
	return 1 + digits + 1
}

func (e *Editor) drawLineWithGutter(s tcell.Screen, y, w, gutterWidth, lineIdx int) {
	if gutterWidth > 0 {
		// gutterWidth = 1 (leading space) + digits + 1 (trailing space)
		digits := gutterWidth - 2
		if digits < 1 {
			digits = 1
		}
		num := lineIdx + 1
		if e.lineNumberMode == LineNumberRelative && lineIdx != e.cursor.Row {
			diff := lineIdx - e.cursor.Row
			if diff < 0 {
				diff = -diff
			}
			num = diff
		}
		numStr := fmt.Sprintf("%*d", digits, num)
		style := e.styleLineNumber
		if lineIdx == e.cursor.Row {
			style = e.styleLineNumberActive
		}
		// Draw leading space
		if w > 0 {
			s.SetContent(0, y, ' ', nil, e.styleMain)
		}
		// Draw number (right-aligned with leading spaces)
		for i, r := range numStr {
			x := 1 + i
			if x >= gutterWidth-1 || x >= w {
				break
			}
			s.SetContent(x, y, r, nil, style)
		}
		// Draw trailing space
		if gutterWidth-1 < w {
			s.SetContent(gutterWidth-1, y, ' ', nil, e.styleMain)
		}
	}
	if gutterWidth >= w {
		return
	}
	selStart, selEnd, ok := e.selectionRangeForLine(lineIdx)
	if !ok {
		selStart = -1
		selEnd = -1
	}
	highlightActive := e.highlightStart >= 0 && lineIdx >= e.highlightStart && lineIdx <= e.highlightEnd
	var spans []HighlightSpan
	if highlightActive {
		spans = e.highlights[lineIdx]
	}
	e.drawLine(s, y, w, gutterWidth, e.lines[lineIdx], e.tabWidth, selStart, selEnd, spans, highlightActive, e.searchMatches, lineIdx, e.searchMatchIndex)
}

func (e *Editor) renderBranchPicker(s tcell.Screen, w, viewHeight int) {
	if !e.branchPickerActive || len(e.branchPickerItems) == 0 {
		return
	}
	if w < 6 || viewHeight < 3 {
		return
	}
	title := "Select git branch"
	titleRunes := []rune(title)
	titleWidth := len(titleRunes) + 2
	maxItem := titleWidth
	for _, name := range e.branchPickerItems {
		if l := len([]rune(name)); l > maxItem {
			maxItem = l
		}
	}
	boxWidth := maxItem + 4
	if boxWidth > w-2 {
		boxWidth = w - 2
	}
	if boxWidth < 8 {
		if w < 8 {
			boxWidth = w
		} else {
			boxWidth = 8
		}
	}
	listHeight := viewHeight - 2
	if listHeight < 1 {
		return
	}
	if listHeight > len(e.branchPickerItems) {
		listHeight = len(e.branchPickerItems)
	}
	boxHeight := listHeight + 2
	if boxHeight > viewHeight {
		boxHeight = viewHeight
		listHeight = boxHeight - 2
	}
	x0 := (w - boxWidth) / 2
	if x0 < 0 {
		x0 = 0
	}
	y0 := (viewHeight - boxHeight) / 2
	if y0 < 0 {
		y0 = 0
	}

	borderStyle := e.styleStatus
	itemStyle := e.styleStatus
	selectedStyle := e.styleSelection
	innerWidth := boxWidth - 2

	topLeft := '┌'
	topRight := '┐'
	bottomLeft := '└'
	bottomRight := '┘'
	hLine := '─'
	vLine := '│'
	for x := 0; x < boxWidth; x++ {
		chTop := hLine
		chBottom := hLine
		if x == 0 {
			chTop = topLeft
			chBottom = bottomLeft
		} else if x == boxWidth-1 {
			chTop = topRight
			chBottom = bottomRight
		}
		s.SetContent(x0+x, y0, chTop, nil, borderStyle)
		s.SetContent(x0+x, y0+boxHeight-1, chBottom, nil, borderStyle)
	}
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, vLine, nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, vLine, nil, borderStyle)
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, itemStyle)
		}
	}
	if innerWidth > 0 {
		label := make([]rune, 0, innerWidth)
		label = append(label, ' ')
		label = append(label, titleRunes...)
		label = append(label, ' ')
		if len(label) > innerWidth {
			label = titleRunes
			if len(label) > innerWidth {
				label = label[:innerWidth]
			}
		}
		for i, r := range label {
			s.SetContent(x0+1+i, y0, r, nil, borderStyle)
		}
	}

	start := e.branchPickerIndex - listHeight/2
	maxStart := len(e.branchPickerItems) - listHeight
	if maxStart < 0 {
		maxStart = 0
	}
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}

	for i := 0; i < listHeight; i++ {
		idx := start + i
		if idx >= len(e.branchPickerItems) {
			break
		}
		style := itemStyle
		if idx == e.branchPickerIndex {
			style = selectedStyle
		}
		lineY := y0 + 1 + i
		for x := 0; x < innerWidth; x++ {
			s.SetContent(x0+1+x, lineY, ' ', nil, style)
		}
		runes := []rune(e.branchPickerItems[idx])
		if len(runes) > innerWidth {
			runes = runes[:innerWidth]
		}
		for i, r := range runes {
			s.SetContent(x0+1+i, lineY, r, nil, style)
		}
	}
}

func (e *Editor) renderSpaceMenu(s tcell.Screen, w, viewHeight int) {
	if !e.spaceMenuActive {
		return
	}
	if w < 20 || viewHeight < 5 {
		return
	}

	// Find the maximum label width
	maxLabelWidth := 0
	for _, item := range SpaceMenuItems {
		labelWidth := len(item.Label) + 6 // "x   Label"
		if labelWidth > maxLabelWidth {
			maxLabelWidth = labelWidth
		}
	}

	// Box dimensions
	boxWidth := maxLabelWidth + 4
	if boxWidth > w-4 {
		boxWidth = w - 4
	}
	innerWidth := boxWidth - 2

	listHeight := len(SpaceMenuItems)
	if listHeight > viewHeight-3 {
		listHeight = viewHeight - 3
	}
	boxHeight := listHeight + 2

	// Position at bottom right, above status line
	x0 := w - boxWidth - 1
	if x0 < 0 {
		x0 = 0
	}
	y0 := viewHeight - boxHeight
	if y0 < 0 {
		y0 = 0
	}

	borderStyle := e.styleStatus
	itemStyle := e.styleCommand
	dimStyle := e.styleLineNumber // for unimplemented items

	// Draw border
	topLeft := '┌'
	topRight := '┐'
	bottomLeft := '└'
	bottomRight := '┘'
	hLine := '─'
	vLine := '│'

	// Top border with title
	title := "Space"
	titleRunes := []rune(title)

	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = topLeft
		} else if x == boxWidth-1 {
			ch = topRight
		}
		s.SetContent(x0+x, y0, ch, nil, borderStyle)
	}

	// Embed title in top border
	if len(titleRunes)+2 <= boxWidth-2 {
		for i, r := range titleRunes {
			s.SetContent(x0+1+i, y0, r, nil, borderStyle)
		}
	}

	// Bottom border
	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = bottomLeft
		} else if x == boxWidth-1 {
			ch = bottomRight
		}
		s.SetContent(x0+x, y0+boxHeight-1, ch, nil, borderStyle)
	}

	// Side borders and content
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, vLine, nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, vLine, nil, borderStyle)

		// Clear interior
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, itemStyle)
		}
	}

	// Draw menu items
	for i := 0; i < listHeight; i++ {
		if i >= len(SpaceMenuItems) {
			break
		}
		item := SpaceMenuItems[i]
		lineY := y0 + 1 + i

		// Choose style based on whether item is implemented
		style := itemStyle
		if !item.Implemented {
			style = dimStyle
		}

		// Clear line
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, lineY, ' ', nil, style)
		}

		// Format: " k   Label text"
		keyStr := string(item.Key)
		label := " " + keyStr + "   " + item.Label

		runes := []rune(label)
		if len(runes) > innerWidth {
			runes = runes[:innerWidth]
		}

		for j, r := range runes {
			s.SetContent(x0+1+j, lineY, r, nil, style)
		}
	}
}

// renderMenu renders a generic mode menu popup
func (e *Editor) renderMenu(s tcell.Screen, w, viewHeight int, title string, items []SpaceMenuItem) {
	if w < 20 || viewHeight < 5 {
		return
	}

	// Find the maximum label width
	maxLabelWidth := 0
	for _, item := range items {
		labelWidth := len(item.Label) + 6 // "x   Label"
		if labelWidth > maxLabelWidth {
			maxLabelWidth = labelWidth
		}
	}

	// Box dimensions
	boxWidth := maxLabelWidth + 4
	if boxWidth > w-4 {
		boxWidth = w - 4
	}
	innerWidth := boxWidth - 2

	listHeight := len(items)
	if listHeight > viewHeight-3 {
		listHeight = viewHeight - 3
	}
	boxHeight := listHeight + 2

	// Position at bottom right, above status line
	x0 := w - boxWidth - 1
	if x0 < 0 {
		x0 = 0
	}
	y0 := viewHeight - boxHeight
	if y0 < 0 {
		y0 = 0
	}

	borderStyle := e.styleStatus
	itemStyle := e.styleCommand
	dimStyle := e.styleLineNumber

	// Draw border
	topLeft := '┌'
	topRight := '┐'
	bottomLeft := '└'
	bottomRight := '┘'
	hLine := '─'
	vLine := '│'

	// Top border with title
	titleRunes := []rune(title)

	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = topLeft
		} else if x == boxWidth-1 {
			ch = topRight
		}
		s.SetContent(x0+x, y0, ch, nil, borderStyle)
	}

	// Embed title in top border
	if len(titleRunes)+2 <= boxWidth-2 {
		for i, r := range titleRunes {
			s.SetContent(x0+1+i, y0, r, nil, borderStyle)
		}
	}

	// Bottom border
	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = bottomLeft
		} else if x == boxWidth-1 {
			ch = bottomRight
		}
		s.SetContent(x0+x, y0+boxHeight-1, ch, nil, borderStyle)
	}

	// Side borders and content
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, vLine, nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, vLine, nil, borderStyle)

		// Clear interior
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, itemStyle)
		}
	}

	// Draw menu items
	for i := 0; i < listHeight; i++ {
		if i >= len(items) {
			break
		}
		item := items[i]
		lineY := y0 + 1 + i

		// Choose style based on whether item is implemented
		style := itemStyle
		if !item.Implemented {
			style = dimStyle
		}

		// Clear line
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, lineY, ' ', nil, style)
		}

		// Format: " k   Label text"
		keyStr := string(item.Key)
		label := " " + keyStr + "   " + item.Label

		runes := []rune(label)
		if len(runes) > innerWidth {
			runes = runes[:innerWidth]
		}

		for j, r := range runes {
			s.SetContent(x0+1+j, lineY, r, nil, style)
		}
	}
}

// renderKeybindingsHelp renders a help popup showing all keybindings
func (e *Editor) renderKeybindingsHelp(s tcell.Screen, w, viewHeight int) {
	if w < 40 || viewHeight < 10 {
		return
	}

	// Build keybinding list
	type keybinding struct {
		key    string
		action string
		desc   string
	}
	var bindings []keybinding

	// Add header
	bindings = append(bindings, keybinding{"Key", "Action", "Description"})
	bindings = append(bindings, keybinding{"───", "──────", "───────────"})

	// Add normal mode bindings
	bindingDescs := map[string]string{
		"move_left":          "Move cursor left",
		"move_right":         "Move cursor right",
		"move_up":            "Move cursor up",
		"move_down":          "Move cursor down",
		"word_left":          "Move to previous word",
		"word_right":         "Move to next word",
		"line_start":         "Move to line start",
		"line_end":           "Move to line end",
		"file_start":         "Move to file start",
		"file_end":           "Move to file end",
		"page_up":            "Page up",
		"page_down":          "Page down",
		"enter_insert":       "Enter insert mode",
		"enter_command":      "Enter command mode",
		"quit":               "Quit editor",
		"undo":               "Undo last change",
		"redo":               "Redo last change",
		"delete":             "Delete selection/char",
		"change":             "Delete and enter insert",
		"yank":               "Yank (copy) selection",
		"paste":              "Paste after cursor",
		"paste_before":       "Paste before cursor",
		"open_below":         "Open line below",
		"open_above":         "Open line above",
		"append":             "Append after cursor",
		"append_line_end":    "Append at line end",
		"insert_line_start":  "Insert at line start",
		"join_lines":         "Join lines",
		"toggle_select":      "Toggle select mode",
		"extend_line":        "Extend to full line",
		"collapse_selection": "Collapse selection",
		"select_all":         "Select all",
		"indent":             "Indent line(s)",
		"unindent":           "Unindent line(s)",
		"word_forward":       "Move to next word",
		"word_backward":      "Move to previous word",
		"word_end":           "Move to word end",
		"goto_mode":          "Enter goto mode",
		"match_mode":         "Enter match mode",
		"view_mode":          "Enter view mode",
		"space_mode":         "Open space menu",
		"find_char":          "Find char forward",
		"find_char_backward": "Find char backward",
		"till_char":          "Till char forward",
		"till_char_backward": "Till char backward",
		"search_forward":     "Search forward",
		"search_backward":    "Search backward",
		"search_next":        "Next search match",
		"search_prev":        "Previous search match",
		"replace_char":       "Replace char under cursor",
		"delete_line":        "Delete current line",
		"scroll_up":          "Scroll up",
		"scroll_down":        "Scroll down",
		"branch_picker":      "Open branch picker",
		"insert_line_above":  "Insert line at cursor",
		"toggle_line_numbers":"Toggle line numbers",
	}

	for key, action := range e.keymap.normal {
		desc := bindingDescs[action]
		if desc == "" {
			desc = action
		}
		bindings = append(bindings, keybinding{key, action, desc})
	}

	// Box dimensions
	boxWidth := w - 4
	if boxWidth > 80 {
		boxWidth = 80
	}
	boxHeight := viewHeight - 2
	if boxHeight > len(bindings)+2 {
		boxHeight = len(bindings) + 2
	}
	innerWidth := boxWidth - 2
	listHeight := boxHeight - 2

	// Center the popup
	x0 := (w - boxWidth) / 2
	y0 := (viewHeight - boxHeight) / 2

	borderStyle := e.styleStatus
	contentStyle := e.styleCommand
	headerStyle := e.styleStatus

	// Draw border
	for x := 0; x < boxWidth; x++ {
		ch := '─'
		if x == 0 {
			ch = '┌'
		} else if x == boxWidth-1 {
			ch = '┐'
		}
		s.SetContent(x0+x, y0, ch, nil, borderStyle)
		ch = '─'
		if x == 0 {
			ch = '└'
		} else if x == boxWidth-1 {
			ch = '┘'
		}
		s.SetContent(x0+x, y0+boxHeight-1, ch, nil, borderStyle)
	}

	// Title
	title := "Keybindings (j/k to scroll, q/Esc to close)"
	titleRunes := []rune(title)
	for i, r := range titleRunes {
		if i+1 < boxWidth-1 {
			s.SetContent(x0+1+i, y0, r, nil, borderStyle)
		}
	}

	// Side borders and clear interior
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, '│', nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, '│', nil, borderStyle)
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, contentStyle)
		}
	}

	// Draw bindings
	for i := 0; i < listHeight; i++ {
		idx := i + e.keybindingsHelpScroll
		if idx >= len(bindings) {
			break
		}
		b := bindings[idx]
		lineY := y0 + 1 + i

		style := contentStyle
		if idx < 2 {
			style = headerStyle
		}

		// Format columns
		keyCol := fmt.Sprintf("%-12s", b.key)
		actionCol := fmt.Sprintf("%-20s", b.action)
		descCol := b.desc

		line := " " + keyCol + " " + actionCol + " " + descCol
		runes := []rune(line)
		if len(runes) > innerWidth {
			runes = runes[:innerWidth]
		}

		for j, r := range runes {
			s.SetContent(x0+1+j, lineY, r, nil, style)
		}
	}

	// Scroll indicator
	if len(bindings) > listHeight {
		scrollInfo := fmt.Sprintf(" %d/%d ", e.keybindingsHelpScroll+1, len(bindings)-listHeight+1)
		infoRunes := []rune(scrollInfo)
		startX := x0 + boxWidth - len(infoRunes) - 1
		for i, r := range infoRunes {
			s.SetContent(startX+i, y0+boxHeight-1, r, nil, borderStyle)
		}
	}
}

func keyString(ev *tcell.EventKey) string {
	// Handle alt+shift+arrow combinations first
	if ev.Modifiers()&tcell.ModAlt != 0 && ev.Modifiers()&tcell.ModShift != 0 {
		switch ev.Key() {
		case tcell.KeyUp:
			return "alt+shift+up"
		case tcell.KeyDown:
			return "alt+shift+down"
		case tcell.KeyLeft:
			return "alt+shift+left"
		case tcell.KeyRight:
			return "alt+shift+right"
		}
	}
	// Handle alt+arrow combinations
	if ev.Modifiers()&tcell.ModAlt != 0 {
		switch ev.Key() {
		case tcell.KeyUp:
			return "alt+up"
		case tcell.KeyDown:
			return "alt+down"
		case tcell.KeyLeft:
			return "alt+left"
		case tcell.KeyRight:
			return "alt+right"
		}
	}
	if ev.Modifiers()&tcell.ModCtrl != 0 {
		switch ev.Key() {
		case tcell.KeyHome:
			return "ctrl+home"
		case tcell.KeyEnd:
			return "ctrl+end"
		}
	}
	if ev.Modifiers()&tcell.ModMeta != 0 {
		if ev.Key() == tcell.KeyRune {
			r := ev.Rune()
			if ev.Modifiers()&tcell.ModShift != 0 {
				if r == ' ' {
					return "cmd+shift+space"
				}
				return "cmd+shift+" + strings.ToLower(string(r))
			}
			if r == ' ' {
				return "cmd+space"
			}
			return "cmd+" + strings.ToLower(string(r))
		}
		switch ev.Key() {
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			return "cmd+backspace"
		case tcell.KeyEnter:
			return "cmd+enter"
		case tcell.KeyLeft:
			if ev.Modifiers()&tcell.ModShift != 0 {
				return "cmd+shift+left"
			}
			return "cmd+left"
		case tcell.KeyRight:
			if ev.Modifiers()&tcell.ModShift != 0 {
				return "cmd+shift+right"
			}
			return "cmd+right"
		case tcell.KeyUp:
			if ev.Modifiers()&tcell.ModShift != 0 {
				return "cmd+shift+up"
			}
			return "cmd+up"
		case tcell.KeyDown:
			if ev.Modifiers()&tcell.ModShift != 0 {
				return "cmd+shift+down"
			}
			return "cmd+down"
		case tcell.KeyHome:
			if ev.Modifiers()&tcell.ModShift != 0 {
				return "cmd+shift+home"
			}
			return "cmd+home"
		case tcell.KeyEnd:
			if ev.Modifiers()&tcell.ModShift != 0 {
				return "cmd+shift+end"
			}
			return "cmd+end"
		}
	}
	if ev.Key() == tcell.KeyRune {
		r := ev.Rune()
		if r == ' ' {
			return "space"
		}
		return string(r)
	}
	// Check Tab before ctrlKeyName since KeyTab == KeyCtrlI (0x09)
	switch ev.Key() {
	case tcell.KeyTab:
		if ev.Modifiers()&tcell.ModShift != 0 {
			return "shift+tab"
		}
		return "tab"
	case tcell.KeyBacktab:
		return "shift+tab"
	}
	if name := ctrlKeyName(ev.Key()); name != "" {
		return name
	}
	switch ev.Key() {
	case tcell.KeyUp:
		return "up"
	case tcell.KeyDown:
		return "down"
	case tcell.KeyLeft:
		return "left"
	case tcell.KeyRight:
		return "right"
	case tcell.KeyPgUp:
		return "pgup"
	case tcell.KeyPgDn:
		return "pgdn"
	case tcell.KeyHome:
		return "home"
	case tcell.KeyEnd:
		return "end"
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return "backspace"
	case tcell.KeyEnter:
		return "enter"
	case tcell.KeyDelete:
		return "del"
	case tcell.KeyEscape:
		return "esc"
	}
	return ""
}

func keyStringDisplay(ev *tcell.EventKey) string {
	var parts []string

	// Build modifier prefix in order: CMD, CTRL, SHIFT, ALT
	if ev.Modifiers()&tcell.ModMeta != 0 {
		parts = append(parts, "CMD")
	}
	if ev.Modifiers()&tcell.ModCtrl != 0 {
		parts = append(parts, "CTRL")
	}
	if ev.Modifiers()&tcell.ModShift != 0 {
		parts = append(parts, "SHIFT")
	}
	if ev.Modifiers()&tcell.ModAlt != 0 {
		parts = append(parts, "ALT")
	}

	// Get key name
	var keyName string
	if ev.Key() == tcell.KeyRune {
		r := ev.Rune()
		if r == ' ' {
			keyName = "SPACE"
		} else {
			keyName = strings.ToUpper(string(r))
		}
	} else {
		switch ev.Key() {
		case tcell.KeyUp:
			keyName = "UP"
		case tcell.KeyDown:
			keyName = "DOWN"
		case tcell.KeyLeft:
			keyName = "LEFT"
		case tcell.KeyRight:
			keyName = "RIGHT"
		case tcell.KeyPgUp:
			keyName = "PGUP"
		case tcell.KeyPgDn:
			keyName = "PGDN"
		case tcell.KeyHome:
			keyName = "HOME"
		case tcell.KeyEnd:
			keyName = "END"
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			keyName = "BKSP"
		case tcell.KeyEnter:
			keyName = "ENTER"
		case tcell.KeyDelete:
			keyName = "DEL"
		case tcell.KeyEscape:
			keyName = "ESC"
		case tcell.KeyTab:
			keyName = "TAB"
		case tcell.KeyCtrlA:
			keyName = "A"
		case tcell.KeyCtrlB:
			keyName = "B"
		case tcell.KeyCtrlC:
			keyName = "C"
		case tcell.KeyCtrlD:
			keyName = "D"
		case tcell.KeyCtrlE:
			keyName = "E"
		case tcell.KeyCtrlF:
			keyName = "F"
		case tcell.KeyCtrlG:
			keyName = "G"
		case tcell.KeyCtrlH:
			keyName = "H"
		case tcell.KeyCtrlI:
			keyName = "I"
		case tcell.KeyCtrlJ:
			keyName = "J"
		case tcell.KeyCtrlK:
			keyName = "K"
		case tcell.KeyCtrlL:
			keyName = "L"
		case tcell.KeyCtrlM:
			keyName = "M"
		case tcell.KeyCtrlN:
			keyName = "N"
		case tcell.KeyCtrlO:
			keyName = "O"
		case tcell.KeyCtrlP:
			keyName = "P"
		case tcell.KeyCtrlQ:
			keyName = "Q"
		case tcell.KeyCtrlR:
			keyName = "R"
		case tcell.KeyCtrlS:
			keyName = "S"
		case tcell.KeyCtrlT:
			keyName = "T"
		case tcell.KeyCtrlU:
			keyName = "U"
		case tcell.KeyCtrlV:
			keyName = "V"
		case tcell.KeyCtrlW:
			keyName = "W"
		case tcell.KeyCtrlX:
			keyName = "X"
		case tcell.KeyCtrlY:
			keyName = "Y"
		case tcell.KeyCtrlZ:
			keyName = "Z"
		default:
			keyName = fmt.Sprintf("KEY%d", ev.Key())
		}
	}

	if keyName != "" {
		parts = append(parts, keyName)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "-")
}

func keyStringForMap(ev *tcell.EventKey, keymap map[string]string) string {
	if ev.Modifiers()&tcell.ModMeta != 0 {
		switch ev.Key() {
		case tcell.KeyHome:
			if _, ok := keymap["cmd+left"]; ok {
				return "cmd+left"
			}
		case tcell.KeyEnd:
			if _, ok := keymap["cmd+right"]; ok {
				return "cmd+right"
			}
		}
	}
	return keyString(ev)
}

func ctrlKeyName(key tcell.Key) string {
	switch key {
	case tcell.KeyCtrlA:
		return "ctrl+a"
	case tcell.KeyCtrlB:
		return "ctrl+b"
	case tcell.KeyCtrlC:
		return "ctrl+c"
	case tcell.KeyCtrlD:
		return "ctrl+d"
	case tcell.KeyCtrlE:
		return "ctrl+e"
	case tcell.KeyCtrlF:
		return "ctrl+f"
	case tcell.KeyCtrlG:
		return "ctrl+g"
	case tcell.KeyCtrlH:
		return "ctrl+h"
	case tcell.KeyCtrlI:
		return "ctrl+i"
	case tcell.KeyCtrlJ:
		return "ctrl+j"
	case tcell.KeyCtrlK:
		return "ctrl+k"
	case tcell.KeyCtrlL:
		return "ctrl+l"
	case tcell.KeyCtrlM:
		return "ctrl+m"
	case tcell.KeyCtrlN:
		return "ctrl+n"
	case tcell.KeyCtrlO:
		return "ctrl+o"
	case tcell.KeyCtrlP:
		return "ctrl+p"
	case tcell.KeyCtrlQ:
		return "ctrl+q"
	case tcell.KeyCtrlR:
		return "ctrl+r"
	case tcell.KeyCtrlS:
		return "ctrl+s"
	case tcell.KeyCtrlT:
		return "ctrl+t"
	case tcell.KeyCtrlU:
		return "ctrl+u"
	case tcell.KeyCtrlV:
		return "ctrl+v"
	case tcell.KeyCtrlW:
		return "ctrl+w"
	case tcell.KeyCtrlX:
		return "ctrl+x"
	case tcell.KeyCtrlY:
		return "ctrl+y"
	case tcell.KeyCtrlZ:
		return "ctrl+z"
	}
	return ""
}
