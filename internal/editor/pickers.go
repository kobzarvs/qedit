package editor

import (
	"path/filepath"
)

func (e *Editor) ConsumeBranchPickerRequest() bool {
	if !e.requests.branchPickerRequested {
		return false
	}
	e.requests.branchPickerRequested = false
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
	if e.requests.branchSelection == "" {
		return "", false
	}
	selection := e.requests.branchSelection
	e.requests.branchSelection = ""
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
	e.requests.branchSelection = selection
}

// showRefsPicker shows the references/implementations picker
func (e *Editor) showRefsPicker(title string, items []LSPLocation) {
	if len(items) == 0 {
		return
	}
	if e.sidebar != nil && e.sidebar.Visible {
		e.closeSidebar()
	}
	e.refsPicker.active = true
	e.refsPicker.title = title
	e.refsPicker.items = items
	e.refsPicker.index = 0
	e.refsPicker.fileCache = make(map[string][][]rune)
	e.refsPicker.highlights = make(map[string]map[int][]HighlightSpan)
	e.mode = ModeNormal
}

// closeRefsPicker closes the picker and optionally jumps to selected location
func (e *Editor) closeRefsPicker(jump bool) {
	if jump && len(e.refsPicker.items) > 0 && e.refsPicker.index < len(e.refsPicker.items) {
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
	e.refsPicker = refsPickerState{}
}

// refsPickerPageSize returns the number of items per page
func (e *Editor) refsPickerPageSize() int {
	size := e.viewHeightCached() - 6
	if size < 1 {
		return 1
	}
	return size
}
