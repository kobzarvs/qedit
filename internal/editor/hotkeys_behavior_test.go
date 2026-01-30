package editor

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestSearchEntryHotkeys(t *testing.T) {
	cases := []struct {
		key     string
		forward bool
		fuzzy   bool
		regex   bool
	}{
		{"/", true, false, false},
		{"?", false, false, false},
		{"cmd+f", true, true, false},
		{"cmd+e", true, false, true},
	}
	for _, tt := range cases {
		t.Run(tt.key, func(t *testing.T) {
			e := newTestEditor("one")
			e.HandleKey(eventForKeyString(t, tt.key))
			if e.mode != ModeSearch {
				t.Fatalf("mode = %v, want search", e.mode)
			}
			if e.searchForward != tt.forward || e.searchFuzzy != tt.fuzzy || e.searchRegex != tt.regex {
				t.Fatalf("flags forward=%v fuzzy=%v regex=%v, want %v/%v/%v",
					e.searchForward, e.searchFuzzy, e.searchRegex, tt.forward, tt.fuzzy, tt.regex)
			}
		})
	}
}

func TestSearchNextPrevHotkeys(t *testing.T) {
	e := newTestEditor("one two one")
	e.HandleKey(keyRune('/'))
	e.handleSearch(keyRune('o'))
	e.handleSearch(keyRune('n'))
	e.handleSearch(keyRune('e'))
	e.handleSearch(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if len(e.searchMatches) < 2 {
		t.Fatalf("expected matches, got %d", len(e.searchMatches))
	}
	first := e.searchMatches[0]
	second := e.searchMatches[1]

	e.HandleKey(keyRune('n'))
	if e.cursor.Row != second.Row || e.cursor.Col != second.Col+second.Length {
		t.Fatalf("n cursor=%+v, want second match end", e.cursor)
	}

	// Move cursor to start of second match so prev goes to first
	e.cursor = Cursor{Row: second.Row, Col: second.Col}
	e.HandleKey(keyRune('N'))
	if e.cursor.Row != first.Row || e.cursor.Col != first.Col+first.Length {
		t.Fatalf("N cursor=%+v, want first match end", e.cursor)
	}
}

func TestBranchPickerHotkey(t *testing.T) {
	e := newTestEditor("one")
	e.HandleKey(eventForKeyString(t, "cmd+b"))
	if !e.ConsumeBranchPickerRequest() {
		t.Fatalf("expected branch picker request")
	}
	if e.ConsumeBranchPickerRequest() {
		t.Fatalf("expected request to be consumed")
	}
}

func TestToggleLineNumbersHotkey(t *testing.T) {
	e := newTestEditor("one")
	if e.lineNumberMode != LineNumberAbsolute {
		t.Fatalf("default lineNumberMode = %v, want absolute", e.lineNumberMode)
	}
	e.HandleKey(eventForKeyString(t, "cmd+l"))
	if e.lineNumberMode != LineNumberRelative {
		t.Fatalf("lineNumberMode = %v, want relative", e.lineNumberMode)
	}
	e.HandleKey(eventForKeyString(t, "cmd+l"))
	if e.lineNumberMode != LineNumberAbsolute {
		t.Fatalf("lineNumberMode = %v, want absolute", e.lineNumberMode)
	}
}

func TestDeleteLineHotkey(t *testing.T) {
	e := newTestEditor("one", "two", "three")
	e.cursor = Cursor{Row: 1, Col: 0}
	e.HandleKey(eventForKeyString(t, "cmd+y"))
	if len(e.lines) != 2 {
		t.Fatalf("lines len = %d, want 2", len(e.lines))
	}
	if string(e.lines[1]) != "three" {
		t.Fatalf("line1 = %q, want %q", string(e.lines[1]), "three")
	}
}

func TestDeleteCharHotkey(t *testing.T) {
	e := newTestEditor("abc")
	e.cursor = Cursor{Row: 0, Col: 1}
	e.HandleKey(eventForKeyString(t, "del"))
	if e.Content() != "ac" {
		t.Fatalf("content = %q, want %q", e.Content(), "ac")
	}
}

func TestDeleteWordLeftHotkey(t *testing.T) {
	e := newTestEditor("foo bar")
	e.cursor = Cursor{Row: 0, Col: len(e.lines[0])}
	e.HandleKey(eventForKeyString(t, "cmd+backspace"))
	if e.Content() != "foo " {
		t.Fatalf("content = %q, want %q", e.Content(), "foo ")
	}
}

func TestDeleteWordRightHotkey(t *testing.T) {
	t.Run("delete word at start", func(t *testing.T) {
		e := newTestEditor("foo bar")
		e.cursor = Cursor{Row: 0, Col: 0}
		e.HandleKey(eventForKeyString(t, "cmd+del"))
		if e.Content() != "bar" {
			t.Fatalf("content = %q, want %q", e.Content(), "bar")
		}
	})

	t.Run("delete word in middle", func(t *testing.T) {
		e := newTestEditor("foo bar baz")
		e.cursor = Cursor{Row: 0, Col: 4}
		e.HandleKey(eventForKeyString(t, "cmd+del"))
		if e.Content() != "foo baz" {
			t.Fatalf("content = %q, want %q", e.Content(), "foo baz")
		}
	})

	t.Run("delete at end of line joins next", func(t *testing.T) {
		e := newTestEditor("foo\nbar")
		e.cursor = Cursor{Row: 0, Col: 3}
		e.HandleKey(eventForKeyString(t, "cmd+del"))
		if e.Content() != "foobar" {
			t.Fatalf("content = %q, want %q", e.Content(), "foobar")
		}
	})
}

func TestDeleteWordRightUndo(t *testing.T) {
	t.Run("undo single word delete", func(t *testing.T) {
		e := newTestEditor("foo bar")
		e.cursor = Cursor{Row: 0, Col: 0}
		e.HandleKey(eventForKeyString(t, "cmd+del"))
		if e.Content() != "bar" {
			t.Fatalf("after delete: content = %q, want %q", e.Content(), "bar")
		}
		e.HandleKey(keyRune('u'))
		if e.Content() != "foo bar" {
			t.Fatalf("after undo: content = %q, want %q", e.Content(), "foo bar")
		}
	})

	t.Run("undo multiple word deletes", func(t *testing.T) {
		e := newTestEditor("one two three")
		e.cursor = Cursor{Row: 0, Col: 0}
		e.HandleKey(eventForKeyString(t, "cmd+del"))
		if e.Content() != "two three" {
			t.Fatalf("after first delete: content = %q, want %q", e.Content(), "two three")
		}
		e.HandleKey(eventForKeyString(t, "cmd+del"))
		if e.Content() != "three" {
			t.Fatalf("after second delete: content = %q, want %q", e.Content(), "three")
		}
		e.HandleKey(keyRune('u'))
		if e.Content() != "two three" {
			t.Fatalf("after first undo: content = %q, want %q", e.Content(), "two three")
		}
		e.HandleKey(keyRune('u'))
		if e.Content() != "one two three" {
			t.Fatalf("after second undo: content = %q, want %q", e.Content(), "one two three")
		}
	})

	t.Run("undo line join", func(t *testing.T) {
		e := newTestEditor("foo\nbar")
		e.cursor = Cursor{Row: 0, Col: 3}
		e.HandleKey(eventForKeyString(t, "cmd+del"))
		if e.Content() != "foobar" {
			t.Fatalf("after delete: content = %q, want %q", e.Content(), "foobar")
		}
		e.HandleKey(keyRune('u'))
		if e.Content() != "foo\nbar" {
			t.Fatalf("after undo: content = %q, want %q", e.Content(), "foo\nbar")
		}
	})
}

func TestDeleteWordLeftUndo(t *testing.T) {
	t.Run("undo single word delete", func(t *testing.T) {
		e := newTestEditor("foo bar")
		e.cursor = Cursor{Row: 0, Col: len(e.lines[0])}
		e.HandleKey(eventForKeyString(t, "cmd+backspace"))
		if e.Content() != "foo " {
			t.Fatalf("after delete: content = %q, want %q", e.Content(), "foo ")
		}
		e.HandleKey(keyRune('u'))
		if e.Content() != "foo bar" {
			t.Fatalf("after undo: content = %q, want %q", e.Content(), "foo bar")
		}
	})
}

func TestOpenBelowAboveHotkeys(t *testing.T) {
	t.Run("open below", func(t *testing.T) {
		e := newTestEditor("one")
		e.HandleKey(keyRune('o'))
		if e.mode != ModeInsert {
			t.Fatalf("mode = %v, want insert", e.mode)
		}
		if len(e.lines) != 2 || string(e.lines[1]) != "" {
			t.Fatalf("lines = %q, want [\"one\" \"\"]", e.Content())
		}
		if e.cursor.Row != 1 {
			t.Fatalf("cursor row = %d, want 1", e.cursor.Row)
		}
	})
	t.Run("open above", func(t *testing.T) {
		e := newTestEditor("one")
		e.HandleKey(keyRune('O'))
		if e.mode != ModeInsert {
			t.Fatalf("mode = %v, want insert", e.mode)
		}
		if len(e.lines) != 2 || string(e.lines[0]) != "" {
			t.Fatalf("lines = %q, want [\"\" \"one\"]", e.Content())
		}
		if e.cursor.Row != 0 {
			t.Fatalf("cursor row = %d, want 0", e.cursor.Row)
		}
	})
}

func TestAppendAndInsertHotkeys(t *testing.T) {
	e := newTestEditor("abc")
	e.HandleKey(keyRune('a'))
	if e.mode != ModeInsert || e.cursor.Col != 1 {
		t.Fatalf("a mode=%v col=%d, want insert/1", e.mode, e.cursor.Col)
	}

	e = newTestEditor("abc")
	e.HandleKey(keyRune('A'))
	if e.mode != ModeInsert || e.cursor.Col != 3 {
		t.Fatalf("A mode=%v col=%d, want insert/3", e.mode, e.cursor.Col)
	}

	e = newTestEditor("  abc")
	e.HandleKey(keyRune('I'))
	if e.mode != ModeInsert || e.cursor.Col != 2 {
		t.Fatalf("I mode=%v col=%d, want insert/2", e.mode, e.cursor.Col)
	}
}

func TestJoinLinesHotkey(t *testing.T) {
	e := newTestEditor("hello", "world")
	e.HandleKey(keyRune('J'))
	if len(e.lines) != 1 {
		t.Fatalf("lines len = %d, want 1", len(e.lines))
	}
	if e.Content() != "hello world" {
		t.Fatalf("content = %q, want %q", e.Content(), "hello world")
	}
}

func TestYankPasteHotkeys(t *testing.T) {
	t.Run("yank selection", func(t *testing.T) {
		e := newTestEditor("abc")
		e.HandleKey(keyRune('v'))
		e.HandleKey(keyRune('l'))
		e.HandleKey(keyRune('y'))
		if len(e.clipboard) != 1 || string(e.clipboard[0]) != "a" {
			t.Fatalf("clipboard = %#v, want [\"a\"]", e.clipboard)
		}
		if e.selectionActive || e.selectMode {
			t.Fatalf("selectionActive=%v selectMode=%v, want false/false", e.selectionActive, e.selectMode)
		}
	})
	t.Run("paste after", func(t *testing.T) {
		e := newTestEditor("abc")
		e.clipboard = [][]rune{[]rune("X")}
		e.HandleKey(keyRune('p'))
		if e.Content() != "aXbc" {
			t.Fatalf("content = %q, want %q", e.Content(), "aXbc")
		}
	})
	t.Run("paste before", func(t *testing.T) {
		e := newTestEditor("abc")
		e.cursor = Cursor{Row: 0, Col: 1}
		e.clipboard = [][]rune{[]rune("Y")}
		e.HandleKey(keyRune('P'))
		if e.Content() != "aYbc" {
			t.Fatalf("content = %q, want %q", e.Content(), "aYbc")
		}
	})
}

func TestInsertLineBelowHotkeyInsertMode(t *testing.T) {
	e := newTestEditor("one")
	e.mode = ModeInsert
	e.HandleKey(eventForKeyString(t, "cmd+enter"))
	if len(e.lines) != 2 || string(e.lines[1]) != "" {
		t.Fatalf("lines = %q, want [\"one\" \"\"]", e.Content())
	}
	if e.cursor.Row != 1 || e.mode != ModeInsert {
		t.Fatalf("cursor row=%d mode=%v, want 1/insert", e.cursor.Row, e.mode)
	}
}

func TestShiftEnterInsertLineAboveHotkey(t *testing.T) {
	e := newTestEditor("one")
	e.HandleKey(eventForKeyString(t, "shift+enter"))
	if len(e.lines) != 2 || string(e.lines[0]) != "" {
		t.Fatalf("lines = %q, want [\"\" \"one\"]", e.Content())
	}
	if e.cursor.Row != 0 {
		t.Fatalf("cursor row = %d, want 0", e.cursor.Row)
	}
}

func TestSelectAllHotkeys(t *testing.T) {
	e := newTestEditor("a", "b")
	e.HandleKey(keyRune('%'))
	if !e.selectionActive {
		t.Fatalf("selectionActive = false, want true")
	}
	if e.selectionStart != (Cursor{Row: 0, Col: 0}) {
		t.Fatalf("selectionStart = %+v, want 0,0", e.selectionStart)
	}
	if e.selectionEnd != (Cursor{Row: 1, Col: 1}) {
		t.Fatalf("selectionEnd = %+v, want 1,1", e.selectionEnd)
	}
}

func TestExpandShrinkSelectionHotkeys(t *testing.T) {
	e := newTestEditor("abcd")
	e.filename = "test.go"
	e.nodeStackFunc = func(path string, row, col int) []NodeRange {
		return []NodeRange{
			{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 1},
			{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3},
		}
	}
	e.HandleKey(eventForKeyString(t, "alt+shift+up"))
	if !e.selectionActive || e.selectionEnd.Col != 1 {
		t.Fatalf("expand1 selectionEnd = %d, want 1", e.selectionEnd.Col)
	}
	e.HandleKey(eventForKeyString(t, "alt+shift+up"))
	if e.selectionEnd.Col != 3 {
		t.Fatalf("expand2 selectionEnd = %d, want 3", e.selectionEnd.Col)
	}
	e.HandleKey(eventForKeyString(t, "alt+shift+down"))
	if e.selectionEnd.Col != 1 {
		t.Fatalf("shrink selectionEnd = %d, want 1", e.selectionEnd.Col)
	}
}

func TestSaveHotkeyNoFilename(t *testing.T) {
	e := newTestEditor("one")
	e.HandleKey(eventForKeyString(t, "cmd+s"))
	if e.statusMessage != "no file name" {
		t.Fatalf("status = %q, want %q", e.statusMessage, "no file name")
	}
}
