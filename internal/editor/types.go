package editor

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeCommand
	ModeBranchPicker
	ModeSearch
	ModeMerge
)

const (
	actionMoveLeft           = "move_left"
	actionMoveRight          = "move_right"
	actionMoveUp             = "move_up"
	actionMoveDown           = "move_down"
	actionWordLeft           = "word_left"
	actionWordRight          = "word_right"
	actionLineStart          = "line_start"
	actionLineEnd            = "line_end"
	actionFileStart          = "file_start"
	actionFileEnd            = "file_end"
	actionPageUp             = "page_up"
	actionPageDown           = "page_down"
	actionMoveLineUp         = "move_line_up"
	actionMoveLineDown       = "move_line_down"
	actionToggleLineNumbers  = "toggle_line_numbers"
	actionBranchPicker       = "branch_picker"
	actionWorktreeMenu       = "worktree_menu"
	actionWorktreeNew        = "worktree_new"
	actionWorktreeSwitch     = "worktree_switch"
	actionWorktreeRemove     = "worktree_remove"
	actionWorktreeRefresh    = "worktree_refresh"
	actionOpenFileTree       = "open_file_tree"
	actionToggleSidebar      = "toggle_sidebar"
	actionToggleSidebarFocus = "toggle_sidebar_focus"
	actionFocusSidebar       = "focus_sidebar"
	actionFocusPrevPane      = "focus_prev_pane"
	actionFocusNextPane      = "focus_next_pane"
	actionFocusEditor        = "focus_editor"
	actionFocusCommandLine   = "focus_command"
	actionEnterInsert        = "enter_insert"
	actionEnterNormal        = "enter_normal"
	actionEnterCommand       = "enter_command"
	actionQuit               = "quit"
	actionBackspace          = "backspace"
	actionNewline            = "newline"
	actionInsertTab          = "insert_tab"
	actionUndo               = "undo"
	actionRedo               = "redo"
	actionDeleteLine         = "delete_line"
	actionDeleteChar         = "delete_char"
	actionDeleteWordLeft     = "delete_word_left"
	actionDeleteWordRight    = "delete_word_right"
	actionInsertLineBelow    = "insert_line_below"
	actionUndoLine           = "undo_line"
	actionScrollUp           = "scroll_up"
	actionScrollDown         = "scroll_down"
	actionIndent             = "indent"
	actionUnindent           = "unindent"
	actionSelectAll          = "select_all"

	// Helix-style motions
	actionWordForward      = "word_forward"       // w - move to next word start
	actionWordBackward     = "word_backward"      // b - move to previous word start
	actionWordEnd          = "word_end"           // e - move to word end
	actionGotoMode         = "goto_mode"          // g - enter goto mode
	actionGotoLine         = "goto_line"          // G - go to last line (or specific line)
	actionGotoLinePrompt   = "goto_line_prompt"   // cmd+g - prompt for line number
	actionGotoFirstLine    = "goto_first_line"    // gg - go to first line
	actionGotoFileEnd      = "goto_file_end"      // ge - go to end of file
	actionFindChar         = "find_char"          // f - find char forward
	actionFindCharBackward = "find_char_backward" // F - find char backward
	actionTillChar         = "till_char"          // t - till char forward
	actionTillCharBackward = "till_char_backward" // T - till char backward

	// Helix-style editing
	actionDelete          = "delete"            // d - delete selection
	actionChange          = "change"            // c - change (delete + insert)
	actionYank            = "yank"              // y - yank/copy
	actionPaste           = "paste"             // p - paste after
	actionPasteBefore     = "paste_before"      // P - paste before
	actionOpenBelow       = "open_below"        // o - open line below
	actionOpenAbove       = "open_above"        // O - open line above
	actionAppend          = "append"            // a - append (insert after cursor)
	actionAppendLineEnd   = "append_line_end"   // A - insert at line end
	actionInsertLineStart = "insert_line_start" // I - insert at first non-whitespace
	actionReplaceChar     = "replace_char"      // r - replace with single char
	actionJoinLines       = "join_lines"        // J - join lines

	// Helix-style selection
	actionToggleSelect      = "toggle_select"      // v - toggle selection mode
	actionExtendLine        = "extend_line"        // x - extend to full line
	actionCollapseSelection = "collapse_selection" // ; - collapse selection to cursor
	actionFlipSelection     = "flip_selection"     // Alt+; - flip selection anchor

	// Space mode
	actionSpaceMode = "space_mode" // Space - open space menu

	// Match mode
	actionMatchMode = "match_mode" // m - enter match mode

	// View mode
	actionViewMode = "view_mode" // z - enter view mode

	// Merge mode
	actionMergeMode = "merge_mode" // Shift+M - enter merge mode

	// Search
	actionSearchForward  = "search_forward"  // / - exact search forward
	actionSearchBackward = "search_backward" // ? - exact search backward
	actionSearchFuzzy    = "search_fuzzy"    // Cmd+F - fuzzy search forward
	actionSearchRegex    = "search_regex"    // Cmd+E - regex search forward
	actionSearchNext     = "search_next"     // n - next match
	actionSearchPrev     = "search_prev"     // N - previous match

	// Git
	actionGitNextChange = "git_next_change" // F3 - next git change
	actionGitPrevChange = "git_prev_change" // Shift+F3 - previous git change

	// Special
	actionInsertLineAbove = "insert_line_above" // Shift+Enter - insert indented line above cursor

	// Terminal zoom
	actionTerminalZoomIn = "terminal_zoom_in" // Cmd+= - zoom in terminal 5x

	// Selection scope
	actionExpandSelection = "expand_selection" // Alt+Shift+Up - expand selection to parent node
	actionShrinkSelection = "shrink_selection" // Alt+Shift+Down - shrink selection to child node

	// File operations
	actionSave = "save" // Cmd+S - save file

	// Buffer management
	actionBufferPicker     = "buffer_picker"      // open buffer picker in sidebar
	actionGotoNextBuffer   = "goto_next_buffer"   // gn - next buffer
	actionGotoPrevBuffer   = "goto_prev_buffer"   // gp - previous buffer
	actionGotoLastAccessed = "goto_last_accessed" // ga - last accessed buffer
	actionCloseBuffer      = "close_buffer"       // close current buffer
)

// CommandInfo describes an available command with description
type CommandInfo struct {
	Name        string
	Description string
	Group       string
}

// WorktreeInfo represents a git worktree entry for UI.
type WorktreeInfo struct {
	Path   string
	Branch string
}

// Command groups for autocomplete
const (
	CmdGroupFile = "File"
	CmdGroupEdit = "Edit"
	CmdGroupView = "View"
	CmdGroupGit  = "Git"
)

// SpaceMenuItem represents an item in the space menu
type SpaceMenuItem struct {
	Key         rune
	Label       string
	Action      string
	Implemented bool // whether this action is implemented
}

// SpaceMenuItems defines the space menu structure
var SpaceMenuItems = []SpaceMenuItem{
	{'f', "Open file picker", "file_picker", false},
	{'F', "Open file picker at cwd", "file_picker_cwd", false},
	{'e', "Open file explorer", "file_explorer", false},
	{'E', "Open file explorer at buffer dir", "file_explorer_buffer", false},
	{'b', "Open buffer picker", "buffer_picker", true},
	{'j', "Open jumplist picker", "jumplist_picker", false},
	{'s', "Open symbol picker", "symbol_picker", false},
	{'S', "Open workspace symbol picker", "workspace_symbol_picker", false},
	{'d', "Open diagnostic picker", "diagnostic_picker", false},
	{'D', "Open workspace diagnostic picker", "workspace_diagnostic_picker", false},
	{'g', "Open changed file picker", "changed_file_picker", false},
	{'a', "Perform code action", "code_action", false},
	{'\'', "Open last picker", "last_picker", false},
	{'G', "Debug (experimental)", "debug", false},
	{'w', "Window mode", "window_mode", true},
	{'y', "Yank to clipboard", "yank_clipboard", true},
	{'Y', "Yank main to clipboard", "yank_main_clipboard", true},
	{'p', "Paste from clipboard", "paste_clipboard", true},
	{'P', "Paste before from clipboard", "paste_clipboard_before", true},
	{'R', "Replace with clipboard", "replace_clipboard", false},
	{'/', "Global search", "global_search", false},
	{'k', "Show docs for item", "show_docs", false},
	{'r', "Rename symbol", "rename_symbol", false},
	{'h', "Select symbol references", "select_references", false},
	{'c', "Comment/uncomment", "toggle_comment", true},
	{'C', "Block comment/uncomment", "toggle_block_comment", false},
	{'?', "Show all keybindings", "show_keybindings", true},
}

// GotoMenuItems defines the goto mode menu (g prefix)
var GotoMenuItems = []SpaceMenuItem{
	{'g', "Go to file start", "goto_first_line", true},
	{'e', "Go to file end", "goto_file_end", true},
	{'h', "Go to line start", "line_start", true},
	{'l', "Go to line end", "line_end", true},
	{'s', "Go to first non-whitespace", "goto_first_nonblank", true},
	{'d', "Go to definition", "goto_definition", true},
	{'D', "Go to declaration", "goto_declaration", true},
	{'y', "Go to type definition", "goto_type_definition", true},
	{'r', "Go to references", "goto_references", true},
	{'i', "Go to implementation", "goto_implementation", true},
	{'t', "Go to window top", "goto_window_top", true},
	{'c', "Go to window center", "goto_window_center", true},
	{'b', "Go to window bottom", "goto_window_bottom", true},
	{'a', "Go to last accessed file", "goto_last_accessed", true},
	{'m', "Go to last modified file", "goto_last_modified", false},
	{'n', "Go to next buffer", "goto_next_buffer", true},
	{'p', "Go to previous buffer", "goto_prev_buffer", true},
	{'.', "Go to last change", "goto_last_change", false},
}

// MatchMenuItems defines the match mode menu (m prefix)
var MatchMenuItems = []SpaceMenuItem{
	{'m', "Go to matching bracket", "match_bracket", true},
	{'s', "Surround add", "surround_add", false},
	{'r', "Surround replace", "surround_replace", false},
	{'d', "Surround delete", "surround_delete", false},
	{'a', "Select around object", "select_around", false},
	{'i', "Select inside object", "select_inside", false},
}

// ViewMenuItems defines the view/scroll mode menu (z prefix)
var ViewMenuItems = []SpaceMenuItem{
	{'c', "Center cursor line", "view_center", true},
	{'t', "Scroll cursor to top", "view_top", true},
	{'b', "Scroll cursor to bottom", "view_bottom", true},
	{'k', "Scroll up", "scroll_up", true},
	{'j', "Scroll down", "scroll_down", true},
}

// WindowMenuItems defines the window mode menu (space-w prefix)
var WindowMenuItems = []SpaceMenuItem{
	{'w', "Switch to next window", "window_next", false},
	{'v', "Vertical split", "window_vsplit", false},
	{'s', "Horizontal split", "window_hsplit", false},
	{'h', "Move to left window", "window_left", false},
	{'j', "Move to window below", "window_down", false},
	{'k', "Move to window above", "window_up", false},
	{'l', "Move to right window", "window_right", false},
	{'q', "Close window", "window_close", false},
	{'o', "Close other windows", "window_only", false},
}

type actionKind int

const (
	actionInsertRune actionKind = iota
	actionDeleteRune
	actionSplitLine
	actionJoinLine
	actionMoveLine
	actionInsertText // Bulk insert of multiple lines
	actionDeleteText // Bulk delete of multiple lines
)

type action struct {
	kind           actionKind
	pos            Cursor
	r              rune
	rowFrom        int
	rowTo          int
	group          uint64
	text           [][]rune // For bulk text operations
	endPos         Cursor   // End position for bulk delete
	selectionStart Cursor   // Selection to restore on undo
	selectionEnd   Cursor   // Selection to restore on undo
	hasSelection   bool     // Whether to restore selection on undo
}

// actionJSON is used for serializing actions to changelog files
type actionJSON struct {
	Kind           int      `json:"k"`
	PosRow         int      `json:"pr"`
	PosCol         int      `json:"pc"`
	R              rune     `json:"r,omitempty"`
	RowFrom        int      `json:"rf,omitempty"`
	RowTo          int      `json:"rt,omitempty"`
	Group          uint64   `json:"g"`
	Text           []string `json:"t,omitempty"`
	EndPosRow      int      `json:"er,omitempty"`
	EndPosCol      int      `json:"ec,omitempty"`
	SelectionStart [2]int   `json:"ss,omitempty"`
	SelectionEnd   [2]int   `json:"se,omitempty"`
	HasSelection   bool     `json:"hs,omitempty"`
}

type Cursor struct {
	Row int
	Col int
}

type TextEdit struct {
	Valid          bool
	StartByte      int
	OldEndByte     int
	NewEndByte     int
	StartRow       int
	StartColBytes  int
	OldEndRow      int
	OldEndColBytes int
	NewEndRow      int
	NewEndColBytes int
}

type HighlightSpan struct {
	StartCol int
	EndCol   int
	Kind     string
}

type keymapSet struct {
	normal map[string]string
	insert map[string]string
}

// NodeRange represents a syntax node's position range
type NodeRange struct {
	StartRow int
	StartCol int
	EndRow   int
	EndCol   int
}

// LSPLocation represents a location returned by LSP
type LSPLocation struct {
	Path      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

type Editor struct {
	Buffer
	Selection
	UndoManager
	SearchState
	mode                         Mode
	document                     editorDocumentState
	file                         editorFileState
	bindings                     editorBindingsState
	commandLine                  editorCommandLineState
	styleMain                    Style
	styleStatus                  Style
	styleStatusWarning           Style
	styleMergeLocal              Style
	styleMergeRemote             Style
	styleMergeHeader             Style
	styleDiffInlineLocal         Style
	styleDiffInlineRemote        Style
	styleCommand                 Style
	styleLineNumber              Style
	styleLineNumberActive        Style
	styleSelection               Style
	styleSearchMatch             Style
	styleSyntaxKeyword           Style
	styleSyntaxString            Style
	styleSyntaxComment           Style
	styleSyntaxType              Style
	styleSyntaxFunction          Style
	styleSyntaxNumber            Style
	styleSyntaxConstant          Style
	styleSyntaxOperator          Style
	styleSyntaxPunctuation       Style
	styleSyntaxField             Style
	styleSyntaxBuiltin           Style
	styleSyntaxUnknown           Style
	styleSyntaxVariable          Style
	styleSyntaxParameter         Style
	styleSyntaxYAMLKey           Style
	styleSyntaxYAMLValue         Style
	styleSyntaxYAMLListItem      Style
	styleTableBorder             Style
	styleBranch                  Style
	styleMainBranch              Style
	styleLayoutUS                Style
	styleLayoutRU                Style
	styleLayoutOther             Style
	styleAutoComplete            Style
	styleAutoCompleteHotkey      Style
	styleAutoCompleteDescription Style
	styleAutoCompleteGroup       Style
	styleCommandCheckmark        Style
	styleScrollIndicator         Style
	styleBranchMarker            Style
	styleFilterActive            Style
	styleFilterInactive          Style
	styleBoxBorder               Style
	styleNotificationBright      Style
	notificationFadeStyles       []Style
	display                      editorDisplayState
	profile                      editorProfileState
	ui                           editorUIFeedbackState
	viewport                     editorViewportState
	huge                         editorHugeFileState
	git                          editorGitState
	highlight                    editorHighlightState
	conflicts                    editorConflictState
	change                       editorChangeState
	branchPicker                 branchPickerState
	sidebar                      *Sidebar
	behaviorProfiles             behaviorProfileRegistry
	sidebarModes                 sidebarModeRegistry
	sidebarStyles                SidebarStyles
	formatters                   formatterRegistry
	languageFeatures             languageFeatureRegistry
	gitFeatures                  gitFeatureRegistry
	fileTree                     editorFileTreeState
	fileTreePreview              fileTreePreviewState
	interaction                  editorInteractionState
	runtime                      editorRuntimeDeps

	// Helix-style state
	clipboard       editorClipboardState
	modal           editorModalState
	keybindingsHelp keybindingsHelpState

	// Terminal zoom state
	zoom editorZoomState

	// Selection scope (expand/shrink)
	selectionScope selectionScopeState

	// LSP integration
	refsPicker refsPickerState

	// Command autocomplete state
	cmdAutoComplete commandAutocompleteState

	// Multi-buffer state
	buffers  *BufferManager // tracks multiple open buffers
	commands commandRegistry
	requests editorRequestState
}

// SearchMatch represents a match location
type SearchMatch struct {
	Row         int
	Col         int
	Length      int
	Score       int   // fuzzy match score (higher = better)
	MatchedCols []int // columns of matched chars within the word (for fuzzy highlight)
}

type LineNumberMode int

const (
	LineNumberOff LineNumberMode = iota
	LineNumberAbsolute
	LineNumberRelative
)

type tableAlign struct {
	left  bool
	right bool
}
