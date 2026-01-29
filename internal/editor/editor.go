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
	"unicode"

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
}

type Cursor struct {
	Row int
	Col int
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
	branchPickerActive     bool
	branchPickerItems      []string
	branchPickerIndex      int
	branchPickerRequested  bool
	branchPickerSelection  string
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
	e.highlights = nil
	e.highlightStart = -1
	e.highlightEnd = -1
	e.updateDirty()
	return nil
}

func (e *Editor) HandleKey(ev *tcell.EventKey) bool {
	if e.mode != ModeCommand && e.statusMessage != "" {
		e.statusMessage = ""
	}
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
	e.ensureCursorVisible(viewHeight)

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
	if e.mode != ModeCommand && e.mode != ModeBranchPicker {
		cy = e.cursor.Row - e.scroll
		if cy < 0 {
			cy = 0
		}
		if cy >= viewHeight {
			cy = viewHeight - 1
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
	if e.mode == ModeBranchPicker {
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
	e.clearSelection()
	return e.execAction(action)
}

func (e *Editor) handleInsert(ev *tcell.EventKey) bool {
	if e.handleSelectionMove(ev) {
		return false
	}
	key := keyStringForMap(ev, e.keymap.insert)
	if key != "" {
		if action, ok := e.keymap.insert[key]; ok {
			e.clearSelection()
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
	e.changeTick++
	e.updateDirty()
}

func (e *Editor) Undo() {
	if len(e.undo) == 0 {
		e.setStatus("nothing to undo")
		return
	}
	idx := len(e.undo) - 1
	act := e.undo[idx]
	e.undo = e.undo[:idx]
	inv, ok := e.applyAction(act)
	if !ok {
		e.setStatus("undo failed")
		return
	}
	e.redo = append(e.redo, inv)
	e.changeTick++
	e.updateDirty()
}

func (e *Editor) Redo() {
	if len(e.redo) == 0 {
		e.setStatus("nothing to redo")
		return
	}
	idx := len(e.redo) - 1
	act := e.redo[idx]
	e.redo = e.redo[:idx]
	inv, ok := e.applyAction(act)
	if !ok {
		e.setStatus("redo failed")
		return
	}
	e.undo = append(e.undo, inv)
	e.changeTick++
	e.updateDirty()
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
	e.undo = append(e.undo, act)
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

func (e *Editor) deleteRuneAt(pos Cursor) bool {
	if pos.Row < 0 || pos.Row >= len(e.lines) {
		return false
	}
	line := e.lines[pos.Row]
	if pos.Col < 0 || pos.Col >= len(line) {
		return false
	}
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
}

func (e *Editor) moveDown() {
	if e.cursor.Row >= len(e.lines)-1 {
		return
	}
	e.cursor.Row++
	e.clampCursorCol()
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
	e.cursor.Row = 0
	e.cursor.Col = 0
}

func (e *Editor) moveFileEnd() {
	if len(e.lines) == 0 {
		e.cursor.Row = 0
		e.cursor.Col = 0
		return
	}
	e.cursor.Row = len(e.lines) - 1
	e.cursor.Col = len(e.lines[e.cursor.Row])
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
}

func (e *Editor) pageUp() {
	height := e.viewHeightCached()
	if height < 1 {
		height = 1
	}
	e.cursor.Row -= height
	if e.cursor.Row < 0 {
		e.cursor.Row = 0
	}
	e.clampCursorCol()
}

func (e *Editor) pageDown() {
	height := e.viewHeightCached()
	if height < 1 {
		height = 1
	}
	e.cursor.Row += height
	if e.cursor.Row >= len(e.lines) {
		e.cursor.Row = len(e.lines) - 1
		if e.cursor.Row < 0 {
			e.cursor.Row = 0
		}
	}
	e.clampCursorCol()
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
	cursorX := len(cmdRunes)
	if len(cmdRunes) > w {
		start := len(cmdRunes) - w
		cmdRunes = cmdRunes[start:]
		cursorX = len(cmdRunes)
	}
	for x := 0; x < w; x++ {
		if x < len(cmdRunes) {
			s.SetContent(x, y, cmdRunes[x], nil, e.styleCommand)
			continue
		}
		s.SetContent(x, y, ' ', nil, e.styleCommand)
	}
	if cursorX < 0 {
		cursorX = 0
	}
	if cursorX >= w {
		cursorX = w - 1
	}
	return cursorX
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

func (e *Editor) clearSelection() {
	e.selectionActive = false
	e.selectionStart = Cursor{}
	e.selectionEnd = Cursor{}
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
	return true
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
	return digits + 1
}

func (e *Editor) drawLineWithGutter(s tcell.Screen, y, w, gutterWidth, lineIdx int) {
	if gutterWidth > 0 {
		digits := gutterWidth - 1
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
		for i, r := range numStr {
			if i >= gutterWidth-1 || i >= w {
				break
			}
			s.SetContent(i, y, r, nil, style)
		}
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
	case tcell.KeyEscape:
		return "esc"
	case tcell.KeyTab:
		return "tab"
	}
	return ""
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
