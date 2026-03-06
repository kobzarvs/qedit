package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExternalChangeDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	e := newTestEditor()
	if err := e.OpenFile(path); err != nil {
		t.Fatalf("open file: %v", err)
	}
	if change, err := e.CheckExternalChange(); err != nil || change != ExternalChangeNone {
		t.Fatalf("initial change=%v err=%v, want none", change, err)
	}

	if err := os.WriteFile(path, []byte("world"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	bump := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, bump, bump)

	change, err := e.CheckExternalChange()
	if err != nil {
		t.Fatalf("check change: %v", err)
	}
	if change != ExternalChangeModified {
		t.Fatalf("change=%v, want modified", change)
	}
}

func TestReloadFromDiskClearsExternalChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	e := newTestEditor()
	if err := e.OpenFile(path); err != nil {
		t.Fatalf("open file: %v", err)
	}

	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	e.SetExternalChange(ExternalChangeModified)

	if err := e.ReloadFromDisk(true); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := e.Content(); got != "two" {
		t.Fatalf("content=%q, want %q", got, "two")
	}
	if e.IsDirty() {
		t.Fatalf("expected clean buffer after reload")
	}
	if e.ExternalChange() != ExternalChangeNone {
		t.Fatalf("expected external change cleared")
	}
	if change, err := e.CheckExternalChange(); err != nil || change != ExternalChangeNone {
		t.Fatalf("post-reload change=%v err=%v, want none", change, err)
	}
}

func TestReloadFromDiskRequiresForceWhenDirty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	e := newTestEditor()
	if err := e.OpenFile(path); err != nil {
		t.Fatalf("open file: %v", err)
	}
	e.replaceBuffer("changed", true)

	if err := e.ReloadFromDisk(false); err == nil {
		t.Fatalf("expected error when reloading dirty buffer without force")
	}
}

func TestMergeExternalContentConflict(t *testing.T) {
	prev := mergeWithGitFunc
	mergeWithGitFunc = func(base, local, remote string) (string, bool, error) {
		merged := strings.Join([]string{
			"alpha",
			"<<<<<<< local",
			"local",
			"=======",
			"remote",
			">>>>>>> remote",
			"charlie",
			"",
		}, "\n")
		return merged, true, nil
	}
	defer func() { mergeWithGitFunc = prev }()

	e := newTestEditor()
	e.replaceBuffer("alpha\nbravo\ncharlie\n", false)
	e.file.diskContent = e.Content()
	e.replaceBuffer("alpha\nlocal\ncharlie\n", true)

	conflict, err := e.MergeExternalContent("alpha\nremote\ncharlie\n")
	if err != nil {
		t.Fatalf("merge error: %v", err)
	}
	if !conflict {
		t.Fatalf("expected conflict")
	}
	if got := e.Content(); got != "alpha\nlocal\nremote\ncharlie\n" {
		t.Fatalf("content=%q", got)
	}
	if len(e.conflicts.blocks) != 1 {
		t.Fatalf("conflict blocks=%d, want 1", len(e.conflicts.blocks))
	}
	block := e.conflicts.blocks[0]
	if block.localStart != 1 || block.localEnd != 1 {
		t.Fatalf("local range=%d..%d, want 1..1", block.localStart, block.localEnd)
	}
	if block.remoteStart != 2 || block.remoteEnd != 2 {
		t.Fatalf("remote range=%d..%d, want 2..2", block.remoteStart, block.remoteEnd)
	}
	if kind, _ := e.conflictLineInfo(1); kind != conflictLocal {
		t.Fatalf("line1 kind=%v, want local", kind)
	}
	if kind, _ := e.conflictLineInfo(2); kind != conflictRemote {
		t.Fatalf("line2 kind=%v, want remote", kind)
	}
}

func TestMergeExternalContentNoConflict(t *testing.T) {
	prev := mergeWithGitFunc
	mergeWithGitFunc = func(base, local, remote string) (string, bool, error) {
		return "alpha\nmerged\ncharlie\n", false, nil
	}
	defer func() { mergeWithGitFunc = prev }()

	e := newTestEditor()
	e.replaceBuffer("alpha\nbravo\ncharlie\n", false)
	e.file.diskContent = e.Content()
	e.replaceBuffer("alpha\nlocal\ncharlie\n", true)

	conflict, err := e.MergeExternalContent("alpha\nremote\ncharlie\n")
	if err != nil {
		t.Fatalf("merge error: %v", err)
	}
	if conflict {
		t.Fatalf("unexpected conflict")
	}
	if got := e.Content(); got != "alpha\nmerged\ncharlie\n" {
		t.Fatalf("content=%q", got)
	}
	if len(e.conflicts.blocks) != 0 {
		t.Fatalf("expected no conflict blocks")
	}
}

func TestResolveConflictAcceptLocal(t *testing.T) {
	merged := strings.Join([]string{
		"alpha",
		"<<<<<<< local",
		"local",
		"=======",
		"remote",
		">>>>>>> remote",
		"charlie",
		"",
	}, "\n")
	cleaned, blocks := buildConflictView(merged)

	e := newTestEditor()
	e.replaceBuffer(cleaned, true)
	e.conflicts.blocks = blocks
	e.conflicts.dirty = false
	e.cursor = Cursor{Row: 1, Col: 0}

	if !e.resolveConflictAtCursor(true) {
		t.Fatalf("expected conflict resolution")
	}
	if got := e.Content(); got != "alpha\nlocal\ncharlie\n" {
		t.Fatalf("content=%q", got)
	}
	if len(e.conflicts.blocks) != 0 {
		t.Fatalf("expected conflict blocks cleared")
	}
}

func TestResolveConflictRejectLocal(t *testing.T) {
	merged := strings.Join([]string{
		"alpha",
		"<<<<<<< local",
		"local",
		"=======",
		"remote",
		">>>>>>> remote",
		"charlie",
		"",
	}, "\n")
	cleaned, blocks := buildConflictView(merged)

	e := newTestEditor()
	e.replaceBuffer(cleaned, true)
	e.conflicts.blocks = blocks
	e.conflicts.dirty = false
	e.cursor = Cursor{Row: 1, Col: 0}

	if !e.resolveConflictAtCursor(false) {
		t.Fatalf("expected conflict resolution")
	}
	if got := e.Content(); got != "alpha\nremote\ncharlie\n" {
		t.Fatalf("content=%q", got)
	}
	if len(e.conflicts.blocks) != 0 {
		t.Fatalf("expected conflict blocks cleared")
	}
}
