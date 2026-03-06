package app

import (
	"time"

	"github.com/kobzarvs/qedit/internal/editor"
)

type editorRuntimeState struct {
	openPath           string
	gitPath            string
	langName           string
	highlightEnabled   bool
	highlightExpected  bool
	lastGitCheck       time.Time
	lastChangeTick     uint64
	lastHighlightStart int
	lastHighlightEnd   int
}

func newEditorRuntimeState(ed *editor.Editor) editorRuntimeState {
	return editorRuntimeState{
		highlightEnabled:   true,
		lastGitCheck:       time.Now(),
		lastChangeTick:     ed.ChangeTick(),
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
	s.lastChangeTick = state.lastChangeTick
	s.lastHighlightStart = state.lastHighlightStart
	s.lastHighlightEnd = state.lastHighlightEnd
}
