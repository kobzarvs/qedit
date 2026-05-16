package editor

func (e *Editor) handleInsert(ev EventKey) bool {
	if e.handleSelectionMove(ev) {
		return false
	}
	key := keyStringForMap(ev, e.bindings.keymap.insert)
	if key != "" {
		if action, ok := e.bindings.keymap.insert[key]; ok {
			if e.BehaviorProfile() == BehaviorProfileHelix {
				if e.applyHelixMotionToCursors(action, 1) {
					return false
				}
				if e.applyHelixInsertActionToCursors(action) {
					return false
				}
			}
			return e.execAction(action)
		}
	}
	if ev.Key() == KeyRune {
		if e.BehaviorProfile() == BehaviorProfileHelix && e.insertRuneAtHelixCursors(ev.Rune()) {
			return false
		}
		e.clearSelection()
		e.insertRune(ev.Rune())
	}
	return false
}

func (e *Editor) handleSelectionMove(ev EventKey) bool {
	if ev.Modifiers()&ModShift == 0 {
		return false
	}
	// Don't handle if Alt is pressed - let keymap handle alt+shift combinations
	if ev.Modifiers()&ModAlt != 0 {
		return false
	}
	switch ev.Key() {
	case KeyLeft:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveWordLeft)
		} else {
			e.extendSelection(e.moveLeft)
		}
		return true
	case KeyRight:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveWordRight)
		} else {
			e.extendSelection(e.moveRight)
		}
		return true
	case KeyUp:
		e.extendSelection(e.moveUp)
		return true
	case KeyDown:
		e.extendSelection(e.moveDown)
		return true
	case KeyPgUp:
		e.extendSelection(e.pageUp)
		return true
	case KeyPgDn:
		e.extendSelection(e.pageDown)
		return true
	case KeyHome:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveFileStart)
			return true
		}
		e.extendSelection(e.moveLineStart)
		return true
	case KeyEnd:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveFileEnd)
			return true
		}
		e.extendSelection(e.moveLineEnd)
		return true
	}
	return false
}
