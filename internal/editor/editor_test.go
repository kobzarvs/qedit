package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
)

func newTestEditor(lines ...string) *Editor {
	if len(lines) == 0 {
		lines = []string{""}
	}
	e := New(config.Default())
	e.lines = make([][]rune, len(lines))
	for i, line := range lines {
		e.lines[i] = []rune(line)
	}
	return e
}

func TestVisualColWithTabs(t *testing.T) {
	line := []rune("a\tb")
	if got := visualCol(line, 0, 4); got != 0 {
		t.Fatalf("col0 = %d, want 0", got)
	}
	if got := visualCol(line, 1, 4); got != 1 {
		t.Fatalf("col1 = %d, want 1", got)
	}
	if got := visualCol(line, 2, 4); got != 4 {
		t.Fatalf("col2 = %d, want 4", got)
	}
	if got := visualCol(line, 3, 4); got != 5 {
		t.Fatalf("col3 = %d, want 5", got)
	}
}

func TestMoveWordLeftRight(t *testing.T) {
	e := newTestEditor("foo  bar_baz;qux")
	e.cursor = Cursor{Row: 0, Col: len(e.lines[0])}
	e.moveWordLeft()
	if e.cursor.Col != 13 {
		t.Fatalf("word left col = %d, want 13", e.cursor.Col)
	}
	e.moveWordLeft()
	if e.cursor.Col != 12 {
		t.Fatalf("word left col = %d, want 12", e.cursor.Col)
	}

	e.cursor = Cursor{Row: 0, Col: 0}
	e.moveWordRight()
	if e.cursor.Col != 5 {
		t.Fatalf("word right col = %d, want 5", e.cursor.Col)
	}
	e.moveWordRight()
	if e.cursor.Col != 12 {
		t.Fatalf("word right col = %d, want 12", e.cursor.Col)
	}
}

func TestMoveLineUpDownUndo(t *testing.T) {
	e := newTestEditor("one", "two", "three")
	e.cursor = Cursor{Row: 1, Col: 0}
	e.moveLineUp()
	if got := string(e.lines[0]); got != "two" {
		t.Fatalf("line0 = %q, want %q", got, "two")
	}
	if e.cursor.Row != 0 {
		t.Fatalf("cursor row = %d, want 0", e.cursor.Row)
	}
	e.Undo()
	if got := string(e.lines[0]); got != "one" {
		t.Fatalf("undo line0 = %q, want %q", got, "one")
	}
	e.Redo()
	if got := string(e.lines[0]); got != "two" {
		t.Fatalf("redo line0 = %q, want %q", got, "two")
	}
}

func TestSelectionRangeForLine(t *testing.T) {
	e := newTestEditor("abc", "defg", "hi")
	e.selectionActive = true
	e.selectionStart = Cursor{Row: 1, Col: 2}
	e.selectionEnd = Cursor{Row: 0, Col: 1}

	start, end, ok := e.selectionRangeForLine(0)
	if !ok || start != 1 || end != 3 {
		t.Fatalf("line0 range = %d..%d ok=%v, want 1..3 true", start, end, ok)
	}
	start, end, ok = e.selectionRangeForLine(1)
	if !ok || start != 0 || end != 2 {
		t.Fatalf("line1 range = %d..%d ok=%v, want 0..2 true", start, end, ok)
	}
	_, _, ok = e.selectionRangeForLine(2)
	if ok {
		t.Fatalf("line2 ok = true, want false")
	}
}

func TestSelectionMoveWithShiftMeta(t *testing.T) {
	e := newTestEditor("foo  bar")
	e.cursor = Cursor{Row: 0, Col: len(e.lines[0])}
	ev := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModMeta|tcell.ModShift)
	if !e.handleSelectionMove(ev) {
		t.Fatalf("handleSelectionMove returned false")
	}
	if !e.selectionActive {
		t.Fatalf("selectionActive = false, want true")
	}
	if e.cursor.Col != 5 {
		t.Fatalf("cursor col = %d, want 5", e.cursor.Col)
	}
}

func TestSelectionMoveWithShiftPgUp(t *testing.T) {
	e := newTestEditor("0", "1", "2", "3", "4", "5")
	e.cursor = Cursor{Row: 4, Col: 0}
	e.viewHeight = 3
	ev := tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModShift)
	if !e.handleSelectionMove(ev) {
		t.Fatalf("handleSelectionMove returned false")
	}
	if e.cursor.Row != 1 {
		t.Fatalf("cursor row = %d, want 1", e.cursor.Row)
	}
	if !e.selectionActive {
		t.Fatalf("selectionActive = false, want true")
	}
}

func TestExecCommandLineNumbers(t *testing.T) {
	e := newTestEditor("a")
	if e.lineNumberMode != LineNumberAbsolute {
		t.Fatalf("default lineNumberMode = %v, want absolute", e.lineNumberMode)
	}
	e.execCommand("ln rel")
	if e.lineNumberMode != LineNumberRelative {
		t.Fatalf("lineNumberMode = %v, want relative", e.lineNumberMode)
	}
	e.execCommand("ln off")
	if e.lineNumberMode != LineNumberOff {
		t.Fatalf("lineNumberMode = %v, want off", e.lineNumberMode)
	}
}

func TestExecCommandWriteAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	e := newTestEditor("hello")
	if quit := e.execCommand("w " + path); quit {
		t.Fatalf("execCommand w returned true")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("file contents = %q, want %q", string(data), "hello")
	}
	if e.dirty {
		t.Fatalf("dirty = true, want false")
	}
}

func TestExecCommandQuitWithDirty(t *testing.T) {
	e := newTestEditor("a")
	e.insertRune('b')
	if !e.dirty {
		t.Fatalf("dirty = false, want true")
	}
	if quit := e.execCommand("q"); quit {
		t.Fatalf("expected quit=false when dirty")
	}
	if e.statusMessage == "" {
		t.Fatalf("expected status message for dirty quit")
	}
	if quit := e.execCommand("q!"); !quit {
		t.Fatalf("expected quit=true for q!")
	}
}

func TestExecCommandFmtNoGo(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not available")
	}
	e := newTestEditor("package main\nfunc main() {  }\n")
	if quit := e.execCommand("fmt"); quit {
		t.Fatalf("execCommand fmt returned true")
	}
	if e.statusMessage != "formatted" {
		t.Fatalf("unexpected status: %q", e.statusMessage)
	}
	if got := e.Content(); got == "package main\nfunc main() {  }\n" {
		t.Fatalf("expected formatted content, got unchanged")
	}
}

func TestExecCommandUnknown(t *testing.T) {
	e := newTestEditor("a")
	if quit := e.execCommand("nope"); quit {
		t.Fatalf("execCommand unknown returned true")
	}
	if e.statusMessage == "" {
		t.Fatalf("expected status message for unknown command")
	}
}

func TestHandleInsertUndoRedo(t *testing.T) {
	e := newTestEditor("")
	e.mode = ModeInsert
	ev := tcell.NewEventKey(tcell.KeyRune, 'a', 0)
	e.handleInsert(ev)
	if got := e.Content(); got != "a" {
		t.Fatalf("content = %q, want %q", got, "a")
	}
	e.Undo()
	if got := e.Content(); got != "" {
		t.Fatalf("undo content = %q, want %q", got, "")
	}
	e.Redo()
	if got := e.Content(); got != "a" {
		t.Fatalf("redo content = %q, want %q", got, "a")
	}
}

func TestHandleInsertBackspaceUndo(t *testing.T) {
	e := newTestEditor("ab")
	e.mode = ModeInsert
	e.cursor = Cursor{Row: 0, Col: 2}
	ev := tcell.NewEventKey(tcell.KeyBackspace, 0, 0)
	e.handleInsert(ev)
	if got := e.Content(); got != "a" {
		t.Fatalf("content = %q, want %q", got, "a")
	}
	e.Undo()
	if got := e.Content(); got != "ab" {
		t.Fatalf("undo content = %q, want %q", got, "ab")
	}
}

func TestHandleInsertNewlineUndo(t *testing.T) {
	e := newTestEditor("ab")
	e.mode = ModeInsert
	e.cursor = Cursor{Row: 0, Col: 1}
	ev := tcell.NewEventKey(tcell.KeyEnter, 0, 0)
	e.handleInsert(ev)
	if len(e.lines) != 2 || string(e.lines[0]) != "a" || string(e.lines[1]) != "b" {
		t.Fatalf("lines = %q, want [\"a\" \"b\"]", e.Content())
	}
	e.Undo()
	if got := e.Content(); got != "ab" {
		t.Fatalf("undo content = %q, want %q", got, "ab")
	}
}

func TestHandleInsertClearsSelection(t *testing.T) {
	e := newTestEditor("ab")
	e.mode = ModeInsert
	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 0}
	e.selectionEnd = Cursor{Row: 0, Col: 1}
	ev := tcell.NewEventKey(tcell.KeyRune, 'x', 0)
	e.handleInsert(ev)
	if e.selectionActive {
		t.Fatalf("selectionActive = true, want false")
	}
}

func TestKeyStringForMapMetaHome(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModMeta)
	key := keyStringForMap(ev, map[string]string{"cmd+left": "word_left"})
	if key != "cmd+left" {
		t.Fatalf("key = %q, want %q", key, "cmd+left")
	}
	key = keyStringForMap(ev, map[string]string{})
	if key != "cmd+home" {
		t.Fatalf("key = %q, want %q", key, "cmd+home")
	}
}

func TestFormatGitBranch(t *testing.T) {
	if got := formatGitBranch("", "main"); got != "git:main" {
		t.Fatalf("formatGitBranch default = %q, want %q", got, "git:main")
	}
	if got := formatGitBranch("branch", "dev"); got != "branch dev" {
		t.Fatalf("formatGitBranch symbol = %q, want %q", got, "branch dev")
	}
}

func TestHandleKeyCommandWriteQuit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	e := newTestEditor("hi")

	if quit := e.HandleKey(keyRune(':')); quit {
		t.Fatalf("enter command returned quit")
	}
	for _, r := range "w " + path {
		if quit := e.HandleKey(keyRune(r)); quit {
			t.Fatalf("write command returned quit")
		}
	}
	if quit := e.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)); quit {
		t.Fatalf("write command returned quit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hi" {
		t.Fatalf("file contents = %q, want %q", string(data), "hi")
	}

	if quit := e.HandleKey(keyRune(':')); quit {
		t.Fatalf("enter command returned quit")
	}
	if quit := e.HandleKey(keyRune('q')); quit {
		t.Fatalf("q rune returned quit")
	}
	if quit := e.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)); !quit {
		t.Fatalf("expected quit on :q")
	}
}

func TestBranchPickerSelection(t *testing.T) {
	e := newTestEditor("a")
	e.ShowBranchPicker([]string{"dev", "main"}, "dev")
	if e.mode != ModeBranchPicker {
		t.Fatalf("mode = %v, want branch picker", e.mode)
	}
	_ = e.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	_ = e.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	branch, ok := e.ConsumeBranchSelection()
	if !ok || branch != "main" {
		t.Fatalf("selection = %q ok=%v, want main", branch, ok)
	}
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want normal", e.mode)
	}
}

func TestBranchPickerCancel(t *testing.T) {
	e := newTestEditor("a")
	e.ShowBranchPicker([]string{"dev", "main"}, "dev")
	_ = e.HandleKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	if _, ok := e.ConsumeBranchSelection(); ok {
		t.Fatalf("expected no selection on cancel")
	}
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want normal", e.mode)
	}
}

func keyRune(r rune) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, r, 0)
}

// ============================================================================
// TAB / Shift+TAB (indent/unindent) tests - REAL WORKFLOW via HandleKey
// ============================================================================

func keyTab() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyTab, 0, 0)
}

func keyShiftTab() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyBacktab, 0, 0)
}

func keyUndo() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, 'u', 0)
}

func keyShiftDown() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModShift)
}

func TestKeyStringForTab(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyTab, 0, 0)
	key := keyString(ev)
	if key != "tab" {
		t.Fatalf("keyString(Tab) = %q, want %q", key, "tab")
	}
}

func TestKeyStringForShiftTab(t *testing.T) {
	// Test KeyTab with ModShift
	ev := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModShift)
	key := keyString(ev)
	if key != "shift+tab" {
		t.Fatalf("keyString(Shift+Tab) = %q, want %q", key, "shift+tab")
	}

	// Test KeyBacktab (alternative representation)
	ev2 := tcell.NewEventKey(tcell.KeyBacktab, 0, 0)
	key2 := keyString(ev2)
	if key2 != "shift+tab" {
		t.Fatalf("keyString(Backtab) = %q, want %q", key2, "shift+tab")
	}
}

// Test: INSERT mode + selection + TAB key press
func TestInsertModeWithSelectionTabViaHandleKey(t *testing.T) {
	e := newTestEditor("aa", "bb", "cc")
	e.mode = ModeInsert
	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 0}
	e.selectionEnd = Cursor{Row: 1, Col: 2}
	e.cursor = Cursor{Row: 1, Col: 2}

	// Send TAB key event
	e.HandleKey(keyTab())

	if string(e.lines[0]) != "\taa" {
		t.Fatalf("line0 = %q, want %q", string(e.lines[0]), "\taa")
	}
	if string(e.lines[1]) != "\tbb" {
		t.Fatalf("line1 = %q, want %q", string(e.lines[1]), "\tbb")
	}
	if string(e.lines[2]) != "cc" {
		t.Fatalf("line2 = %q, want %q", string(e.lines[2]), "cc")
	}
	if !e.selectionActive {
		t.Fatalf("selectionActive = false, want true")
	}
}

// Test: INSERT mode + selection + Shift+TAB key press
func TestInsertModeWithSelectionShiftTabViaHandleKey(t *testing.T) {
	e := newTestEditor("\taa", "\tbb", "cc")
	e.mode = ModeInsert
	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 0}
	e.selectionEnd = Cursor{Row: 1, Col: 3}
	e.cursor = Cursor{Row: 1, Col: 3}

	// Send Shift+TAB key event
	e.HandleKey(keyShiftTab())

	if string(e.lines[0]) != "aa" {
		t.Fatalf("line0 = %q, want %q", string(e.lines[0]), "aa")
	}
	if string(e.lines[1]) != "bb" {
		t.Fatalf("line1 = %q, want %q", string(e.lines[1]), "bb")
	}
	if string(e.lines[2]) != "cc" {
		t.Fatalf("line2 = %q, want %q", string(e.lines[2]), "cc")
	}
	if !e.selectionActive {
		t.Fatalf("selectionActive = false, want true")
	}
}

// Test: INSERT mode + NO selection + TAB (insert at cursor position)
func TestInsertModeNoSelectionTabViaHandleKey(t *testing.T) {
	e := newTestEditor("ab")
	e.mode = ModeInsert
	e.cursor = Cursor{Row: 0, Col: 1}
	e.selectionActive = false

	// Send TAB key event
	e.HandleKey(keyTab())

	if string(e.lines[0]) != "a\tb" {
		t.Fatalf("line0 = %q, want %q", string(e.lines[0]), "a\tb")
	}
	if e.cursor.Col != 2 {
		t.Fatalf("cursor.Col = %d, want 2", e.cursor.Col)
	}
}

// Test: INSERT mode + NO selection + Shift+TAB (unindent current line)
func TestInsertModeNoSelectionShiftTabViaHandleKey(t *testing.T) {
	e := newTestEditor("\tab")
	e.mode = ModeInsert
	e.cursor = Cursor{Row: 0, Col: 1}
	e.selectionActive = false

	// Send Shift+TAB key event
	e.HandleKey(keyShiftTab())

	if string(e.lines[0]) != "ab" {
		t.Fatalf("line0 = %q, want %q", string(e.lines[0]), "ab")
	}
}

// Test: NORMAL mode + selection + TAB key press
func TestNormalModeWithSelectionTabViaHandleKey(t *testing.T) {
	e := newTestEditor("aa", "bb", "cc")
	e.mode = ModeNormal
	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 0}
	e.selectionEnd = Cursor{Row: 1, Col: 2}
	e.cursor = Cursor{Row: 1, Col: 2}

	// Send TAB key event
	e.HandleKey(keyTab())

	if string(e.lines[0]) != "\taa" {
		t.Fatalf("line0 = %q, want %q", string(e.lines[0]), "\taa")
	}
	if string(e.lines[1]) != "\tbb" {
		t.Fatalf("line1 = %q, want %q", string(e.lines[1]), "\tbb")
	}
	if string(e.lines[2]) != "cc" {
		t.Fatalf("line2 = %q, want %q", string(e.lines[2]), "cc")
	}
	if !e.selectionActive {
		t.Fatalf("selectionActive = false, want true")
	}
}

// Test: NORMAL mode + selection + Shift+TAB key press
func TestNormalModeWithSelectionShiftTabViaHandleKey(t *testing.T) {
	e := newTestEditor("\taa", "\tbb", "cc")
	e.mode = ModeNormal
	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 0}
	e.selectionEnd = Cursor{Row: 1, Col: 3}
	e.cursor = Cursor{Row: 1, Col: 3}

	// Send Shift+TAB key event
	e.HandleKey(keyShiftTab())

	if string(e.lines[0]) != "aa" {
		t.Fatalf("line0 = %q, want %q", string(e.lines[0]), "aa")
	}
	if string(e.lines[1]) != "bb" {
		t.Fatalf("line1 = %q, want %q", string(e.lines[1]), "bb")
	}
	if string(e.lines[2]) != "cc" {
		t.Fatalf("line2 = %q, want %q", string(e.lines[2]), "cc")
	}
	if !e.selectionActive {
		t.Fatalf("selectionActive = false, want true")
	}
}

// Test: NORMAL mode + NO selection + TAB (indent at LINE START, not cursor!)
func TestNormalModeNoSelectionTabViaHandleKey(t *testing.T) {
	e := newTestEditor("ab")
	e.mode = ModeNormal
	e.cursor = Cursor{Row: 0, Col: 1} // cursor in middle
	e.selectionActive = false

	// Send TAB key event
	e.HandleKey(keyTab())

	// Tab should go at LINE START, not cursor position!
	if string(e.lines[0]) != "\tab" {
		t.Fatalf("line0 = %q, want %q (tab at LINE START)", string(e.lines[0]), "\tab")
	}
}

// Test: NORMAL mode + NO selection + Shift+TAB
func TestNormalModeNoSelectionShiftTabViaHandleKey(t *testing.T) {
	e := newTestEditor("\tab")
	e.mode = ModeNormal
	e.cursor = Cursor{Row: 0, Col: 0}
	e.selectionActive = false

	// Send Shift+TAB key event
	e.HandleKey(keyShiftTab())

	if string(e.lines[0]) != "ab" {
		t.Fatalf("line0 = %q, want %q", string(e.lines[0]), "ab")
	}
}

// Test: Selection with end.Col == 0 should NOT include that last line
func TestSelectionWithEndColZeroExcludesLastLineViaHandleKey(t *testing.T) {
	e := newTestEditor("aa", "bb", "cc", "dd")
	e.mode = ModeInsert
	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 1}
	e.selectionEnd = Cursor{Row: 3, Col: 0} // Col=0 means line 3 is NOT selected
	e.cursor = Cursor{Row: 3, Col: 0}

	// Send TAB key event
	e.HandleKey(keyTab())

	if string(e.lines[0]) != "\taa" {
		t.Fatalf("line0 = %q, want %q", string(e.lines[0]), "\taa")
	}
	if string(e.lines[1]) != "\tbb" {
		t.Fatalf("line1 = %q, want %q", string(e.lines[1]), "\tbb")
	}
	if string(e.lines[2]) != "\tcc" {
		t.Fatalf("line2 = %q, want %q", string(e.lines[2]), "\tcc")
	}
	// Line 3 should NOT be indented because selectionEnd.Col == 0
	if string(e.lines[3]) != "dd" {
		t.Fatalf("line3 = %q, want %q (should NOT be indented)", string(e.lines[3]), "dd")
	}
}

// Test: Undo after indent should restore ALL lines in one undo
func TestUndoAfterIndentViaHandleKey(t *testing.T) {
	e := newTestEditor("aa", "bb")
	e.mode = ModeNormal
	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 0}
	e.selectionEnd = Cursor{Row: 1, Col: 2}
	e.cursor = Cursor{Row: 1, Col: 2}

	// Send TAB key event
	e.HandleKey(keyTab())

	if string(e.lines[0]) != "\taa" || string(e.lines[1]) != "\tbb" {
		t.Fatalf("after indent: lines = %q, %q", string(e.lines[0]), string(e.lines[1]))
	}

	// Send 'u' for undo
	e.HandleKey(keyUndo())

	if string(e.lines[0]) != "aa" {
		t.Fatalf("after undo line0 = %q, want %q", string(e.lines[0]), "aa")
	}
	if string(e.lines[1]) != "bb" {
		t.Fatalf("after undo line1 = %q, want %q", string(e.lines[1]), "bb")
	}
}

// Test: Selection should be preserved after TAB
func TestSelectionPreservedAfterTabViaHandleKey(t *testing.T) {
	e := newTestEditor("aa", "bb")
	e.mode = ModeInsert
	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 0}
	e.selectionEnd = Cursor{Row: 1, Col: 2}
	e.cursor = Cursor{Row: 1, Col: 2}

	// Send TAB key event
	e.HandleKey(keyTab())

	if !e.selectionActive {
		t.Fatalf("selectionActive = false, want true")
	}
}

// Test: Verify keybindings exist in config
func TestNormalModeTabKeybinding(t *testing.T) {
	cfg := config.Default()
	// Verify that tab and shift+tab are in Normal keymap
	if _, ok := cfg.Keymap.Normal["tab"]; !ok {
		t.Fatalf("Normal keymap missing 'tab' binding")
	}
	if _, ok := cfg.Keymap.Normal["shift+tab"]; !ok {
		t.Fatalf("Normal keymap missing 'shift+tab' binding")
	}
}

// ============================================================================
// REAL WORKFLOW TEST - selection created via Shift+Down, then TAB
// ============================================================================

func TestRealWorkflowInsertModeShiftSelectThenTab(t *testing.T) {
	// Реальный сценарий как на скриншоте:
	// 1. Файл с несколькими строками
	// 2. INSERT режим
	// 3. Курсор в середине текста (не в начале!)
	// 4. Shift+Down несколько раз
	// 5. TAB

	e := newTestEditor(
		`import (`,
		`	"os"`,
		`	"os/exec"`,
		`	"path/filepath"`,
		`	"testing"`,
		`)`,
	)
	e.mode = ModeInsert
	e.cursor = Cursor{Row: 1, Col: 4} // курсор на "os", col=4 (после tab и ")

	t.Logf("BEFORE: cursor=%+v, selectionActive=%v", e.cursor, e.selectionActive)

	// Shift+Down 3 раза (выделяем строки 1,2,3,4)
	e.HandleKey(keyShiftDown())
	t.Logf("After 1st Shift+Down: cursor=%+v, selStart=%+v, selEnd=%+v, active=%v",
		e.cursor, e.selectionStart, e.selectionEnd, e.selectionActive)

	e.HandleKey(keyShiftDown())
	t.Logf("After 2nd Shift+Down: cursor=%+v, selStart=%+v, selEnd=%+v, active=%v",
		e.cursor, e.selectionStart, e.selectionEnd, e.selectionActive)

	e.HandleKey(keyShiftDown())
	t.Logf("After 3rd Shift+Down: cursor=%+v, selStart=%+v, selEnd=%+v, active=%v",
		e.cursor, e.selectionStart, e.selectionEnd, e.selectionActive)

	// Проверяем что выделение создано
	if !e.selectionActive {
		t.Fatalf("selectionActive = false, want true")
	}

	// TAB
	e.HandleKey(keyTab())

	t.Logf("AFTER TAB:")
	for i, line := range e.lines {
		t.Logf("  line[%d] = %q", i, string(line))
	}
	t.Logf("  cursor=%+v, selectionActive=%v", e.cursor, e.selectionActive)

	// Строки 1,2,3 должны получить indent (строка 4 - зависит от Col)
	if string(e.lines[1]) != "\t\t\"os\"" {
		t.Fatalf("line1 = %q, want %q", string(e.lines[1]), "\t\t\"os\"")
	}
	if string(e.lines[2]) != "\t\t\"os/exec\"" {
		t.Fatalf("line2 = %q, want %q", string(e.lines[2]), "\t\t\"os/exec\"")
	}
	if string(e.lines[3]) != "\t\t\"path/filepath\"" {
		t.Fatalf("line3 = %q, want %q", string(e.lines[3]), "\t\t\"path/filepath\"")
	}
}
