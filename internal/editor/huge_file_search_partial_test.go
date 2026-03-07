package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHugeFileSearchUsesIndexedPrefixWhileBackgroundIndexingRuns(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	hugeFileInitialSampleBytes = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 6000; i++ {
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
	if err := e.LoadHugeFile(path, slowTestFileStore{maxRead: 128, delay: time.Millisecond}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	if e.huge.buffer.IndexingComplete() {
		t.Fatalf("expected indexing to still be in progress")
	}

	e.searchQuery = []rune("match")
	e.updateSearchMatches()

	if len(e.searchMatches) == 0 {
		t.Fatalf("expected partial search matches")
	}
	if len(e.searchMatches) >= 6000 {
		t.Fatalf("expected partial search to stay below full file match count, got %d", len(e.searchMatches))
	}
	if !strings.Contains(e.ui.statusMessage, "indexed portion") {
		t.Fatalf("status = %q, want indexed-portion message", e.ui.statusMessage)
	}

	e.huge.buffer.WaitForIndexing()
	e.updateSearchMatches()

	if len(e.searchMatches) != 6000 {
		t.Fatalf("full search matches = %d, want %d", len(e.searchMatches), 6000)
	}
}
