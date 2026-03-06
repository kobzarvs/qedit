package editor

func (e *Editor) handleNormal(ev EventKey) bool {
	// Handle zoom mode - only allow = (more zoom) or space (restore)
	if e.zoom.pendingRestore {
		if ev.Key() == KeyRune {
			switch ev.Rune() {
			case ' ':
				e.zoomWithAnimation(false, 20) // zoom out with scroll restore
				e.zoom.pendingRestore = false
				return false
			case '=':
				e.zoomWithAnimation(true, 20) // zoom in more, keep centering
				return false
			}
		}
		// Block all other keys during zoom mode
		return false
	}

	// Handle space menu
	if e.spaceMenuActive {
		return e.handleSpaceMenu(ev)
	}

	// Handle refs picker - only intercept navigation keys, let others fall through
	if e.refsPicker.active {
		if handled := e.handleRefsPicker(ev); handled {
			return false
		}
		// Key not handled by refs picker, continue to normal handling
	}

	// Handle keybindings help popup
	if e.keybindingsHelp.active {
		return e.handleKeybindingsHelp(ev)
	}

	// Handle goto mode (g prefix)
	if e.gotoMode {
		e.gotoMode = false
		e.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleGotoKey(ev.Rune())
		}
		return false
	}

	// Handle match mode (m prefix)
	if e.matchMode {
		e.matchMode = false
		e.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleMatchKey(ev.Rune())
		}
		return false
	}

	// Handle view mode (z prefix)
	if e.viewMode {
		e.viewMode = false
		e.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleViewKey(ev.Rune())
		}
		return false
	}

	// Handle window mode (space-w prefix)
	if e.windowMode {
		e.windowMode = false
		e.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleWindowKey(ev.Rune())
		}
		return false
	}

	// Handle pending char input (f/F/t/T/r)
	if e.pendingAction != "" {
		pendingKey := e.pendingKeys
		e.pendingKeys = ""
		if ev.Key() == KeyEscape {
			e.pendingAction = ""
			return false
		}
		if ev.Key() == KeyRune {
			e.handlePendingChar(ev.Rune())
			e.lastCommand = pendingKey + string(ev.Rune())
			return false
		}
		// Ignore other keys while waiting for char
		return false
	}

	if e.handleSelectionMove(ev) {
		return false
	}
	key := keyStringForMap(ev, e.keymap.normal)
	if key == "" {
		return false
	}
	action, ok := e.keymap.normal[key]
	if !ok {
		return false
	}

	// Helix-style: w, b, e, f, F, t, T - anchor moves to old cursor, cursor moves to target
	// Selection covers what was "jumped over"
	if isHelixSelectingMotion(action) {
		// Anchor = where cursor WAS
		anchor := e.cursor
		result := e.execAction(action)
		if anchor != e.cursor {
			// Selection from old position to new position
			e.selectionActive = true
			e.selectionStart = anchor
			e.selectionEnd = e.cursor
			e.selectMode = true
		}
		return result
	}

	// In select mode, extend selection for other motion commands
	if e.selectMode && isMotionAction(action) {
		before := e.cursor
		result := e.execAction(action)
		if before != e.cursor {
			e.selectionEnd = e.cursor
		}
		return result
	}

	return e.execAction(action)
}

// isMotionAction returns true if the action is a motion that should extend selection
func isMotionAction(action string) bool {
	switch action {
	case actionMoveLeft, actionMoveRight, actionMoveUp, actionMoveDown,
		actionWordLeft, actionWordRight, actionLineStart, actionLineEnd,
		actionFileStart, actionFileEnd, actionPageUp, actionPageDown,
		actionWordForward, actionWordBackward, actionWordEnd,
		actionGotoLine, actionGotoFirstLine, actionGotoFileEnd,
		actionFindChar, actionFindCharBackward, actionTillChar, actionTillCharBackward:
		return true
	}
	return false
}

// isHelixSelectingMotion returns true if motion should auto-start selection (Helix style)
// These motions extend selection from current position to target
func isHelixSelectingMotion(action string) bool {
	switch action {
	case actionWordForward, actionWordBackward, actionWordEnd,
		actionFindChar, actionFindCharBackward, actionTillChar, actionTillCharBackward:
		return true
	}
	return false
}
