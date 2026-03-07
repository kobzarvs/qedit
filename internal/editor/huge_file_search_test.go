package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHugeFileSearchLimitsMatchCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < hugeFileSearchMatchLimit+200; i++ {
		content.WriteString("match\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.searchQuery = []rune("match")
	e.updateSearchMatches()

	if got := len(e.searchMatches); got != hugeFileSearchMatchLimit {
		t.Fatalf("match count = %d, want %d", got, hugeFileSearchMatchLimit)
	}
	if !strings.Contains(e.ui.statusMessage, "truncated") {
		t.Fatalf("status = %q, want truncated message", e.ui.statusMessage)
	}
}

func TestHugeFileAllowsSearchActions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.execAction(actionSearchForward)

	if e.mode != ModeSearch {
		t.Fatalf("mode = %v, want %v", e.mode, ModeSearch)
	}
	if strings.Contains(e.ui.statusMessage, "unavailable in huge file mode") {
		t.Fatalf("status = %q, did not expect huge mode rejection", e.ui.statusMessage)
	}
}
