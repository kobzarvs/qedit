package editor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
)

type actionKind int

const (
	actionInsertRune actionKind = iota
	actionDeleteRune
	actionSplitLine
	actionJoinLine
	actionMoveLine
)

type action struct {
	kind    actionKind
	pos     Cursor
	r       rune
	rowFrom int
	rowTo   int
	group   uint64
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

type Editor struct {
	lines                  [][]rune
	cursor                 Cursor
	scroll                 int
	mode                   Mode
	filename               string
	dirty                  bool
	keymap                 keymapSet
	cmd                    []rune
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
	return nil
}

func (e *Editor) HandleKey(ev *tcell.EventKey) bool {
	e.freeScroll = false
	if e.mode != ModeCommand && e.statusMessage != "" {
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
	if statusY >= 0 {
		e.renderStatusline(s, w, statusY)
	}
	if cmdY >= 0 {
		cmdCursor := e.renderCommandline(s, w, cmdY)
		if e.mode == ModeCommand {
			cx = cmdCursor
			cy = cmdY
		}
	}
	cursorVisible := true
	if e.mode != ModeCommand && e.mode != ModeBranchPicker {
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
	if e.mode == ModeBranchPicker || !cursorVisible {
		s.HideCursor()
		s.Show()
		return
	}
	cursorStyle := tcell.CursorStyleSteadyBlock
	if e.mode == ModeInsert {
		cursorStyle = tcell.CursorStyleSteadyBar
	}
	s.SetCursorStyle(cursorStyle)
	s.ShowCursor(cx, cy)
	s.Show()
}

func (e *Editor) handleNormal(ev *tcell.EventKey) bool {
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
	return e.execAction(action)
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
		return false
	case tcell.KeyCtrlC:
		e.mode = ModeNormal
		e.cmd = e.cmd[:0]
		return false
	case tcell.KeyEnter:
		cmd := strings.TrimSpace(string(e.cmd))
		e.mode = ModeNormal
		e.cmd = e.cmd[:0]
		return e.execCommand(cmd)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(e.cmd) > 0 {
			e.cmd = e.cmd[:len(e.cmd)-1]
		}
		return false
	case tcell.KeyRune:
		e.cmd = append(e.cmd, ev.Rune())
		return false
	}
	return false
}

func (e *Editor) handleSelectionMove(ev *tcell.EventKey) bool {
	if ev.Modifiers()&tcell.ModShift == 0 {
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
	case actionRedo:
		e.Redo()
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
	}
	e.clearSelection()
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

func (e *Editor) setStatus(msg string) {
	e.statusMessage = msg
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
		// Calculate byte offset BEFORE changes
		deletedBytes := runeSliceByteLen(line)
		e.lastEdit = TextEdit{
			Valid:          true,
			StartByte:      0,
			OldEndByte:     deletedBytes,
			NewEndByte:     0,
			StartRow:       0,
			StartColBytes:  0,
			OldEndRow:      0,
			OldEndColBytes: deletedBytes,
			NewEndRow:      0,
			NewEndColBytes: 0,
		}
		// Record undo for each character as a group
		e.startUndoGroup()
		for i := len(line) - 1; i >= 0; i-- {
			e.appendUndo(action{kind: actionInsertRune, pos: Cursor{Row: 0, Col: i}, r: line[i]})
		}
		e.finishUndoGroup()
		e.lines[0] = []rune{}
		e.cursor.Col = 0
		return
	}

	// Calculate byte offsets BEFORE making changes
	startByte, startColBytes := e.byteOffset(Cursor{Row: row, Col: 0})
	lineBytes := runeSliceByteLen(line)

	var oldEndByte int
	var oldEndRow int
	var oldEndColBytes int
	var newEndRow int
	var newEndColBytes int

	if row < len(e.lines)-1 {
		// Not the last line: delete from start of line to start of next line
		oldEndByte = startByte + lineBytes + 1 // +1 for newline
		oldEndRow = row + 1
		oldEndColBytes = 0
		newEndRow = row
		newEndColBytes = 0
	} else {
		// Last line: delete from end of previous line (newline) to end of this line
		prevLineBytes := runeSliceByteLen(e.lines[row-1])
		startByte = startByte - 1 // include the newline before this line
		startColBytes = prevLineBytes
		oldEndByte = startByte + 1 + lineBytes // newline + line content
		oldEndRow = row
		oldEndColBytes = lineBytes
		newEndRow = row - 1
		newEndColBytes = prevLineBytes
	}

	startRow := row
	if row >= len(e.lines)-1 && row > 0 {
		startRow = row - 1
	}

	e.lastEdit = TextEdit{
		Valid:          true,
		StartByte:      startByte,
		OldEndByte:     oldEndByte,
		NewEndByte:     startByte, // Nothing inserted
		StartRow:       startRow,
		StartColBytes:  startColBytes,
		OldEndRow:      oldEndRow,
		OldEndColBytes: oldEndColBytes,
		NewEndRow:      newEndRow,
		NewEndColBytes: newEndColBytes,
	}

	// Record undo: first the line join, then the characters as a group
	e.startUndoGroup()
	if row < len(e.lines)-1 {
		e.appendUndo(action{kind: actionSplitLine, pos: Cursor{Row: row, Col: 0}})
	} else {
		e.appendUndo(action{kind: actionSplitLine, pos: Cursor{Row: row - 1, Col: len(e.lines[row-1])}})
	}
	for i := len(line) - 1; i >= 0; i-- {
		e.appendUndo(action{kind: actionInsertRune, pos: Cursor{Row: row, Col: i}, r: line[i]})
	}
	e.finishUndoGroup()

	// Actually remove the line
	newLines := make([][]rune, 0, len(e.lines)-1)
	newLines = append(newLines, e.lines[:row]...)
	newLines = append(newLines, e.lines[row+1:]...)
	e.lines = newLines

	// Adjust cursor
	if row >= len(e.lines) {
		e.cursor.Row = len(e.lines) - 1
		if e.cursor.Row < 0 {
			e.cursor.Row = 0
		}
	}
	e.cursor.Col = 0
	e.clampCursorCol()
	e.changeTick++
	e.updateDirty()
}

func (e *Editor) deleteChar() {
	// If there's a selection, delete the selected text
	if start, end, ok := e.selectionRange(); ok {
		e.deleteSelection(start, end)
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

func (e *Editor) deleteSelection(start, end Cursor) {
	if start.Row < 0 || end.Row >= len(e.lines) {
		return
	}

	// Calculate byte offsets BEFORE making changes
	startByte, startColBytes := e.byteOffset(start)
	oldEndByte, oldEndColBytes := e.byteOffset(end)

	// Collect deleted content for undo (from end to start) as a group
	e.startUndoGroup()
	// First, record line joins for multi-line selections
	for row := end.Row; row > start.Row; row-- {
		joinPos := Cursor{Row: row - 1, Col: len(e.lines[row-1])}
		e.appendUndo(action{kind: actionSplitLine, pos: joinPos})
	}

	// Then record character deletions
	for row := end.Row; row >= start.Row; row-- {
		line := e.lines[row]
		startCol := 0
		endCol := len(line)
		if row == start.Row {
			startCol = start.Col
		}
		if row == end.Row {
			endCol = end.Col
		}
		for col := endCol - 1; col >= startCol; col-- {
			if col >= 0 && col < len(line) {
				e.appendUndo(action{kind: actionInsertRune, pos: Cursor{Row: row, Col: col}, r: line[col]})
			}
		}
	}
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
	cmdRunes := e.cmd
	if e.mode == ModeCommand {
		cmdRunes = append([]rune{':'}, e.cmd...)
	}

	// Prepare right side: last key combo
	rightText := ""
	if e.lastKeyCombo != "" {
		rightText = " " + e.lastKeyCombo + " "
	}
	rightRunes := []rune(rightText)
	rightStart := w - len(rightRunes)
	if rightStart < 0 {
		rightStart = 0
		rightRunes = rightRunes[:w]
	}

	// Calculate available width for command
	availableWidth := rightStart
	if availableWidth < 0 {
		availableWidth = 0
	}

	cursorX := len(cmdRunes)
	if len(cmdRunes) > availableWidth {
		start := len(cmdRunes) - availableWidth
		cmdRunes = cmdRunes[start:]
		cursorX = len(cmdRunes)
	}

	// Draw command line content
	for x := 0; x < w; x++ {
		if x < len(cmdRunes) {
			s.SetContent(x, y, cmdRunes[x], nil, e.styleCommand)
		} else if x >= rightStart && x-rightStart < len(rightRunes) {
			s.SetContent(x, y, rightRunes[x-rightStart], nil, e.styleCommand)
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

func (e *Editor) drawLine(s tcell.Screen, y, w, startX int, line []rune, tabWidth int, selStart, selEnd int, spans []HighlightSpan, highlightActive bool) {
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
		activeStyle := fallbackStyle
		if selStart >= 0 && selEnd > selStart && idx >= selStart && idx < selEnd {
			activeStyle = e.styleSelection
		} else if kind, ok := highlightKindAt(spans, idx); ok {
			if style, ok := e.styleForHighlight(kind); ok {
				activeStyle = style
			}
		} else if highlightActive && !isWordRune(r) {
			activeStyle = e.styleMain
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
	e.drawLine(s, y, w, gutterWidth, e.lines[lineIdx], e.tabWidth, selStart, selEnd, spans, highlightActive)
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

func keyString(ev *tcell.EventKey) string {
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
