package editor

import "testing"

type testFileStore struct {
	absPath        string
	absPaths       map[string]string
	readData       []byte
	readDataByPath map[string][]byte
	writtenPath    string
	writtenData    []byte
	stats          map[string]FileMetadata
}

func (s *testFileStore) Abs(path string) (string, error) {
	if abs, ok := s.absPaths[path]; ok {
		return abs, nil
	}
	if s.absPath != "" {
		return s.absPath, nil
	}
	return path, nil
}

func (s *testFileStore) Read(path string) ([]byte, error) {
	if data, ok := s.readDataByPath[path]; ok {
		return append([]byte(nil), data...), nil
	}
	return append([]byte(nil), s.readData...), nil
}

func (s *testFileStore) Write(path string, data []byte) error {
	s.writtenPath = path
	s.writtenData = append([]byte(nil), data...)
	return nil
}

func (s *testFileStore) Stat(path string) (FileMetadata, error) {
	if meta, ok := s.stats[path]; ok {
		return meta, nil
	}
	return FileMetadata{}, nil
}

func (s *testFileStore) IsNotExist(err error) bool {
	return false
}

func TestSaveUsesRuntimeFileStore(t *testing.T) {
	e := newTestEditor("hello")
	store := &testFileStore{
		stats: map[string]FileMetadata{
			"/tmp/out.txt": {},
		},
	}
	e.SetFileStore(store)
	e.document.filename = "/tmp/out.txt"

	if err := e.Save(""); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if store.writtenPath != "/tmp/out.txt" {
		t.Fatalf("written path = %q, want %q", store.writtenPath, "/tmp/out.txt")
	}
	if string(store.writtenData) != "hello" {
		t.Fatalf("written data = %q, want %q", string(store.writtenData), "hello")
	}
}

func TestReloadFromDiskUsesRuntimeFileStore(t *testing.T) {
	e := newTestEditor("old")
	store := &testFileStore{
		readData: []byte("new"),
		stats: map[string]FileMetadata{
			"/tmp/file.txt": {},
		},
	}
	e.SetFileStore(store)
	e.document.filename = "/tmp/file.txt"

	if err := e.ReloadFromDisk(true); err != nil {
		t.Fatalf("ReloadFromDisk returned error: %v", err)
	}
	if got := e.Content(); got != "new" {
		t.Fatalf("content = %q, want %q", got, "new")
	}
}

func TestFileTreePreviewUsesRuntimeFileStore(t *testing.T) {
	e := newTestEditor()
	e.SetFileStore(&testFileStore{
		readDataByPath: map[string][]byte{
			"/tmp/file.txt": []byte("preview text"),
		},
	})
	content := &SidebarFileTreeContent{
		items: []SidebarItem{{
			Path: "/tmp/file.txt",
		}},
	}

	e.fileTreePreviewCurrent(content)

	if !e.fileTreePreview.active {
		t.Fatalf("preview active = false, want true")
	}
	if got := e.fileTreePreview.text.String(); got != "preview text" {
		t.Fatalf("preview text = %q, want %q", got, "preview text")
	}
}

func TestIsBinaryPathUsesRuntimeFileStore(t *testing.T) {
	e := newTestEditor()
	e.SetFileStore(&testFileStore{
		readDataByPath: map[string][]byte{
			"/tmp/file.bin": {0x00, 0x01, 0x02},
		},
	})

	if !e.isBinaryPath("/tmp/file.bin") {
		t.Fatalf("isBinaryPath = false, want true")
	}
}
