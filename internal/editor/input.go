package editor

func (e *Editor) HandleKey(ev EventKey) bool {
	e.freeScroll = false
	if e.mode != ModeCommand && e.mode != ModeSearch && e.statusMessage != "" && !e.file.autoReloadInProgress {
		e.statusMessage = ""
	}
	// Track last key combination for display
	e.ui.lastKeyCombo = keyStringDisplay(ev)

	if handled, quit := e.handleGlobalFocusHotkeys(ev); handled {
		return quit
	}

	// Handle sidebar if focused
	if e.sidebar != nil && e.sidebar.Visible && e.sidebar.Focused {
		return e.handleSidebarKey(ev)
	}

	switch e.mode {
	case ModeInsert:
		return e.handleInsert(ev)
	case ModeCommand:
		return e.handleCommand(ev)
	case ModeBranchPicker:
		return e.handleBranchPicker(ev)
	case ModeSearch:
		return e.handleSearch(ev)
	case ModeMerge:
		return e.handleMerge(ev)
	default:
		return e.handleNormal(ev)
	}
}

func (e *Editor) handleGlobalFocusHotkeys(ev EventKey) (bool, bool) {
	if ev.Modifiers()&ModAlt == 0 {
		return false, false
	}

	key := keyStringForMap(ev, e.keymap.normal)
	action, ok := e.keymap.normal[key]
	if !ok {
		return false, false
	}

	switch action {
	case actionToggleSidebar,
		actionToggleSidebarFocus,
		actionFocusEditor,
		actionFocusPrevPane,
		actionFocusNextPane,
		actionFocusSidebar,
		actionFocusCommandLine:
		return true, e.execAction(action)
	default:
		return false, false
	}
}

// ConsumeSidebarOpenFile consumes the file path selected from sidebar.
func (e *Editor) ConsumeSidebarOpenFile() (string, bool) {
	if e.requests.sidebarOpenFilePath == "" {
		return "", false
	}
	path := e.requests.sidebarOpenFilePath
	e.requests.sidebarOpenFilePath = ""
	return path, true
}
