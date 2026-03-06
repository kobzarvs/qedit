package editor

import "strconv"

func (e *Editor) handleVimProfileKey(ev EventKey) bool {
	switch e.mode {
	case ModeCommand:
		return e.handleCommand(ev)
	case ModeBranchPicker:
		return e.handleBranchPicker(ev)
	case ModeSearch:
		return e.handleSearch(ev)
	case ModeMerge:
		return e.handleMerge(ev)
	case ModeInsert:
		if ev.Key() == KeyEscape {
			e.mode = ModeNormal
			e.resetVimPendingState()
			return false
		}
		return e.handleInsert(ev)
	default:
		if handled, quit := e.handleCommonProfileOverlays(ev); handled {
			return quit
		}
		if e.modal.pendingAction != "" {
			return e.handleVimPendingChar(ev)
		}
		if e.profile.vim.visual {
			return e.handleVimVisual(ev)
		}
		return e.handleVimNormal(ev)
	}
}

func (e *Editor) handleCommonProfileOverlays(ev EventKey) (bool, bool) {
	if e.zoom.pendingRestore {
		if ev.Key() == KeyRune {
			switch ev.Rune() {
			case ' ':
				e.zoomWithAnimation(false, 20)
				e.zoom.pendingRestore = false
				return true, false
			case '=':
				e.zoomWithAnimation(true, 20)
				return true, false
			}
		}
		return true, false
	}

	if e.refsPicker.active {
		if handled := e.handleRefsPicker(ev); handled {
			return true, false
		}
	}

	if e.keybindingsHelp.active {
		return true, e.handleKeybindingsHelp(ev)
	}

	return false, false
}

func (e *Editor) handleVimPendingChar(ev EventKey) bool {
	pendingKey := e.modal.pendingKeys
	e.modal.pendingKeys = ""
	if ev.Key() == KeyEscape {
		e.modal.pendingAction = ""
		e.resetVimPendingState()
		return false
	}
	if ev.Key() == KeyRune {
		e.handlePendingChar(ev.Rune())
		e.modal.lastCommand = pendingKey + string(ev.Rune())
		e.resetVimPendingState()
		return false
	}
	return false
}

func (e *Editor) handleVimNormal(ev EventKey) bool {
	if ev.Key() == KeyEscape {
		e.resetVimPendingState()
		return false
	}
	if ev.Key() == KeyRune {
		return e.handleVimNormalRune(ev.Rune())
	}
	return e.handleVimFallbackAction(ev)
}

func (e *Editor) handleVimVisual(ev EventKey) bool {
	if ev.Key() == KeyEscape {
		e.exitVimVisualMode(true)
		return false
	}
	if ev.Key() == KeyRune {
		if e.handleVimCountRune(ev.Rune()) {
			return false
		}
		switch ev.Rune() {
		case 'v':
			e.exitVimVisualMode(true)
			return false
		case 'y':
			e.yankSelection()
			e.exitVimVisualMode(false)
			return false
		case 'd':
			if start, end, ok := e.selectionRange(); ok {
				e.deleteSelection(start, end, true)
			}
			e.exitVimVisualMode(false)
			return false
		case 'c':
			if start, end, ok := e.selectionRange(); ok {
				e.deleteSelection(start, end, true)
			}
			e.exitVimVisualMode(false)
			e.mode = ModeInsert
			e.saveLineState()
			return false
		}
		before := e.cursor
		if e.applyVimMotionRune(ev.Rune(), e.consumeVimCount()) {
			e.selectionActive = true
			e.selectionEnd = e.cursor
			if before == e.cursor && e.selectionStart == e.selectionEnd {
				e.selectionEnd.Col++
			}
			return false
		}
	}
	return e.handleVimFallbackAction(ev)
}

func (e *Editor) handleVimNormalRune(r rune) bool {
	if e.profile.vim.pendingGoto {
		return e.handleVimPendingGoto(r)
	}
	if e.profile.vim.operator != "" {
		return e.handleVimOperatorRune(r)
	}
	if e.handleVimCountRune(r) {
		return false
	}

	switch r {
	case 'h', 'j', 'k', 'l', 'w', 'b', 'e', '$':
		e.applyVimMotionRune(r, e.consumeVimCount())
		return false
	case '0':
		e.moveLineStart()
		e.resetVimPendingState()
		return false
	case 'g':
		e.profile.vim.pendingGoto = true
		return false
	case 'G':
		count := e.consumeVimCount()
		if count > 1 {
			e.gotoLineNumber(count)
		} else {
			e.gotoLastLine()
		}
		return false
	case 'i':
		e.mode = ModeInsert
		e.saveLineState()
		e.resetVimPendingState()
		return false
	case 'a':
		e.appendMode()
		e.resetVimPendingState()
		return false
	case 'A':
		e.appendLineEnd()
		e.resetVimPendingState()
		return false
	case 'I':
		e.insertLineStart()
		e.resetVimPendingState()
		return false
	case 'o':
		e.openBelow()
		e.resetVimPendingState()
		return false
	case 'O':
		e.openAbove()
		e.resetVimPendingState()
		return false
	case 'x':
		for i := 0; i < e.consumeVimCount(); i++ {
			e.deleteChar()
		}
		return false
	case 'u':
		e.Undo()
		e.resetVimPendingState()
		return false
	case 'v':
		e.enterVimVisualMode()
		return false
	case 'd', 'c', 'y':
		e.profile.vim.operator = string(r)
		e.profile.vim.operatorCount = e.profile.vim.count
		e.profile.vim.count = ""
		return false
	case 'p':
		e.pasteAfter()
		e.resetVimPendingState()
		return false
	case 'P':
		e.pasteBefore()
		e.resetVimPendingState()
		return false
	case '/':
		e.enterSearchMode(true, false, false)
		e.resetVimPendingState()
		return false
	case '?':
		e.enterSearchMode(false, false, false)
		e.resetVimPendingState()
		return false
	case 'n':
		e.searchNext()
		e.resetVimPendingState()
		return false
	case 'N':
		e.searchPrev()
		e.resetVimPendingState()
		return false
	case ':':
		e.mode = ModeCommand
		e.commandLine.text = e.commandLine.text[:0]
		e.commandLine.cursor = 0
		e.commandLine.historyIndex = -1
		e.resetVimPendingState()
		return false
	case 'f':
		e.setPendingFindChar(actionFindChar)
		e.modal.pendingKeys = "f"
		return false
	case 'F':
		e.setPendingFindChar(actionFindCharBackward)
		e.modal.pendingKeys = "F"
		return false
	case 't':
		e.setPendingFindChar(actionTillChar)
		e.modal.pendingKeys = "t"
		return false
	case 'T':
		e.setPendingFindChar(actionTillCharBackward)
		e.modal.pendingKeys = "T"
		return false
	case 'r':
		e.setPendingFindChar(actionReplaceChar)
		e.modal.pendingKeys = "r"
		return false
	case 'J':
		for i := 0; i < e.consumeVimCount(); i++ {
			e.joinLinesCmd()
		}
		return false
	}

	e.resetVimPendingState()
	return false
}

func (e *Editor) handleVimPendingGoto(r rune) bool {
	e.profile.vim.pendingGoto = false
	switch r {
	case 'g':
		count := e.consumeVimCount()
		if count > 1 {
			e.gotoLineNumber(count)
		} else {
			e.gotoFirstLine()
		}
		return false
	}
	e.resetVimPendingState()
	return false
}

func (e *Editor) handleVimOperatorRune(r rune) bool {
	operator := e.profile.vim.operator
	if operator == "" {
		return false
	}
	if r == rune(operator[0]) {
		count := e.consumeVimOperatorCount()
		switch operator {
		case "d":
			e.vimDeleteLines(count)
		case "c":
			e.vimDeleteLines(count)
			e.mode = ModeInsert
			e.saveLineState()
		case "y":
			e.vimYankLines(count)
		}
		e.resetVimPendingState()
		return false
	}

	start := e.cursor
	switch r {
	case 'j', 'k', 'G':
		count := e.consumeVimOperatorCount()
		if r == 'G' {
			if count > 1 {
				e.gotoLineNumber(count)
			} else {
				e.gotoLastLine()
			}
		} else {
			e.applyVimMotionRune(r, count)
		}
		e.vimApplyLinewiseOperator(operator, start.Row, e.cursor.Row)
		e.resetVimPendingState()
		return false
	case 'g':
		e.profile.vim.pendingGoto = true
		return false
	}

	count := e.consumeVimOperatorCount()
	if !e.applyVimMotionRune(r, count) {
		e.resetVimPendingState()
		return false
	}
	end := e.cursor
	if r == 'l' || r == 'e' {
		end = e.advanceCursorOne(end)
	}
	e.vimApplyOperatorRange(operator, start, end)
	e.resetVimPendingState()
	return false
}

func (e *Editor) handleVimFallbackAction(ev EventKey) bool {
	key := keyStringForMap(ev, e.bindings.keymap.normal)
	if key == "" {
		return false
	}
	action, ok := e.bindings.keymap.normal[key]
	if !ok {
		return false
	}
	if isHelixSelectingMotion(action) || action == actionToggleSelect || action == actionGotoMode || action == actionMatchMode || action == actionViewMode {
		return false
	}
	e.resetVimPendingState()
	return e.execAction(action)
}

func (e *Editor) handleVimCountRune(r rune) bool {
	if r >= '1' && r <= '9' {
		e.profile.vim.count += string(r)
		return true
	}
	if r == '0' && e.profile.vim.count != "" {
		e.profile.vim.count += "0"
		return true
	}
	return false
}

func (e *Editor) consumeVimCount() int {
	count := 1
	if e.profile.vim.count != "" {
		if parsed, err := strconv.Atoi(e.profile.vim.count); err == nil && parsed > 0 {
			count = parsed
		}
	}
	e.profile.vim.count = ""
	return count
}

func (e *Editor) consumeVimOperatorCount() int {
	operatorCount := 1
	motionCount := 1
	if e.profile.vim.operatorCount != "" {
		if parsed, err := strconv.Atoi(e.profile.vim.operatorCount); err == nil && parsed > 0 {
			operatorCount = parsed
		}
	}
	if e.profile.vim.count != "" {
		if parsed, err := strconv.Atoi(e.profile.vim.count); err == nil && parsed > 0 {
			motionCount = parsed
		}
	}
	e.profile.vim.operatorCount = ""
	e.profile.vim.count = ""
	return operatorCount * motionCount
}

func (e *Editor) resetVimPendingState() {
	e.profile.vim.operator = ""
	e.profile.vim.count = ""
	e.profile.vim.operatorCount = ""
	e.profile.vim.pendingGoto = false
}

func (e *Editor) enterVimVisualMode() {
	e.profile.vim.visual = true
	e.selectionActive = true
	e.selectionStart = e.cursor
	e.selectionEnd = e.cursor
	e.resetVimPendingState()
}

func (e *Editor) exitVimVisualMode(clear bool) {
	e.profile.vim.visual = false
	e.resetVimPendingState()
	if clear {
		e.clearSelection()
	}
}

func (e *Editor) applyVimMotionRune(r rune, count int) bool {
	if count < 1 {
		count = 1
	}
	switch r {
	case 'h':
		for i := 0; i < count; i++ {
			e.moveLeft()
		}
	case 'j':
		for i := 0; i < count; i++ {
			e.moveDown()
		}
	case 'k':
		for i := 0; i < count; i++ {
			e.moveUp()
		}
	case 'l':
		for i := 0; i < count; i++ {
			e.moveRight()
		}
	case 'w':
		for i := 0; i < count; i++ {
			e.wordForward()
		}
	case 'b':
		for i := 0; i < count; i++ {
			e.wordBackward()
		}
	case 'e':
		for i := 0; i < count; i++ {
			e.wordEnd()
		}
	case '$':
		e.moveLineEnd()
	default:
		return false
	}
	return true
}

func (e *Editor) vimApplyOperatorRange(operator string, start, end Cursor) {
	e.selectionStart = start
	e.selectionEnd = end
	e.selectionActive = true
	switch operator {
	case "y":
		e.clipboard.linewise = false
		e.yankSelection()
	case "d":
		if s, en, ok := e.selectionRange(); ok {
			e.deleteSelection(s, en, true)
		}
	case "c":
		if s, en, ok := e.selectionRange(); ok {
			e.deleteSelection(s, en, true)
		}
		e.mode = ModeInsert
		e.saveLineState()
	}
	e.clearSelection()
}

func (e *Editor) vimApplyLinewiseOperator(operator string, startRow, endRow int) {
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	count := endRow - startRow + 1
	if count < 1 {
		count = 1
	}
	e.cursor.Row = startRow
	e.cursor.Col = 0
	switch operator {
	case "y":
		e.vimYankLines(count)
	case "d":
		e.vimDeleteLines(count)
	case "c":
		e.vimDeleteLines(count)
		e.mode = ModeInsert
		e.saveLineState()
	}
}

func (e *Editor) vimDeleteLines(count int) {
	if count < 1 {
		count = 1
	}
	for i := 0; i < count; i++ {
		e.deleteLine()
		if e.cursor.Row >= e.LineCount() {
			e.cursor.Row = e.LineCount() - 1
			if e.cursor.Row < 0 {
				e.cursor.Row = 0
			}
		}
	}
	e.cursor.Col = 0
}

func (e *Editor) vimYankLines(count int) {
	if count < 1 || e.LineCount() == 0 {
		return
	}
	start := e.cursor.Row
	end := start + count - 1
	if end >= e.LineCount() {
		end = e.LineCount() - 1
	}
	e.clipboard.lines = e.clipboard.lines[:0]
	e.clipboard.linewise = true
	for row := start; row <= end; row++ {
		e.clipboard.lines = append(e.clipboard.lines, append([]rune(nil), e.line(row)...))
	}
	e.copyToSystemClipboard(false)
}

func (e *Editor) advanceCursorOne(pos Cursor) Cursor {
	if pos.Row < 0 || pos.Row >= e.LineCount() {
		return pos
	}
	lineLen := e.lineLen(pos.Row)
	if pos.Col < lineLen {
		pos.Col++
	}
	return pos
}
