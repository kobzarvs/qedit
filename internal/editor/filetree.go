package editor

import (
	"fmt"
	"os"
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
	if e.fileTreePreviewActive && e.fileTreePreviewPath == path && e.fileTreePreviewText != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		e.clearFileTreePreview()
		return
	}
	isBinary := isBinaryData(data)
	if isBinary {
		e.fileTreePreviewText = NewTextBufferFromString(formatHexPreview(data))
	} else {
		e.fileTreePreviewText = NewTextBufferFromBytes(data)
	}
	e.fileTreePreviewPath = path
	e.fileTreePreviewScroll = 0
	e.fileTreePreviewScrollX = 0
	e.fileTreePreviewActive = true
	e.fileTreePreviewBinary = isBinary
	e.fileTreePreviewHighlights = nil
	e.fileTreePreviewHighlightStart = -1
	e.fileTreePreviewHighlightEnd = -1
	e.updateFileTreePreviewHighlights()
}

func (e *Editor) updateFileTreePreviewHighlights() {
	if e.runtime.highlightRangeFunc == nil || e.fileTreePreviewText == nil || e.fileTreePreviewPath == "" || e.fileTreePreviewBinary {
		return
	}
	lineCount := e.fileTreePreviewText.LineCount()
	if lineCount <= 0 {
		return
	}
	end := e.viewHeight - 1
	if end < 0 {
		end = 0
	}
	if end >= lineCount {
		end = lineCount - 1
	}
	spans := e.runtime.highlightRangeFunc(e.fileTreePreviewPath, 0, end)
	if spans == nil {
		e.fileTreePreviewHighlights = nil
		e.fileTreePreviewHighlightStart = -1
		e.fileTreePreviewHighlightEnd = -1
		return
	}
	e.fileTreePreviewHighlights = spans
	e.fileTreePreviewHighlightStart = 0
	e.fileTreePreviewHighlightEnd = end
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
	if !e.fileTreePreviewActive || e.fileTreePreviewText == nil {
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
	e.fileTreePreviewActive = false
	e.fileTreePreviewPath = ""
	e.fileTreePreviewText = nil
	e.fileTreePreviewScroll = 0
	e.fileTreePreviewScrollX = 0
	e.fileTreePreviewBinary = false
	e.fileTreePreviewHighlights = nil
	e.fileTreePreviewHighlightStart = -1
	e.fileTreePreviewHighlightEnd = -1
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

func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8000)
	n, err := f.Read(buf)
	if err != nil || n <= 0 {
		return false
	}
	return isBinaryData(buf[:n])
}
