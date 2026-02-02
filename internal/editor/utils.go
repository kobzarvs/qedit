package editor

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (e *Editor) setStatus(msg string) {
	e.statusMessage = msg
}
func (e *Editor) line(row int) []rune {
	if e.text == nil {
		return nil
	}
	return e.text.Line(row)
}
func (e *Editor) lineLen(row int) int {
	if e.text == nil {
		return 0
	}
	return e.text.LineLen(row)
}
func (e *Editor) clampCursorCol() {
	lineCount := e.LineCount()
	if lineCount == 0 {
		e.cursor.Col = 0
		return
	}
	if e.cursor.Row < 0 || e.cursor.Row >= lineCount {
		return
	}
	lineLen := e.text.LineLen(e.cursor.Row)
	if e.cursor.Col > lineLen {
		e.cursor.Col = lineLen
	}
	if e.cursor.Col < 0 {
		e.cursor.Col = 0
	}
}
func (e *Editor) ensureCursorVisible(viewHeight int) {
	if viewHeight <= 0 {
		return
	}
	const margin = 5 // scroll when cursor is within 5 lines of edge

	// If cursor is far outside visible area, center it
	if e.cursor.Row < e.scroll-1 || e.cursor.Row >= e.scroll+viewHeight+1 {
		e.scroll = e.cursor.Row - viewHeight/2
		if e.scroll < 0 {
			e.scroll = 0
		}
		return
	}
	// Scroll when cursor approaches top edge (within margin)
	if e.cursor.Row < e.scroll+margin {
		e.scroll = e.cursor.Row - margin
		if e.scroll < 0 {
			e.scroll = 0
		}
		return
	}
	// Scroll when cursor approaches bottom edge (within margin)
	if e.cursor.Row >= e.scroll+viewHeight-margin {
		e.scroll = e.cursor.Row - viewHeight + margin + 1
	}
}
func (e *Editor) ensureCursorVisibleHorizontal(viewWidth, gutterWidth int) {
	if viewWidth <= gutterWidth {
		return
	}
	textWidth := viewWidth - gutterWidth
	margin := 10
	if margin > textWidth/3 {
		margin = textWidth / 3
	}
	if margin < 1 {
		margin = 1
	}

	var visualCursorCol int
	if e.cursor.Row >= 0 && e.cursor.Row < e.LineCount() {
		visualCursorCol = visualCol(e.text.Line(e.cursor.Row), e.cursor.Col, e.tabWidth)
	}

	// Cursor position relative to scrollX
	relativeX := visualCursorCol - e.scrollX

	// If cursor is too close to right edge, scroll right
	if relativeX > textWidth-margin {
		e.scrollX = visualCursorCol - (textWidth - margin)
	}
	// If cursor is too close to left edge (or off screen left), scroll left
	if relativeX < margin {
		e.scrollX = visualCursorCol - margin
	}
	e.clampScrollX(textWidth)
}
func (e *Editor) viewHeightCached() int {
	if e.viewHeight < 1 {
		return 1
	}
	return e.viewHeight
}
func splitLines(data []byte) [][]rune {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	parts := strings.Split(text, "\n")
	lines := make([][]rune, len(parts))
	for i, p := range parts {
		lines[i] = []rune(p)
	}
	return lines
}
func joinLines(lines [][]rune) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(string(line))
	}
	return b.String()
}
func joinText(lines [][]rune) []rune {
	if len(lines) == 0 {
		return nil
	}
	length := 0
	for _, line := range lines {
		length += len(line)
	}
	length += len(lines) - 1
	out := make([]rune, 0, length)
	for i, line := range lines {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, line...)
	}
	return out
}
func splitRunesByNewline(runes []rune) [][]rune {
	if len(runes) == 0 {
		return nil
	}
	lines := make([][]rune, 0, 1)
	start := 0
	for i, r := range runes {
		if r != '\n' {
			continue
		}
		lines = append(lines, append([]rune(nil), runes[start:i]...))
		start = i + 1
	}
	lines = append(lines, append([]rune(nil), runes[start:]...))
	return lines
}
func (e *Editor) styleForHighlight(kind string) (Style, bool) {
	switch kind {
	case "keyword":
		return e.styleSyntaxKeyword, true
	case "string":
		return e.styleSyntaxString, true
	case "comment":
		return e.styleSyntaxComment, true
	case "type":
		return e.styleSyntaxType, true
	case "function":
		return e.styleSyntaxFunction, true
	case "number":
		return e.styleSyntaxNumber, true
	case "constant":
		return e.styleSyntaxConstant, true
	case "operator":
		return e.styleSyntaxOperator, true
	case "punctuation":
		return e.styleSyntaxPunctuation, true
	case "field":
		return e.styleSyntaxField, true
	case "builtin":
		return e.styleSyntaxBuiltin, true
	case "variable":
		return e.styleSyntaxVariable, true
	case "parameter":
		return e.styleSyntaxParameter, true
	case "text":
		return e.styleTableBorder, true
	default:
		return e.styleMain, false
	}
}
func highlightPriority(kind string) int {
	switch kind {
	case "comment":
		return 7
	case "string":
		return 6
	case "keyword":
		return 5
	case "constant":
		return 4
	case "builtin":
		return 4
	case "parameter":
		return 3
	case "type", "function", "number":
		return 3
	case "field":
		return 2
	case "variable":
		return 2
	case "operator":
		return 1
	case "punctuation":
		return 1
	case "text":
		return 8
	default:
		return 0
	}
}
func highlightKindAt(spans []HighlightSpan, col int) (string, bool) {
	bestKind := ""
	bestPriority := 0
	for _, span := range spans {
		if col < span.StartCol || col >= span.EndCol {
			continue
		}
		priority := highlightPriority(span.Kind)
		if priority > bestPriority {
			bestPriority = priority
			bestKind = span.Kind
		}
	}
	if bestKind == "" {
		return "", false
	}
	return bestKind, true
}
func clampRange(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
func (e *Editor) swapLines(a, b int) bool {
	lineCount := e.LineCount()
	if a < 0 || b < 0 || a >= lineCount || b >= lineCount {
		return false
	}
	if a == b {
		return true
	}
	lineA := e.text.Line(a)
	lineB := e.text.Line(b)
	_ = e.text.ReplaceLine(a, lineB)
	_ = e.text.ReplaceLine(b, lineA)
	e.lastEdit.Valid = false
	return true
}
func runeByteLen(r rune) int {
	n := utf8.RuneLen(r)
	if n < 1 {
		return 1
	}
	return n
}
func runeSliceByteLen(rs []rune) int {
	n := 0
	for _, r := range rs {
		n += runeByteLen(r)
	}
	return n
}
func (e *Editor) byteOffset(pos Cursor) (int, int) {
	lineCount := e.LineCount()
	row := pos.Row
	if row < 0 {
		row = 0
	}
	if row > lineCount {
		row = lineCount
	}
	if row == lineCount {
		endIndex := e.text.RuneCount()
		return e.text.ByteOffset(endIndex), 0
	}
	lineStart := e.text.LineStartIndex(row)
	lineLen := e.text.LineLen(row)
	col := pos.Col
	if col < 0 {
		col = 0
	}
	if col > lineLen {
		col = lineLen
	}
	index := lineStart + col
	byteOffset := e.text.ByteOffset(index)
	lineStartBytes := e.text.ByteOffset(lineStart)
	return byteOffset, byteOffset - lineStartBytes
}
func (e *Editor) recordTextEdit(start, oldEnd, newEnd Cursor, insertedBytes int) {
	startByte, startColBytes := e.byteOffset(start)
	oldEndByte, oldEndColBytes := e.byteOffset(oldEnd)
	newEndByte := startByte + insertedBytes
	newEndColBytes := 0
	if newEnd.Row == start.Row {
		newEndColBytes = startColBytes + insertedBytes
	}
	e.lastEdit = TextEdit{
		Valid:          true,
		StartByte:      startByte,
		OldEndByte:     oldEndByte,
		NewEndByte:     newEndByte,
		StartRow:       start.Row,
		StartColBytes:  startColBytes,
		OldEndRow:      oldEnd.Row,
		OldEndColBytes: oldEndColBytes,
		NewEndRow:      newEnd.Row,
		NewEndColBytes: newEndColBytes,
	}
	e.adjustConflictBlocksForEdit(start.Row, oldEnd.Row, newEnd.Row)
}
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
func isSpaceRune(r rune) bool {
	return unicode.IsSpace(r)
}
func visualCol(line []rune, logicalCol int, tabWidth int) int {
	if tabWidth < 1 {
		tabWidth = 1
	}
	if logicalCol < 0 {
		logicalCol = 0
	}
	if logicalCol > len(line) {
		logicalCol = len(line)
	}
	col := 0
	for i := 0; i < logicalCol; i++ {
		if line[i] == '\t' {
			col += tabWidth - (col % tabWidth)
			continue
		}
		col++
	}
	return col
}
func visualToLogicalCol(line []rune, visualX int, tabWidth int) int {
	if tabWidth < 1 {
		tabWidth = 1
	}
	if visualX <= 0 {
		return 0
	}
	col := 0
	for i, r := range line {
		var advance int
		if r == '\t' {
			advance = tabWidth - (col % tabWidth)
		} else {
			advance = 1
		}
		if col+advance > visualX {
			// Click is within this character - return current position
			return i
		}
		col += advance
		if col >= visualX {
			return i + 1
		}
	}
	// Click is past end of line
	return len(line)
}
func parseLineNumberMode(value string) LineNumberMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "relative", "rel":
		return LineNumberRelative
	case "off", "none", "false":
		return LineNumberOff
	default:
		return LineNumberAbsolute
	}
}
func (e *Editor) toggleLineNumbers() {
	switch e.lineNumberMode {
	case LineNumberAbsolute:
		e.lineNumberMode = LineNumberRelative
		e.setStatus("line numbers relative")
	case LineNumberRelative:
		e.lineNumberMode = LineNumberAbsolute
		e.setStatus("line numbers absolute")
	default:
		e.lineNumberMode = LineNumberAbsolute
		e.setStatus("line numbers absolute")
	}
}
func (e *Editor) gutterWidth() int {
	diffWidth := 0
	if e.hasConflictBlocks() || e.gitDiffGutterActive() {
		diffWidth = 1
	}
	if e.lineNumberMode == LineNumberOff {
		return diffWidth
	}
	maxLine := e.LineCount()
	if maxLine < 1 {
		maxLine = 1
	}
	digits := len(strconv.Itoa(maxLine))
	if digits < 2 {
		digits = 2
	}
	// Format: " " + digits + " " + diff sign
	return 1 + digits + 1 + diffWidth
}

func previewGutterWidth(lineCount int, mode LineNumberMode) int {
	if mode == LineNumberOff {
		return 0
	}
	if mode == LineNumberRelative {
		mode = LineNumberAbsolute
	}
	maxLine := lineCount
	if maxLine < 1 {
		maxLine = 1
	}
	digits := len(strconv.Itoa(maxLine))
	if digits < 2 {
		digits = 2
	}
	return 1 + digits + 1
}

// fuzzyMatch checks if pattern matches text (simple substring for now)
func fuzzyMatch(pattern, text string) bool {
	if pattern == "" {
		return true
	}
	pi := 0
	for _, c := range text {
		if c == rune(pattern[pi]) {
			pi++
			if pi == len(pattern) {
				return true
			}
		}
	}
	return false
}
