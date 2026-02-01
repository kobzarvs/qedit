package ui

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
)

// StylesFromConfig builds editor styles from the config theme.
func StylesFromConfig(cfg config.Config) editor.EditorStyles {
	theme := cfg.Theme

	colors := make(map[string]tcell.Color)
	resolve := func(value string, fallback tcell.Color) tcell.Color {
		if value == "" {
			return fallback
		}
		if c, ok := colors[value]; ok {
			return c
		}
		return parseColor(value, fallback)
	}

	colors["foreground"] = parseColor(theme.Foreground, tcell.ColorWhite)
	colors["background"] = parseColor(theme.Background, tcell.ColorBlack)
	colors["statusline-foreground"] = resolve(theme.StatuslineForeground, tcell.ColorBlack)
	colors["statusline-background"] = resolve(theme.StatuslineBackground, tcell.ColorGray)
	colors["commandline-foreground"] = resolve(theme.CommandlineForeground, colors["statusline-foreground"])
	colors["commandline-background"] = resolve(theme.CommandlineBackground, colors["statusline-background"])
	colors["line-number-foreground"] = resolve(theme.LineNumberForeground, tcell.ColorGray)
	colors["line-number-active-foreground"] = resolve(theme.LineNumberActiveForeground, colors["foreground"])
	colors["selection-foreground"] = resolve(theme.SelectionForeground, colors["foreground"])
	colors["selection-background"] = resolve(theme.SelectionBackground, colors["background"])
	colors["search-foreground"] = resolve(theme.SearchMatchForeground, tcell.ColorBlack)
	colors["search-background"] = resolve(theme.SearchMatchBackground, tcell.ColorYellow)
	colors["syntax-keyword"] = resolve(theme.SyntaxKeyword, colors["foreground"])
	colors["syntax-string"] = resolve(theme.SyntaxString, colors["foreground"])
	colors["syntax-comment"] = resolve(theme.SyntaxComment, colors["foreground"])
	colors["syntax-type"] = resolve(theme.SyntaxType, colors["foreground"])
	colors["syntax-function"] = resolve(theme.SyntaxFunction, colors["foreground"])
	colors["syntax-number"] = resolve(theme.SyntaxNumber, colors["foreground"])
	colors["syntax-constant"] = resolve(theme.SyntaxConstant, colors["foreground"])
	colors["syntax-operator"] = resolve(theme.SyntaxOperator, colors["foreground"])
	colors["syntax-punctuation"] = resolve(theme.SyntaxPunctuation, colors["foreground"])
	colors["syntax-field"] = resolve(theme.SyntaxField, colors["foreground"])
	colors["syntax-builtin"] = resolve(theme.SyntaxBuiltin, colors["foreground"])
	colors["syntax-unknown"] = resolve(theme.SyntaxUnknown, tcell.ColorRed)
	colors["syntax-variable"] = resolve(theme.SyntaxVariable, colors["foreground"])
	colors["syntax-parameter"] = resolve(theme.SyntaxParameter, colors["foreground"])
	colors["branch-foreground"] = resolve(theme.BranchForeground, colors["statusline-foreground"])
	colors["branch-background"] = resolve(theme.BranchBackground, colors["statusline-background"])

	mainBranchDefaultFg := tcell.NewRGBColor(144, 238, 144) // #90EE90
	colors["main-branch-foreground"] = resolve(theme.MainBranchForeground, mainBranchDefaultFg)
	colors["main-branch-background"] = resolve(theme.MainBranchBackground, colors["statusline-background"])

	layoutUSFg := tcell.NewRGBColor(144, 238, 144) // #90EE90
	layoutRUFg := tcell.NewRGBColor(135, 206, 250) // #87CEFA
	colors["layout-us-foreground"] = layoutUSFg
	colors["layout-ru-foreground"] = layoutRUFg
	colors["layout-other-foreground"] = colors["statusline-foreground"]

	colors["autocomplete-background"] = resolve(theme.AutocompleteBackground, colors["commandline-background"])
	colors["autocomplete-hotkey"] = resolve(theme.AutocompleteHotkey, tcell.ColorWhite)
	colors["autocomplete-description"] = resolve(theme.AutocompleteDescription, colors["commandline-foreground"])
	colors["autocomplete-group"] = resolve(theme.AutocompleteGroup, tcell.ColorGray)

	colors["sidebar-foreground"] = resolve(theme.SidebarForeground, colors["foreground"])
	colors["sidebar-background"] = resolve(theme.SidebarBackground, colors["background"])
	colors["sidebar-dir-foreground"] = resolve(theme.SidebarDirForeground, tcell.ColorBlue)
	colors["sidebar-selected-foreground"] = resolve(theme.SidebarSelectedForeground, colors["background"])
	colors["sidebar-selected-background"] = resolve(theme.SidebarSelectedBackground, tcell.ColorYellow)
	colors["sidebar-header-foreground"] = resolve(theme.SidebarHeaderForeground, colors["foreground"])
	colors["sidebar-header-background"] = resolve(theme.SidebarHeaderBackground, colors["statusline-background"])
	colors["sidebar-border-foreground"] = resolve(theme.SidebarBorderForeground, colors["line-number-foreground"])
	colors["sidebar-hidden-foreground"] = resolve(theme.SidebarHiddenForeground, colors["line-number-foreground"])
	colors["sidebar-ignored-foreground"] = resolve(theme.SidebarIgnoredForeground, colors["line-number-foreground"])
	colors["sidebar-indicator-foreground"] = resolve(theme.SidebarIndicatorForeground, tcell.ColorYellow)
	colors["sidebar-hotkey-foreground"] = resolve(theme.SidebarHotkeyForeground, tcell.ColorBlue)
	colors["sidebar-unavailable-foreground"] = resolve(theme.SidebarUnavailableForeground, colors["line-number-foreground"])
	colors["box-border-foreground"] = resolve(theme.BoxBorderForeground, colors["statusline-foreground"])
	colors["box-border-background"] = resolve(theme.BoxBorderBackground, colors["statusline-background"])

	main := style(colors["foreground"], colors["background"])
	status := style(colors["statusline-foreground"], colors["statusline-background"])
	command := style(colors["commandline-foreground"], colors["commandline-background"])
	lineNumber := style(colors["line-number-foreground"], colors["background"])
	lineNumberActive := style(colors["line-number-active-foreground"], colors["background"])
	selection := style(colors["selection-foreground"], colors["selection-background"])
	searchMatch := style(colors["search-foreground"], colors["search-background"])
	syntaxKeyword := style(colors["syntax-keyword"], colors["background"])
	syntaxString := style(colors["syntax-string"], colors["background"])
	syntaxComment := style(colors["syntax-comment"], colors["background"])
	syntaxType := style(colors["syntax-type"], colors["background"])
	syntaxFunction := style(colors["syntax-function"], colors["background"])
	syntaxNumber := style(colors["syntax-number"], colors["background"])
	syntaxConstant := style(colors["syntax-constant"], colors["background"])
	syntaxOperator := style(colors["syntax-operator"], colors["background"])
	syntaxPunctuation := style(colors["syntax-punctuation"], colors["background"])
	syntaxField := style(colors["syntax-field"], colors["background"])
	syntaxBuiltin := style(colors["syntax-builtin"], colors["background"])
	syntaxUnknown := style(colors["syntax-unknown"], colors["background"])
	syntaxVariable := style(colors["syntax-variable"], colors["background"])
	syntaxParameter := style(colors["syntax-parameter"], colors["background"])
	tableBorder := style(tcell.ColorWhite, colors["background"])
	branch := style(colors["branch-foreground"], colors["branch-background"])
	mainBranch := style(colors["main-branch-foreground"], colors["main-branch-background"])
	layoutUS := style(colors["layout-us-foreground"], colors["statusline-background"])
	layoutRU := style(colors["layout-ru-foreground"], colors["statusline-background"])
	layoutOther := style(colors["layout-other-foreground"], colors["statusline-background"])
	autoComplete := style(colors["autocomplete-description"], colors["autocomplete-background"])
	autoHotkey := style(colors["autocomplete-hotkey"], colors["autocomplete-background"])
	autoDesc := style(colors["autocomplete-description"], colors["autocomplete-background"])
	autoGroup := style(colors["autocomplete-group"], colors["autocomplete-background"])
	scrollIndicator := style(colors["line-number-active-foreground"], colors["background"])
	boxBorder := style(colors["box-border-foreground"], colors["box-border-background"])

	sidebarBase := style(colors["sidebar-foreground"], colors["sidebar-background"])
	sidebarDir := style(colors["sidebar-dir-foreground"], colors["sidebar-background"])
	sidebarSelected := style(colors["sidebar-selected-foreground"], colors["sidebar-selected-background"])
	sidebarHeader := style(colors["sidebar-header-foreground"], colors["sidebar-header-background"])
	sidebarBorder := style(colors["sidebar-border-foreground"], colors["sidebar-background"])
	sidebarHidden := style(colors["sidebar-hidden-foreground"], colors["sidebar-background"])
	sidebarIgnored := style(colors["sidebar-ignored-foreground"], colors["sidebar-background"])
	sidebarIndicator := style(colors["sidebar-indicator-foreground"], colors["sidebar-background"])
	sidebarHotkey := style(colors["sidebar-hotkey-foreground"], colors["sidebar-background"])
	sidebarUnavailable := style(colors["sidebar-unavailable-foreground"], colors["sidebar-background"])

	return editor.EditorStyles{
		Main:                    main,
		Status:                  status,
		Command:                 command,
		CommandCheckmark:        command,
		LineNumber:              lineNumber,
		LineNumberActive:        lineNumberActive,
		Selection:               selection,
		SearchMatch:             searchMatch,
		SyntaxKeyword:           syntaxKeyword,
		SyntaxString:            syntaxString,
		SyntaxComment:           syntaxComment,
		SyntaxType:              syntaxType,
		SyntaxFunction:          syntaxFunction,
		SyntaxNumber:            syntaxNumber,
		SyntaxConstant:          syntaxConstant,
		SyntaxOperator:          syntaxOperator,
		SyntaxPunctuation:       syntaxPunctuation,
		SyntaxField:             syntaxField,
		SyntaxBuiltin:           syntaxBuiltin,
		SyntaxUnknown:           syntaxUnknown,
		SyntaxVariable:          syntaxVariable,
		SyntaxParameter:         syntaxParameter,
		TableBorder:             tableBorder,
		Branch:                  branch,
		MainBranch:              mainBranch,
		LayoutUS:                layoutUS,
		LayoutRU:                layoutRU,
		LayoutOther:             layoutOther,
		AutoComplete:            autoComplete,
		AutoCompleteHotkey:      autoHotkey,
		AutoCompleteDescription: autoDesc,
		AutoCompleteGroup:       autoGroup,
		ScrollIndicator:         scrollIndicator,
		BranchMarker:            mainBranch,
		FilterActive:            selection,
		FilterInactive:          command,
		BoxBorder:               boxBorder,
		Sidebar: editor.SidebarStyles{
			Base:               sidebarBase,
			Dir:                sidebarDir,
			Selected:           sidebarSelected,
			SelectedBackground: editor.Color(colors["sidebar-selected-background"]),
			Header:             sidebarHeader,
			Border:             sidebarBorder,
			Hidden:             sidebarHidden,
			Ignored:            sidebarIgnored,
			Indicator:          sidebarIndicator,
			Hotkey:             sidebarHotkey,
			Unavailable:        sidebarUnavailable,
			Current:            sidebarIndicator,
		},
	}
}

func style(fg, bg tcell.Color) editor.Style {
	return wrapStyle(tcell.StyleDefault.Foreground(fg).Background(bg))
}

func parseColor(name string, fallback tcell.Color) tcell.Color {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}
	if strings.HasPrefix(name, "#") && len(name) == 7 {
		if v, err := strconv.ParseInt(name[1:], 16, 32); err == nil {
			return tcell.NewHexColor(int32(v))
		}
		return fallback
	}
	if len(name) == 6 && isHex(name) {
		if v, err := strconv.ParseInt(name, 16, 32); err == nil {
			return tcell.NewHexColor(int32(v))
		}
		return fallback
	}
	if len(name) == 8 && strings.HasPrefix(name, "0x") && isHex(name[2:]) {
		if v, err := strconv.ParseInt(name[2:], 16, 32); err == nil {
			return tcell.NewHexColor(int32(v))
		}
		return fallback
	}
	name = strings.ToLower(name)
	if name == "default" {
		return tcell.ColorDefault
	}
	c := tcell.GetColor(name)
	if c == tcell.ColorDefault {
		return fallback
	}
	return c
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
