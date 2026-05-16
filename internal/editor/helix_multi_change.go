package editor

func (e *Editor) beginHelixMultiChange() {
	ranges := normalizedSelectionRanges(e.selectionRanges)
	if len(ranges) == 0 {
		e.clearSelection()
		return
	}
	primary := e.primarySelection
	if primary < 0 || primary >= len(ranges) {
		primary = 0
	}
	start := ranges[primary].Start
	end := ranges[primary].End
	e.profile.helix.multiChangeActive = true
	e.profile.helix.multiChangeRanges = cloneSelectionRanges(ranges)
	e.profile.helix.multiChangePrimary = primary
	e.profile.helix.multiChangeStart = start
	e.profile.helix.multiChangeOriginalEnd = end
	e.deleteSelection(start, end, true)
	e.cursor = start
	e.selectionRanges = nil
	e.primarySelection = 0
}

func (e *Editor) finishHelixMultiChange() {
	if !e.profile.helix.multiChangeActive {
		return
	}
	ranges := cloneSelectionRanges(e.profile.helix.multiChangeRanges)
	primary := e.profile.helix.multiChangePrimary
	start := e.profile.helix.multiChangeStart
	originalEnd := e.profile.helix.multiChangeOriginalEnd
	e.profile.helix.multiChangeActive = false
	e.profile.helix.multiChangeRanges = nil
	if primary < 0 || primary >= len(ranges) {
		return
	}
	if e.cursor.Row != start.Row || e.cursor.Col < start.Col {
		return
	}
	inserted := e.collectDeletedText(start, e.cursor)
	insertedLen := 0
	if len(inserted) == 1 {
		insertedLen = len(inserted[0])
	} else if len(inserted) > 1 {
		// Multi-line replay needs position transforms across rows; leave the
		// primary edit intact until that path is implemented.
		return
	}
	originalLen := originalEnd.Col - start.Col
	delta := insertedLen - originalLen
	var replacements []editorSelectionRange
	for i, r := range ranges {
		if i == primary {
			continue
		}
		replacements = append(replacements, adjustRangeAfterPrimaryChange(r, start, originalEnd, delta))
	}
	sortSelectionRangesDescending(replacements)
	e.startUndoGroup()
	for _, r := range replacements {
		deleted := e.deleteTextRange(r.Start, r.End)
		if len(deleted) > 0 {
			e.appendUndo(action{kind: actionInsertText, pos: r.Start, text: deleted})
		}
		if len(inserted) > 0 {
			endPos := e.insertTextAt(r.Start, inserted)
			e.appendUndo(action{kind: actionDeleteText, pos: r.Start, endPos: endPos, text: inserted})
		}
	}
	e.finishUndoGroup()
	e.clearSelection()
	e.modal.selectMode = false
	e.change.lastEdit.Valid = false
}

func adjustRangeAfterPrimaryChange(r editorSelectionRange, start, originalEnd Cursor, delta int) editorSelectionRange {
	if delta == 0 || r.Start.Row != start.Row {
		return r
	}
	if r.Start.Col >= originalEnd.Col {
		r.Start.Col += delta
		r.End.Col += delta
	}
	return r
}
