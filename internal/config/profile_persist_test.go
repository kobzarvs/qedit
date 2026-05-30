package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const userStyleConfig = `
[editor]
  profile = "vim"
  tab-width = 4
  line-numbers = "absolute"

[keymap]
  [keymap.insert]
    "cmd+s" = "save"
    tab = "indent"

`

func TestLoadNestedKeymapPreservesEditorProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QEDIT_CONFIG_HOME", dir)
	writeFile(t, filepath.Join(dir, "config.toml"), userStyleConfig)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Editor.Profile != "vim" {
		t.Fatalf("Profile = %q, want vim", cfg.Editor.Profile)
	}
	if cfg.Keymap.Insert["cmd+s"] != "save" {
		t.Fatalf("keymap insert cmd+s = %q, want save", cfg.Keymap.Insert["cmd+s"])
	}
}

func TestUpdateEditorProfilePreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QEDIT_CONFIG_HOME", dir)
	path := filepath.Join(dir, "config.toml")
	writeFile(t, path, userStyleConfig)

	if err := UpdateEditorProfile("basic"); err != nil {
		t.Fatalf("UpdateEditorProfile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, `profile = "basic"`) && !strings.Contains(body, "profile = 'basic'") {
		t.Fatalf("config missing basic profile:\n%s", body)
	}
	if !strings.Contains(body, "cmd+s") {
		t.Fatalf("config lost keymap after profile update:\n%s", body)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load after update: %v", err)
	}
	if cfg.Editor.Profile != "basic" {
		t.Fatalf("Profile = %q, want basic", cfg.Editor.Profile)
	}
	if cfg.Keymap.Insert["cmd+s"] != "save" {
		t.Fatalf("keymap insert cmd+s = %q, want save", cfg.Keymap.Insert["cmd+s"])
	}
}
