package editor

// baseStyle is a minimal Style implementation used as a safe default.
type baseStyle struct {
	fg    Color
	bg    Color
	attrs AttrMask
}

func (s baseStyle) Decompose() (Color, Color, AttrMask) {
	return s.fg, s.bg, s.attrs
}

func (s baseStyle) Foreground(c Color) Style {
	s.fg = c
	return s
}

func (s baseStyle) Background(c Color) Style {
	s.bg = c
	return s
}

func defaultEditorStyles() EditorStyles {
	base := baseStyle{}
	return EditorStyles{
		Main:                    base,
		Status:                  base,
		StatusWarning:           base,
		MergeLocal:              base,
		MergeRemote:             base,
		MergeHeader:             base,
		Command:                 base,
		CommandCheckmark:        base,
		LineNumber:              base,
		LineNumberActive:        base,
		Selection:               base,
		SearchMatch:             base,
		SyntaxKeyword:           base,
		SyntaxString:            base,
		SyntaxComment:           base,
		SyntaxType:              base,
		SyntaxFunction:          base,
		SyntaxNumber:            base,
		SyntaxConstant:          base,
		SyntaxOperator:          base,
		SyntaxPunctuation:       base,
		SyntaxField:             base,
		SyntaxBuiltin:           base,
		SyntaxUnknown:           base,
		SyntaxVariable:          base,
		SyntaxParameter:         base,
		TableBorder:             base,
		Branch:                  base,
		MainBranch:              base,
		LayoutUS:                base,
		LayoutRU:                base,
		LayoutOther:             base,
		AutoComplete:            base,
		AutoCompleteHotkey:      base,
		AutoCompleteDescription: base,
		AutoCompleteGroup:       base,
		ScrollIndicator:         base,
		BranchMarker:            base,
		FilterActive:            base,
		FilterInactive:          base,
		BoxBorder:               base,
		Sidebar: SidebarStyles{
			Base:               base,
			Dir:                base,
			Selected:           base,
			SelectedBackground: 0,
			Header:             base,
			Border:             base,
			Hidden:             base,
			Ignored:            base,
			Indicator:          base,
			Hotkey:             base,
			Unavailable:        base,
			Current:            base,
		},
	}
}
