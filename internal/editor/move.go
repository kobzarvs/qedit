package editor

// goToMatchingBracket jumps to the matching bracket or quote
func (e *Editor) goToMatchingBracket() {
	lineCount := e.LineCount()
	if e.cursor.Row < 0 || e.cursor.Row >= lineCount {
		return
	}
	line := e.line(e.cursor.Row)
	if e.cursor.Col < 0 || e.cursor.Col >= len(line) {
		return
	}

	ch := line[e.cursor.Col]

	// Handle quotes/backticks (same char for open/close)
	if ch == '"' || ch == '\'' || ch == '`' {
		e.goToMatchingQuote(ch)
		return
	}

	// Handle brackets (different chars for open/close)
	var match rune
	var forward bool

	switch ch {
	case '(':
		match, forward = ')', true
	case ')':
		match, forward = '(', false
	case '[':
		match, forward = ']', true
	case ']':
		match, forward = '[', false
	case '{':
		match, forward = '}', true
	case '}':
		match, forward = '{', false
	case '<':
		match, forward = '>', true
	case '>':
		match, forward = '<', false
	default:
		e.setStatus("no bracket or quote under cursor")
		return
	}

	depth := 1
	row, col := e.cursor.Row, e.cursor.Col

	if forward {
		col++
		for row < lineCount {
			line := e.line(row)
			for col < len(line) {
				if line[col] == ch {
					depth++
				} else if line[col] == match {
					depth--
					if depth == 0 {
						e.cursor.Row = row
						e.cursor.Col = col
						return
					}
				}
				col++
			}
			row++
			col = 0
		}
	} else {
		col--
		for row >= 0 {
			line := e.line(row)
			if col < 0 {
				row--
				if row >= 0 {
					col = e.lineLen(row) - 1
				}
				continue
			}
			for col >= 0 {
				if line[col] == ch {
					depth++
				} else if line[col] == match {
					depth--
					if depth == 0 {
						e.cursor.Row = row
						e.cursor.Col = col
						return
					}
				}
				col--
			}
			row--
			if row >= 0 {
				col = e.lineLen(row) - 1
			}
		}
	}
	e.setStatus("no matching bracket found")
}

// goToMatchingQuote jumps to the matching quote character
// For quotes, we determine if it's opening or closing by counting quotes before cursor
func (e *Editor) goToMatchingQuote(quoteChar rune) {
	row, col := e.cursor.Row, e.cursor.Col

	// Count quotes of this type before cursor position to determine if opening/closing
	// Even count = opening quote (search forward)
	// Odd count = closing quote (search backward)
	count := 0
	for r := 0; r <= row; r++ {
		line := e.line(r)
		endCol := len(line)
		if r == row {
			endCol = col
		}
		for c := 0; c < endCol; c++ {
			if line[c] == quoteChar {
				// Skip escaped quotes (check for backslash before)
				if c > 0 && line[c-1] == '\\' {
					continue
				}
				count++
			}
		}
	}

	if count%2 == 0 {
		// Opening quote - search forward for closing
		e.findMatchingQuoteForward(quoteChar)
	} else {
		// Closing quote - search backward for opening
		e.findMatchingQuoteBackward(quoteChar)
	}
}

// findMatchingQuoteForward finds the closing quote
func (e *Editor) findMatchingQuoteForward(quoteChar rune) {
	row, col := e.cursor.Row, e.cursor.Col+1

	for row < e.LineCount() {
		line := e.line(row)
		for col < len(line) {
			if line[col] == quoteChar {
				// Check if escaped
				escaped := false
				if col > 0 && line[col-1] == '\\' {
					// Count consecutive backslashes
					bs := 0
					for i := col - 1; i >= 0 && line[i] == '\\'; i-- {
						bs++
					}
					escaped = bs%2 == 1
				}
				if !escaped {
					e.cursor.Row = row
					e.cursor.Col = col
					return
				}
			}
			col++
		}
		row++
		col = 0
	}
	e.setStatus("no matching quote found")
}

// findMatchingQuoteBackward finds the opening quote
func (e *Editor) findMatchingQuoteBackward(quoteChar rune) {
	row, col := e.cursor.Row, e.cursor.Col-1

	for row >= 0 {
		if col < 0 {
			row--
			if row >= 0 {
				col = e.lineLen(row) - 1
			}
			continue
		}
		line := e.line(row)
		for col >= 0 {
			if line[col] == quoteChar {
				// Check if escaped
				escaped := false
				if col > 0 && line[col-1] == '\\' {
					// Count consecutive backslashes
					bs := 0
					for i := col - 1; i >= 0 && line[i] == '\\'; i-- {
						bs++
					}
					escaped = bs%2 == 1
				}
				if !escaped {
					e.cursor.Row = row
					e.cursor.Col = col
					return
				}
			}
			col--
		}
		row--
		if row >= 0 {
			col = e.lineLen(row) - 1
		}
	}
	e.setStatus("no matching quote found")
}

// centerCursorLine scrolls to center cursor line on screen
func (e *Editor) centerCursorLine() {
	viewHeight := e.viewHeightCached()
	e.scroll = e.cursor.Row - viewHeight/2
	if e.scroll < 0 {
		e.scroll = 0
	}
}

// scrollCursorToTop scrolls to put cursor line at top
func (e *Editor) scrollCursorToTop() {
	e.scroll = e.cursor.Row
}

// scrollCursorToBottom scrolls to put cursor line at bottom
func (e *Editor) scrollCursorToBottom() {
	viewHeight := e.viewHeightCached()
	e.scroll = e.cursor.Row - viewHeight + 1
	if e.scroll < 0 {
		e.scroll = 0
	}
}
func (e *Editor) moveLeft() {
	if e.cursor.Col > 0 {
		e.cursor.Col--
		return
	}
	if e.cursor.Row == 0 {
		return
	}
	e.cursor.Row--
	e.cursor.Col = e.lineLen(e.cursor.Row)
}
func (e *Editor) moveRight() {
	lineLen := e.lineLen(e.cursor.Row)
	if e.cursor.Col < lineLen {
		e.cursor.Col++
		return
	}
	if e.cursor.Row >= e.LineCount()-1 {
		return
	}
	e.cursor.Row++
	e.cursor.Col = 0
}
func (e *Editor) moveUp() {
	if e.cursor.Row == 0 {
		return
	}
	e.cursor.Row--
	e.clampCursorCol()
	if e.mode == ModeInsert {
		e.saveLineState()
	}
}
func (e *Editor) moveDown() {
	if e.cursor.Row >= e.LineCount()-1 {
		return
	}
	e.cursor.Row++
	e.clampCursorCol()
	if e.mode == ModeInsert {
		e.saveLineState()
	}
}
func (e *Editor) moveWordLeft() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	if e.cursor.Col <= 0 {
		if e.cursor.Row == 0 {
			return
		}
		e.cursor.Row--
		e.cursor.Col = e.lineLen(e.cursor.Row)
		return
	}
	line := e.line(e.cursor.Row)
	idx := e.cursor.Col - 1
	if idx >= len(line) {
		idx = len(line) - 1
	}
	for idx > 0 && isSpaceRune(line[idx]) {
		idx--
	}
	if idx < 0 {
		e.cursor.Col = 0
		return
	}
	if isWordRune(line[idx]) {
		for idx > 0 && isWordRune(line[idx-1]) {
			idx--
		}
		e.cursor.Col = idx
		return
	}
	for idx > 0 && !isSpaceRune(line[idx-1]) && !isWordRune(line[idx-1]) {
		idx--
	}
	e.cursor.Col = idx
}
func (e *Editor) moveWordRight() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	line := e.line(e.cursor.Row)
	if e.cursor.Col >= len(line) {
		if e.cursor.Row >= e.LineCount()-1 {
			return
		}
		e.cursor.Row++
		e.cursor.Col = 0
		return
	}
	idx := e.cursor.Col
	if idx < 0 {
		idx = 0
	}
	if idx >= len(line) {
		e.cursor.Col = len(line)
		return
	}
	if isSpaceRune(line[idx]) {
		for idx < len(line) && isSpaceRune(line[idx]) {
			idx++
		}
		e.cursor.Col = idx
		return
	}
	if isWordRune(line[idx]) {
		for idx < len(line) && isWordRune(line[idx]) {
			idx++
		}
	} else {
		for idx < len(line) && !isSpaceRune(line[idx]) && !isWordRune(line[idx]) {
			idx++
		}
	}
	for idx < len(line) && isSpaceRune(line[idx]) {
		idx++
	}
	e.cursor.Col = idx
}
func (e *Editor) moveLineStart() {
	e.cursor.Col = 0
}
func (e *Editor) moveLineEnd() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		e.cursor.Col = 0
		return
	}
	e.cursor.Col = e.lineLen(e.cursor.Row)
}
func (e *Editor) moveFileStart() {
	prevRow := e.cursor.Row
	e.cursor.Row = 0
	e.cursor.Col = 0
	if e.mode == ModeInsert && e.cursor.Row != prevRow {
		e.saveLineState()
	}
}
func (e *Editor) moveFileEnd() {
	if e.LineCount() == 0 {
		e.cursor.Row = 0
		e.cursor.Col = 0
		return
	}
	prevRow := e.cursor.Row
	e.cursor.Row = e.LineCount() - 1
	e.cursor.Col = e.lineLen(e.cursor.Row)
	if e.mode == ModeInsert && e.cursor.Row != prevRow {
		e.saveLineState()
	}
}
func (e *Editor) moveLineUp() {
	if e.cursor.Row <= 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	from := e.cursor.Row
	to := e.cursor.Row - 1
	if !e.swapLines(from, to) {
		return
	}
	e.cursor.Row = to
	e.recordUndo(action{kind: actionMoveLine, rowFrom: from, rowTo: to})
	if e.mode == ModeInsert {
		e.saveLineState()
	}
}
func (e *Editor) moveLineDown() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount()-1 {
		return
	}
	from := e.cursor.Row
	to := e.cursor.Row + 1
	if !e.swapLines(from, to) {
		return
	}
	e.cursor.Row = to
	e.recordUndo(action{kind: actionMoveLine, rowFrom: from, rowTo: to})
	if e.mode == ModeInsert {
		e.saveLineState()
	}
}
func (e *Editor) pageUp() {
	height := e.viewHeightCached()
	if height < 1 {
		height = 1
	}
	prevRow := e.cursor.Row
	e.cursor.Row -= height
	if e.cursor.Row < 0 {
		e.cursor.Row = 0
	}
	e.clampCursorCol()
	if e.mode == ModeInsert && e.cursor.Row != prevRow {
		e.saveLineState()
	}
}
func (e *Editor) pageDown() {
	height := e.viewHeightCached()
	if height < 1 {
		height = 1
	}
	prevRow := e.cursor.Row
	e.cursor.Row += height
	if e.cursor.Row >= e.LineCount() {
		e.cursor.Row = e.LineCount() - 1
		if e.cursor.Row < 0 {
			e.cursor.Row = 0
		}
	}
	e.clampCursorCol()
	if e.mode == ModeInsert && e.cursor.Row != prevRow {
		e.saveLineState()
	}
}

// Helix-style word forward (w) - move to next word start
func (e *Editor) wordForward() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	line := e.line(e.cursor.Row)
	idx := e.cursor.Col

	// If at end of line, move to next line
	if idx >= len(line) {
		if e.cursor.Row >= e.LineCount()-1 {
			return
		}
		e.cursor.Row++
		e.cursor.Col = 0
		// Skip to first non-space
		line = e.line(e.cursor.Row)
		for e.cursor.Col < len(line) && isSpaceRune(line[e.cursor.Col]) {
			e.cursor.Col++
		}
		return
	}

	// Remember if we started on a word (not punctuation)
	startedOnWord := isWordRune(line[idx])
	wordEndIdx := idx

	// Skip current word or punctuation
	if isWordRune(line[idx]) {
		for idx < len(line) && isWordRune(line[idx]) {
			idx++
		}
		wordEndIdx = idx - 1 // Last char of word
	} else if !isSpaceRune(line[idx]) {
		for idx < len(line) && !isSpaceRune(line[idx]) && !isWordRune(line[idx]) {
			idx++
		}
	}

	// Check if there's whitespace before next word
	hasWhitespace := idx < len(line) && isSpaceRune(line[idx])

	// Edge case: started on word, no whitespace, next char is punctuation
	// In this case, behave like 'e' - stop at end of current word
	if startedOnWord && !hasWhitespace && idx < len(line) && !isWordRune(line[idx]) {
		e.cursor.Col = wordEndIdx
		return
	}

	// Skip whitespace to next word
	for idx < len(line) && isSpaceRune(line[idx]) {
		idx++
	}

	// If reached end of line, move to next line
	if idx >= len(line) && e.cursor.Row < e.LineCount()-1 {
		e.cursor.Row++
		e.cursor.Col = 0
		line = e.line(e.cursor.Row)
		for e.cursor.Col < len(line) && isSpaceRune(line[e.cursor.Col]) {
			e.cursor.Col++
		}
		return
	}

	e.cursor.Col = idx
}

// Helix-style word backward (b) - move to previous word start
func (e *Editor) wordBackward() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	line := e.line(e.cursor.Row)
	idx := e.cursor.Col

	// If at start of line, move to previous line
	if idx <= 0 {
		if e.cursor.Row <= 0 {
			return
		}
		e.cursor.Row--
		line = e.line(e.cursor.Row)
		e.cursor.Col = len(line)
		// Recursively find previous word start
		e.wordBackward()
		return
	}

	// Move back one char to get off current position
	idx--

	// Skip whitespace backwards
	for idx > 0 && isSpaceRune(line[idx]) {
		idx--
	}

	// If reached start of line
	if idx <= 0 {
		if isSpaceRune(line[0]) && e.cursor.Row > 0 {
			e.cursor.Row--
			line = e.line(e.cursor.Row)
			e.cursor.Col = len(line)
			e.wordBackward()
			return
		}
		e.cursor.Col = 0
		return
	}

	// Find start of current word
	if isWordRune(line[idx]) {
		for idx > 0 && isWordRune(line[idx-1]) {
			idx--
		}
	} else {
		for idx > 0 && !isSpaceRune(line[idx-1]) && !isWordRune(line[idx-1]) {
			idx--
		}
	}

	e.cursor.Col = idx
}

// Helix-style word end (e) - move to end of word
func (e *Editor) wordEnd() {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return
	}
	line := e.line(e.cursor.Row)
	idx := e.cursor.Col

	// Move forward one position to get off current word end
	idx++

	// Skip whitespace
	for idx < len(line) && isSpaceRune(line[idx]) {
		idx++
	}

	// If reached end of line, move to next line
	if idx >= len(line) {
		if e.cursor.Row >= e.LineCount()-1 {
			e.cursor.Col = len(line)
			return
		}
		e.cursor.Row++
		line = e.line(e.cursor.Row)
		idx = 0
		// Skip whitespace on new line
		for idx < len(line) && isSpaceRune(line[idx]) {
			idx++
		}
	}

	// Find end of word
	if idx < len(line) {
		if isWordRune(line[idx]) {
			for idx < len(line)-1 && isWordRune(line[idx+1]) {
				idx++
			}
		} else if !isSpaceRune(line[idx]) {
			for idx < len(line)-1 && !isSpaceRune(line[idx+1]) && !isWordRune(line[idx+1]) {
				idx++
			}
		}
	}

	e.cursor.Col = idx
}

// Helix-style goto line (G) - go to last line
func (e *Editor) gotoLastLine() {
	if e.LineCount() == 0 {
		return
	}
	e.cursor.Row = e.LineCount() - 1
	e.cursor.Col = 0
}

// Helix-style goto first line (gg)
func (e *Editor) gotoFirstLine() {
	e.cursor.Row = 0
	e.cursor.Col = 0
}

// Helix-style goto file end (ge) - go to end of file
func (e *Editor) gotoFileEnd() {
	if e.LineCount() == 0 {
		e.cursor.Row = 0
		e.cursor.Col = 0
		return
	}
	e.cursor.Row = e.LineCount() - 1
	e.cursor.Col = e.lineLen(e.cursor.Row)
}

// findCharForward finds next occurrence of char on current line
// isBracketOrQuote returns true if char is a bracket or quote that should search across lines
func isBracketOrQuote(ch rune) bool {
	switch ch {
	case '(', ')', '[', ']', '{', '}', '<', '>', '\'', '"', '`':
		return true
	}
	return false
}
func (e *Editor) findCharForward(ch rune, till bool) bool {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return false
	}

	// For brackets/quotes, search across lines
	if isBracketOrQuote(ch) {
		startRow := e.cursor.Row
		startCol := e.cursor.Col + 1

		// For till mode: if char at cursor+1 is the target, skip it
		// (we're already at the "till" position from previous search)
		if till && startCol < e.lineLen(startRow) && e.line(startRow)[startCol] == ch {
			startCol++
		}

		for row := startRow; row < e.LineCount(); row++ {
			line := e.line(row)
			fromCol := 0
			if row == startRow {
				fromCol = startCol
			}
			for col := fromCol; col < len(line); col++ {
				if line[col] == ch {
					e.cursor.Row = row
					if till {
						// For till, stop one char before
						if col > 0 {
							e.cursor.Col = col - 1
						} else if row > startRow {
							// If at start of line, go to end of previous line
							e.cursor.Row = row - 1
							e.cursor.Col = e.lineLen(row - 1)
						} else {
							e.cursor.Col = col
						}
					} else {
						e.cursor.Col = col
					}
					return true
				}
			}
		}
		return false
	}

	// For regular chars, search only on current line
	line := e.line(e.cursor.Row)
	startIdx := e.cursor.Col + 1

	// For till mode: skip immediate target
	if till && startIdx < len(line) && line[startIdx] == ch {
		startIdx++
	}

	for i := startIdx; i < len(line); i++ {
		if line[i] == ch {
			if till {
				e.cursor.Col = i - 1
			} else {
				e.cursor.Col = i
			}
			return true
		}
	}
	return false
}

// findCharBackward finds previous occurrence of char
func (e *Editor) findCharBackward(ch rune, till bool) bool {
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return false
	}

	// For brackets/quotes, search across lines backwards
	if isBracketOrQuote(ch) {
		startRow := e.cursor.Row
		startCol := e.cursor.Col - 1

		// For till mode: if char at cursor-1 is the target, skip it
		// (we're already at the "till" position from previous search)
		if till && startCol >= 0 && e.line(startRow)[startCol] == ch {
			startCol--
		}

		for row := startRow; row >= 0; row-- {
			line := e.line(row)
			toCol := len(line) - 1
			if row == startRow {
				toCol = startCol
			}
			for col := toCol; col >= 0; col-- {
				if line[col] == ch {
					e.cursor.Row = row
					if till {
						// For till, stop one char after
						if col < len(line)-1 {
							e.cursor.Col = col + 1
						} else if row < e.LineCount()-1 {
							// If at end of line, go to start of next line
							e.cursor.Row = row + 1
							e.cursor.Col = 0
						} else {
							e.cursor.Col = col
						}
					} else {
						e.cursor.Col = col
					}
					return true
				}
			}
		}
		return false
	}

	// For regular chars, search only on current line
	line := e.line(e.cursor.Row)
	startIdx := e.cursor.Col - 1

	// For till mode: skip immediate target
	if till && startIdx >= 0 && line[startIdx] == ch {
		startIdx--
	}

	for i := startIdx; i >= 0; i-- {
		if line[i] == ch {
			if till {
				e.cursor.Col = i + 1
			} else {
				e.cursor.Col = i
			}
			return true
		}
	}
	return false
}
