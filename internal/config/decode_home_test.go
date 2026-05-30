package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDecodeConfiguredConfigHomeEditorProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QEDIT_CONFIG_HOME", dir)
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[editor]
profile = "VIM"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var userCfg Config
	if _, err := toml.Decode(string(data), &userCfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if userCfg.Editor.Profile != "VIM" {
		t.Fatalf("decoded profile = %q, want VIM", userCfg.Editor.Profile)
	}
	want := normalizeEditorProfile(userCfg.Editor.Profile)
	if cfg.Editor.Profile != want {
		t.Fatalf("Load profile %q != want %q", cfg.Editor.Profile, want)
	}
}
