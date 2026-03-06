package editor

func (e *Editor) exitCommandLine() {
	switch e.mode {
	case ModeCommand:
		e.mode = ModeNormal
		e.commandLine.text = e.commandLine.text[:0]
		e.commandLine.cursor = 0
		e.commandLine.historyIndex = -1
	case ModeSearch:
		e.mode = ModeNormal
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.searchMatches = nil
		e.searchHistoryIndex = -1
	}
}

func (e *Editor) focusSidebar() {
	if e.sidebar == nil {
		return
	}
	if !e.sidebar.Visible {
		e.openSidebar()
		return
	}
	e.sidebar.Focused = true
	e.exitCommandLine()
}

func (e *Editor) focusEditor() {
	if e.sidebar != nil {
		e.sidebar.Focused = false
	}
	e.clearFileTreePreview()
	e.exitCommandLine()
}

func (e *Editor) focusCommandLine() {
	if e.sidebar != nil {
		e.sidebar.Focused = false
	}
	e.clearFileTreePreview()
	if e.mode == ModeCommand {
		return
	}
	if e.mode == ModeSearch {
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.searchMatches = nil
		e.searchHistoryIndex = -1
	}
	e.mode = ModeCommand
	e.commandLine.text = e.commandLine.text[:0]
	e.commandLine.cursor = 0
	e.commandLine.historyIndex = -1
}

type focusPane int

const (
	focusPaneSidebar focusPane = iota
	focusPaneEditor
)

func (e *Editor) focusablePanes() []focusPane {
	panes := make([]focusPane, 0, 2)
	if e.sidebar != nil && e.sidebar.Visible {
		panes = append(panes, focusPaneSidebar)
	}
	panes = append(panes, focusPaneEditor)
	return panes
}

func (e *Editor) currentPane() focusPane {
	if e.sidebar != nil && e.sidebar.Visible && e.sidebar.Focused {
		return focusPaneSidebar
	}
	return focusPaneEditor
}

func (e *Editor) focusPane(target focusPane) {
	switch target {
	case focusPaneSidebar:
		if e.sidebar == nil || !e.sidebar.Visible {
			return
		}
		e.focusSidebar()
	default:
		e.focusEditor()
	}
}

func (e *Editor) focusPrevPane() {
	panes := e.focusablePanes()
	if len(panes) < 2 {
		return
	}
	current := e.currentPane()
	idx := 0
	found := false
	for i, pane := range panes {
		if pane == current {
			idx = i
			found = true
			break
		}
	}
	if !found {
		e.focusPane(panes[0])
		return
	}
	idx--
	if idx < 0 {
		idx = len(panes) - 1
	}
	e.focusPane(panes[idx])
}

func (e *Editor) focusNextPane() {
	panes := e.focusablePanes()
	if len(panes) < 2 {
		return
	}
	current := e.currentPane()
	idx := 0
	found := false
	for i, pane := range panes {
		if pane == current {
			idx = i
			found = true
			break
		}
	}
	if !found {
		e.focusPane(panes[0])
		return
	}
	idx++
	if idx >= len(panes) {
		idx = 0
	}
	e.focusPane(panes[idx])
}
