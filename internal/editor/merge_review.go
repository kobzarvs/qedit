package editor

import "fmt"

func (e *Editor) mergeReviewActive() bool {
	return e.conflicts.review.active && e.mode == ModeMerge && e.hasConflictBlocks() && !e.gitDiffPreviewActive()
}

func (e *Editor) activateMergeReview() {
	e.conflicts.review.active = true
	if e.conflicts.review.pane != mergeReviewPaneLocal && e.conflicts.review.pane != mergeReviewPaneRemote {
		e.conflicts.review.pane = mergeReviewPaneRemote
	}
	e.focusEditor()
}

func (e *Editor) deactivateMergeReview() {
	e.conflicts.review = editorMergeReviewState{}
}

func (e *Editor) setMergeReviewPane(pane mergeReviewPane) {
	e.conflicts.review.pane = pane
}

func (e *Editor) cycleMergeReviewPane() {
	if e.conflicts.review.pane == mergeReviewPaneLocal {
		e.conflicts.review.pane = mergeReviewPaneRemote
		return
	}
	e.conflicts.review.pane = mergeReviewPaneLocal
}

func (e *Editor) currentMergeReviewBlockIndex() int {
	if len(e.conflicts.blocks) == 0 {
		return -1
	}
	if idx, _ := e.conflictBlockIndexAtRow(e.cursor.Row); idx >= 0 {
		return idx
	}
	bestIdx := 0
	bestDistance := int(^uint(0) >> 1)
	for i, block := range e.conflicts.blocks {
		dist := 0
		if e.cursor.Row < block.start {
			dist = block.start - e.cursor.Row
		} else if e.cursor.Row > block.end {
			dist = e.cursor.Row - block.end
		}
		if dist < bestDistance {
			bestDistance = dist
			bestIdx = i
		}
	}
	return bestIdx
}

func (e *Editor) applySelectedMergeReviewPane() bool {
	switch e.conflicts.review.pane {
	case mergeReviewPaneLocal:
		return e.applyMergeReviewPane(mergeReviewPaneLocal)
	case mergeReviewPaneRemote:
		return e.applyMergeReviewPane(mergeReviewPaneRemote)
	default:
		e.setStatus("select CURRENT or LATEST pane first")
		return false
	}
}

func (e *Editor) applyMergeReviewPane(pane mergeReviewPane) bool {
	idx := e.currentMergeReviewBlockIndex()
	if idx < 0 {
		e.setStatus("no conflict at cursor")
		return false
	}
	switch pane {
	case mergeReviewPaneLocal:
		return e.resolveConflictBlock(idx, true)
	case mergeReviewPaneRemote:
		return e.resolveConflictBlock(idx, false)
	default:
		e.setStatus("cannot apply RESULT pane")
		return false
	}
}

func (e *Editor) resolveConflictBlock(idx int, chooseLocal bool) bool {
	if idx < 0 || idx >= len(e.conflicts.blocks) {
		return false
	}
	block := e.conflicts.blocks[idx]
	deleteStart := -1
	deleteEnd := -1
	switch {
	case chooseLocal:
		deleteStart = block.remoteStart
		deleteEnd = block.remoteEnd
	case !chooseLocal:
		deleteStart = block.localStart
		deleteEnd = block.localEnd
	}

	deletedLines := 0
	if deleteStart >= 0 && deleteEnd >= deleteStart {
		deletedLines = e.deleteConflictLines(deleteStart, deleteEnd)
	}
	e.removeConflictBlock(idx, deleteEnd, deletedLines)
	e.conflicts.dirty = false
	e.cursor.Row = block.start
	if e.cursor.Row >= e.LineCount() {
		e.cursor.Row = e.LineCount() - 1
	}
	if e.cursor.Row < 0 {
		e.cursor.Row = 0
	}
	e.cursor.Col = 0
	e.clampCursorCol()
	e.ensureCursorVisible(e.viewHeightCached())
	if len(e.conflicts.blocks) == 0 {
		e.deactivateMergeReview()
		if e.mode == ModeMerge {
			e.mode = ModeNormal
		}
	}
	return true
}

func (e *Editor) mergeReviewBlock() (*conflictBlock, int) {
	idx := e.currentMergeReviewBlockIndex()
	if idx < 0 || idx >= len(e.conflicts.blocks) {
		return nil, -1
	}
	return &e.conflicts.blocks[idx], idx
}

func (e *Editor) mergeReviewBlockContainsRow(row int) bool {
	block, _ := e.mergeReviewBlock()
	if block == nil {
		return false
	}
	return row >= block.start && row <= block.end
}

func (e *Editor) gotoMergeReviewConflict(forward bool) bool {
	if len(e.conflicts.blocks) == 0 {
		e.setStatus("no conflicts")
		return false
	}
	currentIdx := e.currentMergeReviewBlockIndex()
	if currentIdx < 0 {
		currentIdx = 0
	}
	targetIdx := currentIdx
	if forward {
		targetIdx++
		if targetIdx >= len(e.conflicts.blocks) {
			targetIdx = 0
		}
	} else {
		targetIdx--
		if targetIdx < 0 {
			targetIdx = len(e.conflicts.blocks) - 1
		}
	}
	block := e.conflicts.blocks[targetIdx]
	e.cursor.Row = block.start
	e.cursor.Col = 0
	e.clampCursorCol()
	e.ensureCursorVisible(e.viewHeightCached())
	e.setStatus(fmt.Sprintf("Conflict %d/%d", targetIdx+1, len(e.conflicts.blocks)))
	return false
}

func (e *Editor) mergeReviewPaneLabel(pane mergeReviewPane) string {
	block, _ := e.mergeReviewBlock()
	switch pane {
	case mergeReviewPaneLocal:
		if block != nil && block.localLabel != "" {
			return "CURRENT " + block.localLabel
		}
		return "CURRENT"
	case mergeReviewPaneRemote:
		if block != nil && block.remoteLabel != "" {
			return "LATEST " + block.remoteLabel
		}
		return "LATEST"
	default:
		return "RESULT"
	}
}

func (e *Editor) mergeReviewStatus() string {
	if !e.mergeReviewActive() {
		return ""
	}
	_, idx := e.mergeReviewBlock()
	if idx < 0 {
		return "Merge review"
	}
	selected := e.mergeReviewPaneLabel(e.conflicts.review.pane)
	if selected == "RESULT" {
		selected = "RESULT"
	}
	return fmt.Sprintf("Conflict %d/%d | Selected: %s | Enter/click apply | F3/Shift+F3 navigate", idx+1, len(e.conflicts.blocks), selected)
}
