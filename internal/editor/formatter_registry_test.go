package editor

import (
	"strings"
	"testing"
)

func TestRegisterFormatterOverridesBuiltInGoFormatter(t *testing.T) {
	e := newTestEditor("package main\nfunc main() {}\n")
	e.document.filename = "main.go"
	called := false

	e.RegisterFormatter(FormatterProvider{
		Name: "custom-go",
		Supports: func(path, content string) bool {
			return strings.HasSuffix(path, ".go")
		},
		Format: func(ed *Editor, path, content string) error {
			called = true
			ed.replaceBuffer(content+"// custom\n", true)
			ed.setStatus("custom format")
			return nil
		},
	})

	if quit := e.execCommand("fmt"); quit {
		t.Fatalf("execCommand fmt returned true")
	}
	if !called {
		t.Fatalf("custom formatter was not called")
	}
	if got := e.Content(); got != "package main\nfunc main() {}\n// custom\n" {
		t.Fatalf("content = %q", got)
	}
	if _, ok := e.ConsumeRuntimeRequest(); ok {
		t.Fatalf("expected no runtime format request when custom formatter handles Go")
	}
}

func TestRegisterFormatterHandlesCustomExtension(t *testing.T) {
	e := newTestEditor("hello")
	e.document.filename = "note.txt"

	e.RegisterFormatter(FormatterProvider{
		Name: "txt-upper",
		Supports: func(path, content string) bool {
			return strings.HasSuffix(path, ".txt")
		},
		Format: func(ed *Editor, path, content string) error {
			ed.replaceBuffer(strings.ToUpper(content), true)
			return nil
		},
	})

	if err := e.queueFormatRequest(); err != nil {
		t.Fatalf("queueFormatRequest returned error: %v", err)
	}
	if got := e.Content(); got != "HELLO" {
		t.Fatalf("content = %q, want %q", got, "HELLO")
	}
}
