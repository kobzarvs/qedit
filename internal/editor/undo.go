package editor

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func (e *Editor) Undo() {
	if len(e.undo) == 0 {
		e.setStatus("nothing to undo")
		return
	}

	// Get the group of the last action
	group := e.undo[len(e.undo)-1].group

	linesBefore := e.LineCount()
	minRow := linesBefore
	// Undo all actions in this group
	for len(e.undo) > 0 && e.undo[len(e.undo)-1].group == group {
		idx := len(e.undo) - 1
		act := e.undo[idx]
		if act.pos.Row < minRow {
			minRow = act.pos.Row
		}
		e.undo = e.undo[:idx]
		inv, ok := e.applyAction(act)
		if !ok {
			e.setStatus("undo failed")
			return
		}
		inv.group = act.group
		e.redo = append(e.redo, inv)
	}
	e.changeTick++
	e.updateDirty()
	e.setUndoRedoEdit(minRow, linesBefore)
}
func (e *Editor) Redo() {
	if len(e.redo) == 0 {
		e.setStatus("nothing to redo")
		return
	}

	// Get the group of the last action
	group := e.redo[len(e.redo)-1].group

	linesBefore := e.LineCount()
	minRow := linesBefore
	// Redo all actions in this group
	for len(e.redo) > 0 && e.redo[len(e.redo)-1].group == group {
		idx := len(e.redo) - 1
		act := e.redo[idx]
		if act.pos.Row < minRow {
			minRow = act.pos.Row
		}
		e.redo = e.redo[:idx]
		inv, ok := e.applyAction(act)
		if !ok {
			e.setStatus("redo failed")
			return
		}
		inv.group = act.group
		e.undo = append(e.undo, inv)
	}
	e.changeTick++
	e.updateDirty()
	e.setUndoRedoEdit(minRow, linesBefore)
}

// setUndoRedoEdit computes a synthetic TextEdit from undo/redo line changes
// so that AdjustHighlights can shift the cached highlight map.
func (e *Editor) setUndoRedoEdit(minRow, linesBefore int) {
	linesAfter := e.LineCount()
	delta := linesAfter - linesBefore
	oldEnd := minRow
	newEnd := minRow
	if delta > 0 {
		newEnd = minRow + delta
	} else if delta < 0 {
		oldEnd = minRow - delta
	}
	e.lastEdit = TextEdit{
		Valid:     true,
		StartRow:  minRow,
		OldEndRow: oldEnd,
		NewEndRow: newEnd,
	}
}

func (e *Editor) applyAction(act action) (action, bool) {
	switch act.kind {
	case actionInsertRune:
		if !e.insertRuneAt(act.pos, act.r) {
			return action{}, false
		}
		return action{kind: actionDeleteRune, pos: act.pos, r: act.r}, true
	case actionDeleteRune:
		if !e.deleteRuneAt(act.pos) {
			return action{}, false
		}
		return action{kind: actionInsertRune, pos: act.pos, r: act.r}, true
	case actionSplitLine:
		if !e.splitLineAt(act.pos) {
			return action{}, false
		}
		return action{kind: actionJoinLine, pos: act.pos}, true
	case actionJoinLine:
		if !e.joinLineAt(act.pos) {
			return action{}, false
		}
		return action{kind: actionSplitLine, pos: act.pos}, true
	case actionMoveLine:
		if !e.swapLines(act.rowFrom, act.rowTo) {
			return action{}, false
		}
		e.cursor.Row = act.rowTo
		if e.cursor.Row < 0 {
			e.cursor.Row = 0
		}
		if e.cursor.Row >= e.LineCount() {
			e.cursor.Row = e.LineCount() - 1
		}
		e.clampCursorCol()
		return action{kind: actionMoveLine, rowFrom: act.rowTo, rowTo: act.rowFrom}, true
	case actionInsertText:
		endPos := e.insertTextAt(act.pos, act.text)
		// Restore selection if this action has one
		if act.hasSelection {
			e.selectionActive = true
			e.selectionStart = act.selectionStart
			e.selectionEnd = act.selectionEnd
		}
		// Preserve selection info for redo→undo cycle
		return action{
			kind:           actionDeleteText,
			pos:            act.pos,
			endPos:         endPos,
			text:           act.text,
			selectionStart: act.selectionStart,
			selectionEnd:   act.selectionEnd,
			hasSelection:   act.hasSelection,
		}, true
	case actionDeleteText:
		deleted := e.deleteTextRange(act.pos, act.endPos)
		// Clear selection - after delete there's no selection
		e.selectionActive = false
		// Preserve selection info for undo cycle
		return action{
			kind:           actionInsertText,
			pos:            act.pos,
			text:           deleted,
			selectionStart: act.selectionStart,
			selectionEnd:   act.selectionEnd,
			hasSelection:   act.hasSelection,
		}, true
	default:
		return action{}, false
	}
}
func (e *Editor) recordUndo(act action) {
	e.undoGroup++
	act.group = e.undoGroup
	e.undo = append(e.undo, act)
	e.redo = e.redo[:0]
	e.changeTick++
	e.updateDirty()
}

// startUndoGroup starts a new undo group. All subsequent appendUndo calls will use this group.
// Call this before a series of appendUndo calls, then call finishUndoGroup at the end.
func (e *Editor) startUndoGroup() {
	e.undoGroup++
}

// appendUndo adds an action to undo stack with the current group.
// Use this when recording multiple actions as part of a single logical operation.
func (e *Editor) appendUndo(act action) {
	act.group = e.undoGroup
	e.undo = append(e.undo, act)
}

// finishUndoGroup clears redo and updates state after a group of undo actions.
func (e *Editor) finishUndoGroup() {
	e.redo = e.redo[:0]
	e.changeTick++
	e.updateDirty()
}
func (e *Editor) updateDirty() {
	e.dirty = len(e.undo) != e.savePoint || e.file.externalChange != ExternalChangeNone
}

// changelogFilePath returns the path for the changelog file for the given file path.
// Format: $XDG_STATE_HOME/qedit/undo/<encoded-path>.log
func changelogFilePath(filePath string) string {
	// XDG state directory (same as session)
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(stateDir, "qedit", "undo")

	// Get absolute path and encode it
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	// Replace path separators with underscores and other special chars
	encoded := strings.ReplaceAll(absPath, string(filepath.Separator), "_")
	encoded = strings.ReplaceAll(encoded, ":", "_")
	encoded = strings.ReplaceAll(encoded, " ", "_")

	return filepath.Join(dir, encoded+".log")
}

// actionToJSON converts an action to its JSON-serializable form
func actionToJSON(a action) actionJSON {
	var textStrings []string
	if len(a.text) > 0 {
		textStrings = make([]string, len(a.text))
		for i, line := range a.text {
			textStrings[i] = string(line)
		}
	}
	return actionJSON{
		Kind:           int(a.kind),
		PosRow:         a.pos.Row,
		PosCol:         a.pos.Col,
		R:              a.r,
		RowFrom:        a.rowFrom,
		RowTo:          a.rowTo,
		Group:          a.group,
		Text:           textStrings,
		EndPosRow:      a.endPos.Row,
		EndPosCol:      a.endPos.Col,
		SelectionStart: [2]int{a.selectionStart.Row, a.selectionStart.Col},
		SelectionEnd:   [2]int{a.selectionEnd.Row, a.selectionEnd.Col},
		HasSelection:   a.hasSelection,
	}
}

// jsonToAction converts a JSON-serializable action back to an action
func jsonToAction(j actionJSON) action {
	var text [][]rune
	if len(j.Text) > 0 {
		text = make([][]rune, len(j.Text))
		for i, s := range j.Text {
			text[i] = []rune(s)
		}
	}
	return action{
		kind:           actionKind(j.Kind),
		pos:            Cursor{Row: j.PosRow, Col: j.PosCol},
		r:              j.R,
		rowFrom:        j.RowFrom,
		rowTo:          j.RowTo,
		group:          j.Group,
		text:           text,
		endPos:         Cursor{Row: j.EndPosRow, Col: j.EndPosCol},
		selectionStart: Cursor{Row: j.SelectionStart[0], Col: j.SelectionStart[1]},
		selectionEnd:   Cursor{Row: j.SelectionEnd[0], Col: j.SelectionEnd[1]},
		hasSelection:   j.HasSelection,
	}
}

// undoHistoryHeader stores metadata for undo history validation
type undoHistoryHeader struct {
	Version int   `json:"v"`
	Mtime   int64 `json:"mtime"`
}

// SaveUndoHistory saves the undo history to the changelog file
func (e *Editor) SaveUndoHistory() error {
	if e.filename == "" {
		return nil // No file path, nothing to save
	}

	logPath := changelogFilePath(e.filename)
	if logPath == "" {
		return nil
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Get file mtime for validation
	var mtime int64
	if info, err := os.Stat(e.filename); err == nil {
		mtime = info.ModTime().UnixNano()
	}

	// Open file for writing
	f, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	encoder := json.NewEncoder(writer)

	// Write header with mtime for validation
	header := undoHistoryHeader{Version: 1, Mtime: mtime}
	if err := encoder.Encode(header); err != nil {
		return err
	}

	// Write each action as a JSON line
	for _, a := range e.undo {
		if err := encoder.Encode(actionToJSON(a)); err != nil {
			return err
		}
	}

	return writer.Flush()
}

// LoadUndoHistory loads the undo history from the changelog file
func (e *Editor) LoadUndoHistory() error {
	if e.filename == "" {
		return nil
	}

	logPath := changelogFilePath(e.filename)
	if logPath == "" {
		return nil
	}

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No history file, that's ok
		}
		return err
	}
	defer f.Close()

	// Get current file mtime for validation
	var currentMtime int64
	if info, err := os.Stat(e.filename); err == nil {
		currentMtime = info.ModTime().UnixNano()
	}

	e.undo = nil
	scanner := bufio.NewScanner(f)
	// Increase buffer size for large actions
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	// Read and validate header first
	if scanner.Scan() {
		var header undoHistoryHeader
		if err := json.Unmarshal(scanner.Bytes(), &header); err == nil {
			if header.Version > 0 {
				// New format with header - validate mtime
				if header.Mtime != currentMtime {
					// File was modified externally, discard history
					return nil
				}
			} else {
				// Old format without header - treat as action
				var j actionJSON
				if err := json.Unmarshal(scanner.Bytes(), &j); err == nil {
					e.undo = append(e.undo, jsonToAction(j))
				}
			}
		}
	}

	for scanner.Scan() {
		var j actionJSON
		if err := json.Unmarshal(scanner.Bytes(), &j); err != nil {
			continue // Skip malformed lines
		}
		e.undo = append(e.undo, jsonToAction(j))
	}

	return scanner.Err()
}

// ClearUndoHistory removes the changelog file for the current file
func (e *Editor) ClearUndoHistory() error {
	if e.filename == "" {
		return nil
	}

	logPath := changelogFilePath(e.filename)
	if logPath == "" {
		return nil
	}

	err := os.Remove(logPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func (e *Editor) saveLineState() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		e.lineUndoValid = false
		return
	}
	if e.lineUndoValid && e.lineUndoRow == e.cursor.Row {
		return // Already tracking this line
	}
	e.lineUndoRow = e.cursor.Row
	e.lineUndoContent = append([]rune(nil), e.line(e.cursor.Row)...)
	e.lineUndoValid = true
}
func (e *Editor) undoLine() {
	if !e.lineUndoValid {
		e.setStatus("no line changes to undo")
		return
	}
	row := e.lineUndoRow
	if row < 0 || row >= e.LineCount() {
		e.setStatus("line no longer exists")
		e.lineUndoValid = false
		return
	}

	currentLine := e.line(row)
	originalLine := e.lineUndoContent

	// If line hasn't changed, nothing to do
	if string(currentLine) == string(originalLine) {
		e.setStatus("no changes on this line")
		return
	}

	e.startUndoGroup()

	// Delete current line content (backwards for proper undo)
	for i := len(currentLine) - 1; i >= 0; i-- {
		pos := Cursor{Row: row, Col: i}
		r := currentLine[i]
		if e.deleteRuneAt(pos) {
			e.appendUndo(action{kind: actionInsertRune, pos: pos, r: r})
		}
	}

	// Insert original content
	for i, r := range originalLine {
		pos := Cursor{Row: row, Col: i}
		if e.insertRuneAt(pos, r) {
			e.appendUndo(action{kind: actionDeleteRune, pos: pos, r: r})
		}
	}

	e.finishUndoGroup()

	// Position cursor at start of line
	e.cursor.Row = row
	e.cursor.Col = 0
	if e.cursor.Col > e.lineLen(row) {
		e.cursor.Col = e.lineLen(row)
	}

	// Invalidate line undo since we've restored it
	e.lineUndoValid = false
	e.setStatus("line restored")
}
