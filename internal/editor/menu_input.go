package editor

// handleViewKey handles the second key after 'z' prefix
func (e *Editor) handleViewKey(ch rune) bool {
	e.lastCommand = "z" + string(ch)

	switch ch {
	case 'c':
		e.centerCursorLine()
	case 't':
		e.scrollCursorToTop()
	case 'b':
		e.scrollCursorToBottom()
	case 'k':
		e.scrollUp(1)
	case 'j':
		e.scrollDown(1)
	default:
		return false
	}
	return false
}

// handleWindowKey handles the second key after 'space-w' prefix
func (e *Editor) handleWindowKey(ch rune) bool {
	e.lastCommand = "SPC w" + string(ch)
	e.setStatus("window mode (not implemented)")
	return false
}

// handleKeybindingsHelp handles key input in keybindings help popup
func (e *Editor) handleKeybindingsHelp(ev EventKey) bool {
	// Get current filter based on focus
	currentFilter := func() *[]rune {
		switch e.keybindingsHelp.filterFocus {
		case 0:
			return &e.keybindingsHelp.filterKey
		case 1:
			return &e.keybindingsHelp.filterAct
		default:
			return &e.keybindingsHelp.filterDesc
		}
	}

	switch ev.Key() {
	case KeyEscape:
		e.keybindingsHelp = keybindingsHelpState{}
		return false
	case KeyEnter:
		// Clear all filters on Enter
		if len(e.keybindingsHelp.filterKey) > 0 || len(e.keybindingsHelp.filterAct) > 0 || len(e.keybindingsHelp.filterDesc) > 0 {
			e.keybindingsHelp.filterKey = nil
			e.keybindingsHelp.filterAct = nil
			e.keybindingsHelp.filterDesc = nil
			e.keybindingsHelp.scroll = 0
		} else {
			e.keybindingsHelp.active = false
		}
		return false
	case KeyTab:
		// Switch between filter fields
		e.keybindingsHelp.filterFocus = (e.keybindingsHelp.filterFocus + 1) % 3
	case KeyBacktab:
		// Switch backwards
		e.keybindingsHelp.filterFocus = (e.keybindingsHelp.filterFocus + 2) % 3
	case KeyBackspace, KeyBackspace2:
		f := currentFilter()
		if len(*f) > 0 {
			*f = (*f)[:len(*f)-1]
			e.keybindingsHelp.scroll = 0
		}
	case KeyUp, KeyCtrlP:
		if e.keybindingsHelp.scroll > 0 {
			e.keybindingsHelp.scroll--
		}
	case KeyDown, KeyCtrlN:
		e.keybindingsHelp.scroll++
	case KeyPgUp:
		e.keybindingsHelp.scroll -= 10
		if e.keybindingsHelp.scroll < 0 {
			e.keybindingsHelp.scroll = 0
		}
	case KeyPgDn:
		e.keybindingsHelp.scroll += 10
	case KeyHome:
		e.keybindingsHelp.scroll = 0
	case KeyEnd:
		e.keybindingsHelp.scroll = 999999 // will be clamped in render
	case KeyRune:
		// Type into current filter
		f := currentFilter()
		*f = append(*f, ev.Rune())
		e.keybindingsHelp.scroll = 0
	}
	return false
}

// handleSpaceMenu handles key input when space menu is active
func (e *Editor) handleSpaceMenu(ev EventKey) bool {
	if ev.Key() == KeyEscape {
		e.spaceMenuActive = false
		e.pendingKeys = ""
		return false
	}

	if ev.Key() == KeyRune {
		ch := ev.Rune()
		for _, item := range SpaceMenuItems {
			if item.Key == ch {
				e.spaceMenuActive = false
				e.pendingKeys = ""
				e.lastCommand = "SPC " + string(ch)
				return e.executeSpaceAction(item)
			}
		}
	}

	// Unknown key - close menu
	e.spaceMenuActive = false
	e.pendingKeys = ""
	return false
}

// executeSpaceAction executes the action from space menu
func (e *Editor) executeSpaceAction(item SpaceMenuItem) bool {
	if !item.Implemented {
		e.setStatus(item.Label + " (not implemented)")
		return false
	}

	switch item.Action {
	case "yank_clipboard":
		e.yankToSystemClipboard()
	case "yank_main_clipboard":
		e.yankToSystemClipboard()
	case "paste_clipboard":
		e.pasteFromSystemClipboard(false)
	case "paste_clipboard_before":
		e.pasteFromSystemClipboard(true)
	case "window_mode":
		e.windowMode = true
		e.pendingKeys = "SPC w"
		return false
	case "toggle_comment":
		e.toggleLineComment()
	case "buffer_picker":
		e.openSidebarBuffers()
	case "show_keybindings":
		e.keybindingsHelp = keybindingsHelpState{active: true}
	default:
		e.setStatus(item.Label + " (not implemented)")
	}
	return false
}
