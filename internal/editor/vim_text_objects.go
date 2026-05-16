package editor

import "unicode"

func (e *Editor) handleVimTextObjectRune(operator string, object rune) {
	around := e.profile.vim.pendingTextObjectAround
	if e.handleVimCountRune(object) {
		return
	}
	_ = e.consumeVimOperatorCount()
	start, end, ok := e.vimTextObjectRange(object, around)
	if !ok {
		e.resetVimPendingState()
		e.vimCancelRepeatRecordingForOperator(operator)
		return
	}
	if e.vimApplyTextObjectOperator(operator, start, end) {
		e.resetVimPendingState()
		e.vimFinishRepeatRecordingForOperator(operator)
		return
	}
	e.resetVimPendingState()
	e.vimCancelRepeatRecordingForOperator(operator)
}

func (e *Editor) vimApplyTextObjectOperator(operator string, start, end Cursor) bool {
	switch operator {
	case "y", "d", "c", "gu", "gU", "g~":
		e.vimApplyOperatorRange(operator, start, end)
		return true
	case ">":
		e.vimApplyLinewiseOperator(operator, start.Row, end.Row)
		return true
	case "<":
		e.vimApplyLinewiseOperator(operator, start.Row, end.Row)
		return true
	default:
		return false
	}
}

func (e *Editor) vimTextObjectRange(object rune, around bool) (Cursor, Cursor, bool) {
	switch object {
	case 'w':
		return e.vimWordTextObjectRange(around)
	case 'p':
		return e.vimParagraphTextObjectRange(around)
	case '"', '\'', '`':
		return e.vimQuoteTextObjectRange(object, around)
	case '(', ')', 'b':
		return e.vimPairTextObjectRange('(', ')', around)
	case '[', ']':
		return e.vimPairTextObjectRange('[', ']', around)
	case '{', '}', 'B':
		return e.vimPairTextObjectRange('{', '}', around)
	case '<', '>':
		return e.vimPairTextObjectRange('<', '>', around)
	default:
		e.setStatus("unsupported text object")
		return Cursor{}, Cursor{}, false
	}
}

func (e *Editor) vimWordTextObjectRange(around bool) (Cursor, Cursor, bool) {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return Cursor{}, Cursor{}, false
	}
	line := e.line(e.cursor.Row)
	if len(line) == 0 {
		return Cursor{}, Cursor{}, false
	}
	col := e.cursor.Col
	if col >= len(line) {
		col = len(line) - 1
	}
	if col < 0 {
		col = 0
	}
	if !isWordChar(line[col]) {
		if col > 0 && isWordChar(line[col-1]) {
			col--
		} else {
			next := col
			for next < len(line) && !isWordChar(line[next]) {
				next++
			}
			if next >= len(line) {
				return Cursor{}, Cursor{}, false
			}
			col = next
		}
	}
	startCol := col
	for startCol > 0 && isWordChar(line[startCol-1]) {
		startCol--
	}
	endCol := col + 1
	for endCol < len(line) && isWordChar(line[endCol]) {
		endCol++
	}
	if around {
		if endCol < len(line) && unicode.IsSpace(line[endCol]) {
			for endCol < len(line) && unicode.IsSpace(line[endCol]) {
				endCol++
			}
		} else {
			for startCol > 0 && unicode.IsSpace(line[startCol-1]) {
				startCol--
			}
		}
	}
	return Cursor{Row: e.cursor.Row, Col: startCol}, Cursor{Row: e.cursor.Row, Col: endCol}, true
}

func (e *Editor) vimParagraphTextObjectRange(around bool) (Cursor, Cursor, bool) {
	if e.LineCount() == 0 {
		return Cursor{}, Cursor{}, false
	}
	row := e.cursor.Row
	if row < 0 {
		row = 0
	}
	if row >= e.LineCount() {
		row = e.LineCount() - 1
	}
	for row < e.LineCount() && e.vimLineBlank(row) {
		row++
	}
	if row >= e.LineCount() {
		return Cursor{}, Cursor{}, false
	}
	startRow := row
	for startRow > 0 && !e.vimLineBlank(startRow-1) {
		startRow--
	}
	endRow := row
	for endRow+1 < e.LineCount() && !e.vimLineBlank(endRow+1) {
		endRow++
	}
	end := Cursor{Row: endRow, Col: e.lineLen(endRow)}
	if around && endRow+1 < e.LineCount() && e.vimLineBlank(endRow+1) {
		end = Cursor{Row: endRow + 1, Col: e.lineLen(endRow + 1)}
	}
	return Cursor{Row: startRow, Col: 0}, end, true
}

func (e *Editor) vimLineBlank(row int) bool {
	if row < 0 || row >= e.LineCount() {
		return true
	}
	for _, r := range e.line(row) {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func (e *Editor) vimQuoteTextObjectRange(quote rune, around bool) (Cursor, Cursor, bool) {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return Cursor{}, Cursor{}, false
	}
	line := e.line(e.cursor.Row)
	if len(line) == 0 {
		return Cursor{}, Cursor{}, false
	}
	col := e.cursor.Col
	if col >= len(line) {
		col = len(line) - 1
	}
	left := -1
	for i := col; i >= 0; i-- {
		if line[i] == quote && !vimEscaped(line, i) {
			left = i
			break
		}
	}
	right := -1
	for i := col + 1; i < len(line); i++ {
		if line[i] == quote && !vimEscaped(line, i) {
			right = i
			break
		}
	}
	if left < 0 || right < 0 {
		return Cursor{}, Cursor{}, false
	}
	if around {
		return Cursor{Row: e.cursor.Row, Col: left}, Cursor{Row: e.cursor.Row, Col: right + 1}, true
	}
	return Cursor{Row: e.cursor.Row, Col: left + 1}, Cursor{Row: e.cursor.Row, Col: right}, true
}

func vimEscaped(line []rune, idx int) bool {
	count := 0
	for i := idx - 1; i >= 0 && line[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

func (e *Editor) vimPairTextObjectRange(open, close rune, around bool) (Cursor, Cursor, bool) {
	if e.hugeFileActive() || e.text == nil {
		return Cursor{}, Cursor{}, false
	}
	runes := []rune(e.Content())
	if len(runes) == 0 {
		return Cursor{}, Cursor{}, false
	}
	cursorIdx := e.text.IndexForCursor(e.cursor)
	if cursorIdx >= len(runes) {
		cursorIdx = len(runes) - 1
	}
	for i := cursorIdx; i >= 0; i-- {
		if runes[i] != open {
			continue
		}
		if closeIdx, ok := vimFindPairClose(runes, i, open, close); ok && closeIdx >= cursorIdx {
			if around {
				return e.text.CursorForIndex(i), e.text.CursorForIndex(closeIdx + 1), true
			}
			return e.text.CursorForIndex(i + 1), e.text.CursorForIndex(closeIdx), true
		}
	}
	return Cursor{}, Cursor{}, false
}

func vimFindPairClose(runes []rune, openIdx int, open, close rune) (int, bool) {
	depth := 0
	for i := openIdx; i < len(runes); i++ {
		switch runes[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return -1, false
}
