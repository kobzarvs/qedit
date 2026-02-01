package editor

import "strings"

type TextBuffer struct {
	base runeBuffer
	add  runeBuffer
	root *pieceNode
}

func NewTextBufferFromBytes(data []byte) *TextBuffer {
	runes := normalizeNewlines(data)
	return NewTextBufferFromRunes(runes)
}

func NewTextBufferFromString(text string) *TextBuffer {
	runes := normalizeNewlines([]byte(text))
	return NewTextBufferFromRunes(runes)
}

func NewTextBufferFromRunes(runes []rune) *TextBuffer {
	base := newRuneBuffer(runes)
	add := newRuneBuffer(nil)
	var root *pieceNode
	if len(runes) > 0 {
		p := newPiece(pieceBase, 0, len(runes), &base)
		root = &pieceNode{piece: p, priority: nextPriority()}
		recalc(root)
	}
	return &TextBuffer{base: base, add: add, root: root}
}

func (t *TextBuffer) RuneCount() int {
	return nodeSize(t.root)
}

func (t *TextBuffer) LineCount() int {
	if t.root == nil {
		return 1
	}
	return t.root.lineBreaks + 1
}

func (t *TextBuffer) LineStartIndex(row int) int {
	if row <= 0 {
		return 0
	}
	lineCount := t.LineCount()
	if row >= lineCount {
		row = lineCount - 1
	}
	idx := t.indexOfNthNewline(row - 1)
	if idx < 0 {
		return 0
	}
	if idx+1 > t.RuneCount() {
		return t.RuneCount()
	}
	return idx + 1
}

func (t *TextBuffer) LineEndIndex(row int) int {
	lineCount := t.LineCount()
	if row < 0 {
		return 0
	}
	if row >= lineCount-1 {
		return t.RuneCount()
	}
	idx := t.indexOfNthNewline(row)
	if idx < 0 {
		return t.RuneCount()
	}
	return idx
}

func (t *TextBuffer) LineLen(row int) int {
	start := t.LineStartIndex(row)
	end := t.LineEndIndex(row)
	if end < start {
		return 0
	}
	return end - start
}

func (t *TextBuffer) Line(row int) []rune {
	start := t.LineStartIndex(row)
	end := t.LineEndIndex(row)
	return t.Slice(start, end)
}

func (t *TextBuffer) IndexForCursor(pos Cursor) int {
	row := pos.Row
	if row < 0 {
		row = 0
	}
	lineCount := t.LineCount()
	if row >= lineCount {
		row = lineCount - 1
	}
	start := t.LineStartIndex(row)
	lineLen := t.LineEndIndex(row) - start
	col := pos.Col
	if col < 0 {
		col = 0
	}
	if col > lineLen {
		col = lineLen
	}
	return start + col
}

func (t *TextBuffer) CursorForIndex(index int) Cursor {
	if index < 0 {
		index = 0
	}
	max := t.RuneCount()
	if index > max {
		index = max
	}
	row := t.lineBreaksBefore(index)
	lineStart := 0
	if row > 0 {
		idx := t.indexOfNthNewline(row - 1)
		if idx >= 0 {
			lineStart = idx + 1
		}
	}
	col := index - lineStart
	return Cursor{Row: row, Col: col}
}

func (t *TextBuffer) ByteOffset(index int) int {
	if index <= 0 {
		return 0
	}
	max := t.RuneCount()
	if index > max {
		index = max
	}
	return t.byteOffset(t.root, index)
}

func (t *TextBuffer) Slice(start, end int) []rune {
	if start < 0 {
		start = 0
	}
	max := t.RuneCount()
	if end > max {
		end = max
	}
	if start >= end {
		return nil
	}
	out := make([]rune, 0, end-start)
	t.appendSlice(t.root, start, end, &out)
	return out
}

func (t *TextBuffer) String() string {
	if t.root == nil {
		return ""
	}
	var b strings.Builder
	b.Grow(t.root.byteLen)
	t.appendString(t.root, &b)
	return b.String()
}

func (t *TextBuffer) Insert(index int, runes []rune) {
	if len(runes) == 0 {
		return
	}
	max := t.RuneCount()
	if index < 0 {
		index = 0
	}
	if index > max {
		index = max
	}
	start := t.add.appendRunes(runes)
	newPiece := newPiece(pieceAdd, start, len(runes), &t.add)
	newNode := &pieceNode{piece: newPiece, priority: nextPriority()}
	recalc(newNode)
	left, right := t.split(t.root, index)
	t.root = merge(merge(left, newNode), right)
}

func (t *TextBuffer) DeleteRange(start, end int) []rune {
	if start < 0 {
		start = 0
	}
	max := t.RuneCount()
	if end > max {
		end = max
	}
	if start >= end {
		return nil
	}
	deleted := t.Slice(start, end)
	left, right := t.split(t.root, end)
	left, _ = t.split(left, start)
	t.root = merge(left, right)
	return deleted
}

func (t *TextBuffer) ReplaceLine(row int, line []rune) bool {
	lineCount := t.LineCount()
	if row < 0 || row >= lineCount {
		return false
	}
	start := t.LineStartIndex(row)
	end := t.LineEndIndex(row)
	t.DeleteRange(start, end)
	t.Insert(start, line)
	return true
}

type pieceKind uint8

const (
	pieceBase pieceKind = iota
	pieceAdd
)

type piece struct {
	kind       pieceKind
	start      int
	length     int
	lineBreaks int
	byteLen    int
}

type pieceNode struct {
	piece      piece
	priority   uint32
	left       *pieceNode
	right      *pieceNode
	runeCount  int
	lineBreaks int
	byteLen    int
}

type runeBuffer struct {
	runes      []rune
	linePrefix []int
	bytePrefix []int
}

func newRuneBuffer(runes []rune) runeBuffer {
	buf := runeBuffer{
		runes:      runes,
		linePrefix: make([]int, len(runes)+1),
		bytePrefix: make([]int, len(runes)+1),
	}
	for i, r := range runes {
		buf.linePrefix[i+1] = buf.linePrefix[i]
		if r == '\n' {
			buf.linePrefix[i+1]++
		}
		buf.bytePrefix[i+1] = buf.bytePrefix[i] + runeByteLen(r)
	}
	return buf
}

func (b *runeBuffer) appendRunes(runes []rune) int {
	start := len(b.runes)
	b.runes = append(b.runes, runes...)
	for _, r := range runes {
		lastLine := b.linePrefix[len(b.linePrefix)-1]
		if r == '\n' {
			lastLine++
		}
		b.linePrefix = append(b.linePrefix, lastLine)
		lastBytes := b.bytePrefix[len(b.bytePrefix)-1]
		b.bytePrefix = append(b.bytePrefix, lastBytes+runeByteLen(r))
	}
	return start
}

func (b *runeBuffer) lineBreaks(start, length int) int {
	if length <= 0 {
		return 0
	}
	return b.linePrefix[start+length] - b.linePrefix[start]
}

func (b *runeBuffer) byteLen(start, length int) int {
	if length <= 0 {
		return 0
	}
	return b.bytePrefix[start+length] - b.bytePrefix[start]
}

func newPiece(kind pieceKind, start, length int, buf *runeBuffer) piece {
	lineBreaks := 0
	byteLen := 0
	if length > 0 {
		lineBreaks = buf.lineBreaks(start, length)
		byteLen = buf.byteLen(start, length)
	}
	return piece{
		kind:       kind,
		start:      start,
		length:     length,
		lineBreaks: lineBreaks,
		byteLen:    byteLen,
	}
}

var treapSeed uint32 = 1

func nextPriority() uint32 {
	treapSeed ^= treapSeed << 13
	treapSeed ^= treapSeed >> 17
	treapSeed ^= treapSeed << 5
	return treapSeed
}

func nodeSize(n *pieceNode) int {
	if n == nil {
		return 0
	}
	return n.runeCount
}

func nodeLines(n *pieceNode) int {
	if n == nil {
		return 0
	}
	return n.lineBreaks
}

func nodeBytes(n *pieceNode) int {
	if n == nil {
		return 0
	}
	return n.byteLen
}

func recalc(n *pieceNode) {
	if n == nil {
		return
	}
	n.runeCount = nodeSize(n.left) + nodeSize(n.right) + n.piece.length
	n.lineBreaks = nodeLines(n.left) + nodeLines(n.right) + n.piece.lineBreaks
	n.byteLen = nodeBytes(n.left) + nodeBytes(n.right) + n.piece.byteLen
}

func merge(a, b *pieceNode) *pieceNode {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.priority < b.priority {
		a.right = merge(a.right, b)
		recalc(a)
		return a
	}
	b.left = merge(a, b.left)
	recalc(b)
	return b
}

func (t *TextBuffer) split(n *pieceNode, index int) (*pieceNode, *pieceNode) {
	if n == nil {
		return nil, nil
	}
	leftSize := nodeSize(n.left)
	if index < leftSize {
		left, right := t.split(n.left, index)
		n.left = right
		recalc(n)
		return left, n
	}
	if index > leftSize+n.piece.length {
		left, right := t.split(n.right, index-leftSize-n.piece.length)
		n.right = left
		recalc(n)
		return n, right
	}
	if index == leftSize {
		left := n.left
		n.left = nil
		recalc(n)
		return left, n
	}
	if index == leftSize+n.piece.length {
		right := n.right
		n.right = nil
		recalc(n)
		return n, right
	}
	offset := index - leftSize
	leftPiece := t.slicePiece(n.piece, 0, offset)
	rightPiece := t.slicePiece(n.piece, offset, n.piece.length-offset)
	leftNode := &pieceNode{piece: leftPiece, priority: n.priority}
	rightNode := &pieceNode{piece: rightPiece, priority: n.priority}
	leftNode.left = n.left
	rightNode.right = n.right
	recalc(leftNode)
	recalc(rightNode)
	return leftNode, rightNode
}

func (t *TextBuffer) slicePiece(p piece, offset, length int) piece {
	if length <= 0 {
		return piece{kind: p.kind, start: p.start + offset, length: 0}
	}
	buf := t.bufferForPiece(p)
	return newPiece(p.kind, p.start+offset, length, buf)
}

func (t *TextBuffer) bufferForPiece(p piece) *runeBuffer {
	if p.kind == pieceBase {
		return &t.base
	}
	return &t.add
}

func (t *TextBuffer) pieceRunes(p piece, offset, length int) []rune {
	buf := t.bufferForPiece(p)
	start := p.start + offset
	end := start + length
	return buf.runes[start:end]
}

func (t *TextBuffer) appendSlice(n *pieceNode, start, end int, out *[]rune) {
	if n == nil || start >= end {
		return
	}
	leftSize := nodeSize(n.left)
	if start < leftSize {
		leftEnd := end
		if leftEnd > leftSize {
			leftEnd = leftSize
		}
		t.appendSlice(n.left, start, leftEnd, out)
	}
	pieceStart := start - leftSize
	if pieceStart < 0 {
		pieceStart = 0
	}
	pieceEnd := end - leftSize
	if pieceEnd > n.piece.length {
		pieceEnd = n.piece.length
	}
	if pieceStart < pieceEnd {
		*out = append(*out, t.pieceRunes(n.piece, pieceStart, pieceEnd-pieceStart)...)
	}
	if end > leftSize+n.piece.length {
		t.appendSlice(n.right, start-leftSize-n.piece.length, end-leftSize-n.piece.length, out)
	}
}

func (t *TextBuffer) appendString(n *pieceNode, out *strings.Builder) {
	if n == nil {
		return
	}
	t.appendString(n.left, out)
	runes := t.pieceRunes(n.piece, 0, n.piece.length)
	out.WriteString(string(runes))
	t.appendString(n.right, out)
}

func (t *TextBuffer) lineBreaksBefore(index int) int {
	if index <= 0 || t.root == nil {
		return 0
	}
	max := t.RuneCount()
	if index > max {
		index = max
	}
	return t.lineBreaksBeforeNode(t.root, index)
}

func (t *TextBuffer) lineBreaksBeforeNode(n *pieceNode, index int) int {
	if n == nil {
		return 0
	}
	leftSize := nodeSize(n.left)
	if index <= leftSize {
		return t.lineBreaksBeforeNode(n.left, index)
	}
	if index <= leftSize+n.piece.length {
		breaks := nodeLines(n.left)
		offset := index - leftSize
		breaks += t.pieceLineBreaksPrefix(n.piece, offset)
		return breaks
	}
	breaks := nodeLines(n.left) + n.piece.lineBreaks
	return breaks + t.lineBreaksBeforeNode(n.right, index-leftSize-n.piece.length)
}

func (t *TextBuffer) byteOffset(n *pieceNode, index int) int {
	if n == nil {
		return 0
	}
	leftSize := nodeSize(n.left)
	if index <= leftSize {
		return t.byteOffset(n.left, index)
	}
	if index <= leftSize+n.piece.length {
		bytes := nodeBytes(n.left)
		offset := index - leftSize
		bytes += t.pieceByteLenPrefix(n.piece, offset)
		return bytes
	}
	bytes := nodeBytes(n.left) + n.piece.byteLen
	return bytes + t.byteOffset(n.right, index-leftSize-n.piece.length)
}

func (t *TextBuffer) pieceLineBreaksPrefix(p piece, length int) int {
	if length <= 0 {
		return 0
	}
	if length > p.length {
		length = p.length
	}
	buf := t.bufferForPiece(p)
	start := p.start
	return buf.linePrefix[start+length] - buf.linePrefix[start]
}

func (t *TextBuffer) pieceByteLenPrefix(p piece, length int) int {
	if length <= 0 {
		return 0
	}
	if length > p.length {
		length = p.length
	}
	buf := t.bufferForPiece(p)
	start := p.start
	return buf.bytePrefix[start+length] - buf.bytePrefix[start]
}

func (t *TextBuffer) indexOfNthNewline(n int) int {
	if n < 0 || t.root == nil {
		return -1
	}
	idx := 0
	node := t.root
	for node != nil {
		leftBreaks := nodeLines(node.left)
		if n < leftBreaks {
			node = node.left
			continue
		}
		n -= leftBreaks
		if n < node.piece.lineBreaks {
			offset := t.pieceNthNewlineOffset(node.piece, n)
			return idx + nodeSize(node.left) + offset
		}
		n -= node.piece.lineBreaks
		idx += nodeSize(node.left) + node.piece.length
		node = node.right
	}
	return -1
}

func (t *TextBuffer) pieceNthNewlineOffset(p piece, n int) int {
	buf := t.bufferForPiece(p)
	start := p.start
	end := p.start + p.length
	base := buf.linePrefix[start]
	target := base + n + 1
	lo := start
	hi := end
	for lo < hi {
		mid := (lo + hi) / 2
		if buf.linePrefix[mid+1] >= target {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo - start
}

func normalizeNewlines(data []byte) []rune {
	if len(data) == 0 {
		return nil
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	return []rune(text)
}
