package app

import (
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/lsp"
)

func syncEditorChangeToLSP(ls *lsp.Manager, ed *editor.Editor, state *editorRuntimeState) {
	if ls == nil || ed == nil || state == nil || state.openPath == "" {
		return
	}
	if ed.Filename() != state.openPath {
		return
	}
	tick := ed.ChangeTick()
	if tick == state.lastLSPChangeTick {
		return
	}
	if ed.HugeFileMode() {
		if ed.HugeFileKind() != editor.HugeFileKindLongLine {
			return
		}
		content, ok := ed.HighlightContent(state.highlightMaxBytes)
		if !ok {
			return
		}
		ls.DidChange(state.openPath, content)
		state.lastLSPChangeTick = tick
		return
	}
	ls.DidChange(state.openPath, ed.Content())
	state.lastLSPChangeTick = tick
}

func refreshEditorBufferInLSP(ls *lsp.Manager, ed *editor.Editor, state *editorRuntimeState, fileStore editor.FileStore, path string) {
	if ls == nil || ed == nil {
		return
	}
	path = normalizeAppPath(fileStore, path)
	if path == "" || ed.Filename() != path {
		return
	}
	content := ""
	if ed.HugeFileMode() {
		if ed.HugeFileKind() != editor.HugeFileKindLongLine {
			return
		}
		limit := int64(0)
		if state != nil {
			limit = state.highlightMaxBytes
		}
		var ok bool
		content, ok = ed.HighlightContent(limit)
		if !ok {
			return
		}
	} else {
		content = ed.Content()
	}
	ls.DidClose(path)
	ls.OpenFile(path, content)
	if state != nil {
		state.lastLSPChangeTick = ed.ChangeTick()
	}
}
