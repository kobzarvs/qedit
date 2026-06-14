package app

import (
	"testing"

	"github.com/kobzarvs/qedit/internal/editor"
)

type testAutoReloadMerger struct {
	base     string
	local    string
	remote   string
	merged   string
	conflict bool
	err      error
}

func (m *testAutoReloadMerger) Merge(base, local, remote string) (string, bool, error) {
	m.base = base
	m.local = local
	m.remote = remote
	return m.merged, m.conflict, m.err
}

func TestHandleAutoReloadResultsUsesRuntimeMerger(t *testing.T) {
	ed := editor.New(editor.Options{})
	if err := ed.LoadFileContent("/tmp/main.txt", []byte("alpha\nbravo\ncharlie\n")); err != nil {
		t.Fatalf("LoadFileContent returned error: %v", err)
	}
	ed.ApplyFormattedContent("alpha\nlocal\ncharlie\n")

	merger := &testAutoReloadMerger{merged: "alpha\nmerged\ncharlie\n"}
	monitor := &externalFileMonitor{
		ed:                ed,
		merger:            merger,
		autoReloadSeq:     1,
		autoReloadActive:  true,
		autoReloadResults: make(chan autoReloadResult, 1),
	}
	monitor.autoReloadResults <- autoReloadResult{
		seq:  1,
		data: []byte("alpha\nremote\ncharlie\n"),
	}

	monitor.HandleAutoReloadResults()

	if merger.base != "alpha\nbravo\ncharlie\n" {
		t.Fatalf("merge base = %q, want %q", merger.base, "alpha\nbravo\ncharlie\n")
	}
	if merger.local != "alpha\nlocal\ncharlie\n" {
		t.Fatalf("merge local = %q, want %q", merger.local, "alpha\nlocal\ncharlie\n")
	}
	if merger.remote != "alpha\nremote\ncharlie\n" {
		t.Fatalf("merge remote = %q, want %q", merger.remote, "alpha\nremote\ncharlie\n")
	}
	if got := ed.Content(); got != "alpha\nmerged\ncharlie\n" {
		t.Fatalf("content = %q, want %q", got, "alpha\nmerged\ncharlie\n")
	}
	if ed.ExternalChange() != editor.ExternalChangeNone {
		t.Fatalf("external change = %v, want none", ed.ExternalChange())
	}
}

func TestHandleAutoReloadResultsUsesEmptyDiskBase(t *testing.T) {
	ed := editor.New(editor.Options{})
	if err := ed.LoadFileContent("/tmp/empty.txt", nil); err != nil {
		t.Fatalf("LoadFileContent returned error: %v", err)
	}
	ed.ApplyFormattedContent("local")

	merger := &testAutoReloadMerger{merged: "merged"}
	monitor := &externalFileMonitor{
		ed:                ed,
		merger:            merger,
		autoReloadSeq:     1,
		autoReloadActive:  true,
		autoReloadResults: make(chan autoReloadResult, 1),
	}
	monitor.autoReloadResults <- autoReloadResult{
		seq:  1,
		data: []byte("remote"),
	}

	monitor.HandleAutoReloadResults()

	if merger.base != "" {
		t.Fatalf("merge base = %q, want empty", merger.base)
	}
	if merger.local != "local" {
		t.Fatalf("merge local = %q, want local", merger.local)
	}
	if merger.remote != "remote" {
		t.Fatalf("merge remote = %q, want remote", merger.remote)
	}
	if got := ed.Content(); got != "merged" {
		t.Fatalf("content = %q, want merged", got)
	}
}

func TestFileWatchEventMatchesImmutableWatchedPath(t *testing.T) {
	if !fileWatchEventMatches("/tmp/project/../project/file.txt", "/tmp/project/file.txt") {
		t.Fatalf("expected cleaned event path to match watched path")
	}
	if fileWatchEventMatches("/tmp/project/old.txt", "/tmp/project/new.txt") {
		t.Fatalf("old watcher event matched new watched path")
	}
}

func TestHandleAutoReloadResultsWithoutMergerKeepsPendingExternalChange(t *testing.T) {
	ed := editor.New(editor.Options{})
	if err := ed.LoadFileContent("/tmp/main.txt", []byte("alpha\nbravo\ncharlie\n")); err != nil {
		t.Fatalf("LoadFileContent returned error: %v", err)
	}
	ed.ApplyFormattedContent("alpha\nlocal\ncharlie\n")

	monitor := &externalFileMonitor{
		ed:                ed,
		autoReloadSeq:     1,
		autoReloadActive:  true,
		autoReloadResults: make(chan autoReloadResult, 1),
	}
	monitor.autoReloadResults <- autoReloadResult{
		seq:  1,
		data: []byte("alpha\nremote\ncharlie\n"),
	}

	monitor.HandleAutoReloadResults()

	if got := ed.Content(); got != "alpha\nlocal\ncharlie\n" {
		t.Fatalf("content = %q, want %q", got, "alpha\nlocal\ncharlie\n")
	}
	if ed.ExternalChange() != editor.ExternalChangeModified {
		t.Fatalf("external change = %v, want modified", ed.ExternalChange())
	}
}
