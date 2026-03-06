package editor

import (
	"errors"
	"testing"
)

var errTestFileStoreNotExist = errors.New("file store entry not found")

type testFileStore struct {
	absPath        string
	absPaths       map[string]string
	homeDir        string
	readData       []byte
	readDataByPath map[string][]byte
	dirEntries     map[string][]DirEntry
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

func (s *testFileStore) HomeDir() (string, error) {
	return s.homeDir, nil
}

func (s *testFileStore) Read(path string) ([]byte, error) {
	if data, ok := s.readDataByPath[path]; ok {
		return append([]byte(nil), data...), nil
	}
	return append([]byte(nil), s.readData...), nil
}

func (s *testFileStore) ReadDir(path string) ([]DirEntry, error) {
	entries, ok := s.dirEntries[path]
	if !ok {
		return nil, errTestFileStoreNotExist
	}
	return append([]DirEntry(nil), entries...), nil
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
	return FileMetadata{}, errTestFileStoreNotExist
}

func (s *testFileStore) IsNotExist(err error) bool {
	return errors.Is(err, errTestFileStoreNotExist)
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

func TestOpenExistingBufferSwitchesWithoutRead(t *testing.T) {
	e := newTestEditor("initial")
	store := &testFileStore{
		stats: map[string]FileMetadata{
			"/tmp/a.txt": {},
			"/tmp/b.txt": {},
		},
	}
	e.SetFileStore(store)

	if err := e.LoadFileContent("/tmp/a.txt", []byte("alpha")); err != nil {
		t.Fatalf("LoadFileContent(a) returned error: %v", err)
	}
	if err := e.LoadFileContent("/tmp/b.txt", []byte("beta")); err != nil {
		t.Fatalf("LoadFileContent(b) returned error: %v", err)
	}

	if !e.OpenExistingBuffer("/tmp/a.txt") {
		t.Fatalf("OpenExistingBuffer returned false, want true")
	}
	if got := e.Filename(); got != "/tmp/a.txt" {
		t.Fatalf("filename = %q, want %q", got, "/tmp/a.txt")
	}
	if got := e.Content(); got != "alpha" {
		t.Fatalf("content = %q, want %q", got, "alpha")
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

func TestSidebarFileTreeContentUsesRuntimeFileStoreReadDir(t *testing.T) {
	store := &testFileStore{
		absPaths: map[string]string{
			".":   "/project",
			"rel": "/project",
		},
		dirEntries: map[string][]DirEntry{
			"/project": {
				{Name: "b.txt"},
				{Name: "a", IsDir: true},
			},
		},
		stats: map[string]FileMetadata{
			"rel":      {IsDir: true},
			"/project": {IsDir: true},
		},
	}

	content := NewSidebarFileTreeContent(store, "rel", false, false)

	items := content.Items()
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if items[0].Label != "a" || !items[0].IsDir {
		t.Fatalf("first item = %#v, want directory %q", items[0], "a")
	}
	if items[1].Label != "b.txt" || items[1].IsDir {
		t.Fatalf("second item = %#v, want file %q", items[1], "b.txt")
	}
}
