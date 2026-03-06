package editor

import (
	"strconv"
	"time"
)

type resizeTarget int

const (
	resizeTargetNone resizeTarget = iota
	resizeTargetSidebar
)

func (e *Editor) HandleMouse(ev EventMouse) {
	// Intercept mouse events when modal is open
	if e.keybindingsHelpActive {
		if ev.Buttons() == WheelUp {
			if e.keybindingsHelpScroll > 0 {
				e.keybindingsHelpScroll--
			}
		} else if ev.Buttons() == WheelDown {
			e.keybindingsHelpScroll++
		}
		return
	}

	if e.handleMouseResize(ev) {
		return
	}

	if ev.Buttons() == WheelUp {
		e.scrollUp(1)
		e.freeScroll = true
		e.lastScrollTime = time.Now()
	} else if ev.Buttons() == WheelDown {
		e.scrollDown(1)
		e.freeScroll = true
		e.lastScrollTime = time.Now()
	} else if ev.Buttons() == WheelLeft {
		e.scrollLeft(1)
	} else if ev.Buttons() == WheelRight {
		textWidth := e.viewWidth - e.gutterWidth()
		e.scrollRight(1, textWidth)
	} else if ev.Buttons() == Button1 {
		e.handleMouseClick(ev)
	}
}

func (e *Editor) handleMouseResize(ev EventMouse) bool {
	buttons := ev.Buttons()
	if e.resizeDragging {
		if buttons == 0 {
			e.finishMouseResize()
			return true
		}
		if buttons == Button1 {
			x, _ := ev.Position()
			e.updateMouseResize(x)
		}
		return true
	}

	if buttons != Button1 {
		return false
	}

	x, y := ev.Position()
	if y < 0 || y >= e.viewHeight {
		return false
	}
	if e.tryStartSidebarResize(x) {
		return true
	}
	return false
}

func (e *Editor) tryStartSidebarResize(x int) bool {
	if e.sidebar == nil || !e.sidebar.Visible {
		return false
	}
	if e.viewWidth <= 0 {
		return false
	}
	sidebarWidth := e.sidebar.CalculateWidth(e.viewWidth)
	if sidebarWidth <= 0 {
		return false
	}
	if x != sidebarWidth-1 {
		return false
	}
	e.resizeDragging = true
	e.resizeTarget = resizeTargetSidebar
	e.updateSidebarWidthFromX(x)
	return true
}

func (e *Editor) updateMouseResize(x int) {
	switch e.resizeTarget {
	case resizeTargetSidebar:
		e.updateSidebarWidthFromX(x)
	}
}

func (e *Editor) updateSidebarWidthFromX(x int) {
	if e.sidebar == nil {
		return
	}
	width := x + 1
	width = e.clampSidebarWidth(width)
	e.sidebar.WidthConfig = strconv.Itoa(width)
}

func (e *Editor) clampSidebarWidth(width int) int {
	if width < 0 {
		width = 0
	}
	if e.sidebar == nil {
		return width
	}
	if width < e.sidebar.MinWidth {
		width = e.sidebar.MinWidth
	}
	maxWidth := parseWidthValue(e.sidebar.MaxWidthConfig, e.viewWidth)
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
	}
	if e.viewWidth > 0 && width > e.viewWidth/2 {
		width = e.viewWidth / 2
	}
	return width
}

func (e *Editor) finishMouseResize() {
	switch e.resizeTarget {
	case resizeTargetSidebar:
		if e.runtime.sidebarWidthConfigHook != nil && e.sidebar != nil {
			if err := e.runtime.sidebarWidthConfigHook(e.sidebar.WidthConfig); err != nil {
				e.setStatus("config write failed: " + err.Error())
			}
		}
	}
	e.resizeDragging = false
	e.resizeTarget = resizeTargetNone
}

func (e *Editor) scrollLeft(amount int) {
	e.scrollX -= amount
	if e.scrollX < 0 {
		e.scrollX = 0
	}
}

func (e *Editor) scrollRight(amount, textWidth int) {
	e.scrollX += amount
	e.clampScrollX(textWidth)
}

// clampScrollX limits horizontal scroll so text doesn't scroll past the end.
func (e *Editor) clampScrollX(textWidth int) {
	maxX := e.maxVisibleLineWidth() - textWidth + 10
	if maxX < 0 {
		maxX = 0
	}
	if e.scrollX > maxX {
		e.scrollX = maxX
	}
	if e.scrollX < 0 {
		e.scrollX = 0
	}
}

// maxVisibleLineWidth returns the maximum visual width of lines in the visible
// area plus 2 lines above and below (buffer zone).
func (e *Editor) maxVisibleLineWidth() int {
	lineCount := e.LineCount()
	if lineCount == 0 {
		return 0
	}
	startLine := e.scroll - 2
	if startLine < 0 {
		startLine = 0
	}
	endLine := e.scroll + e.viewHeight + 2
	if endLine > lineCount {
		endLine = lineCount
	}
	maxWidth := 0
	for i := startLine; i < endLine; i++ {
		line := e.text.Line(i)
		w := visualCol(line, len(line), e.tabWidth)
		if w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

func (e *Editor) handleMouseClick(ev EventMouse) {
	x, y := ev.Position()
	if e.sidebar != nil && e.sidebar.Visible {
		sidebarWidth := e.sidebar.CalculateWidth(e.viewWidth)
		if x < sidebarWidth {
			e.sidebar.Focused = true
			return
		}
		e.sidebar.Focused = false
	}

	// Convert screen Y to line number
	row := y + e.scroll
	if row < 0 {
		row = 0
	}
	lineCount := e.LineCount()
	if row >= lineCount {
		row = lineCount - 1
	}
	if row < 0 {
		return // empty file
	}

	// Convert screen X to column (accounting for gutter and horizontal scroll)
	gutterW := e.gutterWidth()
	visualX := x - gutterW + e.scrollX
	if visualX < 0 {
		visualX = 0
	}

	// Convert visual column to logical column
	col := visualToLogicalCol(e.text.Line(row), visualX, e.tabWidth)

	// Set cursor position
	e.cursor.Row = row
	e.cursor.Col = col
	e.clampCursorCol()

	// Clear selection and free scroll mode
	e.selectionActive = false
	e.freeScroll = false
}

func (e *Editor) scrollUp(lines int) {
	e.scroll -= lines
	if e.scroll < 0 {
		e.scroll = 0
	}
}

func (e *Editor) scrollDown(lines int) {
	// Keep last line at least 5 lines above status line
	viewHeight := e.viewHeightCached()
	maxScroll := e.LineCount() - viewHeight + 5
	if maxScroll < 0 {
		maxScroll = 0
	}
	e.scroll += lines
	if e.scroll > maxScroll {
		e.scroll = maxScroll
	}
}

// scrollViewUp scrolls the view up (shows earlier lines), keeping cursor visible
func (e *Editor) scrollViewUp() {
	if e.scroll <= 0 {
		return
	}
	e.scroll--
	e.lastScrollTime = time.Now()
	// If cursor is now below visible area, move it up
	viewHeight := e.viewHeightCached()
	if e.cursor.Row >= e.scroll+viewHeight {
		e.cursor.Row = e.scroll + viewHeight - 1
		e.clampCursorCol()
	}
}

// scrollViewDown scrolls the view down (shows later lines), keeping cursor visible
func (e *Editor) scrollViewDown() {
	// Keep last line at least 5 lines above status line
	viewHeight := e.viewHeightCached()
	maxScroll := e.LineCount() - viewHeight + 5
	if maxScroll < 0 {
		maxScroll = 0
	}
	if e.scroll >= maxScroll {
		return
	}
	e.scroll++
	e.lastScrollTime = time.Now()
	// If cursor is now above visible area, move it down
	if e.cursor.Row < e.scroll {
		e.cursor.Row = e.scroll
		e.clampCursorCol()
	}
}
