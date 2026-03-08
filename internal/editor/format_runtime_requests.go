package editor

import "errors"

func (e *Editor) queueFormatRequest() error {
	if e.hugeFileActive() {
		if e.document.dirty {
			return errors.New("format with unsaved huge-file edits is unavailable")
		}
		if e.document.filename == "" {
			return errors.New("format not supported")
		}
		return e.formatters.Format(e, e.document.filename, "")
	}
	return e.formatters.Format(e, e.document.filename, e.Content())
}

func (e *Editor) ApplyFormattedContent(formatted string) {
	if e.hugeFileActive() {
		e.closeHugeFileBuffer()
		e.highlight.reset()
	}
	if formatted == e.Content() {
		return
	}
	e.replaceBuffer(formatted, true)
}
