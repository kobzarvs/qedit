package editor

import (
	"strings"
)

func New(opts Options) *Editor {
	normal := make(map[string]string, len(opts.KeymapNormal))
	for k, v := range opts.KeymapNormal {
		normal[k] = v
	}
	insert := make(map[string]string, len(opts.KeymapInsert))
	for k, v := range opts.KeymapInsert {
		insert[k] = v
	}
	tabWidth := opts.TabWidth
	if tabWidth < 1 {
		tabWidth = 1
	}

	lineNumberMode := parseLineNumberMode(opts.LineNumbers)
	gitBranchSymbol := strings.TrimSpace(opts.GitBranchSymbol)

	e := &Editor{
		Buffer: Buffer{
			text:   NewTextBufferFromRunes(nil),
			cursor: Cursor{},
		},
		SearchState: SearchState{
			searchHistoryPath: opts.SearchHistoryPath,
		},
		mode: ModeNormal,
		bindings: editorBindingsState{
			keymap: keymapSet{normal: normal, insert: insert},
		},
		display: editorDisplayState{
			tabWidth:       tabWidth,
			lineNumberMode: lineNumberMode,
		},
		profile: editorProfileState{
			name: normalizeBehaviorProfileName(opts.Profile),
		},
		file: editorFileState{
			autoReloadOnChanges: opts.AutoReloadOnChanges,
		},
		git: editorGitState{
			branchSymbol: gitBranchSymbol,
		},
		highlight:       editorHighlightState{start: -1, end: -1},
		fileTreePreview: fileTreePreviewState{highlight: editorHighlightState{start: -1, end: -1}},
		commandLine: editorCommandLineState{
			historyPath:  opts.CmdHistoryPath,
			historyIndex: -1,
		},
		runtime: editorRuntimeDeps{
			persistence: NewStoreBackedPersistenceRuntime(opts.SessionStore, nil, nil),
		},
		behaviorProfiles: newBehaviorProfileRegistry(),
		sidebarModes:     newSidebarModeRegistry(),
		commands:         newCommandRegistry(),
		formatters:       newFormatterRegistry(),
		languageFeatures: newLanguageFeatureRegistry(),
		gitFeatures:      newGitFeatureRegistry(),
		sidebar: NewSidebar(
			opts.SidebarWidth,
			opts.SidebarMinWidth,
			opts.SidebarMaxWidth,
			opts.SidebarCloseOnSelect,
		),
		fileTree: editorFileTreeState{
			showHidden:  opts.FileTreeShowHidden,
			showIgnored: opts.FileTreeShowIgnored,
		},
		buffers: NewBufferManager(),
	}
	e.registerBuiltInBehaviorProfiles()
	e.registerBuiltInSidebarModes()
	e.registerBuiltInCommands()
	e.registerBuiltInFormatters()
	e.registerBuiltInLanguageFeatures()
	e.registerBuiltInGitFeatures()
	e.SetBehaviorProfile(e.profile.name)
	e.SetStyles(defaultEditorStyles())
	return e
}
func (e *Editor) Content() string {
	if e.hugeFileActive() {
		return ""
	}
	return e.docString()
}
func (e *Editor) SetKeyboardLayout(name string) {
	e.ui.layoutName = strings.TrimSpace(name)
}
func (e *Editor) SetGitBranch(name string) {
	name = strings.TrimSpace(name)
	if e.git.branch == name {
		return
	}
	e.git.branch = name
	if e.sidebar == nil {
		return
	}
	if content, ok := e.sidebar.Content.(*SidebarBranchesContent); ok {
		content.SetCurrent(name)
	}
	if content, ok := e.sidebar.Content.(*SidebarWorktreesContent); ok {
		content.SetCurrentBranch(name)
	}
}
func (e *Editor) SetGitMainBranch(name string) {
	e.git.mainBranch = strings.TrimSpace(name)
}
func (e *Editor) GetGitMainBranch() string {
	return e.git.mainBranch
}

// IsMainBranch returns true if the current branch is the main branch
func (e *Editor) IsMainBranch() bool {
	if e.git.branch == "" || e.git.mainBranch == "" {
		return false
	}
	return e.git.branch == e.git.mainBranch
}

func (e *Editor) SetStatusMessage(msg string) {
	e.setStatus(msg)
	e.Notify(msg)
}
func (e *Editor) ChangeTick() uint64 {
	return e.change.tick
}
func (e *Editor) UpdateScroll() {
	if e.interaction.freeScroll {
		return
	}
	e.ensureCursorVisible(e.viewHeightCached())
}
func (e *Editor) ConsumeLastEdit() (TextEdit, bool) {
	if !e.change.lastEdit.Valid {
		return TextEdit{}, false
	}
	edit := e.change.lastEdit
	e.change.lastEdit.Valid = false
	return edit, true
}

// PeekLastEdit returns the last edit info without consuming it.
func (e *Editor) PeekLastEdit() (TextEdit, bool) {
	if !e.change.lastEdit.Valid {
		return TextEdit{}, false
	}
	return e.change.lastEdit, true
}
func (e *Editor) LineCount() int {
	if e.gitDiffPreviewActive() {
		return len(e.git.diffPreview.lines)
	}
	if e.hugeFileActive() {
		return e.huge.buffer.LineCount()
	}
	return e.docLineCount()
}
func (e *Editor) VisibleRange() (int, int) {
	lineCount := e.LineCount()
	if lineCount == 0 {
		return 0, 0
	}
	start := e.viewport.scroll
	if start < 0 {
		start = 0
	}
	end := start + e.viewport.height - 1
	if end < start {
		end = start
	}
	if end >= lineCount {
		end = lineCount - 1
	}
	return start, end
}
func (e *Editor) SetHighlights(startLine, endLine int, spans map[int][]HighlightSpan) {
	if spans == nil || startLine < 0 || endLine < startLine {
		e.highlight = editorHighlightState{start: -1, end: -1}
		return
	}
	e.highlight.spans = spans
	e.highlight.start = startLine
	e.highlight.end = endLine
}
func (e *Editor) HasHighlights() bool {
	return e.highlight.spans != nil && e.highlight.start >= 0 && e.highlight.end >= e.highlight.start
}

// AdjustHighlights shifts the cached highlight map in-place after an edit.
// Lines in [editStart, oldEnd) are removed; lines >= oldEnd shift by (newEnd - oldEnd).
func (e *Editor) AdjustHighlights(editStart, oldEnd, newEnd int) {
	if e.highlight.spans == nil {
		return
	}
	delta := newEnd - oldEnd // negative for deletions, positive for insertions
	if delta == 0 && editStart == oldEnd {
		return
	}
	updated := make(map[int][]HighlightSpan, len(e.highlight.spans))
	for line, spans := range e.highlight.spans {
		if line < editStart {
			updated[line] = spans
		} else if line >= oldEnd {
			updated[line+delta] = spans
		}
		// lines in [editStart, oldEnd) are dropped
	}
	e.highlight.spans = updated
	e.highlight.end += delta
	if e.highlight.end < e.highlight.start {
		e.highlight = editorHighlightState{start: -1, end: -1}
	}
}
