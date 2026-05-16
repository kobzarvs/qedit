package editor

import "sort"

type helixCursorTextRange struct {
	start int
	end   int
}

func (e *Editor) duplicateHelixCursor(direction int) {
	if direction == 0 || e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	if len(e.profile.helix.multiCursors) == 0 {
		e.profile.helix.multiCursors = []Cursor{e.cursor}
	}
	base := e.profile.helix.multiCursors[len(e.profile.helix.multiCursors)-1]
	if direction < 0 {
		base = e.profile.helix.multiCursors[0]
	}
	row := base.Row + direction
	for row >= 0 && row < e.LineCount() {
		col := base.Col
		if col > e.lineLen(row) {
			row += direction
			continue
		}
		next := Cursor{Row: row, Col: col}
		if !cursorListContains(e.profile.helix.multiCursors, next) {
			e.profile.helix.multiCursors = append(e.profile.helix.multiCursors, next)
			sortCursors(e.profile.helix.multiCursors)
			e.cursor = next
			return
		}
		row += direction
	}
}

func (e *Editor) insertRuneAtHelixCursors(r rune) bool {
	if len(e.profile.helix.multiCursors) <= 1 {
		return false
	}
	return e.insertTextAtHelixCursors([][]rune{{r}})
}

func (e *Editor) applyHelixInsertActionToCursors(action string) bool {
	if len(e.profile.helix.multiCursors) <= 1 {
		return false
	}
	switch action {
	case actionBackspace:
		return e.backspaceAtHelixCursors()
	case actionDeleteChar:
		return e.deleteCharAtHelixCursors()
	case actionNewline:
		return e.insertTextAtHelixCursors([][]rune{nil, nil})
	default:
		return false
	}
}

func (e *Editor) backspaceAtHelixCursors() bool {
	maxIndex := e.docRuneCount()
	var ranges []helixCursorTextRange
	var finalIndices []int
	for _, idx := range e.helixCursorIndices() {
		if idx <= 0 || idx > maxIndex {
			continue
		}
		ranges = append(ranges, helixCursorTextRange{start: idx - 1, end: idx})
		finalIndices = append(finalIndices, idx-1)
	}
	return e.deleteTextRangesAtHelixCursors(ranges, finalIndices)
}

func (e *Editor) deleteCharAtHelixCursors() bool {
	maxIndex := e.docRuneCount()
	var ranges []helixCursorTextRange
	var finalIndices []int
	for _, idx := range e.helixCursorIndices() {
		if idx < 0 || idx >= maxIndex {
			continue
		}
		ranges = append(ranges, helixCursorTextRange{start: idx, end: idx + 1})
		finalIndices = append(finalIndices, idx)
	}
	return e.deleteTextRangesAtHelixCursors(ranges, finalIndices)
}

func (e *Editor) deleteTextRangesAtHelixCursors(ranges []helixCursorTextRange, finalIndices []int) bool {
	if len(e.profile.helix.multiCursors) <= 1 {
		return false
	}
	if len(ranges) == 0 {
		return true
	}
	sort.SliceStable(ranges, func(i, j int) bool {
		if ranges[i].start != ranges[j].start {
			return ranges[i].start > ranges[j].start
		}
		return ranges[i].end > ranges[j].end
	})
	primaryIndex := e.text.IndexForCursor(e.cursor)
	e.startUndoGroup()
	for _, r := range ranges {
		start := e.text.CursorForIndex(r.start)
		end := e.text.CursorForIndex(r.end)
		deleted := e.deleteTextRange(start, end)
		if len(deleted) > 0 {
			e.appendUndo(action{kind: actionInsertText, pos: start, text: deleted})
		}
	}
	e.finishUndoGroup()
	sort.SliceStable(ranges, func(i, j int) bool {
		if ranges[i].start != ranges[j].start {
			return ranges[i].start < ranges[j].start
		}
		return ranges[i].end < ranges[j].end
	})
	next := make([]Cursor, 0, len(finalIndices))
	for _, idx := range finalIndices {
		cursor := e.text.CursorForIndex(transformIndexAfterHelixDeletions(idx, ranges))
		if !cursorListContains(next, cursor) {
			next = append(next, cursor)
		}
	}
	primary := e.text.CursorForIndex(transformIndexAfterHelixDeletions(primaryIndex, ranges))
	e.setHelixMultiCursors(next, primary)
	e.change.lastEdit.Valid = false
	return true
}

func (e *Editor) insertTextAtHelixCursors(text [][]rune) bool {
	if len(e.profile.helix.multiCursors) <= 1 {
		return false
	}
	insert := joinText(text)
	if len(insert) == 0 {
		return true
	}
	indices := e.helixCursorIndices()
	if len(indices) == 0 {
		return true
	}
	primaryIndex := e.text.IndexForCursor(e.cursor)
	desc := append([]int(nil), indices...)
	sort.SliceStable(desc, func(i, j int) bool {
		return desc[i] > desc[j]
	})
	e.startUndoGroup()
	for _, idx := range desc {
		pos := e.text.CursorForIndex(idx)
		endPos := e.insertTextAt(pos, text)
		e.appendUndo(action{kind: actionDeleteText, pos: pos, endPos: endPos, text: text})
	}
	e.finishUndoGroup()
	next := make([]Cursor, 0, len(indices))
	for _, idx := range indices {
		cursor := e.text.CursorForIndex(transformIndexAfterHelixInsertions(idx, indices, len(insert)) + len(insert))
		if !cursorListContains(next, cursor) {
			next = append(next, cursor)
		}
	}
	primary := e.text.CursorForIndex(transformIndexAfterHelixInsertions(primaryIndex, indices, len(insert)) + len(insert))
	e.setHelixMultiCursors(next, primary)
	e.change.lastEdit.Valid = false
	return true
}

func (e *Editor) helixCursorIndices() []int {
	cursors := append([]Cursor(nil), e.profile.helix.multiCursors...)
	sortCursors(cursors)
	indices := make([]int, 0, len(cursors))
	for _, cursor := range cursors {
		idx := e.text.IndexForCursor(cursor)
		if len(indices) == 0 || indices[len(indices)-1] != idx {
			indices = append(indices, idx)
		}
	}
	return indices
}

func (e *Editor) setHelixMultiCursors(cursors []Cursor, primary Cursor) {
	sortCursors(cursors)
	unique := cursors[:0]
	for _, cursor := range cursors {
		cursor = e.clampInsertCursor(cursor)
		if len(unique) == 0 || unique[len(unique)-1] != cursor {
			unique = append(unique, cursor)
		}
	}
	if len(unique) > 1 {
		e.profile.helix.multiCursors = append([]Cursor(nil), unique...)
	} else {
		e.profile.helix.multiCursors = nil
	}
	if cursorListContains(unique, primary) {
		e.cursor = primary
	} else if len(unique) > 0 {
		e.cursor = unique[0]
	}
}

func transformIndexAfterHelixDeletions(index int, ranges []helixCursorTextRange) int {
	shift := 0
	for _, r := range ranges {
		if r.end <= index {
			shift -= r.end - r.start
			continue
		}
		if r.start < index {
			return r.start + shift
		}
		break
	}
	return index + shift
}

func transformIndexAfterHelixInsertions(index int, insertions []int, insertedLen int) int {
	shift := 0
	for _, insertion := range insertions {
		if insertion < index {
			shift += insertedLen
		}
	}
	return index + shift
}

func (e *Editor) applyHelixSelectingMotionToCursors(action string, count int) bool {
	if len(e.profile.helix.multiCursors) == 0 {
		return false
	}
	switch action {
	case actionFindChar, actionFindCharBackward, actionTillChar, actionTillCharBackward:
		return false
	}
	cursors := append([]Cursor(nil), e.profile.helix.multiCursors...)
	sortCursors(cursors)
	primary := 0
	for i, cursor := range cursors {
		if cursor == e.cursor {
			primary = i
			break
		}
	}
	ranges := make([]editorSelectionRange, 0, len(cursors))
	for _, anchor := range cursors {
		e.cursor = anchor
		for i := 0; i < count; i++ {
			e.execAction(action)
		}
		if anchor == e.cursor {
			continue
		}
		ranges = append(ranges, editorSelectionRange{
			Start: anchor,
			End:   e.helixSelectionEndForAction(action),
		})
	}
	e.profile.helix.multiCursors = nil
	e.setSelectionRanges(ranges, primary)
	return true
}

func (e *Editor) applyHelixMotionToSelectionRanges(action string, count int) bool {
	if !e.modal.selectMode || len(e.selectionRanges) <= 1 || !isMotionAction(action) {
		return false
	}
	switch action {
	case actionFindChar, actionFindCharBackward, actionTillChar, actionTillCharBackward:
		return false
	}
	if count < 1 {
		count = 1
	}
	ranges := cloneSelectionRanges(e.selectionRanges)
	primary := e.primarySelection
	if primary < 0 || primary >= len(ranges) {
		primary = 0
	}
	next := make([]editorSelectionRange, 0, len(ranges))
	for _, r := range ranges {
		e.cursor = r.End
		for i := 0; i < count; i++ {
			e.execAction(action)
		}
		next = append(next, editorSelectionRange{
			Start: r.Start,
			End:   e.helixSelectionEndForAction(action),
		})
	}
	e.setSelectionRanges(next, primary)
	return true
}

func (e *Editor) applyHelixMotionToCursors(action string, count int) bool {
	if len(e.profile.helix.multiCursors) <= 1 || !isHelixMultiCursorMotion(action) {
		return false
	}
	if count < 1 {
		count = 1
	}
	current := append([]Cursor(nil), e.profile.helix.multiCursors...)
	primaryBefore := e.cursor
	next := make([]Cursor, 0, len(current))
	primaryAfter := Cursor{}
	hadPrimary := false
	for _, pos := range current {
		e.cursor = pos
		for i := 0; i < count; i++ {
			e.execAction(action)
		}
		moved := e.cursor
		if pos == primaryBefore {
			primaryAfter = moved
			hadPrimary = true
		}
		if !cursorListContains(next, moved) {
			next = append(next, moved)
		}
	}
	if len(next) == 0 {
		e.profile.helix.multiCursors = nil
		e.cursor = primaryBefore
		return true
	}
	sortCursors(next)
	e.profile.helix.multiCursors = next
	if hadPrimary && cursorListContains(next, primaryAfter) {
		e.cursor = primaryAfter
	} else {
		e.cursor = next[0]
	}
	return true
}

func isHelixMultiCursorMotion(action string) bool {
	switch action {
	case actionMoveLeft, actionMoveRight, actionMoveUp, actionMoveDown,
		actionWordLeft, actionWordRight, actionLineStart, actionLineEnd,
		actionFileStart, actionFileEnd, actionPageUp, actionPageDown,
		actionWordForward, actionWordBackward, actionWordEnd,
		actionWordForwardLong, actionWordBackwardLong, actionWordEndLong,
		actionGotoLine, actionGotoFirstLine, actionGotoFileEnd:
		return true
	default:
		return false
	}
}

func (e *Editor) enterHelixInsertFromSelections(after bool) bool {
	ranges := e.activeSelectionRanges()
	if len(ranges) == 0 {
		return false
	}
	cursors := make([]Cursor, 0, len(ranges))
	for _, r := range ranges {
		start, end, ok := r.normalized()
		if !ok {
			continue
		}
		pos := start
		if after {
			pos = end
		}
		pos = e.clampInsertCursor(pos)
		if !cursorListContains(cursors, pos) {
			cursors = append(cursors, pos)
		}
	}
	return e.enterHelixInsertAtCursors(cursors)
}

func (e *Editor) enterHelixLineInsertFromSelections(lineEnd bool) bool {
	ranges := e.activeSelectionRanges()
	if len(ranges) == 0 {
		return false
	}
	rowsSeen := make(map[int]bool)
	var cursors []Cursor
	for _, r := range ranges {
		start, end, ok := r.normalized()
		if !ok {
			continue
		}
		endRow := end.Row
		if end.Col == 0 && endRow > start.Row {
			endRow--
		}
		for row := start.Row; row <= endRow && row < e.LineCount(); row++ {
			if row < 0 || rowsSeen[row] {
				continue
			}
			rowsSeen[row] = true
			col := e.firstNonBlankCol(row)
			if lineEnd {
				col = e.lineLen(row)
			}
			cursors = append(cursors, Cursor{Row: row, Col: col})
		}
	}
	return e.enterHelixInsertAtCursors(cursors)
}

func (e *Editor) enterHelixInsertAtCursors(cursors []Cursor) bool {
	if len(cursors) == 0 {
		return false
	}
	sortCursors(cursors)
	unique := cursors[:0]
	for _, cursor := range cursors {
		if len(unique) == 0 || unique[len(unique)-1] != cursor {
			unique = append(unique, cursor)
		}
	}
	e.clearSelection()
	e.modal.selectMode = false
	if len(unique) > 1 {
		e.profile.helix.multiCursors = append([]Cursor(nil), unique...)
	} else {
		e.profile.helix.multiCursors = nil
	}
	e.cursor = unique[0]
	e.mode = ModeInsert
	e.saveLineState()
	return true
}

func (e *Editor) clampInsertCursor(pos Cursor) Cursor {
	if pos.Row < 0 {
		pos.Row = 0
	}
	if pos.Row >= e.LineCount() {
		pos.Row = e.LineCount() - 1
	}
	if pos.Row < 0 {
		return Cursor{}
	}
	if pos.Col < 0 {
		pos.Col = 0
	}
	if maxCol := e.lineLen(pos.Row); pos.Col > maxCol {
		pos.Col = maxCol
	}
	return pos
}

func (e *Editor) firstNonBlankCol(row int) int {
	if row < 0 || row >= e.LineCount() {
		return 0
	}
	line := e.line(row)
	col := 0
	for col < len(line) && (line[col] == ' ' || line[col] == '\t') {
		col++
	}
	return col
}

func cursorListContains(cursors []Cursor, needle Cursor) bool {
	for _, cursor := range cursors {
		if cursor == needle {
			return true
		}
	}
	return false
}

func sortCursors(cursors []Cursor) {
	sort.SliceStable(cursors, func(i, j int) bool {
		return cursorLess(cursors[i], cursors[j])
	})
}
