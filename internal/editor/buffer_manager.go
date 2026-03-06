package editor

// BufferState captures all per-file state from the Editor so that multiple
// buffers can be tracked independently.
type BufferState struct {
	// Core buffer
	text   *TextBuffer
	cursor Cursor

	// File identity
	filename string
	file     editorFileState
	dirty    bool

	// Undo/redo
	undo            []action
	redo            []action
	savePoint       int
	undoGroup       uint64
	lineUndoRow     int
	lineUndoContent []rune
	lineUndoValid   bool

	// Viewport
	scroll  int
	scrollX int

	// Selection
	selectionActive bool
	selectionStart  Cursor
	selectionEnd    Cursor

	// Search matches (per-file)
	searchMatches    []SearchMatch
	searchMatchIndex int

	// Mode
	mode Mode

	// Syntax highlight cache
	highlight editorHighlightState

	// Change tracking
	changeTick uint64
	lastEdit   TextEdit

	// Merge conflicts
	conflictBlocks      []conflictBlock
	conflictBlocksDirty bool

	// Per-buffer yank clipboard
	clipboard [][]rune

	// Selection scope
	selectionScope selectionScopeState
}

// BufferInfo provides read-only info about a buffer for display purposes.
type BufferInfo struct {
	Index    int
	Filename string
	Dirty    bool
	Active   bool
}

// BufferManager tracks multiple open buffers.
type BufferManager struct {
	buffers        []*BufferState
	activeIndex    int
	prevIndex      int // for "goto last accessed" (ga)
	pathNormalizer func(string) string
}

// NewBufferManager creates a new buffer manager.
func NewBufferManager() *BufferManager {
	return &BufferManager{
		activeIndex: 0,
		prevIndex:   -1,
	}
}

func (bm *BufferManager) SetPathNormalizer(fn func(string) string) {
	bm.pathNormalizer = fn
}

// Add appends a new buffer and returns its index.
func (bm *BufferManager) Add(bs *BufferState) int {
	bm.buffers = append(bm.buffers, bs)
	return len(bm.buffers) - 1
}

// Remove removes a buffer at the given index. Returns false if index is invalid
// or if it's the last buffer.
func (bm *BufferManager) Remove(index int) bool {
	if index < 0 || index >= len(bm.buffers) || len(bm.buffers) <= 1 {
		return false
	}
	bm.buffers = append(bm.buffers[:index], bm.buffers[index+1:]...)
	if bm.activeIndex > index {
		bm.activeIndex--
	} else if bm.activeIndex >= len(bm.buffers) {
		bm.activeIndex = len(bm.buffers) - 1
	}
	if bm.prevIndex == index {
		bm.prevIndex = -1
	} else if bm.prevIndex > index {
		bm.prevIndex--
	}
	return true
}

// Active returns the currently active buffer state.
func (bm *BufferManager) Active() *BufferState {
	if len(bm.buffers) == 0 {
		return nil
	}
	return bm.buffers[bm.activeIndex]
}

// Count returns the number of open buffers.
func (bm *BufferManager) Count() int {
	return len(bm.buffers)
}

// Next returns the index of the next buffer (wrapping around).
func (bm *BufferManager) Next() int {
	if len(bm.buffers) <= 1 {
		return bm.activeIndex
	}
	return (bm.activeIndex + 1) % len(bm.buffers)
}

// Prev returns the index of the previous buffer (wrapping around).
func (bm *BufferManager) Prev() int {
	if len(bm.buffers) <= 1 {
		return bm.activeIndex
	}
	return (bm.activeIndex - 1 + len(bm.buffers)) % len(bm.buffers)
}

// FindByPath returns the index of the buffer with the given absolute path,
// or -1 if not found.
func (bm *BufferManager) FindByPath(absPath string) int {
	for i, bs := range bm.buffers {
		bsAbs := bs.filename
		if bm.pathNormalizer != nil {
			bsAbs = bm.pathNormalizer(bsAbs)
		}
		if bsAbs == absPath {
			return i
		}
	}
	return -1
}

// Items returns a list of BufferInfo for display.
func (bm *BufferManager) Items() []BufferInfo {
	items := make([]BufferInfo, len(bm.buffers))
	for i, bs := range bm.buffers {
		items[i] = BufferInfo{
			Index:    i,
			Filename: bs.filename,
			Dirty:    bs.dirty,
			Active:   i == bm.activeIndex,
		}
	}
	return items
}

// HasDirtyBuffers returns true if any buffer has unsaved changes.
func (bm *BufferManager) HasDirtyBuffers() bool {
	for _, bs := range bm.buffers {
		if bs.dirty {
			return true
		}
	}
	return false
}

// SetActive sets the active buffer index, updating prevIndex.
func (bm *BufferManager) SetActive(index int) {
	if index < 0 || index >= len(bm.buffers) || index == bm.activeIndex {
		return
	}
	bm.prevIndex = bm.activeIndex
	bm.activeIndex = index
}

// UpdateActive updates the active buffer state in the manager.
func (bm *BufferManager) UpdateActive(bs *BufferState) {
	if len(bm.buffers) == 0 {
		return
	}
	bm.buffers[bm.activeIndex] = bs
}

// ActiveIndex returns the current active buffer index.
func (bm *BufferManager) ActiveIndex() int {
	return bm.activeIndex
}

// PrevIndex returns the previously active buffer index (-1 if none).
func (bm *BufferManager) PrevIndex() int {
	return bm.prevIndex
}

// snapshotBufferState copies per-file fields from the Editor into a BufferState.
func (e *Editor) snapshotBufferState() *BufferState {
	return &BufferState{
		text:                e.text,
		cursor:              e.cursor,
		filename:            e.document.filename,
		file:                e.file,
		dirty:               e.document.dirty,
		undo:                e.undo,
		redo:                e.redo,
		savePoint:           e.savePoint,
		undoGroup:           e.undoGroup,
		lineUndoRow:         e.lineUndoRow,
		lineUndoContent:     e.lineUndoContent,
		lineUndoValid:       e.lineUndoValid,
		scroll:              e.viewport.scroll,
		scrollX:             e.viewport.scrollX,
		selectionActive:     e.selectionActive,
		selectionStart:      e.selectionStart,
		selectionEnd:        e.selectionEnd,
		searchMatches:       e.searchMatches,
		searchMatchIndex:    e.searchMatchIndex,
		mode:                e.mode,
		highlight:           e.highlight,
		changeTick:          e.change.tick,
		lastEdit:            e.change.lastEdit,
		conflictBlocks:      e.conflicts.blocks,
		conflictBlocksDirty: e.conflicts.dirty,
		clipboard:           e.clipboard.lines,
		selectionScope:      e.selectionScope,
	}
}

// restoreBufferState writes per-file fields from a BufferState back into the Editor.
func (e *Editor) restoreBufferState(bs *BufferState) {
	e.text = bs.text
	e.cursor = bs.cursor
	e.document.filename = bs.filename
	e.file = bs.file
	e.document.dirty = bs.dirty
	e.undo = bs.undo
	e.redo = bs.redo
	e.savePoint = bs.savePoint
	e.undoGroup = bs.undoGroup
	e.lineUndoRow = bs.lineUndoRow
	e.lineUndoContent = bs.lineUndoContent
	e.lineUndoValid = bs.lineUndoValid
	e.viewport.scroll = bs.scroll
	e.viewport.scrollX = bs.scrollX
	e.selectionActive = bs.selectionActive
	e.selectionStart = bs.selectionStart
	e.selectionEnd = bs.selectionEnd
	e.searchMatches = bs.searchMatches
	e.searchMatchIndex = bs.searchMatchIndex
	e.mode = bs.mode
	e.highlight = bs.highlight
	e.change.tick = bs.changeTick
	e.change.lastEdit = bs.lastEdit
	e.conflicts = editorConflictState{
		blocks: bs.conflictBlocks,
		dirty:  bs.conflictBlocksDirty,
	}
	e.clipboard.lines = bs.clipboard
	e.selectionScope = bs.selectionScope
}
