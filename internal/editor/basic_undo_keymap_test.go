package editor

import "testing"

func TestBasicProfileCmdZUndoRedo(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileBasic, "one")
	pressKeyScript(t, e, "a")
	if e.Content() != "aone" {
		t.Fatalf("content = %q, want %q", e.Content(), "aone")
	}
	pressKeyScript(t, e, "<cmd+z>")
	if e.Content() != "one" {
		t.Fatalf("after undo content = %q, want %q", e.Content(), "one")
	}
	pressKeyScript(t, e, "<cmd+shift+z>")
	if e.Content() != "aone" {
		t.Fatalf("after redo content = %q, want %q", e.Content(), "aone")
	}
}
