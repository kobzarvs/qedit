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
	defer e.Stop()

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
	defer e.Stop()

	e.OpenFile("README.md", "hello")
	select {
	case ev := <-e.Events():
		t.Fatalf("unexpected event: %#v", ev)
	case <-time.After(150 * time.Millisecond):
	}
}
