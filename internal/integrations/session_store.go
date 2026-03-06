package integrations

import (
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/session"
)

// SessionStore adapts session.Manager to editor.SessionStore.
type SessionStore struct {
	mgr *session.Manager
}

func NewSessionStore(mgr *session.Manager) *SessionStore {
	if mgr == nil {
		return nil
	}
	return &SessionStore{mgr: mgr}
}

func (s *SessionStore) GetFileState(path string) (editor.FileState, bool) {
	if s == nil || s.mgr == nil {
		return editor.FileState{}, false
	}
	state, ok := s.mgr.GetFileState(path)
	if !ok {
		return editor.FileState{}, false
	}
	return editor.FileState{
		CursorRow:         state.CursorRow,
		CursorCol:         state.CursorCol,
		ScrollY:           state.ScrollY,
		ScrollX:           state.ScrollX,
		Mode:              state.Mode,
		SelectionActive:   state.SelectionActive,
		SelectionStartRow: state.SelectionStartRow,
		SelectionStartCol: state.SelectionStartCol,
		SelectionEndRow:   state.SelectionEndRow,
		SelectionEndCol:   state.SelectionEndCol,
	}, true
}

func (s *SessionStore) SetFileState(path string, state editor.FileState) {
	if s == nil || s.mgr == nil {
		return
	}
	s.mgr.SetFileState(path, session.FileState{
		CursorRow:         state.CursorRow,
		CursorCol:         state.CursorCol,
		ScrollY:           state.ScrollY,
		ScrollX:           state.ScrollX,
		Mode:              state.Mode,
		SelectionActive:   state.SelectionActive,
		SelectionStartRow: state.SelectionStartRow,
		SelectionStartCol: state.SelectionStartCol,
		SelectionEndRow:   state.SelectionEndRow,
		SelectionEndCol:   state.SelectionEndCol,
	})
}

func (s *SessionStore) Stop() {
	if s == nil || s.mgr == nil {
		return
	}
	s.mgr.Stop()
}
