package editor

import (
	"path/filepath"
)

func (e *Editor) ConsumeBranchPickerRequest() bool {
	if !e.branchPickerRequested {
		return false
	}
	e.branchPickerRequested = false
	return true
}
func (e *Editor) ShowBranchPicker(branches []string, current string) {
	if len(branches) == 0 {
		e.setStatus("no branches")
		return
	}
	items := make([]string, len(branches))
	copy(items, branches)
	e.branchPickerItems = items
	e.branchPickerIndex = 0
	if current != "" {
		for i, name := range items {
			if name == current {
				e.branchPickerIndex = i
				break
			}
		}
	}
	e.branchPickerActive = true
	e.mode = ModeBranchPicker
}
func (e *Editor) ConsumeBranchSelection() (string, bool) {
	if e.branchPickerSelection == "" {
		return "", false
	}
	selection := e.branchPickerSelection
	e.branchPickerSelection = ""
	return selection, true
}
func (e *Editor) branchPickerPageSize() int {
	size := e.viewHeightCached() - 4
	if size < 1 {
		return 1
	}
	return size
}
func (e *Editor) closeBranchPicker(selection string) {
	e.branchPickerActive = false
	e.branchPickerItems = nil
	e.branchPickerIndex = 0
	e.mode = ModeNormal
	e.branchPickerSelection = selection
}

// showRefsPicker shows the references/implementations picker
func (e *Editor) showRefsPicker(title string, items []LSPLocation) {
	if len(items) == 0 {
		return
	}
	if e.sidebar != nil && e.sidebar.Visible {
		e.closeSidebar()
	}
	e.refsPickerActive = true
	e.refsPickerTitle = title
	e.refsPickerItems = items
	e.refsPickerIndex = 0
	e.refsPickerFileCache = make(map[string][][]rune)
	e.refsPickerHighlights = make(map[string]map[int][]HighlightSpan)
	e.mode = ModeNormal
}

// closeRefsPicker closes the picker and optionally jumps to selected location
func (e *Editor) closeRefsPicker(jump bool) {
	if jump && len(e.refsPickerItems) > 0 && e.refsPickerIndex < len(e.refsPickerItems) {
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
	e.refsPickerActive = false
	e.refsPickerItems = nil
	e.refsPickerIndex = 0
	e.refsPickerFileCache = nil
	e.refsPickerHighlights = nil
}

// refsPickerPageSize returns the number of items per page
func (e *Editor) refsPickerPageSize() int {
	size := e.viewHeightCached() - 6
	if size < 1 {
		return 1
	}
	return size
}
