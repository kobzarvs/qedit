package editor

import (
	"fmt"
	"strings"
)

func (e *Editor) updateFileTreePreview(content *SidebarFileTreeContent, action SidebarActionData) {
	switch action.Action {
	case SidebarActionClose, SidebarActionBackToMenu, SidebarActionSwitchMode, SidebarActionOpenFile, SidebarActionFocusEditor:
		e.clearFileTreePreview()
		return
	}
	if !content.PreviewEnabled() {
		e.clearFileTreePreview()
		return
	}
	e.fileTreePreviewCurrent(content)
}

func (e *Editor) fileTreePreviewCurrent(content *SidebarFileTreeContent) {
	if content == nil {
		return
	}
	path, isDir := content.SelectedPath()
	if path == "" || isDir {
		e.clearFileTreePreview()
		return
	}
	if e.fileTreePreview.active && e.fileTreePreview.path == path && e.fileTreePreview.text != nil {
		return
	}
	data, err := e.readFilePreview(path)
	if err != nil {
		e.clearFileTreePreview()
		return
	}
	isBinary := isBinaryData(data)
	if isBinary {
		e.fileTreePreview.text = NewTextBufferFromString(formatHexPreview(data))
	} else {
		e.fileTreePreview.text = NewTextBufferFromBytes(data)
	}
	e.fileTreePreview.path = path
	e.fileTreePreview.scroll = 0
	e.fileTreePreview.scrollX = 0
	e.fileTreePreview.active = true
	e.fileTreePreview.binary = isBinary
	e.fileTreePreview.highlight = editorHighlightState{start: -1, end: -1}
	e.updateFileTreePreviewHighlights()
}

func (e *Editor) updateFileTreePreviewHighlights() {
	if e.runtime.highlightRangeFunc == nil || e.fileTreePreview.text == nil || e.fileTreePreview.path == "" || e.fileTreePreview.binary {
		return
	}
	lineCount := e.fileTreePreview.text.LineCount()
	if lineCount <= 0 {
		return
	}
	end := e.viewport.height - 1
	if end < 0 {
		end = 0
	}
	if end >= lineCount {
		end = lineCount - 1
	}
	spans := e.runtime.highlightRangeFunc(e.fileTreePreview.path, 0, end)
	if spans == nil {
		e.fileTreePreview.highlight = editorHighlightState{start: -1, end: -1}
		return
	}
	e.fileTreePreview.highlight.spans = spans
	e.fileTreePreview.highlight.start = 0
	e.fileTreePreview.highlight.end = end
}

func (e *Editor) updateFileTreeStatus(content *SidebarFileTreeContent, action SidebarActionData) {
	if content == nil {
		return
	}
	switch action.Action {
	case SidebarActionClose, SidebarActionBackToMenu, SidebarActionSwitchMode, SidebarActionOpenFile, SidebarActionFocusEditor:
		return
	}
	path, ok := content.selectedRelativePath()
	if !ok || path == "" {
		return
	}
	e.setStatus(path)
}

func (e *Editor) fileTreePreviewVisible() bool {
	if !e.fileTreePreview.active || e.fileTreePreview.text == nil {
		return false
	}
	if e.sidebar == nil || !e.sidebar.Visible || !e.sidebar.Focused {
		return false
	}
	content, ok := e.sidebar.Content.(*SidebarFileTreeContent)
	if !ok || !content.PreviewEnabled() {
		return false
	}
	return true
}

func (e *Editor) clearFileTreePreview() {
	e.fileTreePreview = fileTreePreviewState{
		highlight: editorHighlightState{start: -1, end: -1},
	}
}

const (
	hexPreviewMaxBytes    = 4096
	hexPreviewBytesPerRow = 16
)

func isBinaryData(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 8000 {
		sample = sample[:8000]
	}
	nonText := 0
	for _, b := range sample {
		if b == 0 {
			return true
		}
		if b < 0x09 || (b > 0x0D && b < 0x20) || b == 0x7F {
			nonText++
		}
	}
	return float64(nonText)/float64(len(sample)) > 0.2
}

func formatHexPreview(data []byte) string {
	total := len(data)
	truncated := false
	if len(data) > hexPreviewMaxBytes {
		data = data[:hexPreviewMaxBytes]
		truncated = true
	}
	var b strings.Builder
	for offset := 0; offset < len(data); offset += hexPreviewBytesPerRow {
		end := offset + hexPreviewBytesPerRow
		if end > len(data) {
			end = len(data)
		}
		fmt.Fprintf(&b, "%08x: ", offset)
		for i := offset; i < offset+hexPreviewBytesPerRow; i++ {
			if i < end {
				fmt.Fprintf(&b, "%02x ", data[i])
			} else {
				b.WriteString("   ")
			}
			if i == offset+7 {
				b.WriteByte(' ')
			}
		}
		b.WriteString("|")
		for i := offset; i < end; i++ {
			ch := data[i]
			if ch >= 0x20 && ch <= 0x7E {
				b.WriteByte(ch)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString("|\n")
	}
	if truncated {
		b.WriteString(fmt.Sprintf("... (%d bytes truncated)\n", total-hexPreviewMaxBytes))
	}
	return b.String()
}

func (e *Editor) isBinaryPath(path string) bool {
	data, err := e.readFilePreview(path)
	if err != nil {
		return false
	}
	return isBinaryData(data)
}

func (e *Editor) readFilePreview(path string) ([]byte, error) {
	if e.runtime.fileStore == nil {
		return nil, errFileStoreUnavailable()
	}
	return e.runtime.fileStore.Read(path)
}
