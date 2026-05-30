package editor

import "strconv"

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
	if e.modal.spaceMenuActive {
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
	if e.modal.gotoMode {
		e.modal.gotoMode = false
		e.modal.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleGotoKey(ev.Rune())
		}
		return false
	}

	// Handle match mode (m prefix)
	if e.modal.matchMode {
		e.modal.matchMode = false
		e.modal.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleMatchKey(ev.Rune())
		}
		return false
	}

	// Handle view mode (z prefix)
	if e.modal.viewMode {
		e.modal.viewMode = false
		e.modal.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleViewKey(ev.Rune())
		}
		return false
	}

	// Handle window mode (space-w prefix)
	if e.modal.windowMode {
		return e.handleWindowModeKey(ev)
	}

	// Handle pending char input (f/F/t/T/r)
	if e.modal.pendingAction != "" {
		pendingKey := e.modal.pendingKeys
		e.modal.pendingKeys = ""
		if ev.Key() == KeyEscape {
			e.modal.pendingAction = ""
			return false
		}
		if ev.Key() == KeyRune {
			e.handlePendingChar(ev.Rune())
			e.modal.lastCommand = pendingKey + string(ev.Rune())
			return false
		}
		// Ignore other keys while waiting for char
		return false
	}

	if ev.Key() == KeyEscape {
		e.profile.helix.count = ""
		e.modal.pendingKeys = ""
		if e.modal.selectMode {
			e.collapseSelection()
		}
		return false
	}
	if ev.Key() == KeyCtrlC {
		e.execAction(actionToggleComment)
		e.profile.helix.count = ""
		e.modal.pendingKeys = ""
		return false
	}
	if ev.Key() == KeyCtrlS {
		e.saveJumpPosition()
		e.profile.helix.count = ""
		e.modal.pendingKeys = ""
		return false
	}
	if ev.Key() == KeyCtrlO {
		e.jumpBackward()
		e.profile.helix.count = ""
		e.modal.pendingKeys = ""
		return false
	}
	if ev.Key() == KeyCtrlI {
		e.jumpForward()
		e.profile.helix.count = ""
		e.modal.pendingKeys = ""
		return false
	}

	if e.handleSelectionMove(ev) {
		return false
	}
	if e.handleHelixCountKey(ev) {
		return false
	}
	key := keyStringForMap(ev, e.bindings.keymap.normal)
	if key == "" {
		return false
	}
	action, ok := e.bindings.keymap.normal[key]
	if !ok {
		e.profile.helix.count = ""
		e.modal.pendingKeys = ""
		return false
	}
	count := e.consumeHelixCount()

	if e.applyHelixMotionToSelectionRanges(action, count) {
		return false
	}

	// Helix-style: w, b, e, f, F, t, T - anchor moves to old cursor, cursor moves to target
	// Selection covers what was "jumped over"
	if isHelixSelectingMotion(action) {
		if e.applyHelixSelectingMotionToCursors(action, count) {
			return false
		}
		// Anchor = where cursor WAS
		anchor := e.cursor
		var result bool
		for i := 0; i < count; i++ {
			result = e.execAction(action)
		}
		if anchor != e.cursor {
			// Selection from old position to new position
			e.selectionActive = true
			e.selectionStart = anchor
			e.selectionEnd = e.helixSelectionEndForAction(action)
			e.modal.selectMode = true
		}
		return result
	}

	// In select mode, extend selection for other motion commands
	if e.modal.selectMode && isMotionAction(action) {
		before := e.cursor
		var result bool
		for i := 0; i < count; i++ {
			result = e.execAction(action)
		}
		if before != e.cursor {
			e.selectionEnd = e.helixSelectionEndForAction(action)
		}
		return result
	}

	if e.applyHelixMotionToCursors(action, count) {
		return false
	}

	if isHelixCountableAction(action) {
		before := e.cursor
		var result bool
		for i := 0; i < count; i++ {
			result = e.execAction(action)
		}
		if action == actionGotoLine || action == actionGotoFirstLine || action == actionGotoFileEnd {
			e.recordJump(before, e.cursor)
		}
		return result
	}

	return e.execAction(action)
}

func (e *Editor) handleHelixCountKey(ev EventKey) bool {
	if ev.Key() != KeyRune {
		return false
	}
	r := ev.Rune()
	if r >= '1' && r <= '9' {
		e.profile.helix.count += string(r)
		e.modal.pendingKeys = e.profile.helix.count
		return true
	}
	if r == '0' && e.profile.helix.count != "" {
		e.profile.helix.count += "0"
		e.modal.pendingKeys = e.profile.helix.count
		return true
	}
	return false
}

func (e *Editor) consumeHelixCount() int {
	count := 1
	if e.profile.helix.count != "" {
		if parsed, err := strconv.Atoi(e.profile.helix.count); err == nil && parsed > 0 {
			count = parsed
		}
	}
	e.profile.helix.count = ""
	e.modal.pendingKeys = ""
	return count
}

// isMotionAction returns true if the action is a motion that should extend selection
func isMotionAction(action string) bool {
	switch action {
	case actionMoveLeft, actionMoveRight, actionMoveUp, actionMoveDown,
		actionWordLeft, actionWordRight, actionLineStart, actionLineEnd,
		actionFileStart, actionFileEnd, actionPageUp, actionPageDown,
		actionWordForward, actionWordBackward, actionWordEnd,
		actionWordForwardLong, actionWordBackwardLong, actionWordEndLong,
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
		actionWordForwardLong, actionWordBackwardLong, actionWordEndLong,
		actionFindChar, actionFindCharBackward, actionTillChar, actionTillCharBackward:
		return true
	}
	return false
}

func (e *Editor) helixSelectionEndForAction(action string) Cursor {
	if action == actionWordEnd || action == actionWordEndLong {
		return e.advanceCursorOne(e.cursor)
	}
	return e.cursor
}

func isHelixCountableAction(action string) bool {
	return isMotionAction(action) || action == actionExtendLine ||
		action == actionDuplicateCursorNext || action == actionDuplicateCursorPrev
}
