package treesitter

import (
	"fmt"
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

func TestAsyncParseKeepsLatestQueuedText(t *testing.T) {
	langs := config.Languages{
		Languages: []config.Language{
			{Name: "javascript", FileTypes: []string{"js"}},
		},
	}
	e := New(langs)

	path := "sample.js"
	const requests = 20
	for i := 0; i < requests; i++ {
		version := e.Parse(path, "javascript", fmt.Sprintf("const value = %d;\n", i))
		if version != uint64(i+1) {
			t.Fatalf("parse version = %d, want %d", version, i+1)
		}
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer func() { _ = e.Stop() }()

	select {
	case ev := <-e.Events():
		if ev.Kind != "parsed" || ev.Path != path {
			t.Fatalf("event = %#v, want parsed event for %s", ev, path)
		}
		if ev.Version != requests {
			t.Fatalf("event version = %d, want %d", ev.Version, requests)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for parse event")
	}

	e.mu.RLock()
	got := string(e.sources[path])
	e.mu.RUnlock()
	want := fmt.Sprintf("const value = %d;\n", requests-1)
	if got != want {
		t.Fatalf("stored source = %q, want %q", got, want)
	}
}

func TestJSONHighlightsUsePlainFallbackForInvalidLine(t *testing.T) {
	e := New(config.Languages{})
	line := `{"event_type":"bi`

	spans := e.highlightJSONLine(line)
	if len(spans) == 0 {
		t.Fatalf("expected fallback span for invalid JSON line")
	}
	if spans[0].Kind != "plain" || spans[0].StartCol != 0 || spans[0].EndCol != len([]rune(line)) {
		t.Fatalf("first span = %#v, want full-line plain fallback", spans[0])
	}
	if kind, ok := highlightKindForTest(spans, 15); !ok || kind != "plain" {
		t.Fatalf("highlight at unfinished string = (%q,%v), want plain,true; spans=%#v", kind, ok, spans)
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

func highlightKindForTest(spans []HighlightSpan, col int) (string, bool) {
	bestKind := ""
	bestPriority := 0
	for _, span := range spans {
		if col < span.StartCol || col >= span.EndCol {
			continue
		}
		priority := highlightPriorityForTest(span.Kind)
		if priority > bestPriority {
			bestPriority = priority
			bestKind = span.Kind
		}
	}
	if bestKind == "" {
		return "", false
	}
	return bestKind, true
}

func highlightPriorityForTest(kind string) int {
	switch kind {
	case "comment":
		return 7
	case "string":
		return 6
	case "keyword":
		return 5
	case "constant", "builtin", "yaml-key":
		return 4
	case "parameter", "yaml-list-item", "type", "function", "number":
		return 3
	case "field", "variable", "yaml-value":
		return 2
	case "operator", "punctuation", "plain":
		return 1
	case "text":
		return 8
	default:
		return 0
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
	sb.WriteString("const css = `\n") // line 0: template start
	for i := 0; i < 10; i++ {
		sb.WriteString("  .class" + string(rune('a'+i)) + " { color: red; }\n") // lines 1-10
	}
	sb.WriteString("`;\n")            // line 11: template end
	sb.WriteString("const x = 42;\n") // line 12

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

func TestTemplateStringAfterLongLineHighlightsWindow(t *testing.T) {
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

	// Simulate mermaid.min.js: a very long minified line followed by a
	// template literal whose content is on short lines.  When queried via
	// HighlightsWindow (huge file mode path), the short lines inside the
	// template should still get "string" spans even though the template
	// literal starts on the preceding long line.
	var sb strings.Builder
	// Line 0: very long minified JS (>4096 bytes) ending with template start.
	sb.WriteString("var x=" + strings.Repeat("foo()+", 1000) + "0;var css=`\n")
	// Lines 1-5: short CSS lines inside the template.
	for i := 0; i < 5; i++ {
		sb.WriteString("  .item" + string(rune('a'+i)) + " { color: red; }\n")
	}
	// Line 6: template end + more code.
	sb.WriteString("`;var y=1;\n")

	path := "minified_tpl.js"
	text := sb.String()
	if ok := e.ParseSync(path, "javascript", text); !ok {
		t.Fatalf("ParseSync failed")
	}

	// Query lines 1-5 (short lines inside template) via HighlightsWindow —
	// this is the code path used in huge file mode.
	spans := e.HighlightsWindow(path, 1, 5, 0, 40)
	if spans == nil {
		t.Fatalf("expected spans, got nil")
	}
	for row := 1; row <= 5; row++ {
		lineSpans := spans[row]
		found := false
		for _, s := range lineSpans {
			if s.Kind == "string" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("line %d: expected 'string' span inside template literal (after long line), got %v", row, lineSpans)
		}
	}
}

func TestLongLineInsideTemplateLiteralGetsStringSpan(t *testing.T) {
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

	// A template literal whose interior contains a long line (>4096 bytes).
	// The individual long-line query must still capture the template_string
	// node starting on a previous row.
	var sb strings.Builder
	sb.WriteString("var css = `\n")                                               // line 0: template start
	sb.WriteString("  .short { color: red; }\n")                                  // line 1: short
	sb.WriteString("  .long { content: '" + strings.Repeat("x", 5000) + "'; }\n") // line 2: long (>4096 bytes)
	sb.WriteString("  .another { color: blue; }\n")                               // line 3: short
	sb.WriteString("`;\n")                                                        // line 4: template end

	path := "long_in_tpl.js"
	text := sb.String()
	if ok := e.ParseSync(path, "javascript", text); !ok {
		t.Fatalf("ParseSync failed")
	}

	// Query line 2 (long line inside template) via HighlightsWindow.
	spans := e.HighlightsWindow(path, 1, 3, 0, 40)
	if spans == nil {
		t.Fatalf("expected spans, got nil")
	}
	// Line 2 (long, inside template) should have a "string" span.
	found := false
	for _, s := range spans[2] {
		if s.Kind == "string" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("line 2 (long line inside template literal): expected 'string' span, got %v", spans[2])
	}
}

func TestHighlightsWindowPerformanceNoDeepLookback(t *testing.T) {
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

	// Build a file with 100 long minified lines followed by short lines.
	// Querying the short lines at the end must NOT scan back through all
	// the long lines — that would be catastrophically slow.
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("var v" + string(rune('a'+(i%26))) + "=" + strings.Repeat("foo()+", 800) + "0;\n")
	}
	// Lines 100-105: short lines.
	for i := 0; i < 5; i++ {
		sb.WriteString("var short" + string(rune('a'+i)) + " = " + string(rune('0'+i)) + ";\n")
	}
	sb.WriteString("\n") // line 106

	path := "perf_test.js"
	text := sb.String()
	if ok := e.ParseSync(path, "javascript", text); !ok {
		t.Fatalf("ParseSync failed")
	}

	// Query only the short lines at the end (100-105) with column clipping.
	// This should complete in <500ms; with a naive lookback to row 0 it would
	// take seconds because it scans 100 * ~5KB = ~500KB of minified nodes.
	start := time.Now()
	spans := e.HighlightsWindow(path, 100, 105, 0, 30)
	elapsed := time.Since(start)
	if spans == nil {
		t.Fatalf("HighlightsWindow returned nil")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("HighlightsWindow took %v (>500ms) — lookback may be scanning too many lines", elapsed)
	}
	// Short lines should have spans.
	if len(spans[100]) == 0 {
		t.Errorf("line 100 has no spans")
	}
	t.Logf("HighlightsWindow for lines 100-105 took %v", elapsed)
}

func TestEndOfFileHighlightsAfterCommentBlock(t *testing.T) {
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

	// Simulate end of mermaid.min.js: long minified lines, then a comment
	// block, then a last code line.  All highlight methods should produce
	// spans for the last code line.
	var sb strings.Builder
	// Lines 0-2: long minified JS lines.
	for i := 0; i < 3; i++ {
		sb.WriteString("var v" + string(rune('a'+i)) + "=" + strings.Repeat("foo()+", 1000) + "0;\n")
	}
	// Lines 3-8: multi-line comment block (short lines).
	sb.WriteString("/*\n")
	sb.WriteString(" * Copyright 2024\n")
	sb.WriteString(" * Licensed under MIT\n")
	sb.WriteString(" * http://example.com\n")
	sb.WriteString(" */\n")
	// Line 8: last code line (short).
	sb.WriteString("globalThis[\"mermaid\"] = require(\"mermaid\");\n")
	// Line 9: empty final line.

	path := "end_of_file.js"
	text := sb.String()
	if ok := e.ParseSync(path, "javascript", text); !ok {
		t.Fatalf("ParseSync failed")
	}

	// Test 1: Highlights (no column clipping) — lines 3-9.
	spans := e.Highlights(path, 3, 9)
	if spans == nil {
		t.Fatalf("Highlights returned nil")
	}
	// Line 8 should have spans (string "mermaid", identifier, etc.).
	if len(spans[8]) == 0 {
		t.Errorf("Highlights: line 8 has no spans; expected highlights for code line")
	}
	// Comment lines (3-7) should have comment spans.
	for row := 3; row <= 7; row++ {
		found := false
		for _, s := range spans[row] {
			if s.Kind == "comment" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Highlights: line %d has no comment span; spans=%v", row, spans[row])
		}
	}

	// Test 2: HighlightsWindow (with column clipping) — lines 3-9.
	spansW := e.HighlightsWindow(path, 3, 9, 0, 50)
	if spansW == nil {
		t.Fatalf("HighlightsWindow returned nil")
	}
	if len(spansW[8]) == 0 {
		t.Errorf("HighlightsWindow: line 8 has no spans; expected highlights for code line")
	}
	t.Logf("Line 8 spans (Highlights):       %v", spans[8])
	t.Logf("Line 8 spans (HighlightsWindow): %v", spansW[8])
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
