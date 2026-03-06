package editor

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestCommandModeEditingKeys(t *testing.T) {
	e := newTestEditor("one")
	e.mode = ModeCommand
	e.commandLine.text = []rune("hello world")
	e.commandLine.cursor = 11

	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyCtrlB, 0, 0)))
	if e.commandLine.cursor != 10 {
		t.Fatalf("ctrl+b cursor = %d, want 10", e.commandLine.cursor)
	}
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyCtrlF, 0, 0)))
	if e.commandLine.cursor != 11 {
		t.Fatalf("ctrl+f cursor = %d, want 11", e.commandLine.cursor)
	}
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyHome, 0, 0)))
	if e.commandLine.cursor != 0 {
		t.Fatalf("home cursor = %d, want 0", e.commandLine.cursor)
	}
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyEnd, 0, 0)))
	if e.commandLine.cursor != len(e.commandLine.text) {
		t.Fatalf("end cursor = %d, want %d", e.commandLine.cursor, len(e.commandLine.text))
	}

	e.commandLine.cursor = 5
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyBackspace, 0, 0)))
	if string(e.commandLine.text) != "hell world" {
		t.Fatalf("backspace cmd = %q, want %q", string(e.commandLine.text), "hell world")
	}
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyDelete, 0, 0)))
	if string(e.commandLine.text) != "hellworld" {
		t.Fatalf("delete cmd = %q, want %q", string(e.commandLine.text), "hellworld")
	}

	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyCtrlU, 0, 0)))
	if len(e.commandLine.text) != 0 || e.commandLine.cursor != 0 {
		t.Fatalf("ctrl+u cmd=%q cursor=%d, want empty/0", string(e.commandLine.text), e.commandLine.cursor)
	}

	e.handleCommand(keyRune('a'))
	e.handleCommand(keyRune('b'))
	e.handleCommand(keyRune('c'))
	e.commandLine.cursor = 3
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyCtrlW, 0, 0)))
	if string(e.commandLine.text) != "" {
		t.Fatalf("ctrl+w cmd = %q, want empty", string(e.commandLine.text))
	}

	e.handleCommand(keyRune('x'))
	e.handleCommand(keyRune('y'))
	e.handleCommand(keyRune('z'))
	e.commandLine.cursor = 1
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyCtrlK, 0, 0)))
	if string(e.commandLine.text) != "x" {
		t.Fatalf("ctrl+k cmd = %q, want %q", string(e.commandLine.text), "x")
	}
}

func TestCommandModeHistoryAndExitKeys(t *testing.T) {
	e := newTestEditor("one")

	// Seed history via real command entry
	e.HandleKey(keyRune(':'))
	e.HandleKey(keyRune('l'))
	e.HandleKey(keyRune('n'))
	e.HandleKey(keyRune(' '))
	e.HandleKey(keyRune('o'))
	e.HandleKey(keyRune('f'))
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)))

	e.HandleKey(keyRune(':'))
	e.HandleKey(keyRune('l'))
	e.HandleKey(keyRune('n'))
	e.HandleKey(keyRune(' '))
	e.HandleKey(keyRune('r'))
	e.HandleKey(keyRune('e'))
	e.HandleKey(keyRune('l'))
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)))

	e.HandleKey(keyRune(':'))
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyUp, 0, 0)))
	if string(e.commandLine.text) != "ln rel" {
		t.Fatalf("up history = %q, want %q", string(e.commandLine.text), "ln rel")
	}
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyDown, 0, 0)))
	if string(e.commandLine.text) != "" {
		t.Fatalf("down history = %q, want empty", string(e.commandLine.text))
	}
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyCtrlP, 0, 0)))
	if string(e.commandLine.text) != "ln rel" {
		t.Fatalf("ctrl+p history = %q, want %q", string(e.commandLine.text), "ln rel")
	}
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyCtrlN, 0, 0)))
	if string(e.commandLine.text) != "" {
		t.Fatalf("ctrl+n history = %q, want empty", string(e.commandLine.text))
	}

	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0)))
	if e.mode != ModeNormal || len(e.commandLine.text) != 0 {
		t.Fatalf("esc exit mode=%v cmd=%q, want normal/empty", e.mode, string(e.commandLine.text))
	}

	e.HandleKey(keyRune(':'))
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0)))
	if e.mode != ModeNormal || len(e.commandLine.text) != 0 {
		t.Fatalf("ctrl+c exit mode=%v cmd=%q, want normal/empty", e.mode, string(e.commandLine.text))
	}

	e.HandleKey(keyRune(':'))
	e.HandleKey(keyRune('n'))
	e.HandleKey(keyRune('o'))
	e.HandleKey(keyRune('p'))
	e.HandleKey(keyRune('e'))
	e.handleCommand(wrapKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)))
	if e.mode != ModeNormal || len(e.commandLine.text) != 0 {
		t.Fatalf("enter exit mode=%v cmd=%q, want normal/empty", e.mode, string(e.commandLine.text))
	}
	if e.statusMessage == "" {
		t.Fatalf("expected status for unknown command")
	}
}

func TestSearchModeEditingKeys(t *testing.T) {
	e := newTestEditor("one two one")
	e.HandleKey(keyRune('/'))

	e.handleSearch(keyRune('o'))
	e.handleSearch(keyRune('n'))
	e.handleSearch(keyRune('e'))
	if string(e.searchQuery) != "one" {
		t.Fatalf("query = %q, want %q", string(e.searchQuery), "one")
	}

	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyLeft, 0, 0)))
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyCtrlB, 0, 0)))
	if e.searchCursor != 1 {
		t.Fatalf("left/ctrl+b cursor = %d, want 1", e.searchCursor)
	}
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyRight, 0, 0)))
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyCtrlF, 0, 0)))
	if e.searchCursor != 3 {
		t.Fatalf("right/ctrl+f cursor = %d, want 3", e.searchCursor)
	}

	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyHome, 0, 0)))
	if e.searchCursor != 0 {
		t.Fatalf("home cursor = %d, want 0", e.searchCursor)
	}
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyEnd, 0, 0)))
	if e.searchCursor != len(e.searchQuery) {
		t.Fatalf("end cursor = %d, want %d", e.searchCursor, len(e.searchQuery))
	}

	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyBackspace, 0, 0)))
	if string(e.searchQuery) != "on" {
		t.Fatalf("backspace query = %q, want %q", string(e.searchQuery), "on")
	}
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyDelete, 0, 0)))
	if string(e.searchQuery) != "on" {
		t.Fatalf("delete query = %q, want %q", string(e.searchQuery), "on")
	}

	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyCtrlW, 0, 0)))
	if string(e.searchQuery) != "" {
		t.Fatalf("ctrl+w query = %q, want empty", string(e.searchQuery))
	}

	e.handleSearch(keyRune('a'))
	e.handleSearch(keyRune('b'))
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyCtrlU, 0, 0)))
	if string(e.searchQuery) != "" {
		t.Fatalf("ctrl+u query = %q, want empty", string(e.searchQuery))
	}
}

func TestSearchModeHistoryAndExitKeys(t *testing.T) {
	e := newTestEditor("one two one")

	// Seed history via actual search
	e.HandleKey(keyRune('/'))
	e.handleSearch(keyRune('o'))
	e.handleSearch(keyRune('n'))
	e.handleSearch(keyRune('e'))
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)))
	if e.lastSearchQuery != "one" {
		t.Fatalf("lastSearchQuery = %q, want %q", e.lastSearchQuery, "one")
	}

	e.HandleKey(keyRune('/'))
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyUp, 0, 0)))
	if string(e.searchQuery) != "one" {
		t.Fatalf("up history = %q, want %q", string(e.searchQuery), "one")
	}
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyDown, 0, 0)))
	if string(e.searchQuery) != "" {
		t.Fatalf("down history = %q, want empty", string(e.searchQuery))
	}
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyCtrlP, 0, 0)))
	if string(e.searchQuery) != "one" {
		t.Fatalf("ctrl+p history = %q, want %q", string(e.searchQuery), "one")
	}
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyCtrlN, 0, 0)))
	if string(e.searchQuery) != "" {
		t.Fatalf("ctrl+n history = %q, want empty", string(e.searchQuery))
	}

	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0)))
	if e.mode != ModeNormal {
		t.Fatalf("esc mode = %v, want normal", e.mode)
	}

	e.HandleKey(keyRune('/'))
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0)))
	if e.mode != ModeNormal {
		t.Fatalf("ctrl+c mode = %v, want normal", e.mode)
	}
}

func TestSearchModeMetaUpDownNavigatesMatches(t *testing.T) {
	e := newTestEditor("one two one", "one")
	e.HandleKey(keyRune('/'))
	e.handleSearch(keyRune('o'))
	e.handleSearch(keyRune('n'))
	e.handleSearch(keyRune('e'))
	if len(e.searchMatches) < 2 {
		t.Fatalf("expected matches, got %d", len(e.searchMatches))
	}
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModMeta)))
	if e.cursor.Row != e.searchMatches[1].Row || e.cursor.Col != e.searchMatches[1].Col+e.searchMatches[1].Length {
		t.Fatalf("cmd+down cursor=%+v, want match1", e.cursor)
	}
	e.handleSearch(wrapKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModMeta)))
	if e.cursor.Row != e.searchMatches[0].Row || e.cursor.Col != e.searchMatches[0].Col+e.searchMatches[0].Length {
		t.Fatalf("cmd+up cursor=%+v, want match0", e.cursor)
	}
}

func TestGotoModeHotkeys(t *testing.T) {
	tests := []struct {
		key      rune
		wantRow  int
		wantCol  int
		lastCmd  string
		startRow int
		startCol int
	}{
		{'g', 0, 0, "gg", 1, 2},
		{'e', 2, 4, "ge", 1, 2},
		{'h', 1, 0, "gh", 1, 2},
		{'l', 1, 3, "gl", 1, 2},
		{'s', 0, 0, "gs", 1, 2},
	}
	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			e := newTestEditor("aa", "bbb", "cccc")
			e.cursor = Cursor{Row: tt.startRow, Col: tt.startCol}
			e.HandleKey(keyRune('g'))
			e.HandleKey(keyRune(tt.key))
			if e.cursor.Row != tt.wantRow || e.cursor.Col != tt.wantCol {
				t.Fatalf("cursor=%+v, want row=%d col=%d", e.cursor, tt.wantRow, tt.wantCol)
			}
			if e.modal.lastCommand != tt.lastCmd {
				t.Fatalf("lastCommand = %q, want %q", e.modal.lastCommand, tt.lastCmd)
			}
		})
	}
}

func TestMatchModeHotkeys(t *testing.T) {
	e := newTestEditor("a(b)c")
	e.cursor = Cursor{Row: 0, Col: 1}
	e.HandleKey(keyRune('m'))
	e.HandleKey(keyRune('m'))
	if e.cursor.Col != 3 {
		t.Fatalf("match cursor col = %d, want 3", e.cursor.Col)
	}
	if e.modal.lastCommand != "mm" {
		t.Fatalf("lastCommand = %q, want %q", e.modal.lastCommand, "mm")
	}

	cases := []struct {
		key    rune
		status string
	}{
		{'a', "select around (not implemented)"},
		{'i', "select inside (not implemented)"},
		{'s', "surround add (not implemented)"},
		{'r', "surround replace (not implemented)"},
		{'d', "surround delete (not implemented)"},
	}
	for _, tt := range cases {
		t.Run(string(tt.key), func(t *testing.T) {
			e := newTestEditor("abc")
			e.HandleKey(keyRune('m'))
			e.HandleKey(keyRune(tt.key))
			if e.statusMessage != tt.status {
				t.Fatalf("status = %q, want %q", e.statusMessage, tt.status)
			}
		})
	}
}

func TestViewModeHotkeys(t *testing.T) {
	tests := []struct {
		key    rune
		scroll int
		want   int
	}{
		{'c', 0, 1},
		{'t', 0, 3},
		{'b', 0, 0},
		{'k', 2, 1},
		{'j', 2, 3},
	}
	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			e := newTestEditor("a", "b", "c", "d", "e", "f", "g")
			e.cursor = Cursor{Row: 3, Col: 0}
			e.viewHeight = 5
			e.scroll = tt.scroll
			e.HandleKey(keyRune('z'))
			e.HandleKey(keyRune(tt.key))
			if e.scroll != tt.want {
				t.Fatalf("scroll = %d, want %d", e.scroll, tt.want)
			}
			if e.modal.lastCommand != "z"+string(tt.key) {
				t.Fatalf("lastCommand = %q, want %q", e.modal.lastCommand, "z"+string(tt.key))
			}
		})
	}
}

func TestSpaceMenuHotkeys(t *testing.T) {
	for _, item := range SpaceMenuItems {
		t.Run(string(item.Key), func(t *testing.T) {
			e := newTestEditor("line")
			e.filename = "test.go"
			e.HandleKey(keyRune(' '))
			if !e.modal.spaceMenuActive {
				t.Fatalf("spaceMenuActive = false, want true")
			}
			e.HandleKey(keyRune(item.Key))
			if e.modal.spaceMenuActive {
				t.Fatalf("spaceMenuActive = true, want false")
			}
			if item.Action != "window_mode" && e.modal.pendingKeys != "" {
				t.Fatalf("pendingKeys = %q, want empty", e.modal.pendingKeys)
			}
			wantLast := "SPC " + string(item.Key)
			if item.Action == "yank_clipboard" || item.Action == "yank_main_clipboard" {
				wantLast = "y"
			}
			if e.modal.lastCommand != wantLast {
				t.Fatalf("lastCommand = %q, want %q", e.modal.lastCommand, wantLast)
			}
			if !item.Implemented {
				want := item.Label + " (not implemented)"
				if e.statusMessage != want {
					t.Fatalf("status = %q, want %q", e.statusMessage, want)
				}
			}
			if item.Action == "window_mode" {
				if !e.modal.windowMode || e.modal.pendingKeys != "SPC w" {
					t.Fatalf("windowMode=%v pendingKeys=%q, want true/\"SPC w\"", e.modal.windowMode, e.modal.pendingKeys)
				}
			}
			if item.Action == "show_keybindings" {
				if !e.keybindingsHelp.active {
					t.Fatalf("keybindingsHelpActive = false, want true")
				}
			}
			if item.Action == "toggle_comment" {
				if string(e.line(0)) != "// line" {
					t.Fatalf("comment line = %q, want %q", string(e.line(0)), "// line")
				}
			}
		})
	}
}

func TestWindowModeHotkeys(t *testing.T) {
	e := newTestEditor("one")
	e.HandleKey(keyRune(' '))
	e.HandleKey(keyRune('w'))
	if !e.modal.windowMode {
		t.Fatalf("windowMode = false, want true")
	}
	e.HandleKey(keyRune('v'))
	if e.modal.windowMode {
		t.Fatalf("windowMode = true, want false")
	}
	if e.statusMessage != "window mode (not implemented)" {
		t.Fatalf("status = %q, want %q", e.statusMessage, "window mode (not implemented)")
	}
	if e.modal.lastCommand != "SPC wv" {
		t.Fatalf("lastCommand = %q, want %q", e.modal.lastCommand, "SPC wv")
	}
}

func TestKeybindingsHelpHotkeys(t *testing.T) {
	e := newTestEditor("one")
	e.HandleKey(keyRune(' '))
	e.HandleKey(keyRune('?'))
	if !e.keybindingsHelp.active {
		t.Fatalf("keybindingsHelpActive = false, want true")
	}
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyDown, 0, 0)))
	if e.keybindingsHelp.scroll != 1 {
		t.Fatalf("down scroll = %d, want 1", e.keybindingsHelp.scroll)
	}
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyUp, 0, 0)))
	if e.keybindingsHelp.scroll != 0 {
		t.Fatalf("up scroll = %d, want 0", e.keybindingsHelp.scroll)
	}
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyPgDn, 0, 0)))
	if e.keybindingsHelp.scroll != 10 {
		t.Fatalf("pgdn scroll = %d, want 10", e.keybindingsHelp.scroll)
	}
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyPgUp, 0, 0)))
	if e.keybindingsHelp.scroll != 0 {
		t.Fatalf("pgup scroll = %d, want 0", e.keybindingsHelp.scroll)
	}

	// Close with Escape
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0)))
	if e.keybindingsHelp.active {
		t.Fatalf("esc close = true, want false")
	}

	// Reopen and close with Enter (when filters empty)
	e.HandleKey(keyRune(' '))
	e.HandleKey(keyRune('?'))
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)))
	if e.keybindingsHelp.active {
		t.Fatalf("enter close = true, want false")
	}
}

func TestSelectModeToggleAndCollapseHotkeys(t *testing.T) {
	e := newTestEditor("abcd")

	e.HandleKey(keyRune('v'))
	if !e.modal.selectMode || !e.selectionActive {
		t.Fatalf("selectMode=%v selectionActive=%v, want true/true", e.modal.selectMode, e.selectionActive)
	}
	if e.selectionStart.Col != 0 || e.selectionEnd.Col != 0 {
		t.Fatalf("selection start/end = %v/%v, want 0/0", e.selectionStart.Col, e.selectionEnd.Col)
	}

	e.HandleKey(keyRune('l'))
	if !e.selectionActive || e.selectionEnd.Col != 1 {
		t.Fatalf("selection end = %d, want 1", e.selectionEnd.Col)
	}

	e.HandleKey(keyRune(';'))
	if e.selectionActive || e.modal.selectMode {
		t.Fatalf("selectionActive=%v selectMode=%v, want false/false", e.selectionActive, e.modal.selectMode)
	}
}

func TestHelixSelectingMotionHotkeys(t *testing.T) {
	e := newTestEditor("foo bar")
	e.HandleKey(keyRune('w'))
	if e.cursor.Col != 4 {
		t.Fatalf("cursor col = %d, want 4", e.cursor.Col)
	}
	if !e.selectionActive || !e.modal.selectMode {
		t.Fatalf("selectionActive=%v selectMode=%v, want true/true", e.selectionActive, e.modal.selectMode)
	}
	if e.selectionStart.Col != 0 || e.selectionEnd.Col != 4 {
		t.Fatalf("selection = %d..%d, want 0..4", e.selectionStart.Col, e.selectionEnd.Col)
	}
}

func TestFindCharHotkeyChainCreatesSelection(t *testing.T) {
	e := newTestEditor("abcde")
	e.HandleKey(keyRune('f'))
	e.HandleKey(keyRune('d'))
	if e.cursor.Col != 3 {
		t.Fatalf("cursor col = %d, want 3", e.cursor.Col)
	}
	if !e.selectionActive || !e.modal.selectMode {
		t.Fatalf("selectionActive=%v selectMode=%v, want true/true", e.selectionActive, e.modal.selectMode)
	}
	if e.selectionStart.Col != 0 || e.selectionEnd.Col != 4 {
		t.Fatalf("selection = %d..%d, want 0..4", e.selectionStart.Col, e.selectionEnd.Col)
	}
}

func TestReplaceCharHotkeyChain(t *testing.T) {
	e := newTestEditor("abc")
	e.HandleKey(keyRune('r'))
	e.HandleKey(keyRune('z'))
	if string(e.line(0)) != "zbc" {
		t.Fatalf("line = %q, want %q", string(e.line(0)), "zbc")
	}
}

func TestChangeHotkeyChainEntersInsert(t *testing.T) {
	e := newTestEditor("abc")
	e.HandleKey(keyRune('v'))
	e.HandleKey(keyRune('l'))
	e.HandleKey(keyRune('c'))
	if e.mode != ModeInsert {
		t.Fatalf("mode = %v, want insert", e.mode)
	}
	if e.selectionActive || e.modal.selectMode {
		t.Fatalf("selectionActive=%v selectMode=%v, want false/false", e.selectionActive, e.modal.selectMode)
	}
	if e.Content() != "bc" {
		t.Fatalf("content = %q, want %q", e.Content(), "bc")
	}
}

func TestGotoLinePromptHotkey(t *testing.T) {
	e := newTestEditor("a", "b")
	e.HandleKey(eventForKeyString(t, "cmd+g"))
	if e.mode != ModeCommand {
		t.Fatalf("mode = %v, want command", e.mode)
	}
	if e.statusMessage != "goto line:" {
		t.Fatalf("status = %q, want %q", e.statusMessage, "goto line:")
	}
	if len(e.commandLine.text) != 0 || e.commandLine.cursor != 0 {
		t.Fatalf("cmd=%q cursor=%d, want empty/0", string(e.commandLine.text), e.commandLine.cursor)
	}
}
