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
	SearchMatchForeground      string `toml:"search-foreground"`
	SearchMatchBackground      string `toml:"search-background"`
	SyntaxKeyword              string `toml:"syntax-keyword"`
	SyntaxString               string `toml:"syntax-string"`
	SyntaxComment              string `toml:"syntax-comment"`
	SyntaxType                 string `toml:"syntax-type"`
	SyntaxFunction             string `toml:"syntax-function"`
	SyntaxNumber               string `toml:"syntax-number"`
	SyntaxConstant             string `toml:"syntax-constant"`
	SyntaxOperator             string `toml:"syntax-operator"`
	SyntaxPunctuation          string `toml:"syntax-punctuation"`
	SyntaxField                string `toml:"syntax-field"`
	SyntaxBuiltin              string `toml:"syntax-builtin"`
	SyntaxUnknown              string `toml:"syntax-unknown"`
	SyntaxVariable             string `toml:"syntax-variable"`
	SyntaxParameter            string `toml:"syntax-parameter"`
	BranchForeground           string `toml:"branch-foreground"`
	BranchBackground           string `toml:"branch-background"`
	MainBranchForeground       string `toml:"main-branch-foreground"`
	MainBranchBackground       string `toml:"main-branch-background"`
	AutocompleteBackground     string `toml:"autocomplete-background"`
	AutocompleteHotkey         string `toml:"autocomplete-hotkey"`
	AutocompleteDescription    string `toml:"autocomplete-description"`
	AutocompleteGroup          string `toml:"autocomplete-group"`
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
			SearchMatchForeground:      "#000000",
			SearchMatchBackground:      "#FFD700",
			SyntaxKeyword:              "#FFA759",
			SyntaxString:               "#BAE67E",
			SyntaxComment:              "#5C6773",
			SyntaxType:                 "#5CCFE6",
			SyntaxFunction:             "#FFD173",
			SyntaxNumber:               "#D4BFFF",
			SyntaxConstant:             "#FFDD8E",
			SyntaxOperator:             "#F29668",
			SyntaxPunctuation:          "#C0C0C0",
			SyntaxField:                "#E6B673",
			SyntaxBuiltin:              "#73D0FF",
			SyntaxUnknown:              "#FF0000",
			SyntaxVariable:             "#B3B1AD",
			SyntaxParameter:            "#B3B1AD",
		},
		Keymap: Keymap{
			Normal: map[string]string{
				"h":              "move_left",
				"j":              "move_down",
				"k":              "move_up",
				"l":              "move_right",
				"left":           "move_left",
				"down":           "move_down",
				"up":             "move_up",
				"right":          "move_right",
				"home":           "line_start",
				"end":            "line_end",
				"cmd+home":       "file_start",
				"cmd+end":        "file_end",
				"cmd+left":       "word_left",
				"cmd+right":      "word_right",
				"cmd+up":         "move_line_up",
				"cmd+down":       "move_line_down",
				"cmd+l":          "toggle_line_numbers",
				"cmd+b":          "branch_picker",
				"cmd+y":          "delete_line",
				"del":            "delete_char",
				"cmd+backspace":  "delete_word_left",
				"cmd+del":        "delete_word_right",
				"ctrl+home":      "file_start",
				"ctrl+end":       "file_end",
				"ctrl+y":         "scroll_up",
				"ctrl+e":         "scroll_down",
				"pgup":           "page_up",
				"pgdn":           "page_down",
				"i":              "enter_insert",
				":":              "enter_command",
				"u":              "undo",
				"U":              "redo",
				"ctrl+c":         "quit",
				"ctrl+r":         "redo",
				"tab":            "indent",
				"shift+tab":      "unindent",
				"cmd+a":          "select_all",
				"cmd+g":          "goto_line_prompt",

				// Helix-style motions
				"w":              "word_forward",
				"b":              "word_backward",
				"e":              "word_end",
				"g":              "goto_mode",
				"G":              "goto_line",
				"f":              "find_char",
				"F":              "find_char_backward",
				"t":              "till_char",
				"T":              "till_char_backward",

				// Helix-style editing
				"d":              "delete",
				"c":              "change",
				"y":              "yank",
				"p":              "paste",
				"P":              "paste_before",
				"o":              "open_below",
				"O":              "open_above",
				"a":              "append",
				"A":              "append_line_end",
				"I":              "insert_line_start",
				"r":              "replace_char",
				"J":              "join_lines",

				// Helix-style selection
				"v":              "toggle_select",
				"x":              "extend_line",
				";":              "collapse_selection",
				"%":              "select_all",
				">":              "indent",
				"<":              "unindent",

				// Space mode
				"space":          "space_mode",

				// Match mode
				"m":              "match_mode",

				// View mode
				"z":              "view_mode",

				// Search
				"/":              "search_forward",
				"?":              "search_backward",
				"n":              "search_next",
				"N":              "search_prev",
				"cmd+f":          "search_fuzzy",
				"cmd+e":          "search_regex",

				// Special
				"shift+enter":    "insert_line_above",

				// Terminal zoom
				"=":              "terminal_zoom_in",

				// Selection scope
				"alt+shift+up":   "expand_selection",
				"alt+shift+down": "shrink_selection",

				// File operations
				"cmd+s":          "save",
			},
			Insert: map[string]string{
				"esc":            "enter_normal",
				"left":           "move_left",
				"down":           "move_down",
				"up":             "move_up",
				"right":          "move_right",
				"home":           "line_start",
				"end":            "line_end",
				"cmd+home":       "file_start",
				"cmd+end":        "file_end",
				"cmd+left":       "word_left",
				"cmd+right":      "word_right",
				"cmd+up":         "move_line_up",
				"cmd+down":       "move_line_down",
				"cmd+l":          "toggle_line_numbers",
				"cmd+b":          "branch_picker",
				"cmd+y":          "delete_line",
				"del":            "delete_char",
				"cmd+backspace":  "delete_word_left",
				"cmd+del":        "delete_word_right",
				"cmd+enter":      "insert_line_below",
				"ctrl+home":      "file_start",
				"ctrl+end":       "file_end",
				"ctrl+y":         "scroll_up",
				"ctrl+e":         "scroll_down",
				"pgup":           "page_up",
				"pgdn":           "page_down",
				"backspace":      "backspace",
				"enter":          "newline",
				"tab":            "indent",
				"shift+tab":      "unindent",
				"cmd+a":          "select_all",
				"shift+enter":    "insert_line_above",

				// File operations
				"cmd+s":          "save",
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
	if userCfg.Theme.SearchMatchForeground != "" {
		cfg.Theme.SearchMatchForeground = userCfg.Theme.SearchMatchForeground
	}
	if userCfg.Theme.SearchMatchBackground != "" {
		cfg.Theme.SearchMatchBackground = userCfg.Theme.SearchMatchBackground
	}
	if userCfg.Theme.SyntaxKeyword != "" {
		cfg.Theme.SyntaxKeyword = userCfg.Theme.SyntaxKeyword
	}
	if userCfg.Theme.SyntaxString != "" {
		cfg.Theme.SyntaxString = userCfg.Theme.SyntaxString
	}
	if userCfg.Theme.SyntaxComment != "" {
		cfg.Theme.SyntaxComment = userCfg.Theme.SyntaxComment
	}
	if userCfg.Theme.SyntaxType != "" {
		cfg.Theme.SyntaxType = userCfg.Theme.SyntaxType
	}
	if userCfg.Theme.SyntaxFunction != "" {
		cfg.Theme.SyntaxFunction = userCfg.Theme.SyntaxFunction
	}
	if userCfg.Theme.SyntaxNumber != "" {
		cfg.Theme.SyntaxNumber = userCfg.Theme.SyntaxNumber
	}
	if userCfg.Theme.SyntaxConstant != "" {
		cfg.Theme.SyntaxConstant = userCfg.Theme.SyntaxConstant
	}
	if userCfg.Theme.SyntaxOperator != "" {
		cfg.Theme.SyntaxOperator = userCfg.Theme.SyntaxOperator
	}
	if userCfg.Theme.SyntaxPunctuation != "" {
		cfg.Theme.SyntaxPunctuation = userCfg.Theme.SyntaxPunctuation
	}
	if userCfg.Theme.SyntaxField != "" {
		cfg.Theme.SyntaxField = userCfg.Theme.SyntaxField
	}
	if userCfg.Theme.SyntaxBuiltin != "" {
		cfg.Theme.SyntaxBuiltin = userCfg.Theme.SyntaxBuiltin
	}
	if userCfg.Theme.SyntaxUnknown != "" {
		cfg.Theme.SyntaxUnknown = userCfg.Theme.SyntaxUnknown
	}
	if userCfg.Theme.SyntaxVariable != "" {
		cfg.Theme.SyntaxVariable = userCfg.Theme.SyntaxVariable
	}
	if userCfg.Theme.SyntaxParameter != "" {
		cfg.Theme.SyntaxParameter = userCfg.Theme.SyntaxParameter
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
	if src.SearchMatchForeground != "" {
		dst.SearchMatchForeground = src.SearchMatchForeground
	}
	if src.SearchMatchBackground != "" {
		dst.SearchMatchBackground = src.SearchMatchBackground
	}
	if src.SyntaxKeyword != "" {
		dst.SyntaxKeyword = src.SyntaxKeyword
	}
	if src.SyntaxString != "" {
		dst.SyntaxString = src.SyntaxString
	}
	if src.SyntaxComment != "" {
		dst.SyntaxComment = src.SyntaxComment
	}
	if src.SyntaxType != "" {
		dst.SyntaxType = src.SyntaxType
	}
	if src.SyntaxFunction != "" {
		dst.SyntaxFunction = src.SyntaxFunction
	}
	if src.SyntaxNumber != "" {
		dst.SyntaxNumber = src.SyntaxNumber
	}
	if src.SyntaxConstant != "" {
		dst.SyntaxConstant = src.SyntaxConstant
	}
	if src.SyntaxOperator != "" {
		dst.SyntaxOperator = src.SyntaxOperator
	}
	if src.SyntaxPunctuation != "" {
		dst.SyntaxPunctuation = src.SyntaxPunctuation
	}
	if src.SyntaxField != "" {
		dst.SyntaxField = src.SyntaxField
	}
	if src.SyntaxBuiltin != "" {
		dst.SyntaxBuiltin = src.SyntaxBuiltin
	}
	if src.SyntaxUnknown != "" {
		dst.SyntaxUnknown = src.SyntaxUnknown
	}
	if src.SyntaxVariable != "" {
		dst.SyntaxVariable = src.SyntaxVariable
	}
	if src.SyntaxParameter != "" {
		dst.SyntaxParameter = src.SyntaxParameter
	}
	if src.BranchForeground != "" {
		dst.BranchForeground = src.BranchForeground
	}
	if src.BranchBackground != "" {
		dst.BranchBackground = src.BranchBackground
	}
	if src.MainBranchForeground != "" {
		dst.MainBranchForeground = src.MainBranchForeground
	}
	if src.MainBranchBackground != "" {
		dst.MainBranchBackground = src.MainBranchBackground
	}
	if src.AutocompleteBackground != "" {
		dst.AutocompleteBackground = src.AutocompleteBackground
	}
	if src.AutocompleteHotkey != "" {
		dst.AutocompleteHotkey = src.AutocompleteHotkey
	}
	if src.AutocompleteDescription != "" {
		dst.AutocompleteDescription = src.AutocompleteDescription
	}
	if src.AutocompleteGroup != "" {
		dst.AutocompleteGroup = src.AutocompleteGroup
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
