package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestConfigDirEnv(t *testing.T) {
	t.Setenv("QEDIT_CONFIG_HOME", "/tmp/qedit-config")
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir error: %v", err)
	}
	if dir != "/tmp/qedit-config" {
		t.Fatalf("ConfigDir = %q, want %q", dir, "/tmp/qedit-config")
	}

	t.Setenv("QEDIT_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	dir, err = ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir error: %v", err)
	}
	if dir != "/tmp/xdg/qedit" {
		t.Fatalf("ConfigDir = %q, want %q", dir, "/tmp/xdg/qedit")
	}
}

func TestLoadWithThemeAndOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QEDIT_CONFIG_HOME", dir)

	writeFile(t, filepath.Join(dir, "theme", "test.toml"), `
foreground = "#111111"
background = "#222222"
statusline-foreground = "#333333"
`)

	writeFile(t, filepath.Join(dir, "config.toml"), `
[editor]
tab-width = 8
line-numbers = "relative"
git-branch-symbol = "branch"

[theme]
theme = "test"
commandline-background = "#123456"

[keymap.normal]
x = "quit"
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Editor.TabWidth != 8 {
		t.Fatalf("TabWidth = %d, want 8", cfg.Editor.TabWidth)
	}
	if cfg.Editor.LineNumbers != "relative" {
		t.Fatalf("LineNumbers = %q, want %q", cfg.Editor.LineNumbers, "relative")
	}
	if cfg.Editor.GitBranchSymbol != "branch" {
		t.Fatalf("GitBranchSymbol = %q, want %q", cfg.Editor.GitBranchSymbol, "branch")
	}
	if cfg.Theme.Foreground != "#111111" {
		t.Fatalf("Foreground = %q, want %q", cfg.Theme.Foreground, "#111111")
	}
	if cfg.Theme.Background != "#222222" {
		t.Fatalf("Background = %q, want %q", cfg.Theme.Background, "#222222")
	}
	if cfg.Theme.CommandlineBackground != "#123456" {
		t.Fatalf("CommandlineBackground = %q, want %q", cfg.Theme.CommandlineBackground, "#123456")
	}
	if cfg.Keymap.Normal["x"] != "quit" {
		t.Fatalf("keymap x = %q, want %q", cfg.Keymap.Normal["x"], "quit")
	}
	if cfg.Keymap.Normal["h"] != "move_left" {
		t.Fatalf("keymap h = %q, want %q", cfg.Keymap.Normal["h"], "move_left")
	}
}

func TestLoadThemeWrapped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QEDIT_CONFIG_HOME", dir)

	writeFile(t, filepath.Join(dir, "theme", "wrapped.toml"), `
[theme]
foreground = "#aaaaaa"
background = "#bbbbbb"
`)

	theme, err := LoadTheme("wrapped")
	if err != nil {
		t.Fatalf("LoadTheme error: %v", err)
	}
	if theme.Foreground != "#aaaaaa" {
		t.Fatalf("Foreground = %q, want %q", theme.Foreground, "#aaaaaa")
	}
	if theme.Background != "#bbbbbb" {
		t.Fatalf("Background = %q, want %q", theme.Background, "#bbbbbb")
	}
}
