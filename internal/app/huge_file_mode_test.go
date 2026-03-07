package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/integrations"
)

func TestOpenRuntimeFileUsesHugeFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevThreshold := hugeFileThresholdBytes
	hugeFileThresholdBytes = 4
	defer func() {
		hugeFileThresholdBytes = prevThreshold
	}()

	ed := editor.New(editor.Options{})
	state, err := openRuntimeFile(ed, nil, nil, nil, config.Languages{}, integrations.FileStore{}, path, 0)
	if err != nil {
		t.Fatalf("openRuntimeFile returned error: %v", err)
	}

	if state.openPath != path {
		t.Fatalf("open path = %q, want %q", state.openPath, path)
	}
	if !ed.HugeFileMode() {
		t.Fatalf("expected editor to enter huge file mode")
	}
	if ed.LineCount() != 4 {
		t.Fatalf("line count = %d, want 4", ed.LineCount())
	}
}
