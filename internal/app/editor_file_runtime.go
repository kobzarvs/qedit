package app

import (
	"io"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

var hugeFileThresholdBytes int64 = 64 << 20
var hugeFileLongLineThresholdBytes = 128 << 10
var hugeFileLongLineSampleBytes int64 = 1 << 20

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
		if fileStore != nil {
			if info, err := fileStore.Stat(path); err == nil && shouldUseHugeFileMode(path, fileStore, info) {
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

func shouldUseHugeFileMode(path string, fileStore editor.FileStore, info editor.FileMetadata) bool {
	if hugeFileThresholdBytes > 0 && info.Size >= hugeFileThresholdBytes {
		return true
	}
	if hugeFileLongLineThresholdBytes <= 0 || hugeFileLongLineSampleBytes == 0 || fileStore == nil {
		return false
	}
	return fileHasLongLine(path, fileStore, hugeFileLongLineThresholdBytes, hugeFileLongLineSampleBytes)
}

func fileHasLongLine(path string, fileStore editor.FileStore, threshold int, sampleBytes int64) bool {
	if threshold <= 0 || sampleBytes == 0 || fileStore == nil {
		return false
	}
	reader, err := fileStore.Open(path)
	if err != nil {
		return false
	}
	defer reader.Close()

	buf := make([]byte, 64<<10)
	remaining := sampleBytes
	currentLineLen := 0
	for {
		if remaining == 0 {
			break
		}
		readSize := len(buf)
		if remaining > 0 && int64(readSize) > remaining {
			readSize = int(remaining)
		}
		n, err := reader.Read(buf[:readSize])
		if n > 0 {
			if remaining > 0 {
				remaining -= int64(n)
			}
			for _, b := range buf[:n] {
				if b == '\n' {
					currentLineLen = 0
					continue
				}
				currentLineLen++
				if currentLineLen >= threshold {
					return true
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
		if n == 0 {
			break
		}
	}
	return currentLineLen >= threshold
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
