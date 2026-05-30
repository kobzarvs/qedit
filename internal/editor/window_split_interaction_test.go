package editor

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func numberedBuffer(lines int) string {
	var b strings.Builder
	b.Grow(lines * 10)
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&b, "line%04d\n", i)
	}
	return b.String()
}

func assertWindowViewEquals(t *testing.T, e *Editor, windowID int, want editorWindowView, label string) {
	t.Helper()
	got := windowLeafView(t, e, windowID)
	if got != want {
		t.Fatalf("%s: window %d view = %+v, want %+v", label, windowID, got, want)
	}
}

func TestSplitPgUpPgDownFocusRoundTrip(t *testing.T) {
	const (
		screenW, screenH = 80, 24
		lineCount        = 80
	)
	body := numberedBuffer(lineCount)
	contentH := screenH - 2

	e := newVimWindowEditor(t, body)
	pressKeyScript(t, e, "<ctrl+w>v")
	rightID := e.windows.activeID
	leftID := otherWindowID(t, e, rightID)

	simulateEditorRender(t, e, screenW, screenH)
	rightLayout := activeWindowLayoutForSize(t, e, screenW, contentH)

	for i := 0; i < 4; i++ {
		pressKeyScript(t, e, "<pgdn>")
	}
	e.syncActiveWindowView()
	rightAfterPages := windowLeafView(t, e, rightID)
	if rightAfterPages.cursor.Row < rightLayout.h {
		t.Fatalf("after pgdn×4 right cursor row = %d, want at least pane height %d", rightAfterPages.cursor.Row, rightLayout.h)
	}

	pressKeyScript(t, e, "<ctrl+w>h")
	if e.windows.activeID == rightID {
		t.Fatalf("ctrl+w h did not leave right pane (status %q)", e.ui.statusMessage)
	}
	assertWindowViewEquals(t, e, rightID, rightAfterPages, "right immediately after focus left")
	simulateEditorRender(t, e, screenW, screenH)
	assertWindowViewEquals(t, e, rightID, rightAfterPages, "right after focus left and render")

	for i := 0; i < 3; i++ {
		pressKeyScript(t, e, "<pgup>")
	}
	for i := 0; i < 3; i++ {
		pressKeyScript(t, e, "<pgdn>")
	}
	assertWindowViewEquals(t, e, rightID, rightAfterPages, "right after pgup/pgdn in left pane")

	pressKeyScript(t, e, "<ctrl+w>l")
	simulateEditorRender(t, e, screenW, screenH)
	if e.cursor != rightAfterPages.cursor {
		t.Fatalf("restored cursor = %+v, want %+v", e.cursor, rightAfterPages.cursor)
	}
	if e.viewport.scroll != rightAfterPages.scroll {
		t.Fatalf("restored scroll = %d, want %d", e.viewport.scroll, rightAfterPages.scroll)
	}
	if e.viewport.height != rightLayout.h {
		t.Fatalf("viewport.height = %d, want pane %d", e.viewport.height, rightLayout.h)
	}

	for i := 0; i < 2; i++ {
		pressKeyScript(t, e, "<pgup>")
	}
	e.syncActiveWindowView()
	rightMid := windowLeafView(t, e, rightID)

	pressKeyScript(t, e, "<ctrl+w>h")
	assertWindowViewEquals(t, e, rightID, rightMid, "right after second focus left")

	pressKeyScript(t, e, "<ctrl+w>l")
	if e.cursor != rightMid.cursor || e.viewport.scroll != rightMid.scroll {
		t.Fatalf("right mid lost: cursor=%+v scroll=%d want %+v scroll=%d",
			e.cursor, e.viewport.scroll, rightMid.cursor, rightMid.scroll)
	}
	_ = leftID
}

func TestSplitArrowsToEdgesPreservesOtherPane(t *testing.T) {
	const (
		screenW, screenH = 80, 24
		lineCount        = 60
	)
	body := numberedBuffer(lineCount)

	e := newVimWindowEditor(t, body)
	pressKeyScript(t, e, "<ctrl+w>s")
	bottomID := e.windows.activeID
	topID := otherWindowID(t, e, bottomID)

	simulateEditorRender(t, e, screenW, screenH)

	for i := 0; i < 5; i++ {
		pressKeyScript(t, e, "<pgdn>")
	}
	e.syncActiveWindowView()
	bottomAfterPg := windowLeafView(t, e, bottomID)

	pressKeyScript(t, e, "<ctrl+w>k")
	simulateEditorRender(t, e, screenW, screenH)
	assertWindowViewEquals(t, e, bottomID, bottomAfterPg, "bottom after focus top")

	for i := 0; i < lineCount+5; i++ {
		pressKeyScript(t, e, "k")
	}
	pressKeyScript(t, e, "gg")
	topAtStart := windowLeafView(t, e, topID)
	if topAtStart.cursor.Row != 0 {
		t.Fatalf("top at start row = %d, want 0", topAtStart.cursor.Row)
	}
	assertWindowViewEquals(t, e, bottomID, bottomAfterPg, "bottom after top gg")

	for i := 0; i < lineCount+5; i++ {
		pressKeyScript(t, e, "j")
	}
	pressKeyScript(t, e, "G")
	topAtEnd := windowLeafView(t, e, topID)
	lastRow := e.LineCount() - 1
	if topAtEnd.cursor.Row != lastRow {
		t.Fatalf("top at end row = %d, want %d (LineCount=%d)", topAtEnd.cursor.Row, lastRow, e.LineCount())
	}
	assertWindowViewEquals(t, e, bottomID, bottomAfterPg, "bottom after top G")

	for i := 0; i < 80; i++ {
		pressKeyScript(t, e, "h")
	}
	for i := 0; i < 80; i++ {
		pressKeyScript(t, e, "l")
	}
	assertWindowViewEquals(t, e, bottomID, bottomAfterPg, "bottom after top h/l sweep")

	pressKeyScript(t, e, "<ctrl+w>j")
	simulateEditorRender(t, e, screenW, screenH)
	if e.cursor != bottomAfterPg.cursor {
		t.Fatalf("bottom cursor = %+v, want %+v", e.cursor, bottomAfterPg.cursor)
	}
	if e.viewport.scroll != bottomAfterPg.scroll {
		t.Fatalf("bottom scroll = %d, want %d", e.viewport.scroll, bottomAfterPg.scroll)
	}
	_ = topAtStart
}

func TestSplitPageDownStepUsesPaneHeightNotFullScreen(t *testing.T) {
	const (
		screenW, screenH = 80, 24
		lineCount        = 100
	)
	body := numberedBuffer(lineCount)
	contentH := screenH - 2

	e := newVimWindowEditor(t, body)
	pressKeyScript(t, e, "<ctrl+w>s")
	simulateEditorRender(t, e, screenW, screenH)

	paneH := activeWindowLayoutForSize(t, e, screenW, contentH).h
	if paneH >= contentH {
		t.Fatalf("pane height %d should be less than content %d in horizontal split", paneH, contentH)
	}

	startRow := e.cursor.Row
	pressKeyScript(t, e, "<pgdn>")
	if got := e.cursor.Row - startRow; got != paneH && got != paneH-1 && got != paneH+1 {
		t.Fatalf("single pgdn moved %d lines, pane height %d (full screen would be ~%d)",
			got, paneH, contentH)
	}
}

func TestSplitBufferSwitchUsesTargetBufferView(t *testing.T) {
	dir := t.TempDir()
	e := newVimWindowEditor(t, "")
	alpha := filepath.Join(dir, "alpha.txt")
	beta := filepath.Join(dir, "beta.txt")
	if err := e.LoadFileContent(alpha, []byte(numberedBuffer(80))); err != nil {
		t.Fatalf("LoadFileContent(alpha): %v", err)
	}
	if err := e.LoadFileContent(beta, []byte(numberedBuffer(80))); err != nil {
		t.Fatalf("LoadFileContent(beta): %v", err)
	}

	e.cursor = Cursor{Row: 30, Col: 0}
	e.viewport.scroll = 25
	e.syncActiveBufferState()
	e.switchToBuffer(0)

	e.cursor = Cursor{Row: 2, Col: 0}
	e.viewport.scroll = 0
	pressKeyScript(t, e, "<ctrl+w>v")

	pressKeyScript(t, e, ":b 2<enter>")
	if got := e.ActiveBufferIndex(); got != 1 {
		t.Fatalf("active buffer = %d, want beta index 1", got)
	}
	if e.cursor.Row != 30 {
		t.Fatalf("cursor row after split :b = %d, want target buffer row 30", e.cursor.Row)
	}
	if e.viewport.scroll != 25 {
		t.Fatalf("scroll after split :b = %d, want target buffer scroll 25", e.viewport.scroll)
	}
	if got := windowLeafView(t, e, e.windows.activeID); got.cursor.Row != 30 || got.scroll != 25 {
		t.Fatalf("active leaf view after split :b = %+v, want target buffer cursor row 30 scroll 25", got)
	}
}
