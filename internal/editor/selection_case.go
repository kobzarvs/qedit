package editor

import "unicode"

type caseTransformKind int

const (
	caseTransformToggle caseTransformKind = iota
	caseTransformLower
	caseTransformUpper
)

func (e *Editor) transformSelectionCase(kind caseTransformKind) {
	if e.hasMultipleSelections() {
		e.transformSelectionRangesCase(e.activeSelectionRanges(), kind)
		return
	}
	if e.BehaviorProfile() == BehaviorProfileHelix && len(e.profile.helix.multiCursors) > 1 {
		e.transformHelixCursorCellsCase(kind)
		return
	}
	start, end, hadSelection := e.selectionRange()
	if !hadSelection {
		if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() || e.cursor.Col >= e.lineLen(e.cursor.Row) {
			return
		}
		start = e.cursor
		end = e.advanceCursorOne(e.cursor)
	}
	e.startUndoGroup()
	deleted := e.deleteTextRange(start, end)
	if len(deleted) == 0 {
		return
	}
	transformed := transformCaseLines(deleted, kind)
	e.appendUndo(action{kind: actionInsertText, pos: start, text: deleted})
	endPos := e.insertTextAt(start, transformed)
	e.appendUndo(action{kind: actionDeleteText, pos: start, endPos: endPos, text: transformed})
	e.finishUndoGroup()
	e.cursor = start
	if hadSelection {
		e.selectionStart = start
		e.selectionEnd = endPos
		e.selectionActive = true
		e.modal.selectMode = true
	}
}

func (e *Editor) transformHelixCursorCellsCase(kind caseTransformKind) {
	cursors := append([]Cursor(nil), e.profile.helix.multiCursors...)
	sortCursors(cursors)
	e.startUndoGroup()
	for _, pos := range cursors {
		if pos.Row < 0 || pos.Row >= e.LineCount() || pos.Col < 0 || pos.Col >= e.lineLen(pos.Row) {
			continue
		}
		line := e.line(pos.Row)
		old := line[pos.Col]
		next := transformCaseRune(old, kind)
		if next == old {
			continue
		}
		if e.deleteRuneAt(pos) {
			e.appendUndo(action{kind: actionInsertRune, pos: pos, r: old})
		}
		if e.insertRuneAt(pos, next) {
			e.appendUndo(action{kind: actionDeleteRune, pos: pos, r: next})
		}
	}
	e.finishUndoGroup()
	e.profile.helix.multiCursors = cursors
	e.cursor = cursors[0]
	e.change.lastEdit.Valid = false
}

func (e *Editor) transformSelectionRangesCase(ranges []editorSelectionRange, kind caseTransformKind) {
	ranges = normalizedSelectionRanges(ranges)
	if len(ranges) == 0 {
		return
	}
	sortSelectionRangesDescending(ranges)
	e.startUndoGroup()
	for _, r := range ranges {
		deleted := e.deleteTextRange(r.Start, r.End)
		if len(deleted) == 0 {
			continue
		}
		transformed := transformCaseLines(deleted, kind)
		e.appendUndo(action{kind: actionInsertText, pos: r.Start, text: deleted})
		endPos := e.insertTextAt(r.Start, transformed)
		e.appendUndo(action{kind: actionDeleteText, pos: r.Start, endPos: endPos, text: transformed})
	}
	e.finishUndoGroup()
	e.clearSelection()
	e.modal.selectMode = false
	e.change.lastEdit.Valid = false
}

func transformCaseLines(lines [][]rune, kind caseTransformKind) [][]rune {
	out := make([][]rune, len(lines))
	for i, line := range lines {
		out[i] = make([]rune, len(line))
		for j, r := range line {
			out[i][j] = transformCaseRune(r, kind)
		}
	}
	return out
}

func transformCaseRune(r rune, kind caseTransformKind) rune {
	switch kind {
	case caseTransformLower:
		return unicode.ToLower(r)
	case caseTransformUpper:
		return unicode.ToUpper(r)
	default:
		if unicode.IsLower(r) {
			return unicode.ToUpper(r)
		}
		if unicode.IsUpper(r) {
			return unicode.ToLower(r)
		}
		return r
	}
}
