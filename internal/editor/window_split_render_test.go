package editor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
)

func newVimSplitRenderEditor(t *testing.T, body string) *Editor {
	t.Helper()
	cfg := config.Default()
	cfg.Editor.LineNumbers = "off"
	e := New(optionsFromConfig(cfg))
	e.SetFileStore(realTestFileStore{})
	e.SetMerger(noopTestMerger{})
	e.SetUndoStore(noopTestUndoStore{})
	applyTestStyles(e)
	e.text = NewTextBufferFromString(body)
	e.SetBehaviorProfile(BehaviorProfileVim)
	e.mode = ModeNormal
	return e
}

func renderToCells(t *testing.T, e *Editor, w, h int) ([]tcell.SimCell, int, int, int, int, bool) {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(w, h)
	e.Render(wrapScreen(s))
	cells, cw, ch := s.GetContents()
	cx, cy, vis := s.GetCursor()
	return cells, cw, ch, cx, cy, vis
}

func rowText(cells []tcell.SimCell, w, y, xStart, xEnd int) string {
	var b strings.Builder
	for x := xStart; x < xEnd && x < w; x++ {
		runes := cells[y*w+x].Runes
		if len(runes) == 0 {
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(runes[0])
	}
	return strings.TrimRight(b.String(), " ")
}

// TestSplitActivePaneRendersOwnScroll guards the scroll/cursor corruption bug:
// the active pane must render starting at its own per-window scroll, not the
// buffer's shared BufferState.scroll. Before the fix, restoreBufferState
// overwrote the live scroll with the buffer's stale value, so the active pane
// drew the wrong lines while the on-screen cursor used the correct scroll —
// leaving the cursor visually detached from its line.
func TestSplitActivePaneRendersOwnScroll(t *testing.T) {
	const screenW, screenH = 80, 24
	e := newVimSplitRenderEditor(t, numberedBuffer(60))

	pressKeyScript(t, e, "<ctrl+w>v")
	for i := 0; i < 20; i++ {
		pressKeyScript(t, e, "j")
	}
	simulateEditorRender(t, e, screenW, screenH)
	e.syncActiveWindowView()

	activeScroll := e.viewport.scroll
	activeRow := e.cursor.Row
	if activeScroll <= 0 {
		t.Fatalf("expected active pane to have scrolled, scroll=%d", activeScroll)
	}

	cells, w, _, cx, cy, vis := renderToCells(t, e, screenW, screenH)
	if !vis {
		t.Fatal("cursor not visible")
	}

	// Vertical split at width 80: left 0..38, separator at 39, right pane x=40.
	const rightPaneX = 40
	topLine := rowText(cells, w, 0, rightPaneX, screenW)
	wantTop := fmt.Sprintf("line%04d", activeScroll)
	if topLine != wantTop {
		t.Fatalf("active pane top line = %q, want %q (rendered at buffer-shared scroll instead of pane scroll)", topLine, wantTop)
	}

	// The on-screen cursor must sit on its own line's text.
	wantCY := activeRow - activeScroll
	if cy != wantCY {
		t.Fatalf("cursor cy = %d, want %d", cy, wantCY)
	}
	cursorCell := cells[cy*w+cx]
	if len(cursorCell.Runes) == 0 || cursorCell.Runes[0] != 'l' {
		t.Fatalf("cursor cell rune = %q, want 'l' (start of lineNNNN); cursor detached from text", cursorCell.Runes)
	}
	if cx < rightPaneX {
		t.Fatalf("cursor cx = %d, want >= right pane x %d", cx, rightPaneX)
	}
}

// TestThreePaneSwitchKeepsPerPaneScroll reproduces the reported scenario: two
// horizontal splits plus a third vertical split, all showing the same buffer.
// Cycling focus and scrolling each pane must not bleed scroll positions between
// panes, and every pane must keep rendering at its own offset.
func TestThreePaneSwitchKeepsPerPaneScroll(t *testing.T) {
	const screenW, screenH = 100, 30
	e := newVimSplitRenderEditor(t, numberedBuffer(120))

	// Build: horizontal split, then vertical split of the bottom pane.
	pressKeyScript(t, e, "<ctrl+w>s")
	pressKeyScript(t, e, "<ctrl+w>v")
	if e.windowCount() != 3 {
		t.Fatalf("window count = %d, want 3", e.windowCount())
	}
	simulateEditorRender(t, e, screenW, screenH)

	leaves := e.windowLeaves()
	ids := make([]int, 0, 3)
	for _, leaf := range leaves {
		ids = append(ids, leaf.id)
	}

	// Give each pane a distinct scroll by focusing and paging/moving.
	scrollByID := map[int]int{}
	moves := []int{2, 9, 16}
	for i, id := range ids {
		e.focusWindowByID(id)
		pressKeyScript(t, e, "gg")
		for j := 0; j < moves[i]*5; j++ {
			pressKeyScript(t, e, "j")
		}
		simulateEditorRender(t, e, screenW, screenH)
		e.syncActiveWindowView()
		scrollByID[id] = e.viewport.scroll
	}

	// Distinct scrolls expected (different move counts on same-height-ish panes).
	seen := map[int]bool{}
	for _, sc := range scrollByID {
		seen[sc] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected distinct pane scrolls, got %+v", scrollByID)
	}

	// Re-focus each pane; its restored scroll/cursor must match what we stored,
	// and the active pane must actually render starting at that scroll —
	// regardless of the order we visit them.
	for _, id := range []int{ids[1], ids[0], ids[2], ids[0]} {
		e.focusWindowByID(id)
		if e.viewport.scroll != scrollByID[id] {
			t.Fatalf("pane %d scroll after refocus = %d, want %d", id, e.viewport.scroll, scrollByID[id])
		}
		layout, ok := e.activePaneLayout()
		if !ok {
			t.Fatalf("no active pane layout for %d", id)
		}
		cells, w, _, _, _, _ := renderToCells(t, e, screenW, screenH)
		topLine := rowText(cells, w, layout.y, layout.x, layout.x+layout.w)
		wantTop := fmt.Sprintf("line%04d", scrollByID[id])
		if topLine != wantTop {
			t.Fatalf("pane %d rendered top line = %q, want %q", id, topLine, wantTop)
		}
	}

	// All other panes' stored views must be untouched after the round trip.
	for id, sc := range scrollByID {
		leaf := findWindowLeaf(e.windows.root, id)
		if leaf == nil {
			t.Fatalf("leaf %d vanished", id)
		}
		if id != e.windows.activeID && leaf.view.scroll != sc {
			t.Fatalf("inactive pane %d scroll drifted to %d, want %d", id, leaf.view.scroll, sc)
		}
	}
}

// simulateHighlightRuntime mirrors the non-huge syncVisibleHighlights decision
// for a synchronously-parsed language with no edits: it recomputes (replacing
// the span map) only when the requested highlight range actually changes,
// exactly like the app runtime. spans are stamped on every line in range so
// lineCovered reflects the requested range precisely.
type simulateHighlightRuntime struct {
	lastStart int
	lastEnd   int
}

func newSimulateHighlightRuntime() *simulateHighlightRuntime {
	return &simulateHighlightRuntime{lastStart: -1, lastEnd: -1}
}

func (r *simulateHighlightRuntime) sync(e *Editor) {
	start, end := e.HighlightVisibleRange()
	if start == r.lastStart && end == r.lastEnd {
		return
	}
	spans := make(map[int][]HighlightSpan, end-start+1)
	for ln := start; ln <= end; ln++ {
		spans[ln] = []HighlightSpan{{StartCol: 0, EndCol: 1, Kind: "keyword"}}
	}
	e.SetHighlights(start, end, spans)
	r.lastStart = start
	r.lastEnd = end
}

// TestSplitActivePaneHighlightCoverageWhileScrolling reproduces the reported
// bug: after switching to a split and scrolling it, lines that scroll into view
// lose highlighting until focus moves to another split. It drives the exact
// loop order (key -> UpdateScroll -> highlight sync -> render) and asserts the
// active pane's whole visible window stays covered the entire time.
func TestSplitActivePaneHighlightCoverageWhileScrolling(t *testing.T) {
	const screenW, screenH = 100, 30
	e := newVimSplitRenderEditor(t, numberedBuffer(300))

	pressKeyScript(t, e, "<ctrl+w>s")
	pressKeyScript(t, e, "<ctrl+w>v")
	if e.windowCount() != 3 {
		t.Fatalf("window count = %d, want 3", e.windowCount())
	}
	simulateEditorRender(t, e, screenW, screenH)

	rt := newSimulateHighlightRuntime()
	rt.sync(e)

	assertActivePaneCovered := func(step string) {
		t.Helper()
		start := e.viewport.scroll
		h := e.paneViewHeight()
		for ln := start; ln < start+h && ln < e.LineCount(); ln++ {
			if !e.highlight.lineCovered(ln) {
				t.Fatalf("%s: active pane line %d not highlighted (scroll=%d paneH=%d covered=[%d,%d])",
					step, ln, start, h, e.highlight.start, e.highlight.end)
			}
		}
	}

	// Scroll the active (bottom-right) pane downward a long way.
	for i := 0; i < 120; i++ {
		pressKeyScript(t, e, "j")
		e.UpdateScroll()
		rt.sync(e)
		simulateEditorRender(t, e, screenW, screenH)
		assertActivePaneCovered(fmt.Sprintf("after %d down", i+1))
	}

	// Now scroll back upward.
	for i := 0; i < 120; i++ {
		pressKeyScript(t, e, "k")
		e.UpdateScroll()
		rt.sync(e)
		simulateEditorRender(t, e, screenW, screenH)
		assertActivePaneCovered(fmt.Sprintf("after %d up", i+1))
	}
}

// TestSplitActivePaneHighlightCoverageBetweenBracketingPanes reproduces the
// exact screenshot layout: the focused pane sits *between* two other panes that
// pin the highlight union's min and max. Scrolling the focused pane inside that
// bracket must keep its lines highlighted even though the union bounds never
// move (so the runtime never re-requests).
func TestSplitActivePaneHighlightCoverageBetweenBracketingPanes(t *testing.T) {
	const screenW, screenH = 100, 30
	e := newVimSplitRenderEditor(t, numberedBuffer(300))

	pressKeyScript(t, e, "<ctrl+w>s")
	pressKeyScript(t, e, "<ctrl+w>v")
	if e.windowCount() != 3 {
		t.Fatalf("window count = %d, want 3", e.windowCount())
	}
	simulateEditorRender(t, e, screenW, screenH)

	leaves := e.windowLeaves()
	// Park the two non-active panes far apart so they bracket the active pane.
	activeID := e.windows.activeID
	others := make([]int, 0, 2)
	for _, leaf := range leaves {
		if leaf.id != activeID {
			others = append(others, leaf.id)
		}
	}
	// Lowest pane near top, the other near the bottom of the file.
	e.focusWindowByID(others[0])
	pressKeyScript(t, e, "gg")
	simulateEditorRender(t, e, screenW, screenH)
	e.syncActiveWindowView()

	e.focusWindowByID(others[1])
	pressKeyScript(t, e, "G")
	simulateEditorRender(t, e, screenW, screenH)
	e.syncActiveWindowView()

	// Back to the middle pane and scroll inside the bracket.
	e.focusWindowByID(activeID)
	pressKeyScript(t, e, "gg")
	for i := 0; i < 30; i++ {
		pressKeyScript(t, e, "j")
	}
	simulateEditorRender(t, e, screenW, screenH)

	rt := newSimulateHighlightRuntime()
	rt.sync(e)

	for i := 0; i < 120; i++ {
		pressKeyScript(t, e, "j")
		e.UpdateScroll()
		rt.sync(e)
		simulateEditorRender(t, e, screenW, screenH)
		start := e.viewport.scroll
		h := e.paneViewHeight()
		for ln := start; ln < start+h && ln < e.LineCount(); ln++ {
			if !e.highlight.lineCovered(ln) {
				t.Fatalf("after %d down: middle pane line %d not highlighted (scroll=%d paneH=%d covered=[%d,%d])",
					i+1, ln, start, h, e.highlight.start, e.highlight.end)
			}
		}
	}
}

func paneHasHighlightedCell(cells []tcell.SimCell, w int, layout editorWindowLayout) bool {
	for y := layout.y; y < layout.y+layout.h; y++ {
		for x := layout.x; x < layout.x+layout.w && x < w; x++ {
			fg, _, _ := cells[y*w+x].Style.Decompose()
			if fg == tcell.ColorGreen {
				return true
			}
		}
	}
	return false
}

func paneHasSelectionCell(cells []tcell.SimCell, w int, layout editorWindowLayout) bool {
	for y := layout.y; y < layout.y+layout.h; y++ {
		for x := layout.x; x < layout.x+layout.w && x < w; x++ {
			_, bg, _ := cells[y*w+x].Style.Decompose()
			if bg == tcell.ColorBlue {
				return true
			}
		}
	}
	return false
}

// TestSplitActivePaneRendersLiveHighlightNotStaleSnapshot reproduces the
// reported highlighting bug. The syntax runtime writes fresh spans only into
// the live editor highlight state each tick; the per-buffer BufferState
// snapshot is refreshed lazily (e.g. on window/buffer switch). renderWindows
// swaps in each pane's BufferState via restoreBufferState, which overwrites
// e.highlight with that stale snapshot — so panes showing the active buffer
// paint stale (missing) highlights until a focus switch refreshes the
// snapshot. The active pane must render the live spans immediately.
func TestSplitActivePaneRendersLiveHighlightNotStaleSnapshot(t *testing.T) {
	const screenW, screenH = 100, 30
	e := newVimSplitRenderEditor(t, numberedBuffer(300))

	pressKeyScript(t, e, "<ctrl+w>s")
	pressKeyScript(t, e, "<ctrl+w>v")
	if e.windowCount() != 3 {
		t.Fatalf("window count = %d, want 3", e.windowCount())
	}

	// Scroll the active pane down into lines that are not covered by whatever
	// stale snapshot the buffer holds.
	pressKeyScript(t, e, "gg")
	for i := 0; i < 40; i++ {
		pressKeyScript(t, e, "j")
	}
	simulateEditorRender(t, e, screenW, screenH)
	e.syncActiveWindowView()

	// Mimic one syntax-runtime tick: write fresh spans into the LIVE highlight
	// state for the union of visible ranges, exactly like syncVisibleHighlights
	// does. The stored BufferState highlight intentionally stays empty/stale.
	start, end := e.HighlightVisibleRange()
	spans := make(map[int][]HighlightSpan, end-start+1)
	for ln := start; ln <= end; ln++ {
		spans[ln] = []HighlightSpan{{StartCol: 0, EndCol: 8, Kind: "keyword"}}
	}
	e.SetHighlights(start, end, spans)

	layout, ok := e.activePaneLayout()
	if !ok {
		t.Fatal("no active pane layout")
	}
	cells, w, _, _, _, _ := renderToCells(t, e, screenW, screenH)
	if !paneHasHighlightedCell(cells, w, layout) {
		t.Fatalf("active pane painted no highlighted cells: rendered stale buffer snapshot instead of live spans (covered=[%d,%d] paneScroll=%d)",
			start, end, e.viewport.scroll)
	}
}

func TestSplitActivePaneRendersLiveSelectionNotStaleSnapshot(t *testing.T) {
	const screenW, screenH = 100, 30
	e := newVimSplitRenderEditor(t, numberedBuffer(20))

	pressKeyScript(t, e, "<ctrl+w>v")
	simulateEditorRender(t, e, screenW, screenH)

	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 0}
	e.selectionEnd = Cursor{Row: 0, Col: 4}
	e.cursor = Cursor{Row: 0, Col: 3}

	layout, ok := e.activePaneLayout()
	if !ok {
		t.Fatal("no active pane layout")
	}
	cells, w, _, _, _, _ := renderToCells(t, e, screenW, screenH)
	if !paneHasSelectionCell(cells, w, layout) {
		t.Fatal("active pane painted no selected cells: rendered stale buffer snapshot instead of live selection")
	}
}

// TestHighlightVisibleRangeUnionAcrossPanes ensures the syntax runtime is asked
// to highlight the union of every pane that shows the active buffer, so splits
// of the same file at different scroll offsets all keep their highlighting.
func TestHighlightVisibleRangeUnionAcrossPanes(t *testing.T) {
	const screenW, screenH = 100, 30
	e := newVimSplitRenderEditor(t, numberedBuffer(200))

	pressKeyScript(t, e, "<ctrl+w>s")
	pressKeyScript(t, e, "<ctrl+w>v")
	simulateEditorRender(t, e, screenW, screenH)

	leaves := e.windowLeaves()
	if len(leaves) != 3 {
		t.Fatalf("window count = %d, want 3", len(leaves))
	}

	// Scatter scroll offsets across panes.
	scrolls := []int{0, 60, 150}
	for i, leaf := range leaves {
		leaf.view.scroll = scrolls[i]
		leaf.view.cursor = Cursor{Row: scrolls[i]}
	}
	active := e.activeWindowLeaf()
	e.viewport.scroll = active.view.scroll
	e.cursor = active.view.cursor

	start, end := e.HighlightVisibleRange()
	if start != 0 {
		t.Fatalf("union start = %d, want 0 (top pane)", start)
	}
	// Union must reach into the lowest pane's visible window.
	if end < 150 {
		t.Fatalf("union end = %d, want >= 150 (bottom pane visible lines)", end)
	}

	// Sanity: a single window collapses to the plain visible range.
	single := newVimSplitRenderEditor(t, numberedBuffer(200))
	single.viewport.height = 20
	single.viewport.scroll = 5
	s0, s1 := single.HighlightVisibleRange()
	v0, v1 := single.VisibleRange()
	if s0 != v0 || s1 != v1 {
		t.Fatalf("single-window union = (%d,%d), want VisibleRange (%d,%d)", s0, s1, v0, v1)
	}
}
