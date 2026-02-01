package editor

import (
	"os"
	"path/filepath"
	"strings"
)

func New(opts Options) *Editor {
	normal := make(map[string]string, len(opts.KeymapNormal))
	for k, v := range opts.KeymapNormal {
		normal[k] = v
	}
	insert := make(map[string]string, len(opts.KeymapInsert))
	for k, v := range opts.KeymapInsert {
		insert[k] = v
	}
	tabWidth := opts.TabWidth
	if tabWidth < 1 {
		tabWidth = 1
	}

	lineNumberMode := parseLineNumberMode(opts.LineNumbers)
	gitBranchSymbol := strings.TrimSpace(opts.GitBranchSymbol)

	e := &Editor{
		Buffer: Buffer{
			text:   NewTextBufferFromRunes(nil),
			cursor: Cursor{},
		},
		mode:                ModeNormal,
		keymap:              keymapSet{normal: normal, insert: insert},
		tabWidth:            tabWidth,
		lineNumberMode:      lineNumberMode,
		gitBranchSymbol:     gitBranchSymbol,
		autoReloadOnChanges: opts.AutoReloadOnChanges,
		highlightStart:      -1,
		highlightEnd:        -1,
		cmdHistoryPath:      opts.CmdHistoryPath,
		searchHistoryPath:   opts.SearchHistoryPath,
		sessionStore:        opts.SessionStore,
		sidebar: NewSidebar(
			opts.SidebarWidth,
			opts.SidebarMinWidth,
			opts.SidebarMaxWidth,
			opts.SidebarCloseOnSelect,
		),
	}
	e.SetStyles(defaultEditorStyles())
	return e
}
func (e *Editor) OpenFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	e.text = NewTextBufferFromBytes(data)
	e.cursor = Cursor{}
	e.diskContent = e.Content()
	e.resetConflictBlocks()
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
	_ = e.syncFileSnapshot()
	e.externalChange = ExternalChangeNone

	// Restore session state
	e.restoreSessionState()

	return nil
}
func (e *Editor) restoreSessionState() {
	if e.sessionStore == nil || e.filename == "" {
		return
	}
	absPath, err := filepath.Abs(e.filename)
	if err != nil {
		return
	}
	state, ok := e.sessionStore.GetFileState(absPath)
	if !ok {
		return
	}

	// Restore cursor (clamped to valid range)
	e.cursor.Row = state.CursorRow
	lineCount := e.LineCount()
	if e.cursor.Row >= lineCount {
		e.cursor.Row = lineCount - 1
	}
	if e.cursor.Row < 0 {
		e.cursor.Row = 0
	}
	e.cursor.Col = state.CursorCol
	if e.cursor.Row < lineCount && e.cursor.Col > e.text.LineLen(e.cursor.Row) {
		e.cursor.Col = e.text.LineLen(e.cursor.Row)
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
		if state.SelectionStartRow >= lineCount || state.SelectionEndRow >= lineCount {
			// File was shortened - reset selection
			e.selectionActive = false
		} else {
			e.selectionActive = true
			// Clamp columns to line lengths
			startCol := state.SelectionStartCol
			if startCol > e.text.LineLen(state.SelectionStartRow) {
				startCol = e.text.LineLen(state.SelectionStartRow)
			}
			endCol := state.SelectionEndCol
			if endCol > e.text.LineLen(state.SelectionEndRow) {
				endCol = e.text.LineLen(state.SelectionEndRow)
			}
			e.selectionStart = Cursor{Row: state.SelectionStartRow, Col: startCol}
			e.selectionEnd = Cursor{Row: state.SelectionEndRow, Col: endCol}
		}
	}
}
func (e *Editor) saveSessionState() {
	if e.sessionStore == nil || e.filename == "" {
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

	state := FileState{
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
	e.sessionStore.SetFileState(absPath, state)
}

// Shutdown saves session state and stops background tasks
func (e *Editor) Shutdown() {
	e.saveSessionState()
	if e.sessionStore != nil {
		e.sessionStore.Stop()
	}
}
func (e *Editor) Content() string {
	if e.text == nil {
		return ""
	}
	return e.text.String()
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
	if e.text == nil {
		return 1
	}
	return e.text.LineCount()
}
func (e *Editor) VisibleRange() (int, int) {
	lineCount := e.LineCount()
	if lineCount == 0 {
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
	if end >= lineCount {
		end = lineCount - 1
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
