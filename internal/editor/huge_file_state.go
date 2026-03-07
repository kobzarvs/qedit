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

func (e *Editor) closeHugeFileBuffer() {
	if e.huge.buffer != nil {
		_ = e.huge.buffer.Close()
	}
	e.huge = editorHugeFileState{}
}
