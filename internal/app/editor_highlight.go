package app

import (
	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

func isAsyncParseLang(name string) bool {
	switch name {
	case "typescript", "typescriptreact", "javascript", "javascriptreact", "vue":
		return true
	default:
		return false
	}
}

func detectHighlightLanguage(fileStore editor.FileStore, path string, langs config.Languages, highlightMaxBytes int64) (string, bool) {
	highlightEnabled := true
	if fileStore != nil && highlightMaxBytes > 0 {
		if info, err := fileStore.Stat(path); err == nil && info.Size > highlightMaxBytes {
			highlightEnabled = false
		}
	}
	if !highlightEnabled {
		return "", false
	}
	if lang := langs.Match(path); lang != nil {
		return lang.Name, true
	}
	return "", true
}

func applyInitialScreenHighlights(ed *editor.Editor, screen tcell.Screen, ts *treesitter.Engine, path string) (int, int, bool) {
	start := 0
	_, h := screen.Size()
	viewHeight := h - 2
	if viewHeight < 0 {
		viewHeight = 0
	}
	end := viewHeight - 1
	if end < 0 {
		end = 0
	}
	lineCount := ed.LineCount()
	if lineCount > 0 && end >= lineCount {
		end = lineCount - 1
	}
	return start, end, applyHighlightRange(ed, ts, path, start, end)
}

func applyHighlightRange(ed *editor.Editor, ts *treesitter.Engine, path string, start, end int) bool {
	editorSpans := toEditorHighlightSpans(ts.Highlights(path, start, end))
	if editorSpans == nil {
		return false
	}
	ed.SetHighlights(start, end, editorSpans)
	return true
}
