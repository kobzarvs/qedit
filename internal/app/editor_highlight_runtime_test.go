package app

import (
	"testing"
	"time"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

func TestSyncVisibleHighlightsClearsStaleAsyncHighlightsOnBulkEdit(t *testing.T) {
	langs := config.Languages{
		Languages: []config.Language{
			{Name: "javascript", FileTypes: []string{"js"}},
		},
	}
	ts := treesitter.New(langs)
	if err := ts.Start(); err != nil {
		t.Fatalf("start treesitter: %v", err)
	}
	defer func() { _ = ts.Stop() }()

	path := "sample.js"
	ed := editor.New(editor.Options{})
	if err := ed.LoadFileContent(path, []byte("const stale = 1;\n")); err != nil {
		t.Fatalf("load file: %v", err)
	}

	version := ts.Parse(path, "javascript", ed.Content())
	waitForHighlightSpans(t, ts, path)
	if !applyHighlightRange(ed, ts, path, 0, 0) {
		t.Fatalf("expected initial highlights")
	}
	if !ed.HasHighlights() {
		t.Fatalf("expected editor to have initial highlights")
	}

	state := newEditorRuntimeState(ed)
	state.openPath = path
	state.langName = "javascript"
	state.highlightEnabled = true
	state.highlightExpected = true
	state.highlightParseVersion = version
	state.lastChangeTick = ed.ChangeTick()
	lastTick := state.lastChangeTick
	lastStart := 0
	lastEnd := 0

	ed.ApplyFormattedContent("const fresh = 2;\n")
	lastTick, lastStart, lastEnd = syncVisibleHighlights(ed, ts, &state, path, "javascript", true, lastTick, lastStart, lastEnd)

	if ed.HasHighlights() {
		t.Fatalf("stale highlights should be cleared while async bulk edit is reparsed")
	}
	if state.highlightParseVersion <= version {
		t.Fatalf("parse version was not advanced after edit: got %d, previous %d", state.highlightParseVersion, version)
	}
	if lastStart != -1 || lastEnd != -1 {
		t.Fatalf("last highlight range = (%d,%d), want (-1,-1)", lastStart, lastEnd)
	}
	_ = lastTick
}

func waitForHighlightSpans(t *testing.T, ts *treesitter.Engine, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if spans := ts.Highlights(path, 0, 0); spans != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for highlights")
}
