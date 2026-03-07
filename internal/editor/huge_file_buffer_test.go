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

func TestLoadHugeFileEntersReadOnlyMode(t *testing.T) {
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
	if !e.file.readOnly {
		t.Fatalf("expected huge file to be read-only")
	}
	if got := e.LineCount(); got != 4 {
		t.Fatalf("line count = %d, want 4", got)
	}
	if got := string(e.line(1)); got != "beta" {
		t.Fatalf("line 1 = %q, want %q", got, "beta")
	}

	e.SetBehaviorProfile(BehaviorProfileBasic)
	e.HandleKey(keyRune('x'))
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want %v", e.mode, ModeNormal)
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
