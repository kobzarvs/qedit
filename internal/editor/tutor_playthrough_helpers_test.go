package editor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Tutor playthrough tests simulate a user completing embedded tutorials:
// open :tutor, navigate with hjkl motions (and / only for whole-line jumps when unique),
// edit with pressKeyScript only. Cursor is never assigned directly.

type vimTutorExpectations struct {
	Expect map[string]json.RawMessage `json:"expect"`
}

type helixTutorExpectations struct {
	MustContain []string `json:"must_contain"`
	Deleted     []string `json:"deleted"`
}

type vimTutorPlaythroughExpectations struct {
	MustContain []string `json:"must_contain"`
	Deleted     []string `json:"deleted"`
}

func openTutorViaKeys(t *testing.T, profile string) *Editor {
	t.Helper()
	e := newSimulatedProfileEditor(profile, "")
	switch profile {
	case BehaviorProfileVim:
		pressKeyScript(t, e, ":tutor vim<enter>")
		if e.document.title != "[Tutor: Vim]" {
			t.Fatalf("title = %q, want Vim tutor", e.document.title)
		}
	case BehaviorProfileHelix:
		pressKeyScript(t, e, ":tutor helix<enter>")
		if e.document.title != "[Tutor: Helix]" {
			t.Fatalf("title = %q, want Helix tutor", e.document.title)
		}
	default:
		t.Fatalf("unsupported tutor profile %q", profile)
	}
	return e
}

func loadVimBeginnerTutor(t *testing.T) ([]string, vimTutorExpectations) {
	t.Helper()
	tutorPath := filepath.Join("testdata", "vim-01-beginner.tutor")
	rawTutor, err := os.ReadFile(tutorPath)
	if err != nil {
		t.Fatalf("read %s: %v", tutorPath, err)
	}
	expectPath := filepath.Join("testdata", "vim-01-beginner.tutor.json")
	rawExpect, err := os.ReadFile(expectPath)
	if err != nil {
		t.Fatalf("read %s: %v", expectPath, err)
	}
	var expects vimTutorExpectations
	if err := json.Unmarshal(rawExpect, &expects); err != nil {
		t.Fatalf("parse %s: %v", expectPath, err)
	}
	if len(expects.Expect) == 0 {
		t.Fatalf("%s has no expectations", expectPath)
	}
	text := strings.ReplaceAll(string(rawTutor), "\r\n", "\n")
	return strings.Split(strings.TrimRight(text, "\n"), "\n"), expects
}

func loadHelixTutorExpectations(t *testing.T) ([]string, helixTutorExpectations) {
	t.Helper()
	lines := loadHelixTutorLines(t)
	expectPath := filepath.Join("testdata", "helix-tutor.json")
	rawExpect, err := os.ReadFile(expectPath)
	if err != nil {
		t.Fatalf("read %s: %v", expectPath, err)
	}
	var expects helixTutorExpectations
	if err := json.Unmarshal(rawExpect, &expects); err != nil {
		t.Fatalf("parse %s: %v", expectPath, err)
	}
	if len(expects.MustContain) == 0 {
		t.Fatalf("%s has no expectations", expectPath)
	}
	return lines, expects
}

func loadHelixTutorLines(t *testing.T) []string {
	t.Helper()
	path := filepath.Join("testdata", "helix-tutor")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

func tutorLine(t *testing.T, lines []string, lineNumber int) string {
	t.Helper()
	index := lineNumber - 1
	if index < 0 || index >= len(lines) {
		t.Fatalf("tutor line %d out of range", lineNumber)
	}
	return lines[index]
}

func tutorExpectedLine(t *testing.T, expects vimTutorExpectations, lineNumber int) string {
	t.Helper()
	raw, ok := expects.Expect[strconv.Itoa(lineNumber)]
	if !ok {
		t.Fatalf("missing tutor expectation for line %d", lineNumber)
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("line %d expectation is not a string: %v", lineNumber, err)
	}
	return got
}

func helixExpectedLine(t *testing.T, lines []string, lineNumber int) string {
	t.Helper()
	// Reference lines under "-->" exercises omit the marker; only the text is compared.
	return strings.TrimSpace(tutorLine(t, lines, lineNumber))
}

func helixMarkedExpectedLine(t *testing.T, lines []string, lineNumber int) string {
	t.Helper()
	return " --> " + helixExpectedLine(t, lines, lineNumber)
}

func loadVimTutorPlaythroughExpectations(t *testing.T) vimTutorPlaythroughExpectations {
	t.Helper()
	expectPath := filepath.Join("testdata", "vim-01-beginner.tutor.playthrough.json")
	rawExpect, err := os.ReadFile(expectPath)
	if err != nil {
		t.Fatalf("read %s: %v", expectPath, err)
	}
	var expects vimTutorPlaythroughExpectations
	if err := json.Unmarshal(rawExpect, &expects); err != nil {
		t.Fatalf("parse %s: %v", expectPath, err)
	}
	if len(expects.MustContain) == 0 {
		t.Fatalf("%s has no expectations", expectPath)
	}
	return expects
}

func assertVimTutorPlaythroughExpectations(t *testing.T, e *Editor, expects vimTutorPlaythroughExpectations) {
	t.Helper()
	for _, line := range expects.MustContain {
		tutorPlaythroughAssertLine(t, e, line)
	}
	for _, line := range expects.Deleted {
		tutorPlaythroughAssertNoLine(t, e, line)
	}
}

func assertHelixTutorDocumentedExpectations(t *testing.T, e *Editor, expects helixTutorExpectations) {
	t.Helper()
	for _, line := range expects.MustContain {
		tutorPlaythroughAssertLine(t, e, line)
	}
	for _, line := range expects.Deleted {
		tutorPlaythroughAssertNoLine(t, e, line)
	}
}

func tutorPlaythroughEditAt(t *testing.T, e *Editor, needle string, offset int, script string) {
	t.Helper()
	tutorPlaythroughGotoNeedle(t, e, needle, offset)
	pressKeyScript(t, e, script)
	if e.mode == ModeInsert {
		t.Fatalf("script %q left editor in insert mode", script)
	}
}

func tutorPlaythroughEditLineAt(t *testing.T, e *Editor, line string, offset int, script string) {
	t.Helper()
	tutorPlaythroughGotoLineExact(t, e, line, 1)
	tutorPlaythroughMoveRight(t, e, offset)
	pressKeyScript(t, e, script)
	if e.mode == ModeInsert {
		t.Fatalf("script %q left editor in insert mode", script)
	}
}

func tutorPlaythroughEditLineOccurrenceAt(t *testing.T, e *Editor, line string, occurrence int, offset int, script string) {
	t.Helper()
	tutorPlaythroughGotoLineExact(t, e, line, occurrence)
	tutorPlaythroughMoveRight(t, e, offset)
	pressKeyScript(t, e, script)
	if e.mode == ModeInsert {
		t.Fatalf("script %q left editor in insert mode", script)
	}
}

func tutorPlaythroughGotoColumn(t *testing.T, e *Editor, profile string, col int) {
	t.Helper()
	switch profile {
	case BehaviorProfileVim:
		pressKeyScript(t, e, "0")
	default:
		pressKeyScript(t, e, "<home>")
	}
	tutorPlaythroughMoveRight(t, e, col)
}

func tutorPlaythroughGotoRow(t *testing.T, e *Editor, targetRow int) {
	t.Helper()
	pressKeyScript(t, e, "<esc>,")
	pressKeyScript(t, e, "gg")
	for i := 0; i < targetRow; i++ {
		pressKeyScript(t, e, "j")
	}
	if e.cursor.Row != targetRow {
		t.Fatalf("cursor row = %d, want %d", e.cursor.Row, targetRow)
	}
}

func tutorPlaythroughGotoNeedle(t *testing.T, e *Editor, needle string, offset int) {
	t.Helper()
	if matches := tutorPlaythroughExactMatchCount(e, needle); matches != 1 {
		t.Fatalf("needle %q appears in %d lines, want 1", needle, matches)
	}
	row := tutorPlaythroughRowWithNeedle(e, needle)
	if row < 0 {
		t.Fatalf("needle %q not found in tutorial buffer", needle)
	}
	tutorPlaythroughGotoRow(t, e, row)
	col := tutorPlaythroughNeedleCol(e, row, needle) + offset
	tutorPlaythroughGotoColumn(t, e, e.BehaviorProfile(), col)
	if e.mode != ModeNormal {
		t.Fatalf("goto %q left mode = %v, want normal", needle, e.mode)
	}
	line := string(e.line(e.cursor.Row))
	runes := []rune(line)
	if e.cursor.Col < 0 || e.cursor.Col > len(runes) {
		t.Fatalf("goto %q left cursor col out of range: line=%q cursor=%+v", needle, line, e.cursor)
	}
	needleRunes := []rune(needle)
	want := needleRunes
	if offset > 0 {
		if offset > len(needleRunes) {
			t.Fatalf("goto %q offset %d exceeds needle length", needle, offset)
		}
		want = needleRunes[offset:]
	}
	got := string(runes[e.cursor.Col:])
	if !strings.HasPrefix(got, string(want)) {
		t.Fatalf("goto %q landed at line %q col %d, suffix %q", needle, line, e.cursor.Col, got)
	}
}

func tutorPlaythroughGotoLineExact(t *testing.T, e *Editor, line string, occurrence int) {
	t.Helper()
	if occurrence < 1 {
		t.Fatalf("line %q requested invalid occurrence %d", line, occurrence)
	}
	rows := tutorPlaythroughRowsExact(e, line)
	if len(rows) < occurrence {
		t.Fatalf("line %q appears %d times, want at least %d", line, len(rows), occurrence)
	}
	tutorPlaythroughGotoRow(t, e, rows[occurrence-1])
	tutorPlaythroughGotoColumn(t, e, e.BehaviorProfile(), 0)
	if e.mode != ModeNormal {
		t.Fatalf("goto line %q left mode = %v, want normal", line, e.mode)
	}
	if got := string(e.line(e.cursor.Row)); got != line {
		t.Fatalf("goto line %q landed at %q", line, got)
	}
}

func tutorPlaythroughRowWithNeedle(e *Editor, needle string) int {
	for row := 0; row < e.LineCount(); row++ {
		if strings.Contains(string(e.line(row)), needle) {
			return row
		}
	}
	return -1
}

func tutorPlaythroughNeedleCol(e *Editor, row int, needle string) int {
	line := []rune(string(e.line(row)))
	needleRunes := []rune(needle)
	for col := 0; col+len(needleRunes) <= len(line); col++ {
		if string(line[col:col+len(needleRunes)]) == needle {
			return col
		}
	}
	return 0
}

func tutorPlaythroughRowsExact(e *Editor, line string) []int {
	rows := make([]int, 0, 1)
	for row := 0; row < e.LineCount(); row++ {
		if string(e.line(row)) == line {
			rows = append(rows, row)
		}
	}
	return rows
}

func tutorPlaythroughExactMatchCount(e *Editor, needle string) int {
	matches := 0
	for row := 0; row < e.LineCount(); row++ {
		if strings.Contains(string(e.line(row)), needle) {
			matches++
		}
	}
	return matches
}

func tutorPlaythroughTypeLiteral(t *testing.T, e *Editor, text string) {
	t.Helper()
	pressKeyScript(t, e, text)
}

func tutorPlaythroughMoveRight(t *testing.T, e *Editor, count int) {
	t.Helper()
	if count <= 0 {
		return
	}
	pressKeyScript(t, e, strings.Repeat("l", count))
}

func tutorPlaythroughAssertLine(t *testing.T, e *Editor, line string) {
	t.Helper()
	if !tutorPlaythroughHasLine(e, line) {
		t.Fatalf("tutorial buffer does not contain line %q", line)
	}
}

func tutorPlaythroughAssertNoLine(t *testing.T, e *Editor, line string) {
	t.Helper()
	if tutorPlaythroughHasLine(e, line) {
		t.Fatalf("tutorial buffer still contains line %q", line)
	}
}

func tutorPlaythroughAssertLineCount(t *testing.T, e *Editor, line string, want int) {
	t.Helper()
	got := 0
	for row := 0; row < e.LineCount(); row++ {
		if string(e.line(row)) == line {
			got++
		}
	}
	if got != want {
		t.Fatalf("tutorial buffer contains line %q %d times, want %d", line, got, want)
	}
}

func tutorPlaythroughHasLine(e *Editor, line string) bool {
	for row := 0; row < e.LineCount(); row++ {
		if string(e.line(row)) == line {
			return true
		}
	}
	return false
}

func newVimTutorEditor(lines ...string) *Editor {
	e := newTestEditor(lines...)
	e.SetBehaviorProfile(BehaviorProfileVim)
	e.mode = ModeNormal
	return e
}

func vimPressRunes(t *testing.T, e *Editor, keys string) {
	t.Helper()
	pressKeyScript(t, e, keys)
}

func vimPressKey(t *testing.T, e *Editor, key string) {
	t.Helper()
	pressKeyScript(t, e, "<"+key+">")
}

func tutorPlaythroughResetSingleWindow(t *testing.T, e *Editor) {
	t.Helper()
	pressKeyScript(t, e, "<esc>")
	for e.windowCount() > 1 {
		pressKeyScript(t, e, "<ctrl+w>q")
	}
	if e.windowCount() != 1 {
		pressKeyScript(t, e, "<ctrl+w>o")
	}
	if e.windowCount() != 1 {
		t.Fatalf("window count after reset = %d, want 1", e.windowCount())
	}
}

func tutorPlaythroughHelixWindowChapter(t *testing.T, e *Editor) {
	t.Helper()
	tutorPlaythroughResetSingleWindow(t, e)

	pressKeyScript(t, e, "<ctrl+w>nv")
	if got := e.windowCount(); got != 2 {
		t.Fatalf("window count after ctrl-w nv = %d, want 2", got)
	}
	if got := e.BufferCount(); got < 2 {
		t.Fatalf("buffer count after ctrl-w nv = %d, want at least 2", got)
	}
	rightID := e.windows.activeID

	pressKeyScript(t, e, "<ctrl+w>ns")
	if got := e.windowCount(); got != 3 {
		t.Fatalf("window count after ctrl-w ns = %d, want 3", got)
	}

	pressKeyScript(t, e, "<ctrl+w>h")
	if e.windows.activeID == rightID {
		t.Fatalf("ctrl-w h did not leave the right split")
	}

	tutorPlaythroughResetSingleWindow(t, e)

	pressKeyScript(t, e, "<ctrl+w>v")
	pressKeyScript(t, e, "<ctrl+w>s")
	if got := e.windowCount(); got != 3 {
		t.Fatalf("window count after current-buffer splits = %d, want 3", got)
	}

	pressKeyScript(t, e, "<ctrl+w>q")
	if got := e.windowCount(); got != 2 {
		t.Fatalf("window count after ctrl-w q = %d, want 2", got)
	}

	pressKeyScript(t, e, "<ctrl+w>o")
	if got := e.windowCount(); got != 1 {
		t.Fatalf("window count after ctrl-w o = %d, want 1", got)
	}

	pressKeyScript(t, e, ":vs hello1<enter>")
	pressKeyScript(t, e, ":hs hello2<enter>")
	if got := e.windowCount(); got != 3 {
		t.Fatalf("window count after :vs/:hs = %d, want 3", got)
	}
	if got := e.BufferCount(); got < 3 {
		t.Fatalf("buffer count after :vs/:hs = %d, want at least 3", got)
	}

	pressKeyScript(t, e, "<ctrl+w>K")
	if !strings.HasSuffix(e.document.filename, "hello2") {
		t.Fatalf("active file after ctrl-w K = %q, want hello2", e.document.filename)
	}

	pressKeyScript(t, e, "<ctrl+w>t")
	if parent := e.activeWindowLeaf().parent; parent == nil || parent.axis != editorWindowVertical {
		t.Fatalf("active parent axis = %#v, want vertical after transpose", parent)
	}

	tutorPlaythroughResetSingleWindow(t, e)
}
