package config

import (
	"path/filepath"
	"testing"
)

func TestLanguagesMatch(t *testing.T) {
	cfg := Languages{
		Languages: []Language{
			{Name: "go", FileTypes: []string{"go", "go.mod", ".go"}},
			{Name: "git", FileTypes: []string{".gitignore", "Makefile"}},
		},
	}

	if got := cfg.Match("main.go"); got == nil || got.Name != "go" {
		t.Fatalf("Match main.go = %#v, want go", got)
	}
	if got := cfg.Match("go.mod"); got == nil || got.Name != "go" {
		t.Fatalf("Match go.mod = %#v, want go", got)
	}
	if got := cfg.Match(".gitignore"); got == nil || got.Name != "git" {
		t.Fatalf("Match .gitignore = %#v, want git", got)
	}
	if got := cfg.Match("Makefile"); got == nil || got.Name != "git" {
		t.Fatalf("Match Makefile = %#v, want git", got)
	}
	if got := cfg.Match("unknown.txt"); got != nil {
		t.Fatalf("Match unknown.txt = %#v, want nil", got)
	}
}

func TestLoadLanguages(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QEDIT_CONFIG_HOME", dir)

	writeFile(t, filepath.Join(dir, "languages.toml"), `
[[language]]
name = "go"
file-types = ["go"]
language-servers = ["gopls"]

[language-server.gopls]
command = "gopls"
args = ["-remote=auto"]
`)

	cfg, err := LoadLanguages()
	if err != nil {
		t.Fatalf("LoadLanguages error: %v", err)
	}
	if len(cfg.Languages) != 1 {
		t.Fatalf("Languages len = %d, want 1", len(cfg.Languages))
	}
	if cfg.LanguageServers == nil {
		t.Fatalf("LanguageServers is nil")
	}
	server, ok := cfg.LanguageServers["gopls"]
	if !ok {
		t.Fatalf("LanguageServers missing gopls")
	}
	if server.Command != "gopls" {
		t.Fatalf("gopls command = %q, want %q", server.Command, "gopls")
	}
}

func TestLoadLanguagesMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QEDIT_CONFIG_HOME", dir)

	cfg, err := LoadLanguages()
	if err != nil {
		t.Fatalf("LoadLanguages error: %v", err)
	}
	if len(cfg.Languages) != 0 {
		t.Fatalf("Languages len = %d, want 0", len(cfg.Languages))
	}
}
