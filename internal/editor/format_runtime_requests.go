package editor

import "errors"

func (e *Editor) queueFormatRequest() error {
	if e.hugeFileActive() {
		return errors.New("format is unavailable in huge file mode")
	}
	return e.formatters.Format(e, e.document.filename, e.Content())
}

func (e *Editor) ApplyFormattedContent(formatted string) {
	if formatted == e.Content() {
		return
	}
	e.replaceBuffer(formatted, true)
}
