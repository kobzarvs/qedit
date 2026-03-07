package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
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
	e := newTestEditor()
	e.SetMerger(testMerger{
		merged: strings.Join([]string{
			"alpha",
			"<<<<<<< local",
			"local",
			"=======",
			"remote",
			">>>>>>> remote",
			"charlie",
			"",
		}, "\n"),
		conflict: true,
	})
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
	if !e.mergeReviewActive() {
		t.Fatalf("expected merge review to become active")
	}
}

func TestMergeExternalContentNoConflict(t *testing.T) {
	e := newTestEditor()
	e.SetMerger(testMerger{merged: "alpha\nmerged\ncharlie\n"})
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

type testMerger struct {
	merged   string
	conflict bool
	err      error
}

func (m testMerger) Merge(base, local, remote string) (string, bool, error) {
	return m.merged, m.conflict, m.err
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

func TestApplyMergeReviewPaneRemote(t *testing.T) {
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
	e.mode = ModeMerge
	e.activateMergeReview()
	e.cursor = Cursor{Row: 1, Col: 0}
	e.setMergeReviewPane(mergeReviewPaneRemote)

	if !e.applySelectedMergeReviewPane() {
		t.Fatalf("expected merge review apply")
	}
	if got := e.Content(); got != "alpha\nremote\ncharlie\n" {
		t.Fatalf("content=%q", got)
	}
	if len(e.conflicts.blocks) != 0 {
		t.Fatalf("expected conflict blocks cleared")
	}
	if e.mergeReviewActive() {
		t.Fatalf("expected merge review to be inactive after resolving all conflicts")
	}
}

func TestRenderSnapshotMergeReviewShowsThreePanes(t *testing.T) {
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
	e.mode = ModeMerge
	e.activateMergeReview()
	e.cursor = Cursor{Row: 1, Col: 0}

	got := renderSnapshot(t, e, 90, 10)
	if !strings.Contains(got, "CURRENT") || !strings.Contains(got, "RESULT") || !strings.Contains(got, "LATEST") {
		t.Fatalf("snapshot does not show merge review pane headers:\n%s", got)
	}
	if strings.Count(got, "alpha") < 3 {
		t.Fatalf("snapshot does not show full-file context for alpha across panes:\n%s", got)
	}
	if strings.Count(got, "charlie") < 3 {
		t.Fatalf("snapshot does not show full-file context for charlie across panes:\n%s", got)
	}
}

func TestGotoMergeReviewConflictWraps(t *testing.T) {
	merged := strings.Join([]string{
		"alpha",
		"<<<<<<< local",
		"local-one",
		"=======",
		"remote-one",
		">>>>>>> remote",
		"middle",
		"<<<<<<< local",
		"local-two",
		"=======",
		"remote-two",
		">>>>>>> remote",
		"omega",
		"",
	}, "\n")
	cleaned, blocks := buildConflictView(merged)

	e := newTestEditor()
	e.replaceBuffer(cleaned, true)
	e.conflicts.blocks = blocks
	e.conflicts.dirty = false
	e.mode = ModeMerge
	e.activateMergeReview()
	e.cursor = Cursor{Row: blocks[0].start, Col: 0}

	e.gotoMergeReviewConflict(true)
	if e.cursor.Row != blocks[1].start {
		t.Fatalf("after next conflict cursor row=%d, want %d", e.cursor.Row, blocks[1].start)
	}

	e.gotoMergeReviewConflict(true)
	if e.cursor.Row != blocks[0].start {
		t.Fatalf("after wrap next conflict cursor row=%d, want %d", e.cursor.Row, blocks[0].start)
	}

	e.gotoMergeReviewConflict(false)
	if e.cursor.Row != blocks[1].start {
		t.Fatalf("after prev conflict wrap cursor row=%d, want %d", e.cursor.Row, blocks[1].start)
	}
}

func TestMergeReviewMouseHeaderSelectsPane(t *testing.T) {
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
	e.mode = ModeMerge
	e.activateMergeReview()

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(90, 10)
	e.Render(wrapScreen(s))

	layout := e.computeMergeReviewLayout(90)
	e.HandleMouse(wrapMouse(tcell.NewEventMouse(layout.leftX+2, 0, tcell.Button1, 0)))
	if e.conflicts.review.pane != mergeReviewPaneLocal {
		t.Fatalf("selected pane = %v, want local", e.conflicts.review.pane)
	}

	e.HandleMouse(wrapMouse(tcell.NewEventMouse(layout.rightX+2, 0, tcell.Button1, 0)))
	if e.conflicts.review.pane != mergeReviewPaneRemote {
		t.Fatalf("selected pane = %v, want remote", e.conflicts.review.pane)
	}
}

func TestMergeReviewMouseClickAppliesPaneForActiveBlock(t *testing.T) {
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
	e.mode = ModeMerge
	e.activateMergeReview()
	e.cursor = Cursor{Row: blocks[0].start, Col: 0}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(90, 10)
	e.Render(wrapScreen(s))

	layout := e.computeMergeReviewLayout(90)
	screenY := layout.headerH + (blocks[0].start - e.viewport.scroll)
	e.HandleMouse(wrapMouse(tcell.NewEventMouse(layout.rightX+2, screenY, tcell.Button1, 0)))

	if got := e.Content(); got != "alpha\nremote\ncharlie\n" {
		t.Fatalf("content=%q", got)
	}
	if e.mergeReviewActive() {
		t.Fatalf("expected merge review inactive after resolving final conflict")
	}
}

func TestMergeReviewMouseClickApplyHandleAppliesLocalPane(t *testing.T) {
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
	e.mode = ModeMerge
	e.activateMergeReview()
	e.cursor = Cursor{Row: blocks[0].start, Col: 0}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(90, 10)
	e.Render(wrapScreen(s))

	layout := e.computeMergeReviewLayout(90)
	screenY := layout.headerH + (blocks[0].start - e.viewport.scroll)
	e.HandleMouse(wrapMouse(tcell.NewEventMouse(layout.sepLeftX, screenY, tcell.Button1, 0)))

	if got := e.Content(); got != "alpha\nlocal\ncharlie\n" {
		t.Fatalf("content=%q", got)
	}
	if e.mergeReviewActive() {
		t.Fatalf("expected merge review inactive after resolving final conflict")
	}
}
