package treesitter

import (
	"strings"
	"testing"
	"time"

	"github.com/kobzarvs/qedit/internal/config"
)

func TestEngineOpenFileParseEvent(t *testing.T) {
	langs := config.Languages{
		Languages: []config.Language{
			{Name: "go", FileTypes: []string{"go"}},
		},
	}
	e := New(langs)
	if err := e.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer func() { _ = e.Stop() }()

	e.OpenFile("main.go", "package main\nfunc main(){}\n")
	select {
	case ev := <-e.Events():
		if ev.Kind != "parsed" {
			t.Fatalf("event kind = %q, want %q", ev.Kind, "parsed")
		}
		if ev.Path != "main.go" {
			t.Fatalf("event path = %q, want %q", ev.Path, "main.go")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for parse event")
	}
}

func TestEngineOpenFileUnknown(t *testing.T) {
	langs := config.Languages{
		Languages: []config.Language{
			{Name: "go", FileTypes: []string{"go"}},
		},
	}
	e := New(langs)
	if err := e.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer func() { _ = e.Stop() }()

	e.OpenFile("README.md", "hello")
	select {
	case ev := <-e.Events():
		t.Fatalf("unexpected event: %#v", ev)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestMarkdownHighlightRuneColumns(t *testing.T) {
	langs := config.Languages{
		Languages: []config.Language{
			{Name: "markdown", FileTypes: []string{"md"}},
		},
	}
	e := New(langs)
	if err := e.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer func() { _ = e.Stop() }()

	text := "Функция `SetCurrent`."
	path := "note.md"
	if ok := e.ParseSync(path, "markdown", text); !ok {
		t.Fatalf("ParseSync failed")
	}
	spans := e.Highlights(path, 0, 0)
	if spans == nil {
		t.Fatalf("expected spans, got nil")
	}
	lineSpans := spans[0]
	if len(lineSpans) == 0 {
		t.Fatalf("expected highlight spans for line 0")
	}

	codeStart := runeIndex(text, "SetCurrent")
	if codeStart < 0 {
		t.Fatalf("failed to locate code span start")
	}
	if !spanCoversKind(lineSpans, codeStart, "string") {
		t.Fatalf("expected code span highlight at rune col %d", codeStart)
	}
}

func TestJavaScriptHighlightsWindowClipsColumns(t *testing.T) {
	langs := config.Languages{
		Languages: []config.Language{
			{Name: "javascript", FileTypes: []string{"js"}},
		},
	}
	e := New(langs)
	if err := e.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer func() { _ = e.Stop() }()

	text := "const mermaid=foo(bar)+baz(qux)+quux;"
	path := "sample.js"
	if ok := e.ParseSync(path, "javascript", text); !ok {
		t.Fatalf("ParseSync failed")
	}
	spans := e.HighlightsWindow(path, 0, 0, 18, 28)
	if spans == nil {
		t.Fatalf("expected spans, got nil")
	}
	lineSpans := spans[0]
	if len(lineSpans) == 0 {
		t.Fatalf("expected clipped highlight spans")
	}
	for _, span := range lineSpans {
		if span.StartCol < 18 || span.EndCol > 28 {
			t.Fatalf("span %#v escapes requested window", span)
		}
	}
}

func runeIndex(haystack, needle string) int {
	hr := []rune(haystack)
	nr := []rune(needle)
	if len(nr) == 0 || len(hr) < len(nr) {
		return -1
	}
	for i := 0; i+len(nr) <= len(hr); i++ {
		match := true
		for j := range nr {
			if hr[i+j] != nr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func TestLongLinePerLineQuery(t *testing.T) {
	langs := config.Languages{
		Languages: []config.Language{
			{Name: "javascript", FileTypes: []string{"js"}},
		},
	}
	e := New(langs)
	if err := e.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer func() { _ = e.Stop() }()

	// Build a multi-line file where one line is very long (> 4096 bytes).
	shortLine := "var x = 1;\n"
	longLine := "var longVar=" + strings.Repeat("foo()+", 1000) + "bar;\n"
	if len(longLine) < 4096 {
		t.Fatalf("long line should exceed threshold, got %d", len(longLine))
	}
	text := shortLine + longLine + shortLine
	path := "longline.js"
	if ok := e.ParseSync(path, "javascript", text); !ok {
		t.Fatalf("ParseSync failed")
	}
	// Query all 3 lines with column window clipping.
	spans := e.HighlightsWindow(path, 0, 2, 0, 20)
	if spans == nil {
		t.Fatalf("expected spans, got nil")
	}
	// The long line (row 1) should only have spans within [0, 20).
	for _, span := range spans[1] {
		if span.StartCol >= 20 {
			t.Fatalf("long line span %#v should be clipped to [0,20)", span)
		}
	}
	// Short lines (row 0, 2) should have spans too.
	if len(spans[0]) == 0 {
		t.Fatalf("expected spans for short line 0")
	}
}

func TestLongLinePerLineQueryNonASCIIFile(t *testing.T) {
	langs := config.Languages{
		Languages: []config.Language{
			{Name: "javascript", FileTypes: []string{"js"}},
		},
	}
	e := New(langs)
	if err := e.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer func() { _ = e.Stop() }()

	// File where one line has non-ASCII (making asciiOnly=false globally)
	// but the long line is pure ASCII — per-line ASCII should still optimize it.
	unicodeLine := "var name = '\u0424\u0443\u043d\u043a\u0446\u0438\u044f';\n"
	longLine := "var longVar=" + strings.Repeat("foo()+", 1000) + "bar;\n"
	shortLine := "var x = 1;\n"
	text := unicodeLine + longLine + shortLine
	path := "mixed.js"
	if ok := e.ParseSync(path, "javascript", text); !ok {
		t.Fatalf("ParseSync failed")
	}
	// Query all 3 lines with column window in the middle of the long line.
	spans := e.HighlightsWindow(path, 0, 2, 100, 200)
	if spans == nil {
		t.Fatalf("expected spans, got nil")
	}
	// The long line (row 1) should have spans clipped to [100, 200).
	for _, span := range spans[1] {
		if span.StartCol >= 200 {
			t.Fatalf("long line span %#v should be clipped to [100,200)", span)
		}
	}
}

func TestMultiLineTemplateStringHighlightsCrossChunkBoundary(t *testing.T) {
	langs := config.Languages{
		Languages: []config.Language{
			{Name: "javascript", FileTypes: []string{"js"}},
		},
	}
	e := New(langs)
	if err := e.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer func() { _ = e.Stop() }()

	// Build a file where a template literal starts at line 0 and spans many
	// lines.  If we query starting at line 5, the template_string node (which
	// starts at line 0) must still be captured — otherwise the interior lines
	// get no "string" highlight and render as styleSyntaxUnknown (red).
	var sb strings.Builder
	sb.WriteString("const css = `\n")       // line 0: template start
	for i := 0; i < 10; i++ {
		sb.WriteString("  .class" + string(rune('a'+i)) + " { color: red; }\n") // lines 1-10
	}
	sb.WriteString("`;\n")                   // line 11: template end
	sb.WriteString("const x = 42;\n")        // line 12

	path := "tpl.js"
	text := sb.String()
	if ok := e.ParseSync(path, "javascript", text); !ok {
		t.Fatalf("ParseSync failed")
	}

	// Query lines 5-10 only — the template_string starts at line 0.
	spans := e.Highlights(path, 5, 10)
	if spans == nil {
		t.Fatalf("expected spans, got nil")
	}
	// Lines 5-10 are inside a template_string.  Each should have a "string" span.
	for row := 5; row <= 10; row++ {
		lineSpans := spans[row]
		found := false
		for _, s := range lineSpans {
			if s.Kind == "string" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("line %d: expected 'string' span inside template literal, got %v", row, lineSpans)
		}
	}
}

func spanCoversKind(spans []HighlightSpan, col int, kind string) bool {
	for _, span := range spans {
		if span.Kind != kind {
			continue
		}
		if col >= span.StartCol && col < span.EndCol {
			return true
		}
	}
	return false
}
