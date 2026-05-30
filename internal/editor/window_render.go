package editor

func (e *Editor) renderWindows(s Screen, layouts []editorWindowLayout, x, y, w, h int, skipHeavyHugeFirstPaint bool) {
	if len(layouts) == 0 {
		return
	}
	e.syncActiveWindowView()

	activeSnapshot := e.snapshotBufferState()
	activeLeaf := e.activeWindowLeaf()
	savedViewportH := e.viewport.height
	savedViewportW := e.viewport.width
	activeBufferIndex := -1
	if e.buffers != nil {
		activeBufferIndex = e.buffers.ActiveIndex()
	}
	defer func() {
		e.restoreBufferState(activeSnapshot)
		e.loadWindowView(activeLeaf)
		e.viewport.height = savedViewportH
		e.viewport.width = savedViewportW
	}()

	for _, layout := range layouts {
		if layout.w <= 0 || layout.h <= 0 {
			continue
		}
		leaf := findWindowLeaf(e.windows.root, layout.id)
		if leaf == nil {
			continue
		}
		bs := e.bufferStateForWindow(layout.bufferIndex)
		if layout.bufferIndex == activeBufferIndex {
			bs = activeSnapshot
		}
		if bs == nil {
			e.clearWindowRect(s, layout)
			continue
		}
		// Panes showing the active buffer use activeSnapshot rather than the
		// lazily refreshed BufferState so render sees live selection/search/
		// highlight state. Each pane still owns cursor/scroll, so re-apply the
		// leaf view after restoring buffer content.
		e.restoreBufferState(bs)
		e.cursor = leaf.view.cursor
		e.viewport.scroll = leaf.view.scroll
		e.viewport.scrollX = leaf.view.scrollX
		if count := e.LineCount(); count > 0 && e.cursor.Row >= count {
			e.cursor.Row = count - 1
		}
		if e.viewport.scroll < 0 {
			e.viewport.scroll = 0
		}

		e.viewport.height = layout.h
		e.viewport.width = layout.w
		gutterWidth := e.gutterWidth()
		if !skipHeavyHugeFirstPaint {
			e.prefetchHugeViewport(layout.h)
		}
		for row := 0; row < layout.h; row++ {
			lineIdx := e.viewport.scroll + row
			screenY := layout.y + row
			if lineIdx >= e.LineCount() {
				clearLineAt(s, layout.x, screenY, layout.w, e.styleMain)
				continue
			}
			e.drawLineWithGutterAt(s, layout.x, screenY, layout.w, gutterWidth, lineIdx)
		}
	}
	e.restoreBufferState(activeSnapshot)
	if activeLeaf != nil {
		e.loadWindowView(activeLeaf)
		e.viewport.height = savedViewportH
		e.viewport.width = savedViewportW
	}
	e.syncActiveWindowView()
	e.drawWindowSeparators(s, x, y, w, h)
}

func (e *Editor) bufferStateForWindow(index int) *BufferState {
	if e.buffers == nil || index < 0 || index >= e.buffers.Count() {
		return nil
	}
	return e.buffers.buffers[index]
}

func (e *Editor) clearWindowRect(s Screen, layout editorWindowLayout) {
	for row := 0; row < layout.h; row++ {
		clearLineAt(s, layout.x, layout.y+row, layout.w, e.styleMain)
	}
}
