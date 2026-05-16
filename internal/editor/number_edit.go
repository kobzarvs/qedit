package editor

import (
	"strconv"
	"unicode"
)

func (e *Editor) changeNumberAtCursor(delta int) {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	line := e.line(e.cursor.Row)
	if len(line) == 0 {
		return
	}
	col := e.cursor.Col
	if col >= len(line) {
		col = len(line) - 1
	}
	if col < 0 {
		col = 0
	}
	start, end, ok := numberRangeNearColumn(line, col)
	if !ok {
		return
	}
	value, err := strconv.Atoi(string(line[start:end]))
	if err != nil {
		return
	}
	replacement := []rune(strconv.Itoa(value + delta))
	pos := Cursor{Row: e.cursor.Row, Col: start}
	endPos := Cursor{Row: e.cursor.Row, Col: end}
	e.startUndoGroup()
	deleted := e.deleteTextRange(pos, endPos)
	if len(deleted) > 0 {
		e.appendUndo(action{kind: actionInsertText, pos: pos, text: deleted})
	}
	insertEnd := e.insertTextAt(pos, [][]rune{replacement})
	e.appendUndo(action{kind: actionDeleteText, pos: pos, endPos: insertEnd, text: [][]rune{replacement}})
	e.finishUndoGroup()
	e.cursor = Cursor{Row: pos.Row, Col: start}
}

func numberRangeNearColumn(line []rune, col int) (int, int, bool) {
	if col < len(line) && (unicode.IsDigit(line[col]) || line[col] == '-') {
		return expandNumberRange(line, col)
	}
	for i := col; i < len(line); i++ {
		if unicode.IsDigit(line[i]) || line[i] == '-' {
			return expandNumberRange(line, i)
		}
	}
	for i := col - 1; i >= 0; i-- {
		if unicode.IsDigit(line[i]) || line[i] == '-' {
			return expandNumberRange(line, i)
		}
	}
	return 0, 0, false
}

func expandNumberRange(line []rune, col int) (int, int, bool) {
	if line[col] == '-' && (col+1 >= len(line) || !unicode.IsDigit(line[col+1])) {
		return 0, 0, false
	}
	start := col
	if unicode.IsDigit(line[start]) {
		for start > 0 && unicode.IsDigit(line[start-1]) {
			start--
		}
	}
	if start > 0 && line[start-1] == '-' {
		start--
	}
	end := col
	if line[end] == '-' {
		end++
	}
	for end < len(line) && unicode.IsDigit(line[end]) {
		end++
	}
	return start, end, end > start
}
