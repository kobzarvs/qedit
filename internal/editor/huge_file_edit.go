package editor

import (
	"errors"
	"io"
	"unicode/utf8"
)

var errHugeFileUnavailable = errors.New("huge file buffer unavailable")

type hugeFileOverlayRow struct {
	text []rune
	eol  string
}

type hugeFileRowPatch struct {
	baseStart  int
	baseDelete int
	rows       []hugeFileOverlayRow
}

type hugeFileLogicalRowRef struct {
	logicalStart int
	rowOffset    int
	baseStart    int
	baseDelete   int
	patchIndex   int
}

type hugeFileTextWindow struct {
	baseStart    int
	baseDelete   int
	logicalStart int
	start        Cursor
	end          Cursor
	startIdx     int
	endIdx       int
	rows         []hugeFileOverlayRow
}

func copyRunes(src []rune) []rune {
	if len(src) == 0 {
		return nil
	}
	return append([]rune(nil), src...)
}

func equalRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func copyHugeOverlayRows(rows []hugeFileOverlayRow) []hugeFileOverlayRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]hugeFileOverlayRow, len(rows))
	for i, row := range rows {
		out[i] = hugeFileOverlayRow{
			text: copyRunes(row.text),
			eol:  row.eol,
		}
	}
	return out
}

func equalHugeOverlayRows(a, b []hugeFileOverlayRow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].eol != b[i].eol || !equalRunes(a[i].text, b[i].text) {
			return false
		}
	}
	return true
}

func (e *Editor) hugeLineCount() int {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return 1
	}
	count := e.huge.buffer.LineCount()
	for _, patch := range e.huge.patches {
		count += len(patch.rows) - patch.baseDelete
	}
	if count < 1 {
		return 1
	}
	return count
}

func (e *Editor) invalidateHugeResolveCache() {
	e.huge.resolve = hugeFileResolveCache{}
}

func (e *Editor) hugeResolveLogicalRow(row int) (hugeFileLogicalRowRef, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil || row < 0 {
		return hugeFileLogicalRowRef{}, false
	}
	if cache := e.huge.resolve; cache.valid && row >= cache.logicalStart && row <= cache.logicalEnd {
		if cache.patchIndex >= 0 {
			return hugeFileLogicalRowRef{
				logicalStart: cache.logicalStart,
				rowOffset:    row - cache.logicalStart,
				baseStart:    cache.baseStart,
				baseDelete:   cache.baseDelete,
				patchIndex:   cache.patchIndex,
			}, true
		}
		return hugeFileLogicalRowRef{
			logicalStart: row,
			rowOffset:    0,
			baseStart:    cache.baseStart + (row - cache.logicalStart),
			baseDelete:   1,
			patchIndex:   -1,
		}, true
	}
	baseLineCount := e.huge.buffer.LineCount()
	logicalRow := 0
	baseRow := 0
	for i, patch := range e.huge.patches {
		unchanged := patch.baseStart - baseRow
		if row < logicalRow+unchanged {
			targetBaseRow := baseRow + (row - logicalRow)
			e.huge.resolve = hugeFileResolveCache{
				valid:        true,
				logicalStart: logicalRow,
				logicalEnd:   logicalRow + unchanged - 1,
				baseStart:    baseRow,
				baseDelete:   1,
				patchIndex:   -1,
			}
			return hugeFileLogicalRowRef{
				logicalStart: row,
				rowOffset:    0,
				baseStart:    targetBaseRow,
				baseDelete:   1,
				patchIndex:   -1,
			}, true
		}
		logicalRow += unchanged
		if row < logicalRow+len(patch.rows) {
			e.huge.resolve = hugeFileResolveCache{
				valid:        true,
				logicalStart: logicalRow,
				logicalEnd:   logicalRow + len(patch.rows) - 1,
				baseStart:    patch.baseStart,
				baseDelete:   patch.baseDelete,
				patchIndex:   i,
			}
			return hugeFileLogicalRowRef{
				logicalStart: logicalRow,
				rowOffset:    row - logicalRow,
				baseStart:    patch.baseStart,
				baseDelete:   patch.baseDelete,
				patchIndex:   i,
			}, true
		}
		logicalRow += len(patch.rows)
		baseRow = patch.baseStart + patch.baseDelete
	}
	if row >= logicalRow+(baseLineCount-baseRow) {
		return hugeFileLogicalRowRef{}, false
	}
	targetBaseRow := baseRow + (row - logicalRow)
	e.huge.resolve = hugeFileResolveCache{
		valid:        true,
		logicalStart: logicalRow,
		logicalEnd:   logicalRow + (baseLineCount - baseRow) - 1,
		baseStart:    baseRow,
		baseDelete:   1,
		patchIndex:   -1,
	}
	return hugeFileLogicalRowRef{
		logicalStart: row,
		rowOffset:    0,
		baseStart:    targetBaseRow,
		baseDelete:   1,
		patchIndex:   -1,
	}, true
}

func (e *Editor) hugeBaseRow(baseRow int) hugeFileOverlayRow {
	row := hugeFileOverlayRow{
		text: e.huge.buffer.Line(baseRow),
		eol:  e.huge.buffer.LineEnding(baseRow),
	}
	if line, ok := e.huge.edits[baseRow]; ok {
		row.text = copyRunes(line)
	}
	return row
}

func (e *Editor) hugeCurrentRowsForBaseInterval(baseStart, baseDelete int) []hugeFileOverlayRow {
	baseEnd := baseStart + baseDelete
	rows := make([]hugeFileOverlayRow, 0, baseDelete)
	baseRow := baseStart
	for _, patch := range e.huge.patches {
		patchEnd := patch.baseStart + patch.baseDelete
		if patchEnd <= baseStart {
			continue
		}
		if patch.baseStart >= baseEnd {
			break
		}
		for baseRow < patch.baseStart && baseRow < baseEnd {
			rows = append(rows, e.hugeBaseRow(baseRow))
			baseRow++
		}
		rows = append(rows, copyHugeOverlayRows(patch.rows)...)
		if patchEnd > baseRow {
			baseRow = patchEnd
		}
	}
	for baseRow < baseEnd {
		rows = append(rows, e.hugeBaseRow(baseRow))
		baseRow++
	}
	return rows
}

func (e *Editor) hugeBaseIntervalMatchesOriginal(baseStart, baseDelete int, rows []hugeFileOverlayRow) bool {
	if len(rows) != baseDelete {
		return false
	}
	for i := 0; i < baseDelete; i++ {
		baseRow := baseStart + i
		if !equalRunes(rows[i].text, e.huge.buffer.Line(baseRow)) {
			return false
		}
		if rows[i].eol != e.huge.buffer.LineEnding(baseRow) {
			return false
		}
	}
	return true
}

func (e *Editor) hugeApplyBaseInterval(baseStart, baseDelete int, rows []hugeFileOverlayRow) {
	baseEnd := baseStart + baseDelete
	for row := baseStart; row < baseEnd; row++ {
		delete(e.huge.edits, row)
	}

	old := append([]hugeFileRowPatch(nil), e.huge.patches...)
	replacementNeeded := !e.hugeBaseIntervalMatchesOriginal(baseStart, baseDelete, rows)
	replacement := hugeFileRowPatch{
		baseStart:  baseStart,
		baseDelete: baseDelete,
		rows:       copyHugeOverlayRows(rows),
	}

	e.huge.patches = e.huge.patches[:0]
	inserted := false
	for _, patch := range old {
		patchEnd := patch.baseStart + patch.baseDelete
		if patchEnd <= baseStart || patch.baseStart >= baseEnd {
			if replacementNeeded && !inserted && patch.baseStart > baseStart {
				e.huge.patches = append(e.huge.patches, replacement)
				inserted = true
			}
			e.huge.patches = append(e.huge.patches, patch)
			continue
		}
	}
	if replacementNeeded && !inserted {
		e.huge.patches = append(e.huge.patches, replacement)
	}
	e.normalizeHugePatches()
	e.invalidateHugeResolveCache()
}

func (e *Editor) normalizeHugePatches() {
	if len(e.huge.patches) < 2 {
		if len(e.huge.patches) == 1 && e.hugeBaseIntervalMatchesOriginal(
			e.huge.patches[0].baseStart,
			e.huge.patches[0].baseDelete,
			e.huge.patches[0].rows,
		) {
			e.huge.patches = nil
		}
		return
	}

	normalized := make([]hugeFileRowPatch, 0, len(e.huge.patches))
	current := hugeFileRowPatch{
		baseStart:  e.huge.patches[0].baseStart,
		baseDelete: e.huge.patches[0].baseDelete,
		rows:       copyHugeOverlayRows(e.huge.patches[0].rows),
	}

	appendPatch := func(patch hugeFileRowPatch) {
		if e.hugeBaseIntervalMatchesOriginal(patch.baseStart, patch.baseDelete, patch.rows) {
			return
		}
		normalized = append(normalized, patch)
	}

	for _, next := range e.huge.patches[1:] {
		if current.baseStart+current.baseDelete == next.baseStart {
			current.baseDelete += next.baseDelete
			current.rows = append(current.rows, copyHugeOverlayRows(next.rows)...)
			continue
		}
		appendPatch(current)
		current = hugeFileRowPatch{
			baseStart:  next.baseStart,
			baseDelete: next.baseDelete,
			rows:       copyHugeOverlayRows(next.rows),
		}
	}
	appendPatch(current)
	e.huge.patches = normalized
}

func (e *Editor) hugeDefaultLineEnding() string {
	if e.huge.defaultEOL != "" {
		return e.huge.defaultEOL
	}
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return "\n"
	}
	limit := e.huge.buffer.IndexedLineCount()
	if limit <= 0 {
		limit = e.huge.buffer.LineCount()
	}
	if limit > 128 {
		limit = 128
	}
	for row := 0; row < limit; row++ {
		if eol := e.huge.buffer.LineEnding(row); eol != "" {
			e.huge.defaultEOL = eol
			return eol
		}
	}
	e.huge.defaultEOL = "\n"
	return e.huge.defaultEOL
}

func (e *Editor) hugeTextWindow(start, end Cursor) (hugeFileTextWindow, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return hugeFileTextWindow{}, false
	}
	lineCount := e.LineCount()
	if start.Row < 0 || start.Row >= lineCount || end.Row < 0 || end.Row >= lineCount {
		return hugeFileTextWindow{}, false
	}
	startLen := e.lineLen(start.Row)
	endLen := e.lineLen(end.Row)
	start.Col = clampRange(start.Col, 0, startLen)
	end.Col = clampRange(end.Col, 0, endLen)
	if cursorLess(end, start) {
		return hugeFileTextWindow{}, false
	}

	startRef, ok := e.hugeResolveLogicalRow(start.Row)
	if !ok {
		return hugeFileTextWindow{}, false
	}
	endRef, ok := e.hugeResolveLogicalRow(end.Row)
	if !ok {
		return hugeFileTextWindow{}, false
	}

	baseStart := startRef.baseStart
	baseEnd := startRef.baseStart + startRef.baseDelete
	if endIntervalEnd := endRef.baseStart + endRef.baseDelete; endIntervalEnd > baseEnd {
		baseEnd = endIntervalEnd
	}
	logicalStart := startRef.logicalStart
	rows := e.hugeCurrentRowsForBaseInterval(baseStart, baseEnd-baseStart)
	startIdx := start.Row - logicalStart
	endIdx := end.Row - logicalStart
	if startIdx < 0 || endIdx < startIdx || endIdx >= len(rows) {
		return hugeFileTextWindow{}, false
	}

	return hugeFileTextWindow{
		baseStart:    baseStart,
		baseDelete:   baseEnd - baseStart,
		logicalStart: logicalStart,
		start:        start,
		end:          end,
		startIdx:     startIdx,
		endIdx:       endIdx,
		rows:         rows,
	}, true
}

func hugeWindowDeletedRunes(win hugeFileTextWindow) []rune {
	if win.startIdx == win.endIdx && win.start.Col >= win.end.Col {
		return nil
	}
	var deleted []rune
	for row := win.startIdx; row <= win.endIdx; row++ {
		line := win.rows[row].text
		startCol := 0
		endCol := len(line)
		if row == win.startIdx {
			startCol = win.start.Col
		}
		if row == win.endIdx {
			endCol = win.end.Col
		}
		deleted = append(deleted, line[startCol:endCol]...)
		if row < win.endIdx {
			deleted = append(deleted, '\n')
		}
	}
	return deleted
}

func hugeInsertedEndPos(start Cursor, replacement [][]rune) Cursor {
	if len(replacement) == 0 {
		return start
	}
	if len(replacement) == 1 {
		return Cursor{Row: start.Row, Col: start.Col + len(replacement[0])}
	}
	return Cursor{Row: start.Row + len(replacement) - 1, Col: len(replacement[len(replacement)-1])}
}

func (e *Editor) hugeReplacementRows(win hugeFileTextWindow, replacement [][]rune) []hugeFileOverlayRow {
	before := copyHugeOverlayRows(win.rows[:win.startIdx])
	after := copyHugeOverlayRows(win.rows[win.endIdx+1:])
	prefix := copyRunes(win.rows[win.startIdx].text[:win.start.Col])
	suffix := copyRunes(win.rows[win.endIdx].text[win.end.Col:])
	suffixEOL := win.rows[win.endIdx].eol

	replaced := make([]hugeFileOverlayRow, 0, len(replacement)+1)
	switch len(replacement) {
	case 0:
		line := append(prefix, suffix...)
		replaced = append(replaced, hugeFileOverlayRow{text: line, eol: suffixEOL})
	case 1:
		line := append(prefix, replacement[0]...)
		line = append(line, suffix...)
		replaced = append(replaced, hugeFileOverlayRow{text: line, eol: suffixEOL})
	default:
		lineEOL := e.hugeDefaultLineEnding()
		first := append(prefix, replacement[0]...)
		replaced = append(replaced, hugeFileOverlayRow{text: first, eol: lineEOL})
		for _, line := range replacement[1 : len(replacement)-1] {
			replaced = append(replaced, hugeFileOverlayRow{
				text: copyRunes(line),
				eol:  lineEOL,
			})
		}
		last := append(copyRunes(replacement[len(replacement)-1]), suffix...)
		replaced = append(replaced, hugeFileOverlayRow{text: last, eol: suffixEOL})
	}

	rows := make([]hugeFileOverlayRow, 0, len(before)+len(replaced)+len(after))
	rows = append(rows, before...)
	rows = append(rows, replaced...)
	rows = append(rows, after...)
	return rows
}

func (e *Editor) hugeCollectDeletedText(start, end Cursor) [][]rune {
	win, ok := e.hugeTextWindow(start, end)
	if !ok {
		return nil
	}
	deleted := hugeWindowDeletedRunes(win)
	if len(deleted) == 0 {
		return nil
	}
	return splitRunesByNewline(deleted)
}

func (e *Editor) hugeReplaceTextRange(start, end Cursor, replacement [][]rune) (Cursor, [][]rune, bool) {
	win, ok := e.hugeTextWindow(start, end)
	if !ok {
		return start, nil, false
	}
	if win.start == win.end && len(replacement) == 0 {
		return win.start, nil, true
	}

	deletedRunes := hugeWindowDeletedRunes(win)
	deleted := splitRunesByNewline(deletedRunes)
	rows := e.hugeReplacementRows(win, replacement)
	e.hugeApplyBaseInterval(win.baseStart, win.baseDelete, rows)
	e.change.lastEdit.Valid = false
	return hugeInsertedEndPos(win.start, replacement), deleted, true
}

func (e *Editor) hugeLine(row int) ([]rune, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return nil, false
	}
	ref, ok := e.hugeResolveLogicalRow(row)
	if !ok {
		return nil, false
	}
	if ref.patchIndex >= 0 {
		return copyRunes(e.huge.patches[ref.patchIndex].rows[ref.rowOffset].text), true
	}
	if line, ok := e.huge.edits[ref.baseStart]; ok {
		return copyRunes(line), true
	}
	return e.huge.buffer.Line(ref.baseStart), true
}

func (e *Editor) hugeTryLine(row int) ([]rune, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return nil, false
	}
	ref, ok := e.hugeResolveLogicalRow(row)
	if !ok {
		return nil, false
	}
	if ref.patchIndex >= 0 {
		return copyRunes(e.huge.patches[ref.patchIndex].rows[ref.rowOffset].text), true
	}
	if line, ok := e.huge.edits[ref.baseStart]; ok {
		return copyRunes(line), true
	}
	return e.huge.buffer.TryLine(ref.baseStart)
}

func (e *Editor) hugeLineLen(row int) (int, bool) {
	info, ok := e.hugeLineInfo(row)
	if !ok {
		return 0, false
	}
	return info.runeLen, true
}

func (e *Editor) hugeLineInfoQuick(row int) (hugeFileLineInfo, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return hugeFileLineInfo{}, false
	}
	ref, ok := e.hugeResolveLogicalRow(row)
	if !ok {
		return hugeFileLineInfo{}, false
	}
	if ref.patchIndex >= 0 {
		return analyzeHugeRunes(e.huge.patches[ref.patchIndex].rows[ref.rowOffset].text), true
	}
	if line, ok := e.huge.edits[ref.baseStart]; ok {
		return analyzeHugeRunes(line), true
	}
	return e.huge.buffer.TryCachedLineInfo(ref.baseStart)
}

func (e *Editor) hugeLineInfo(row int) (hugeFileLineInfo, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return hugeFileLineInfo{}, false
	}
	ref, ok := e.hugeResolveLogicalRow(row)
	if !ok {
		return hugeFileLineInfo{}, false
	}
	if ref.patchIndex >= 0 {
		return analyzeHugeRunes(e.huge.patches[ref.patchIndex].rows[ref.rowOffset].text), true
	}
	if line, ok := e.huge.edits[ref.baseStart]; ok {
		return analyzeHugeRunes(line), true
	}
	if !e.huge.buffer.canResolveLineQuick(ref.baseStart) {
		return hugeFileLineInfo{}, false
	}
	return e.huge.buffer.LineInfo(ref.baseStart)
}

func (e *Editor) hugeLineSegment(row, startCol, maxCols int) ([]rune, int, int, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return nil, 0, 0, false
	}
	if maxCols <= 0 {
		return []rune{}, startCol, startCol, true
	}
	if startCol < 0 {
		startCol = 0
	}
	ref, ok := e.hugeResolveLogicalRow(row)
	if !ok {
		return nil, 0, 0, false
	}
	if ref.patchIndex >= 0 {
		return hugeOverlaySegment(e.huge.patches[ref.patchIndex].rows[ref.rowOffset].text, startCol, maxCols)
	}
	if line, ok := e.huge.edits[ref.baseStart]; ok {
		return hugeOverlaySegment(line, startCol, maxCols)
	}
	info, ok := e.huge.buffer.LineInfo(ref.baseStart)
	if !ok {
		return nil, 0, 0, false
	}
	// ASCII line with tabs: lazy tab expansion over the visible window only.
	if info.hasTabs && info.asciiOnly {
		segment, logicalStart, visualStart, ok := e.huge.buffer.LineSegmentWithTabs(ref.baseStart, startCol, maxCols, e.display.tabWidth)
		if ok {
			return segment, logicalStart, visualStart, true
		}
		return nil, 0, 0, false
	}
	if info.hasTabs {
		return nil, 0, 0, false
	}
	if info.asciiOnly {
		segment, ok := e.huge.buffer.LineSegment(ref.baseStart, startCol, maxCols)
		if !ok {
			return nil, 0, 0, false
		}
		return segment, startCol, startCol, true
	}
	// Non-ASCII line without tabs: try partial ASCII segment if the visible
	// byte window is entirely within an ASCII region of the line.
	segment, ok := e.huge.buffer.LineSegmentASCIIWindow(ref.baseStart, startCol, maxCols)
	if !ok {
		return nil, 0, 0, false
	}
	return segment, startCol, startCol, true
}

func (e *Editor) hugeLinePrefix(row, maxBytes int) ([]rune, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return nil, false
	}
	ref, ok := e.hugeResolveLogicalRow(row)
	if !ok {
		return nil, false
	}
	if ref.patchIndex >= 0 {
		line := e.huge.patches[ref.patchIndex].rows[ref.rowOffset].text
		if len(line) <= maxBytes {
			return append([]rune(nil), line...), true
		}
		return append([]rune(nil), line[:maxBytes]...), true
	}
	if line, ok := e.huge.edits[ref.baseStart]; ok {
		if len(line) <= maxBytes {
			return append([]rune(nil), line...), true
		}
		return append([]rune(nil), line[:maxBytes]...), true
	}
	return e.huge.buffer.LinePrefix(ref.baseStart, maxBytes)
}

func analyzeHugeRunes(line []rune) hugeFileLineInfo {
	info := hugeFileLineInfo{
		runeLen:   len(line),
		asciiOnly: true,
	}
	for _, r := range line {
		if r == '\t' {
			info.hasTabs = true
		}
		if r >= utf8.RuneSelf {
			info.asciiOnly = false
		}
	}
	return info
}

func hugeOverlaySegment(line []rune, startCol, maxCols int) ([]rune, int, int, bool) {
	if maxCols <= 0 {
		return []rune{}, startCol, startCol, true
	}
	info := analyzeHugeRunes(line)
	if info.hasTabs {
		return nil, 0, 0, false
	}
	if startCol < 0 {
		startCol = 0
	}
	if startCol > len(line) {
		startCol = len(line)
	}
	endCol := startCol + maxCols
	if endCol > len(line) {
		endCol = len(line)
	}
	if endCol <= startCol {
		return []rune{}, startCol, startCol, true
	}
	return append([]rune(nil), line[startCol:endCol]...), startCol, startCol, true
}

func (e *Editor) hugeSetLine(row int, line []rune) bool {
	if !e.hugeFileActive() || e.huge.buffer == nil || row < 0 || row >= e.LineCount() {
		return false
	}
	ref, ok := e.hugeResolveLogicalRow(row)
	if !ok {
		return false
	}
	if ref.patchIndex >= 0 {
		patch := e.huge.patches[ref.patchIndex]
		rows := copyHugeOverlayRows(patch.rows)
		rows[ref.rowOffset].text = copyRunes(line)
		e.hugeApplyBaseInterval(patch.baseStart, patch.baseDelete, rows)
		return true
	}

	base := e.huge.buffer.Line(ref.baseStart)
	if equalRunes(base, line) {
		delete(e.huge.edits, ref.baseStart)
		return true
	}
	if e.huge.edits == nil {
		e.huge.edits = make(map[int][]rune)
	}
	e.huge.edits[ref.baseStart] = copyRunes(line)
	return true
}

func (e *Editor) hugeSplitLineAt(pos Cursor) bool {
	ref, ok := e.hugeResolveLogicalRow(pos.Row)
	if !ok {
		return false
	}
	rows := e.hugeCurrentRowsForBaseInterval(ref.baseStart, ref.baseDelete)
	if ref.rowOffset < 0 || ref.rowOffset >= len(rows) {
		return false
	}
	line := rows[ref.rowOffset].text
	col := clampRange(pos.Col, 0, len(line))
	newRows := make([]hugeFileOverlayRow, 0, len(rows)+1)
	newRows = append(newRows, copyHugeOverlayRows(rows[:ref.rowOffset])...)
	newRows = append(newRows, hugeFileOverlayRow{
		text: copyRunes(line[:col]),
		eol:  e.hugeDefaultLineEnding(),
	})
	newRows = append(newRows, hugeFileOverlayRow{
		text: copyRunes(line[col:]),
		eol:  rows[ref.rowOffset].eol,
	})
	newRows = append(newRows, copyHugeOverlayRows(rows[ref.rowOffset+1:])...)
	e.hugeApplyBaseInterval(ref.baseStart, ref.baseDelete, newRows)
	e.cursor = Cursor{Row: pos.Row + 1, Col: 0}
	e.change.lastEdit.Valid = false
	return true
}

func (e *Editor) hugeJoinLineAt(pos Cursor) bool {
	if pos.Row < 0 || pos.Row+1 >= e.LineCount() {
		return false
	}
	startRef, ok := e.hugeResolveLogicalRow(pos.Row)
	if !ok {
		return false
	}
	nextRef, ok := e.hugeResolveLogicalRow(pos.Row + 1)
	if !ok {
		return false
	}

	baseStart := startRef.baseStart
	baseEnd := startRef.baseStart + startRef.baseDelete
	nextEnd := nextRef.baseStart + nextRef.baseDelete
	if nextEnd > baseEnd {
		baseEnd = nextEnd
	}
	rows := e.hugeCurrentRowsForBaseInterval(baseStart, baseEnd-baseStart)
	startIdx := pos.Row - startRef.logicalStart
	if startIdx < 0 || startIdx+1 >= len(rows) {
		return false
	}
	joined := hugeFileOverlayRow{
		text: append(copyRunes(rows[startIdx].text), rows[startIdx+1].text...),
		eol:  rows[startIdx+1].eol,
	}
	newRows := make([]hugeFileOverlayRow, 0, len(rows)-1)
	newRows = append(newRows, copyHugeOverlayRows(rows[:startIdx])...)
	newRows = append(newRows, joined)
	newRows = append(newRows, copyHugeOverlayRows(rows[startIdx+2:])...)
	e.hugeApplyBaseInterval(baseStart, baseEnd-baseStart, newRows)
	e.cursor = Cursor{Row: pos.Row, Col: pos.Col}
	e.change.lastEdit.Valid = false
	return true
}

func (e *Editor) clearHugeEdits() {
	if !e.hugeFileActive() {
		return
	}
	e.huge.edits = nil
	e.huge.patches = nil
	e.huge.defaultEOL = ""
	e.invalidateHugeResolveCache()
}

func (e *Editor) copyHugeBaseRows(reader io.ReadSeeker, w io.Writer, startRow, endRow int) error {
	if startRow >= endRow {
		return nil
	}
	start, end, err := e.huge.buffer.RawSpanForRows(startRow, endRow)
	if err != nil {
		return err
	}
	if end <= start {
		return nil
	}
	if _, err := reader.Seek(start, io.SeekStart); err != nil {
		return err
	}
	_, err = io.CopyN(w, reader, end-start)
	return err
}

func (e *Editor) writeHugeBaseRange(reader io.ReadSeeker, w io.Writer, startRow, endRow int, edits map[int]string) error {
	if startRow >= endRow {
		return nil
	}
	pendingStart := startRow
	for row := startRow; row < endRow; row++ {
		replacement, edited := edits[row]
		if !edited {
			continue
		}
		if err := e.copyHugeBaseRows(reader, w, pendingStart, row); err != nil {
			return err
		}
		if _, err := io.WriteString(w, replacement); err != nil {
			return err
		}
		if _, err := io.WriteString(w, e.huge.buffer.LineEnding(row)); err != nil {
			return err
		}
		delete(edits, row)
		pendingStart = row + 1
	}
	return e.copyHugeBaseRows(reader, w, pendingStart, endRow)
}

func writeHugeOverlayRow(w io.Writer, row hugeFileOverlayRow) error {
	if _, err := io.WriteString(w, string(row.text)); err != nil {
		return err
	}
	if row.eol == "" {
		return nil
	}
	_, err := io.WriteString(w, row.eol)
	return err
}

func (e *Editor) WriteHugeFileTo(w io.Writer) error {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return errHugeFileUnavailable
	}
	reader, err := e.huge.buffer.OpenReader()
	if err != nil {
		return err
	}
	defer reader.Close()

	edits := make(map[int]string, len(e.huge.edits))
	for row, line := range e.huge.edits {
		edits[row] = string(line)
	}
	patches := append([]hugeFileRowPatch(nil), e.huge.patches...)

	currentBaseRow := 0
	baseLineCount := e.huge.buffer.LineCount()
	for _, patch := range patches {
		if patch.baseStart > currentBaseRow {
			if err := e.writeHugeBaseRange(reader, w, currentBaseRow, patch.baseStart, edits); err != nil {
				return err
			}
		}
		for _, row := range patch.rows {
			if err := writeHugeOverlayRow(w, row); err != nil {
				return err
			}
		}
		currentBaseRow = patch.baseStart + patch.baseDelete
	}
	if err := e.writeHugeBaseRange(reader, w, currentBaseRow, baseLineCount, edits); err != nil {
		return err
	}
	if len(edits) > 0 {
		return errors.New("huge file save failed: unresolved edited rows")
	}
	return nil
}

func (e *Editor) WriteHugeFile(path string, store FileStore) error {
	if !e.hugeFileActive() {
		return errors.New("huge file mode is not active")
	}
	if store == nil {
		return errFileStoreUnavailable()
	}
	pr, pw := io.Pipe()
	writeErrCh := make(chan error, 1)
	go func() {
		err := e.WriteHugeFileTo(pw)
		_ = pw.CloseWithError(err)
		writeErrCh <- err
	}()

	writeErr := store.WriteFrom(path, pr)
	streamErr := <-writeErrCh
	if writeErr != nil {
		return writeErr
	}
	if streamErr != nil {
		return streamErr
	}
	return e.ReloadSavedHugeFile(path, store)
}

func (e *Editor) ReloadSavedHugeFile(path string, store FileStore) error {
	if !e.hugeFileActive() {
		return errors.New("huge file mode is not active")
	}
	if store == nil {
		return errFileStoreUnavailable()
	}
	meta, err := store.Stat(path)
	if err != nil {
		return err
	}
	buffer, err := OpenHugeFileBuffer(path, meta, store)
	if err != nil {
		return err
	}

	oldBuffer := e.huge.buffer
	e.huge.active = true
	e.huge.sizeBytes = meta.Size
	e.huge.buffer = buffer
	e.clearHugeEdits()
	e.clearGitDiffPreview()
	e.document.filename = path
	e.file.readOnly = false
	e.file.externalChange = ExternalChangeNone
	e.file.diskContent = ""
	e.file.snapshot = snapshotFromMetadata(meta)
	e.savePoint = len(e.undo)
	e.updateDirty()
	_ = e.SaveUndoHistory()
	e.saveSessionState()
	if e.buffers != nil && e.buffers.Count() > 0 {
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
	}
	e.primeHugeViewport()
	if oldBuffer != nil {
		_ = oldBuffer.Close()
	}
	return nil
}
