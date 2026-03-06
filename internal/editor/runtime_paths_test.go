package editor

import "testing"

type testSessionStore struct {
	states   map[string]FileState
	setPath  string
	setState FileState
}

func (s *testSessionStore) GetFileState(path string) (FileState, bool) {
	state, ok := s.states[path]
	return state, ok
}

func (s *testSessionStore) SetFileState(path string, state FileState) {
	s.setPath = path
	s.setState = state
}

func (s *testSessionStore) Stop() {}

func TestRestoreSessionStateUsesRuntimeFileStoreAbs(t *testing.T) {
	e := newTestEditor("one", "two")
	e.SetFileStore(&testFileStore{
		absPaths: map[string]string{
			"rel.txt": "/abs/rel.txt",
		},
	})
	e.SetSessionStore(&testSessionStore{
		states: map[string]FileState{
			"/abs/rel.txt": {
				CursorRow: 1,
				CursorCol: 2,
				ScrollY:   3,
				ScrollX:   4,
				Mode:      "insert",
			},
		},
	})
	e.document.filename = "rel.txt"

	e.restoreSessionState()

	if e.cursor.Row != 1 || e.cursor.Col != 2 {
		t.Fatalf("cursor = (%d,%d), want (1,2)", e.cursor.Row, e.cursor.Col)
	}
	if e.viewport.scroll != 3 || e.viewport.scrollX != 4 {
		t.Fatalf("scroll = (%d,%d), want (3,4)", e.viewport.scroll, e.viewport.scrollX)
	}
	if e.mode != ModeInsert {
		t.Fatalf("mode = %v, want %v", e.mode, ModeInsert)
	}
}

func TestSaveSessionStateUsesRuntimeFileStoreAbs(t *testing.T) {
	e := newTestEditor("one", "two")
	e.SetFileStore(&testFileStore{
		absPaths: map[string]string{
			"rel.txt": "/abs/rel.txt",
		},
	})
	sessionStore := &testSessionStore{}
	e.SetSessionStore(sessionStore)
	e.document.filename = "rel.txt"
	e.cursor = Cursor{Row: 1, Col: 1}
	e.viewport.scroll = 5
	e.viewport.scrollX = 6
	e.mode = ModeInsert

	e.saveSessionState()

	if sessionStore.setPath != "/abs/rel.txt" {
		t.Fatalf("saved path = %q, want %q", sessionStore.setPath, "/abs/rel.txt")
	}
	if sessionStore.setState.CursorRow != 1 || sessionStore.setState.CursorCol != 1 {
		t.Fatalf("saved cursor = (%d,%d), want (1,1)", sessionStore.setState.CursorRow, sessionStore.setState.CursorCol)
	}
	if sessionStore.setState.ScrollY != 5 || sessionStore.setState.ScrollX != 6 {
		t.Fatalf("saved scroll = (%d,%d), want (5,6)", sessionStore.setState.ScrollY, sessionStore.setState.ScrollX)
	}
	if sessionStore.setState.Mode != "insert" {
		t.Fatalf("saved mode = %q, want %q", sessionStore.setState.Mode, "insert")
	}
}

func TestCloseRefsPickerUsesRuntimeFileStoreAbs(t *testing.T) {
	e := newTestEditor("one", "two", "three")
	e.SetFileStore(&testFileStore{
		absPaths: map[string]string{
			"rel.txt": "/abs/rel.txt",
		},
	})
	e.document.filename = "rel.txt"
	e.refsPicker.active = true
	e.refsPicker.items = []LSPLocation{{
		Path:      "/abs/rel.txt",
		StartLine: 2,
		StartCol:  3,
	}}

	e.closeRefsPicker(true)

	if e.cursor.Row != 2 || e.cursor.Col != 3 {
		t.Fatalf("cursor = (%d,%d), want (2,3)", e.cursor.Row, e.cursor.Col)
	}
	if _, ok := e.ConsumeRuntimeRequest(); ok {
		t.Fatalf("runtime request still present, want queue drained")
	}
}

func TestRelativePathFromWorkingDirUsesRuntimeFileStoreAbs(t *testing.T) {
	e := newTestEditor("one")
	e.SetFileStore(&testFileStore{
		absPaths: map[string]string{
			".":       "/cwd",
			"rel.txt": "/cwd/sub/rel.txt",
		},
	})

	rel, ok := e.relativePathFromWorkingDir("rel.txt")

	if !ok {
		t.Fatalf("relativePathFromWorkingDir ok = false, want true")
	}
	if rel != "sub/rel.txt" {
		t.Fatalf("relativePathFromWorkingDir = %q, want %q", rel, "sub/rel.txt")
	}
}

func TestSidebarGitChangesUsesRuntimeFileStoreAbs(t *testing.T) {
	e := newTestEditor("one")
	e.SetFileStore(&testFileStore{
		absPaths: map[string]string{
			"rel.txt": "/abs/rel.txt",
		},
	})
	content := NewSidebarGitChangesContent(e)

	content.SetCurrentPath("rel.txt")

	if content.currentPath != "/abs/rel.txt" {
		t.Fatalf("currentPath = %q, want %q", content.currentPath, "/abs/rel.txt")
	}
}

func TestBufferManagerUsesRuntimeFileStoreAbs(t *testing.T) {
	e := newTestEditor("one")
	e.SetFileStore(&testFileStore{
		absPaths: map[string]string{
			"rel.txt": "/abs/rel.txt",
		},
	})
	e.buffers.Add(&BufferState{filename: "rel.txt"})

	idx := e.buffers.FindByPath("/abs/rel.txt")

	if idx != 0 {
		t.Fatalf("buffer index = %d, want 0", idx)
	}
}
