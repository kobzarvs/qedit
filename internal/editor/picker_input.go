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
		e.refsPickerIndex--
	case "down", "j":
		e.refsPickerIndex++
	case "pgup", "ctrl+u":
		e.refsPickerIndex -= e.refsPickerPageSize()
	case "pgdn", "ctrl+d":
		e.refsPickerIndex += e.refsPickerPageSize()
	case "home", "g":
		e.refsPickerIndex = 0
	case "end", "G":
		e.refsPickerIndex = len(e.refsPickerItems) - 1
	default:
		return false // Not handled, let normal key handling proceed
	}
	// Clamp index
	if e.refsPickerIndex < 0 {
		e.refsPickerIndex = 0
	}
	if e.refsPickerIndex >= len(e.refsPickerItems) {
		e.refsPickerIndex = len(e.refsPickerItems) - 1
		if e.refsPickerIndex < 0 {
			e.refsPickerIndex = 0
		}
	}
	// Move cursor to selected reference (if same file)
	e.jumpToSelectedRef()
	return true // Key was handled
}

// jumpToSelectedRef moves cursor to the currently selected reference
func (e *Editor) jumpToSelectedRef() {
	if e.refsPickerIndex >= len(e.refsPickerItems) {
		return
	}
	loc := e.refsPickerItems[e.refsPickerIndex]
	currentAbs, _ := filepath.Abs(e.filename)
	if loc.Path == currentAbs || loc.Path == e.filename {
		e.cursor.Row = loc.StartLine
		e.cursor.Col = loc.StartCol
		e.ensureCursorVisible(e.viewHeightCached())
	} else {
		e.requestOpenLocation(loc.Path, loc.StartLine, loc.StartCol)
	}
}
