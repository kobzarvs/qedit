package editor

import "path/filepath"

func (e *Editor) handleBranchPicker(ev EventKey) bool {
	switch keyString(ev) {
	case "esc", "ctrl+c":
		e.closeBranchPicker("")
		return false
	case "enter":
		if len(e.branchPicker.items) == 0 {
			e.closeBranchPicker("")
			return false
		}
		selection := e.branchPicker.items[e.branchPicker.index]
		e.closeBranchPicker(selection)
		return false
	case "up", "k":
		e.branchPicker.index--
	case "down", "j":
		e.branchPicker.index++
	case "pgup":
		e.branchPicker.index -= e.branchPickerPageSize()
	case "pgdn":
		e.branchPicker.index += e.branchPickerPageSize()
	case "home":
		e.branchPicker.index = 0
	case "end":
		e.branchPicker.index = len(e.branchPicker.items) - 1
	default:
		return false
	}
	if e.branchPicker.index < 0 {
		e.branchPicker.index = 0
	}
	if e.branchPicker.index >= len(e.branchPicker.items) {
		e.branchPicker.index = len(e.branchPicker.items) - 1
		if e.branchPicker.index < 0 {
			e.branchPicker.index = 0
		}
	}
	return false
}

func (e *Editor) handleRefsPicker(ev EventKey) bool {
	switch keyString(ev) {
	case "esc", "ctrl+c", "q":
		e.closeRefsPicker(false)
		return true
	case "enter":
		e.closeRefsPicker(true)
		return true
	case "up", "k":
		e.refsPicker.index--
	case "down", "j":
		e.refsPicker.index++
	case "pgup", "ctrl+u":
		e.refsPicker.index -= e.refsPickerPageSize()
	case "pgdn", "ctrl+d":
		e.refsPicker.index += e.refsPickerPageSize()
	case "home", "g":
		e.refsPicker.index = 0
	case "end", "G":
		e.refsPicker.index = len(e.refsPicker.items) - 1
	default:
		return false // Not handled, let normal key handling proceed
	}
	// Clamp index
	if e.refsPicker.index < 0 {
		e.refsPicker.index = 0
	}
	if e.refsPicker.index >= len(e.refsPicker.items) {
		e.refsPicker.index = len(e.refsPicker.items) - 1
		if e.refsPicker.index < 0 {
			e.refsPicker.index = 0
		}
	}
	// Move cursor to selected reference (if same file)
	e.jumpToSelectedRef()
	return true // Key was handled
}

// jumpToSelectedRef moves cursor to the currently selected reference
func (e *Editor) jumpToSelectedRef() {
	if e.refsPicker.index >= len(e.refsPicker.items) {
		return
	}
	loc := e.refsPicker.items[e.refsPicker.index]
	currentAbs, _ := filepath.Abs(e.document.filename)
	if loc.Path == currentAbs || loc.Path == e.document.filename {
		e.cursor.Row = loc.StartLine
		e.cursor.Col = loc.StartCol
		e.ensureCursorVisible(e.viewHeightCached())
	} else {
		e.requestOpenLocation(loc.Path, loc.StartLine, loc.StartCol)
	}
}
