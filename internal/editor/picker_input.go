package editor

import "path/filepath"

func (e *Editor) handleBranchPicker(ev EventKey) bool {
	switch keyString(ev) {
	case "esc", "ctrl+c":
		e.closeBranchPicker("")
		return false
	case "enter":
		if len(e.branchPickerItems) == 0 {
			e.closeBranchPicker("")
			return false
		}
		selection := e.branchPickerItems[e.branchPickerIndex]
		e.closeBranchPicker(selection)
		return false
	case "up", "k":
		e.branchPickerIndex--
	case "down", "j":
		e.branchPickerIndex++
	case "pgup":
		e.branchPickerIndex -= e.branchPickerPageSize()
	case "pgdn":
		e.branchPickerIndex += e.branchPickerPageSize()
	case "home":
		e.branchPickerIndex = 0
	case "end":
		e.branchPickerIndex = len(e.branchPickerItems) - 1
	default:
		return false
	}
	if e.branchPickerIndex < 0 {
		e.branchPickerIndex = 0
	}
	if e.branchPickerIndex >= len(e.branchPickerItems) {
		e.branchPickerIndex = len(e.branchPickerItems) - 1
		if e.branchPickerIndex < 0 {
			e.branchPickerIndex = 0
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
	currentAbs, _ := filepath.Abs(e.filename)
	if loc.Path == currentAbs || loc.Path == e.filename {
		e.cursor.Row = loc.StartLine
		e.cursor.Col = loc.StartCol
		e.ensureCursorVisible(e.viewHeightCached())
	} else {
		e.requestOpenLocation(loc.Path, loc.StartLine, loc.StartCol)
	}
}
