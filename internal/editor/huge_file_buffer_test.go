package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenHugeFileBufferIndexesLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\r\ngamma\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, info.Size(), realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if got := buf.LineCount(); got != 4 {
		t.Fatalf("line count = %d, want 4", got)
	}
	if got := string(buf.Line(0)); got != "alpha" {
		t.Fatalf("line 0 = %q, want %q", got, "alpha")
	}
	if got := string(buf.Line(1)); got != "beta" {
		t.Fatalf("line 1 = %q, want %q", got, "beta")
	}
	if got := string(buf.Line(2)); got != "gamma" {
		t.Fatalf("line 2 = %q, want %q", got, "gamma")
	}
	if got := string(buf.Line(3)); got != "" {
		t.Fatalf("line 3 = %q, want empty", got)
	}
}

func TestLoadHugeFileEntersReadOnlyMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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

	if !e.HugeFileMode() {
		t.Fatalf("expected huge file mode to be active")
	}
	if !e.file.readOnly {
		t.Fatalf("expected huge file to be read-only")
	}
	if got := e.LineCount(); got != 4 {
		t.Fatalf("line count = %d, want 4", got)
	}
	if got := string(e.line(1)); got != "beta" {
		t.Fatalf("line 1 = %q, want %q", got, "beta")
	}

	e.SetBehaviorProfile(BehaviorProfileBasic)
	e.HandleKey(keyRune('x'))
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want %v", e.mode, ModeNormal)
	}
}

func TestOpenHugeFileBufferUsesSparseCheckpoints(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < hugeFileCheckpointSpacing*3+17; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, info.Size(), realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if got := buf.LineCount(); got != hugeFileCheckpointSpacing*3+18 {
		t.Fatalf("line count = %d, want %d", got, hugeFileCheckpointSpacing*3+18)
	}
	if len(buf.checkpoints) >= buf.LineCount() {
		t.Fatalf("checkpoint count = %d, want sparse index smaller than line count %d", len(buf.checkpoints), buf.LineCount())
	}
	if got := string(buf.Line(hugeFileCheckpointSpacing * 2)); got != "line" {
		t.Fatalf("line at checkpoint boundary = %q, want %q", got, "line")
	}
	if got := string(buf.Line(buf.LineCount() - 1)); got != "" {
		t.Fatalf("last line = %q, want empty", got)
	}
}

func TestHugeFileBufferPrefetchLinesCachesViewport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 200; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, info.Size(), realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if err := buf.PrefetchLines(50, 12); err != nil {
		t.Fatalf("PrefetchLines returned error: %v", err)
	}
	for row := 50; row < 62; row++ {
		if got := string(buf.lineCache[row]); got != "line" {
			t.Fatalf("cached line %d = %q, want %q", row, got, "line")
		}
	}
}
