package editor

import (
	"os"
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
	e.fileTreePreviewText = NewTextBufferFromBytes(data)
	e.fileTreePreviewPath = path
	e.fileTreePreviewScroll = 0
	e.fileTreePreviewScrollX = 0
	e.fileTreePreviewActive = true
	e.fileTreePreviewHighlights = nil
	e.fileTreePreviewHighlightStart = -1
	e.fileTreePreviewHighlightEnd = -1
	e.updateFileTreePreviewHighlights()
}

func (e *Editor) updateFileTreePreviewHighlights() {
	if e.highlightRangeFunc == nil || e.fileTreePreviewText == nil || e.fileTreePreviewPath == "" {
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
	spans := e.highlightRangeFunc(e.fileTreePreviewPath, 0, end)
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
	e.fileTreePreviewHighlights = nil
	e.fileTreePreviewHighlightStart = -1
	e.fileTreePreviewHighlightEnd = -1
}
