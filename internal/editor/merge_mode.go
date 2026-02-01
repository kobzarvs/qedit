package editor

func (e *Editor) handleMerge(ev EventKey) bool {
	if ev.Key() == KeyEscape {
		e.mode = ModeNormal
		return false
	}
	if ev.Key() == KeyRune {
		switch ev.Rune() {
		case 'i':
			e.mode = ModeInsert
			return false
		case 'y', 'a':
			if !e.resolveConflictAtCursor(true) {
				e.setStatus("no conflict at cursor")
			}
			return false
		case 'r', 'n':
			if !e.resolveConflictAtCursor(false) {
				e.setStatus("no conflict at cursor")
			}
			return false
		}
	}
	return e.handleNormal(ev)
}

func (e *Editor) enterMergeMode() bool {
	e.ensureConflictBlocks()
	if !e.hasConflictBlocks() {
		e.setStatus("no conflicts")
		return false
	}
	e.mode = ModeMerge
	return false
}

func (e *Editor) resolveConflictAtCursor(accept bool) bool {
	idx, kind := e.conflictBlockIndexAtRow(e.cursor.Row)
	if idx < 0 {
		return false
	}
	block := e.conflictBlocks[idx]
	deleteStart := -1
	deleteEnd := -1
	if kind == conflictLocal {
		if accept {
			deleteStart = block.remoteStart
			deleteEnd = block.remoteEnd
		} else {
			deleteStart = block.localStart
			deleteEnd = block.localEnd
		}
	} else if kind == conflictRemote {
		if accept {
			deleteStart = block.localStart
			deleteEnd = block.localEnd
		} else {
			deleteStart = block.remoteStart
			deleteEnd = block.remoteEnd
		}
	} else {
		return false
	}
	deletedLines := 0
	if deleteStart >= 0 && deleteEnd >= deleteStart {
		deletedLines = e.deleteConflictLines(deleteStart, deleteEnd)
	}
	e.removeConflictBlock(idx, deleteEnd, deletedLines)
	e.conflictBlocksDirty = false
	if len(e.conflictBlocks) == 0 && e.mode == ModeMerge {
		e.mode = ModeNormal
	}
	return true
}

func (e *Editor) deleteConflictLines(startLine, endLine int) int {
	lineCount := e.LineCount()
	if lineCount == 0 {
		return 0
	}
	if startLine < 0 {
		startLine = 0
	}
	if endLine >= lineCount {
		endLine = lineCount - 1
	}
	if startLine > endLine {
		return 0
	}
	start := Cursor{Row: startLine, Col: 0}
	end := Cursor{Row: endLine, Col: e.lineLen(endLine)}
	if endLine < lineCount-1 {
		end = Cursor{Row: endLine + 1, Col: 0}
	} else if startLine > 0 {
		start = Cursor{Row: startLine - 1, Col: e.lineLen(startLine - 1)}
	}
	if start.Row > end.Row || (start.Row == end.Row && start.Col >= end.Col) {
		return 0
	}
	e.deleteSelection(start, end, false)
	e.clearSelection()
	return endLine - startLine + 1
}

func (e *Editor) removeConflictBlock(idx int, deleteEnd, deletedLines int) {
	if idx < 0 || idx >= len(e.conflictBlocks) {
		return
	}
	e.conflictBlocks = append(e.conflictBlocks[:idx], e.conflictBlocks[idx+1:]...)
	if deletedLines == 0 {
		return
	}
	for i := idx; i < len(e.conflictBlocks); i++ {
		block := &e.conflictBlocks[i]
		if block.start > deleteEnd {
			shiftBlock(block, deletedLines)
		}
	}
}
