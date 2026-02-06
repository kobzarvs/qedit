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
		fileTreeShowHidden:      opts.FileTreeShowHidden,
		fileTreeShowIgnored:     opts.FileTreeShowIgnored,
		aiThinkingLevels:        append([]string(nil), opts.AIThinkingLevels...),
		aiThinkingLevelsByModel: copyStringSliceMap(opts.AIThinkingLevelsByModel),
		aiPanelDefaultWidth:     opts.AIPanelWidth,
		buffers:                 NewBufferManager(),
	}
	e.SetStyles(defaultEditorStyles())
	return e
}

func copyStringSliceMap(src map[string][]string) map[string][]string {
	if src == nil {
		return nil
	}
	dst := make(map[string][]string, len(src))
	for k, v := range src {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		if v == nil {
			dst[key] = nil
			continue
		}
		copySlice := append([]string(nil), v...)
		dst[key] = copySlice
	}
	return dst
}
func (e *Editor) OpenFile(path string) error {
	absPath := path
	if abs, err := filepath.Abs(path); err == nil {
		absPath = abs
	}

	// Check if this file is already open in a buffer
	if e.buffers != nil && e.buffers.Count() > 0 {
		if idx := e.buffers.FindByPath(absPath); idx >= 0 {
			e.switchToBuffer(idx)
			return nil
		}
	}

	// Save current buffer state before opening new file
	if e.buffers != nil && e.buffers.Count() > 0 && e.filename != "" {
		e.saveSessionState()
		_ = e.SaveUndoHistory()
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
	}

	// Load the new file
	data, err := os.ReadFile(absPath)
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
	e.filename = absPath
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
	e.clipboard = nil
	e.selectionScopeStack = nil
	e.selectionScopeIndex = 0
	e.searchMatches = nil
	e.searchMatchIndex = 0
	e.updateDirty()
	_ = e.LoadUndoHistory()
	_ = e.syncFileSnapshot()
	e.externalChange = ExternalChangeNone

	// Restore session state
	e.restoreSessionState()

	// Add new buffer to manager
	if e.buffers != nil {
		bs := e.snapshotBufferState()
		newIdx := e.buffers.Add(bs)
		e.buffers.SetActive(newIdx)
	}

	return nil
}

// switchToBuffer switches to the buffer at the given index.
func (e *Editor) switchToBuffer(index int) {
	if e.buffers == nil || index < 0 || index >= e.buffers.Count() || index == e.buffers.ActiveIndex() {
		return
	}

	// Save current state
	e.saveSessionState()
	_ = e.SaveUndoHistory()
	bs := e.snapshotBufferState()
	e.buffers.UpdateActive(bs)

	// Switch
	e.buffers.SetActive(index)

	// Restore new buffer state
	e.restoreBufferState(e.buffers.Active())

	// Signal app.go
	e.bufferSwitched = true
}

// gotoNextBuffer switches to the next buffer.
func (e *Editor) gotoNextBuffer() {
	if e.buffers == nil || e.buffers.Count() <= 1 {
		e.setStatus("no other buffers")
		return
	}
	e.switchToBuffer(e.buffers.Next())
}

// gotoPrevBuffer switches to the previous buffer.
func (e *Editor) gotoPrevBuffer() {
	if e.buffers == nil || e.buffers.Count() <= 1 {
		e.setStatus("no other buffers")
		return
	}
	e.switchToBuffer(e.buffers.Prev())
}

// gotoLastAccessedBuffer switches to the previously accessed buffer.
func (e *Editor) gotoLastAccessedBuffer() {
	if e.buffers == nil {
		e.setStatus("no other buffers")
		return
	}
	prev := e.buffers.PrevIndex()
	if prev < 0 || prev >= e.buffers.Count() {
		e.setStatus("no last accessed buffer")
		return
	}
	e.switchToBuffer(prev)
}

// closeCurrentBuffer closes the current buffer. If force is false and the buffer
// is dirty, shows a warning instead.
func (e *Editor) closeCurrentBuffer(force bool) {
	if e.buffers == nil || e.buffers.Count() <= 1 {
		e.setStatus("last buffer (use :q to quit)")
		return
	}
	if !force && e.dirty {
		e.setStatus("unsaved changes (use :bc!)")
		return
	}

	idx := e.buffers.ActiveIndex()
	count := e.buffers.Count()

	// Determine which buffer to switch to after removal.
	// If we're closing the last buffer in the list, go to the previous one.
	// Otherwise, stay at the same index (which will point to the next buffer after removal).
	nextIdx := idx
	if idx >= count-1 {
		nextIdx = idx - 1
	}
	if nextIdx < 0 {
		nextIdx = 0
	}

	// Remove the buffer. This shifts indices >= idx down by 1.
	e.buffers.Remove(idx)

	// After removal, adjust nextIdx if it was shifted.
	if nextIdx > idx {
		nextIdx--
	}

	// Explicitly set the active buffer and restore its state.
	e.buffers.SetActive(nextIdx)
	e.restoreBufferState(e.buffers.Active())
	e.bufferSwitched = true
}

// closeBufferAtIndex closes a buffer at the given index (called from sidebar).
// If it's the active buffer, switches to another one.
func (e *Editor) closeBufferAtIndex(index int) {
	if e.buffers == nil || e.buffers.Count() <= 1 {
		e.setStatus("last buffer (use :q to quit)")
		return
	}
	activeIdx := e.buffers.ActiveIndex()

	if index == activeIdx {
		// Closing the active buffer - same as closeCurrentBuffer(true)
		e.closeCurrentBuffer(true)
		return
	}

	// Closing a non-active buffer
	e.buffers.Remove(index)
}

// openSidebarBuffers opens the sidebar with the buffer list.
func (e *Editor) openSidebarBuffers() {
	if e.sidebar == nil {
		return
	}
	if e.refsPickerActive {
		e.closeRefsPicker(false)
	}
	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo(), e.aiManager != nil)
	}
	content := NewSidebarBuffersContent(e)
	// Position cursor on active buffer
	if e.buffers != nil {
		content.SetIndex(e.buffers.ActiveIndex())
	}
	e.sidebar.Open(content)
	e.clearFileTreePreview()
}

// ConsumeBufferSwitch returns true and resets the flag if a buffer switch occurred.
func (e *Editor) ConsumeBufferSwitch() bool {
	if !e.bufferSwitched {
		return false
	}
	e.bufferSwitched = false
	return true
}

// BufferCount returns the number of open buffers.
func (e *Editor) BufferCount() int {
	if e.buffers == nil {
		return 0
	}
	return e.buffers.Count()
}

// ActiveBufferIndex returns the index of the active buffer.
func (e *Editor) ActiveBufferIndex() int {
	if e.buffers == nil {
		return 0
	}
	return e.buffers.ActiveIndex()
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
	// Save all buffer session states
	if e.buffers != nil && e.buffers.Count() > 0 {
		// Update the active buffer first
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
		// Save session state for all buffers
		for _, info := range e.buffers.Items() {
			if info.Filename != "" && e.sessionStore != nil {
				// Temporarily set filename to save each buffer's session
				// (restoreSessionState/saveSessionState use e.filename)
				origFilename := e.filename
				bufState := e.buffers.buffers[info.Index]
				e.filename = bufState.filename
				e.cursor = bufState.cursor
				e.scroll = bufState.scroll
				e.scrollX = bufState.scrollX
				e.mode = bufState.mode
				e.selectionActive = bufState.selectionActive
				e.selectionStart = bufState.selectionStart
				e.selectionEnd = bufState.selectionEnd
				e.saveSessionState()
				e.filename = origFilename
			}
		}
	} else {
		e.saveSessionState()
	}
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
	name = strings.TrimSpace(name)
	if e.gitBranch == name {
		return
	}
	e.gitBranch = name
	if e.sidebar == nil {
		return
	}
	if content, ok := e.sidebar.Content.(*SidebarBranchesContent); ok {
		content.SetCurrent(name)
	}
	if content, ok := e.sidebar.Content.(*SidebarWorktreesContent); ok {
		content.SetCurrentBranch(name)
	}
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
func (e *Editor) SetAIMarkdownHighlightFunc(fn MarkdownHighlightFunc) {
	e.aiMarkdownHighlight = fn
}
func (e *Editor) SetStatusMessage(msg string) {
	e.setStatus(msg)
	e.Notify(msg)
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
