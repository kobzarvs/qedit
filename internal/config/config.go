package config

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Keymap struct {
	Normal map[string]string `toml:"normal"`
	Insert map[string]string `toml:"insert"`
}

type EditorOptions struct {
	TabWidth              int    `toml:"tab-width"`
	Profile               string `toml:"profile"`
	LineNumbers           string `toml:"line-numbers"`
	GitBranchSymbol       string `toml:"git-branch-symbol"`
	SidebarWidth          string `toml:"sidebar-width"`
	SidebarMinWidth       int    `toml:"sidebar-min-width"`
	SidebarMaxWidth       string `toml:"sidebar-max-width"`
	SidebarCloseOnSelect  bool   `toml:"sidebar-close-on-select"`
	FileTreeShowHidden    bool   `toml:"file-tree-show-hidden"`
	FileTreeShowIgnored   bool   `toml:"file-tree-show-ignored"`
	HighlightMaxBytes     int64  `toml:"highlight-max-bytes"`
	AutoReloadOnChanges   bool   `toml:"auto-reload-on-changes"`
	AutoReloadStabilizeMS int    `toml:"auto-reload-stabilize-ms"`
	AutoReloadMaxRetries  int    `toml:"auto-reload-max-retries"`
}

type Theme struct {
	Theme                          string `toml:"theme"`
	Foreground                     string `toml:"foreground"`
	Background                     string `toml:"background"`
	StatuslineForeground           string `toml:"statusline-foreground"`
	StatuslineBackground           string `toml:"statusline-background"`
	StatuslineWarningForeground    string `toml:"statusline-warning-foreground"`
	StatuslineWarningBackground    string `toml:"statusline-warning-background"`
	MergeLocalBackground           string `toml:"merge-local-background"`
	MergeRemoteBackground          string `toml:"merge-remote-background"`
	MergeHeaderForeground          string `toml:"merge-header-foreground"`
	MergeHeaderBackground          string `toml:"merge-header-background"`
	CommandlineForeground          string `toml:"commandline-foreground"`
	CommandlineBackground          string `toml:"commandline-background"`
	LineNumberForeground           string `toml:"line-number-foreground"`
	LineNumberActiveForeground     string `toml:"line-number-active-foreground"`
	SelectionForeground            string `toml:"selection-foreground"`
	SelectionBackground            string `toml:"selection-background"`
	SearchMatchForeground          string `toml:"search-foreground"`
	SearchMatchBackground          string `toml:"search-background"`
	SidebarFilterFileForeground    string `toml:"sidebar-filter-file-foreground"`
	SidebarFilterDirForeground     string `toml:"sidebar-filter-dir-foreground"`
	NotificationForeground         string `toml:"notification-foreground"`
	NotificationBackground         string `toml:"notification-background"`
	NotificationFadeForeground     string `toml:"notification-fade-foreground"`
	NotificationFadeBackground     string `toml:"notification-fade-background"`
	SyntaxKeyword                  string `toml:"syntax-keyword"`
	SyntaxString                   string `toml:"syntax-string"`
	SyntaxComment                  string `toml:"syntax-comment"`
	SyntaxType                     string `toml:"syntax-type"`
	SyntaxFunction                 string `toml:"syntax-function"`
	SyntaxNumber                   string `toml:"syntax-number"`
	SyntaxConstant                 string `toml:"syntax-constant"`
	SyntaxOperator                 string `toml:"syntax-operator"`
	SyntaxPunctuation              string `toml:"syntax-punctuation"`
	SyntaxField                    string `toml:"syntax-field"`
	SyntaxBuiltin                  string `toml:"syntax-builtin"`
	SyntaxUnknown                  string `toml:"syntax-unknown"`
	SyntaxVariable                 string `toml:"syntax-variable"`
	SyntaxParameter                string `toml:"syntax-parameter"`
	SyntaxYAMLKey                  string `toml:"syntax-yaml-key"`
	SyntaxYAMLValue                string `toml:"syntax-yaml-value"`
	SyntaxYAMLListItem             string `toml:"syntax-yaml-list-item"`
	BranchForeground               string `toml:"branch-foreground"`
	BranchBackground               string `toml:"branch-background"`
	MainBranchForeground           string `toml:"main-branch-foreground"`
	MainBranchBackground           string `toml:"main-branch-background"`
	AutocompleteBackground         string `toml:"autocomplete-background"`
	AutocompleteHotkey             string `toml:"autocomplete-hotkey"`
	AutocompleteDescription        string `toml:"autocomplete-description"`
	AutocompleteGroup              string `toml:"autocomplete-group"`
	SidebarForeground              string `toml:"sidebar-foreground"`
	SidebarBackground              string `toml:"sidebar-background"`
	SidebarDirForeground           string `toml:"sidebar-dir-foreground"`
	SidebarSelectedForeground      string `toml:"sidebar-selected-foreground"`
	SidebarSelectedBackground      string `toml:"sidebar-selected-background"`
	SidebarHeaderForeground        string `toml:"sidebar-header-foreground"`
	SidebarHeaderBackground        string `toml:"sidebar-header-background"`
	SidebarBorderForeground        string `toml:"sidebar-border-foreground"`
	SidebarHiddenForeground        string `toml:"sidebar-hidden-foreground"`
	SidebarIgnoredForeground       string `toml:"sidebar-ignored-foreground"`
	SidebarIndicatorForeground     string `toml:"sidebar-indicator-foreground"`
	SidebarStatusOnlineForeground  string `toml:"sidebar-status-online-foreground"`
	SidebarStatusOfflineForeground string `toml:"sidebar-status-offline-foreground"`
	SidebarHotkeyForeground        string `toml:"sidebar-hotkey-foreground"`
	SidebarUnavailableForeground   string `toml:"sidebar-unavailable-foreground"`
	SidebarDiffAddForeground       string `toml:"sidebar-diff-add-foreground"`
	SidebarDiffDelForeground       string `toml:"sidebar-diff-del-foreground"`
	BoxBorderForeground            string `toml:"box-border-foreground"`
	BoxBorderBackground            string `toml:"box-border-background"`
}

type Config struct {
	Editor EditorOptions `toml:"editor"`
	Theme  Theme         `toml:"theme"`
	Keymap Keymap        `toml:"keymap"`
}

func Default() Config {
	return Config{
		Editor: EditorOptions{
			TabWidth:              4,
			Profile:               "helix",
			LineNumbers:           "absolute",
			GitBranchSymbol:       "git:",
			SidebarWidth:          "30",
			SidebarMinWidth:       15,
			SidebarMaxWidth:       "50",
			SidebarCloseOnSelect:  false,
			FileTreeShowHidden:    false,
			FileTreeShowIgnored:   false,
			HighlightMaxBytes:     8 << 20,
			AutoReloadOnChanges:   true,
			AutoReloadStabilizeMS: 300,
			AutoReloadMaxRetries:  10,
		},
		Theme: Theme{
			Theme:                          "",
			Foreground:                     "#B3B1AD",
			Background:                     "#0A0E14",
			StatuslineForeground:           "#B3B1AD",
			StatuslineBackground:           "#0F1419",
			StatuslineWarningForeground:    "#FFD700",
			StatuslineWarningBackground:    "#0F1419",
			MergeLocalBackground:           "#3A1E1E",
			MergeRemoteBackground:          "#1E3A24",
			MergeHeaderForeground:          "#FFD700",
			MergeHeaderBackground:          "#0F1419",
			CommandlineForeground:          "#B3B1AD",
			CommandlineBackground:          "#0F1419",
			LineNumberForeground:           "#3E4B59",
			LineNumberActiveForeground:     "#B3B1AD",
			SelectionForeground:            "#B3B1AD",
			SelectionBackground:            "#27425A",
			SearchMatchForeground:          "#000000",
			SearchMatchBackground:          "#FFD700",
			SidebarFilterFileForeground:    "#000000",
			SidebarFilterDirForeground:     "#59C2FF",
			NotificationForeground:         "#000000",
			NotificationBackground:         "#FFD700",
			NotificationFadeForeground:     "#B3B1AD",
			NotificationFadeBackground:     "#0F1419",
			SyntaxKeyword:                  "#FFA759",
			SyntaxString:                   "#BAE67E",
			SyntaxComment:                  "#5C6773",
			SyntaxType:                     "#5CCFE6",
			SyntaxFunction:                 "#FFD173",
			SyntaxNumber:                   "#D4BFFF",
			SyntaxConstant:                 "#FFDD8E",
			SyntaxOperator:                 "#F29668",
			SyntaxPunctuation:              "#C0C0C0",
			SyntaxField:                    "#E6B673",
			SyntaxBuiltin:                  "#73D0FF",
			SyntaxUnknown:                  "#FF0000",
			SyntaxVariable:                 "#B3B1AD",
			SyntaxParameter:                "#B3B1AD",
			SidebarForeground:              "#B3B1AD",
			SidebarBackground:              "#0A0E14",
			SidebarDirForeground:           "#59C2FF",
			SidebarSelectedForeground:      "#0A0E14",
			SidebarSelectedBackground:      "#E6B450",
			SidebarHeaderForeground:        "#B3B1AD",
			SidebarHeaderBackground:        "#0F1419",
			SidebarBorderForeground:        "#3E4B59",
			SidebarHiddenForeground:        "#3E4B59",
			SidebarIgnoredForeground:       "#3E4B59",
			SidebarIndicatorForeground:     "#E6B450",
			SidebarStatusOnlineForeground:  "#7FD962",
			SidebarStatusOfflineForeground: "#E06C75",
			SidebarHotkeyForeground:        "#59C2FF",
			SidebarUnavailableForeground:   "#3E4B59",
			SidebarDiffAddForeground:       "#7FD962",
			SidebarDiffDelForeground:       "#E06C75",
			BoxBorderForeground:            "",
			BoxBorderBackground:            "",
		},
		Keymap: Keymap{
			Normal: map[string]string{
				"h":             "move_left",
				"j":             "move_down",
				"k":             "move_up",
				"l":             "move_right",
				"left":          "move_left",
				"down":          "move_down",
				"up":            "move_up",
				"right":         "move_right",
				"home":          "line_start",
				"end":           "line_end",
				"cmd+home":      "file_start",
				"cmd+end":       "file_end",
				"cmd+left":      "word_left",
				"cmd+right":     "word_right",
				"cmd+up":        "move_line_up",
				"cmd+down":      "move_line_down",
				"cmd+l":         "toggle_line_numbers",
				"cmd+b":         "branch_picker",
				"cmd+shift+w":   "worktree_menu",
				"cmd+shift+n":   "worktree_new",
				"cmd+shift+s":   "worktree_switch",
				"cmd+shift+d":   "worktree_remove",
				"cmd+shift+r":   "worktree_refresh",
				"cmd+o":         "open_file_tree",
				"alt+left":      "focus_prev_pane",
				"alt+right":     "focus_next_pane",
				"alt+up":        "focus_editor",
				"alt+down":      "focus_command",
				"alt+1":         "toggle_sidebar",
				"alt+2":         "focus_editor",
				"cmd+y":         "delete_line",
				"del":           "delete_char",
				"cmd+backspace": "delete_word_left",
				"cmd+del":       "delete_word_right",
				"ctrl+home":     "file_start",
				"ctrl+end":      "file_end",
				"ctrl+y":        "scroll_up",
				"ctrl+e":        "scroll_down",
				"pgup":          "page_up",
				"pgdn":          "page_down",
				"i":             "enter_insert",
				":":             "enter_command",
				"u":             "undo",
				"U":             "redo",
				"ctrl+c":        "quit",
				"ctrl+r":        "redo",
				"tab":           "indent",
				"shift+tab":     "unindent",
				"cmd+a":         "select_all",
				"cmd+g":         "goto_line_prompt",
				"f3":            "git_next_change",
				"shift+f3":      "git_prev_change",

				// Helix-style motions
				"w": "word_forward",
				"b": "word_backward",
				"e": "word_end",
				"g": "goto_mode",
				"G": "goto_line",
				"f": "find_char",
				"F": "find_char_backward",
				"t": "till_char",
				"T": "till_char_backward",

				// Helix-style editing
				"d": "delete",
				"c": "change",
				"y": "yank",
				"p": "paste",
				"P": "paste_before",
				"o": "open_below",
				"O": "open_above",
				"a": "append",
				"A": "append_line_end",
				"I": "insert_line_start",
				"r": "replace_char",
				"J": "join_lines",

				// Helix-style selection
				"v": "toggle_select",
				"x": "extend_line",
				";": "collapse_selection",
				"%": "select_all",
				">": "indent",
				"<": "unindent",

				// Space mode
				"space": "space_mode",

				// Match mode
				"m": "match_mode",

				// View mode
				"z": "view_mode",

				// Search
				"/":     "search_forward",
				"?":     "search_backward",
				"n":     "search_next",
				"N":     "search_prev",
				"cmd+f": "search_fuzzy",
				"cmd+e": "search_regex",

				// Special
				"shift+enter": "insert_line_above",

				// Terminal zoom
				"=": "terminal_zoom_in",

				// Selection scope
				"alt+shift+up":   "expand_selection",
				"alt+shift+down": "shrink_selection",

				// File operations
				"cmd+s": "save",

				"M": "merge_mode",
			},
			Insert: map[string]string{
				"esc":           "enter_normal",
				"left":          "move_left",
				"down":          "move_down",
				"up":            "move_up",
				"right":         "move_right",
				"home":          "line_start",
				"end":           "line_end",
				"cmd+home":      "file_start",
				"cmd+end":       "file_end",
				"cmd+left":      "word_left",
				"cmd+right":     "word_right",
				"cmd+up":        "move_line_up",
				"cmd+down":      "move_line_down",
				"cmd+l":         "toggle_line_numbers",
				"cmd+b":         "branch_picker",
				"cmd+shift+w":   "worktree_menu",
				"cmd+shift+n":   "worktree_new",
				"cmd+shift+s":   "worktree_switch",
				"cmd+shift+d":   "worktree_remove",
				"cmd+shift+r":   "worktree_refresh",
				"cmd+o":         "open_file_tree",
				"alt+left":      "focus_prev_pane",
				"alt+right":     "focus_next_pane",
				"alt+up":        "focus_editor",
				"alt+down":      "focus_command",
				"alt+1":         "toggle_sidebar",
				"alt+2":         "focus_editor",
				"cmd+y":         "delete_line",
				"del":           "delete_char",
				"cmd+backspace": "delete_word_left",
				"cmd+del":       "delete_word_right",
				"cmd+enter":     "insert_line_below",
				"ctrl+home":     "file_start",
				"ctrl+end":      "file_end",
				"ctrl+y":        "scroll_up",
				"ctrl+e":        "scroll_down",
				"pgup":          "page_up",
				"pgdn":          "page_down",
				"backspace":     "backspace",
				"enter":         "newline",
				"tab":           "indent",
				"shift+tab":     "unindent",
				"cmd+a":         "select_all",
				"f3":            "git_next_change",
				"shift+f3":      "git_prev_change",
				"shift+enter":   "insert_line_above",

				// File operations
				"cmd+s": "save",
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
	md, err := toml.Decode(string(data), &userCfg)
	if err != nil {
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
	if userCfg.Editor.SidebarWidth != "" {
		cfg.Editor.SidebarWidth = userCfg.Editor.SidebarWidth
	}
	if userCfg.Editor.SidebarMinWidth > 0 {
		cfg.Editor.SidebarMinWidth = userCfg.Editor.SidebarMinWidth
	}
	if userCfg.Editor.SidebarMaxWidth != "" {
		cfg.Editor.SidebarMaxWidth = userCfg.Editor.SidebarMaxWidth
	}
	if md.IsDefined("editor", "sidebar-close-on-select") {
		cfg.Editor.SidebarCloseOnSelect = userCfg.Editor.SidebarCloseOnSelect
	}
	if md.IsDefined("editor", "file-tree-show-hidden") {
		cfg.Editor.FileTreeShowHidden = userCfg.Editor.FileTreeShowHidden
	}
	if md.IsDefined("editor", "file-tree-show-ignored") {
		cfg.Editor.FileTreeShowIgnored = userCfg.Editor.FileTreeShowIgnored
	}
	if md.IsDefined("editor", "highlight-max-bytes") {
		if userCfg.Editor.HighlightMaxBytes < 0 {
			cfg.Editor.HighlightMaxBytes = 0
		} else {
			cfg.Editor.HighlightMaxBytes = userCfg.Editor.HighlightMaxBytes
		}
	}
	if md.IsDefined("editor", "auto-reload-on-changes") {
		cfg.Editor.AutoReloadOnChanges = userCfg.Editor.AutoReloadOnChanges
	}
	if md.IsDefined("editor", "auto-reload-stabilize-ms") {
		if userCfg.Editor.AutoReloadStabilizeMS < 0 {
			cfg.Editor.AutoReloadStabilizeMS = 0
		} else {
			cfg.Editor.AutoReloadStabilizeMS = userCfg.Editor.AutoReloadStabilizeMS
		}
	}
	if md.IsDefined("editor", "auto-reload-max-retries") {
		if userCfg.Editor.AutoReloadMaxRetries < 1 {
			cfg.Editor.AutoReloadMaxRetries = 1
		} else {
			cfg.Editor.AutoReloadMaxRetries = userCfg.Editor.AutoReloadMaxRetries
		}
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
	mergeTheme(&cfg.Theme, userCfg.Theme)
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
	if src.StatuslineWarningForeground != "" {
		dst.StatuslineWarningForeground = src.StatuslineWarningForeground
	}
	if src.StatuslineWarningBackground != "" {
		dst.StatuslineWarningBackground = src.StatuslineWarningBackground
	}
	if src.MergeLocalBackground != "" {
		dst.MergeLocalBackground = src.MergeLocalBackground
	}
	if src.MergeRemoteBackground != "" {
		dst.MergeRemoteBackground = src.MergeRemoteBackground
	}
	if src.MergeHeaderForeground != "" {
		dst.MergeHeaderForeground = src.MergeHeaderForeground
	}
	if src.MergeHeaderBackground != "" {
		dst.MergeHeaderBackground = src.MergeHeaderBackground
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
	if src.SidebarFilterFileForeground != "" {
		dst.SidebarFilterFileForeground = src.SidebarFilterFileForeground
	}
	if src.SidebarFilterDirForeground != "" {
		dst.SidebarFilterDirForeground = src.SidebarFilterDirForeground
	}
	if src.NotificationForeground != "" {
		dst.NotificationForeground = src.NotificationForeground
	}
	if src.NotificationBackground != "" {
		dst.NotificationBackground = src.NotificationBackground
	}
	if src.NotificationFadeForeground != "" {
		dst.NotificationFadeForeground = src.NotificationFadeForeground
	}
	if src.NotificationFadeBackground != "" {
		dst.NotificationFadeBackground = src.NotificationFadeBackground
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
	if src.SyntaxYAMLKey != "" {
		dst.SyntaxYAMLKey = src.SyntaxYAMLKey
	}
	if src.SyntaxYAMLValue != "" {
		dst.SyntaxYAMLValue = src.SyntaxYAMLValue
	}
	if src.SyntaxYAMLListItem != "" {
		dst.SyntaxYAMLListItem = src.SyntaxYAMLListItem
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
	if src.SidebarForeground != "" {
		dst.SidebarForeground = src.SidebarForeground
	}
	if src.SidebarBackground != "" {
		dst.SidebarBackground = src.SidebarBackground
	}
	if src.SidebarDirForeground != "" {
		dst.SidebarDirForeground = src.SidebarDirForeground
	}
	if src.SidebarSelectedForeground != "" {
		dst.SidebarSelectedForeground = src.SidebarSelectedForeground
	}
	if src.SidebarSelectedBackground != "" {
		dst.SidebarSelectedBackground = src.SidebarSelectedBackground
	}
	if src.SidebarHeaderForeground != "" {
		dst.SidebarHeaderForeground = src.SidebarHeaderForeground
	}
	if src.SidebarHeaderBackground != "" {
		dst.SidebarHeaderBackground = src.SidebarHeaderBackground
	}
	if src.SidebarBorderForeground != "" {
		dst.SidebarBorderForeground = src.SidebarBorderForeground
	}
	if src.SidebarHiddenForeground != "" {
		dst.SidebarHiddenForeground = src.SidebarHiddenForeground
	}
	if src.SidebarIgnoredForeground != "" {
		dst.SidebarIgnoredForeground = src.SidebarIgnoredForeground
	}
	if src.SidebarIndicatorForeground != "" {
		dst.SidebarIndicatorForeground = src.SidebarIndicatorForeground
	}
	if src.SidebarStatusOnlineForeground != "" {
		dst.SidebarStatusOnlineForeground = src.SidebarStatusOnlineForeground
	}
	if src.SidebarStatusOfflineForeground != "" {
		dst.SidebarStatusOfflineForeground = src.SidebarStatusOfflineForeground
	}
	if src.SidebarHotkeyForeground != "" {
		dst.SidebarHotkeyForeground = src.SidebarHotkeyForeground
	}
	if src.SidebarUnavailableForeground != "" {
		dst.SidebarUnavailableForeground = src.SidebarUnavailableForeground
	}
	if src.SidebarDiffAddForeground != "" {
		dst.SidebarDiffAddForeground = src.SidebarDiffAddForeground
	}
	if src.SidebarDiffDelForeground != "" {
		dst.SidebarDiffDelForeground = src.SidebarDiffDelForeground
	}
	if src.BoxBorderForeground != "" {
		dst.BoxBorderForeground = src.BoxBorderForeground
	}
	if src.BoxBorderBackground != "" {
		dst.BoxBorderBackground = src.BoxBorderBackground
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

// UpdateEditorAutoReloadOnChanges persists auto-reload-on-changes in config.
func UpdateEditorAutoReloadOnChanges(enabled bool) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	cfg := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	section, ok := cfg["editor"].(map[string]interface{})
	if !ok || section == nil {
		section = map[string]interface{}{}
		cfg["editor"] = section
	}
	section["auto-reload-on-changes"] = enabled
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// UpdateEditorSidebarWidth persists sidebar-width in config.
func UpdateEditorSidebarWidth(width string) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	cfg := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	section, ok := cfg["editor"].(map[string]interface{})
	if !ok || section == nil {
		section = map[string]interface{}{}
		cfg["editor"] = section
	}
	section["sidebar-width"] = width
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// UpdateEditorProfile persists behavior profile in config.
func UpdateEditorProfile(profile string) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	cfg := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	section, ok := cfg["editor"].(map[string]interface{})
	if !ok || section == nil {
		section = map[string]interface{}{}
		cfg["editor"] = section
	}
	section["profile"] = profile
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
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
