package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/integrations"
)

func TestConfiguredEditorAppliesLoadedProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QEDIT_CONFIG_HOME", dir)
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[editor]
profile = "vim"
tab-width = 4
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Editor.Profile != editor.BehaviorProfileVim {
		t.Fatalf("cfg profile = %q, want vim", cfg.Editor.Profile)
	}

	ed := newConfiguredEditor(&cfg, nil, integrations.FileStore{}, nil)
	if got := ed.BehaviorProfile(); got != editor.BehaviorProfileVim {
		t.Fatalf("editor profile = %q, want vim", got)
	}
}

func TestConfiguredEditorUsesLoadedProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QEDIT_CONFIG_HOME", dir)
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[editor]
profile = "basic"
tab-width = 4
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	ed := newConfiguredEditor(&cfg, nil, integrations.FileStore{}, nil)
	if got := ed.BehaviorProfile(); got != cfg.Editor.Profile {
		t.Fatalf("editor profile = %q, cfg profile = %q", got, cfg.Editor.Profile)
	}
}
