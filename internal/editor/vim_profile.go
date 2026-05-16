package editor

import (
	"fmt"
	"strconv"
	"unicode"
)

func (e *Editor) handleVimProfileKey(ev EventKey) bool {
	e.vimRecordRepeatEvent(ev)
	if !(e.mode == ModeNormal && ev.Key() == KeyRune && ev.Rune() == 'q') {
		e.vimRecordMacroEvent(ev)
	}
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
			e.profile.vim.replace = false
			e.resetVimPendingState()
			e.vimFinishRepeatRecording()
			return false
		}
		if e.profile.vim.replace {
			return e.handleVimReplace(ev)
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

	if e.modal.windowMode {
		e.modal.windowMode = false
		if ev.Key() == KeyEscape {
			e.modal.pendingKeys = ""
			e.modal.windowNewPending = false
			return true, false
		}
		if ev.Key() == KeyRune {
			return true, e.handleWindowKey(ev.Rune())
		}
		if ev.Key() == KeyCtrlW {
			return true, e.handleWindowKey('w')
		}
		e.modal.pendingKeys = ""
		e.modal.windowNewPending = false
		return true, false
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
		if e.modal.pendingAction == actionReplaceChar {
			e.saveLineState()
		}
		pendingAction := e.modal.pendingAction
		e.handleVimPendingCharAction(ev.Rune())
		e.modal.lastCommand = pendingKey + string(ev.Rune())
		e.resetVimPendingState()
		if pendingAction == actionReplaceChar {
			e.vimFinishRepeatRecording()
		}
		return false
	}
	return false
}

func (e *Editor) handleVimPendingCharAction(ch rune) bool {
	switch e.modal.pendingAction {
	case actionFindChar:
		e.modal.lastFindChar = ch
		e.modal.lastFindForward = true
		e.modal.lastFindTill = false
		e.modal.pendingAction = ""
		return e.findCharForward(ch, false)
	case actionFindCharBackward:
		e.modal.lastFindChar = ch
		e.modal.lastFindForward = false
		e.modal.lastFindTill = false
		e.modal.pendingAction = ""
		return e.findCharBackward(ch, false)
	case actionTillChar:
		e.modal.lastFindChar = ch
		e.modal.lastFindForward = true
		e.modal.lastFindTill = true
		e.modal.pendingAction = ""
		return e.findCharForward(ch, true)
	case actionTillCharBackward:
		e.modal.lastFindChar = ch
		e.modal.lastFindForward = false
		e.modal.lastFindTill = true
		e.modal.pendingAction = ""
		return e.findCharBackward(ch, true)
	case actionReplaceChar:
		e.modal.pendingAction = ""
		return e.replaceCharAtCursor(ch)
	case actionVimSetMark:
		e.modal.pendingAction = ""
		e.vimSetMark(ch)
		return false
	case actionVimGotoMarkLine:
		e.modal.pendingAction = ""
		return e.vimGotoMark(ch, false)
	case actionVimGotoMark:
		e.modal.pendingAction = ""
		return e.vimGotoMark(ch, true)
	case actionVimStartMacro:
		e.modal.pendingAction = ""
		e.vimStartMacroRecording(ch)
		return false
	case actionVimReplayMacro:
		e.modal.pendingAction = ""
		e.vimReplayMacro(ch, e.consumeVimCount())
		return false
	default:
		e.modal.pendingAction = ""
		return false
	}
}

func (e *Editor) showVimFileInfo() {
	name := e.document.filename
	if name == "" {
		name = e.document.title
	}
	if name == "" {
		name = "[No Name]"
	}
	lineCount := e.LineCount()
	if lineCount < 1 {
		lineCount = 1
	}
	line := e.cursor.Row + 1
	if line < 1 {
		line = 1
	}
	if line > lineCount {
		line = lineCount
	}
	percent := 100
	if lineCount > 1 {
		percent = (line * 100) / lineCount
	}
	modified := ""
	if e.document.dirty {
		modified = " [Modified]"
	}
	e.setStatus(fmt.Sprintf("\"%s\"%s line %d of %d --%d%%-- col %d", name, modified, line, lineCount, percent, e.cursor.Col+1))
}

func (e *Editor) handleVimNormal(ev EventKey) bool {
	if ev.Key() == KeyEscape {
		e.resetVimPendingState()
		return false
	}
	if ev.Key() == KeyCtrlO {
		e.jumpBackward()
		e.resetVimPendingState()
		return false
	}
	if ev.Key() == KeyCtrlI {
		e.jumpForward()
		e.resetVimPendingState()
		return false
	}
	if ev.Key() == KeyCtrlS {
		e.saveJumpPosition()
		e.resetVimPendingState()
		return false
	}
	if ev.Key() == KeyCtrlR {
		e.Redo()
		e.resetVimPendingState()
		return false
	}
	if ev.Key() == KeyCtrlG {
		e.showVimFileInfo()
		e.resetVimPendingState()
		return false
	}
	if ev.Key() == KeyRune && ev.Modifiers() == 0 {
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
		case 'V':
			if e.profile.vim.visualLine {
				e.exitVimVisualMode(true)
			} else {
				e.enterVimVisualLineMode()
			}
			return false
		case 'y':
			if e.profile.vim.visualLine {
				e.vimApplyVisualLineOperator("y")
				return false
			}
			e.yankSelection()
			e.exitVimVisualMode(false)
			return false
		case 'd':
			if e.profile.vim.visualLine {
				e.vimApplyVisualLineOperator("d")
				return false
			}
			if start, end, ok := e.selectionRange(); ok {
				e.deleteSelection(start, end, true)
			}
			e.exitVimVisualMode(false)
			return false
		case 'c':
			if e.profile.vim.visualLine {
				e.vimApplyVisualLineOperator("c")
				return false
			}
			if start, end, ok := e.selectionRange(); ok {
				e.deleteSelection(start, end, true)
			}
			e.exitVimVisualMode(false)
			e.mode = ModeInsert
			e.saveLineState()
			return false
		case ':':
			e.mode = ModeCommand
			e.commandLine.text = []rune("'<,'>")
			e.commandLine.cursor = len(e.commandLine.text)
			e.commandLine.historyIndex = -1
			return false
		}
		before := e.cursor
		if e.applyVimMotionRune(ev.Rune(), e.consumeVimCount()) {
			e.selectionActive = true
			if e.profile.vim.visualLine {
				e.updateVimVisualLineSelection()
			} else {
				e.selectionEnd = e.cursor
			}
			if !e.profile.vim.visualLine && before == e.cursor && e.selectionStart == e.selectionEnd {
				e.selectionEnd.Col++
			}
			e.resetVimPendingState()
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
	if e.profile.vim.pendingRegister {
		e.vimSetActiveRegister(r)
		e.profile.vim.pendingRegister = false
		e.modal.pendingKeys += string(r)
		return false
	}
	if e.handleVimCountRune(r) {
		return false
	}

	switch r {
	case '"':
		e.profile.vim.pendingRegister = true
		e.modal.pendingKeys += string(r)
		return false
	case 'q':
		if e.profile.vim.macroRecording {
			e.vimStopMacroRecording()
			e.resetVimPendingState()
			return false
		}
		e.setPendingFindChar(actionVimStartMacro)
		e.modal.pendingKeys = "q"
		return false
	case '@':
		e.setPendingFindChar(actionVimReplayMacro)
		e.modal.pendingKeys = "@"
		return false
	case '.':
		count := e.consumeVimCount()
		e.resetVimPendingState()
		e.replayVimRepeat(count)
		return false
	case 'h', 'j', 'k', 'l', 'w', 'b', 'e', 'W', 'B', 'E', '(', ')', '{', '}', '$', '^':
		e.applyVimMotionRune(r, e.consumeVimCount())
		e.resetVimPendingState()
		return false
	case '0':
		e.moveLineStart()
		e.resetVimPendingState()
		return false
	case 'g':
		e.modal.pendingKeys += string(r)
		e.profile.vim.pendingGoto = true
		return false
	case 'm':
		e.setPendingFindChar(actionVimSetMark)
		e.modal.pendingKeys = "m"
		return false
	case '\'':
		e.setPendingFindChar(actionVimGotoMarkLine)
		e.modal.pendingKeys = "'"
		return false
	case '`':
		e.setPendingFindChar(actionVimGotoMark)
		e.modal.pendingKeys = "`"
		return false
	case 'G':
		count := e.consumeVimCount()
		before := e.cursor
		if count > 1 {
			e.gotoLineNumber(count)
		} else {
			e.gotoLastLine()
		}
		e.recordJump(before, e.cursor)
		e.resetVimPendingState()
		return false
	case 'i':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.mode = ModeInsert
		e.saveLineState()
		e.resetVimPendingState()
		return false
	case 'a':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.appendMode()
		e.resetVimPendingState()
		return false
	case 'A':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.appendLineEnd()
		e.resetVimPendingState()
		return false
	case 'I':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.insertLineStart()
		e.resetVimPendingState()
		return false
	case 'o':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.openBelow()
		e.resetVimPendingState()
		return false
	case 'O':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.openAbove()
		e.resetVimPendingState()
		return false
	case 'x':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.saveLineState()
		for i := 0; i < e.consumeVimCount(); i++ {
			e.deleteChar()
		}
		e.resetVimPendingState()
		e.vimFinishRepeatRecording()
		return false
	case 'X':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.saveLineState()
		e.vimDeleteBeforeCursor(e.consumeVimCount())
		e.resetVimPendingState()
		e.vimFinishRepeatRecording()
		return false
	case '~':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.vimToggleCase(e.consumeVimCount())
		e.resetVimPendingState()
		e.vimFinishRepeatRecording()
		return false
	case '%':
		e.goToMatchingBracket()
		e.resetVimPendingState()
		return false
	case 'D':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.vimDeleteToLineEnd()
		e.resetVimPendingState()
		e.vimFinishRepeatRecording()
		return false
	case 'C':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.vimChangeToLineEnd()
		e.resetVimPendingState()
		return false
	case 's':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.vimSubstituteChars(e.consumeVimCount())
		e.resetVimPendingState()
		return false
	case 'S':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.vimChangeLines(e.consumeVimCount())
		e.resetVimPendingState()
		return false
	case 'u':
		e.Undo()
		e.resetVimPendingState()
		return false
	case 'U':
		e.undoLine()
		e.resetVimPendingState()
		return false
	case 'v':
		e.enterVimVisualMode()
		return false
	case 'V':
		e.enterVimVisualLineMode()
		return false
	case 'Y':
		e.vimYankLines(e.consumeVimCount())
		e.resetVimPendingState()
		return false
	case 'd', 'c', 'y', '>', '<':
		if r != 'y' {
			e.vimStartRepeatRecording(e.vimCommandPrefix(string(r)))
		}
		e.profile.vim.operator = string(r)
		e.profile.vim.operatorStart = e.cursor
		e.profile.vim.operatorCount = e.profile.vim.count
		e.profile.vim.count = ""
		e.modal.pendingKeys += string(r)
		return false
	case 'p':
		e.vimStartRepeatRecording(e.vimCommandPrefix(string(r)))
		if e.vimLoadActiveRegister() {
			e.pasteAfter()
		}
		e.resetVimPendingState()
		e.vimFinishRepeatRecording()
		return false
	case 'P':
		e.vimStartRepeatRecording(e.vimCommandPrefix(string(r)))
		if e.vimLoadActiveRegister() {
			e.pasteBefore()
		}
		e.resetVimPendingState()
		e.vimFinishRepeatRecording()
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
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.setPendingFindChar(actionReplaceChar)
		e.modal.pendingKeys = "r"
		return false
	case 'R':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.mode = ModeInsert
		e.profile.vim.replace = true
		e.saveLineState()
		e.resetVimPendingState()
		return false
	case 'J':
		e.vimStartRepeatRecording(e.profile.vim.count + string(r))
		e.saveLineState()
		for i := 0; i < e.consumeVimCount(); i++ {
			e.joinLinesCmd()
		}
		e.resetVimPendingState()
		e.vimFinishRepeatRecording()
		return false
	}

	e.resetVimPendingState()
	return false
}

func (e *Editor) handleVimPendingGoto(r rune) bool {
	e.profile.vim.pendingGoto = false
	operator := e.profile.vim.operator
	switch r {
	case 'g':
		e.modal.pendingKeys += string(r)
		if operator != "" {
			start := e.profile.vim.operatorStart
			count := e.consumeVimOperatorCount()
			if count > 1 {
				e.gotoLineNumber(count)
			} else {
				e.gotoFirstLine()
			}
			e.vimApplyLinewiseOperator(operator, start.Row, e.cursor.Row)
			e.resetVimPendingState()
			e.vimFinishRepeatRecordingForOperator(operator)
			return false
		}
		count := e.consumeVimCount()
		before := e.cursor
		if count > 1 {
			e.gotoLineNumber(count)
		} else {
			e.gotoFirstLine()
		}
		e.recordJump(before, e.cursor)
		e.resetVimPendingState()
		return false
	case 'e', 'E':
		e.modal.pendingKeys += string(r)
		if operator != "" {
			start := e.profile.vim.operatorStart
			count := e.consumeVimOperatorCount()
			for i := 0; i < count; i++ {
				if r == 'E' {
					e.wordEndBackwardLong()
				} else {
					e.wordEndBackward()
				}
			}
			e.vimApplyOperatorRange(operator, start, e.cursor)
			e.resetVimPendingState()
			e.vimFinishRepeatRecordingForOperator(operator)
			return false
		}
		count := e.consumeVimCount()
		before := e.cursor
		for i := 0; i < count; i++ {
			if r == 'E' {
				e.wordEndBackwardLong()
			} else {
				e.wordEndBackward()
			}
		}
		e.recordJump(before, e.cursor)
		e.resetVimPendingState()
		return false
	case 'u', 'U', '~':
		e.modal.pendingKeys += string(r)
		if operator == "" {
			operator = "g" + string(r)
			e.vimStartRepeatRecording(e.vimCommandPrefix("g" + string(r)))
			e.profile.vim.operator = operator
			e.profile.vim.operatorStart = e.cursor
			e.profile.vim.operatorCount = e.profile.vim.count
			e.profile.vim.count = ""
			return false
		}
	}
	e.resetVimPendingState()
	return false
}

func (e *Editor) handleVimOperatorRune(r rune) bool {
	operator := e.profile.vim.operator
	if operator == "" {
		return false
	}
	if e.profile.vim.pendingTextObject {
		e.handleVimTextObjectRune(operator, r)
		return false
	}
	if e.handleVimCountRune(r) {
		return false
	}
	if r == 'i' || r == 'a' {
		e.profile.vim.pendingTextObject = true
		e.profile.vim.pendingTextObjectAround = r == 'a'
		e.modal.pendingKeys += string(r)
		return false
	}
	if e.vimOperatorRepeatsLinewise(operator, r) {
		count := e.consumeVimOperatorCount()
		switch operator {
		case "d":
			e.saveLineState()
			e.vimDeleteLines(count)
		case "c":
			e.vimChangeLines(count)
		case "y":
			e.vimYankLines(count)
		case ">":
			e.vimIndentLines(count)
		case "<":
			e.vimUnindentLines(count)
		case "gu", "gU", "g~":
			e.vimApplyCaseLinewiseOperator(operator, e.cursor.Row, e.cursor.Row+count-1)
		}
		e.resetVimPendingState()
		e.vimFinishRepeatRecordingForOperator(operator)
		return false
	}

	start := e.profile.vim.operatorStart
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
		e.vimFinishRepeatRecordingForOperator(operator)
		return false
	case 'g':
		e.modal.pendingKeys += string(r)
		e.profile.vim.pendingGoto = true
		return false
	case '\'':
		e.setPendingFindChar(actionVimGotoMarkLine)
		e.modal.pendingKeys += string(r)
		return false
	case '`':
		e.setPendingFindChar(actionVimGotoMark)
		e.modal.pendingKeys += string(r)
		return false
	}

	count := e.consumeVimOperatorCount()
	if operator == "c" && r == 'w' && count == 1 {
		e.wordEnd()
		end := e.advanceCursorOne(e.cursor)
		e.vimApplyOperatorRange(operator, start, end)
		e.resetVimPendingState()
		e.vimFinishRepeatRecordingForOperator(operator)
		return false
	}
	if !e.applyVimMotionRune(r, count) {
		e.resetVimPendingState()
		e.vimCancelRepeatRecordingForOperator(operator)
		return false
	}
	end := e.cursor
	if r == 'l' || r == 'e' {
		end = e.advanceCursorOne(end)
	}
	e.vimApplyOperatorRange(operator, start, end)
	e.resetVimPendingState()
	e.vimFinishRepeatRecordingForOperator(operator)
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
	if isHelixSelectingMotion(action) || action == actionToggleSelect || action == actionGotoMode || action == actionMatchMode || action == actionViewMode ||
		action == actionToggleCase || action == actionLowercase || action == actionUppercase {
		return false
	}
	e.resetVimPendingState()
	return e.execAction(action)
}

func (e *Editor) handleVimReplace(ev EventKey) bool {
	if ev.Key() != KeyRune {
		return e.handleInsert(ev)
	}
	e.clearSelection()
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return false
	}
	if e.cursor.Col < e.lineLen(e.cursor.Row) {
		e.replaceCharAtCursor(ev.Rune())
		return false
	}
	e.insertRune(ev.Rune())
	return false
}

func (e *Editor) handleVimCountRune(r rune) bool {
	if r >= '1' && r <= '9' {
		e.profile.vim.count += string(r)
		e.modal.pendingKeys += string(r)
		return true
	}
	if r == '0' && e.profile.vim.count != "" {
		e.profile.vim.count += "0"
		e.modal.pendingKeys += string(r)
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
	e.profile.vim.operatorStart = Cursor{}
	e.profile.vim.count = ""
	e.profile.vim.operatorCount = ""
	e.profile.vim.pendingGoto = false
	e.profile.vim.pendingTextObject = false
	e.profile.vim.pendingTextObjectAround = false
	e.profile.vim.pendingRegister = false
	e.profile.vim.registerName = 0
	e.modal.pendingKeys = ""
}

func (e *Editor) enterVimVisualMode() {
	e.profile.vim.visual = true
	e.profile.vim.visualLine = false
	e.selectionActive = true
	e.selectionStart = e.cursor
	e.selectionEnd = e.cursor
	e.resetVimPendingState()
}

func (e *Editor) exitVimVisualMode(clear bool) {
	e.profile.vim.visual = false
	e.profile.vim.visualLine = false
	e.profile.vim.visualAnchor = Cursor{}
	e.resetVimPendingState()
	if clear {
		e.clearSelection()
	}
}

func (e *Editor) enterVimVisualLineMode() {
	e.profile.vim.visual = true
	e.profile.vim.visualLine = true
	e.profile.vim.visualAnchor = e.cursor
	e.updateVimVisualLineSelection()
	e.resetVimPendingState()
}

func (e *Editor) updateVimVisualLineSelection() {
	startRow := e.profile.vim.visualAnchor.Row
	endRow := e.cursor.Row
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	if startRow < 0 {
		startRow = 0
	}
	if endRow >= e.LineCount() {
		endRow = e.LineCount() - 1
	}
	if startRow > endRow || e.LineCount() == 0 {
		e.clearSelection()
		return
	}
	e.selectionActive = true
	e.selectionStart = Cursor{Row: startRow, Col: 0}
	e.selectionEnd = e.vimLinewiseSelectionEnd(endRow)
}

func (e *Editor) vimLinewiseSelectionEnd(row int) Cursor {
	if row+1 < e.LineCount() {
		return Cursor{Row: row + 1, Col: 0}
	}
	return Cursor{Row: row, Col: e.lineLen(row)}
}

func (e *Editor) vimApplyVisualLineOperator(operator string) {
	start, end, ok := e.selectionRange()
	if !ok {
		return
	}
	startRow := start.Row
	endRow := end.Row
	if end.Col == 0 && end.Row > start.Row {
		endRow--
	}
	e.exitVimVisualMode(false)
	e.clearSelection()
	e.vimApplyLinewiseOperator(operator, startRow, endRow)
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
	case 'W':
		for i := 0; i < count; i++ {
			e.wordForwardLong()
		}
	case 'B':
		for i := 0; i < count; i++ {
			e.wordBackwardLong()
		}
	case 'E':
		for i := 0; i < count; i++ {
			e.wordEndLong()
		}
	case '(':
		for i := 0; i < count; i++ {
			e.vimSentenceBackward()
		}
	case ')':
		for i := 0; i < count; i++ {
			e.vimSentenceForward()
		}
	case '{':
		for i := 0; i < count; i++ {
			e.vimParagraphBackward()
		}
	case '}':
		for i := 0; i < count; i++ {
			e.vimParagraphForward()
		}
	case '$':
		e.moveLineEnd()
	case '0':
		e.moveLineStart()
	case '^':
		e.moveFirstNonBlank()
	default:
		return false
	}
	return true
}

func (e *Editor) vimOperatorRepeatsLinewise(operator string, r rune) bool {
	switch operator {
	case "d", "c", "y", ">", "<":
		return r == rune(operator[0])
	case "gu":
		return r == 'u'
	case "gU":
		return r == 'U'
	case "g~":
		return r == '~'
	default:
		return false
	}
}

func (e *Editor) vimApplyOperatorRange(operator string, start, end Cursor) {
	e.selectionStart = start
	e.selectionEnd = end
	e.selectionActive = true
	switch operator {
	case "y":
		if e.vimWritesClipboard() {
			e.clipboard.linewise = false
			e.yankSelection()
			e.vimStoreActiveRegister()
		}
	case "d":
		e.saveLineState()
		if s, en, ok := e.selectionRange(); ok {
			if e.vimWritesClipboard() {
				e.fillClipboardFromSelection()
				e.vimStoreActiveRegister()
			}
			e.deleteSelection(s, en, true)
		}
	case "c":
		e.saveLineState()
		if s, en, ok := e.selectionRange(); ok {
			if e.vimWritesClipboard() {
				e.fillClipboardFromSelection()
				e.vimStoreActiveRegister()
			}
			e.deleteSelection(s, en, true)
		}
		e.mode = ModeInsert
		e.saveLineState()
	case "gu", "gU", "g~":
		e.vimApplyCaseSelectionOperator(operator)
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
		e.saveLineState()
		e.vimDeleteLines(count)
	case "c":
		e.vimChangeLines(count)
	case ">":
		e.vimIndentLines(count)
	case "<":
		e.vimUnindentLines(count)
	case "gu", "gU", "g~":
		e.vimApplyCaseLinewiseOperator(operator, startRow, endRow)
	}
}

func (e *Editor) vimDeleteLines(count int) {
	if count < 1 {
		count = 1
	}
	if e.vimWritesClipboard() {
		e.vimYankLines(count)
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

func (e *Editor) vimChangeLines(count int) {
	if count < 1 {
		count = 1
	}
	if e.LineCount() == 0 {
		return
	}
	startRow := e.cursor.Row
	if startRow < 0 {
		startRow = 0
	}
	if startRow >= e.LineCount() {
		startRow = e.LineCount() - 1
	}
	endRow := startRow + count - 1
	if endRow >= e.LineCount() {
		endRow = e.LineCount() - 1
	}
	start := Cursor{Row: startRow, Col: 0}
	end := Cursor{Row: endRow, Col: e.lineLen(endRow)}
	e.saveLineState()
	if e.vimWritesClipboard() {
		e.vimYankLines(count)
	}
	e.deleteSelection(start, end, true)
	e.cursor = start
	e.mode = ModeInsert
	e.saveLineState()
}

func (e *Editor) vimDeleteToLineEnd() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	e.vimApplyOperatorRange("d", e.cursor, Cursor{Row: e.cursor.Row, Col: e.lineLen(e.cursor.Row)})
}

func (e *Editor) vimChangeToLineEnd() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	e.vimApplyOperatorRange("c", e.cursor, Cursor{Row: e.cursor.Row, Col: e.lineLen(e.cursor.Row)})
}

func (e *Editor) vimSubstituteChars(count int) {
	if count < 1 {
		count = 1
	}
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	end := e.cursor
	lineLen := e.lineLen(e.cursor.Row)
	end.Col += count
	if end.Col > lineLen {
		end.Col = lineLen
	}
	e.vimApplyOperatorRange("c", e.cursor, end)
}

func (e *Editor) vimDeleteBeforeCursor(count int) {
	if count < 1 {
		count = 1
	}
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() || e.cursor.Col <= 0 {
		return
	}
	startCol := e.cursor.Col - count
	if startCol < 0 {
		startCol = 0
	}
	e.deleteSelection(Cursor{Row: e.cursor.Row, Col: startCol}, e.cursor, true)
}

func (e *Editor) vimToggleCase(count int) {
	if count < 1 {
		count = 1
	}
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	e.saveLineState()
	for i := 0; i < count; i++ {
		if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() || e.cursor.Col >= e.lineLen(e.cursor.Row) {
			return
		}
		ch := e.line(e.cursor.Row)[e.cursor.Col]
		next := ch
		if unicode.IsLower(ch) {
			next = unicode.ToUpper(ch)
		} else if unicode.IsUpper(ch) {
			next = unicode.ToLower(ch)
		}
		if next != ch {
			e.replaceCharAtCursor(next)
		} else {
			e.moveRight()
		}
	}
}

func (e *Editor) vimIndentLines(count int) {
	if count < 1 {
		count = 1
	}
	startRow := e.cursor.Row
	for i := 0; i < count && startRow+i < e.LineCount(); i++ {
		e.cursor.Row = startRow + i
		e.cursor.Col = 0
		e.indentCurrentLine()
	}
	e.cursor.Row = startRow
	e.cursor.Col = 0
}

func (e *Editor) vimUnindentLines(count int) {
	if count < 1 {
		count = 1
	}
	startRow := e.cursor.Row
	for i := 0; i < count && startRow+i < e.LineCount(); i++ {
		e.cursor.Row = startRow + i
		e.cursor.Col = 0
		e.unindentSelection()
	}
	e.cursor.Row = startRow
	e.cursor.Col = 0
}

func (e *Editor) moveFirstNonBlank() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		e.cursor.Col = 0
		return
	}
	line := e.line(e.cursor.Row)
	col := 0
	for col < len(line) && (line[col] == ' ' || line[col] == '\t') {
		col++
	}
	e.cursor.Col = col
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
	e.vimStoreActiveRegister()
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
