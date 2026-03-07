package editor

type editorHugeFileState struct {
	active     bool
	sizeBytes  int64
	buffer     *HugeFileBuffer
	edits      map[int][]rune
	patches    []hugeFileRowPatch
	defaultEOL string
	resolve    hugeFileResolveCache
}

type hugeFileResolveCache struct {
	valid        bool
	logicalStart int
	logicalEnd   int
	baseStart    int
	baseDelete   int
	patchIndex   int
}

const hugeFilePrimeViewportLines = 64

func (e *Editor) hugeFileActive() bool {
	return e.huge.active && e.huge.buffer != nil
}

func (e *Editor) HugeFileMode() bool {
	return e.hugeFileActive()
}

func (e *Editor) prefetchHugeViewport(viewHeight int) {
	e.prefetchHugeRows(e.viewport.scroll, viewHeight)
}

func (e *Editor) prefetchHugeRows(startRow, count int) {
	if !e.hugeFileActive() || count <= 0 {
		return
	}
	if startRow < 0 {
		startRow = 0
	}
	if !e.huge.buffer.CanPrefetchQuick(startRow, count) {
		e.huge.buffer.WarmLines(startRow, count)
		return
	}
	_ = e.huge.buffer.PrefetchLines(startRow, count)
}

func (e *Editor) primeHugeViewport() {
	if !e.hugeFileActive() {
		return
	}
	count := e.viewport.height
	if count < hugeFilePrimeViewportLines {
		count = hugeFilePrimeViewportLines
	}
	startRow := e.viewport.scroll
	if startRow < 0 {
		startRow = 0
	}
	e.prefetchHugeRows(startRow, count)
	if e.cursor.Row < startRow || e.cursor.Row >= startRow+count {
		e.prefetchHugeRows(e.cursor.Row, count)
	}
}

func (e *Editor) primeHugeRowsAround(row int) {
	if !e.hugeFileActive() {
		return
	}
	count := e.viewport.height
	if count < hugeFilePrimeViewportLines {
		count = hugeFilePrimeViewportLines
	}
	e.prefetchHugeRows(row, count)
}

func (e *Editor) closeHugeFileBuffer() {
	if e.huge.buffer != nil {
		_ = e.huge.buffer.Close()
	}
	e.huge = editorHugeFileState{}
}
