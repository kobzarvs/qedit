package editor

import "testing"

func TestRegisterCommandAddsAutocompleteAndDispatch(t *testing.T) {
	e := newTestEditor("hello")
	executed := false

	e.RegisterCommand(CommandDefinition{
		Names: []string{"hello"},
		Entries: []CommandInfo{
			{Name: "hello", Description: "custom command", Group: CmdGroupView},
		},
		Handle: func(ed *Editor, args []string) bool {
			executed = true
			ed.setStatus("hello")
			return false
		},
	})

	items := e.filterCommands("hel")
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].Name != "hello" {
		t.Fatalf("item name = %q, want %q", items[0].Name, "hello")
	}

	if quit := e.execCommand("hello"); quit {
		t.Fatalf("quit = true, want false")
	}
	if !executed {
		t.Fatalf("custom command handler was not executed")
	}
}

func TestCommandRegistryKeepsBuiltInAutocomplete(t *testing.T) {
	e := newTestEditor("hello")

	items := e.filterCommands("worktree")
	if len(items) == 0 {
		t.Fatalf("expected built-in worktree commands in autocomplete")
	}

	found := false
	for _, item := range items {
		if item.Name == "worktree switch <name>" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected worktree switch autocomplete entry")
	}
}

func TestSubstituteCommandDoesNotShadowSCommands(t *testing.T) {
	e := newTestEditor("hello")

	e.execCommand("sidebar")

	if e.ui.statusMessage == "usage: s/old/new/[g]" {
		t.Fatalf("sidebar command was parsed as substitute")
	}
}
