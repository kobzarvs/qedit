package editor

// EditorStyles bundles all UI styles needed by the editor renderer.
type EditorStyles struct {
	Main                    Style
	Status                  Style
	StatusWarning           Style
	MergeLocal              Style
	MergeRemote             Style
	MergeHeader             Style
	Command                 Style
	CommandCheckmark        Style
	LineNumber              Style
	LineNumberActive        Style
	Selection               Style
	SearchMatch             Style
	SyntaxKeyword           Style
	SyntaxString            Style
	SyntaxComment           Style
	SyntaxType              Style
	SyntaxFunction          Style
	SyntaxNumber            Style
	SyntaxConstant          Style
	SyntaxOperator          Style
	SyntaxPunctuation       Style
	SyntaxField             Style
	SyntaxBuiltin           Style
	SyntaxUnknown           Style
	SyntaxVariable          Style
	SyntaxParameter         Style
	SyntaxYAMLKey           Style
	SyntaxYAMLValue         Style
	SyntaxYAMLListItem      Style
	TableBorder             Style
	Branch                  Style
	MainBranch              Style
	LayoutUS                Style
	LayoutRU                Style
	LayoutOther             Style
	AutoComplete            Style
	AutoCompleteHotkey      Style
	AutoCompleteDescription Style
	AutoCompleteGroup       Style
	ScrollIndicator         Style
	BranchMarker            Style
	FilterActive            Style
	FilterInactive          Style
	BoxBorder               Style
	NotificationFade        []Style
	Sidebar                 SidebarStyles
}

// SetStyles applies UI styles to the editor.
func (e *Editor) SetStyles(s EditorStyles) {
	e.styleMain = s.Main
	e.styleStatus = s.Status
	e.styleStatusWarning = s.StatusWarning
	e.styleMergeLocal = s.MergeLocal
	e.styleMergeRemote = s.MergeRemote
	e.styleMergeHeader = s.MergeHeader
	e.styleCommand = s.Command
	e.styleCommandCheckmark = s.CommandCheckmark
	e.styleLineNumber = s.LineNumber
	e.styleLineNumberActive = s.LineNumberActive
	e.styleSelection = s.Selection
	e.styleSearchMatch = s.SearchMatch
	e.styleSyntaxKeyword = s.SyntaxKeyword
	e.styleSyntaxString = s.SyntaxString
	e.styleSyntaxComment = s.SyntaxComment
	e.styleSyntaxType = s.SyntaxType
	e.styleSyntaxFunction = s.SyntaxFunction
	e.styleSyntaxNumber = s.SyntaxNumber
	e.styleSyntaxConstant = s.SyntaxConstant
	e.styleSyntaxOperator = s.SyntaxOperator
	e.styleSyntaxPunctuation = s.SyntaxPunctuation
	e.styleSyntaxField = s.SyntaxField
	e.styleSyntaxBuiltin = s.SyntaxBuiltin
	e.styleSyntaxUnknown = s.SyntaxUnknown
	e.styleSyntaxVariable = s.SyntaxVariable
	e.styleSyntaxParameter = s.SyntaxParameter
	e.styleSyntaxYAMLKey = s.SyntaxYAMLKey
	e.styleSyntaxYAMLValue = s.SyntaxYAMLValue
	e.styleSyntaxYAMLListItem = s.SyntaxYAMLListItem
	e.styleTableBorder = s.TableBorder
	e.styleBranch = s.Branch
	e.styleMainBranch = s.MainBranch
	e.styleLayoutUS = s.LayoutUS
	e.styleLayoutRU = s.LayoutRU
	e.styleLayoutOther = s.LayoutOther
	e.styleAutoComplete = s.AutoComplete
	e.styleAutoCompleteHotkey = s.AutoCompleteHotkey
	e.styleAutoCompleteDescription = s.AutoCompleteDescription
	e.styleAutoCompleteGroup = s.AutoCompleteGroup
	e.styleScrollIndicator = s.ScrollIndicator
	e.styleBranchMarker = s.BranchMarker
	e.styleFilterActive = s.FilterActive
	e.styleFilterInactive = s.FilterInactive
	e.styleBoxBorder = s.BoxBorder
	e.notificationFadeStyles = s.NotificationFade
	if len(s.NotificationFade) > 0 {
		e.styleNotificationBright = s.NotificationFade[0]
	} else {
		e.styleNotificationBright = s.Status
	}
	e.sidebarStyles = s.Sidebar
}
