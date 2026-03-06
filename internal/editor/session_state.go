package editor

func (e *Editor) restoreSessionState() {
	if e.runtime.persistence == nil || !e.runtime.persistence.HasSessionState() || e.document.filename == "" {
		return
	}
	path := e.normalizedPath(e.document.filename)
	if path == "" {
		return
	}
	state, ok := e.runtime.persistence.GetFileState(path)
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
	e.viewport.scroll = state.ScrollY
	if e.viewport.scroll < 0 {
		e.viewport.scroll = 0
	}
	e.viewport.scrollX = state.ScrollX
	if e.viewport.scrollX < 0 {
		e.viewport.scrollX = 0
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
	if e.runtime.persistence == nil || !e.runtime.persistence.HasSessionState() || e.document.filename == "" {
		return
	}
	path := e.normalizedPath(e.document.filename)
	if path == "" {
		return
	}

	mode := "normal"
	if e.mode == ModeInsert {
		mode = "insert"
	}

	state := FileState{
		CursorRow:         e.cursor.Row,
		CursorCol:         e.cursor.Col,
		ScrollY:           e.viewport.scroll,
		ScrollX:           e.viewport.scrollX,
		Mode:              mode,
		SelectionActive:   e.selectionActive,
		SelectionStartRow: e.selectionStart.Row,
		SelectionStartCol: e.selectionStart.Col,
		SelectionEndRow:   e.selectionEnd.Row,
		SelectionEndCol:   e.selectionEnd.Col,
	}
	e.runtime.persistence.SetFileState(path, state)
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
			if info.Filename != "" && e.runtime.persistence != nil && e.runtime.persistence.HasSessionState() {
				// Temporarily set filename to save each buffer's session
				// (restoreSessionState/saveSessionState use current document filename)
				origFilename := e.document.filename
				bufState := e.buffers.buffers[info.Index]
				e.document.filename = bufState.filename
				e.cursor = bufState.cursor
				e.viewport.scroll = bufState.scroll
				e.viewport.scrollX = bufState.scrollX
				e.mode = bufState.mode
				e.selectionActive = bufState.selectionActive
				e.selectionStart = bufState.selectionStart
				e.selectionEnd = bufState.selectionEnd
				e.saveSessionState()
				e.document.filename = origFilename
			}
		}
	} else {
		e.saveSessionState()
	}
	if e.runtime.persistence != nil && e.runtime.persistence.HasSessionState() {
		e.runtime.persistence.Stop()
	}
}
