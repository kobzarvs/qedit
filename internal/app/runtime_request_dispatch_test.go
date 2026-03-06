package app

import (
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
