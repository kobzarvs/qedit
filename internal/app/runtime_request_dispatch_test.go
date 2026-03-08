package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kobzarvs/qedit/internal/editor"
)

type testAppFormatter struct {
	seen      string
	formatted string
	err       error
}

func (f *testAppFormatter) FormatGo(src string) (string, error) {
	f.seen = src
	return f.formatted, f.err
}

type testAppClipboard struct {
	written string
	read    string
	readErr error
}

func (c *testAppClipboard) Read() (string, error) {
	return c.read, c.readErr
}

func (c *testAppClipboard) Write(text string) error {
	c.written = text
	return nil
}

func TestHandleFormatBufferRequestAppliesFormattedContent(t *testing.T) {
	ed := editor.New(editor.Options{})
	original := "package main\nfunc main() {  }\n"
	formatted := "package main\n\nfunc main() {}\n"
	if err := ed.LoadFileContent("/tmp/main.go", []byte(original)); err != nil {
		t.Fatalf("LoadFileContent returned error: %v", err)
	}

	formatter := &testAppFormatter{formatted: formatted}
	controller := editorRuntimeController{
		ed:        ed,
		formatter: formatter,
	}

	controller.handleFormatBufferRequest(editor.RuntimeRequest{
		Kind:    editor.RuntimeRequestFormatBuffer,
		Content: original,
	})

	if formatter.seen != original {
		t.Fatalf("formatter input = %q, want %q", formatter.seen, original)
	}
	if got := ed.Content(); got != formatted {
		t.Fatalf("content = %q, want %q", got, formatted)
	}
}

func TestHandleFormatBufferRequestAppliesJavaScriptPrettier(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	prettierPath := filepath.Join(binDir, "prettier")
	script := "#!/bin/sh\ncat >/dev/null\nprintf 'const x = 1;\\n'\n"
	if err := os.WriteFile(prettierPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write prettier stub: %v", err)
	}

	path := filepath.Join(dir, "app.min.js")
	ed := editor.New(editor.Options{})
	if err := ed.LoadFileContent(path, []byte("const x=1;")); err != nil {
		t.Fatalf("LoadFileContent returned error: %v", err)
	}
	controller := editorRuntimeController{
		ed: ed,
	}

	controller.handleFormatBufferRequest(editor.RuntimeRequest{
		Kind:    editor.RuntimeRequestFormatBuffer,
		Path:    path,
		Content: "const x=1;",
	})

	if got := ed.Content(); got != "const x = 1;\n" {
		t.Fatalf("content = %q, want %q", got, "const x = 1;\n")
	}
}

func TestHandleReadClipboardRequestAppliesClipboardText(t *testing.T) {
	ed := editor.New(editor.Options{})
	if err := ed.LoadFileContent("/tmp/main.txt", []byte("abc")); err != nil {
		t.Fatalf("LoadFileContent returned error: %v", err)
	}

	clipboard := &testAppClipboard{read: "X"}
	controller := editorRuntimeController{
		ed:        ed,
		clipboard: clipboard,
	}

	controller.handleReadClipboardRequest(editor.RuntimeRequest{
		Kind:   editor.RuntimeRequestReadClipboard,
		Before: false,
	})

	if got := ed.Content(); got != "aXbc" {
		t.Fatalf("content = %q, want %q", got, "aXbc")
	}
}
