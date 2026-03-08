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
	openPath           string
	gitPath            string
	langName           string
	highlightEnabled   bool
	highlightExpected  bool
	highlightParsed    bool
	highlightJobSeq    int64
	highlightJobActive bool
	highlightJobStart  int
	highlightJobEnd    int
	highlightJobCancel chan struct{}
	lastVisibleStart   int
	highlightResults   chan asyncHighlightResult
	quitRequested      bool
	lastGitCheck       time.Time
	lastChangeTick     uint64
	lastLSPChangeTick  uint64
	lastHighlightStart int
	lastHighlightEnd   int
}

func newEditorRuntimeState(ed *editor.Editor) editorRuntimeState {
	return editorRuntimeState{
		highlightEnabled:   true,
		highlightResults:   make(chan asyncHighlightResult, 4),
		lastVisibleStart:   -1,
		lastGitCheck:       time.Now(),
		lastChangeTick:     ed.ChangeTick(),
		lastLSPChangeTick:  ed.ChangeTick(),
		lastHighlightStart: -1,
		lastHighlightEnd:   -1,
	}
}

func (s *editorRuntimeState) applyActiveFile(state activeFileState) {
	s.openPath = state.openPath
	s.gitPath = state.gitPath
	s.langName = state.langName
	s.highlightEnabled = state.highlightEnabled
	s.highlightExpected = state.highlightExpected
	s.highlightParsed = false
	s.highlightJobSeq++
	s.highlightJobActive = false
	s.highlightJobStart = -1
	s.highlightJobEnd = -1
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
