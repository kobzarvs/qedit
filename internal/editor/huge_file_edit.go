package editor

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

var errHugeFileUnavailable = errors.New("huge file buffer unavailable")

func copyRunes(src []rune) []rune {
	if len(src) == 0 {
		return nil
	}
	return append([]rune(nil), src...)
}

func equalRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (e *Editor) hugeLine(row int) ([]rune, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return nil, false
	}
	if line, ok := e.huge.edits[row]; ok {
		return copyRunes(line), true
	}
	return e.huge.buffer.Line(row), true
}

func (e *Editor) hugeTryLine(row int) ([]rune, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return nil, false
	}
	if line, ok := e.huge.edits[row]; ok {
		return copyRunes(line), true
	}
	return e.huge.buffer.TryLine(row)
}

func (e *Editor) hugeLineLen(row int) (int, bool) {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return 0, false
	}
	if line, ok := e.huge.edits[row]; ok {
		return len(line), true
	}
	return e.huge.buffer.LineLen(row), true
}

func (e *Editor) hugeSetLine(row int, line []rune) bool {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return false
	}
	if row < 0 || row >= e.LineCount() {
		return false
	}
	base := e.huge.buffer.Line(row)
	if equalRunes(base, line) {
		delete(e.huge.edits, row)
		return true
	}
	if e.huge.edits == nil {
		e.huge.edits = make(map[int][]rune)
	}
	e.huge.edits[row] = copyRunes(line)
	return true
}

func (e *Editor) clearHugeEdits() {
	if !e.hugeFileActive() {
		return
	}
	e.huge.edits = nil
}

func (e *Editor) WriteHugeFileTo(w io.Writer) error {
	if !e.hugeFileActive() || e.huge.buffer == nil {
		return errHugeFileUnavailable
	}
	reader, err := e.huge.buffer.OpenReader()
	if err != nil {
		return err
	}
	defer reader.Close()

	edits := make(map[int]string, len(e.huge.edits))
	for row, line := range e.huge.edits {
		edits[row] = string(line)
	}

	br := bufio.NewReaderSize(reader, 256<<10)
	row := 0
	for {
		lineBytes, err := br.ReadBytes('\n')
		if len(lineBytes) > 0 {
			if replacement, ok := edits[row]; ok {
				if _, writeErr := io.WriteString(w, replacement); writeErr != nil {
					return writeErr
				}
				switch {
				case bytes.HasSuffix(lineBytes, []byte("\r\n")):
					if _, writeErr := io.WriteString(w, "\r\n"); writeErr != nil {
						return writeErr
					}
				case bytes.HasSuffix(lineBytes, []byte("\n")):
					if _, writeErr := io.WriteString(w, "\n"); writeErr != nil {
						return writeErr
					}
				}
			} else {
				if _, writeErr := w.Write(lineBytes); writeErr != nil {
					return writeErr
				}
			}
			delete(edits, row)
			row++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	if replacement, ok := edits[row]; ok {
		if _, err := io.WriteString(w, replacement); err != nil {
			return err
		}
		delete(edits, row)
	}
	if len(edits) > 0 {
		return errors.New("huge file save failed: unresolved edited rows")
	}
	return nil
}

func (e *Editor) WriteHugeFile(path string, store FileStore) error {
	if !e.hugeFileActive() {
		return errors.New("huge file mode is not active")
	}
	if store == nil {
		return errFileStoreUnavailable()
	}
	pr, pw := io.Pipe()
	writeErrCh := make(chan error, 1)
	go func() {
		err := e.WriteHugeFileTo(pw)
		_ = pw.CloseWithError(err)
		writeErrCh <- err
	}()

	writeErr := store.WriteFrom(path, pr)
	streamErr := <-writeErrCh
	if writeErr != nil {
		return writeErr
	}
	if streamErr != nil {
		return streamErr
	}
	return e.ReloadSavedHugeFile(path, store)
}

func (e *Editor) ReloadSavedHugeFile(path string, store FileStore) error {
	if !e.hugeFileActive() {
		return errors.New("huge file mode is not active")
	}
	if store == nil {
		return errFileStoreUnavailable()
	}
	meta, err := store.Stat(path)
	if err != nil {
		return err
	}
	buffer, err := OpenHugeFileBuffer(path, meta, store)
	if err != nil {
		return err
	}

	oldBuffer := e.huge.buffer
	e.huge.active = true
	e.huge.sizeBytes = meta.Size
	e.huge.buffer = buffer
	e.clearHugeEdits()
	e.clearGitDiffPreview()
	e.document.filename = path
	e.file.readOnly = false
	e.file.externalChange = ExternalChangeNone
	e.file.diskContent = ""
	e.file.snapshot = snapshotFromMetadata(meta)
	e.savePoint = len(e.undo)
	e.updateDirty()
	_ = e.SaveUndoHistory()
	e.saveSessionState()
	if e.buffers != nil && e.buffers.Count() > 0 {
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
	}
	e.primeHugeViewport()
	if oldBuffer != nil {
		_ = oldBuffer.Close()
	}
	return nil
}
