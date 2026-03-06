package editor

import "strings"

func (e *Editor) clipboardText() string {
	if len(e.clipboard.lines) == 0 {
		return ""
	}
	var lines []string
	for _, line := range e.clipboard.lines {
		lines = append(lines, string(line))
	}
	return strings.Join(lines, "\n")
}

func (e *Editor) queueClipboardWriteRequest(notify bool) {
	text := e.clipboardText()
	if text == "" {
		return
	}
	e.enqueueRuntimeRequest(RuntimeRequest{
		Kind:    RuntimeRequestWriteClipboard,
		Content: text,
		Notify:  notify,
	})
}

func (e *Editor) queueClipboardReadRequest(before bool) {
	e.enqueueRuntimeRequest(RuntimeRequest{
		Kind:   RuntimeRequestReadClipboard,
		Before: before,
	})
}

func (e *Editor) ApplyClipboardText(text string, before bool) {
	lines := strings.Split(text, "\n")
	e.clipboard.lines = make([][]rune, len(lines))
	for i, line := range lines {
		e.clipboard.lines[i] = []rune(line)
	}

	if before {
		e.pasteBefore()
	} else {
		e.pasteAfter()
	}
}
