package editor

import (
	"os"
	"path/filepath"
)

func (e *Editor) OpenFile(path string) error {
	absPath := path
	if abs, err := filepath.Abs(path); err == nil {
		absPath = abs
	}

	// Check if this file is already open in a buffer
	if e.buffers != nil && e.buffers.Count() > 0 {
		if idx := e.buffers.FindByPath(absPath); idx >= 0 {
			e.switchToBuffer(idx)
			return nil
		}
	}

	// Save current buffer state before opening new file
	if e.buffers != nil && e.buffers.Count() > 0 && e.filename != "" {
		e.saveSessionState()
		_ = e.SaveUndoHistory()
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
	}

	// Load the new file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	e.text = NewTextBufferFromBytes(data)
	e.cursor = Cursor{}
	e.file.diskContent = e.Content()
	e.resetConflictBlocks()
	e.scroll = 0
	e.scrollX = 0
	e.mode = ModeNormal
	e.filename = absPath
	e.cmd = e.cmd[:0]
	e.statusMessage = ""
	e.undo = nil
	e.redo = nil
	e.savePoint = 0
	e.changeTick = 0
	e.lastEdit.Valid = false
	e.highlight = editorHighlightState{start: -1, end: -1}
	e.selectionActive = false
	e.clipboard = nil
	e.selectionScopeStack = nil
	e.selectionScopeIndex = 0
	e.searchMatches = nil
	e.searchMatchIndex = 0
	e.updateDirty()
	_ = e.LoadUndoHistory()
	_ = e.syncFileSnapshot()
	e.file.externalChange = ExternalChangeNone

	// Restore session state
	e.restoreSessionState()

	// Add new buffer to manager
	if e.buffers != nil {
		bs := e.snapshotBufferState()
		newIdx := e.buffers.Add(bs)
		e.buffers.SetActive(newIdx)
	}

	return nil
}

// switchToBuffer switches to the buffer at the given index.
func (e *Editor) switchToBuffer(index int) {
	if e.buffers == nil || index < 0 || index >= e.buffers.Count() || index == e.buffers.ActiveIndex() {
		return
	}

	// Save current state
	e.saveSessionState()
	_ = e.SaveUndoHistory()
	bs := e.snapshotBufferState()
	e.buffers.UpdateActive(bs)

	// Switch
	e.buffers.SetActive(index)

	// Restore new buffer state
	e.restoreBufferState(e.buffers.Active())

	// Signal runtime controller.
	e.requests.bufferSwitched = true
}

// gotoNextBuffer switches to the next buffer.
func (e *Editor) gotoNextBuffer() {
	if e.buffers == nil || e.buffers.Count() <= 1 {
		e.setStatus("no other buffers")
		return
	}
	e.switchToBuffer(e.buffers.Next())
}

// gotoPrevBuffer switches to the previous buffer.
func (e *Editor) gotoPrevBuffer() {
	if e.buffers == nil || e.buffers.Count() <= 1 {
		e.setStatus("no other buffers")
		return
	}
	e.switchToBuffer(e.buffers.Prev())
}

// gotoLastAccessedBuffer switches to the previously accessed buffer.
func (e *Editor) gotoLastAccessedBuffer() {
	if e.buffers == nil {
		e.setStatus("no other buffers")
		return
	}
	prev := e.buffers.PrevIndex()
	if prev < 0 || prev >= e.buffers.Count() {
		e.setStatus("no last accessed buffer")
		return
	}
	e.switchToBuffer(prev)
}

// closeCurrentBuffer closes the current buffer. If force is false and the buffer
// is dirty, shows a warning instead.
func (e *Editor) closeCurrentBuffer(force bool) {
	if e.buffers == nil || e.buffers.Count() <= 1 {
		e.setStatus("last buffer (use :q to quit)")
		return
	}
	if !force && e.dirty {
		e.setStatus("unsaved changes (use :bc!)")
		return
	}

	idx := e.buffers.ActiveIndex()
	count := e.buffers.Count()

	// Determine which buffer to switch to after removal.
	// If we're closing the last buffer in the list, go to the previous one.
	// Otherwise, stay at the same index (which will point to the next buffer after removal).
	nextIdx := idx
	if idx >= count-1 {
		nextIdx = idx - 1
	}
	if nextIdx < 0 {
		nextIdx = 0
	}

	// Remove the buffer. This shifts indices >= idx down by 1.
	e.buffers.Remove(idx)

	// After removal, adjust nextIdx if it was shifted.
	if nextIdx > idx {
		nextIdx--
	}

	// Explicitly set the active buffer and restore its state.
	e.buffers.SetActive(nextIdx)
	e.restoreBufferState(e.buffers.Active())
	e.requests.bufferSwitched = true
}

// closeBufferAtIndex closes a buffer at the given index (called from sidebar).
// If it's the active buffer, switches to another one.
func (e *Editor) closeBufferAtIndex(index int) {
	if e.buffers == nil || e.buffers.Count() <= 1 {
		e.setStatus("last buffer (use :q to quit)")
		return
	}
	activeIdx := e.buffers.ActiveIndex()

	if index == activeIdx {
		// Closing the active buffer - same as closeCurrentBuffer(true)
		e.closeCurrentBuffer(true)
		return
	}

	// Closing a non-active buffer
	e.buffers.Remove(index)
}

// openSidebarBuffers opens the sidebar with the buffer list.
func (e *Editor) openSidebarBuffers() {
	if e.sidebar == nil {
		return
	}
	if e.refsPicker.active {
		e.closeRefsPicker(false)
	}
	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo())
	}
	content := NewSidebarBuffersContent(e)
	// Position cursor on active buffer
	if e.buffers != nil {
		content.SetIndex(e.buffers.ActiveIndex())
	}
	e.sidebar.Open(content)
	e.clearFileTreePreview()
}

// ConsumeBufferSwitch returns true and resets the flag if a buffer switch occurred.
func (e *Editor) ConsumeBufferSwitch() bool {
	if !e.requests.bufferSwitched {
		return false
	}
	e.requests.bufferSwitched = false
	return true
}

// BufferCount returns the number of open buffers.
func (e *Editor) BufferCount() int {
	if e.buffers == nil {
		return 0
	}
	return e.buffers.Count()
}

// ActiveBufferIndex returns the index of the active buffer.
func (e *Editor) ActiveBufferIndex() int {
	if e.buffers == nil {
		return 0
	}
	return e.buffers.ActiveIndex()
}
