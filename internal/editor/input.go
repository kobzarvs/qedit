package editor

func (e *Editor) HandleKey(ev EventKey) bool {
	e.interaction.freeScroll = false
	if e.mode != ModeCommand && e.mode != ModeSearch && e.ui.statusMessage != "" && !e.file.autoReloadInProgress {
		e.ui.statusMessage = ""
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

	return e.handleProfileKey(ev)
}

func (e *Editor) handleGlobalFocusHotkeys(ev EventKey) (bool, bool) {
	if ev.Modifiers()&ModAlt == 0 {
		return false, false
	}

	key := keyStringForMap(ev, e.bindings.keymap.normal)
	action, ok := e.bindings.keymap.normal[key]
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
		actionFocusCommandLine,
		actionMergeMode:
		return true, e.execAction(action)
	default:
		return false, false
	}
}
