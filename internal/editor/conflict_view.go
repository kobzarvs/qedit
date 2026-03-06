package editor

import (
	"sort"
	"strings"
)

type conflictLineKind uint8

const (
	conflictNone conflictLineKind = iota
	conflictMarkerStart
	conflictMarkerSep
	conflictMarkerEnd
	conflictLocal
	conflictRemote
)

type conflictBlock struct {
	start       int
	end         int
	localStart  int
	localEnd    int
	remoteStart int
	remoteEnd   int
	localLabel  string
	remoteLabel string
}

func (e *Editor) resetConflictBlocks() {
	e.conflicts.blocks = nil
	e.conflicts.dirty = true
}

func (e *Editor) hasConflictBlocks() bool {
	return len(e.conflicts.blocks) > 0
}

func (e *Editor) markConflictBlocksDirty(startRow, endRow int) {
	if len(e.conflicts.blocks) == 0 {
		return
	}
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	for _, block := range e.conflicts.blocks {
		if endRow >= block.start && startRow <= block.end {
			e.conflicts.dirty = true
			return
		}
	}
}

func (e *Editor) adjustConflictBlocksForEdit(startRow, oldEndRow, newEndRow int) {
	if len(e.conflicts.blocks) == 0 {
		return
	}
	editStart := startRow
	editEnd := oldEndRow
	if editEnd < editStart {
		editEnd = editStart
	}
	delta := newEndRow - oldEndRow
	if delta == 0 && (editEnd < editStart || editStart > editEnd) {
		return
	}
	updated := make([]conflictBlock, 0, len(e.conflicts.blocks))
	for _, block := range e.conflicts.blocks {
		if editEnd < block.start {
			if delta != 0 {
				shiftBlock(&block, delta)
			}
			updated = append(updated, block)
			continue
		}
		if editStart > block.end {
			updated = append(updated, block)
			continue
		}
	}
	e.conflicts.blocks = updated
	if len(e.conflicts.blocks) == 0 && e.mode == ModeMerge {
		e.mode = ModeNormal
	}
}

func (e *Editor) ensureConflictBlocks() {
	if !e.conflicts.dirty {
		return
	}
	lineCount := e.LineCount()
	if lineCount == 0 {
		e.conflicts.blocks = nil
		e.conflicts.dirty = false
		return
	}
	blocks := make([]conflictBlock, 0, 1)
	startMarker := -1
	sepMarker := -1
	localLabel := ""
	remoteLabel := ""
	inBlock := false
	for i := 0; i < lineCount; i++ {
		line := e.text.Line(i)
		kind, label := parseConflictMarker(line)
		switch kind {
		case conflictMarkerStart:
			if inBlock {
				continue
			}
			startMarker = i
			sepMarker = -1
			localLabel = label
			remoteLabel = ""
			inBlock = true
		case conflictMarkerSep:
			if inBlock && sepMarker < 0 {
				sepMarker = i
			}
		case conflictMarkerEnd:
			if inBlock {
				endMarker := i
				remoteLabel = label
				if sepMarker >= 0 && endMarker > sepMarker {
					localStart := startMarker + 1
					localEnd := sepMarker - 1
					remoteStart := sepMarker + 1
					remoteEnd := endMarker - 1
					if localEnd < localStart {
						localStart = -1
						localEnd = -1
					}
					if remoteEnd < remoteStart {
						remoteStart = -1
						remoteEnd = -1
					}
					start := localStart
					end := localEnd
					if start < 0 || (remoteStart >= 0 && remoteStart < start) {
						start = remoteStart
					}
					if end < 0 || remoteEnd > end {
						end = remoteEnd
					}
					if start >= 0 && end >= start {
						blocks = append(blocks, conflictBlock{
							start:       start,
							end:         end,
							localStart:  localStart,
							localEnd:    localEnd,
							remoteStart: remoteStart,
							remoteEnd:   remoteEnd,
							localLabel:  localLabel,
							remoteLabel: remoteLabel,
						})
					}
				}
				inBlock = false
			}
		}
	}
	e.conflicts.blocks = blocks
	e.conflicts.dirty = false
}

func (e *Editor) conflictLineInfo(lineIdx int) (conflictLineKind, *conflictBlock) {
	if len(e.conflicts.blocks) == 0 {
		return conflictNone, nil
	}
	idx := sort.Search(len(e.conflicts.blocks), func(i int) bool {
		return e.conflicts.blocks[i].start > lineIdx
	}) - 1
	if idx < 0 {
		return conflictNone, nil
	}
	block := &e.conflicts.blocks[idx]
	if lineIdx < block.start || lineIdx > block.end {
		return conflictNone, nil
	}
	if block.localStart >= 0 && lineIdx >= block.localStart && lineIdx <= block.localEnd {
		return conflictLocal, block
	}
	if block.remoteStart >= 0 && lineIdx >= block.remoteStart && lineIdx <= block.remoteEnd {
		return conflictRemote, block
	}
	return conflictNone, nil
}

func (e *Editor) conflictBlockIndexAtRow(lineIdx int) (int, conflictLineKind) {
	if len(e.conflicts.blocks) == 0 {
		return -1, conflictNone
	}
	idx := sort.Search(len(e.conflicts.blocks), func(i int) bool {
		return e.conflicts.blocks[i].start > lineIdx
	}) - 1
	if idx < 0 {
		return -1, conflictNone
	}
	block := &e.conflicts.blocks[idx]
	if lineIdx < block.start || lineIdx > block.end {
		return -1, conflictNone
	}
	if block.localStart >= 0 && lineIdx >= block.localStart && lineIdx <= block.localEnd {
		return idx, conflictLocal
	}
	if block.remoteStart >= 0 && lineIdx >= block.remoteStart && lineIdx <= block.remoteEnd {
		return idx, conflictRemote
	}
	return -1, conflictNone
}

func parseConflictMarker(line []rune) (conflictLineKind, string) {
	if len(line) < 7 {
		return conflictNone, ""
	}
	if hasPrefixRunes(line, "<<<<<<<") {
		return conflictMarkerStart, strings.TrimSpace(string(line[7:]))
	}
	if hasPrefixRunes(line, "=======") {
		return conflictMarkerSep, ""
	}
	if hasPrefixRunes(line, ">>>>>>>") {
		return conflictMarkerEnd, strings.TrimSpace(string(line[7:]))
	}
	return conflictNone, ""
}

func hasPrefixRunes(line []rune, prefix string) bool {
	if len(line) < len(prefix) {
		return false
	}
	for i, r := range prefix {
		if line[i] != r {
			return false
		}
	}
	return true
}

func (e *Editor) conflictBackground(kind conflictLineKind) (Color, bool) {
	var style Style
	switch kind {
	case conflictLocal:
		style = e.styleMergeLocal
	case conflictRemote:
		style = e.styleMergeRemote
	default:
		return 0, false
	}
	if style == nil {
		return 0, false
	}
	_, bg, _ := style.Decompose()
	return bg, true
}

func (e *Editor) conflictSign(kind conflictLineKind) (rune, Style, bool) {
	switch kind {
	case conflictLocal:
		if e.styleMergeLocal == nil {
			return '-', e.styleMain, true
		}
		return '-', e.styleMergeLocal, true
	case conflictRemote:
		if e.styleMergeRemote == nil {
			return '+', e.styleMain, true
		}
		return '+', e.styleMergeRemote, true
	default:
		return ' ', nil, false
	}
}

func shiftBlock(block *conflictBlock, delta int) {
	if delta == 0 {
		return
	}
	block.start -= delta
	block.end -= delta
	if block.localStart >= 0 {
		block.localStart -= delta
		block.localEnd -= delta
	}
	if block.remoteStart >= 0 {
		block.remoteStart -= delta
		block.remoteEnd -= delta
	}
}

func (e *Editor) conflictLineOffsets(row int) (int, int) {
	localBefore := 0
	remoteBefore := 0
	for _, block := range e.conflicts.blocks {
		if block.localStart >= 0 {
			if row > block.localEnd {
				localBefore += block.localEnd - block.localStart + 1
			} else if row > block.localStart {
				localBefore += row - block.localStart
			}
		}
		if block.remoteStart >= 0 {
			if row > block.remoteEnd {
				remoteBefore += block.remoteEnd - block.remoteStart + 1
			} else if row > block.remoteStart {
				remoteBefore += row - block.remoteStart
			}
		}
	}
	return localBefore, remoteBefore
}

func buildConflictView(merged string) (string, []conflictBlock) {
	lines := strings.Split(merged, "\n")
	output := make([]string, 0, len(lines))
	blocks := make([]conflictBlock, 0, 1)
	inBlock := false
	localStart := -1
	localEnd := -1
	remoteStart := -1
	remoteEnd := -1
	localLabel := ""
	remoteLabel := ""
	for _, line := range lines {
		kind, label := parseConflictMarker([]rune(line))
		switch kind {
		case conflictMarkerStart:
			if inBlock {
				break
			}
			inBlock = true
			localStart = len(output)
			localEnd = -1
			remoteStart = -1
			remoteEnd = -1
			localLabel = label
			remoteLabel = ""
			continue
		case conflictMarkerSep:
			if inBlock {
				localEnd = len(output) - 1
				remoteStart = len(output)
			}
			continue
		case conflictMarkerEnd:
			if inBlock {
				remoteEnd = len(output) - 1
				remoteLabel = label
				if localEnd < localStart {
					localStart = -1
					localEnd = -1
				}
				if remoteEnd < remoteStart {
					remoteStart = -1
					remoteEnd = -1
				}
				start := localStart
				end := localEnd
				if start < 0 || (remoteStart >= 0 && remoteStart < start) {
					start = remoteStart
				}
				if end < 0 || remoteEnd > end {
					end = remoteEnd
				}
				if start >= 0 && end >= start {
					blocks = append(blocks, conflictBlock{
						start:       start,
						end:         end,
						localStart:  localStart,
						localEnd:    localEnd,
						remoteStart: remoteStart,
						remoteEnd:   remoteEnd,
						localLabel:  localLabel,
						remoteLabel: remoteLabel,
					})
				}
			}
			inBlock = false
			continue
		}
		output = append(output, line)
		if inBlock {
			continue
		}
	}
	return strings.Join(output, "\n"), blocks
}

func (e *Editor) drawConflictMarkerLine(s Screen, y, startX, endX int) {
	if endX <= startX {
		return
	}
	style := e.styleMain
	for x := startX; x < endX; x++ {
		s.SetContent(x, y, ' ', nil, style)
	}
}
