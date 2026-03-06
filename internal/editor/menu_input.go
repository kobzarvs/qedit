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
		switch e.keybindingsHelpFilterFocus {
		case 0:
			return &e.keybindingsHelpFilterKey
		case 1:
			return &e.keybindingsHelpFilterAct
		default:
			return &e.keybindingsHelpFilterDesc
		}
	}

	switch ev.Key() {
	case KeyEscape:
		e.keybindingsHelpActive = false
		e.keybindingsHelpFilterKey = nil
		e.keybindingsHelpFilterAct = nil
		e.keybindingsHelpFilterDesc = nil
		e.keybindingsHelpScroll = 0
		e.keybindingsHelpFilterFocus = 0
		return false
	case KeyEnter:
		// Clear all filters on Enter
		if len(e.keybindingsHelpFilterKey) > 0 || len(e.keybindingsHelpFilterAct) > 0 || len(e.keybindingsHelpFilterDesc) > 0 {
			e.keybindingsHelpFilterKey = nil
			e.keybindingsHelpFilterAct = nil
			e.keybindingsHelpFilterDesc = nil
			e.keybindingsHelpScroll = 0
		} else {
			e.keybindingsHelpActive = false
		}
		return false
	case KeyTab:
		// Switch between filter fields
		e.keybindingsHelpFilterFocus = (e.keybindingsHelpFilterFocus + 1) % 3
	case KeyBacktab:
		// Switch backwards
		e.keybindingsHelpFilterFocus = (e.keybindingsHelpFilterFocus + 2) % 3
	case KeyBackspace, KeyBackspace2:
		f := currentFilter()
		if len(*f) > 0 {
			*f = (*f)[:len(*f)-1]
			e.keybindingsHelpScroll = 0
		}
	case KeyUp, KeyCtrlP:
		if e.keybindingsHelpScroll > 0 {
			e.keybindingsHelpScroll--
		}
	case KeyDown, KeyCtrlN:
		e.keybindingsHelpScroll++
	case KeyPgUp:
		e.keybindingsHelpScroll -= 10
		if e.keybindingsHelpScroll < 0 {
			e.keybindingsHelpScroll = 0
		}
	case KeyPgDn:
		e.keybindingsHelpScroll += 10
	case KeyHome:
		e.keybindingsHelpScroll = 0
	case KeyEnd:
		e.keybindingsHelpScroll = 999999 // will be clamped in render
	case KeyRune:
		// Type into current filter
		f := currentFilter()
		*f = append(*f, ev.Rune())
		e.keybindingsHelpScroll = 0
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
		e.keybindingsHelpActive = true
		e.keybindingsHelpScroll = 0
		e.keybindingsHelpFilterKey = nil
		e.keybindingsHelpFilterAct = nil
		e.keybindingsHelpFilterDesc = nil
		e.keybindingsHelpFilterFocus = 0
	default:
		e.setStatus(item.Label + " (not implemented)")
	}
	return false
}
