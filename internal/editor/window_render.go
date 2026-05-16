package editor

func (e *Editor) renderWindows(s Screen, layouts []editorWindowLayout, x, y, w, h int, skipHeavyHugeFirstPaint bool) {
	if len(layouts) == 0 {
		return
	}
	activeSnapshot := e.snapshotBufferState()
	activeBufferIndex := -1
	if e.buffers != nil && e.buffers.Count() > 0 {
		activeBufferIndex = e.buffers.ActiveIndex()
	}
	for _, layout := range layouts {
		if layout.w <= 0 || layout.h <= 0 {
			continue
		}
		if layout.id == e.windows.activeID || layout.bufferIndex == activeBufferIndex {
			e.restoreBufferState(activeSnapshot)
		} else if bs := e.bufferStateForWindow(layout.bufferIndex); bs != nil {
			e.restoreBufferState(bs)
		} else {
			e.clearWindowRect(s, layout)
			continue
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
