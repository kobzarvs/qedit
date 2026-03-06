package editor

func (e *Editor) queueFormatRequest() error {
	return e.formatters.Format(e, e.document.filename, e.Content())
}

func (e *Editor) ApplyFormattedContent(formatted string) {
	if formatted == e.Content() {
		return
	}
	e.replaceBuffer(formatted, true)
}
