package editor

import "time"

// yankToSystemClipboard copies selection to system clipboard
func (e *Editor) yankToSystemClipboard() {
	hadSelection := e.fillClipboardFromSelection()
	e.copyToSystemClipboard(true)
	e.modal.lastCommand = "y"
	e.ui.copiedMessageTime = time.Now()
	if hadSelection {
		e.clearSelection()
		e.modal.selectMode = false
	}
}

// pasteFromSystemClipboard pastes from system clipboard
func (e *Editor) pasteFromSystemClipboard(before bool) {
	e.queueClipboardReadRequest(before)
}
