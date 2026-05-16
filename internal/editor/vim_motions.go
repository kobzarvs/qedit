package editor

import "unicode"

func (e *Editor) vimParagraphForward() {
	if e.LineCount() == 0 {
		return
	}
	row := e.cursor.Row + 1
	if e.vimLineBlank(e.cursor.Row) {
		for row < e.LineCount() && e.vimLineBlank(row) {
			row++
		}
		if row >= e.LineCount() {
			e.cursor.Row = e.LineCount() - 1
			e.cursor.Col = e.lineLen(e.cursor.Row)
			return
		}
		e.cursor.Row = row
		e.moveFirstNonBlank()
		return
	}
	for row < e.LineCount() && !e.vimLineBlank(row) {
		row++
	}
	if row >= e.LineCount() {
		e.cursor.Row = e.LineCount() - 1
		e.cursor.Col = e.lineLen(e.cursor.Row)
		return
	}
	e.cursor.Row = row
	e.cursor.Col = 0
}

func (e *Editor) vimParagraphBackward() {
	if e.LineCount() == 0 {
		return
	}
	row := e.cursor.Row - 1
	if e.vimLineBlank(e.cursor.Row) {
		for row >= 0 && e.vimLineBlank(row) {
			row--
		}
		for row >= 0 && !e.vimLineBlank(row) {
			row--
		}
		row++
		if row < 0 {
			row = 0
		}
		e.cursor.Row = row
		e.moveFirstNonBlank()
		return
	}
	for row >= 0 && !e.vimLineBlank(row) {
		row--
	}
	if row < 0 {
		row = 0
		e.cursor.Row = row
		e.moveFirstNonBlank()
		return
	}
	e.cursor.Row = row
	e.cursor.Col = 0
}

func (e *Editor) vimSentenceForward() {
	if e.hugeFileActive() || e.text == nil {
		return
	}
	runes := []rune(e.Content())
	if len(runes) == 0 {
		return
	}
	idx := e.text.IndexForCursor(e.cursor)
	if idx < len(runes) {
		idx++
	}
	for idx < len(runes) {
		if vimSentenceTerminator(runes[idx]) {
			idx++
			for idx < len(runes) && unicode.IsSpace(runes[idx]) {
				idx++
			}
			e.cursor = e.text.CursorForIndex(idx)
			return
		}
		idx++
	}
	e.cursor = e.text.CursorForIndex(len(runes))
}

func (e *Editor) vimSentenceBackward() {
	if e.hugeFileActive() || e.text == nil {
		return
	}
	runes := []rune(e.Content())
	if len(runes) == 0 {
		return
	}
	idx := e.text.IndexForCursor(e.cursor)
	prev := vimFindSentenceTerminatorBackward(runes, idx-1)
	if prev < 0 {
		e.cursor = Cursor{}
		return
	}
	prevPrev := vimFindSentenceTerminatorBackward(runes, prev-1)
	start := prevPrev + 1
	for start < len(runes) && unicode.IsSpace(runes[start]) {
		start++
	}
	e.cursor = e.text.CursorForIndex(start)
}

func vimFindSentenceTerminatorBackward(runes []rune, idx int) int {
	if idx >= len(runes) {
		idx = len(runes) - 1
	}
	for idx >= 0 {
		if vimSentenceTerminator(runes[idx]) {
			return idx
		}
		idx--
	}
	return -1
}

func vimSentenceTerminator(r rune) bool {
	return r == '.' || r == '!' || r == '?'
}
