package editor

import "testing"

func TestSplitSameBufferKeepsIndependentWindowViews(t *testing.T) {
	e := newTestEditor("line1\nline2\nline3\nline4\nline5\n")
	e.SetBehaviorProfile(BehaviorProfileVim)
	e.mode = ModeNormal

	pressKeyScript(t, e, "<ctrl+w>v")
	if e.windowCount() != 2 {
		t.Fatalf("window count = %d, want 2", e.windowCount())
	}
	rightID := e.windows.activeID
	leaves := e.windowLeaves()
	var leftID int
	for _, leaf := range leaves {
		if leaf.id != rightID {
			leftID = leaf.id
			break
		}
	}
	if leftID == 0 {
		t.Fatal("missing left window id")
	}

	pressKeyScript(t, e, "jjj")
	e.syncActiveWindowView()
	rightLeaf := findWindowLeaf(e.windows.root, rightID)
	if rightLeaf == nil {
		t.Fatal("right leaf not found")
	}
	if rightLeaf.view.cursor.Row < 3 {
		t.Fatalf("right cursor row = %d, want >= 3 after jjj", rightLeaf.view.cursor.Row)
	}

	e.focusWindowByID(leftID)
	pressKeyScript(t, e, "kk")
	e.syncActiveWindowView()
	leftLeaf := findWindowLeaf(e.windows.root, leftID)
	if leftLeaf == nil {
		t.Fatal("left leaf not found")
	}
	if rightLeaf.view.cursor.Row == leftLeaf.view.cursor.Row {
		t.Fatalf("right cursor row = %d, left = %d, want independent views", rightLeaf.view.cursor.Row, leftLeaf.view.cursor.Row)
	}

	e.focusWindowByID(rightID)
	if e.cursor.Row != rightLeaf.view.cursor.Row {
		t.Fatalf("restored right cursor row = %d, want %d", e.cursor.Row, rightLeaf.view.cursor.Row)
	}
}
