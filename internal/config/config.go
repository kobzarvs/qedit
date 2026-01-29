package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Keymap struct {
	Normal map[string]string `toml:"normal"`
	Insert map[string]string `toml:"insert"`
}

type EditorOptions struct {
	TabWidth        int    `toml:"tab-width"`
	LineNumbers     string `toml:"line-numbers"`
	GitBranchSymbol string `toml:"git-branch-symbol"`
}

type Theme struct {
	Theme                      string `toml:"theme"`
	Foreground                 string `toml:"foreground"`
	Background                 string `toml:"background"`
	StatuslineForeground       string `toml:"statusline-foreground"`
	StatuslineBackground       string `toml:"statusline-background"`
	CommandlineForeground      string `toml:"commandline-foreground"`
	CommandlineBackground      string `toml:"commandline-background"`
	LineNumberForeground       string `toml:"line-number-foreground"`
	LineNumberActiveForeground string `toml:"line-number-active-foreground"`
	SelectionForeground        string `toml:"selection-foreground"`
	SelectionBackground        string `toml:"selection-background"`
}

type Config struct {
	Editor EditorOptions `toml:"editor"`
	Theme  Theme         `toml:"theme"`
	Keymap Keymap        `toml:"keymap"`
}

func Default() Config {
	return Config{
		Editor: EditorOptions{
			TabWidth:        4,
			LineNumbers:     "absolute",
			GitBranchSymbol: "git:",
		},
		Theme: Theme{
			Theme:                      "",
			Foreground:                 "#B3B1AD",
			Background:                 "#0A0E14",
			StatuslineForeground:       "#B3B1AD",
			StatuslineBackground:       "#0F1419",
			CommandlineForeground:      "#B3B1AD",
			CommandlineBackground:      "#0F1419",
			LineNumberForeground:       "#3E4B59",
			LineNumberActiveForeground: "#B3B1AD",
			SelectionForeground:        "#B3B1AD",
			SelectionBackground:        "#27425A",
		},
		Keymap: Keymap{
			Normal: map[string]string{
				"h":         "move_left",
				"j":         "move_down",
				"k":         "move_up",
				"l":         "move_right",
				"left":      "move_left",
				"down":      "move_down",
				"up":        "move_up",
				"right":     "move_right",
				"home":      "line_start",
				"end":       "line_end",
				"cmd+home":  "file_start",
				"cmd+end":   "file_end",
				"cmd+left":  "word_left",
				"cmd+right": "word_right",
				"cmd+up":    "move_line_up",
				"cmd+down":  "move_line_down",
				"cmd+l":     "toggle_line_numbers",
				"cmd+b":     "branch_picker",
				"ctrl+home": "file_start",
				"ctrl+end":  "file_end",
				"ctrl+a":    "file_start",
				"ctrl+e":    "file_end",
				"pgup":      "page_up",
				"pgdn":      "page_down",
				"i":         "enter_insert",
				":":         "enter_command",
				"q":         "quit",
				"u":         "undo",
				"U":         "redo",
				"ctrl+c":    "quit",
				"ctrl+r":    "redo",
			},
			Insert: map[string]string{
				"esc":       "enter_normal",
				"left":      "move_left",
				"down":      "move_down",
				"up":        "move_up",
				"right":     "move_right",
				"home":      "line_start",
				"end":       "line_end",
				"cmd+home":  "file_start",
				"cmd+end":   "file_end",
				"cmd+left":  "word_left",
				"cmd+right": "word_right",
				"cmd+up":    "move_line_up",
				"cmd+down":  "move_line_down",
				"cmd+l":     "toggle_line_numbers",
				"cmd+b":     "branch_picker",
				"ctrl+home": "file_start",
				"ctrl+end":  "file_end",
				"ctrl+a":    "file_start",
				"ctrl+e":    "file_end",
				"pgup":      "page_up",
				"pgdn":      "page_down",
				"backspace": "backspace",
				"enter":     "newline",
				"tab":       "insert_tab",
			},
		},
	}
}

func Load() (Config, error) {
	cfg := Default()
	path, err := ConfigPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	var userCfg Config
	if _, err := toml.Decode(string(data), &userCfg); err != nil {
		return cfg, err
	}

	if userCfg.Editor.TabWidth > 0 {
		cfg.Editor.TabWidth = userCfg.Editor.TabWidth
	}
	if userCfg.Editor.LineNumbers != "" {
		cfg.Editor.LineNumbers = userCfg.Editor.LineNumbers
	}
	if userCfg.Editor.GitBranchSymbol != "" {
		cfg.Editor.GitBranchSymbol = userCfg.Editor.GitBranchSymbol
	}
	if userCfg.Theme.Theme != "" {
		cfg.Theme.Theme = userCfg.Theme.Theme
	}
	if cfg.Theme.Theme != "" {
		theme, err := LoadTheme(cfg.Theme.Theme)
		if err != nil {
			return cfg, err
		}
		mergeTheme(&cfg.Theme, theme)
	}
	if userCfg.Theme.Foreground != "" {
		cfg.Theme.Foreground = userCfg.Theme.Foreground
	}
	if userCfg.Theme.Background != "" {
		cfg.Theme.Background = userCfg.Theme.Background
	}
	if userCfg.Theme.StatuslineForeground != "" {
		cfg.Theme.StatuslineForeground = userCfg.Theme.StatuslineForeground
	}
	if userCfg.Theme.StatuslineBackground != "" {
		cfg.Theme.StatuslineBackground = userCfg.Theme.StatuslineBackground
	}
	if userCfg.Theme.CommandlineForeground != "" {
		cfg.Theme.CommandlineForeground = userCfg.Theme.CommandlineForeground
	}
	if userCfg.Theme.CommandlineBackground != "" {
		cfg.Theme.CommandlineBackground = userCfg.Theme.CommandlineBackground
	}
	if userCfg.Theme.LineNumberForeground != "" {
		cfg.Theme.LineNumberForeground = userCfg.Theme.LineNumberForeground
	}
	if userCfg.Theme.LineNumberActiveForeground != "" {
		cfg.Theme.LineNumberActiveForeground = userCfg.Theme.LineNumberActiveForeground
	}
	if userCfg.Theme.SelectionForeground != "" {
		cfg.Theme.SelectionForeground = userCfg.Theme.SelectionForeground
	}
	if userCfg.Theme.SelectionBackground != "" {
		cfg.Theme.SelectionBackground = userCfg.Theme.SelectionBackground
	}
	if userCfg.Keymap.Normal != nil {
		for k, v := range userCfg.Keymap.Normal {
			cfg.Keymap.Normal[k] = v
		}
	}
	if userCfg.Keymap.Insert != nil {
		for k, v := range userCfg.Keymap.Insert {
			cfg.Keymap.Insert[k] = v
		}
	}

	return cfg, nil
}

func mergeTheme(dst *Theme, src Theme) {
	if src.Foreground != "" {
		dst.Foreground = src.Foreground
	}
	if src.Background != "" {
		dst.Background = src.Background
	}
	if src.StatuslineForeground != "" {
		dst.StatuslineForeground = src.StatuslineForeground
	}
	if src.StatuslineBackground != "" {
		dst.StatuslineBackground = src.StatuslineBackground
	}
	if src.CommandlineForeground != "" {
		dst.CommandlineForeground = src.CommandlineForeground
	}
	if src.CommandlineBackground != "" {
		dst.CommandlineBackground = src.CommandlineBackground
	}
	if src.LineNumberForeground != "" {
		dst.LineNumberForeground = src.LineNumberForeground
	}
	if src.LineNumberActiveForeground != "" {
		dst.LineNumberActiveForeground = src.LineNumberActiveForeground
	}
	if src.SelectionForeground != "" {
		dst.SelectionForeground = src.SelectionForeground
	}
	if src.SelectionBackground != "" {
		dst.SelectionBackground = src.SelectionBackground
	}
}

func ThemePath(name string) (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "theme", name+".toml"), nil
}

func LoadTheme(name string) (Theme, error) {
	path, err := ThemePath(name)
	if err != nil {
		return Theme{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, err
	}
	var t Theme
	if _, err := toml.Decode(string(data), &t); err == nil {
		return t, nil
	}
	var wrap struct {
		Theme Theme `toml:"theme"`
	}
	if _, err := toml.Decode(string(data), &wrap); err != nil {
		return Theme{}, err
	}
	return wrap.Theme, nil
}

func ConfigDir() (string, error) {
	if v := os.Getenv("QEDIT_CONFIG_HOME"); v != "" {
		return filepath.Join(v), nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "qedit"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "qedit"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}
