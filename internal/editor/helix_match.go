package editor

func (e *Editor) matchSelectPair(object rune, around bool) bool {
	start, end, ok := e.vimTextObjectRange(object, around)
	if !ok {
		return false
	}
	e.selectionStart = start
	e.selectionEnd = end
	e.selectionActive = true
	e.selectionRanges = nil
	e.modal.selectMode = true
	e.cursor = end
	return false
}

func (e *Editor) matchSurroundSelection(object rune) bool {
	open, close, ok := delimiterPair(object)
	if !ok {
		return false
	}
	start, end, ok := e.selectionRange()
	if !ok {
		return false
	}
	e.startUndoGroup()
	if e.insertRuneAt(end, close) {
		e.appendUndo(action{kind: actionDeleteRune, pos: end, r: close})
	}
	if e.insertRuneAt(start, open) {
		e.appendUndo(action{kind: actionDeleteRune, pos: start, r: open})
	}
	e.finishUndoGroup()
	e.clearSelection()
	e.modal.selectMode = false
	e.change.lastEdit.Valid = false
	return false
}

func (e *Editor) matchDeleteSurround(object rune) bool {
	start, end, ok := e.vimTextObjectRange(object, true)
	if !ok {
		return false
	}
	closePos, ok := e.cursorBefore(end)
	if !ok {
		return false
	}
	e.startUndoGroup()
	if closePos.Row >= 0 && closePos.Row < e.LineCount() && closePos.Col < e.lineLen(closePos.Row) {
		closeRune := e.line(closePos.Row)[closePos.Col]
		if e.deleteRuneAt(closePos) {
			e.appendUndo(action{kind: actionInsertRune, pos: closePos, r: closeRune})
		}
	}
	if start.Row >= 0 && start.Row < e.LineCount() && start.Col < e.lineLen(start.Row) {
		openRune := e.line(start.Row)[start.Col]
		if e.deleteRuneAt(start) {
			e.appendUndo(action{kind: actionInsertRune, pos: start, r: openRune})
		}
	}
	e.finishUndoGroup()
	e.cursor = start
	e.change.lastEdit.Valid = false
	return false
}

func (e *Editor) matchReplaceSurround(oldObject, newObject rune) bool {
	open, close, ok := delimiterPair(newObject)
	if !ok {
		return false
	}
	start, end, ok := e.vimTextObjectRange(oldObject, true)
	if !ok {
		return false
	}
	closePos, ok := e.cursorBefore(end)
	if !ok {
		return false
	}
	e.startUndoGroup()
	if closePos.Row >= 0 && closePos.Row < e.LineCount() && closePos.Col < e.lineLen(closePos.Row) {
		oldClose := e.line(closePos.Row)[closePos.Col]
		if e.deleteRuneAt(closePos) {
			e.appendUndo(action{kind: actionInsertRune, pos: closePos, r: oldClose})
		}
		if e.insertRuneAt(closePos, close) {
			e.appendUndo(action{kind: actionDeleteRune, pos: closePos, r: close})
		}
	}
	if start.Row >= 0 && start.Row < e.LineCount() && start.Col < e.lineLen(start.Row) {
		oldOpen := e.line(start.Row)[start.Col]
		if e.deleteRuneAt(start) {
			e.appendUndo(action{kind: actionInsertRune, pos: start, r: oldOpen})
		}
		if e.insertRuneAt(start, open) {
			e.appendUndo(action{kind: actionDeleteRune, pos: start, r: open})
		}
	}
	e.finishUndoGroup()
	e.cursor = start
	e.change.lastEdit.Valid = false
	return false
}

func delimiterPair(ch rune) (rune, rune, bool) {
	switch ch {
	case '(', ')':
		return '(', ')', true
	case '[', ']':
		return '[', ']', true
	case '{', '}':
		return '{', '}', true
	case '<', '>':
		return '<', '>', true
	case '"', '\'', '`':
		return ch, ch, true
	default:
		return 0, 0, false
	}
}

func (e *Editor) cursorBefore(pos Cursor) (Cursor, bool) {
	if pos.Row < 0 || pos.Row >= e.LineCount() {
		return Cursor{}, false
	}
	if pos.Col > 0 {
		return Cursor{Row: pos.Row, Col: pos.Col - 1}, true
	}
	if pos.Row == 0 {
		return Cursor{}, false
	}
	prevRow := pos.Row - 1
	return Cursor{Row: prevRow, Col: e.lineLen(prevRow) - 1}, e.lineLen(prevRow) > 0
}
