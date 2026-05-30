package editor

import "testing"

func TestCommandRegistryFiltersByProfile(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "")
	items := e.filterCommands("")
	names := make(map[string]struct{}, len(items))
	for _, item := range items {
		names[item.Name] = struct{}{}
	}
	if _, ok := names["set"]; ok {
		t.Fatal("helix autocomplete should not include vim-only :set")
	}
	if _, ok := names["nohlsearch"]; ok {
		t.Fatal("helix autocomplete should not include vim-only :nohlsearch")
	}

	e.SetBehaviorProfile(BehaviorProfileVim)
	items = e.filterCommands("")
	names = make(map[string]struct{}, len(items))
	for _, item := range items {
		names[item.Name] = struct{}{}
	}
	if _, ok := names["set"]; !ok {
		t.Fatal("vim autocomplete should include :set")
	}
}
