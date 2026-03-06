package editor

import "testing"

func TestRegisterLanguageFeatureOverridesRuntimeGoto(t *testing.T) {
	e := newTestEditor("hello")
	e.SetLanguageRuntime(testLanguageRuntime{
		gotoLocations: []LSPLocation{{Path: "runtime.go", StartLine: 1, StartCol: 2}},
	})

	e.RegisterLanguageFeature(LanguageFeatureProvider{
		Name: "txt-override",
		Available: func(*Editor) bool {
			return true
		},
		Supports: func(path string) bool {
			return path == "note.txt"
		},
		Goto: func(ed *Editor, method, path string, line, col int) ([]LSPLocation, error) {
			return []LSPLocation{{Path: "override.txt", StartLine: 3, StartCol: 4}}, nil
		},
	})

	locs, err := e.languageGoto("definition", "note.txt", 0, 0)
	if err != nil {
		t.Fatalf("languageGoto returned error: %v", err)
	}
	if len(locs) != 1 || locs[0].Path != "override.txt" {
		t.Fatalf("locations = %#v, want override result", locs)
	}
}

func TestBuiltInLanguageFeatureUsesRuntimeFallback(t *testing.T) {
	e := newTestEditor("hello")
	e.SetLanguageRuntime(testLanguageRuntime{
		gotoLocations: []LSPLocation{{Path: "runtime.go", StartLine: 1, StartCol: 2}},
	})

	locs, err := e.languageGoto("definition", "main.go", 0, 0)
	if err != nil {
		t.Fatalf("languageGoto returned error: %v", err)
	}
	if len(locs) != 1 || locs[0].Path != "runtime.go" {
		t.Fatalf("locations = %#v, want runtime result", locs)
	}
}
