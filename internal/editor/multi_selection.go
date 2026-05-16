package editor

import "sort"

func (r editorSelectionRange) normalized() (Cursor, Cursor, bool) {
	start := r.Start
	end := r.End
	if cursorLess(end, start) {
		start, end = end, start
	}
	if start == end {
		return Cursor{}, Cursor{}, false
	}
	return start, end, true
}

func (e *Editor) setSelectionRanges(ranges []editorSelectionRange, primary int) {
	e.selectionRanges = e.selectionRanges[:0]
	for _, r := range ranges {
		if _, _, ok := r.normalized(); ok {
			e.selectionRanges = append(e.selectionRanges, r)
		}
	}
	if len(e.selectionRanges) == 0 {
		e.clearSelection()
		return
	}
	if primary < 0 || primary >= len(e.selectionRanges) {
		primary = 0
	}
	e.primarySelection = primary
	e.selectionActive = true
	e.selectionStart = e.selectionRanges[primary].Start
	e.selectionEnd = e.selectionRanges[primary].End
	e.cursor = e.selectionEnd
	e.modal.selectMode = true
}

func (e *Editor) activeSelectionRanges() []editorSelectionRange {
	if len(e.selectionRanges) > 0 {
		out := make([]editorSelectionRange, 0, len(e.selectionRanges))
		for _, r := range e.selectionRanges {
			if start, end, ok := r.normalized(); ok {
				out = append(out, editorSelectionRange{Start: start, End: end})
			}
		}
		return out
	}
	if start, end, ok := e.selectionRange(); ok {
		return []editorSelectionRange{{Start: start, End: end}}
	}
	return nil
}

func (e *Editor) rawActiveSelectionRanges() []editorSelectionRange {
	if len(e.selectionRanges) > 0 {
		return cloneSelectionRanges(e.selectionRanges)
	}
	if e.selectionActive {
		return []editorSelectionRange{{Start: e.selectionStart, End: e.selectionEnd}}
	}
	return nil
}

func (e *Editor) hasMultipleSelections() bool {
	return len(e.selectionRanges) > 1
}

func (e *Editor) selectionContainsCell(lineIdx, col int, selStart, selEnd int) bool {
	if len(e.selectionRanges) == 0 {
		return selStart >= 0 && selEnd > selStart && col >= selStart && col < selEnd
	}
	for _, r := range e.selectionRanges {
		start, end, ok := r.normalized()
		if !ok || lineIdx < start.Row || lineIdx > end.Row {
			continue
		}
		lineLen := 0
		if lineIdx >= 0 && lineIdx < e.LineCount() {
			lineLen = e.lineLen(lineIdx)
		}
		startCol := 0
		endCol := lineLen
		if start.Row == end.Row {
			startCol = clampRange(start.Col, 0, lineLen)
			endCol = clampRange(end.Col, 0, lineLen)
		} else if lineIdx == start.Row {
			startCol = clampRange(start.Col, 0, lineLen)
		} else if lineIdx == end.Row {
			endCol = clampRange(end.Col, 0, lineLen)
		}
		if endCol > startCol && col >= startCol && col < endCol {
			return true
		}
	}
	return false
}

func cloneSelectionRanges(in []editorSelectionRange) []editorSelectionRange {
	if len(in) == 0 {
		return nil
	}
	out := make([]editorSelectionRange, len(in))
	copy(out, in)
	return out
}

func (e *Editor) deleteSelectionRanges(ranges []editorSelectionRange) {
	ranges = normalizedSelectionRanges(ranges)
	if len(ranges) == 0 {
		return
	}
	sortSelectionRangesDescending(ranges)
	e.startUndoGroup()
	for _, r := range ranges {
		deleted := e.deleteTextRange(r.Start, r.End)
		if len(deleted) > 0 {
			e.appendUndo(action{kind: actionInsertText, pos: r.Start, text: deleted})
		}
	}
	e.finishUndoGroup()
	e.clearSelection()
	e.change.lastEdit.Valid = false
}

func (e *Editor) replaceSelectionRangesWithRune(ranges []editorSelectionRange, ch rune) bool {
	ranges = normalizedSelectionRanges(ranges)
	if len(ranges) == 0 {
		return false
	}
	sortSelectionRangesDescending(ranges)
	e.startUndoGroup()
	for _, r := range ranges {
		deleted := e.deleteTextRange(r.Start, r.End)
		if len(deleted) == 0 {
			continue
		}
		e.appendUndo(action{kind: actionInsertText, pos: r.Start, text: deleted})
		replacement := sameShapeRunes(deleted, ch)
		endPos := e.insertTextAt(r.Start, replacement)
		e.appendUndo(action{kind: actionDeleteText, pos: r.Start, endPos: endPos, text: replacement})
	}
	e.finishUndoGroup()
	e.clearSelection()
	e.modal.selectMode = false
	e.change.lastEdit.Valid = false
	return true
}

func normalizedSelectionRanges(ranges []editorSelectionRange) []editorSelectionRange {
	out := make([]editorSelectionRange, 0, len(ranges))
	for _, r := range ranges {
		start, end, ok := r.normalized()
		if !ok {
			continue
		}
		out = append(out, editorSelectionRange{Start: start, End: end})
	}
	return out
}

func sortSelectionRangesDescending(ranges []editorSelectionRange) {
	sort.SliceStable(ranges, func(i, j int) bool {
		if ranges[i].Start.Row != ranges[j].Start.Row {
			return ranges[i].Start.Row > ranges[j].Start.Row
		}
		return ranges[i].Start.Col > ranges[j].Start.Col
	})
}

func sameShapeRunes(lines [][]rune, ch rune) [][]rune {
	out := make([][]rune, len(lines))
	for i, line := range lines {
		out[i] = make([]rune, len(line))
		for j := range line {
			out[i][j] = ch
		}
	}
	return out
}
