package app

import (
	"time"

	"github.com/kobzarvs/qedit/internal/editor"
)

type asyncHighlightResult struct {
	seq   int64
	path  string
	start int
	end   int
	spans map[int][]editor.HighlightSpan
	done  bool
}

type editorRuntimeState struct {
	openPath              string
	gitPath               string
	langName              string
	highlightEnabled      bool
	highlightExpected     bool
	highlightParsed       bool
	hugeFileKind          editor.HugeFileKind
	highlightMaxBytes     int64
	highlightParseVersion uint64
	highlightJobSeq       int64
	highlightJobActive    bool
	highlightJobStart     int
	highlightJobEnd       int
	highlightJobColStart  int
	highlightJobColEnd    int
	highlightJobCancel    chan struct{}
	lastVisibleStart      int
	highlightResults      chan asyncHighlightResult
	quitRequested         bool
	lastGitCheck          time.Time
	lastChangeTick        uint64
	lastLSPChangeTick     uint64
	lastHighlightStart    int
	lastHighlightEnd      int
}

func newEditorRuntimeState(ed *editor.Editor) editorRuntimeState {
	return editorRuntimeState{
		highlightEnabled:     true,
		highlightResults:     make(chan asyncHighlightResult, 16),
		highlightJobStart:    -1,
		highlightJobEnd:      -1,
		highlightJobColStart: -1,
		highlightJobColEnd:   -1,
		lastVisibleStart:     -1,
		lastGitCheck:         time.Now(),
		lastChangeTick:       ed.ChangeTick(),
		lastLSPChangeTick:    ed.ChangeTick(),
		lastHighlightStart:   -1,
		lastHighlightEnd:     -1,
	}
}

func (s *editorRuntimeState) applyActiveFile(state activeFileState) {
	s.openPath = state.openPath
	s.gitPath = state.gitPath
	s.langName = state.langName
	s.highlightEnabled = state.highlightEnabled
	s.highlightExpected = state.highlightExpected
	s.highlightParsed = false
	s.hugeFileKind = state.hugeFileKind
	s.highlightMaxBytes = state.highlightMaxBytes
	s.highlightParseVersion = state.highlightParseVersion
	s.highlightJobSeq++
	s.highlightJobActive = false
	s.highlightJobStart = -1
	s.highlightJobEnd = -1
	s.highlightJobColStart = -1
	s.highlightJobColEnd = -1
	if s.highlightJobCancel != nil {
		close(s.highlightJobCancel)
		s.highlightJobCancel = nil
	}
	s.lastVisibleStart = -1
	s.lastChangeTick = state.lastChangeTick
	s.lastLSPChangeTick = state.lastChangeTick
	s.lastHighlightStart = state.lastHighlightStart
	s.lastHighlightEnd = state.lastHighlightEnd
}
