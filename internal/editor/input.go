package editor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type resizeTarget int

const (
	resizeTargetNone resizeTarget = iota
	resizeTargetSidebar
)

func (e *Editor) HandleKey(ev EventKey) bool {
	e.freeScroll = false
	if e.mode != ModeCommand && e.mode != ModeSearch && e.statusMessage != "" && !e.autoReloadInProgress {
		e.statusMessage = ""
	}
	// Track last key combination for display
	e.lastKeyCombo = keyStringDisplay(ev)

	if handled, quit := e.handleGlobalFocusHotkeys(ev); handled {
		return quit
	}

	// Handle sidebar if focused
	if e.sidebar != nil && e.sidebar.Visible && e.sidebar.Focused {
		return e.handleSidebarKey(ev)
	}

	switch e.mode {
	case ModeInsert:
		return e.handleInsert(ev)
	case ModeCommand:
		return e.handleCommand(ev)
	case ModeBranchPicker:
		return e.handleBranchPicker(ev)
	case ModeSearch:
		return e.handleSearch(ev)
	case ModeMerge:
		return e.handleMerge(ev)
	default:
		return e.handleNormal(ev)
	}
}

func (e *Editor) handleGlobalFocusHotkeys(ev EventKey) (bool, bool) {
	if ev.Modifiers()&ModAlt == 0 {
		return false, false
	}

	key := keyStringForMap(ev, e.keymap.normal)
	action, ok := e.keymap.normal[key]
	if !ok {
		return false, false
	}

	switch action {
	case actionToggleSidebar,
		actionToggleSidebarFocus,
		actionFocusEditor,
		actionFocusPrevPane,
		actionFocusNextPane,
		actionFocusSidebar,
		actionFocusCommandLine:
		return true, e.execAction(action)
	default:
		return false, false
	}
}
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
		if e.sidebarWidthConfigHook != nil && e.sidebar != nil {
			if err := e.sidebarWidthConfigHook(e.sidebar.WidthConfig); err != nil {
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
func (e *Editor) handleNormal(ev EventKey) bool {
	// Handle zoom mode - only allow = (more zoom) or space (restore)
	if e.zoomPendingRestore {
		if ev.Key() == KeyRune {
			switch ev.Rune() {
			case ' ':
				e.zoomWithAnimation(false, 20) // zoom out with scroll restore
				e.zoomPendingRestore = false
				return false
			case '=':
				e.zoomWithAnimation(true, 20) // zoom in more, keep centering
				return false
			}
		}
		// Block all other keys during zoom mode
		return false
	}

	// Handle space menu
	if e.spaceMenuActive {
		return e.handleSpaceMenu(ev)
	}

	// Handle refs picker - only intercept navigation keys, let others fall through
	if e.refsPickerActive {
		if handled := e.handleRefsPicker(ev); handled {
			return false
		}
		// Key not handled by refs picker, continue to normal handling
	}

	// Handle keybindings help popup
	if e.keybindingsHelpActive {
		return e.handleKeybindingsHelp(ev)
	}

	// Handle goto mode (g prefix)
	if e.gotoMode {
		e.gotoMode = false
		e.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleGotoKey(ev.Rune())
		}
		return false
	}

	// Handle match mode (m prefix)
	if e.matchMode {
		e.matchMode = false
		e.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleMatchKey(ev.Rune())
		}
		return false
	}

	// Handle view mode (z prefix)
	if e.viewMode {
		e.viewMode = false
		e.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleViewKey(ev.Rune())
		}
		return false
	}

	// Handle window mode (space-w prefix)
	if e.windowMode {
		e.windowMode = false
		e.pendingKeys = ""
		if ev.Key() == KeyEscape {
			return false
		}
		if ev.Key() == KeyRune {
			return e.handleWindowKey(ev.Rune())
		}
		return false
	}

	// Handle pending char input (f/F/t/T/r)
	if e.pendingAction != "" {
		pendingKey := e.pendingKeys
		e.pendingKeys = ""
		if ev.Key() == KeyEscape {
			e.pendingAction = ""
			return false
		}
		if ev.Key() == KeyRune {
			e.handlePendingChar(ev.Rune())
			e.lastCommand = pendingKey + string(ev.Rune())
			return false
		}
		// Ignore other keys while waiting for char
		return false
	}

	if e.handleSelectionMove(ev) {
		return false
	}
	key := keyStringForMap(ev, e.keymap.normal)
	if key == "" {
		return false
	}
	action, ok := e.keymap.normal[key]
	if !ok {
		return false
	}

	// Helix-style: w, b, e, f, F, t, T - anchor moves to old cursor, cursor moves to target
	// Selection covers what was "jumped over"
	if isHelixSelectingMotion(action) {
		// Anchor = where cursor WAS
		anchor := e.cursor
		result := e.execAction(action)
		if anchor != e.cursor {
			// Selection from old position to new position
			e.selectionActive = true
			e.selectionStart = anchor
			e.selectionEnd = e.cursor
			e.selectMode = true
		}
		return result
	}

	// In select mode, extend selection for other motion commands
	if e.selectMode && isMotionAction(action) {
		before := e.cursor
		result := e.execAction(action)
		if before != e.cursor {
			e.selectionEnd = e.cursor
		}
		return result
	}

	return e.execAction(action)
}

// handleGotoKey handles the second key after 'g' prefix
func (e *Editor) handleGotoKey(ch rune) bool {
	// Handle LSP goto commands
	switch ch {
	case 'd':
		e.lastCommand = "gd"
		return e.lspGoto("definition")
	case 'D':
		e.lastCommand = "gD"
		return e.lspGoto("declaration")
	case 'y':
		e.lastCommand = "gy"
		return e.lspGoto("typeDefinition")
	case 'r':
		e.lastCommand = "gr"
		return e.lspGoto("references")
	case 'i':
		e.lastCommand = "gi"
		return e.lspGoto("implementation")
	case 't':
		e.lastCommand = "gt"
		e.scrollCursorToTop()
		return false
	case 'c':
		e.lastCommand = "gc"
		e.centerCursorLine()
		return false
	case 'b':
		e.lastCommand = "gb"
		e.scrollCursorToBottom()
		return false
	}

	var action string
	switch ch {
	case 'g':
		action = actionGotoFirstLine
	case 'e':
		action = actionGotoFileEnd
	case 'h':
		action = actionLineStart
	case 'l':
		action = actionLineEnd
	case 's':
		action = actionFileStart // same as gg
	case 'a':
		action = actionGotoLastAccessed
	case 'n':
		action = actionGotoNextBuffer
	case 'p':
		action = actionGotoPrevBuffer
	default:
		return false
	}

	// Record the executed command
	e.lastCommand = "g" + string(ch)

	// In select mode, extend selection
	if e.selectMode && isMotionAction(action) {
		before := e.cursor
		result := e.execAction(action)
		if before != e.cursor {
			e.selectionEnd = e.cursor
		}
		return result
	}

	return e.execAction(action)
}

// lspGoto performs an LSP goto operation
func (e *Editor) lspGoto(method string) bool {
	if e.lspGotoFunc == nil {
		e.setStatus("LSP: callback not set")
		return false
	}
	if e.filename == "" {
		e.setStatus("LSP: no file open")
		return false
	}

	locations, err := e.lspGotoFunc(method, e.filename, e.cursor.Row, e.cursor.Col)
	if err != nil {
		e.setStatus("LSP: " + err.Error())
		return false
	}
	if len(locations) == 0 {
		e.setStatus("LSP: no " + method + " found")
		return false
	}

	// For references or multiple results, show picker
	if method == "references" || len(locations) > 1 {
		title := "References"
		if method == "implementation" {
			title = "Implementations"
		} else if method == "definition" {
			title = "Definitions"
		} else if method == "declaration" {
			title = "Declarations"
		} else if method == "typeDefinition" {
			title = "Type Definitions"
		}
		e.showRefsPicker(title, locations)
		return false
	}

	// Single result: jump directly
	loc := locations[0]
	currentAbs, _ := filepath.Abs(e.filename)
	if loc.Path != currentAbs && loc.Path != e.filename {
		e.requestOpenLocation(loc.Path, loc.StartLine, loc.StartCol)
		return false
	}

	e.cursor.Row = loc.StartLine
	e.cursor.Col = loc.StartCol
	e.ensureCursorVisible(e.viewHeightCached())
	e.setStatus(method + " → line " + strconv.Itoa(loc.StartLine+1))
	return false
}

// handleMatchKey handles the second key after 'm' prefix
func (e *Editor) handleMatchKey(ch rune) bool {
	e.lastCommand = "m" + string(ch)

	switch ch {
	case 'm':
		e.goToMatchingBracket()
	case 'a':
		e.setStatus("select around (not implemented)")
	case 'i':
		e.setStatus("select inside (not implemented)")
	case 's':
		e.setStatus("surround add (not implemented)")
	case 'r':
		e.setStatus("surround replace (not implemented)")
	case 'd':
		e.setStatus("surround delete (not implemented)")
	default:
		return false
	}
	return false
}

// handleViewKey handles the second key after 'z' prefix
func (e *Editor) handleViewKey(ch rune) bool {
	e.lastCommand = "z" + string(ch)

	switch ch {
	case 'c':
		e.centerCursorLine()
	case 't':
		e.scrollCursorToTop()
	case 'b':
		e.scrollCursorToBottom()
	case 'k':
		e.scrollUp(1)
	case 'j':
		e.scrollDown(1)
	default:
		return false
	}
	return false
}

// handleWindowKey handles the second key after 'space-w' prefix
func (e *Editor) handleWindowKey(ch rune) bool {
	e.lastCommand = "SPC w" + string(ch)
	e.setStatus("window mode (not implemented)")
	return false
}

// handleKeybindingsHelp handles key input in keybindings help popup
func (e *Editor) handleKeybindingsHelp(ev EventKey) bool {
	// Get current filter based on focus
	currentFilter := func() *[]rune {
		switch e.keybindingsHelpFilterFocus {
		case 0:
			return &e.keybindingsHelpFilterKey
		case 1:
			return &e.keybindingsHelpFilterAct
		default:
			return &e.keybindingsHelpFilterDesc
		}
	}

	switch ev.Key() {
	case KeyEscape:
		e.keybindingsHelpActive = false
		e.keybindingsHelpFilterKey = nil
		e.keybindingsHelpFilterAct = nil
		e.keybindingsHelpFilterDesc = nil
		e.keybindingsHelpScroll = 0
		e.keybindingsHelpFilterFocus = 0
		return false
	case KeyEnter:
		// Clear all filters on Enter
		if len(e.keybindingsHelpFilterKey) > 0 || len(e.keybindingsHelpFilterAct) > 0 || len(e.keybindingsHelpFilterDesc) > 0 {
			e.keybindingsHelpFilterKey = nil
			e.keybindingsHelpFilterAct = nil
			e.keybindingsHelpFilterDesc = nil
			e.keybindingsHelpScroll = 0
		} else {
			e.keybindingsHelpActive = false
		}
		return false
	case KeyTab:
		// Switch between filter fields
		e.keybindingsHelpFilterFocus = (e.keybindingsHelpFilterFocus + 1) % 3
	case KeyBacktab:
		// Switch backwards
		e.keybindingsHelpFilterFocus = (e.keybindingsHelpFilterFocus + 2) % 3
	case KeyBackspace, KeyBackspace2:
		f := currentFilter()
		if len(*f) > 0 {
			*f = (*f)[:len(*f)-1]
			e.keybindingsHelpScroll = 0
		}
	case KeyUp, KeyCtrlP:
		if e.keybindingsHelpScroll > 0 {
			e.keybindingsHelpScroll--
		}
	case KeyDown, KeyCtrlN:
		e.keybindingsHelpScroll++
	case KeyPgUp:
		e.keybindingsHelpScroll -= 10
		if e.keybindingsHelpScroll < 0 {
			e.keybindingsHelpScroll = 0
		}
	case KeyPgDn:
		e.keybindingsHelpScroll += 10
	case KeyHome:
		e.keybindingsHelpScroll = 0
	case KeyEnd:
		e.keybindingsHelpScroll = 999999 // will be clamped in render
	case KeyRune:
		// Type into current filter
		f := currentFilter()
		*f = append(*f, ev.Rune())
		e.keybindingsHelpScroll = 0
	}
	return false
}

// handleSpaceMenu handles key input when space menu is active
func (e *Editor) handleSpaceMenu(ev EventKey) bool {
	if ev.Key() == KeyEscape {
		e.spaceMenuActive = false
		e.pendingKeys = ""
		return false
	}

	if ev.Key() == KeyRune {
		ch := ev.Rune()
		for _, item := range SpaceMenuItems {
			if item.Key == ch {
				e.spaceMenuActive = false
				e.pendingKeys = ""
				e.lastCommand = "SPC " + string(ch)
				return e.executeSpaceAction(item)
			}
		}
	}

	// Unknown key - close menu
	e.spaceMenuActive = false
	e.pendingKeys = ""
	return false
}

// executeSpaceAction executes the action from space menu
func (e *Editor) executeSpaceAction(item SpaceMenuItem) bool {
	if !item.Implemented {
		e.setStatus(item.Label + " (not implemented)")
		return false
	}

	switch item.Action {
	case "yank_clipboard":
		e.yankToSystemClipboard()
	case "yank_main_clipboard":
		e.yankToSystemClipboard()
	case "paste_clipboard":
		e.pasteFromSystemClipboard(false)
	case "paste_clipboard_before":
		e.pasteFromSystemClipboard(true)
	case "window_mode":
		e.windowMode = true
		e.pendingKeys = "SPC w"
		return false
	case "toggle_comment":
		e.toggleLineComment()
	case "buffer_picker":
		e.openSidebarBuffers()
	case "show_keybindings":
		e.keybindingsHelpActive = true
		e.keybindingsHelpScroll = 0
		e.keybindingsHelpFilterKey = nil
		e.keybindingsHelpFilterAct = nil
		e.keybindingsHelpFilterDesc = nil
		e.keybindingsHelpFilterFocus = 0
	default:
		e.setStatus(item.Label + " (not implemented)")
	}
	return false
}

// yankToSystemClipboard copies selection to system clipboard
func (e *Editor) yankToSystemClipboard() {
	// First yank to internal clipboard
	e.yankSelection()

	// Then copy to system clipboard if available
	if len(e.clipboard) == 0 {
		return
	}

	// Build text from clipboard
	var sb strings.Builder
	for i, line := range e.clipboard {
		if i > 0 {
			sb.WriteRune('\n')
		}
		sb.WriteString(string(line))
	}

	// Try to copy to system clipboard
	if e.systemClipboard == nil {
		e.setStatus("yanked (clipboard unavailable)")
		return
	}
	if err := e.systemClipboard.Write(sb.String()); err != nil {
		e.setStatus("yanked (clipboard unavailable)")
		return
	}
	e.setStatus("yanked to clipboard")
}

// pasteFromSystemClipboard pastes from system clipboard
func (e *Editor) pasteFromSystemClipboard(before bool) {
	// Try to get from system clipboard
	if e.systemClipboard == nil {
		e.setStatus("clipboard unavailable")
		return
	}
	text, err := e.systemClipboard.Read()
	if err != nil {
		e.setStatus("clipboard unavailable")
		return
	}

	if text == "" {
		e.setStatus("clipboard empty")
		return
	}

	// Parse into lines
	lines := strings.Split(text, "\n")
	e.clipboard = make([][]rune, len(lines))
	for i, line := range lines {
		e.clipboard[i] = []rune(line)
	}

	// Paste
	if before {
		e.pasteBefore()
	} else {
		e.pasteAfter()
	}
	e.setStatus("pasted from clipboard")
}

// isMotionAction returns true if the action is a motion that should extend selection
func isMotionAction(action string) bool {
	switch action {
	case actionMoveLeft, actionMoveRight, actionMoveUp, actionMoveDown,
		actionWordLeft, actionWordRight, actionLineStart, actionLineEnd,
		actionFileStart, actionFileEnd, actionPageUp, actionPageDown,
		actionWordForward, actionWordBackward, actionWordEnd,
		actionGotoLine, actionGotoFirstLine, actionGotoFileEnd,
		actionFindChar, actionFindCharBackward, actionTillChar, actionTillCharBackward:
		return true
	}
	return false
}

// isHelixSelectingMotion returns true if motion should auto-start selection (Helix style)
// These motions extend selection from current position to target
func isHelixSelectingMotion(action string) bool {
	switch action {
	case actionWordForward, actionWordBackward, actionWordEnd,
		actionFindChar, actionFindCharBackward, actionTillChar, actionTillCharBackward:
		return true
	}
	return false
}
func (e *Editor) handleInsert(ev EventKey) bool {
	if e.handleSelectionMove(ev) {
		return false
	}
	key := keyStringForMap(ev, e.keymap.insert)
	if key != "" {
		if action, ok := e.keymap.insert[key]; ok {
			return e.execAction(action)
		}
	}
	if ev.Key() == KeyRune {
		e.clearSelection()
		e.insertRune(ev.Rune())
	}
	return false
}
func (e *Editor) handleSelectionMove(ev EventKey) bool {
	if ev.Modifiers()&ModShift == 0 {
		return false
	}
	// Don't handle if Alt is pressed - let keymap handle alt+shift combinations
	if ev.Modifiers()&ModAlt != 0 {
		return false
	}
	switch ev.Key() {
	case KeyLeft:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveWordLeft)
		} else {
			e.extendSelection(e.moveLeft)
		}
		return true
	case KeyRight:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveWordRight)
		} else {
			e.extendSelection(e.moveRight)
		}
		return true
	case KeyUp:
		e.extendSelection(e.moveUp)
		return true
	case KeyDown:
		e.extendSelection(e.moveDown)
		return true
	case KeyPgUp:
		e.extendSelection(e.pageUp)
		return true
	case KeyPgDn:
		e.extendSelection(e.pageDown)
		return true
	case KeyHome:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveFileStart)
			return true
		}
		e.extendSelection(e.moveLineStart)
		return true
	case KeyEnd:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveFileEnd)
			return true
		}
		e.extendSelection(e.moveLineEnd)
		return true
	}
	return false
}

// ConsumeSidebarOpenFile consumes the file path selected from sidebar.
func (e *Editor) ConsumeSidebarOpenFile() (string, bool) {
	if e.sidebarOpenFilePath == "" {
		return "", false
	}
	path := e.sidebarOpenFilePath
	e.sidebarOpenFilePath = ""
	return path, true
}

// setPendingFindChar sets up pending char find (f/F/t/T)
func (e *Editor) setPendingFindChar(action string) {
	e.pendingAction = action
}

// handlePendingChar processes char input for pending action
func (e *Editor) handlePendingChar(ch rune) bool {
	action := e.pendingAction
	e.pendingAction = ""

	// For f, F, t, T - Helix style: anchor moves to old cursor, selection covers jump
	isSelectingAction := action == actionFindChar || action == actionFindCharBackward ||
		action == actionTillChar || action == actionTillCharBackward

	anchor := e.cursor

	var result bool
	switch action {
	case actionFindChar:
		e.lastFindChar = ch
		e.lastFindForward = true
		e.lastFindTill = false
		result = e.findCharForward(ch, false)
	case actionFindCharBackward:
		e.lastFindChar = ch
		e.lastFindForward = false
		e.lastFindTill = false
		result = e.findCharBackward(ch, false)
	case actionTillChar:
		e.lastFindChar = ch
		e.lastFindForward = true
		e.lastFindTill = true
		result = e.findCharForward(ch, true)
	case actionTillCharBackward:
		e.lastFindChar = ch
		e.lastFindForward = false
		e.lastFindTill = true
		result = e.findCharBackward(ch, true)
	case actionReplaceChar:
		return e.replaceCharAtCursor(ch)
	default:
		return false
	}

	// Set selection from anchor to new cursor position (inclusive of cursor char)
	if isSelectingAction && anchor != e.cursor {
		e.selectionActive = true
		e.selectionStart = anchor
		// Selection end is exclusive, so add 1 to include the character at cursor
		e.selectionEnd = Cursor{Row: e.cursor.Row, Col: e.cursor.Col + 1}
		e.selectMode = true
	}

	return result
}
func keyString(ev EventKey) string {
	if ev.Key() == KeyUnknown {
		return ""
	}
	// Handle function keys with optional modifiers.
	if ev.Key() >= KeyF1 && ev.Key() <= KeyF12 {
		keyNum := int(ev.Key()-KeyF1) + 1
		base := fmt.Sprintf("f%d", keyNum)
		var parts []string
		if ev.Modifiers()&ModMeta != 0 {
			parts = append(parts, "cmd")
		}
		if ev.Modifiers()&ModCtrl != 0 {
			parts = append(parts, "ctrl")
		}
		if ev.Modifiers()&ModShift != 0 {
			parts = append(parts, "shift")
		}
		if ev.Modifiers()&ModAlt != 0 {
			parts = append(parts, "alt")
		}
		if len(parts) == 0 {
			return base
		}
		parts = append(parts, base)
		return strings.Join(parts, "+")
	}
	// Handle alt+shift+arrow combinations first
	if ev.Modifiers()&ModAlt != 0 && ev.Modifiers()&ModShift != 0 {
		switch ev.Key() {
		case KeyUp:
			return "alt+shift+up"
		case KeyDown:
			return "alt+shift+down"
		case KeyLeft:
			return "alt+shift+left"
		case KeyRight:
			return "alt+shift+right"
		}
	}
	// Handle alt+arrow combinations
	if ev.Modifiers()&ModAlt != 0 {
		switch ev.Key() {
		case KeyUp:
			return "alt+up"
		case KeyDown:
			return "alt+down"
		case KeyLeft:
			return "alt+left"
		case KeyRight:
			return "alt+right"
		}
		if ev.Key() == KeyRune {
			r := ev.Rune()
			if ev.Modifiers()&ModShift != 0 {
				if r == ' ' {
					return "alt+shift+space"
				}
				return "alt+shift+" + strings.ToLower(string(r))
			}
			if r == ' ' {
				return "alt+space"
			}
			return "alt+" + strings.ToLower(string(r))
		}
	}
	if ev.Modifiers()&ModCtrl != 0 {
		switch ev.Key() {
		case KeyHome:
			return "ctrl+home"
		case KeyEnd:
			return "ctrl+end"
		}
	}
	if ev.Modifiers()&ModMeta != 0 {
		if ev.Key() == KeyRune {
			r := ev.Rune()
			if ev.Modifiers()&ModShift != 0 {
				if r == ' ' {
					return "cmd+shift+space"
				}
				return "cmd+shift+" + strings.ToLower(string(r))
			}
			if r == ' ' {
				return "cmd+space"
			}
			return "cmd+" + strings.ToLower(string(r))
		}
		switch ev.Key() {
		case KeyBackspace, KeyBackspace2:
			return "cmd+backspace"
		case KeyDelete:
			return "cmd+del"
		case KeyEnter:
			return "cmd+enter"
		case KeyLeft:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+left"
			}
			return "cmd+left"
		case KeyRight:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+right"
			}
			return "cmd+right"
		case KeyUp:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+up"
			}
			return "cmd+up"
		case KeyDown:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+down"
			}
			return "cmd+down"
		case KeyHome:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+home"
			}
			return "cmd+home"
		case KeyEnd:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+end"
			}
			return "cmd+end"
		}
	}
	if ev.Key() == KeyRune {
		r := ev.Rune()
		if r == ' ' {
			return "space"
		}
		return string(r)
	}
	// Check Tab before ctrlKeyName since KeyTab == KeyCtrlI (0x09)
	switch ev.Key() {
	case KeyTab:
		if ev.Modifiers()&ModShift != 0 {
			return "shift+tab"
		}
		return "tab"
	case KeyBacktab:
		return "shift+tab"
	}
	if name := ctrlKeyName(ev.Key()); name != "" {
		return name
	}
	switch ev.Key() {
	case KeyUp:
		return "up"
	case KeyDown:
		return "down"
	case KeyLeft:
		return "left"
	case KeyRight:
		return "right"
	case KeyPgUp:
		return "pgup"
	case KeyPgDn:
		return "pgdn"
	case KeyHome:
		return "home"
	case KeyEnd:
		return "end"
	case KeyBackspace, KeyBackspace2:
		return "backspace"
	case KeyEnter:
		if ev.Modifiers()&ModShift != 0 {
			return "shift+enter"
		}
		return "enter"
	case KeyDelete:
		return "del"
	case KeyEscape:
		return "esc"
	}
	return ""
}
func keyStringDisplay(ev EventKey) string {
	var parts []string

	// Build modifier prefix in order: CMD, CTRL, SHIFT, ALT
	if ev.Modifiers()&ModMeta != 0 {
		parts = append(parts, "CMD")
	}
	if ev.Modifiers()&ModCtrl != 0 {
		parts = append(parts, "CTRL")
	}
	if ev.Modifiers()&ModShift != 0 {
		parts = append(parts, "SHIFT")
	}
	if ev.Modifiers()&ModAlt != 0 {
		parts = append(parts, "ALT")
	}

	// Get key name
	var keyName string
	if ev.Key() == KeyRune {
		r := ev.Rune()
		if r == ' ' {
			keyName = "SPACE"
		} else {
			keyName = strings.ToUpper(string(r))
		}
	} else {
		switch ev.Key() {
		case KeyUnknown:
			keyName = ""
		case KeyUp:
			keyName = "UP"
		case KeyDown:
			keyName = "DOWN"
		case KeyLeft:
			keyName = "LEFT"
		case KeyRight:
			keyName = "RIGHT"
		case KeyPgUp:
			keyName = "PGUP"
		case KeyPgDn:
			keyName = "PGDN"
		case KeyHome:
			keyName = "HOME"
		case KeyEnd:
			keyName = "END"
		case KeyBackspace, KeyBackspace2:
			keyName = "BKSP"
		case KeyEnter:
			keyName = "ENTER"
		case KeyDelete:
			keyName = "DEL"
		case KeyEscape:
			keyName = "ESC"
		case KeyTab:
			keyName = "TAB"
		case KeyCtrlA:
			keyName = "A"
		case KeyCtrlB:
			keyName = "B"
		case KeyCtrlC:
			keyName = "C"
		case KeyCtrlD:
			keyName = "D"
		case KeyCtrlE:
			keyName = "E"
		case KeyCtrlF:
			keyName = "F"
		case KeyCtrlG:
			keyName = "G"
		case KeyCtrlH:
			keyName = "H"
		case KeyCtrlI:
			keyName = "I"
		case KeyCtrlJ:
			keyName = "J"
		case KeyCtrlK:
			keyName = "K"
		case KeyCtrlL:
			keyName = "L"
		case KeyCtrlM:
			keyName = "M"
		case KeyCtrlN:
			keyName = "N"
		case KeyCtrlO:
			keyName = "O"
		case KeyCtrlP:
			keyName = "P"
		case KeyCtrlQ:
			keyName = "Q"
		case KeyCtrlR:
			keyName = "R"
		case KeyCtrlS:
			keyName = "S"
		case KeyCtrlT:
			keyName = "T"
		case KeyCtrlU:
			keyName = "U"
		case KeyCtrlV:
			keyName = "V"
		case KeyCtrlW:
			keyName = "W"
		case KeyCtrlX:
			keyName = "X"
		case KeyCtrlY:
			keyName = "Y"
		case KeyCtrlZ:
			keyName = "Z"
		case KeyF1:
			keyName = "F1"
		case KeyF2:
			keyName = "F2"
		case KeyF3:
			keyName = "F3"
		case KeyF4:
			keyName = "F4"
		case KeyF5:
			keyName = "F5"
		case KeyF6:
			keyName = "F6"
		case KeyF7:
			keyName = "F7"
		case KeyF8:
			keyName = "F8"
		case KeyF9:
			keyName = "F9"
		case KeyF10:
			keyName = "F10"
		case KeyF11:
			keyName = "F11"
		case KeyF12:
			keyName = "F12"
		default:
			keyName = fmt.Sprintf("KEY%d", ev.Key())
		}
	}

	if keyName != "" {
		parts = append(parts, keyName)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "-")
}
func keyStringForMap(ev EventKey, keymap map[string]string) string {
	if ev.Modifiers()&ModMeta != 0 {
		switch ev.Key() {
		case KeyHome:
			if _, ok := keymap["cmd+left"]; ok {
				return "cmd+left"
			}
		case KeyEnd:
			if _, ok := keymap["cmd+right"]; ok {
				return "cmd+right"
			}
		}
	}
	return keyString(ev)
}
func ctrlKeyName(key Key) string {
	switch key {
	case KeyCtrlA:
		return "ctrl+a"
	case KeyCtrlB:
		return "ctrl+b"
	case KeyCtrlC:
		return "ctrl+c"
	case KeyCtrlD:
		return "ctrl+d"
	case KeyCtrlE:
		return "ctrl+e"
	case KeyCtrlF:
		return "ctrl+f"
	case KeyCtrlG:
		return "ctrl+g"
	case KeyCtrlH:
		return "ctrl+h"
	case KeyCtrlI:
		return "ctrl+i"
	case KeyCtrlJ:
		return "ctrl+j"
	case KeyCtrlK:
		return "ctrl+k"
	case KeyCtrlL:
		return "ctrl+l"
	case KeyCtrlM:
		return "ctrl+m"
	case KeyCtrlN:
		return "ctrl+n"
	case KeyCtrlO:
		return "ctrl+o"
	case KeyCtrlP:
		return "ctrl+p"
	case KeyCtrlQ:
		return "ctrl+q"
	case KeyCtrlR:
		return "ctrl+r"
	case KeyCtrlS:
		return "ctrl+s"
	case KeyCtrlT:
		return "ctrl+t"
	case KeyCtrlU:
		return "ctrl+u"
	case KeyCtrlV:
		return "ctrl+v"
	case KeyCtrlW:
		return "ctrl+w"
	case KeyCtrlX:
		return "ctrl+x"
	case KeyCtrlY:
		return "ctrl+y"
	case KeyCtrlZ:
		return "ctrl+z"
	}
	return ""
}
