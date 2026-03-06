package editor

import (
	"path/filepath"
	"strings"
	"time"
)

func (e *Editor) insertRune(r rune) {
	pos := e.cursor
	if !e.insertRuneAt(pos, r) {
		return
	}
	e.recordUndo(action{kind: actionDeleteRune, pos: pos, r: r})
}
func (e *Editor) insertTab() {
	e.insertRune('\t')
}
func (e *Editor) insertRuneAt(pos Cursor, r rune) bool {
	if pos.Row < 0 || pos.Row >= e.LineCount() {
		return false
	}
	lineLen := e.lineLen(pos.Row)
	if pos.Col < 0 {
		pos.Col = 0
	}
	if pos.Col > lineLen {
		pos.Col = lineLen
	}
	e.recordTextEdit(pos, pos, Cursor{Row: pos.Row, Col: pos.Col + 1}, runeByteLen(r))
	index := e.text.LineStartIndex(pos.Row) + pos.Col
	e.text.Insert(index, []rune{r})
	e.cursor = Cursor{Row: pos.Row, Col: pos.Col + 1}
	return true
}

// insertTextAt inserts multiple lines at the given position and returns the end position.
// This is a bulk operation for efficiency with large text blocks.
func (e *Editor) insertTextAt(pos Cursor, text [][]rune) Cursor {
	if len(text) == 0 || pos.Row < 0 || pos.Row >= e.LineCount() {
		return pos
	}
	lineLen := e.lineLen(pos.Row)
	if pos.Col < 0 {
		pos.Col = 0
	}
	if pos.Col > lineLen {
		pos.Col = lineLen
	}
	runes := joinText(text)
	index := e.text.LineStartIndex(pos.Row) + pos.Col
	e.text.Insert(index, runes)
	return Cursor{Row: pos.Row + len(text) - 1, Col: len(text[len(text)-1])}
}

// deleteTextRange deletes text from start to end position and returns the deleted text.
// This is a bulk operation for efficiency with large text blocks.
func (e *Editor) deleteTextRange(start, end Cursor) [][]rune {
	lineCount := e.LineCount()
	if start.Row < 0 || end.Row >= lineCount || start.Row > end.Row {
		return nil
	}
	startLen := e.lineLen(start.Row)
	endLen := e.lineLen(end.Row)
	start.Col = clampRange(start.Col, 0, startLen)
	end.Col = clampRange(end.Col, 0, endLen)
	if start.Row == end.Row && start.Col >= end.Col {
		return nil
	}
	startIndex := e.text.LineStartIndex(start.Row) + start.Col
	endIndex := e.text.LineStartIndex(end.Row) + end.Col
	deleted := e.text.DeleteRange(startIndex, endIndex)
	e.cursor = start
	return splitRunesByNewline(deleted)
}
func (e *Editor) insertNewline() {
	pos := e.cursor
	if !e.splitLineAt(pos) {
		return
	}
	e.recordUndo(action{kind: actionJoinLine, pos: pos})
}
func (e *Editor) splitLineAt(pos Cursor) bool {
	if pos.Row < 0 || pos.Row >= e.LineCount() {
		return false
	}
	lineLen := e.lineLen(pos.Row)
	if pos.Col < 0 {
		pos.Col = 0
	}
	if pos.Col > lineLen {
		pos.Col = lineLen
	}
	e.recordTextEdit(pos, pos, Cursor{Row: pos.Row + 1, Col: 0}, 1)
	index := e.text.LineStartIndex(pos.Row) + pos.Col
	e.text.Insert(index, []rune{'\n'})

	e.cursor = Cursor{Row: pos.Row + 1, Col: 0}
	return true
}
func (e *Editor) backspace() {
	if e.cursor.Col > 0 {
		pos := Cursor{Row: e.cursor.Row, Col: e.cursor.Col - 1}
		lineLen := e.lineLen(pos.Row)
		if pos.Col >= lineLen {
			pos.Col = lineLen - 1
		}
		if pos.Col < 0 {
			return
		}
		line := e.line(pos.Row)
		r := line[pos.Col]
		if !e.deleteRuneAt(pos) {
			return
		}
		e.recordUndo(action{kind: actionInsertRune, pos: pos, r: r})
		return
	}
	if e.cursor.Row == 0 {
		return
	}
	pos := Cursor{Row: e.cursor.Row - 1, Col: e.lineLen(e.cursor.Row - 1)}
	if !e.joinLineAt(pos) {
		return
	}
	e.recordUndo(action{kind: actionSplitLine, pos: pos})
}
func (e *Editor) deleteLine() {
	// If there's a selection, delete the selected text (same as 'd' key)
	if start, end, ok := e.selectionRange(); ok {
		e.deleteSelection(start, end, true) // Restore selection on undo
		e.clearSelection()
		e.modal.selectMode = false
		return
	}

	lineCount := e.LineCount()
	if lineCount == 0 {
		return
	}
	row := e.cursor.Row
	if row < 0 || row >= lineCount {
		return
	}

	line := e.line(row)

	if lineCount == 1 {
		// Only one line - just clear it
		if len(line) == 0 {
			return
		}
		// Use deleteSelection for consistency, no selection restore
		e.deleteSelection(Cursor{Row: 0, Col: 0}, Cursor{Row: 0, Col: len(line)}, false)
		return
	}

	// Delete entire line including newline using deleteSelection
	var start, end Cursor
	if row < lineCount-1 {
		// Not the last line: select from start of this line to start of next
		start = Cursor{Row: row, Col: 0}
		end = Cursor{Row: row + 1, Col: 0}
	} else {
		// Last line: select from end of previous line to end of this line
		start = Cursor{Row: row - 1, Col: e.lineLen(row - 1)}
		end = Cursor{Row: row, Col: len(line)}
	}

	e.deleteSelection(start, end, false) // No selection restore for line delete

	// Adjust cursor position
	if e.cursor.Row >= lineCount {
		e.cursor.Row = lineCount - 1
		if e.cursor.Row < 0 {
			e.cursor.Row = 0
		}
	}
	e.cursor.Col = 0
	e.clampCursorCol()
}
func (e *Editor) deleteChar() {
	// If there's a selection, delete the selected text
	if start, end, ok := e.selectionRange(); ok {
		e.deleteSelection(start, end, true) // Restore selection on undo
		return
	}

	// No selection - delete character to the right
	row := e.cursor.Row
	col := e.cursor.Col
	if row < 0 || row >= e.LineCount() {
		return
	}
	line := e.line(row)

	if col < len(line) {
		// Delete character at cursor position
		pos := Cursor{Row: row, Col: col}
		r := line[col]
		if e.deleteRuneAt(pos) {
			e.recordUndo(action{kind: actionInsertRune, pos: pos, r: r})
		}
	} else if row < e.LineCount()-1 {
		// At end of line, join with next line
		pos := Cursor{Row: row, Col: len(line)}
		if e.joinLineAt(pos) {
			e.recordUndo(action{kind: actionSplitLine, pos: pos})
		}
	}
}
func (e *Editor) deleteSelection(start, end Cursor, restoreSelectionOnUndo bool) {
	lineCount := e.LineCount()
	if start.Row < 0 || end.Row >= lineCount {
		return
	}
	startLen := e.lineLen(start.Row)
	endLen := e.lineLen(end.Row)
	start.Col = clampRange(start.Col, 0, startLen)
	end.Col = clampRange(end.Col, 0, endLen)
	if start.Row == end.Row && start.Col >= end.Col {
		return
	}

	// Calculate byte offsets BEFORE making changes
	startByte, startColBytes := e.byteOffset(start)
	oldEndByte, oldEndColBytes := e.byteOffset(end)

	// Collect deleted content for undo
	// Use bulk operation for efficiency with large selections
	deleted := e.collectDeletedText(start, end)

	e.startUndoGroup()
	// Record as a single bulk insert action for undo
	e.appendUndo(action{
		kind:           actionInsertText,
		pos:            start,
		text:           deleted,
		selectionStart: start,
		selectionEnd:   end,
		hasSelection:   restoreSelectionOnUndo,
	})
	e.finishUndoGroup()

	// Record text edit for tree-sitter
	e.lastEdit = TextEdit{
		Valid:          true,
		StartByte:      startByte,
		OldEndByte:     oldEndByte,
		NewEndByte:     startByte, // Nothing inserted
		StartRow:       start.Row,
		StartColBytes:  startColBytes,
		OldEndRow:      end.Row,
		OldEndColBytes: oldEndColBytes,
		NewEndRow:      start.Row,
		NewEndColBytes: startColBytes,
	}

	startIndex := e.text.LineStartIndex(start.Row) + start.Col
	endIndex := e.text.LineStartIndex(end.Row) + end.Col
	e.text.DeleteRange(startIndex, endIndex)

	e.cursor = start
	e.clearSelection()
	e.changeTick++
	e.updateDirty()
}

// collectDeletedText collects text from start to end position without modifying the buffer.
func (e *Editor) collectDeletedText(start, end Cursor) [][]rune {
	startLen := e.lineLen(start.Row)
	endLen := e.lineLen(end.Row)
	start.Col = clampRange(start.Col, 0, startLen)
	end.Col = clampRange(end.Col, 0, endLen)
	startIndex := e.text.LineStartIndex(start.Row) + start.Col
	endIndex := e.text.LineStartIndex(end.Row) + end.Col
	if startIndex >= endIndex {
		return nil
	}
	deleted := e.text.Slice(startIndex, endIndex)
	return splitRunesByNewline(deleted)
}
func (e *Editor) deleteWordLeft() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}

	if e.cursor.Col == 0 {
		// At start of line - join with previous line
		if e.cursor.Row > 0 {
			// Calculate byte offset BEFORE change
			pos := Cursor{Row: e.cursor.Row - 1, Col: e.lineLen(e.cursor.Row - 1)}
			startByte, startColBytes := e.byteOffset(pos)
			oldEndByte := startByte + 1 // +1 for newline

			e.lastEdit = TextEdit{
				Valid:          true,
				StartByte:      startByte,
				OldEndByte:     oldEndByte,
				NewEndByte:     startByte,
				StartRow:       pos.Row,
				StartColBytes:  startColBytes,
				OldEndRow:      e.cursor.Row,
				OldEndColBytes: 0,
				NewEndRow:      pos.Row,
				NewEndColBytes: startColBytes,
			}

			if e.joinLineAt(pos) {
				e.recordUndo(action{kind: actionSplitLine, pos: pos})
			}
		}
		return
	}

	line := e.line(e.cursor.Row)
	endCol := e.cursor.Col
	idx := endCol - 1

	if idx >= len(line) {
		idx = len(line) - 1
	}
	if idx < 0 {
		return
	}

	// Skip spaces first
	for idx > 0 && isSpaceRune(line[idx]) {
		idx--
	}

	// Then skip word characters or non-word/non-space characters
	if idx >= 0 && isWordRune(line[idx]) {
		for idx > 0 && isWordRune(line[idx-1]) {
			idx--
		}
	} else if idx >= 0 && !isSpaceRune(line[idx]) {
		for idx > 0 && !isSpaceRune(line[idx-1]) && !isWordRune(line[idx-1]) {
			idx--
		}
	}

	startCol := idx
	if startCol >= endCol {
		return
	}

	// Calculate byte offsets BEFORE making changes
	startByte, startColBytes := e.byteOffset(Cursor{Row: e.cursor.Row, Col: startCol})
	oldEndByte, oldEndColBytes := e.byteOffset(Cursor{Row: e.cursor.Row, Col: endCol})

	// Record text edit for tree-sitter
	e.lastEdit = TextEdit{
		Valid:          true,
		StartByte:      startByte,
		OldEndByte:     oldEndByte,
		NewEndByte:     startByte,
		StartRow:       e.cursor.Row,
		StartColBytes:  startColBytes,
		OldEndRow:      e.cursor.Row,
		OldEndColBytes: oldEndColBytes,
		NewEndRow:      e.cursor.Row,
		NewEndColBytes: startColBytes,
	}

	// Record undo for each character (backwards) as a group
	e.startUndoGroup()
	for col := endCol - 1; col >= startCol; col-- {
		if col >= 0 && col < len(line) {
			e.appendUndo(action{kind: actionInsertRune, pos: Cursor{Row: e.cursor.Row, Col: col}, r: line[col]})
		}
	}
	e.finishUndoGroup()

	// Actually delete the range
	newLine := append([]rune(nil), line[:startCol]...)
	newLine = append(newLine, line[endCol:]...)
	_ = e.text.ReplaceLine(e.cursor.Row, newLine)

	e.cursor.Col = startCol
}
func (e *Editor) deleteWordRight() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}

	line := e.line(e.cursor.Row)
	lineLen := len(line)

	if e.cursor.Col >= lineLen {
		// At end of line - join with next line
		if e.cursor.Row < e.LineCount()-1 {
			// Calculate byte offset BEFORE change
			startByte, startColBytes := e.byteOffset(e.cursor)
			oldEndByte := startByte + 1 // +1 for newline

			e.lastEdit = TextEdit{
				Valid:          true,
				StartByte:      startByte,
				OldEndByte:     oldEndByte,
				NewEndByte:     startByte,
				StartRow:       e.cursor.Row,
				StartColBytes:  startColBytes,
				OldEndRow:      e.cursor.Row + 1,
				OldEndColBytes: 0,
				NewEndRow:      e.cursor.Row,
				NewEndColBytes: startColBytes,
			}

			if e.joinLineAt(e.cursor) {
				e.recordUndo(action{kind: actionSplitLine, pos: e.cursor})
			}
		}
		return
	}

	startCol := e.cursor.Col
	idx := startCol

	// Skip word characters first
	if idx < lineLen && isWordRune(line[idx]) {
		for idx < lineLen && isWordRune(line[idx]) {
			idx++
		}
	} else if idx < lineLen && !isSpaceRune(line[idx]) {
		// Skip non-word/non-space characters
		for idx < lineLen && !isSpaceRune(line[idx]) && !isWordRune(line[idx]) {
			idx++
		}
	}

	// Then skip trailing spaces
	for idx < lineLen && isSpaceRune(line[idx]) {
		idx++
	}

	endCol := idx
	if endCol <= startCol {
		return
	}

	// Calculate byte offsets BEFORE making changes
	startByte, startColBytes := e.byteOffset(Cursor{Row: e.cursor.Row, Col: startCol})
	oldEndByte, oldEndColBytes := e.byteOffset(Cursor{Row: e.cursor.Row, Col: endCol})

	// Record text edit for tree-sitter
	e.lastEdit = TextEdit{
		Valid:          true,
		StartByte:      startByte,
		OldEndByte:     oldEndByte,
		NewEndByte:     startByte,
		StartRow:       e.cursor.Row,
		StartColBytes:  startColBytes,
		OldEndRow:      e.cursor.Row,
		OldEndColBytes: oldEndColBytes,
		NewEndRow:      e.cursor.Row,
		NewEndColBytes: startColBytes,
	}

	// Record undo for each character (backwards) as a group
	e.startUndoGroup()
	for col := endCol - 1; col >= startCol; col-- {
		if col >= 0 && col < len(line) {
			e.appendUndo(action{kind: actionInsertRune, pos: Cursor{Row: e.cursor.Row, Col: col}, r: line[col]})
		}
	}
	e.finishUndoGroup()

	// Actually delete the range
	newLine := append([]rune(nil), line[:startCol]...)
	newLine = append(newLine, line[endCol:]...)
	_ = e.text.ReplaceLine(e.cursor.Row, newLine)
}
func (e *Editor) insertLineBelow() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}

	// Get current line's indentation
	line := e.line(e.cursor.Row)
	indent := make([]rune, 0)
	for _, r := range line {
		if r == ' ' || r == '\t' {
			indent = append(indent, r)
		} else {
			break
		}
	}

	// Move cursor to end of line
	e.cursor.Col = len(line)

	// Split line (creates new line below) and insert indentation as a group
	e.startUndoGroup()
	pos := e.cursor
	if !e.splitLineAt(pos) {
		return
	}
	e.appendUndo(action{kind: actionJoinLine, pos: pos})

	// Insert indentation on the new line
	for _, r := range indent {
		insertPos := e.cursor
		if e.insertRuneAt(insertPos, r) {
			e.appendUndo(action{kind: actionDeleteRune, pos: insertPos, r: r})
		}
	}
	e.finishUndoGroup()
}
func (e *Editor) indentSelection() {
	start, end, ok := e.selectionRange()
	if !ok {
		// No selection - behavior depends on mode
		if e.mode == ModeNormal {
			// In Normal mode, indent the current line (tab at beginning)
			e.indentCurrentLine()
		} else {
			// In Insert mode, insert tab at cursor position
			e.insertTab()
		}
		return
	}

	// Calculate actual end row - if end.Col == 0, don't include that row
	endRow := end.Row
	if end.Col == 0 && end.Row > start.Row {
		endRow = end.Row - 1
	}

	// Indent all lines in selection as a group
	e.startUndoGroup()
	for row := start.Row; row <= endRow; row++ {
		if row < 0 || row >= e.LineCount() {
			continue
		}
		// Insert tab at beginning of line
		line := e.line(row)
		newLine := make([]rune, len(line)+1)
		newLine[0] = '\t'
		copy(newLine[1:], line)
		_ = e.text.ReplaceLine(row, newLine)
		e.appendUndo(action{kind: actionDeleteRune, pos: Cursor{Row: row, Col: 0}, r: '\t'})
	}
	e.finishUndoGroup()
	e.lastEdit.Valid = false

	// Adjust cursor and selection columns - they shift by 1 for affected lines
	if e.cursor.Row >= start.Row && e.cursor.Row <= endRow {
		e.cursor.Col++
	}
	if e.selectionStart.Row >= start.Row && e.selectionStart.Row <= endRow {
		e.selectionStart.Col++
	}
	if e.selectionEnd.Row >= start.Row && e.selectionEnd.Row <= endRow && end.Col > 0 {
		e.selectionEnd.Col++
	}
}

// indentCurrentLine adds a tab at the beginning of the current line (for Normal mode)
func (e *Editor) indentCurrentLine() {
	row := e.cursor.Row
	if row < 0 || row >= e.LineCount() {
		return
	}
	line := e.line(row)
	newLine := make([]rune, len(line)+1)
	newLine[0] = '\t'
	copy(newLine[1:], line)
	_ = e.text.ReplaceLine(row, newLine)
	e.recordUndo(action{kind: actionDeleteRune, pos: Cursor{Row: row, Col: 0}, r: '\t'})
	e.cursor.Col++
	e.lastEdit.Valid = false
}
func (e *Editor) unindentSelection() {
	start, end, hasSelection := e.selectionRange()
	if !hasSelection {
		// No selection - unindent current line only
		start = e.cursor
		end = e.cursor
	}

	// Calculate actual end row - if end.Col == 0, don't include that row
	endRow := end.Row
	if hasSelection && end.Col == 0 && end.Row > start.Row {
		endRow = end.Row - 1
	}

	// Track how many chars removed from each relevant line
	cursorLineRemoved := 0
	startLineRemoved := 0
	endLineRemoved := 0

	// Unindent all lines in selection as a group
	e.startUndoGroup()
	for row := start.Row; row <= endRow; row++ {
		if row < 0 || row >= e.LineCount() {
			continue
		}
		line := e.line(row)
		if len(line) == 0 {
			continue
		}

		removed := 0
		// Remove leading tab or spaces (up to tabWidth)
		if line[0] == '\t' {
			e.appendUndo(action{kind: actionInsertRune, pos: Cursor{Row: row, Col: 0}, r: '\t'})
			_ = e.text.ReplaceLine(row, line[1:])
			removed = 1
		} else if line[0] == ' ' {
			// Count spaces to remove (up to tabWidth)
			for i := 0; i < e.tabWidth && i < len(line) && line[i] == ' '; i++ {
				removed++
			}
			// Record undo for each space (backwards)
			for i := removed - 1; i >= 0; i-- {
				e.appendUndo(action{kind: actionInsertRune, pos: Cursor{Row: row, Col: i}, r: ' '})
			}
			_ = e.text.ReplaceLine(row, line[removed:])
		}

		if row == e.cursor.Row {
			cursorLineRemoved = removed
		}
		if row == e.selectionStart.Row {
			startLineRemoved = removed
		}
		if row == e.selectionEnd.Row {
			endLineRemoved = removed
		}
	}
	e.finishUndoGroup()
	e.lastEdit.Valid = false

	// Adjust cursor column
	if cursorLineRemoved > 0 {
		e.cursor.Col -= cursorLineRemoved
		if e.cursor.Col < 0 {
			e.cursor.Col = 0
		}
	}

	// Adjust selection columns if there was a selection
	if hasSelection {
		if startLineRemoved > 0 {
			e.selectionStart.Col -= startLineRemoved
			if e.selectionStart.Col < 0 {
				e.selectionStart.Col = 0
			}
		}
		// Only adjust selectionEnd.Col if it's on an affected line
		if endLineRemoved > 0 && end.Col > 0 {
			e.selectionEnd.Col -= endLineRemoved
			if e.selectionEnd.Col < 0 {
				e.selectionEnd.Col = 0
			}
		}
	}
}
func (e *Editor) deleteRuneAt(pos Cursor) bool {
	if pos.Row < 0 || pos.Row >= e.LineCount() {
		return false
	}
	lineLen := e.lineLen(pos.Row)
	if pos.Col < 0 || pos.Col >= lineLen {
		return false
	}
	e.recordTextEdit(pos, Cursor{Row: pos.Row, Col: pos.Col + 1}, pos, 0)
	index := e.text.LineStartIndex(pos.Row) + pos.Col
	e.text.DeleteRange(index, index+1)
	e.cursor = Cursor{Row: pos.Row, Col: pos.Col}
	return true
}
func (e *Editor) joinLineAt(pos Cursor) bool {
	if pos.Row < 0 || pos.Row+1 >= e.LineCount() {
		return false
	}
	lineLen := e.lineLen(pos.Row)
	if pos.Col < 0 {
		pos.Col = 0
	}
	if pos.Col > lineLen {
		pos.Col = lineLen
	}
	e.recordTextEdit(pos, Cursor{Row: pos.Row + 1, Col: 0}, pos, 0)
	newlineIndex := e.text.LineEndIndex(pos.Row)
	e.text.DeleteRange(newlineIndex, newlineIndex+1)

	e.cursor = Cursor{Row: pos.Row, Col: pos.Col}
	return true
}

// Helix-style delete (d) - delete selection or char
func (e *Editor) helixDelete() {
	if start, end, ok := e.selectionRange(); ok {
		e.deleteSelection(start, end, true) // Restore selection on undo
		e.clearSelection()
		e.modal.selectMode = false
		return
	}
	// No selection - delete char at cursor
	e.deleteChar()
}

// Helix-style change (c) - delete selection and enter insert mode
func (e *Editor) helixChange() {
	if start, end, ok := e.selectionRange(); ok {
		e.deleteSelection(start, end, true) // Restore selection on undo
		e.clearSelection()
		e.modal.selectMode = false
	}
	e.mode = ModeInsert
	e.saveLineState()
}

// copyToSystemClipboard copies text to system clipboard when available.
func (e *Editor) copyToSystemClipboard() {
	if len(e.clipboard) == 0 {
		return
	}
	if e.runtime.systemClipboard == nil {
		return
	}
	// Join clipboard lines with newlines
	var lines []string
	for _, line := range e.clipboard {
		lines = append(lines, string(line))
	}
	text := strings.Join(lines, "\n")
	_ = e.runtime.systemClipboard.Write(text)
}

// Helix-style yank (y) - copy selection to clipboard
func (e *Editor) yankSelection() {
	start, end, ok := e.selectionRange()
	if !ok {
		// No selection - yank current line
		if e.cursor.Row >= 0 && e.cursor.Row < e.LineCount() {
			e.clipboard = [][]rune{append([]rune(nil), e.line(e.cursor.Row)...)}
		}
		e.copyToSystemClipboard()
		e.modal.lastCommand = "y"
		e.ui.copiedMessageTime = time.Now()
		return
	}

	// Copy selection to clipboard
	e.clipboard = nil
	for row := start.Row; row <= end.Row; row++ {
		if row < 0 || row >= e.LineCount() {
			continue
		}
		line := e.line(row)
		startCol := 0
		endCol := len(line)
		if row == start.Row {
			startCol = start.Col
		}
		if row == end.Row {
			endCol = end.Col
		}
		if startCol < 0 {
			startCol = 0
		}
		if endCol > len(line) {
			endCol = len(line)
		}
		e.clipboard = append(e.clipboard, append([]rune(nil), line[startCol:endCol]...))
	}
	e.copyToSystemClipboard()
	e.modal.lastCommand = "y"
	e.ui.copiedMessageTime = time.Now()
	e.clearSelection()
	e.modal.selectMode = false
}

// Helix-style paste (p) - paste after cursor
func (e *Editor) pasteAfter() {
	if len(e.clipboard) == 0 {
		return
	}

	e.startUndoGroup()
	defer e.finishUndoGroup()

	if len(e.clipboard) == 1 {
		// Single line - paste inline after cursor
		line := e.clipboard[0]
		pos := Cursor{Row: e.cursor.Row, Col: e.cursor.Col + 1}
		if pos.Col > e.lineLen(e.cursor.Row) {
			pos.Col = e.lineLen(e.cursor.Row)
		}
		for _, r := range line {
			if e.insertRuneAt(pos, r) {
				e.appendUndo(action{kind: actionDeleteRune, pos: pos, r: r})
				pos.Col++
			}
		}
		e.cursor.Col = pos.Col - 1
		if e.cursor.Col < 0 {
			e.cursor.Col = 0
		}
	} else {
		// Multi-line - paste lines below
		insert := append([]rune{'\n'}, joinText(e.clipboard)...)
		index := e.text.LineStartIndex(e.cursor.Row) + e.lineLen(e.cursor.Row)
		e.text.Insert(index, insert)
		e.cursor.Row++
		e.cursor.Col = 0
		e.lastEdit.Valid = false
	}
}

// Helix-style paste before (P) - paste before cursor
func (e *Editor) pasteBefore() {
	if len(e.clipboard) == 0 {
		return
	}

	e.startUndoGroup()
	defer e.finishUndoGroup()

	if len(e.clipboard) == 1 {
		// Single line - paste inline at cursor
		line := e.clipboard[0]
		pos := e.cursor
		for _, r := range line {
			if e.insertRuneAt(pos, r) {
				e.appendUndo(action{kind: actionDeleteRune, pos: pos, r: r})
				pos.Col++
			}
		}
	} else {
		// Multi-line - paste lines above
		insert := append(joinText(e.clipboard), '\n')
		index := e.text.LineStartIndex(e.cursor.Row)
		e.text.Insert(index, insert)
		e.cursor.Col = 0
		e.lastEdit.Valid = false
	}
}

// Helix-style open below (o) - open line below and enter insert
func (e *Editor) openBelow() {
	e.insertLineBelow()
	e.mode = ModeInsert
	e.saveLineState()
}

// Helix-style open above (O) - open line above and enter insert
func (e *Editor) openAbove() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}

	// Get current line's indentation
	line := e.line(e.cursor.Row)
	indent := make([]rune, 0)
	for _, r := range line {
		if r == ' ' || r == '\t' {
			indent = append(indent, r)
		} else {
			break
		}
	}

	// Insert new line above
	insert := append([]rune(nil), indent...)
	insert = append(insert, '\n')
	index := e.text.LineStartIndex(e.cursor.Row)
	e.text.Insert(index, insert)

	e.cursor.Col = len(indent)
	e.mode = ModeInsert
	e.saveLineState()
	e.lastEdit.Valid = false
}

// insertLineAboveCursor inserts an empty line at cursor position,
// pushing current line down. The new line is indented with tabs/spaces
// up to the cursor's visual column. Cursor stays at same position.
func (e *Editor) insertLineAboveCursor() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}

	line := e.line(e.cursor.Row)
	tabWidth := e.tabWidth
	if tabWidth < 1 {
		tabWidth = 4
	}

	// Calculate cursor's visual column
	visualX := visualCol(line, e.cursor.Col, tabWidth)

	// Build indentation: fill with tabs, then spaces for remainder
	indent := make([]rune, 0)
	col := 0
	for col+tabWidth <= visualX {
		indent = append(indent, '\t')
		col += tabWidth
	}
	// Add spaces for remaining 1-3 positions
	for col < visualX {
		indent = append(indent, ' ')
		col++
	}

	// Insert new line at cursor position, push current line down
	insert := append([]rune(nil), indent...)
	insert = append(insert, '\n')
	index := e.text.LineStartIndex(e.cursor.Row)
	e.text.Insert(index, insert)

	// Record undo: to undo this, we need to join the line we created
	// The position is at the end of the new line (which has the indent)
	e.recordUndo(action{kind: actionJoinLine, pos: Cursor{Row: e.cursor.Row, Col: len(indent)}})

	// Cursor stays at same row (now on the new indented line)
	e.cursor.Col = len(indent)
	e.lastEdit.Valid = false
}

// Helix-style append (a) - move right and enter insert
func (e *Editor) appendMode() {
	e.moveRight()
	e.mode = ModeInsert
	e.saveLineState()
}

// Helix-style append line end (A) - go to line end and enter insert
func (e *Editor) appendLineEnd() {
	e.moveLineEnd()
	e.mode = ModeInsert
	e.saveLineState()
}

// Helix-style insert line start (I) - go to first non-whitespace and insert
func (e *Editor) insertLineStart() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	line := e.line(e.cursor.Row)
	// Find first non-whitespace
	col := 0
	for col < len(line) && (line[col] == ' ' || line[col] == '\t') {
		col++
	}
	e.cursor.Col = col
	e.mode = ModeInsert
	e.saveLineState()
}

// Helix-style replace char (r) - replace char at cursor
func (e *Editor) replaceCharAtCursor(ch rune) bool {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return false
	}
	line := e.line(e.cursor.Row)
	if e.cursor.Col < 0 || e.cursor.Col >= len(line) {
		return false
	}

	oldChar := line[e.cursor.Col]
	e.startUndoGroup()
	// Delete old char
	if e.deleteRuneAt(e.cursor) {
		e.appendUndo(action{kind: actionInsertRune, pos: e.cursor, r: oldChar})
	}
	// Insert new char
	if e.insertRuneAt(e.cursor, ch) {
		e.appendUndo(action{kind: actionDeleteRune, pos: e.cursor, r: ch})
	}
	e.finishUndoGroup()
	return true
}

// Helix-style join lines (J) - join current line with next
func (e *Editor) joinLinesCmd() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount()-1 {
		return
	}

	// Position at end of current line
	pos := Cursor{Row: e.cursor.Row, Col: e.lineLen(e.cursor.Row)}

	// Add a space before joining (unless line ends with space or next line starts with space)
	currentLine := e.line(e.cursor.Row)
	nextLine := e.line(e.cursor.Row + 1)
	needSpace := len(currentLine) > 0 && len(nextLine) > 0 &&
		!isSpaceRune(currentLine[len(currentLine)-1]) &&
		!isSpaceRune(nextLine[0])

	if needSpace {
		if e.insertRuneAt(pos, ' ') {
			e.recordUndo(action{kind: actionDeleteRune, pos: pos, r: ' '})
			pos.Col++
		}
	}

	// Join lines
	if e.joinLineAt(pos) {
		e.recordUndo(action{kind: actionSplitLine, pos: pos})
	}

	e.cursor = pos
}

// Helix-style toggle select (v) - toggle selection mode
func (e *Editor) toggleSelectMode() {
	e.modal.selectMode = !e.modal.selectMode
	if e.modal.selectMode {
		// Start selection at cursor
		e.selectionStart = e.cursor
		e.selectionEnd = e.cursor
		e.selectionActive = true
	} else {
		e.clearSelection()
	}
}

// Helix-style extend line (x) - select current line with cursor at end
// If already selecting and cursor at line end, extend selection to next line
func (e *Editor) extendLine() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}

	lineLen := e.lineLen(e.cursor.Row)

	// Check if we should extend to next line:
	// - selection is active
	// - cursor is at the end of current line
	// - selection end matches cursor position
	if e.selectionActive && e.cursor.Col == lineLen &&
		e.selectionEnd.Row == e.cursor.Row && e.selectionEnd.Col == lineLen {
		// Extend to next line if available
		if e.cursor.Row < e.LineCount()-1 {
			e.cursor.Row++
			newLineLen := e.lineLen(e.cursor.Row)
			e.cursor.Col = newLineLen
			e.selectionEnd = Cursor{Row: e.cursor.Row, Col: newLineLen}
		}
		return
	}

	// First press: select entire current line with cursor at end
	e.selectionStart = Cursor{Row: e.cursor.Row, Col: 0}
	e.selectionEnd = Cursor{Row: e.cursor.Row, Col: lineLen}
	e.cursor.Col = lineLen
	e.selectionActive = true
	e.modal.selectMode = true
}

// Helix-style collapse selection (;) - collapse selection to cursor
func (e *Editor) collapseSelection() {
	e.clearSelection()
	e.modal.selectMode = false
}

// Helix-style flip selection (Alt+;) - swap anchor and cursor
func (e *Editor) flipSelection() {
	if !e.selectionActive {
		return
	}
	e.selectionStart, e.selectionEnd = e.selectionEnd, e.selectionStart
	e.cursor = e.selectionEnd
}
func (e *Editor) clearSelection() {
	e.selectionActive = false
	e.selectionStart = Cursor{}
	e.selectionEnd = Cursor{}
}
func (e *Editor) selectAll() {
	if e.LineCount() == 0 {
		return
	}
	e.selectionStart = Cursor{Row: 0, Col: 0}
	lastRow := e.LineCount() - 1
	e.selectionEnd = Cursor{Row: lastRow, Col: e.lineLen(lastRow)}
	e.selectionActive = true
}

// expandSelection expands selection to the next larger syntax node
func (e *Editor) expandSelection() {
	if e.runtime.nodeStackFunc == nil || e.filename == "" {
		e.setStatus("syntax tree not available")
		return
	}

	// Get node stack at cursor position
	stack := e.runtime.nodeStackFunc(e.filename, e.cursor.Row, e.cursor.Col)
	if len(stack) == 0 {
		e.setStatus("no syntax node at cursor")
		return
	}

	// If no selection or selection changed, rebuild scope stack
	if !e.selectionActive || len(e.selectionScopeStack) == 0 {
		e.selectionScopeStack = stack
		e.selectionScopeIndex = 0
	}

	// Find next larger scope
	if e.selectionScopeIndex < len(e.selectionScopeStack) {
		nr := e.selectionScopeStack[e.selectionScopeIndex]
		e.selectionStart = Cursor{Row: nr.StartRow, Col: nr.StartCol}
		e.selectionEnd = Cursor{Row: nr.EndRow, Col: nr.EndCol}
		e.selectionActive = true
		e.modal.selectMode = true
		e.selectionScopeIndex++
	}
}

// shrinkSelection shrinks selection to the next smaller syntax node
func (e *Editor) shrinkSelection() {
	if !e.selectionActive || len(e.selectionScopeStack) == 0 {
		return
	}

	// Go back to previous scope
	if e.selectionScopeIndex > 1 {
		e.selectionScopeIndex--
		nr := e.selectionScopeStack[e.selectionScopeIndex-1]
		e.selectionStart = Cursor{Row: nr.StartRow, Col: nr.StartCol}
		e.selectionEnd = Cursor{Row: nr.EndRow, Col: nr.EndCol}
	} else {
		// Can't shrink further, clear selection
		e.clearSelection()
		e.modal.selectMode = false
		e.selectionScopeStack = nil
		e.selectionScopeIndex = 0
	}
}
func (e *Editor) extendSelection(move func()) {
	before := e.cursor
	if !e.selectionActive {
		e.selectionStart = before
	}
	move()
	if before == e.cursor && !e.selectionActive {
		return
	}
	e.selectionActive = true
	e.selectionEnd = e.cursor
}
func (e *Editor) selectionRange() (Cursor, Cursor, bool) {
	if !e.selectionActive {
		return Cursor{}, Cursor{}, false
	}
	if e.selectionStart == e.selectionEnd {
		return Cursor{}, Cursor{}, false
	}
	start := e.selectionStart
	end := e.selectionEnd
	if cursorLess(end, start) {
		start, end = end, start
	}
	return start, end, true
}
func cursorLess(a, b Cursor) bool {
	if a.Row != b.Row {
		return a.Row < b.Row
	}
	return a.Col < b.Col
}
func (e *Editor) selectionRangeForLine(lineIdx int) (int, int, bool) {
	start, end, ok := e.selectionRange()
	if !ok {
		return 0, 0, false
	}
	if lineIdx < start.Row || lineIdx > end.Row {
		return 0, 0, false
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
		endCol = lineLen
	} else if lineIdx == end.Row {
		startCol = 0
		endCol = clampRange(end.Col, 0, lineLen)
	}
	if endCol <= startCol {
		return 0, 0, false
	}
	return startCol, endCol, true
}

// toggleLineComment toggles comment on current line or selection
func (e *Editor) toggleLineComment() {
	// Detect comment prefix based on file extension
	ext := filepath.Ext(e.filename)
	var prefix, suffix string
	switch ext {
	case ".go", ".c", ".cpp", ".h", ".java", ".js", ".ts", ".rs", ".swift":
		prefix = "//"
	case ".py", ".sh", ".bash", ".zsh", ".yaml", ".yml", ".toml", ".rb":
		prefix = "#"
	case ".lua", ".sql":
		prefix = "--"
	case ".vim":
		prefix = "\""
	case ".html", ".xml":
		prefix = "<!--"
		suffix = " -->"
	default:
		prefix = "//"
	}

	start, end := e.cursor.Row, e.cursor.Row
	if s, en, ok := e.selectionRange(); ok {
		start, end = s.Row, en.Row
		// If selection ends at column 0, don't include that line
		// (common when selecting full lines - cursor ends up at start of next line)
		if en.Col == 0 && en.Row > s.Row {
			end = en.Row - 1
		}
	}

	// Validate range
	if start < 0 {
		start = 0
	}
	lineCount := e.LineCount()
	if end >= lineCount {
		end = lineCount - 1
	}
	if start > end {
		return // nothing to do
	}

	// Find minimum indentation (only count non-empty lines)
	minIndent := -1
	for row := start; row <= end; row++ {
		line := e.line(row)
		if len(line) == 0 {
			continue // skip empty lines for indent calculation
		}
		indent := 0
		for _, r := range line {
			if r == ' ' || r == '\t' {
				indent++
			} else {
				break
			}
		}
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent < 0 {
		minIndent = 0
	}

	// Check if all non-empty lines are already commented at minIndent position
	allCommented := true
	for row := start; row <= end; row++ {
		line := string(e.line(row))
		if len(line) == 0 {
			continue // skip empty lines
		}
		// Check if comment prefix exists exactly at minIndent (after any leading whitespace)
		if minIndent > len(line) {
			allCommented = false
			break
		}
		rest := line[minIndent:]
		trimmed := strings.TrimLeft(rest, " \t")
		if !strings.HasPrefix(trimmed, prefix) {
			allCommented = false
			break
		}
	}

	e.startUndoGroup()

	for row := start; row <= end; row++ {
		line := e.line(row)
		lineStr := string(line)

		// Skip empty lines
		if len(lineStr) == 0 {
			continue
		}

		if allCommented {
			// Remove comment - find the prefix only after minIndent position
			searchStart := minIndent
			if searchStart > len(lineStr) {
				searchStart = len(lineStr)
			}
			rest := lineStr[searchStart:]
			// Find prefix in the rest of the line (should be at start after whitespace)
			trimmedRest := strings.TrimLeft(rest, " \t")
			prefixOffset := len(rest) - len(trimmedRest)
			if strings.HasPrefix(trimmedRest, prefix) {
				idx := searchStart + prefixOffset
				// Remove prefix and one space if present
				removeLen := len(prefix)
				if idx+removeLen < len(lineStr) && lineStr[idx+removeLen] == ' ' {
					removeLen++
				}
				newLine := lineStr[:idx] + lineStr[idx+removeLen:]
				// Also remove suffix if present (for HTML/XML)
				if suffix != "" && strings.HasSuffix(newLine, suffix) {
					newLine = newLine[:len(newLine)-len(suffix)]
				}
				_ = e.text.ReplaceLine(row, []rune(newLine))
			}
		} else {
			// Add comment at minIndent position
			insertAt := minIndent
			if insertAt > len(lineStr) {
				insertAt = len(lineStr)
			}
			newLine := lineStr[:insertAt] + prefix + " " + lineStr[insertAt:] + suffix
			_ = e.text.ReplaceLine(row, []rune(newLine))
		}
	}

	e.finishUndoGroup()
	e.dirty = true
	e.lastEdit.Valid = false
}
func (e *Editor) replaceBuffer(text string, markDirty bool) {
	e.text = NewTextBufferFromString(text)
	lineCount := e.LineCount()
	if e.cursor.Row >= lineCount {
		e.cursor.Row = lineCount - 1
		if e.cursor.Row < 0 {
			e.cursor.Row = 0
		}
	}
	e.clampCursorCol()
	if e.scroll >= lineCount {
		e.scroll = lineCount - 1
		if e.scroll < 0 {
			e.scroll = 0
		}
	}
	e.undo = nil
	e.redo = nil
	if markDirty {
		e.savePoint = -1
	} else {
		e.savePoint = 0
	}
	e.lastEdit.Valid = false
	e.changeTick++
	e.updateDirty()
	e.resetConflictBlocks()
}
