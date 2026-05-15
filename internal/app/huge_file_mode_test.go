package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/integrations"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

func TestOpenRuntimeFileUsesHugeFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevThreshold := hugeFileThresholdBytes
	hugeFileThresholdBytes = 4
	defer func() {
		hugeFileThresholdBytes = prevThreshold
	}()

	ed := editor.New(editor.Options{})
	state, err := openRuntimeFile(ed, nil, nil, nil, config.Languages{}, integrations.FileStore{}, path, 0)
	if err != nil {
		t.Fatalf("openRuntimeFile returned error: %v", err)
	}

	if state.openPath != path {
		t.Fatalf("open path = %q, want %q", state.openPath, path)
	}
	if !ed.HugeFileMode() {
		t.Fatalf("expected editor to enter huge file mode")
	}
	if got := ed.HugeFileKind(); got != editor.HugeFileKindLargeFile {
		t.Fatalf("huge file kind = %q, want %q", got, editor.HugeFileKindLargeFile)
	}
	if ed.LineCount() != 4 {
		t.Fatalf("line count = %d, want 4", ed.LineCount())
	}
}

func TestOpenRuntimeFileUsesHugeModeForLongLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "long-line.js")
	if err := os.WriteFile(path, []byte(strings.Repeat("a", 300000)+"\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevSizeThreshold := hugeFileThresholdBytes
	prevLongLineThreshold := hugeFileLongLineThresholdBytes
	prevSampleBytes := hugeFileLongLineSampleBytes
	hugeFileThresholdBytes = 64 << 20
	hugeFileLongLineThresholdBytes = 128 << 10
	hugeFileLongLineSampleBytes = 1 << 20
	defer func() {
		hugeFileThresholdBytes = prevSizeThreshold
		hugeFileLongLineThresholdBytes = prevLongLineThreshold
		hugeFileLongLineSampleBytes = prevSampleBytes
	}()

	ed := editor.New(editor.Options{})
	state, err := openRuntimeFile(ed, nil, nil, nil, config.Languages{}, integrations.FileStore{}, path, 0)
	if err != nil {
		t.Fatalf("openRuntimeFile returned error: %v", err)
	}

	if state.openPath != path {
		t.Fatalf("open path = %q, want %q", state.openPath, path)
	}
	if !ed.HugeFileMode() {
		t.Fatalf("expected editor to enter huge file mode for long line")
	}
	if got := ed.HugeFileKind(); got != editor.HugeFileKindLongLine {
		t.Fatalf("huge file kind = %q, want %q", got, editor.HugeFileKindLongLine)
	}
}

func TestHugeModeAsyncHighlightsSmallLongLineFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mermaid.min.js")
	content := "const mermaid = \"" + strings.Repeat("a", 150000) + "\";\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevSizeThreshold := hugeFileThresholdBytes
	prevLongLineThreshold := hugeFileLongLineThresholdBytes
	prevSampleBytes := hugeFileLongLineSampleBytes
	hugeFileThresholdBytes = 64 << 20
	hugeFileLongLineThresholdBytes = 128 << 10
	hugeFileLongLineSampleBytes = 1 << 20
	defer func() {
		hugeFileThresholdBytes = prevSizeThreshold
		hugeFileLongLineThresholdBytes = prevLongLineThreshold
		hugeFileLongLineSampleBytes = prevSampleBytes
	}()

	langs := config.Languages{
		Languages: []config.Language{
			{Name: "javascript", FileTypes: []string{"js"}},
		},
	}
	ts := treesitter.New(langs)
	if err := ts.Start(); err != nil {
		t.Fatalf("start treesitter: %v", err)
	}
	defer func() { _ = ts.Stop() }()

	ed := editor.New(editor.Options{})
	state, err := openRuntimeFile(ed, nil, nil, ts, langs, integrations.FileStore{}, path, 8<<20)
	if err != nil {
		t.Fatalf("openRuntimeFile returned error: %v", err)
	}
	if !ed.HugeFileMode() {
		t.Fatalf("expected huge mode for long line file")
	}
	if !state.highlightEnabled || !state.highlightExpected || state.langName != "javascript" {
		t.Fatalf("unexpected highlight state: enabled=%v expected=%v lang=%q", state.highlightEnabled, state.highlightExpected, state.langName)
	}

	lastTick := state.lastChangeTick
	lastStart := state.lastHighlightStart
	lastEnd := state.lastHighlightEnd
	rtState := newEditorRuntimeState(ed)
	rtState.applyActiveFile(state)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		lastTick, lastStart, lastEnd = syncVisibleHighlights(ed, ts, &rtState, path, state.langName, state.highlightEnabled, lastTick, lastStart, lastEnd)
		if ed.HasHighlights() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected async highlights for huge long line file")
}

func TestHugeModeRequeriesCoveredLongLineAtFarColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mermaid.min.js")
	line := `const banner = "` + strings.Repeat("a", 150000) + `";var tail=1;`
	content := line + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	langs := config.Languages{
		Languages: []config.Language{
			{Name: "javascript", FileTypes: []string{"js"}},
		},
	}
	ts := treesitter.New(langs)
	if !ts.ParseSync(path, "javascript", content) {
		t.Fatalf("ParseSync failed")
	}

	store := integrations.FileStore{}
	info, err := store.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	ed := editor.New(editor.Options{LineNumbers: "off"})
	ed.SetStyles(appTestEditorStyles())
	if err := ed.LoadHugeFile(path, store, info); err != nil {
		t.Fatalf("LoadHugeFile returned error: %v", err)
	}

	screen := newAppTestScreen(96, 12)
	ed.Render(screen)
	ed.JumpToLocation(0, len([]rune(line)))
	ed.Render(screen)

	start, end := ed.VisibleRange()
	colStart, colEnd := ed.HighlightWindowCols()
	if colStart < 100000 {
		t.Fatalf("expected far horizontal window, got [%d,%d)", colStart, colEnd)
	}

	// Reproduce the stale-cache shape from huge-mode background highlighting:
	// the visible line range is marked as covered, but only for an old column
	// window near the start of a very long line.
	ed.MergeHighlights(start, end, map[int][]editor.HighlightSpan{})
	ed.SetHighlightColumns(0, 128)

	rtState := newEditorRuntimeState(ed)
	rtState.openPath = path
	rtState.langName = "javascript"
	rtState.highlightEnabled = true
	rtState.highlightParsed = true

	lastTick := ed.ChangeTick()
	lastStart := -1
	lastEnd := -1
	seqBefore := rtState.highlightJobSeq
	lastTick, lastStart, lastEnd = syncVisibleHighlights(ed, ts, &rtState, path, "javascript", true, lastTick, lastStart, lastEnd)
	if rtState.highlightJobSeq == seqBefore || !rtState.highlightJobActive {
		t.Fatalf("expected a new highlight job for far column window; seq before=%d after=%d active=%v", seqBefore, rtState.highlightJobSeq, rtState.highlightJobActive)
	}

	seqAfterLaunch := rtState.highlightJobSeq
	lastTick, lastStart, lastEnd = syncVisibleHighlights(ed, ts, &rtState, path, "javascript", true, lastTick, lastStart, lastEnd)
	if rtState.highlightJobSeq != seqAfterLaunch {
		t.Fatalf("highlight job relaunched while matching column window was pending: before=%d after=%d", seqAfterLaunch, rtState.highlightJobSeq)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		lastTick, lastStart, lastEnd = syncVisibleHighlights(ed, ts, &rtState, path, "javascript", true, lastTick, lastStart, lastEnd)
		ed.Render(screen)
		if screen.rowHasNonMainText(0, appTestMainFG) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected rendered syntax style at far column after async highlight job")
}

func TestHugeModeStartsLSPForJavaScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mermaid.min.js")
	content := []byte("const mermaid = \"" + strings.Repeat("a", 150000) + "\";\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevSizeThreshold := hugeFileThresholdBytes
	prevLongLineThreshold := hugeFileLongLineThresholdBytes
	prevSampleBytes := hugeFileLongLineSampleBytes
	hugeFileThresholdBytes = 64 << 20
	hugeFileLongLineThresholdBytes = 128 << 10
	hugeFileLongLineSampleBytes = 1 << 20
	defer func() {
		hugeFileThresholdBytes = prevSizeThreshold
		hugeFileLongLineThresholdBytes = prevLongLineThreshold
		hugeFileLongLineSampleBytes = prevSampleBytes
	}()

	outPath := filepath.Join(dir, "lsp-events.txt")
	t.Setenv("QEDIT_APP_LSP_HELPER", "1")
	t.Setenv("QEDIT_APP_LSP_OUT", outPath)

	langs := config.Languages{
		Languages: []config.Language{
			{
				Name:            "javascript",
				FileTypes:       []string{"js"},
				LanguageServers: []string{"helper"},
			},
		},
		LanguageServers: map[string]config.LanguageServer{
			"helper": {
				Command: os.Args[0],
				Args:    []string{"-test.run=TestAppLSPServerHelper", "--"},
			},
		},
	}

	manager := lsp.NewManager(langs)
	if err := manager.Start(); err != nil {
		t.Fatalf("start lsp manager: %v", err)
	}
	defer func() { _ = manager.Stop() }()

	ed := editor.New(editor.Options{})
	state, err := openRuntimeFile(ed, nil, manager, nil, langs, integrations.FileStore{}, path, 0)
	if err != nil {
		t.Fatalf("openRuntimeFile returned error: %v", err)
	}
	if state.openPath != path {
		t.Fatalf("open path = %q, want %q", state.openPath, path)
	}
	if !ed.HugeFileMode() {
		t.Fatalf("expected huge mode for long line js file")
	}

	text := waitForAppHelperOutput(t, outPath, 3*time.Second)
	if !strings.Contains(text, "initialize") {
		t.Fatalf("missing initialize in %q", text)
	}
	if !strings.Contains(text, "textDocument/didOpen") {
		t.Fatalf("missing didOpen in %q", text)
	}
}

const appTestMainFG editor.Color = 1

type appTestStyle struct {
	fg    editor.Color
	bg    editor.Color
	attrs editor.AttrMask
}

func (s appTestStyle) Decompose() (editor.Color, editor.Color, editor.AttrMask) {
	return s.fg, s.bg, s.attrs
}

func (s appTestStyle) Foreground(c editor.Color) editor.Style {
	s.fg = c
	return s
}

func (s appTestStyle) Background(c editor.Color) editor.Style {
	s.bg = c
	return s
}

type appTestCell struct {
	ch    rune
	style editor.Style
}

type appTestScreen struct {
	w     int
	h     int
	cells map[[2]int]appTestCell
}

func newAppTestScreen(w, h int) *appTestScreen {
	return &appTestScreen{w: w, h: h, cells: make(map[[2]int]appTestCell)}
}

func (s *appTestScreen) Size() (int, int)      { return s.w, s.h }
func (s *appTestScreen) SetStyle(editor.Style) {}
func (s *appTestScreen) Clear() {
	clear(s.cells)
}
func (s *appTestScreen) SetContent(x, y int, ch rune, _ []rune, style editor.Style) {
	if x < 0 || x >= s.w || y < 0 || y >= s.h {
		return
	}
	s.cells[[2]int{x, y}] = appTestCell{ch: ch, style: style}
}
func (s *appTestScreen) Show()                             {}
func (s *appTestScreen) ShowCursor(int, int)               {}
func (s *appTestScreen) HideCursor()                       {}
func (s *appTestScreen) SetCursorStyle(editor.CursorStyle) {}

func (s *appTestScreen) rowHasNonMainText(y int, mainFG editor.Color) bool {
	for x := 0; x < s.w; x++ {
		cell, ok := s.cells[[2]int{x, y}]
		if !ok || cell.ch == ' ' || cell.style == nil {
			continue
		}
		fg, _, _ := cell.style.Decompose()
		if fg != mainFG {
			return true
		}
	}
	return false
}

func appTestEditorStyles() editor.EditorStyles {
	main := appTestStyle{fg: appTestMainFG}
	lineNumber := appTestStyle{fg: 2}
	status := appTestStyle{fg: 3}
	selection := appTestStyle{fg: appTestMainFG, bg: 4}
	keyword := appTestStyle{fg: 10}
	stringStyle := appTestStyle{fg: 11}
	comment := appTestStyle{fg: 12}
	typeStyle := appTestStyle{fg: 13}
	function := appTestStyle{fg: 14}
	number := appTestStyle{fg: 15}
	constant := appTestStyle{fg: 16}
	operator := appTestStyle{fg: 17}
	punctuation := appTestStyle{fg: 18}
	field := appTestStyle{fg: 19}
	builtin := appTestStyle{fg: 20}
	variable := appTestStyle{fg: 21}
	parameter := appTestStyle{fg: 22}
	return editor.EditorStyles{
		Main:              main,
		Status:            status,
		StatusWarning:     status,
		Command:           status,
		LineNumber:        lineNumber,
		LineNumberActive:  lineNumber,
		Selection:         selection,
		SearchMatch:       selection,
		SyntaxKeyword:     keyword,
		SyntaxString:      stringStyle,
		SyntaxComment:     comment,
		SyntaxType:        typeStyle,
		SyntaxFunction:    function,
		SyntaxNumber:      number,
		SyntaxConstant:    constant,
		SyntaxOperator:    operator,
		SyntaxPunctuation: punctuation,
		SyntaxField:       field,
		SyntaxBuiltin:     builtin,
		SyntaxUnknown:     main,
		SyntaxVariable:    variable,
		SyntaxParameter:   parameter,
		TableBorder:       appTestStyle{fg: 23},
		Branch:            status,
		MainBranch:        status,
		LayoutUS:          status,
		LayoutRU:          status,
		LayoutOther:       status,
		AutoComplete:      status,
		ScrollIndicator:   status,
		BoxBorder:         status,
	}
}

func TestAppLSPServerHelper(t *testing.T) {
	if os.Getenv("QEDIT_APP_LSP_HELPER") != "1" {
		return
	}
	outPath := os.Getenv("QEDIT_APP_LSP_OUT")
	if outPath == "" {
		os.Exit(2)
	}
	time.AfterFunc(2*time.Second, func() {
		os.Exit(2)
	})

	reader := bufio.NewReader(os.Stdin)
	var methods []string
	for {
		msg, err := lspReadMessage(reader)
		if err != nil {
			break
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(msg, &envelope); err != nil {
			continue
		}
		rawMethod, ok := envelope["method"]
		if !ok {
			continue
		}
		var method string
		if err := json.Unmarshal(rawMethod, &method); err != nil {
			continue
		}
		methods = append(methods, method)
		if method == "initialize" {
			var id int
			_ = json.Unmarshal(envelope["id"], &id)
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result":  map[string]any{"capabilities": map[string]any{}},
			}
			payload, err := json.Marshal(resp)
			if err == nil {
				header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
				_, _ = os.Stdout.Write([]byte(header))
				_, _ = os.Stdout.Write(payload)
			}
		}
		if method == "textDocument/didOpen" && len(methods) >= 3 {
			break
		}
	}
	_ = os.WriteFile(outPath, []byte(strings.Join(methods, "\n")), 0o644)
}

func waitForAppHelperOutput(t *testing.T, path string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			return string(b)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no helper output")
	return ""
}

func lspReadMessage(r *bufio.Reader) ([]byte, error) {
	headers := map[string]string{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	var n int
	if _, err := fmt.Sscanf(headers["Content-Length"], "%d", &n); err != nil || n < 0 {
		return nil, fmt.Errorf("bad content length")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
