package editor

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestSearchPrevKeyBinding(t *testing.T) {
	ed := newTestEditor("hello world", "hello again", "hello third")

	// Do a search first
	ed.searchQuery = []rune("hello")
	ed.lastSearchQuery = "hello"
	ed.updateSearchMatches()

	// Position cursor at line 1
	ed.cursor.Row = 1
	ed.cursor.Col = 0

	// Test keyString for Shift+N
	ev := tcell.NewEventKey(tcell.KeyRune, 'N', tcell.ModShift)
	key := keyString(ev)
	t.Logf("keyString for Shift+N: %q", key)

	if key != "N" {
		t.Errorf("keyString returned %q, want %q", key, "N")
	}

	// Check keymap
	action, ok := ed.keymap.normal[key]
	t.Logf("keymap.normal[%q] = %q (ok=%v)", key, action, ok)

	if !ok {
		t.Errorf("keymap.normal[%q] not found", key)
	}
	if action != "search_prev" {
		t.Errorf("action = %q, want %q", action, "search_prev")
	}

	// Test actual key handling
	t.Logf("Before HandleKey: row=%d col=%d", ed.cursor.Row, ed.cursor.Col)
	result := ed.HandleKey(ev)
	t.Logf("After HandleKey: row=%d col=%d, result=%v", ed.cursor.Row, ed.cursor.Col, result)

	// searchPrev should move to previous match (row 0)
	if ed.cursor.Row != 0 {
		t.Errorf("cursor.Row = %d, want 0 (previous hello)", ed.cursor.Row)
	}
}

func TestSearchPrevAfterSearchMode(t *testing.T) {
	ed := newTestEditor("hello world", "hello again", "hello third")

	// Enter search mode
	ed.mode = ModeSearch
	ed.searchQuery = []rune("hello")
	ed.updateSearchMatches()
	t.Logf("In search mode: matches=%d", len(ed.searchMatches))

	// Exit search mode with Enter (confirm)
	enterEv := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	ed.HandleKey(enterEv)
	t.Logf("After Enter: mode=%d, lastSearchQuery=%q", ed.mode, ed.lastSearchQuery)

	if ed.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal (%d)", ed.mode, ModeNormal)
	}
	if ed.lastSearchQuery != "hello" {
		t.Errorf("lastSearchQuery = %q, want %q", ed.lastSearchQuery, "hello")
	}

	// Move cursor to line 2
	ed.cursor.Row = 2
	ed.cursor.Col = 0
	t.Logf("Cursor at row=%d", ed.cursor.Row)

	// Now press Shift+N
	shiftN := tcell.NewEventKey(tcell.KeyRune, 'N', tcell.ModShift)
	ed.HandleKey(shiftN)
	t.Logf("After Shift+N: row=%d, status=%q", ed.cursor.Row, ed.statusMessage)

	// Should have moved to previous match (row 1)
	if ed.cursor.Row != 1 {
		t.Errorf("cursor.Row = %d, want 1", ed.cursor.Row)
	}
}

func TestSearchPrevWithRefsPicker(t *testing.T) {
	ed := newTestEditor("hello world", "hello again", "hello third")

	// Do a search
	ed.searchQuery = []rune("hello")
	ed.lastSearchQuery = "hello"
	ed.updateSearchMatches()

	// Activate refs picker
	ed.refsPickerActive = true
	ed.refsPickerItems = []LSPLocation{
		{Path: "test.go", StartLine: 0, StartCol: 0},
		{Path: "test.go", StartLine: 1, StartCol: 0},
	}

	// Position cursor at line 1
	ed.cursor.Row = 1
	ed.cursor.Col = 0

	// Test Shift+N with refs picker active
	ev := tcell.NewEventKey(tcell.KeyRune, 'N', tcell.ModShift)
	t.Logf("Before HandleKey (refs picker active): row=%d col=%d", ed.cursor.Row, ed.cursor.Col)
	ed.HandleKey(ev)
	t.Logf("After HandleKey: row=%d col=%d", ed.cursor.Row, ed.cursor.Col)

	// searchPrev should still work
	if ed.cursor.Row != 0 {
		t.Errorf("cursor.Row = %d, want 0 (searchPrev should work with refs picker)", ed.cursor.Row)
	}
}
