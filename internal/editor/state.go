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
		mode:           ModeNormal,
		keymap:         keymapSet{normal: normal, insert: insert},
		tabWidth:       tabWidth,
		lineNumberMode: lineNumberMode,
		git: editorGitState{
			branchSymbol: gitBranchSymbol,
		},
		autoReloadOnChanges: opts.AutoReloadOnChanges,
		highlightStart:      -1,
		highlightEnd:        -1,
		cmdHistoryPath:      opts.CmdHistoryPath,
		searchHistoryPath:   opts.SearchHistoryPath,
		runtime: editorRuntimeDeps{
			sessionStore: opts.SessionStore,
		},
		sidebar: NewSidebar(
			opts.SidebarWidth,
			opts.SidebarMinWidth,
			opts.SidebarMaxWidth,
			opts.SidebarCloseOnSelect,
		),
		fileTreeShowHidden:  opts.FileTreeShowHidden,
		fileTreeShowIgnored: opts.FileTreeShowIgnored,
		buffers:             NewBufferManager(),
	}
	e.SetStyles(defaultEditorStyles())
	return e
}
func (e *Editor) Content() string {
	if e.text == nil {
		return ""
	}
	return e.text.String()
}
func (e *Editor) SetKeyboardLayout(name string) {
	e.layoutName = strings.TrimSpace(name)
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

func (e *Editor) SetNodeStackFunc(fn NodeStackFunc) {
	e.runtime.nodeStackFunc = fn
}
func (e *Editor) SetLSPGotoFunc(fn LSPGotoFunc) {
	e.runtime.lspGotoFunc = fn
}
func (e *Editor) SetHighlightRangeFunc(fn HighlightRangeFunc) {
	e.runtime.highlightRangeFunc = fn
}
func (e *Editor) SetStatusMessage(msg string) {
	e.setStatus(msg)
	e.Notify(msg)
}
func (e *Editor) ChangeTick() uint64 {
	return e.changeTick
}
func (e *Editor) UpdateScroll() {
	if e.freeScroll {
		return
	}
	e.ensureCursorVisible(e.viewHeightCached())
}
func (e *Editor) ConsumeLastEdit() (TextEdit, bool) {
	if !e.lastEdit.Valid {
		return TextEdit{}, false
	}
	edit := e.lastEdit
	e.lastEdit.Valid = false
	return edit, true
}

// PeekLastEdit returns the last edit info without consuming it.
func (e *Editor) PeekLastEdit() (TextEdit, bool) {
	if !e.lastEdit.Valid {
		return TextEdit{}, false
	}
	return e.lastEdit, true
}
func (e *Editor) LineCount() int {
	if e.text == nil {
		return 1
	}
	return e.text.LineCount()
}
func (e *Editor) VisibleRange() (int, int) {
	lineCount := e.LineCount()
	if lineCount == 0 {
		return 0, 0
	}
	start := e.scroll
	if start < 0 {
		start = 0
	}
	end := start + e.viewHeight - 1
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
		e.highlights = nil
		e.highlightStart = -1
		e.highlightEnd = -1
		return
	}
	e.highlights = spans
	e.highlightStart = startLine
	e.highlightEnd = endLine
}
func (e *Editor) HasHighlights() bool {
	return e.highlights != nil && e.highlightStart >= 0 && e.highlightEnd >= e.highlightStart
}

// AdjustHighlights shifts the cached highlight map in-place after an edit.
// Lines in [editStart, oldEnd) are removed; lines >= oldEnd shift by (newEnd - oldEnd).
func (e *Editor) AdjustHighlights(editStart, oldEnd, newEnd int) {
	if e.highlights == nil {
		return
	}
	delta := newEnd - oldEnd // negative for deletions, positive for insertions
	if delta == 0 && editStart == oldEnd {
		return
	}
	updated := make(map[int][]HighlightSpan, len(e.highlights))
	for line, spans := range e.highlights {
		if line < editStart {
			updated[line] = spans
		} else if line >= oldEnd {
			updated[line+delta] = spans
		}
		// lines in [editStart, oldEnd) are dropped
	}
	e.highlights = updated
	e.highlightEnd += delta
	if e.highlightEnd < e.highlightStart {
		e.highlights = nil
		e.highlightStart = -1
		e.highlightEnd = -1
	}
}
