package editor

import (
	"os"
	"path/filepath"
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
