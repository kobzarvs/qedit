package editor

func (e *Editor) vimSetMark(name rune) {
	if name == 0 {
		return
	}
	if e.profile.vim.marks == nil {
		e.profile.vim.marks = make(map[rune]Cursor)
	}
	e.profile.vim.marks[name] = e.cursor
}

func (e *Editor) vimGotoMark(name rune, exact bool) bool {
	pos, ok := e.profile.vim.marks[name]
	if !ok {
		e.setStatus("mark not set")
		e.vimCancelRepeatRecordingForOperator(e.profile.vim.operator)
		return false
	}
	if pos.Row < 0 || pos.Row >= e.LineCount() {
		e.setStatus("mark out of range")
		e.vimCancelRepeatRecordingForOperator(e.profile.vim.operator)
		return false
	}
	if pos.Col > e.lineLen(pos.Row) {
		pos.Col = e.lineLen(pos.Row)
	}
	operator := e.profile.vim.operator
	if operator != "" {
		start := e.profile.vim.operatorStart
		if exact {
			e.vimApplyOperatorRange(operator, start, pos)
		} else {
			e.vimApplyLinewiseOperator(operator, start.Row, pos.Row)
		}
		e.vimFinishRepeatRecordingForOperator(operator)
		return false
	}
	before := e.cursor
	e.cursor = pos
	if !exact {
		e.moveFirstNonBlank()
	}
	e.recordJump(before, e.cursor)
	return false
}
