package editor

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOpenHugeFileBufferIndexesLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\r\ngamma\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if got := buf.LineCount(); got != 4 {
		t.Fatalf("line count = %d, want 4", got)
	}
	if got := string(buf.Line(0)); got != "alpha" {
		t.Fatalf("line 0 = %q, want %q", got, "alpha")
	}
	if got := string(buf.Line(1)); got != "beta" {
		t.Fatalf("line 1 = %q, want %q", got, "beta")
	}
	if got := string(buf.Line(2)); got != "gamma" {
		t.Fatalf("line 2 = %q, want %q", got, "gamma")
	}
	if got := string(buf.Line(3)); got != "" {
		t.Fatalf("line 3 = %q, want empty", got)
	}
}

func TestLoadHugeFileEntersLimitedEditMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	if !e.HugeFileMode() {
		t.Fatalf("expected huge file mode to be active")
	}
	if e.file.readOnly {
		t.Fatalf("expected huge file limited edit mode to be writable")
	}
	if got := e.LineCount(); got != 4 {
		t.Fatalf("line count = %d, want 4", got)
	}
	if got := string(e.line(1)); got != "beta" {
		t.Fatalf("line 1 = %q, want %q", got, "beta")
	}

	e.SetBehaviorProfile(BehaviorProfileBasic)
	e.HandleKey(keyRune('x'))
	if e.mode != ModeInsert {
		t.Fatalf("mode = %v, want %v", e.mode, ModeInsert)
	}
	if got := string(e.line(0)); got != "xalpha" {
		t.Fatalf("line 0 after edit = %q, want %q", got, "xalpha")
	}
	if !e.IsDirty() {
		t.Fatalf("expected huge file edit to mark buffer dirty")
	}
}

func TestHugeFileSaveWritesEditedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.SetBehaviorProfile(BehaviorProfileBasic)
	e.cursor = Cursor{Row: 1, Col: 4}
	e.HandleKey(keyRune('!'))

	if err := e.WriteHugeFile(path, realTestFileStore{}); err != nil {
		t.Fatalf("WriteHugeFile returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if got := string(data); got != "alpha\nbeta!\ngamma\n" {
		t.Fatalf("saved content = %q, want %q", got, "alpha\nbeta!\ngamma\n")
	}
	if got := string(e.line(1)); got != "beta!" {
		t.Fatalf("line 1 after reload = %q, want %q", got, "beta!")
	}
	if e.IsDirty() {
		t.Fatalf("expected huge file buffer to be clean after save")
	}
	if len(e.huge.edits) != 0 {
		t.Fatalf("expected huge edit overlay to be cleared after save")
	}
}

func TestHugeFileSavePreservesCRLFAndTrailingEmptyLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\r\nbeta\r\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	if !e.hugeSetLine(2, []rune("omega")) {
		t.Fatalf("hugeSetLine returned false")
	}
	if err := e.WriteHugeFile(path, realTestFileStore{}); err != nil {
		t.Fatalf("WriteHugeFile returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if got := string(data); got != "alpha\r\nbeta\r\nomega" {
		t.Fatalf("saved content = %q, want %q", got, "alpha\r\nbeta\r\nomega")
	}
}

func TestHugeFileSupportsSplitAndJoinLineEdits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.cursor = Cursor{Row: 0, Col: 2}
	e.insertNewline()

	if got := e.LineCount(); got != 4 {
		t.Fatalf("line count after split = %d, want 4", got)
	}
	if got := string(e.line(0)); got != "al" {
		t.Fatalf("line 0 after split = %q, want %q", got, "al")
	}
	if got := string(e.line(1)); got != "pha" {
		t.Fatalf("line 1 after split = %q, want %q", got, "pha")
	}

	e.backspace()

	if got := e.LineCount(); got != 3 {
		t.Fatalf("line count after join = %d, want 3", got)
	}
	if got := string(e.line(0)); got != "alpha" {
		t.Fatalf("line 0 after join = %q, want %q", got, "alpha")
	}
}

func TestHugeFileSaveWritesRowPatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.cursor = Cursor{Row: 0, Col: 2}
	e.insertNewline()

	if err := e.WriteHugeFile(path, realTestFileStore{}); err != nil {
		t.Fatalf("WriteHugeFile returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if got := string(data); got != "al\npha\nbeta\n" {
		t.Fatalf("saved content = %q, want %q", got, "al\npha\nbeta\n")
	}
	if len(e.huge.patches) != 0 {
		t.Fatalf("expected huge row patches to be cleared after save")
	}
}

func TestHugeFileSupportsOpenBelowAndOpenAbove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "  alpha\nbeta\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.cursor = Cursor{Row: 0, Col: 0}
	e.openBelow()
	if got := string(e.line(1)); got != "  " {
		t.Fatalf("line 1 after openBelow = %q, want %q", got, "  ")
	}
	if e.mode != ModeInsert {
		t.Fatalf("mode after openBelow = %v, want %v", e.mode, ModeInsert)
	}

	e.mode = ModeNormal
	e.cursor = Cursor{Row: 2, Col: 0}
	e.openAbove()
	if got := string(e.line(2)); got != "" {
		t.Fatalf("line 2 after openAbove = %q, want empty", got)
	}
	if got := string(e.line(3)); got != "beta" {
		t.Fatalf("line 3 after openAbove = %q, want %q", got, "beta")
	}
}

func TestHugeFileDeleteLineSupportsUndoRedoAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.cursor = Cursor{Row: 1, Col: 0}
	if quit := e.execAction(actionDeleteLine); quit {
		t.Fatalf("delete line returned quit=true")
	}
	if got := e.LineCount(); got != 3 {
		t.Fatalf("line count after delete = %d, want 3", got)
	}
	if got := string(e.line(1)); got != "gamma" {
		t.Fatalf("line 1 after delete = %q, want %q", got, "gamma")
	}

	e.Undo()
	if got := e.LineCount(); got != 4 {
		t.Fatalf("line count after undo = %d, want 4", got)
	}
	if got := string(e.line(1)); got != "beta" {
		t.Fatalf("line 1 after undo = %q, want %q", got, "beta")
	}

	e.Redo()
	if got := e.LineCount(); got != 3 {
		t.Fatalf("line count after redo = %d, want 3", got)
	}
	if got := string(e.line(1)); got != "gamma" {
		t.Fatalf("line 1 after redo = %q, want %q", got, "gamma")
	}

	if err := e.WriteHugeFile(path, realTestFileStore{}); err != nil {
		t.Fatalf("WriteHugeFile returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if got := string(data); got != "alpha\ngamma\n" {
		t.Fatalf("saved content = %q, want %q", got, "alpha\ngamma\n")
	}
}

func TestHugeFileHelixDeleteSelectionSupportsUndo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 2}
	e.selectionEnd = Cursor{Row: 1, Col: 2}
	e.modal.selectMode = true
	if quit := e.execAction(actionDelete); quit {
		t.Fatalf("delete returned quit=true")
	}
	if got := string(e.line(0)); got != "alta" {
		t.Fatalf("line 0 after delete = %q, want %q", got, "alta")
	}
	if got := string(e.line(1)); got != "gamma" {
		t.Fatalf("line 1 after delete = %q, want %q", got, "gamma")
	}
	if e.selectionActive || e.modal.selectMode {
		t.Fatalf("selection should be cleared after helix delete")
	}

	e.Undo()
	if got := string(e.line(0)); got != "alpha" {
		t.Fatalf("line 0 after undo = %q, want %q", got, "alpha")
	}
	if got := string(e.line(1)); got != "beta" {
		t.Fatalf("line 1 after undo = %q, want %q", got, "beta")
	}
	if !e.selectionActive {
		t.Fatalf("expected selection to be restored on undo")
	}
	if e.selectionStart != (Cursor{Row: 0, Col: 2}) || e.selectionEnd != (Cursor{Row: 1, Col: 2}) {
		t.Fatalf("restored selection = %+v..%+v, want {0 2}..{1 2}", e.selectionStart, e.selectionEnd)
	}
}

func TestHugeFileHelixChangeSelectionEntersInsertMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 1}
	e.selectionEnd = Cursor{Row: 0, Col: 4}
	e.modal.selectMode = true
	if quit := e.execAction(actionChange); quit {
		t.Fatalf("change returned quit=true")
	}
	if got := string(e.line(0)); got != "aa" {
		t.Fatalf("line 0 after change = %q, want %q", got, "aa")
	}
	if e.mode != ModeInsert {
		t.Fatalf("mode after change = %v, want %v", e.mode, ModeInsert)
	}
}

func TestHugeFileLinewisePasteSupportsUndo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.clipboard.lines = [][]rune{[]rune("x"), []rune("y")}
	e.clipboard.linewise = true
	e.cursor = Cursor{Row: 0, Col: 0}
	if quit := e.execAction(actionPaste); quit {
		t.Fatalf("paste returned quit=true")
	}
	if got := e.LineCount(); got != 5 {
		t.Fatalf("line count after paste = %d, want 5", got)
	}
	if got := string(e.line(1)); got != "x" {
		t.Fatalf("line 1 after paste = %q, want %q", got, "x")
	}
	if got := string(e.line(2)); got != "y" {
		t.Fatalf("line 2 after paste = %q, want %q", got, "y")
	}

	e.Undo()
	if got := e.LineCount(); got != 3 {
		t.Fatalf("line count after undo = %d, want 3", got)
	}
	if got := string(e.line(1)); got != "beta" {
		t.Fatalf("line 1 after undo = %q, want %q", got, "beta")
	}
}

func TestHugeFileAppendActionsStayAvailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "  alpha\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	if quit := e.execAction(actionAppendLineEnd); quit {
		t.Fatalf("append line end returned quit=true")
	}
	if e.mode != ModeInsert || e.cursor.Col != len([]rune("  alpha")) {
		t.Fatalf("append line end state = mode %v col %d, want insert/%d", e.mode, e.cursor.Col, len([]rune("  alpha")))
	}

	e.mode = ModeNormal
	e.cursor = Cursor{Row: 0, Col: 0}
	if quit := e.execAction(actionInsertLineStart); quit {
		t.Fatalf("insert line start returned quit=true")
	}
	if e.mode != ModeInsert || e.cursor.Col != 2 {
		t.Fatalf("insert line start state = mode %v col %d, want insert/2", e.mode, e.cursor.Col)
	}
}

func TestHugeFileResolveCacheStaysCorrectAcrossSeparatedPatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "zero\none\ntwo\nthree\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.cursor = Cursor{Row: 1, Col: 1}
	e.insertNewline()
	e.cursor = Cursor{Row: 4, Col: 2}
	e.insertNewline()

	want := []string{"zero", "o", "ne", "two", "th", "ree", ""}
	if got := e.LineCount(); got != len(want) {
		t.Fatalf("line count after separated patches = %d, want %d", got, len(want))
	}
	for row, expected := range want {
		if got := string(e.line(row)); got != expected {
			t.Fatalf("line %d = %q, want %q", row, got, expected)
		}
	}
	for row, expected := range want {
		if got := string(e.line(row)); got != expected {
			t.Fatalf("cached line %d = %q, want %q", row, got, expected)
		}
	}
}

func TestHugeFileCoalescesAdjacentRowPatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\nbeta\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, realTestFileStore{}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	e.cursor = Cursor{Row: 0, Col: 1}
	e.insertNewline()
	e.cursor = Cursor{Row: 2, Col: 1}
	e.insertNewline()

	if got := len(e.huge.patches); got != 1 {
		t.Fatalf("patch count after adjacent splits = %d, want 1", got)
	}
	want := []string{"a", "lpha", "b", "eta", ""}
	for row, expected := range want {
		if got := string(e.line(row)); got != expected {
			t.Fatalf("line %d = %q, want %q", row, got, expected)
		}
	}
}

func TestOpenHugeFileBufferUsesSparseCheckpoints(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < hugeFileCheckpointSpacing*3+17; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if got := buf.LineCount(); got != hugeFileCheckpointSpacing*3+18 {
		t.Fatalf("line count = %d, want %d", got, hugeFileCheckpointSpacing*3+18)
	}
	if len(buf.checkpoints) >= buf.LineCount() {
		t.Fatalf("checkpoint count = %d, want sparse index smaller than line count %d", len(buf.checkpoints), buf.LineCount())
	}
	if got := string(buf.Line(hugeFileCheckpointSpacing * 2)); got != "line" {
		t.Fatalf("line at checkpoint boundary = %q, want %q", got, "line")
	}
	if got := string(buf.Line(buf.LineCount() - 1)); got != "" {
		t.Fatalf("last line = %q, want empty", got)
	}
}

func TestHugeFileBufferPrefetchLinesCachesViewport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 200; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if err := buf.PrefetchLines(50, 12); err != nil {
		t.Fatalf("PrefetchLines returned error: %v", err)
	}
	for row := 50; row < 62; row++ {
		if got := string(buf.lineCache[row]); got != "line" {
			t.Fatalf("cached line %d = %q, want %q", row, got, "line")
		}
	}
	if _, ok := buf.cachedPageData(50); !ok {
		t.Fatalf("expected PrefetchLines to warm page data for row 50")
	}

	buf.mu.Lock()
	buf.lineCache = map[int][]rune{}
	buf.cacheOrder = nil
	buf.lineEndings = map[int]string{}
	buf.endingOrder = nil
	buf.lineSpans = map[int]hugeFileLineSpan{}
	buf.spanOrder = nil
	buf.mu.Unlock()

	if line, ok := buf.TryLine(50); !ok || string(line) != "line" {
		t.Fatalf("TryLine(50) after cache eviction = %q, ok=%v, want %q/true", string(line), ok, "line")
	}
	if !buf.CanPrefetchQuick(50, 12) {
		t.Fatalf("expected CanPrefetchQuick(50, 12) to stay true with warmed page cache")
	}
	if got := string(buf.Line(50)); got != "line" {
		t.Fatalf("line(50) after cache eviction = %q, want %q", got, "line")
	}
}

func TestOpenHugeFileBufferBuildsLargeIndexInBackground(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	hugeFileInitialSampleBytes = 32
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 20000; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if got := string(buf.Line(0)); got != "line" {
		t.Fatalf("line 0 = %q, want %q", got, "line")
	}

	buf.WaitForIndexing()

	if !buf.IndexingComplete() {
		t.Fatalf("expected background indexing to complete")
	}
	if got := buf.LineCount(); got != 20001 {
		t.Fatalf("line count = %d, want %d", got, 20001)
	}
}

type slowReadSeekCloser struct {
	*os.File
	maxRead int
	delay   time.Duration
}

func (s *slowReadSeekCloser) Read(p []byte) (int, error) {
	if s.maxRead > 0 && len(p) > s.maxRead {
		p = p[:s.maxRead]
	}
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	return s.File.Read(p)
}

type slowTestFileStore struct {
	maxRead int
	delay   time.Duration
}

func (s slowTestFileStore) Open(path string) (io.ReadSeekCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &slowReadSeekCloser{File: f, maxRead: s.maxRead, delay: s.delay}, nil
}

func (s slowTestFileStore) Read(path string) ([]byte, error) { return os.ReadFile(path) }
func (s slowTestFileStore) Write(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
func (s slowTestFileStore) WriteFrom(path string, src io.Reader) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
func (s slowTestFileStore) Stat(path string) (FileMetadata, error) {
	return realTestFileStore{}.Stat(path)
}
func (s slowTestFileStore) Abs(path string) (string, error) { return realTestFileStore{}.Abs(path) }
func (s slowTestFileStore) HomeDir() (string, error)        { return realTestFileStore{}.HomeDir() }
func (s slowTestFileStore) ReadDir(path string) ([]DirEntry, error) {
	return realTestFileStore{}.ReadDir(path)
}
func (s slowTestFileStore) IsNotExist(err error) bool { return realTestFileStore{}.IsNotExist(err) }

type stagedSlowTestFileStore struct {
	maxRead int
	delay   time.Duration

	mu        sync.Mutex
	openCount int
}

func (s *stagedSlowTestFileStore) Open(path string) (io.ReadSeekCloser, error) {
	s.mu.Lock()
	s.openCount++
	openCount := s.openCount
	s.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if openCount == 1 {
		return f, nil
	}
	return &slowReadSeekCloser{File: f, maxRead: s.maxRead, delay: s.delay}, nil
}

func (s *stagedSlowTestFileStore) Read(path string) ([]byte, error) { return os.ReadFile(path) }
func (s *stagedSlowTestFileStore) Write(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
func (s *stagedSlowTestFileStore) WriteFrom(path string, src io.Reader) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
func (s *stagedSlowTestFileStore) Stat(path string) (FileMetadata, error) {
	return realTestFileStore{}.Stat(path)
}
func (s *stagedSlowTestFileStore) Abs(path string) (string, error) {
	return realTestFileStore{}.Abs(path)
}
func (s *stagedSlowTestFileStore) HomeDir() (string, error) { return realTestFileStore{}.HomeDir() }
func (s *stagedSlowTestFileStore) ReadDir(path string) ([]DirEntry, error) {
	return realTestFileStore{}.ReadDir(path)
}
func (s *stagedSlowTestFileStore) IsNotExist(err error) bool {
	return realTestFileStore{}.IsNotExist(err)
}

func stopHugeFileBackgroundIndex(t *testing.T, buf *HugeFileBuffer) {
	t.Helper()
	if buf == nil {
		return
	}
	buf.mu.Lock()
	cancel := buf.cancelIndex
	indexDone := buf.indexDone
	buf.mu.Unlock()
	if cancel != nil {
		close(cancel)
	}
	if indexDone != nil {
		<-indexDone
	}
	buf.mu.Lock()
	if buf.cancelIndex == cancel {
		buf.cancelIndex = nil
	}
	buf.mu.Unlock()
}

func TestHugeFileBufferTryLineDefersFarRowsUntilIndexed(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	hugeFileInitialSampleBytes = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 8000; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, slowTestFileStore{maxRead: 128, delay: time.Millisecond})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if line, ok := buf.TryLine(0); !ok || string(line) != "line" {
		t.Fatalf("line 0 = %q, ready=%v; want ready line", string(line), ok)
	}
	if line, ok := buf.TryLine(7000); ok {
		t.Fatalf("expected far line to be deferred before indexing completes, got %q", string(line))
	}
	if buf.CanPrefetchQuick(7000, 20) {
		t.Fatalf("expected far viewport prefetch to be deferred before indexing completes")
	}

	buf.WaitForIndexing()

	if line, ok := buf.TryLine(7000); !ok || string(line) != "line" {
		t.Fatalf("line 7000 after indexing = %q, ready=%v; want ready line", string(line), ok)
	}
	if !buf.CanPrefetchQuick(7000, 20) {
		t.Fatalf("expected viewport prefetch to become available after indexing completes")
	}
}

func TestHugeFileOnDemandScanDensifiesAnchors(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	hugeFileInitialSampleBytes = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 200000; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	store := &stagedSlowTestFileStore{maxRead: 128, delay: 2 * time.Millisecond}
	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, store)
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	stopHugeFileBackgroundIndex(t, buf)

	targetRow := 90000
	beforeCheckpoint := buf.nearestCheckpoint(targetRow)
	beforeByteAnchor := buf.nearestByteAnchor(targetRow)
	beforePageAnchor := buf.nearestPageAnchor(targetRow)

	if got := string(buf.Line(targetRow)); got != "line" {
		t.Fatalf("line(%d) = %q, want %q", targetRow, got, "line")
	}

	afterCheckpoint := buf.nearestCheckpoint(targetRow)
	afterByteAnchor := buf.nearestByteAnchor(targetRow)
	afterPageAnchor := buf.nearestPageAnchor(targetRow)

	if afterCheckpoint.row <= beforeCheckpoint.row || afterCheckpoint.offset <= beforeCheckpoint.offset {
		t.Fatalf("expected checkpoint to advance after on-demand scan: before=%#v after=%#v", beforeCheckpoint, afterCheckpoint)
	}
	if afterByteAnchor.row <= beforeByteAnchor.row || afterByteAnchor.offset <= beforeByteAnchor.offset {
		t.Fatalf("expected byte anchor to advance after on-demand scan: before=%#v after=%#v", beforeByteAnchor, afterByteAnchor)
	}
	if afterPageAnchor.row <= beforePageAnchor.row || afterPageAnchor.offset <= beforePageAnchor.offset {
		t.Fatalf("expected page anchor to advance after on-demand scan: before=%#v after=%#v", beforePageAnchor, afterPageAnchor)
	}
}

func TestHugeFileScanStartAnchorUsesBestAnchor(t *testing.T) {
	buf := &HugeFileBuffer{
		checkpoints: []hugeFileCheckpoint{
			{row: 0, offset: 0},
			{row: 1024, offset: 8192},
		},
		byteAnchors: []hugeFileCheckpoint{
			{row: 0, offset: 0},
			{row: 1500, offset: 16384},
		},
		pageAnchors: []hugeFilePageAnchor{
			{row: 0, offset: 0, lineStart: 0},
			{row: 1400, offset: 12288, lineStart: 12240},
		},
	}

	anchor := buf.scanStartAnchor(1600)
	if anchor.row != 1500 || anchor.offset != 16384 || anchor.lineStart != 16384 {
		t.Fatalf("scanStartAnchor picked %#v, want byte-anchor-backed start", anchor)
	}
}

func TestHugeFileScanStartAnchorUsesCachedPageData(t *testing.T) {
	buf := &HugeFileBuffer{
		checkpoints: []hugeFileCheckpoint{
			{row: 0, offset: 0},
			{row: 1024, offset: 8192},
		},
		byteAnchors: []hugeFileCheckpoint{
			{row: 0, offset: 0},
			{row: 1500, offset: 16384},
		},
		pageAnchors: []hugeFilePageAnchor{
			{row: 0, offset: 0, lineStart: 0},
			{row: 1400, offset: 12288, lineStart: 12240},
		},
		pageData: map[int]hugeFileCachedPageData{
			1550: {
				startRow:    1550,
				endRow:      1560,
				startOffset: 20000,
				spans: []hugeFileLineSpan{
					{start: 20000, end: 20004},
					{start: 20005, end: 20009},
					{start: 20010, end: 20014},
					{start: 20015, end: 20019},
					{start: 20020, end: 20024},
					{start: 20025, end: 20029},
					{start: 20030, end: 20034},
					{start: 20035, end: 20039},
					{start: 20040, end: 20044},
					{start: 20045, end: 20049},
					{start: 20050, end: 20054},
				},
			},
		},
	}

	anchor := buf.scanStartAnchor(1700)
	if anchor.row != 1560 || anchor.offset != 20050 || anchor.lineStart != 20050 {
		t.Fatalf("scanStartAnchor picked %#v, want cached-page-backed start", anchor)
	}
}

func TestHugeFileBufferWarmLinesPopulatesFarViewportBeforeFullIndex(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	hugeFileInitialSampleBytes = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 200000; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, slowTestFileStore{maxRead: 128, delay: 2 * time.Millisecond})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if line, ok := buf.TryLine(7000); ok {
		t.Fatalf("expected far line to be deferred before warm, got %q", string(line))
	}

	buf.WarmLines(7000, 20)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		line, ok := buf.TryLine(7000)
		if ok && buf.CanPrefetchQuick(7000, 20) {
			if got := string(line); got != "line" {
				t.Fatalf("warmed line 7000 = %q, want %q", got, "line")
			}
			if buf.IndexingComplete() {
				t.Fatalf("expected warm path to finish before full indexing")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for far viewport warm")
}

func TestHugeFileWarmLinesReusesCachedPageData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 200; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	targetRow := 50
	if err := buf.PrefetchLines(targetRow, 12); err != nil {
		t.Fatalf("PrefetchLines returned error: %v", err)
	}
	page, ok := buf.cachedPageData(targetRow)
	if !ok {
		t.Fatalf("expected cached page data for row %d", targetRow)
	}

	buf.mu.Lock()
	buf.lineCache = map[int][]rune{}
	buf.cacheOrder = nil
	buf.mu.Unlock()

	buf.WarmLines(targetRow, 12)

	for row := page.startRow; row <= page.endRow; row++ {
		if !buf.hasCachedLine(row) {
			t.Fatalf("expected WarmLines to repopulate cached line for row %d in page window %d..%d", row, page.startRow, page.endRow)
		}
	}
}

func TestHugeFileUIPathsAvoidBlockingOnFarUnindexedRows(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	hugeFileInitialSampleBytes = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 200000; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, slowTestFileStore{maxRead: 128, delay: 2 * time.Millisecond}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}
	defer e.closeHugeFileBuffer()

	e.cursor.Row = 7000
	e.cursor.Col = 32
	e.viewport.scroll = 7000
	e.viewport.height = 20
	e.viewport.scrollX = 0

	start := time.Now()
	e.clampCursorCol()
	e.ensureCursorVisibleHorizontal(120, e.gutterWidth())
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("huge UI helpers blocked for %v on unresolved far row", elapsed)
	}
}

func TestLoadHugeFilePrimesRestoredViewport(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	hugeFileInitialSampleBytes = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 200000; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	e.SetSessionStore(&testSessionStore{
		states: map[string]FileState{
			path: {
				CursorRow: 7000,
				CursorCol: 0,
				ScrollY:   7000,
				ScrollX:   0,
				Mode:      "normal",
			},
		},
	})
	if err := e.LoadHugeFile(path, slowTestFileStore{maxRead: 128, delay: 2 * time.Millisecond}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}
	defer e.closeHugeFileBuffer()

	if e.cursor.Row != 7000 || e.viewport.scroll != 7000 {
		t.Fatalf("restored cursor/scroll = row %d scroll %d, want 7000/7000", e.cursor.Row, e.viewport.scroll)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		line, ok := e.huge.buffer.TryLine(7000)
		if ok && e.huge.buffer.CanPrefetchQuick(7000, 20) {
			if got := string(line); got != "line" {
				t.Fatalf("primed restored line = %q, want %q", got, "line")
			}
			if e.huge.buffer.IndexingComplete() {
				t.Fatalf("expected restored viewport warm to finish before full indexing")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for restored huge viewport warm")
}

func TestHugeFileBufferLoadsPersistentIndexCache(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	hugeFileInitialSampleBytes = 32
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
	}()

	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 12000; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	meta := FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}

	first, err := OpenHugeFileBuffer(path, meta, realTestFileStore{})
	if err != nil {
		t.Fatalf("first open huge file buffer: %v", err)
	}
	first.WaitForIndexing()
	if !first.IndexingComplete() {
		t.Fatalf("expected first buffer to finish indexing")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first buffer: %v", err)
	}

	cachePath := hugeFileIndexCachePath(path)
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}

	second, err := OpenHugeFileBuffer(path, meta, slowTestFileStore{maxRead: 128, delay: time.Millisecond})
	if err != nil {
		t.Fatalf("second open huge file buffer: %v", err)
	}
	defer second.Close()

	if !second.IndexingComplete() {
		t.Fatalf("expected cached open to skip background indexing")
	}
	if got := second.LineCount(); got != 12001 {
		t.Fatalf("cached line count = %d, want %d", got, 12001)
	}
	if got := string(second.Line(9000)); got != "line" {
		t.Fatalf("cached line 9000 = %q, want %q", got, "line")
	}
}

func TestHugeFileBufferLoadsPartialIndexCacheAndResumes(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	prevPersistInterval := hugeFileIndexPersistInterval
	hugeFileInitialSampleBytes = 64
	hugeFileIndexPersistInterval = 10 * time.Millisecond
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
		hugeFileIndexPersistInterval = prevPersistInterval
	}()

	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 200000; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	meta := FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}

	first, err := OpenHugeFileBuffer(path, meta, slowTestFileStore{maxRead: 128, delay: 2 * time.Millisecond})
	if err != nil {
		t.Fatalf("first open huge file buffer: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if err := first.Close(); err != nil {
		t.Fatalf("close first buffer: %v", err)
	}

	cachePath := hugeFileIndexCachePath(path)
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("partial cache file not written: %v", err)
	}

	second, err := OpenHugeFileBuffer(path, meta, slowTestFileStore{maxRead: 128, delay: 2 * time.Millisecond})
	if err != nil {
		t.Fatalf("second open huge file buffer: %v", err)
	}
	defer second.Close()

	if second.IndexingComplete() {
		t.Fatalf("expected partial cache reopen to resume indexing instead of finishing immediately")
	}
	if got := second.IndexedLineCount(); got <= 20 {
		t.Fatalf("indexed line count from partial cache = %d, want progress beyond initial sample", got)
	}
}

func TestHugeFileBufferPrefersPageAnchorsForLongLines(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	prevAnchorSpacing := hugeFileByteAnchorSpacingOverride
	hugeFileInitialSampleBytes = 0
	hugeFileByteAnchorSpacingOverride = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
		hugeFileByteAnchorSpacingOverride = prevAnchorSpacing
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 20; i++ {
		content.WriteString(strings.Repeat("x", 120))
		content.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	checkpoint := buf.nearestCheckpoint(10)
	pageAnchor := buf.nearestPageAnchor(10)
	scanAnchor := buf.scanStartAnchor(10)

	if checkpoint.offset != 0 {
		t.Fatalf("checkpoint offset = %d, want 0 without intermediate line checkpoints", checkpoint.offset)
	}
	if pageAnchor.offset <= 0 {
		t.Fatalf("expected page anchors to advance past offset 0")
	}
	if scanAnchor.offset != pageAnchor.offset {
		t.Fatalf("scan anchor offset = %d, want page anchor offset %d", scanAnchor.offset, pageAnchor.offset)
	}
	if scanAnchor.lineStart != pageAnchor.lineStart {
		t.Fatalf("scan anchor lineStart = %d, want page anchor lineStart %d", scanAnchor.lineStart, pageAnchor.lineStart)
	}
}

func TestHugeFileResolveLineSpanCachesWholePageWindow(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	prevAnchorSpacing := hugeFileByteAnchorSpacingOverride
	hugeFileInitialSampleBytes = 0
	hugeFileByteAnchorSpacingOverride = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
		hugeFileByteAnchorSpacingOverride = prevAnchorSpacing
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 80; i++ {
		content.WriteString("line-")
		content.WriteString(strconv.Itoa(i))
		content.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	targetRow := 25
	start := buf.nearestPageAnchor(targetRow)
	end, ok := buf.nextPageAnchor(targetRow)
	if !ok {
		t.Fatalf("expected next page anchor for row %d", targetRow)
	}
	if _, err := buf.resolveLineSpan(targetRow); err != nil {
		t.Fatalf("resolveLineSpan returned error: %v", err)
	}
	for row := start.row; row <= end.row; row++ {
		if !buf.hasCachedLineSpan(row) {
			t.Fatalf("expected cached line span for row %d in page window %d..%d", row, start.row, end.row)
		}
	}
}

func TestHugeFileLineCachesWholePageWindow(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	prevAnchorSpacing := hugeFileByteAnchorSpacingOverride
	hugeFileInitialSampleBytes = 0
	hugeFileByteAnchorSpacingOverride = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
		hugeFileByteAnchorSpacingOverride = prevAnchorSpacing
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 80; i++ {
		content.WriteString("row-")
		content.WriteString(strconv.Itoa(i))
		content.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	targetRow := 25
	start := buf.nearestPageAnchor(targetRow)
	end, ok := buf.nextPageAnchor(targetRow)
	if !ok {
		t.Fatalf("expected next page anchor for row %d", targetRow)
	}

	if got := string(buf.Line(targetRow)); got != "row-25" {
		t.Fatalf("line(%d) = %q, want %q", targetRow, got, "row-25")
	}
	for row := start.row; row <= end.row; row++ {
		if !buf.hasCachedLine(row) {
			t.Fatalf("expected cached line for row %d in page window %d..%d", row, start.row, end.row)
		}
	}
}

func TestHugeFilePageDataSurvivesLineCacheEviction(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	prevAnchorSpacing := hugeFileByteAnchorSpacingOverride
	hugeFileInitialSampleBytes = 0
	hugeFileByteAnchorSpacingOverride = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
		hugeFileByteAnchorSpacingOverride = prevAnchorSpacing
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 80; i++ {
		content.WriteString("row-")
		content.WriteString(strconv.Itoa(i))
		content.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	targetRow := 25
	if got := string(buf.Line(targetRow)); got != "row-25" {
		t.Fatalf("line(%d) = %q, want %q", targetRow, got, "row-25")
	}
	if _, ok := buf.cachedPageData(targetRow); !ok {
		t.Fatalf("expected cached page data for row %d", targetRow)
	}
	buf.mu.Lock()
	buf.lineCache = map[int][]rune{}
	buf.cacheOrder = nil
	buf.mu.Unlock()

	if got := string(buf.Line(targetRow)); got != "row-25" {
		t.Fatalf("line(%d) after line cache eviction = %q, want %q", targetRow, got, "row-25")
	}
	start := buf.nearestPageAnchor(targetRow)
	end, ok := buf.nextPageAnchor(targetRow)
	if !ok {
		t.Fatalf("expected next page anchor for row %d", targetRow)
	}
	for row := start.row; row <= end.row; row++ {
		if !buf.hasCachedLine(row) {
			t.Fatalf("expected repopulated cached line for row %d in page window %d..%d", row, start.row, end.row)
		}
	}
}

func TestHugeFileCachesLineEndingsByPageWindow(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	prevAnchorSpacing := hugeFileByteAnchorSpacingOverride
	hugeFileInitialSampleBytes = 0
	hugeFileByteAnchorSpacingOverride = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
		hugeFileByteAnchorSpacingOverride = prevAnchorSpacing
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\r\nbeta\r\ngamma\nomega\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if got := string(buf.Line(1)); got != "beta" {
		t.Fatalf("line(1) = %q, want %q", got, "beta")
	}
	if got := buf.LineEnding(0); got != "\r\n" {
		t.Fatalf("LineEnding(0) = %q, want CRLF", got)
	}
	if got := buf.LineEnding(1); got != "\r\n" {
		t.Fatalf("LineEnding(1) = %q, want CRLF", got)
	}
	if got := buf.LineEnding(2); got != "\n" {
		t.Fatalf("LineEnding(2) = %q, want LF", got)
	}
	if got, ok := buf.cachedLineEnding(1); !ok || got != "\r\n" {
		t.Fatalf("cached LineEnding(1) = %q ok=%v, want CRLF/true", got, ok)
	}
}

func TestHugeFilePageDataSurvivesLineEndingCacheEviction(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	prevAnchorSpacing := hugeFileByteAnchorSpacingOverride
	hugeFileInitialSampleBytes = 0
	hugeFileByteAnchorSpacingOverride = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
		hugeFileByteAnchorSpacingOverride = prevAnchorSpacing
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	content := "alpha\r\nbeta\r\ngamma\nomega\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	if got := buf.LineEnding(1); got != "\r\n" {
		t.Fatalf("LineEnding(1) = %q, want CRLF", got)
	}
	if _, ok := buf.cachedPageData(1); !ok {
		t.Fatalf("expected cached page data for row 1")
	}
	buf.mu.Lock()
	buf.lineEndings = map[int]string{}
	buf.endingOrder = nil
	buf.mu.Unlock()

	if got := buf.LineEnding(1); got != "\r\n" {
		t.Fatalf("LineEnding(1) after ending cache eviction = %q, want CRLF", got)
	}
	if got, ok := buf.cachedLineEnding(0); !ok || got != "\r\n" {
		t.Fatalf("expected repopulated cached LineEnding(0) = CRLF/true, got %q ok=%v", got, ok)
	}
	if got, ok := buf.cachedLineEnding(1); !ok || got != "\r\n" {
		t.Fatalf("expected repopulated cached LineEnding(1) = CRLF/true, got %q ok=%v", got, ok)
	}
	if got, ok := buf.cachedLineEnding(2); !ok || got != "\n" {
		t.Fatalf("expected repopulated cached LineEnding(2) = LF/true, got %q ok=%v", got, ok)
	}
}

func TestHugeFilePageDataSurvivesSpanCacheEviction(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	prevAnchorSpacing := hugeFileByteAnchorSpacingOverride
	hugeFileInitialSampleBytes = 0
	hugeFileByteAnchorSpacingOverride = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
		hugeFileByteAnchorSpacingOverride = prevAnchorSpacing
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 80; i++ {
		content.WriteString("row-")
		content.WriteString(strconv.Itoa(i))
		content.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	buf, err := OpenHugeFileBuffer(path, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, realTestFileStore{})
	if err != nil {
		t.Fatalf("open huge file buffer: %v", err)
	}
	defer buf.Close()

	targetRow := 25
	if got := string(buf.Line(targetRow)); got != "row-25" {
		t.Fatalf("line(%d) = %q, want %q", targetRow, got, "row-25")
	}
	if _, ok := buf.cachedPageData(targetRow); !ok {
		t.Fatalf("expected cached page data for row %d", targetRow)
	}

	buf.mu.Lock()
	buf.lineSpans = map[int]hugeFileLineSpan{}
	buf.spanOrder = nil
	buf.mu.Unlock()

	span, err := buf.resolveLineSpan(targetRow)
	if err != nil {
		t.Fatalf("resolveLineSpan(%d) after span cache eviction: %v", targetRow, err)
	}
	if span.end <= span.start {
		t.Fatalf("resolveLineSpan(%d) returned invalid span: %#v", targetRow, span)
	}
	if got := string(buf.Line(targetRow)); got != "row-25" {
		t.Fatalf("line(%d) after span cache eviction = %q, want %q", targetRow, got, "row-25")
	}
	if !buf.hasCachedLineSpan(targetRow) {
		t.Fatalf("expected restored cached span for row %d", targetRow)
	}

	start := buf.nearestPageAnchor(targetRow)
	end, ok := buf.nextPageAnchor(targetRow)
	if !ok {
		t.Fatalf("expected next page anchor for row %d", targetRow)
	}
	if !buf.hasCachedLineSpans(start.row, end.row) {
		t.Fatalf("expected page cache to satisfy span coverage for rows %d..%d", start.row, end.row)
	}
}

func TestHugeFileGotoLinePrimesTargetRegion(t *testing.T) {
	prevSampleBytes := hugeFileInitialSampleBytes
	hugeFileInitialSampleBytes = 64
	defer func() {
		hugeFileInitialSampleBytes = prevSampleBytes
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	var content strings.Builder
	for i := 0; i < 200000; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	e := newTestEditor()
	if err := e.LoadHugeFile(path, slowTestFileStore{maxRead: 128, delay: 2 * time.Millisecond}, FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}
	defer e.closeHugeFileBuffer()

	e.gotoLineNumber(7001)
	if e.cursor.Row != 7000 {
		t.Fatalf("cursor row after goto = %d, want 7000", e.cursor.Row)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		line, ok := e.huge.buffer.TryLine(7000)
		if ok && e.huge.buffer.CanPrefetchQuick(7000, hugeFilePrimeViewportLines) {
			if got := string(line); got != "line" {
				t.Fatalf("primed goto line = %q, want %q", got, "line")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for goto line prefetch")
}

func TestEffectiveHugeFileByteAnchorSpacingScalesWithFileSize(t *testing.T) {
	prevOverride := hugeFileByteAnchorSpacingOverride
	prevMin := hugeFileMinByteAnchorSpacing
	prevMax := hugeFileMaxByteAnchorSpacing
	prevTarget := hugeFileTargetByteAnchors
	hugeFileByteAnchorSpacingOverride = 0
	hugeFileMinByteAnchorSpacing = 64 << 10
	hugeFileMaxByteAnchorSpacing = 4 << 20
	hugeFileTargetByteAnchors = 16384
	defer func() {
		hugeFileByteAnchorSpacingOverride = prevOverride
		hugeFileMinByteAnchorSpacing = prevMin
		hugeFileMaxByteAnchorSpacing = prevMax
		hugeFileTargetByteAnchors = prevTarget
	}()

	small := effectiveHugeFileByteAnchorSpacing(1 << 20)
	medium := effectiveHugeFileByteAnchorSpacing(4 << 30)
	huge := effectiveHugeFileByteAnchorSpacing(128 << 30)

	if small != hugeFileMinByteAnchorSpacing {
		t.Fatalf("small spacing = %d, want min spacing %d", small, hugeFileMinByteAnchorSpacing)
	}
	if medium < small {
		t.Fatalf("medium spacing = %d, want >= small spacing %d", medium, small)
	}
	if huge < medium {
		t.Fatalf("huge spacing = %d, want >= medium spacing %d", huge, medium)
	}
	if huge > hugeFileMaxByteAnchorSpacing {
		t.Fatalf("huge spacing = %d, want <= max spacing %d", huge, hugeFileMaxByteAnchorSpacing)
	}
	if medium%hugeFileMinByteAnchorSpacing != 0 {
		t.Fatalf("medium spacing = %d, want multiple of min spacing %d", medium, hugeFileMinByteAnchorSpacing)
	}
}
