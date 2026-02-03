package treesitter

import (
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
