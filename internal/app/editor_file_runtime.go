package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

var hugeFileThresholdBytes int64 = 64 << 20

type activeFileState struct {
	openPath           string
	gitPath            string
	langName           string
	highlightEnabled   bool
	highlightExpected  bool
	lastChangeTick     uint64
	lastHighlightStart int
	lastHighlightEnd   int
}

func openRuntimeFile(ed *editor.Editor, screen tcell.Screen, ls *lsp.Manager, ts *treesitter.Engine, langs config.Languages, fileStore editor.FileStore, path string, highlightMaxBytes int64) (activeFileState, error) {
	path = normalizeAppPath(fileStore, strings.TrimSpace(path))
	if path == "" {
		return activeFileState{}, nil
	}
	if !ed.OpenExistingBuffer(path) {
		if fileStore != nil && hugeFileThresholdBytes > 0 {
			if info, err := fileStore.Stat(path); err == nil && info.Size >= hugeFileThresholdBytes {
				if err := ed.LoadHugeFile(path, fileStore, info); err != nil {
					return activeFileState{}, err
				}
				return activateEditorFile(ed, screen, ls, ts, langs, fileStore, path, highlightMaxBytes), nil
			}
		}
		data, err := fileStore.Read(path)
		if err != nil {
			return activeFileState{}, err
		}
		if err := activateRuntimeFile(ed, path, data); err != nil {
			return activeFileState{}, err
		}
	}
	return activateEditorFile(ed, screen, ls, ts, langs, fileStore, path, highlightMaxBytes), nil
}

func activateRuntimeFile(ed *editor.Editor, path string, data []byte) error {
	if ed.OpenExistingBuffer(path) {
		return nil
	}
	return ed.LoadFileContent(path, data)
}

func activateEditorFile(ed *editor.Editor, screen tcell.Screen, ls *lsp.Manager, ts *treesitter.Engine, langs config.Languages, fileStore editor.FileStore, path string, highlightMaxBytes int64) activeFileState {
	content := ed.Content()
	if !ed.HugeFileMode() {
		ls.OpenFile(path, content)
	}

	state := activeFileState{
		openPath:           path,
		gitPath:            path,
		highlightEnabled:   true,
		highlightExpected:  false,
		lastChangeTick:     ed.ChangeTick(),
		lastHighlightStart: -1,
		lastHighlightEnd:   -1,
	}
	if ed.HugeFileMode() {
		ed.SetHighlights(-1, -1, nil)
		state.highlightEnabled = false
		state.highlightExpected = false
		return state
	}
	state.langName, state.highlightEnabled = detectHighlightLanguage(fileStore, path, langs, highlightMaxBytes)
	state.highlightExpected = state.highlightEnabled && state.langName != ""

	if state.highlightEnabled && state.langName != "" {
		if isAsyncParseLang(state.langName) {
			ts.Parse(path, state.langName, content)
		} else if ts.ParseSync(path, state.langName, content) {
			if _, end, ok := applyInitialScreenHighlights(ed, screen, ts, path); ok {
				state.lastHighlightStart = 0
				state.lastHighlightEnd = end
			}
		} else {
			state.highlightExpected = false
			ed.SetHighlights(-1, -1, nil)
		}
	} else {
		ed.SetHighlights(-1, -1, nil)
	}

	return state
}
