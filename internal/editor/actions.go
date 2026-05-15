package editor

import (
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func (e *Editor) execAction(action string) bool {
	if e.bindings.actionHook != nil {
		e.bindings.actionHook(action)
	}
	if e.hugeFileActive() && !e.hugeFileAllowsAction(action) {
		e.setStatus("operation unavailable in huge file mode")
		return false
	}
	if e.gitDiffPreviewActive() && !e.gitDiffPreviewAllowsAction(action) {
		e.setStatus("diff preview is read-only")
		return false
	}
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
		e.openSidebarBranches()
	case actionWorktreeMenu:
		e.openSidebarWorktrees()
	case actionWorktreeNew:
		e.prefillCommand("worktree new ")
		return false
	case actionWorktreeSwitch:
		e.prefillCommand("worktree switch ")
		return false
	case actionWorktreeRemove:
		e.prefillCommand("worktree remove ")
		return false
	case actionWorktreeRefresh:
		e.refreshSidebarWorktrees()
	case actionOpenFileTree:
		e.openSidebarFileTree("")
	case actionToggleSidebar:
		e.toggleSidebar()
	case actionToggleSidebarFocus:
		e.toggleSidebarFocus()
	case actionFocusSidebar:
		e.focusSidebar()
	case actionFocusPrevPane:
		e.focusPrevPane()
	case actionFocusNextPane:
		e.focusNextPane()
	case actionFocusEditor:
		e.focusEditor()
	case actionFocusCommandLine:
		e.focusCommandLine()
	case actionEnterInsert:
		e.mode = ModeInsert
		e.saveLineState()
	case actionEnterNormal:
		e.mode = ModeNormal
	case actionEnterCommand:
		e.mode = ModeCommand
		e.commandLine.text = e.commandLine.text[:0]
		e.commandLine.cursor = 0
		e.commandLine.historyIndex = -1
	case actionMergeMode:
		return e.enterMergeMode()
	case actionQuit:
		if e.buffers != nil && e.buffers.Count() > 0 {
			bs := e.snapshotBufferState()
			e.buffers.UpdateActive(bs)
			if e.buffers.HasDirtyBuffers() {
				e.setStatus("unsaved changes in open buffers (use :q!)")
				return false
			}
		} else if e.document.dirty {
			e.setStatus("unsaved changes (use :q!)")
			return false
		}
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
	case actionDeleteWordRight:
		e.deleteWordRight()
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
		e.modal.gotoMode = true
		e.modal.pendingKeys = "g"
		return false // Don't clear selection, wait for next key
	case actionGotoLine:
		e.gotoLastLine()
	case actionGotoLinePrompt:
		e.mode = ModeCommand
		e.commandLine.text = []rune{}
		e.commandLine.cursor = 0
		e.setStatus("goto line:")
	case actionGotoFirstLine:
		e.gotoFirstLine()
	case actionGotoFileEnd:
		e.gotoFileEnd()
	case actionFindChar:
		e.setPendingFindChar(action)
		e.modal.pendingKeys = "f"
		return false // Don't clear selection, wait for char input
	case actionFindCharBackward:
		e.setPendingFindChar(action)
		e.modal.pendingKeys = "F"
		return false
	case actionTillChar:
		e.setPendingFindChar(action)
		e.modal.pendingKeys = "t"
		return false
	case actionTillCharBackward:
		e.setPendingFindChar(action)
		e.modal.pendingKeys = "T"
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
		e.modal.pendingKeys = "r"
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
		e.modal.spaceMenuActive = true
		e.modal.pendingKeys = "SPC"
		return false

	// Match mode
	case actionMatchMode:
		e.modal.matchMode = true
		e.modal.pendingKeys = "m"
		return false

	// View mode
	case actionViewMode:
		e.modal.viewMode = true
		e.modal.pendingKeys = "z"
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

	// Git
	case actionGitNextChange:
		if e.mergeReviewActive() {
			return e.gotoMergeReviewConflict(true)
		}
		e.gotoGitChange(true)
	case actionGitPrevChange:
		if e.mergeReviewActive() {
			return e.gotoMergeReviewConflict(false)
		}
		e.gotoGitChange(false)

	// Special
	case actionInsertLineAbove:
		e.insertLineAboveCursor()

	// Terminal zoom
	case actionTerminalZoomIn:
		if e.zoom.pendingRestore {
			return false // already zoomed, ignore
		}
		// Save current scroll positions for restore
		e.zoom.savedScroll = e.viewport.scroll
		e.zoom.savedScrollX = e.viewport.scrollX
		e.zoomWithAnimation(true, 20) // zoom in with scroll animation
		e.zoom.pendingRestore = true
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
		if err := e.queueSaveRequest("", false); err != nil {
			e.setStatus(err.Error())
		}
		return false

	// Buffer management
	case actionBufferPicker:
		e.openSidebarBuffers()
		return false
	case actionGotoNextBuffer:
		e.gotoNextBuffer()
		return false
	case actionGotoPrevBuffer:
		e.gotoPrevBuffer()
		return false
	case actionGotoLastAccessed:
		e.gotoLastAccessedBuffer()
		return false
	case actionCloseBuffer:
		e.closeCurrentBuffer(false)
		return false
	}
	if !e.modal.selectMode {
		e.clearSelection()
	}
	return false
}

func (e *Editor) hugeFileAllowsAction(action string) bool {
	switch action {
	case actionMoveLeft, actionMoveRight, actionMoveUp, actionMoveDown,
		actionWordLeft, actionWordRight, actionLineStart, actionLineEnd,
		actionFileStart, actionFileEnd, actionPageUp, actionPageDown,
		actionToggleLineNumbers,
		actionEnterInsert, actionEnterNormal,
		actionBackspace, actionNewline, actionDeleteChar,
		actionInsertTab, actionIndent, actionUnindent,
		actionDeleteLine, actionDelete, actionChange,
		actionYank, actionPaste, actionPasteBefore,
		actionAppend, actionAppendLineEnd, actionInsertLineStart,
		actionUndo, actionRedo, actionSave,
		actionOpenBelow, actionOpenAbove, actionInsertLineAbove,
		actionSearchForward, actionSearchBackward,
		actionSearchFuzzy, actionSearchRegex,
		actionSearchNext, actionSearchPrev,
		actionBranchPicker, actionWorktreeMenu, actionWorktreeRefresh,
		actionOpenFileTree, actionToggleSidebar, actionToggleSidebarFocus,
		actionFocusSidebar, actionFocusPrevPane, actionFocusNextPane,
		actionFocusEditor, actionFocusCommandLine, actionEnterCommand,
		actionQuit, actionScrollUp, actionScrollDown, actionGotoMode,
		actionGotoLine, actionGotoLinePrompt, actionGotoFirstLine,
		actionGotoFileEnd, actionBufferPicker, actionGotoNextBuffer,
		actionGotoPrevBuffer, actionGotoLastAccessed, actionCloseBuffer:
		return true
	default:
		return false
	}
}
func (e *Editor) Save(path string) error {
	path, data, err := e.prepareSave(path)
	if err != nil {
		return err
	}
	if e.workspaceFileStore() == nil {
		return errFileStoreUnavailable()
	}
	if e.hugeFileActive() {
		return e.WriteHugeFile(path, e.workspaceFileStore())
	}
	if err := e.workspaceWrite(path, data); err != nil {
		return err
	}
	e.ApplySavedFile(path)
	return nil
}
func (e *Editor) FormatGo() error {
	src := e.Content()
	if !e.hasWorkspaceFormatter() {
		return errors.New("formatter unavailable")
	}
	formatted, err := e.workspaceFormatGo(src)
	if err != nil {
		return err
	}
	if formatted == src {
		return nil
	}
	e.replaceBuffer(formatted, true)
	return nil
}
func (e *Editor) FormatCurrent() error {
	if isMarkdownFile(e.document.filename) {
		return e.FormatMarkdownTables()
	}
	if isGoFile(e.document.filename) {
		return e.FormatGo()
	}
	if isPrettierFile(e.document.filename) {
		return e.queueFormatRequest()
	}
	if e.document.filename == "" && looksLikeGo(e.Content()) {
		return e.FormatGo()
	}
	return errors.New("format not supported")
}
func isGoFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".go"
}
func isPrettierFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts", ".json", ".css", ".scss", ".less", ".html", ".htm", ".vue":
		return true
	default:
		return false
	}
}
func looksLikeGo(src string) bool {
	src = strings.TrimLeftFunc(src, unicode.IsSpace)
	return strings.HasPrefix(src, "package ")
}
func isMarkdownFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	default:
		return false
	}
}
func (e *Editor) FormatMarkdownTables() error {
	src := e.Content()
	lines := strings.Split(src, "\n")
	if len(lines) == 0 {
		return nil
	}
	changed := false
	i := 0
	inFence := false
	for i < len(lines) {
		if isFenceLine(lines[i]) {
			inFence = !inFence
			i++
			continue
		}
		if inFence {
			i++
			continue
		}
		if !lineHasPipe(lines[i]) {
			i++
			continue
		}
		sepIdx := findTableSeparator(lines, i)
		if sepIdx != -1 && sepIdx > 0 {
			start := sepIdx - 1
			end := sepIdx + 1
			for end < len(lines) && lineHasPipe(lines[end]) && strings.TrimSpace(lines[end]) != "" {
				end++
			}
			if start < 0 || start >= end {
				i++
				continue
			}
			prefix := leadingWhitespace(lines[start])
			block := lines[start:end]
			formatted := formatMarkdownTableBlock(block, prefix)
			if formatted == nil {
				i = end
				continue
			}
			for j := start; j < end; j++ {
				if lines[j] != formatted[j-start] {
					lines[j] = formatted[j-start]
					changed = true
				}
			}
			i = end
			continue
		}

		start := i
		end := i + 1
		for end < len(lines) && lineHasPipe(lines[end]) && strings.TrimSpace(lines[end]) != "" {
			end++
		}
		if end-start < 2 {
			i++
			continue
		}
		block := lines[start:end]
		if !isPipeTableBlock(block) {
			i++
			continue
		}
		prefix := leadingWhitespace(lines[start])
		formatted := formatMarkdownTableBlockNoSeparator(block, prefix)
		if formatted == nil {
			i = end
			continue
		}
		for j := start; j < end; j++ {
			if lines[j] != formatted[j-start] {
				lines[j] = formatted[j-start]
				changed = true
			}
		}
		i = end
	}
	if !changed {
		return nil
	}
	e.replaceBuffer(strings.Join(lines, "\n"), true)
	return nil
}
func isFenceLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}
func lineHasPipe(line string) bool {
	return strings.Contains(line, "|")
}
func findTableSeparator(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		if isTableSeparator(lines[i]) {
			return i
		}
		if strings.TrimSpace(lines[i]) == "" && i > start {
			return -1
		}
	}
	return -1
}
func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if !strings.Contains(trimmed, "-") || !strings.Contains(trimmed, "|") {
		return false
	}
	if strings.HasPrefix(trimmed, "|") {
		trimmed = trimmed[1:]
	}
	if strings.HasSuffix(trimmed, "|") {
		trimmed = trimmed[:len(trimmed)-1]
	}
	parts := strings.Split(trimmed, "|")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		cell := strings.TrimSpace(part)
		if cell == "" {
			continue
		}
		if !onlyTableSepChars(cell) {
			return false
		}
		if strings.Count(cell, "-") < 3 {
			return false
		}
	}
	return true
}
func onlyTableSepChars(cell string) bool {
	for _, r := range cell {
		if r != '-' && r != ':' {
			return false
		}
	}
	return true
}
func leadingWhitespace(line string) string {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[:i]
}
func formatMarkdownTableBlock(lines []string, prefix string) []string {
	if len(lines) < 2 {
		return nil
	}
	sepIdx := -1
	for i, line := range lines {
		if isTableSeparator(line) {
			sepIdx = i
			break
		}
	}
	if sepIdx <= 0 {
		return nil
	}
	rows := make([][]string, len(lines))
	maxCols := 0
	for i, line := range lines {
		rows[i] = splitTableRow(line)
		if len(rows[i]) > maxCols {
			maxCols = len(rows[i])
		}
	}
	if maxCols == 0 {
		return nil
	}
	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], "")
		}
	}
	widths := make([]int, maxCols)
	for i, row := range rows {
		if i == sepIdx {
			continue
		}
		for c, cell := range row {
			w := runeLen(cell)
			if w > widths[c] {
				widths[c] = w
			}
		}
	}
	aligns := make([]tableAlign, maxCols)
	for c, cell := range rows[sepIdx] {
		s := strings.TrimSpace(cell)
		if strings.HasPrefix(s, ":") {
			aligns[c].left = true
		}
		if strings.HasSuffix(s, ":") {
			aligns[c].right = true
		}
	}
	out := make([]string, len(lines))
	for i, row := range rows {
		cells := make([]string, maxCols)
		if i == sepIdx {
			for c := 0; c < maxCols; c++ {
				width := widths[c]
				if width < 3 {
					width = 3
				}
				switch {
				case aligns[c].left && aligns[c].right:
					dashes := strings.Repeat("-", max(1, width-2))
					cells[c] = ":" + dashes + ":"
				case aligns[c].left:
					dashes := strings.Repeat("-", max(1, width-1))
					cells[c] = ":" + dashes
				case aligns[c].right:
					dashes := strings.Repeat("-", max(1, width-1))
					cells[c] = dashes + ":"
				default:
					cells[c] = strings.Repeat("-", width)
				}
			}
		} else {
			for c, cell := range row {
				padding := widths[c] - runeLen(cell)
				if padding < 0 {
					padding = 0
				}
				cells[c] = cell + strings.Repeat(" ", padding)
			}
		}
		out[i] = prefix + "| " + strings.Join(cells, " | ") + " |"
	}
	return out
}
func formatMarkdownTableBlockNoSeparator(lines []string, prefix string) []string {
	if len(lines) < 2 {
		return nil
	}
	rows := make([][]string, len(lines))
	maxCols := 0
	for i, line := range lines {
		rows[i] = splitTableRow(line)
		if len(rows[i]) > maxCols {
			maxCols = len(rows[i])
		}
	}
	if maxCols < 2 {
		return nil
	}
	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], "")
		}
	}
	widths := make([]int, maxCols)
	for _, row := range rows {
		for c, cell := range row {
			w := runeLen(cell)
			if w > widths[c] {
				widths[c] = w
			}
		}
	}
	out := make([]string, len(lines))
	for i, row := range rows {
		cells := make([]string, maxCols)
		for c, cell := range row {
			padding := widths[c] - runeLen(cell)
			if padding < 0 {
				padding = 0
			}
			cells[c] = cell + strings.Repeat(" ", padding)
		}
		out[i] = prefix + "| " + strings.Join(cells, " | ") + " |"
	}
	return out
}
func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "|") {
		trimmed = trimmed[1:]
	}
	if strings.HasSuffix(trimmed, "|") {
		trimmed = trimmed[:len(trimmed)-1]
	}
	parts := strings.Split(trimmed, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}
func runeLen(s string) int {
	return len([]rune(s))
}

func isPipeTableBlock(lines []string) bool {
	if len(lines) < 2 {
		return false
	}
	colCount := -1
	for _, line := range lines {
		row := splitTableRow(line)
		if len(row) < 2 {
			return false
		}
		if colCount == -1 {
			colCount = len(row)
		} else if len(row) != colCount {
			return false
		}
	}
	return true
}

// sendTerminalZoomStep sends a single zoom command to the terminal via the zoomer.
func (e *Editor) sendTerminalZoomStep(zoomIn bool) {
	e.terminalZoomStep(zoomIn)
}

// zoomWithAnimation performs zoom with synchronized scroll animation.
// For zoom in: scrolls to center cursor. For zoom out: scrolls back to saved position.
func (e *Editor) zoomWithAnimation(zoomIn bool, steps int) {
	if e.zoom.animating {
		return // already animating, ignore
	}
	e.zoom.animating = true
	defer func() { e.zoom.animating = false }()

	// Calculate start and target scroll positions
	startScroll := e.viewport.scroll
	startScrollX := e.viewport.scrollX

	var targetScroll, targetScrollX int
	if zoomIn {
		// Target: center cursor on screen
		// Use current viewport, will be approximate but close enough
		targetScroll = e.cursor.Row - e.viewport.height/2
		targetScrollX = e.cursor.Col - e.viewport.width/2

		// Clamp targets
		if targetScroll < 0 {
			targetScroll = 0
		}
		maxScroll := e.LineCount() - e.viewport.height
		if maxScroll < 0 {
			maxScroll = 0
		}
		if targetScroll > maxScroll {
			targetScroll = maxScroll
		}
		if targetScrollX < 0 {
			targetScrollX = 0
		}
	} else {
		// Target: restore saved scroll positions
		targetScroll = e.zoom.savedScroll
		targetScrollX = e.zoom.savedScrollX
	}

	// Perform animated zoom + scroll
	for i := 1; i <= steps; i++ {
		// Send one zoom step
		e.sendTerminalZoomStep(zoomIn)

		// Interpolate scroll position (linear)
		progress := float64(i) / float64(steps)
		e.viewport.scroll = startScroll + int(float64(targetScroll-startScroll)*progress)
		e.viewport.scrollX = startScrollX + int(float64(targetScrollX-startScrollX)*progress)

		// Small delay between steps for smoothness
		time.Sleep(15 * time.Millisecond)
	}

	// Ensure final position is exact
	e.viewport.scroll = targetScroll
	e.viewport.scrollX = targetScrollX
}
