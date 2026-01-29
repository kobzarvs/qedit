package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type LanguageServer struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
}

type Language struct {
	Name            string   `toml:"name"`
	FileTypes       []string `toml:"file-types"`
	Roots           []string `toml:"roots"`
	LanguageServers []string `toml:"language-servers"`
}

type Languages struct {
	Languages       []Language               `toml:"language"`
	LanguageServers map[string]LanguageServer `toml:"language-server"`
}

func (l Languages) Match(path string) *Language {
	base := filepath.Base(path)
	baseLower := strings.ToLower(base)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(base), "."))
	for i := range l.Languages {
		lang := &l.Languages[i]
		for _, ft := range lang.FileTypes {
			ftLower := strings.ToLower(ft)
			if ftLower == ext || ftLower == baseLower {
				return lang
			}
			if strings.HasPrefix(ftLower, ".") && strings.TrimPrefix(ftLower, ".") == ext {
				return lang
			}
		}
	}
	return nil
}

func LoadLanguages() (Languages, error) {
	path, err := LanguagesPath()
	if err != nil {
		return Languages{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Languages{}, nil
		}
		return Languages{}, err
	}

	var cfg Languages
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Languages{}, err
	}
	if cfg.LanguageServers == nil {
		cfg.LanguageServers = map[string]LanguageServer{}
	}
	return cfg, nil
}

func LanguagesPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "languages.toml"), nil
}
