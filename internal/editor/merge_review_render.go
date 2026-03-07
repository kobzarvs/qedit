package editor

import (
	"fmt"
	"strings"
)

type mergeReviewLayout struct {
	leftX    int
	leftW    int
	centerX  int
	centerW  int
	rightX   int
	rightW   int
	headerH  int
	sepLeftX int
	sepMidX  int
}

func (l mergeReviewLayout) paneAt(x int) (mergeReviewPane, bool) {
	switch {
	case x >= l.leftX && x < l.leftX+l.leftW:
		return mergeReviewPaneLocal, true
	case x >= l.centerX && x < l.centerX+l.centerW:
		return mergeReviewPaneResult, true
	case x >= l.rightX && x < l.rightX+l.rightW:
		return mergeReviewPaneRemote, true
	default:
		return mergeReviewPaneResult, false
	}
}

func (e *Editor) computeMergeReviewLayout(totalWidth int) mergeReviewLayout {
	if totalWidth < 3 {
		return mergeReviewLayout{centerW: totalWidth}
	}
	usable := totalWidth - 2
	if usable < 3 {
		usable = totalWidth
	}
	leftW := usable / 3
	rightW := usable / 3
	centerW := usable - leftW - rightW
	if centerW < leftW {
		centerW = leftW
	}
	used := leftW + centerW + rightW
	if used > totalWidth {
		centerW -= used - totalWidth
	}
	leftX := 0
	sepLeftX := leftX + leftW
	centerX := sepLeftX + 1
	sepMidX := centerX + centerW
	rightX := sepMidX + 1
	return mergeReviewLayout{
		leftX:    leftX,
		leftW:    leftW,
		centerX:  centerX,
		centerW:  centerW,
		rightX:   rightX,
		rightW:   totalWidth - rightX,
		headerH:  1,
		sepLeftX: sepLeftX,
		sepMidX:  sepMidX,
	}
}

func (e *Editor) renderMergeReview(s Screen, viewHeight, totalWidth int) mergeReviewLayout {
	layout := e.computeMergeReviewLayout(totalWidth)
	if viewHeight <= 0 || totalWidth <= 0 {
		return layout
	}

	e.drawMergeReviewSeparator(s, layout.sepLeftX, viewHeight)
	e.drawMergeReviewSeparator(s, layout.sepMidX, viewHeight)
	e.drawMergeReviewHeader(s, layout.leftX, layout.leftW, 0, e.mergeReviewPaneLabel(mergeReviewPaneLocal), mergeReviewPaneLocal)
	e.drawMergeReviewHeader(s, layout.centerX, layout.centerW, 0, e.mergeReviewPaneLabel(mergeReviewPaneResult), mergeReviewPaneResult)
	e.drawMergeReviewHeader(s, layout.rightX, layout.rightW, 0, e.mergeReviewPaneLabel(mergeReviewPaneRemote), mergeReviewPaneRemote)

	contentY := layout.headerH
	contentHeight := viewHeight - layout.headerH
	if contentHeight <= 0 {
		return layout
	}

	centerGutterWidth := e.gutterWidth()
	for y := 0; y < contentHeight; y++ {
		lineIdx := e.viewport.scroll + y
		if lineIdx >= e.LineCount() {
			clearLineAt(s, layout.centerX, contentY+y, layout.centerW, e.styleMain)
			continue
		}
		e.drawLineWithGutterAt(s, layout.centerX, contentY+y, layout.centerW, centerGutterWidth, lineIdx)
	}

	e.renderMergeReviewSidePane(s, layout.leftX, contentY, layout.leftW, contentHeight, mergeReviewPaneLocal)
	e.renderMergeReviewSidePane(s, layout.rightX, contentY, layout.rightW, contentHeight, mergeReviewPaneRemote)
	e.renderMergeReviewActiveBlockMarkers(s, layout, contentY, contentHeight)
	e.renderMergeReviewApplyHandles(s, layout, contentY, contentHeight)
	return layout
}

func (e *Editor) drawMergeReviewSeparator(s Screen, x, height int) {
	if x < 0 || height <= 0 {
		return
	}
	style := e.styleLineNumber
	if style == nil {
		style = e.styleMain
	}
	for y := 0; y < height; y++ {
		s.SetContent(x, y, '│', nil, style)
	}
}

func (e *Editor) mergeReviewHeaderStyle(pane mergeReviewPane) Style {
	base := e.styleStatus
	if base == nil {
		base = e.styleMain
	}
	if e.conflicts.review.pane != pane {
		return base
	}
	if e.styleSelection == nil {
		return base
	}
	fg, _, _ := base.Decompose()
	_, bg, _ := e.styleSelection.Decompose()
	return base.Foreground(fg).Background(bg)
}

func (e *Editor) drawMergeReviewHeader(s Screen, x0, width, y int, label string, pane mergeReviewPane) {
	if width <= 0 {
		return
	}
	style := e.mergeReviewHeaderStyle(pane)
	text := e.mergeReviewHeaderText(label, pane)
	runes := []rune(text)
	for x := 0; x < width; x++ {
		r := ' '
		if x < len(runes) {
			r = runes[x]
		}
		s.SetContent(x0+x, y, r, nil, style)
	}
}

func (e *Editor) mergeReviewHeaderText(label string, pane mergeReviewPane) string {
	switch pane {
	case mergeReviewPaneLocal, mergeReviewPaneRemote:
		if pane == e.conflicts.review.pane {
			return " [" + label + "] apply "
		}
		return " " + label + " select "
	default:
		return " " + label + " "
	}
}

func (e *Editor) renderMergeReviewSidePane(s Screen, x0, y0, width, height int, pane mergeReviewPane) {
	if width <= 0 || height <= 0 {
		return
	}
	for y := 0; y < height; y++ {
		clearLineAt(s, x0, y0+y, width, e.styleMain)
	}
	gutterWidth := previewGutterWidth(e.LineCount(), e.display.lineNumberMode)

	for y := 0; y < height; y++ {
		resultRow := e.viewport.scroll + y
		if resultRow >= e.LineCount() {
			continue
		}
		text, number, kind, actualRow := e.mergeReviewPaneContentAt(resultRow, pane)
		e.drawMergeReviewPaneLine(s, x0, y0+y, width, gutterWidth, text, number, actualRow, kind)
	}
}

func (e *Editor) mergeReviewPaneContentAt(resultRow int, pane mergeReviewPane) ([]rune, int, conflictLineKind, int) {
	localBefore, remoteBefore := e.conflictLineOffsets(resultRow)
	localNum := resultRow + 1 - remoteBefore
	remoteNum := resultRow + 1 - localBefore
	if localNum < 1 {
		localNum = 1
	}
	if remoteNum < 1 {
		remoteNum = 1
	}

	kind, _ := e.conflictLineInfo(resultRow)
	switch kind {
	case conflictLocal:
		if pane == mergeReviewPaneLocal {
			return e.text.Line(resultRow), localNum, conflictLocal, resultRow
		}
		return nil, 0, conflictRemote, -1
	case conflictRemote:
		if pane == mergeReviewPaneRemote {
			return e.text.Line(resultRow), remoteNum, conflictRemote, resultRow
		}
		return nil, 0, conflictLocal, -1
	default:
		if pane == mergeReviewPaneRemote {
			return e.text.Line(resultRow), remoteNum, conflictNone, resultRow
		}
		return e.text.Line(resultRow), localNum, conflictNone, resultRow
	}
}

func (e *Editor) drawMergeReviewPaneLine(s Screen, x0, y, width, gutterWidth int, line []rune, number int, actualRow int, kind conflictLineKind) {
	if gutterWidth >= width {
		return
	}

	if gutterWidth > 0 && e.display.lineNumberMode != LineNumberOff {
		digits := gutterWidth - 2
		if digits < 1 {
			digits = 1
		}
		numStr := fmt.Sprintf("%*s", digits, "")
		if number > 0 {
			numStr = fmt.Sprintf("%*d", digits, number)
		}
		style := e.styleLineNumber
		spaceStyle := e.styleMain
		if signStyle, ok := e.mergeReviewLineStyle(kind); ok {
			style = signStyle
			spaceStyle = signStyle
		}
		if width > 0 {
			s.SetContent(x0, y, ' ', nil, spaceStyle)
		}
		for i, r := range numStr {
			x := 1 + i
			if x >= gutterWidth-1 || x >= width {
				break
			}
			s.SetContent(x0+x, y, r, nil, style)
		}
		if gutterWidth-1 < width {
			s.SetContent(x0+gutterWidth-1, y, ' ', nil, spaceStyle)
		}
	}

	highlightActive := actualRow >= 0 && e.highlight.start >= 0 && actualRow >= e.highlight.start && actualRow <= e.highlight.end
	var spans []HighlightSpan
	if highlightActive {
		spans = e.highlight.spans[actualRow]
	}
	lineIdx := actualRow
	if lineIdx < 0 {
		lineIdx = 0
	}
	e.drawLine(s, y, x0+width, x0+gutterWidth, line, e.display.tabWidth, -1, -1, spans, highlightActive, nil, lineIdx, 0, 0, kind)
}

func (e *Editor) mergeReviewLineStyle(kind conflictLineKind) (Style, bool) {
	switch kind {
	case conflictLocal:
		if e.styleMergeLocal != nil {
			return e.styleMergeLocal, true
		}
	case conflictRemote:
		if e.styleMergeRemote != nil {
			return e.styleMergeRemote, true
		}
	}
	return nil, false
}

func (e *Editor) mergeReviewInstruction() string {
	parts := []string{"Left/Right or Tab choose pane", "Enter apply", "F3 next conflict", "Shift+F3 prev", "Esc exit"}
	return strings.Join(parts, " | ")
}

func (e *Editor) renderMergeReviewActiveBlockMarkers(s Screen, layout mergeReviewLayout, contentY, contentHeight int) {
	block, _ := e.mergeReviewBlock()
	if block == nil || contentHeight <= 0 {
		return
	}
	start := maxInt(block.start, e.viewport.scroll)
	end := minInt(block.end, e.viewport.scroll+contentHeight-1)
	if start > end {
		return
	}
	style := e.styleLineNumberActive
	if style == nil {
		style = e.styleStatus
	}
	if style == nil {
		style = e.styleMain
	}
	for row := start; row <= end; row++ {
		screenY := contentY + row - e.viewport.scroll
		if screenY < contentY || screenY >= contentY+contentHeight {
			continue
		}
		s.SetContent(layout.leftX, screenY, '▌', nil, style)
		s.SetContent(layout.centerX, screenY, '▌', nil, style)
		s.SetContent(layout.rightX, screenY, '▌', nil, style)
	}
}

func (e *Editor) renderMergeReviewApplyHandles(s Screen, layout mergeReviewLayout, contentY, contentHeight int) {
	block, _ := e.mergeReviewBlock()
	if block == nil || contentHeight <= 0 {
		return
	}
	row := maxInt(block.start, e.viewport.scroll)
	screenY := contentY + row - e.viewport.scroll
	if screenY < contentY || screenY >= contentY+contentHeight {
		return
	}
	s.SetContent(layout.sepLeftX, screenY, '▶', nil, e.mergeReviewApplyHandleStyle(mergeReviewPaneLocal))
	s.SetContent(layout.sepMidX, screenY, '◀', nil, e.mergeReviewApplyHandleStyle(mergeReviewPaneRemote))
}

func (e *Editor) mergeReviewApplyHandleStyle(pane mergeReviewPane) Style {
	style := e.styleLineNumberActive
	if style == nil {
		style = e.styleStatus
	}
	if style == nil {
		style = e.styleMain
	}
	if e.conflicts.review.pane == pane {
		return e.mergeReviewHeaderStyle(pane)
	}
	return style
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
