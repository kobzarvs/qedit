package editor

import "path/filepath"

func (e *Editor) restoreSessionState() {
	if e.sessionStore == nil || e.filename == "" {
		return
	}
	absPath, err := filepath.Abs(e.filename)
	if err != nil {
		return
	}
	state, ok := e.sessionStore.GetFileState(absPath)
	if !ok {
		return
	}

	// Restore cursor (clamped to valid range)
	e.cursor.Row = state.CursorRow
	lineCount := e.LineCount()
	if e.cursor.Row >= lineCount {
		e.cursor.Row = lineCount - 1
	}
	if e.cursor.Row < 0 {
		e.cursor.Row = 0
	}
	e.cursor.Col = state.CursorCol
	if e.cursor.Row < lineCount && e.cursor.Col > e.text.LineLen(e.cursor.Row) {
		e.cursor.Col = e.text.LineLen(e.cursor.Row)
	}
	if e.cursor.Col < 0 {
		e.cursor.Col = 0
	}

	// Restore scroll
	e.scroll = state.ScrollY
	if e.scroll < 0 {
		e.scroll = 0
	}
	e.scrollX = state.ScrollX
	if e.scrollX < 0 {
		e.scrollX = 0
	}

	// Restore mode
	switch state.Mode {
	case "insert":
		e.mode = ModeInsert
	default:
		e.mode = ModeNormal
	}

	// Restore selection with bounds validation
	if state.SelectionActive {
		// Validate selection is within file bounds
		if state.SelectionStartRow >= lineCount || state.SelectionEndRow >= lineCount {
			// File was shortened - reset selection
			e.selectionActive = false
		} else {
			e.selectionActive = true
			// Clamp columns to line lengths
			startCol := state.SelectionStartCol
			if startCol > e.text.LineLen(state.SelectionStartRow) {
				startCol = e.text.LineLen(state.SelectionStartRow)
			}
			endCol := state.SelectionEndCol
			if endCol > e.text.LineLen(state.SelectionEndRow) {
				endCol = e.text.LineLen(state.SelectionEndRow)
			}
			e.selectionStart = Cursor{Row: state.SelectionStartRow, Col: startCol}
			e.selectionEnd = Cursor{Row: state.SelectionEndRow, Col: endCol}
		}
	}
}

func (e *Editor) saveSessionState() {
	if e.sessionStore == nil || e.filename == "" {
		return
	}
	absPath, err := filepath.Abs(e.filename)
	if err != nil {
		return
	}

	mode := "normal"
	if e.mode == ModeInsert {
		mode = "insert"
	}

	state := FileState{
		CursorRow:         e.cursor.Row,
		CursorCol:         e.cursor.Col,
		ScrollY:           e.scroll,
		ScrollX:           e.scrollX,
		Mode:              mode,
		SelectionActive:   e.selectionActive,
		SelectionStartRow: e.selectionStart.Row,
		SelectionStartCol: e.selectionStart.Col,
		SelectionEndRow:   e.selectionEnd.Row,
		SelectionEndCol:   e.selectionEnd.Col,
	}
	e.sessionStore.SetFileState(absPath, state)
}

// Shutdown saves session state and stops background tasks
func (e *Editor) Shutdown() {
	// Save all buffer session states
	if e.buffers != nil && e.buffers.Count() > 0 {
		// Update the active buffer first
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
		// Save session state for all buffers
		for _, info := range e.buffers.Items() {
			if info.Filename != "" && e.sessionStore != nil {
				// Temporarily set filename to save each buffer's session
				// (restoreSessionState/saveSessionState use e.filename)
				origFilename := e.filename
				bufState := e.buffers.buffers[info.Index]
				e.filename = bufState.filename
				e.cursor = bufState.cursor
				e.scroll = bufState.scroll
				e.scrollX = bufState.scrollX
				e.mode = bufState.mode
				e.selectionActive = bufState.selectionActive
				e.selectionStart = bufState.selectionStart
				e.selectionEnd = bufState.selectionEnd
				e.saveSessionState()
				e.filename = origFilename
			}
		}
	} else {
		e.saveSessionState()
	}
	if e.sessionStore != nil {
		e.sessionStore.Stop()
	}
}
