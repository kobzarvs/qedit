package editor

import "errors"

func (e *Editor) queueFormatRequest() error {
	if isMarkdownFile(e.document.filename) {
		if err := e.FormatMarkdownTables(); err != nil {
			return err
		}
		e.setStatus("formatted")
		return nil
	}

	content := e.Content()
	if isGoFile(e.document.filename) || (e.document.filename == "" && looksLikeGo(content)) {
		e.enqueueRuntimeRequest(RuntimeRequest{
			Kind:    RuntimeRequestFormatBuffer,
			Path:    e.document.filename,
			Content: content,
		})
		return nil
	}

	return errors.New("format not supported")
}

func (e *Editor) ApplyFormattedContent(formatted string) {
	if formatted == e.Content() {
		return
	}
	e.replaceBuffer(formatted, true)
}
