package editor

import (
	"bytes"
	"errors"
)

var errHighlightContentTooLarge = errors.New("highlight content exceeds limit")

type highlightContentWriter struct {
	buf   bytes.Buffer
	limit int64
	size  int64
}

func (w *highlightContentWriter) Write(p []byte) (int, error) {
	if w.limit > 0 && w.size+int64(len(p)) > w.limit {
		return 0, errHighlightContentTooLarge
	}
	n, err := w.buf.Write(p)
	w.size += int64(n)
	return n, err
}

// HighlightContent returns the current buffer text when it is small enough to
// reparse for syntax highlighting. Huge buffers are streamed through their
// overlay edits so dirty long-line files can still be highlighted without using
// Content(), which is intentionally empty in huge-file mode.
func (e *Editor) HighlightContent(maxBytes int64) (string, bool) {
	if !e.hugeFileActive() {
		content := e.Content()
		if maxBytes > 0 && int64(len(content)) > maxBytes {
			return "", false
		}
		return content, true
	}
	if maxBytes > 0 && e.huge.sizeBytes > maxBytes {
		return "", false
	}
	var w highlightContentWriter
	w.limit = maxBytes
	if err := e.WriteHugeFileTo(&w); err != nil {
		return "", false
	}
	return w.buf.String(), true
}
