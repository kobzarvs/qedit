package editor

import "time"

func (e *Editor) handleMerge(ev EventKey) bool {
	if e.gitDiffPreviewActive() {
		if ev.Key() == KeyEscape {
			e.deactivateGitDiffPreview()
			e.mode = ModeNormal
			return false
		}
		if ev.Key() == KeyRune && ev.Rune() == 'i' {
			e.deactivateGitDiffPreview()
			e.mode = ModeInsert
			return false
		}
	}
	if ev.Key() == KeyEscape {
		e.deactivateMergeReview()
		e.mode = ModeNormal
		return false
	}
	if e.mergeReviewActive() {
		switch ev.Key() {
		case KeyLeft:
			e.setMergeReviewPane(mergeReviewPaneLocal)
			return false
		case KeyRight:
			e.setMergeReviewPane(mergeReviewPaneRemote)
			return false
		case KeyTab:
			e.cycleMergeReviewPane()
			return false
		case KeyEnter:
			e.applySelectedMergeReviewPane()
			return false
		}
	}
	if ev.Key() == KeyRune {
		switch ev.Rune() {
		case '[':
			if e.mergeReviewActive() {
				e.setMergeReviewPane(mergeReviewPaneLocal)
				return false
			}
		case ']':
			if e.mergeReviewActive() {
				e.setMergeReviewPane(mergeReviewPaneRemote)
				return false
			}
		case 'i':
			e.mode = ModeInsert
			return false
		case 'y', 'a':
			if e.mergeReviewActive() {
				if !e.applyMergeReviewPane(mergeReviewPaneLocal) {
					e.setStatus("no conflict at cursor")
				}
				return false
			}
			if !e.resolveConflictAtCursor(true) {
				e.setStatus("no conflict at cursor")
			}
			return false
		case 'r', 'n':
			if e.mergeReviewActive() {
				if !e.applyMergeReviewPane(mergeReviewPaneRemote) {
					e.setStatus("no conflict at cursor")
				}
				return false
			}
			if !e.resolveConflictAtCursor(false) {
				e.setStatus("no conflict at cursor")
			}
			return false
		}
	}
	return e.handleNormal(ev)
}

func (e *Editor) enterMergeMode() bool {
	if e.hugeFileActive() {
		e.setStatus("merge review unavailable in huge file mode")
		return false
	}
	e.ensureConflictBlocks()
	if e.hasConflictBlocks() {
		e.mode = ModeMerge
		e.activateMergeReview()
		return false
	}

	if err := e.refreshGitChangesIfStale(2 * time.Second); err != nil {
		e.setStatus(err.Error())
		return false
	}
	if e.document.dirty {
		e.setStatus("save or reload file before diff merge review")
		return false
	}
	if !e.gitDiffHasCurrentFileHunks() {
		e.setStatus("no conflicts or git changes for current file")
		return false
	}
	e.mode = ModeMerge
	if !e.activateGitDiffPreview() {
		e.mode = ModeNormal
		e.setStatus("unable to build diff preview")
		return false
	}
	e.setStatus("diff review")
	return false
}

func (e *Editor) resolveConflictAtCursor(accept bool) bool {
	idx, kind := e.conflictBlockIndexAtRow(e.cursor.Row)
	if idx < 0 {
		return false
	}
	if kind == conflictLocal {
		return e.resolveConflictBlock(idx, accept)
	} else if kind == conflictRemote {
		return e.resolveConflictBlock(idx, !accept)
	}
	return false
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
	if idx < 0 || idx >= len(e.conflicts.blocks) {
		return
	}
	e.conflicts.blocks = append(e.conflicts.blocks[:idx], e.conflicts.blocks[idx+1:]...)
	if deletedLines == 0 {
		return
	}
	for i := idx; i < len(e.conflicts.blocks); i++ {
		block := &e.conflicts.blocks[i]
		if block.start > deleteEnd {
			shiftBlock(block, deletedLines)
		}
	}
}
