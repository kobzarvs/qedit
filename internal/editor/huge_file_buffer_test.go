package editor

import (
	"io"
	"os"
	"path/filepath"
	"strings"
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
