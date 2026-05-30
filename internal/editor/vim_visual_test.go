package editor

import "testing"

func TestVimCharVisualSelectsInitialCharacter(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileVim, "abc")
	pressKeyScript(t, e, "v")
	if !e.profile.vim.visual {
		t.Fatal("expected char visual mode")
	}
	if !e.selectionActive {
		t.Fatal("expected active selection after v")
	}
	start, end, ok := e.selectionRange()
	if !ok {
		t.Fatal("expected non-empty selection range after v")
	}
	if start != (Cursor{Row: 0, Col: 0}) || end != (Cursor{Row: 0, Col: 1}) {
		t.Fatalf("selection = %+v..%+v, want 0..1", start, end)
	}
}

func TestVimCharVisualExtendsWithMotion(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileVim, "abc")
	pressKeyScript(t, e, "vl")
	start, end, ok := e.selectionRange()
	if !ok {
		t.Fatal("expected selection after v l")
	}
	if start != (Cursor{Row: 0, Col: 0}) || end != (Cursor{Row: 0, Col: 2}) {
		t.Fatalf("selection = %+v..%+v, want 0..2", start, end)
	}
}
