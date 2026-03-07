package editor

func (e *Editor) docString() string {
	if e.text == nil {
		return ""
	}
	return e.text.String()
}

func (e *Editor) docLineCount() int {
	if e.text == nil {
		return 1
	}
	return e.text.LineCount()
}

func (e *Editor) docLine(row int) []rune {
	if e.text == nil {
		return nil
	}
	return e.text.Line(row)
}

func (e *Editor) docLineLen(row int) int {
	if e.text == nil {
		return 0
	}
	return e.text.LineLen(row)
}

func (e *Editor) docRuneCount() int {
	if e.text == nil {
		return 0
	}
	return e.text.RuneCount()
}

func (e *Editor) docLineStartIndex(row int) int {
	if e.text == nil {
		return 0
	}
	return e.text.LineStartIndex(row)
}

func (e *Editor) docLineEndIndex(row int) int {
	if e.text == nil {
		return 0
	}
	return e.text.LineEndIndex(row)
}

func (e *Editor) docByteOffset(index int) int {
	if e.text == nil {
		return 0
	}
	return e.text.ByteOffset(index)
}

func (e *Editor) docSlice(start, end int) []rune {
	if e.text == nil {
		return nil
	}
	return e.text.Slice(start, end)
}

func (e *Editor) docInsert(index int, runes []rune) bool {
	if e.text == nil {
		return false
	}
	e.text.Insert(index, runes)
	return true
}

func (e *Editor) docDeleteRange(start, end int) []rune {
	if e.text == nil {
		return nil
	}
	return e.text.DeleteRange(start, end)
}

func (e *Editor) docReplaceLine(row int, line []rune) bool {
	if e.hugeFileActive() {
		return e.hugeSetLine(row, line)
	}
	if e.text == nil {
		return false
	}
	return e.text.ReplaceLine(row, line)
}
