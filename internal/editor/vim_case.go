package editor

func (e *Editor) vimApplyCaseSelectionOperator(operator string) {
	kind, ok := vimCaseTransformForOperator(operator)
	if !ok {
		return
	}
	e.saveLineState()
	e.transformSelectionCase(kind)
}

func (e *Editor) vimApplyCaseLinewiseOperator(operator string, startRow, endRow int) {
	_, ok := vimCaseTransformForOperator(operator)
	if !ok || e.LineCount() == 0 {
		return
	}
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	if startRow < 0 {
		startRow = 0
	}
	if endRow >= e.LineCount() {
		endRow = e.LineCount() - 1
	}
	if startRow > endRow {
		return
	}
	start := Cursor{Row: startRow, Col: 0}
	end := Cursor{Row: endRow, Col: e.lineLen(endRow)}
	e.selectionStart = start
	e.selectionEnd = end
	e.selectionActive = true
	e.vimApplyCaseSelectionOperator(operator)
	e.clearSelection()
	e.cursor = start
}

func vimCaseTransformForOperator(operator string) (caseTransformKind, bool) {
	switch operator {
	case "gu":
		return caseTransformLower, true
	case "gU":
		return caseTransformUpper, true
	case "g~":
		return caseTransformToggle, true
	default:
		return caseTransformToggle, false
	}
}
