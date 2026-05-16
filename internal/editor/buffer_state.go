package editor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func (e *Editor) OpenFile(path string) error {
	if e.workspaceFileStore() == nil {
		return errFileStoreUnavailable()
	}
	absPath := path
	if abs, err := e.workspaceAbs(path); err == nil {
		absPath = abs
	}
	if e.OpenExistingBuffer(absPath) {
		return nil
	}
	data, err := e.workspaceRead(absPath)
	if err != nil {
		return err
	}
	return e.LoadFileContent(absPath, data)
}

// OpenExistingBuffer switches to an already opened buffer when present.
func (e *Editor) OpenExistingBuffer(path string) bool {
	if e.buffers != nil && e.buffers.Count() > 0 {
		if idx := e.buffers.FindByPath(path); idx >= 0 {
			e.switchToBuffer(idx)
			return true
		}
	}
	return false
}

func (e *Editor) syncActiveBufferState() {
	if e.buffers == nil || e.buffers.Count() == 0 {
		return
	}
	e.buffers.UpdateActive(e.snapshotBufferState())
}

// LoadFileContent replaces the current editor buffer with caller-supplied file contents.
func (e *Editor) LoadFileContent(path string, data []byte) error {
	if e.OpenExistingBuffer(path) {
		return nil
	}
	// Save current buffer state before opening new file
	if e.buffers != nil && e.buffers.Count() > 0 {
		e.saveSessionState()
		_ = e.SaveUndoHistory()
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
	}

	e.text = NewTextBufferFromBytes(data)
	e.huge = editorHugeFileState{}
	e.clearGitDiffPreview()
	e.cursor = Cursor{}
	e.file.diskContent = e.Content()
	e.file.readOnly = false
	e.resetConflictBlocks()
	e.viewport.scroll = 0
	e.viewport.scrollX = 0
	e.mode = ModeNormal
	e.document.filename = path
	e.document.title = ""
	e.commandLine.text = e.commandLine.text[:0]
	e.ui.statusMessage = ""
	e.undo = nil
	e.redo = nil
	e.savePoint = 0
	e.change.tick = 0
	e.change.lastEdit.Valid = false
	e.highlight = editorHighlightState{start: -1, end: -1}
	e.selectionActive = false
	e.clipboard = editorClipboardState{}
	e.selectionScope = selectionScopeState{}
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
		e.setActiveWindowBufferIndex(newIdx)
	}

	return nil
}

func (e *Editor) LoadHugeFile(path string, store FileStore, meta FileMetadata) error {
	return e.LoadHugeFileWithKind(path, store, meta, HugeFileKindLargeFile)
}

func (e *Editor) LoadHugeFileWithKind(path string, store FileStore, meta FileMetadata, kind HugeFileKind) error {
	if kind == "" {
		kind = HugeFileKindLargeFile
	}
	if e.OpenExistingBuffer(path) {
		return nil
	}
	if store == nil {
		return errFileStoreUnavailable()
	}
	if e.buffers != nil && e.buffers.Count() > 0 {
		e.saveSessionState()
		_ = e.SaveUndoHistory()
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
	}

	buffer, err := OpenHugeFileBuffer(path, meta, store)
	if err != nil {
		return err
	}

	e.text = nil
	e.huge = editorHugeFileState{
		active:                   true,
		kind:                     kind,
		sizeBytes:                meta.Size,
		buffer:                   buffer,
		deferInitialViewportWarm: true,
	}
	e.clearGitDiffPreview()
	e.cursor = Cursor{}
	e.file.diskContent = ""
	e.file.readOnly = false
	e.resetConflictBlocks()
	e.viewport.scroll = 0
	e.viewport.scrollX = 0
	e.mode = ModeNormal
	e.document.filename = path
	e.document.title = ""
	e.commandLine.text = e.commandLine.text[:0]
	e.ui.statusMessage = ""
	e.undo = nil
	e.redo = nil
	e.savePoint = 0
	e.change.tick = 0
	e.change.lastEdit.Valid = false
	e.highlight = editorHighlightState{start: -1, end: -1}
	e.selectionActive = false
	e.clipboard = editorClipboardState{}
	e.selectionScope = selectionScopeState{}
	e.searchMatches = nil
	e.searchMatchIndex = 0
	e.updateDirty()
	_ = e.syncFileSnapshot()
	e.file.externalChange = ExternalChangeNone
	e.restoreSessionState()
	if e.buffers != nil {
		bs := e.snapshotBufferState()
		newIdx := e.buffers.Add(bs)
		e.buffers.SetActive(newIdx)
		e.setActiveWindowBufferIndex(newIdx)
	}
	status := fmt.Sprintf("huge file mode: limited edit (%.1f MB)", float64(meta.Size)/(1024*1024))
	if kind == HugeFileKindLongLine {
		status = fmt.Sprintf("long-line mode: optimized rendering (%.1f MB)", float64(meta.Size)/(1024*1024))
	}
	if !buffer.IndexingComplete() {
		status += ", indexing..."
	}
	e.setStatus(status)
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
	e.setActiveWindowBufferIndex(index)

	// Signal runtime controller.
	e.enqueueRuntimeRequest(RuntimeRequest{Kind: RuntimeRequestBufferSwitched})
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

func (e *Editor) showBufferList() {
	if e.buffers == nil || e.buffers.Count() == 0 {
		e.setStatus("no buffers")
		return
	}
	e.syncActiveBufferState()
	infos := e.buffers.Items()
	parts := make([]string, 0, len(infos))
	prev := e.buffers.PrevIndex()
	for _, info := range infos {
		marker := " "
		if info.Active {
			marker = "%"
		} else if info.Index == prev {
			marker = "#"
		}
		dirty := " "
		if info.Dirty {
			dirty = "+"
		}
		parts = append(parts, fmt.Sprintf("%d%s%s %s", info.Index+1, marker, dirty, e.bufferDisplayName(info)))
	}
	e.setStatus("buffers: " + strings.Join(parts, " | "))
}

func (e *Editor) switchToBufferTarget(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		e.showBufferList()
		return false
	}
	index, ok := e.resolveBufferTarget(target)
	if !ok {
		return false
	}
	e.switchToBuffer(index)
	if e.buffers != nil && index >= 0 && index < e.buffers.Count() {
		e.setStatus(fmt.Sprintf("buffer %d/%d: %s", index+1, e.buffers.Count(), e.bufferDisplayName(e.buffers.Items()[index])))
	}
	return false
}

func (e *Editor) closeBufferTarget(target string, force bool) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		e.closeCurrentBuffer(force)
		return false
	}
	index, ok := e.resolveBufferTarget(target)
	if !ok {
		return false
	}
	e.closeBufferByIndex(index, force)
	return false
}

func (e *Editor) resolveBufferTarget(target string) (int, bool) {
	if e.buffers == nil || e.buffers.Count() == 0 {
		e.setStatus("no buffers")
		return -1, false
	}
	e.syncActiveBufferState()
	target = strings.TrimSpace(target)
	if target == "" {
		e.setStatus("buffer target required")
		return -1, false
	}
	if target == "#" {
		prev := e.buffers.PrevIndex()
		if prev < 0 || prev >= e.buffers.Count() {
			e.setStatus("no alternate buffer")
			return -1, false
		}
		return prev, true
	}
	if n, err := strconv.Atoi(target); err == nil {
		index := n - 1
		if index < 0 || index >= e.buffers.Count() {
			e.setStatus("buffer not found: " + target)
			return -1, false
		}
		return index, true
	}

	infos := e.buffers.Items()
	for _, info := range infos {
		if e.bufferTargetExactMatch(info, target) {
			return info.Index, true
		}
	}

	var matches []BufferInfo
	for _, info := range infos {
		if e.bufferTargetPartialMatch(info, target) {
			matches = append(matches, info)
		}
	}
	switch len(matches) {
	case 0:
		e.setStatus("buffer not found: " + target)
		return -1, false
	case 1:
		return matches[0].Index, true
	default:
		e.setStatus("buffer name is ambiguous: " + target)
		return -1, false
	}
}

func (e *Editor) closeBufferByIndex(index int, force bool) bool {
	if e.buffers == nil || e.buffers.Count() <= 1 {
		e.setStatus("last buffer (use :q to quit)")
		return false
	}
	if index < 0 || index >= e.buffers.Count() {
		e.setStatus("buffer not found")
		return false
	}
	if index == e.buffers.ActiveIndex() {
		e.closeCurrentBuffer(force)
		return true
	}
	bs := e.buffers.buffers[index]
	if bs == nil {
		e.setStatus("buffer not found")
		return false
	}
	if !force && bs.dirty {
		e.setStatus("unsaved changes (use :bd!)")
		return false
	}
	name := e.bufferDisplayName(e.buffers.Items()[index])
	if bs.huge.buffer != nil {
		_ = bs.huge.buffer.Close()
	}
	if !e.buffers.Remove(index) {
		e.setStatus("buffer not found")
		return false
	}
	e.adjustWindowsAfterBufferRemove(index)
	e.setStatus("closed buffer: " + name)
	return true
}

func (e *Editor) bufferDisplayName(info BufferInfo) string {
	if info.Filename != "" {
		if rel, ok := e.relativePathFromWorkingDir(info.Filename); ok && len(rel) < len(info.Filename) {
			return rel
		}
		return info.Filename
	}
	if info.Title != "" {
		return info.Title
	}
	return "[No Name]"
}

func (e *Editor) bufferTargetExactMatch(info BufferInfo, target string) bool {
	for _, name := range e.bufferTargetNames(info) {
		if name == target {
			return true
		}
	}
	return false
}

func (e *Editor) bufferTargetPartialMatch(info BufferInfo, target string) bool {
	target = strings.ToLower(target)
	for _, name := range e.bufferTargetNames(info) {
		if strings.Contains(strings.ToLower(name), target) {
			return true
		}
	}
	return false
}

func (e *Editor) bufferTargetNames(info BufferInfo) []string {
	var names []string
	if info.Filename != "" {
		names = append(names, info.Filename, filepath.Base(info.Filename))
		if rel, ok := e.relativePathFromWorkingDir(info.Filename); ok {
			names = append(names, rel)
		}
	}
	if info.Title != "" {
		names = append(names, info.Title)
	}
	return names
}

// closeCurrentBuffer closes the current buffer. If force is false and the buffer
// is dirty, shows a warning instead.
func (e *Editor) closeCurrentBuffer(force bool) {
	if e.buffers == nil || e.buffers.Count() <= 1 {
		e.setStatus("last buffer (use :q to quit)")
		return
	}
	if !force && e.document.dirty {
		e.setStatus("unsaved changes (use :bd!)")
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
	if closing := e.buffers.buffers[idx]; closing != nil && closing.huge.buffer != nil {
		_ = closing.huge.buffer.Close()
	}
	e.buffers.Remove(idx)
	e.adjustWindowsAfterBufferRemove(idx)

	// After removal, adjust nextIdx if it was shifted.
	if nextIdx > idx {
		nextIdx--
	}

	// Explicitly set the active buffer and restore its state.
	e.buffers.SetActive(nextIdx)
	e.restoreBufferState(e.buffers.Active())
	e.setActiveWindowBufferIndex(e.buffers.ActiveIndex())
	e.enqueueRuntimeRequest(RuntimeRequest{Kind: RuntimeRequestBufferSwitched})
	e.setStatus("closed buffer")
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
	if closing := e.buffers.buffers[index]; closing != nil && closing.huge.buffer != nil {
		_ = closing.huge.buffer.Close()
	}
	e.buffers.Remove(index)
	e.adjustWindowsAfterBufferRemove(index)
}

// openSidebarBuffers opens the sidebar with the buffer list.
func (e *Editor) openSidebarBuffers() {
	if e.sidebar == nil {
		return
	}
	if e.refsPicker.active {
		e.closeRefsPicker(false)
	}
	e.syncActiveBufferState()
	e.refreshSidebarMenu()
	content := NewSidebarBuffersContent(e)
	// Position cursor on active buffer
	if e.buffers != nil {
		content.SetIndex(e.buffers.ActiveIndex())
	}
	e.sidebar.Open(content)
	e.clearFileTreePreview()
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
