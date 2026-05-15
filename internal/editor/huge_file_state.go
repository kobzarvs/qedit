package editor

type HugeFileKind string

const (
	HugeFileKindLargeFile HugeFileKind = "large_file"
	HugeFileKindLongLine  HugeFileKind = "long_line"
)

type editorHugeFileState struct {
	active                   bool
	kind                     HugeFileKind
	sizeBytes                int64
	buffer                   *HugeFileBuffer
	edits                    map[int][]rune
	patches                  []hugeFileRowPatch
	defaultEOL               string
	resolve                  hugeFileResolveCache
	deferInitialViewportWarm bool
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
const hugeFileViewportWarmLineLimit = 128 << 10

func (e *Editor) hugeFileActive() bool {
	return e.huge.active && e.huge.buffer != nil
}

func (e *Editor) HugeFileMode() bool {
	return e.hugeFileActive()
}

func (e *Editor) HugeFileKind() HugeFileKind {
	if !e.hugeFileActive() {
		return ""
	}
	if e.huge.kind == "" {
		return HugeFileKindLargeFile
	}
	return e.huge.kind
}

func (e *Editor) hugeFileModeLabel() string {
	switch e.HugeFileKind() {
	case HugeFileKindLongLine:
		return "LONG"
	default:
		return "HUGE"
	}
}

func (e *Editor) hugeFileStatusFlag() string {
	switch e.HugeFileKind() {
	case HugeFileKindLongLine:
		return "[LONG-LINE]"
	default:
		return "[HUGE]"
	}
}

func (e *Editor) skipHeavyHugeFirstPaint() bool {
	return e.hugeFileActive() && e.huge.deferInitialViewportWarm
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
	if e.shouldSkipHugeViewportWarm(startRow, count) {
		return
	}
	if !e.huge.buffer.CanPrefetchQuick(startRow, count) {
		e.huge.buffer.WarmLines(startRow, count)
		return
	}
	_ = e.huge.buffer.PrefetchLines(startRow, count)
}

func (e *Editor) shouldSkipHugeViewportWarm(startRow, count int) bool {
	if !e.hugeFileActive() || count <= 0 {
		return false
	}
	endRow := startRow + count - 1
	checkRows := []int{startRow}
	if e.cursor.Row >= startRow && e.cursor.Row <= endRow && e.cursor.Row != startRow {
		checkRows = append(checkRows, e.cursor.Row)
	}
	for _, row := range checkRows {
		info, ok := e.hugeLineInfoQuick(row)
		if !ok {
			continue
		}
		if !info.hasTabs && info.runeLen > hugeFileViewportWarmLineLimit {
			return true
		}
	}
	return false
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
