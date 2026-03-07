package app

import (
	"os"
	"path/filepath"
	"strings"
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

func TestOpenRuntimeFileUsesHugeModeForLongLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "long-line.js")
	if err := os.WriteFile(path, []byte(strings.Repeat("a", 300000)+"\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevSizeThreshold := hugeFileThresholdBytes
	prevLongLineThreshold := hugeFileLongLineThresholdBytes
	prevSampleBytes := hugeFileLongLineSampleBytes
	hugeFileThresholdBytes = 64 << 20
	hugeFileLongLineThresholdBytes = 128 << 10
	hugeFileLongLineSampleBytes = 1 << 20
	defer func() {
		hugeFileThresholdBytes = prevSizeThreshold
		hugeFileLongLineThresholdBytes = prevLongLineThreshold
		hugeFileLongLineSampleBytes = prevSampleBytes
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
		t.Fatalf("expected editor to enter huge file mode for long line")
	}
}
