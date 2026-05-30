package editor

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func newVimWindowEditor(t *testing.T, lines ...string) *Editor {
	t.Helper()
	e := newSimulatedProfileEditor(BehaviorProfileVim, lines...)
	return e
}

func simulateEditorRender(t *testing.T, e *Editor, w, h int) {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(w, h)
	e.Render(wrapScreen(s))
}

func activeWindowLayoutForSize(t *testing.T, e *Editor, w, contentH int) editorWindowLayout {
	t.Helper()
	for _, layout := range e.windowLayouts(0, 0, w, contentH) {
		if layout.id == e.windows.activeID {
			return layout
		}
	}
	t.Fatalf("active window %d not in layouts", e.windows.activeID)
	return editorWindowLayout{}
}

func windowLeafView(t *testing.T, e *Editor, id int) editorWindowView {
	t.Helper()
	leaf := findWindowLeaf(e.windows.root, id)
	if leaf == nil {
		t.Fatalf("window leaf %d not found", id)
	}
	return leaf.view
}

func otherWindowID(t *testing.T, e *Editor, activeID int) int {
	t.Helper()
	for _, leaf := range e.windowLeaves() {
		if leaf.id != activeID {
			return leaf.id
		}
	}
	t.Fatal("no other window")
	return 0
}

func TestCtrlWArrowKeysHelixSpaceW(t *testing.T) {
	const body = "line0\nline1\nline2\n"
	e := newSimulatedProfileEditor(BehaviorProfileHelix, body)
	pressKeyScript(t, e, " <w>v")
	if e.windowCount() != 2 {
		t.Fatalf("window count = %d, want 2", e.windowCount())
	}
	right := e.windows.activeID
	pressKeyScript(t, e, " <w><left>")
	if e.windows.activeID == right {
		t.Fatal("space-w left did not change window")
	}
	pressKeyScript(t, e, " <w><right>")
	if e.windows.activeID != right {
		t.Fatalf("space-w right = %d, want %d", e.windows.activeID, right)
	}
}

func TestCtrlWCommandsVimPlaythrough(t *testing.T) {
	const body = "line0\nline1\nline2\nline3\nline4\nline5\n"

	t.Run("v current buffer vertical split", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>v")
		if got := e.windowCount(); got != 2 {
			t.Fatalf("window count = %d, want 2", got)
		}
		if e.ui.statusMessage != "vertical split" {
			t.Fatalf("status = %q", e.ui.statusMessage)
		}
	})

	t.Run("s current buffer horizontal split", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>s")
		if got := e.windowCount(); got != 2 {
			t.Fatalf("window count = %d, want 2", got)
		}
		if parent := e.activeWindowLeaf().parent; parent == nil || parent.axis != editorWindowHorizontal {
			t.Fatalf("parent axis = %#v, want horizontal", parent)
		}
	})

	t.Run("n v new empty buffer vertical", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>nv")
		if e.BufferCount() != 2 {
			t.Fatalf("buffer count = %d, want 2", e.BufferCount())
		}
		if e.Content() != "" {
			t.Fatalf("active content = %q, want empty new buffer", e.Content())
		}
	})

	t.Run("n s new empty buffer horizontal", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>ns")
		if e.BufferCount() != 2 {
			t.Fatalf("buffer count = %d, want 2", e.BufferCount())
		}
	})

	t.Run("w cycles focus", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>v")
		first := e.windows.activeID
		pressKeyScript(t, e, "<ctrl+w>w")
		if e.windows.activeID == first {
			t.Fatalf("ctrl-w w did not change active window")
		}
		pressKeyScript(t, e, "<ctrl+w>w")
		if e.windows.activeID != first {
			t.Fatalf("second ctrl-w w = %d, want %d", e.windows.activeID, first)
		}
	})

	t.Run("hjkl focus", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>v")
		right := e.windows.activeID
		pressKeyScript(t, e, "<ctrl+w>h")
		if e.windows.activeID == right {
			t.Fatal("h did not focus left")
		}
		pressKeyScript(t, e, "<ctrl+w>l")
		if e.windows.activeID != right {
			t.Fatalf("l = %d, want right %d", e.windows.activeID, right)
		}
	})

	t.Run("arrow keys focus", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>s")
		bottom := e.windows.activeID
		pressKeyScript(t, e, "<ctrl+w><up>")
		if e.windows.activeID == bottom {
			t.Fatal("up did not focus top window")
		}
		pressKeyScript(t, e, "<ctrl+w><down>")
		if e.windows.activeID != bottom {
			t.Fatalf("down = %d, want bottom %d", e.windows.activeID, bottom)
		}
	})

	t.Run("horizontal arrows on vertical split", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>v")
		right := e.windows.activeID
		pressKeyScript(t, e, "<ctrl+w><left>")
		if e.windows.activeID == right {
			t.Fatal("left did not change window")
		}
		pressKeyScript(t, e, "<ctrl+w><right>")
		if e.windows.activeID != right {
			t.Fatalf("right = %d, want %d", e.windows.activeID, right)
		}
	})

	t.Run("q close and o only", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>v")
		pressKeyScript(t, e, "<ctrl+w>q")
		if e.windowCount() != 1 {
			t.Fatalf("after q count = %d, want 1", e.windowCount())
		}
		pressKeyScript(t, e, "<ctrl+w>v<ctrl+w>o")
		if e.windowCount() != 1 {
			t.Fatalf("after o count = %d, want 1", e.windowCount())
		}
	})

	t.Run("t transpose split", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>s")
		if parent := e.activeWindowLeaf().parent; parent == nil || parent.axis != editorWindowHorizontal {
			t.Fatalf("before transpose axis = %#v", parent)
		}
		pressKeyScript(t, e, "<ctrl+w>t")
		if parent := e.activeWindowLeaf().parent; parent == nil || parent.axis != editorWindowVertical {
			t.Fatalf("after transpose axis = %#v, want vertical", parent)
		}
	})

	t.Run("H J K L swap buffers between panes", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>ns")
		if e.Content() != "" {
			t.Fatalf("new split content = %q, want empty", e.Content())
		}
		top := findWindowLeaf(e.windows.root, otherWindowID(t, e, e.windows.activeID))
		bottom := e.activeWindowLeaf()
		topBuf, bottomBuf := top.bufferIndex, bottom.bufferIndex
		pressKeyScript(t, e, "<ctrl+w>K")
		if e.ui.statusMessage != "windows swapped" {
			t.Fatalf("status after K = %q, want windows swapped", e.ui.statusMessage)
		}
		if top.bufferIndex == topBuf && bottom.bufferIndex == bottomBuf {
			t.Fatalf("buffer indices unchanged: top=%d bottom=%d", top.bufferIndex, bottom.bufferIndex)
		}
		pressKeyScript(t, e, "<ctrl+w>J")
		if e.ui.statusMessage != "windows swapped" {
			t.Fatalf("status after J = %q, want windows swapped", e.ui.statusMessage)
		}
		if top.bufferIndex != topBuf || bottom.bufferIndex != bottomBuf {
			t.Fatalf("buffer indices not restored: top=%d bottom=%d want %d/%d", top.bufferIndex, bottom.bufferIndex, topBuf, bottomBuf)
		}
	})

	t.Run("unknown key", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>?")
		if !strings.Contains(e.ui.statusMessage, "unknown window command") {
			t.Fatalf("status = %q, want unknown window command", e.ui.statusMessage)
		}
	})

	t.Run("n without v or s", func(t *testing.T) {
		e := newVimWindowEditor(t, body)
		pressKeyScript(t, e, "<ctrl+w>n?")
		if !strings.Contains(e.ui.statusMessage, "expected v or s") {
			t.Fatalf("status = %q", e.ui.statusMessage)
		}
	})
}

func TestCtrlWInactiveWindowViewStableAcrossFocusAndRender(t *testing.T) {
	const body = "L0\nL1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\n"
	e := newVimWindowEditor(t, body)

	pressKeyScript(t, e, "<ctrl+w>v")
	rightID := e.windows.activeID
	pressKeyScript(t, e, "jjj")
	e.syncActiveWindowView()
	rightAfterMove := windowLeafView(t, e, rightID)

	pressKeyScript(t, e, "<ctrl+w>h")
	leftID := e.windows.activeID
	pressKeyScript(t, e, "kk")
	e.syncActiveWindowView()
	leftView := windowLeafView(t, e, leftID)

	storedRight := windowLeafView(t, e, rightID)
	if storedRight != rightAfterMove {
		t.Fatalf("right view changed when focusing left: before=%+v after=%+v", rightAfterMove, storedRight)
	}
	if leftView.cursor.Row == storedRight.cursor.Row && leftView.scroll == storedRight.scroll {
		t.Fatalf("left and right views should differ after separate motion")
	}

	const screenW, screenH = 80, 24
	contentH := screenH - 2

	simulateEditorRender(t, e, screenW, screenH)
	storedRightAfterRender := windowLeafView(t, e, rightID)
	if storedRightAfterRender != rightAfterMove {
		t.Fatalf("right view changed after render: before=%+v after=%+v", rightAfterMove, storedRightAfterRender)
	}

	pressKeyScript(t, e, "<ctrl+w>l")
	if e.cursor.Row != storedRightAfterRender.cursor.Row {
		t.Fatalf("cursor row = %d, want %d", e.cursor.Row, storedRightAfterRender.cursor.Row)
	}
	layout := activeWindowLayoutForSize(t, e, screenW, contentH)
	if e.viewport.height != layout.h {
		t.Fatalf("viewport.height = %d, layout.h = %d, want match after focus", e.viewport.height, layout.h)
	}
	if e.viewport.width != layout.w && e.viewport.width+1 != layout.w {
		t.Fatalf("viewport.width = %d, layout.w = %d, want match after focus", e.viewport.width, layout.w)
	}

	simulateEditorRender(t, e, screenW, screenH)
	if e.viewport.height != layout.h {
		t.Fatalf("viewport.height after render = %d, want %d", e.viewport.height, layout.h)
	}
}

func TestCtrlWMovementUsesActivePaneViewportHeight(t *testing.T) {
	const body = "L0\nL1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\n"
	const screenW, screenH = 80, 24
	contentH := screenH - 2

	e := newVimWindowEditor(t, body)
	pressKeyScript(t, e, "<ctrl+w>s")
	bottomID := e.windows.activeID
	bottomLayout := activeWindowLayoutForSize(t, e, screenW, contentH)

	pressKeyScript(t, e, "<ctrl+w>k")
	topID := e.windows.activeID
	if topID == bottomID {
		t.Fatal("k did not focus top pane")
	}
	topLayout := activeWindowLayoutForSize(t, e, screenW, contentH)
	simulateEditorRender(t, e, screenW, screenH)
	if e.viewport.height != topLayout.h {
		t.Fatalf("top pane viewport.height = %d, want %d", e.viewport.height, topLayout.h)
	}

	pressKeyScript(t, e, "<ctrl+w>j")
	if e.windows.activeID != bottomID {
		t.Fatalf("active = %d, want bottom %d", e.windows.activeID, bottomID)
	}
	simulateEditorRender(t, e, screenW, screenH)
	if e.viewport.height != bottomLayout.h {
		t.Fatalf("bottom pane viewport.height = %d, want %d", e.viewport.height, bottomLayout.h)
	}
	diff := topLayout.h - bottomLayout.h
	if diff < 0 {
		diff = -diff
	}
	if diff > 1 {
		t.Fatalf("split heights = top %d bottom %d, want nearly equal halves", topLayout.h, bottomLayout.h)
	}
	if topLayout.h >= contentH {
		t.Fatalf("pane height = %d, want less than content area %d", topLayout.h, contentH)
	}
}
