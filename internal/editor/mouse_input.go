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
	if e.keybindingsHelp.active {
		if ev.Buttons() == WheelUp {
			if e.keybindingsHelp.scroll > 0 {
				e.keybindingsHelp.scroll--
			}
		} else if ev.Buttons() == WheelDown {
			e.keybindingsHelp.scroll++
		}
		return
	}

	if e.handleMouseResize(ev) {
		return
	}

	if ev.Buttons() == WheelUp {
		e.scrollUp(1)
		e.interaction.freeScroll = true
		e.interaction.lastScrollTime = time.Now()
	} else if ev.Buttons() == WheelDown {
		e.scrollDown(1)
		e.interaction.freeScroll = true
		e.interaction.lastScrollTime = time.Now()
	} else if ev.Buttons() == WheelLeft {
		e.scrollLeft(1)
	} else if ev.Buttons() == WheelRight {
		textWidth := e.viewport.width - e.gutterWidth()
		e.scrollRight(1, textWidth)
	} else if ev.Buttons() == Button1 {
		e.handleMouseClick(ev)
	}
}

func (e *Editor) handleMouseResize(ev EventMouse) bool {
	buttons := ev.Buttons()
	if e.interaction.resizeDragging {
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
	if y < 0 || y >= e.viewport.height {
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
	if e.viewport.width <= 0 {
		return false
	}
	sidebarWidth := e.sidebar.CalculateWidth(e.viewport.width)
	if sidebarWidth <= 0 {
		return false
	}
	if x != sidebarWidth-1 {
		return false
	}
	e.interaction.resizeSidebarWidth = e.sidebar.WidthConfig
	e.interaction.resizeDragging = true
	e.interaction.resizeTarget = resizeTargetSidebar
	e.updateSidebarWidthFromX(x)
	return true
}

func (e *Editor) updateMouseResize(x int) {
	switch e.interaction.resizeTarget {
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
	maxWidth := parseWidthValue(e.sidebar.MaxWidthConfig, e.viewport.width)
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
	}
	if e.viewport.width > 0 && width > e.viewport.width/2 {
		width = e.viewport.width / 2
	}
	return width
}

func (e *Editor) finishMouseResize() {
	switch e.interaction.resizeTarget {
	case resizeTargetSidebar:
		if e.sidebar != nil {
			e.enqueueRuntimeRequest(RuntimeRequest{
				Kind:      RuntimeRequestPersistSidebarWidth,
				Value:     e.sidebar.WidthConfig,
				PrevValue: e.interaction.resizeSidebarWidth,
			})
		}
	}
	e.interaction.resizeDragging = false
	e.interaction.resizeTarget = resizeTargetNone
	e.interaction.resizeSidebarWidth = ""
}

func (e *Editor) scrollLeft(amount int) {
	e.viewport.scrollX -= amount
	if e.viewport.scrollX < 0 {
		e.viewport.scrollX = 0
	}
}

func (e *Editor) scrollRight(amount, textWidth int) {
	e.viewport.scrollX += amount
	e.clampScrollX(textWidth)
}

// clampScrollX limits horizontal scroll so text doesn't scroll past the end.
func (e *Editor) clampScrollX(textWidth int) {
	if e.hugeFileActive() {
		if e.viewport.scrollX < 0 {
			e.viewport.scrollX = 0
		}
		return
	}
	maxX := e.maxVisibleLineWidth() - textWidth + 10
	if maxX < 0 {
		maxX = 0
	}
	if e.viewport.scrollX > maxX {
		e.viewport.scrollX = maxX
	}
	if e.viewport.scrollX < 0 {
		e.viewport.scrollX = 0
	}
}

// maxVisibleLineWidth returns the maximum visual width of lines in the visible
// area plus 2 lines above and below (buffer zone).
func (e *Editor) maxVisibleLineWidth() int {
	lineCount := e.LineCount()
	if lineCount == 0 {
		return 0
	}
	startLine := e.viewport.scroll - 2
	if startLine < 0 {
		startLine = 0
	}
	endLine := e.viewport.scroll + e.viewport.height + 2
	if endLine > lineCount {
		endLine = lineCount
	}
	maxWidth := 0
	for i := startLine; i < endLine; i++ {
		w := 0
		if vc, ok := e.hugeVisualCol(i, 1<<30); ok {
			w = vc
		} else {
			line := e.lineForDisplay(i)
			w = visualCol(line, len(line), e.display.tabWidth)
		}
		if w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

func (e *Editor) handleMouseClick(ev EventMouse) {
	x, y := ev.Position()
	if e.mergeReviewActive() && e.handleMergeReviewMouseClick(x, y) {
		return
	}
	if e.sidebar != nil && e.sidebar.Visible {
		sidebarWidth := e.sidebar.CalculateWidth(e.viewport.width)
		if x < sidebarWidth {
			e.sidebar.Focused = true
			return
		}
		e.sidebar.Focused = false
	}

	// Convert screen Y to line number
	row := y + e.viewport.scroll
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
	visualX := x - gutterW + e.viewport.scrollX
	if visualX < 0 {
		visualX = 0
	}

	// Convert visual column to logical column
	col := 0
	if line, ok := e.tryLine(row); ok {
		col = visualToLogicalCol(line, visualX, e.display.tabWidth)
	}

	// Set cursor position
	e.cursor.Row = row
	e.cursor.Col = col
	e.clampCursorCol()

	// Clear selection and free scroll mode
	e.selectionActive = false
	e.interaction.freeScroll = false
}

func (e *Editor) handleMergeReviewMouseClick(x, y int) bool {
	layout := e.computeMergeReviewLayout(e.viewport.width)
	fullHeight := e.viewport.height + layout.headerH
	if x < 0 || x >= e.viewport.width || y < 0 || y >= fullHeight {
		return false
	}
	if y >= layout.headerH {
		row := e.viewport.scroll + (y - layout.headerH)
		if row >= 0 && row < e.LineCount() && e.mergeReviewHandleSeparatorClick(x, row, layout) {
			return true
		}
	}

	pane, ok := layout.paneAt(x)
	if !ok {
		return true
	}
	if pane == mergeReviewPaneLocal || pane == mergeReviewPaneRemote {
		e.setMergeReviewPane(pane)
	}

	if y < layout.headerH {
		return true
	}

	row := e.viewport.scroll + (y - layout.headerH)
	if row < 0 || row >= e.LineCount() {
		return true
	}

	switch pane {
	case mergeReviewPaneLocal, mergeReviewPaneRemote:
		if e.mergeReviewBlockContainsRow(row) {
			e.applyMergeReviewPane(pane)
			return true
		}
		e.cursor.Row = row
		e.cursor.Col = 0
		e.clampCursorCol()
		e.ensureCursorVisible(e.viewHeightCached())
		e.selectionActive = false
		e.interaction.freeScroll = false
		return true
	case mergeReviewPaneResult:
		e.setCursorFromEditorClick(x, y-layout.headerH, layout.centerX)
		return true
	default:
		return true
	}
}

func (e *Editor) mergeReviewHandleSeparatorClick(x, row int, layout mergeReviewLayout) bool {
	if !e.mergeReviewBlockContainsRow(row) {
		return false
	}
	switch x {
	case layout.sepLeftX:
		e.setMergeReviewPane(mergeReviewPaneLocal)
		e.applyMergeReviewPane(mergeReviewPaneLocal)
		return true
	case layout.sepMidX:
		e.setMergeReviewPane(mergeReviewPaneRemote)
		e.applyMergeReviewPane(mergeReviewPaneRemote)
		return true
	default:
		return false
	}
}

func (e *Editor) setCursorFromEditorClick(screenX, contentY, editorX int) {
	row := contentY + e.viewport.scroll
	if row < 0 {
		row = 0
	}
	lineCount := e.LineCount()
	if row >= lineCount {
		row = lineCount - 1
	}
	if row < 0 {
		return
	}

	gutterW := e.gutterWidth()
	visualX := screenX - editorX - gutterW + e.viewport.scrollX
	if visualX < 0 {
		visualX = 0
	}
	col := 0
	if vc, ok := e.hugeVisualCol(row, visualX); ok {
		col = vc
	} else if line, ok := e.tryLine(row); ok {
		col = visualToLogicalCol(line, visualX, e.display.tabWidth)
	}

	e.cursor.Row = row
	e.cursor.Col = col
	e.clampCursorCol()
	e.selectionActive = false
	e.interaction.freeScroll = false
}

func (e *Editor) scrollUp(lines int) {
	e.viewport.scroll -= lines
	if e.viewport.scroll < 0 {
		e.viewport.scroll = 0
	}
	e.afterEditorViewChanged()
}

func (e *Editor) scrollDown(lines int) {
	viewHeight := e.paneViewHeight()
	maxScroll := e.LineCount() - viewHeight + 5
	if maxScroll < 0 {
		maxScroll = 0
	}
	e.viewport.scroll += lines
	if e.viewport.scroll > maxScroll {
		e.viewport.scroll = maxScroll
	}
	e.afterEditorViewChanged()
}

// scrollViewUp scrolls the view up (shows earlier lines), keeping cursor visible
func (e *Editor) scrollViewUp() {
	if e.viewport.scroll <= 0 {
		return
	}
	e.viewport.scroll--
	e.interaction.lastScrollTime = time.Now()
	// If cursor is now below visible area, move it up
	viewHeight := e.paneViewHeight()
	if e.cursor.Row >= e.viewport.scroll+viewHeight {
		e.cursor.Row = e.viewport.scroll + viewHeight - 1
		e.clampCursorCol()
	}
	e.afterEditorViewChanged()
}

// scrollViewDown scrolls the view down (shows later lines), keeping cursor visible
func (e *Editor) scrollViewDown() {
	viewHeight := e.paneViewHeight()
	maxScroll := e.LineCount() - viewHeight + 5
	if maxScroll < 0 {
		maxScroll = 0
	}
	if e.viewport.scroll >= maxScroll {
		return
	}
	e.viewport.scroll++
	e.interaction.lastScrollTime = time.Now()
	// If cursor is now above visible area, move it down
	if e.cursor.Row < e.viewport.scroll {
		e.cursor.Row = e.viewport.scroll
		e.clampCursorCol()
	}
	e.afterEditorViewChanged()
}
