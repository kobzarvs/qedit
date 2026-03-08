package app

import (
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/lsp"
)

func syncEditorChangeToLSP(ls *lsp.Manager, ed *editor.Editor, state *editorRuntimeState) {
	if ls == nil || ed == nil || state == nil || state.openPath == "" {
		return
	}
	if ed.HugeFileMode() || ed.Filename() != state.openPath {
		return
	}
	tick := ed.ChangeTick()
	if tick == state.lastLSPChangeTick {
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
	if path == "" || ed.Filename() != path || ed.HugeFileMode() {
		return
	}
	ls.DidClose(path)
	ls.OpenFile(path, ed.Content())
	if state != nil {
		state.lastLSPChangeTick = ed.ChangeTick()
	}
}
