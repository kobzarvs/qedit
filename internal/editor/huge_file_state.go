package editor

type editorHugeFileState struct {
	active    bool
	sizeBytes int64
	buffer    *HugeFileBuffer
}

func (e *Editor) hugeFileActive() bool {
	return e.huge.active && e.huge.buffer != nil
}

func (e *Editor) HugeFileMode() bool {
	return e.hugeFileActive()
}

func (e *Editor) prefetchHugeViewport(viewHeight int) {
	if !e.hugeFileActive() || viewHeight <= 0 {
		return
	}
	if !e.huge.buffer.CanPrefetchQuick(e.viewport.scroll, viewHeight) {
		return
	}
	_ = e.huge.buffer.PrefetchLines(e.viewport.scroll, viewHeight)
}

func (e *Editor) closeHugeFileBuffer() {
	if e.huge.buffer != nil {
		_ = e.huge.buffer.Close()
	}
	e.huge = editorHugeFileState{}
}
