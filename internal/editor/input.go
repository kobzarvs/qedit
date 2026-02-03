package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kobzarvs/qedit/internal/logger"
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

	// Handle AI panel if focused
	if e.aiPanel != nil && e.aiPanel.Visible && e.aiPanel.Focused {
		return e.handleAIPanelKey(ev)
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
		actionAIPanel,
		actionToggleSidebarFocus,
		actionToggleAIPanelFocus,
		actionFocusEditor,
		actionFocusPrevPane,
		actionFocusNextPane,
		actionFocusSidebar,
		actionFocusAIPanel,
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

	if ev.Buttons() == WheelUp || ev.Buttons() == WheelDown {
		x, y := ev.Position()
		if e.scrollAIPanelIfHovered(ev.Buttons(), x, y) {
			return
		}
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

func (e *Editor) scrollAIPanelIfHovered(buttons MouseButton, x, y int) bool {
	if e.aiPanel == nil || !e.aiPanel.Visible {
		return false
	}
	if e.viewWidth <= 0 || e.viewHeight <= 0 {
		return false
	}
	if y < 0 || y >= e.viewHeight {
		return false
	}

	aiPanelX, ok := e.aiPanelX()
	if !ok {
		return false
	}
	if x < aiPanelX {
		return false
	}

	switch buttons {
	case WheelUp:
		e.aiPanel.ScrollUp(1)
		return true
	case WheelDown:
		e.aiPanel.ScrollDown(1)
		return true
	}

	return false
}

func (e *Editor) aiPanelX() (int, bool) {
	w := e.viewWidth
	if w <= 0 {
		return 0, false
	}
	if e.aiPanel != nil && e.aiPanel.Visible && e.aiPanel.Maximized {
		return 0, true
	}

	sidebarWidth := 0
	if e.sidebar != nil && e.sidebar.Visible {
		sidebarWidth = e.sidebar.CalculateWidth(w)
	} else if e.refsPickerActive && len(e.refsPickerItems) > 0 {
		sidebarWidth = w / 4
		if sidebarWidth < 20 {
			sidebarWidth = 20
		}
		if sidebarWidth > w/2 {
			sidebarWidth = w / 2
		}
	}

	aiPanelWidth := e.aiPanel.Width
	if aiPanelWidth < 30 {
		aiPanelWidth = 30
	}
	remaining := w - sidebarWidth
	if remaining <= 0 {
		return 0, false
	}
	maxAIPanelWidth := remaining / 2
	if aiPanelWidth > maxAIPanelWidth {
		aiPanelWidth = maxAIPanelWidth
	}
	if aiPanelWidth <= 0 {
		return 0, false
	}

	return w - aiPanelWidth, true
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
	if e.aiPanel != nil && e.aiPanel.Visible {
		aiX, ok := e.aiPanelX()
		if ok && x >= aiX {
			e.aiPanel.Focused = true
			if e.sidebar != nil {
				e.sidebar.Focused = false
			}
			if e.aiPanel.Maximized {
				return
			}
			return
		}
	}
	if e.aiPanel != nil {
		e.aiPanel.Focused = false
	}
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

	// For references/implementations or multiple results, show picker
	if method == "references" || method == "implementation" || len(locations) > 1 {
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
		e.setStatus("LSP: " + loc.Path + ":" + strconv.Itoa(loc.StartLine+1) + " (cross-file)")
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
func (e *Editor) handleCommand(ev EventKey) bool {
	switch ev.Key() {
	case KeyEscape:
		e.closeAutoComplete()
		e.mode = ModeNormal
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	case KeyCtrlC:
		e.closeAutoComplete()
		e.mode = ModeNormal
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	case KeyTab:
		prefix := string(e.cmd)
		if !e.cmdAutoCompleteActive {
			e.cmdAutoCompleteItems = filterCommands(prefix)
			if len(e.cmdAutoCompleteItems) == 0 {
				return false
			}
			e.cmdAutoCompleteActive = true
			e.cmdAutoCompleteIndex = -1
			e.cmdAutoCompleteCols = 1 // Will be recalculated on render
		} else {
			// Tab moves to next item (down in column, then next column top)
			if e.cmdAutoCompleteIndex < 0 {
				e.cmdAutoCompleteIndex = 0
			} else {
				e.cmdAutoCompleteIndex++
				if e.cmdAutoCompleteIndex >= len(e.cmdAutoCompleteItems) {
					e.cmdAutoCompleteIndex = 0
				}
			}
		}
		e.updateCmdFromAutocomplete()
		return false
	case KeyBacktab:
		if e.cmdAutoCompleteActive {
			// Shift+Tab moves to previous item (up in column, then prev column bottom)
			if e.cmdAutoCompleteIndex < 0 {
				e.cmdAutoCompleteIndex = len(e.cmdAutoCompleteItems) - 1
			} else {
				e.cmdAutoCompleteIndex--
				if e.cmdAutoCompleteIndex < 0 {
					e.cmdAutoCompleteIndex = len(e.cmdAutoCompleteItems) - 1
				}
			}
			e.updateCmdFromAutocomplete()
		}
		return false
	case KeyEnter:
		e.closeAutoComplete()
		cmd := strings.TrimSpace(string(e.cmd))
		e.mode = ModeNormal
		// Add to history if not empty and different from last
		if cmd != "" && (len(e.cmdHistory) == 0 || e.cmdHistory[len(e.cmdHistory)-1] != cmd) {
			e.cmdHistory = append(e.cmdHistory, cmd)
			e.saveCmdHistory()
		}
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return e.execCommand(cmd)
	case KeyBackspace, KeyBackspace2:
		e.closeAutoComplete()
		if e.cmdCursor > 0 && len(e.cmd) > 0 {
			// Delete char before cursor
			e.cmd = append(e.cmd[:e.cmdCursor-1], e.cmd[e.cmdCursor:]...)
			e.cmdCursor--
			e.cmdHistoryIndex = -1
		}
		return false
	case KeyDelete:
		e.closeAutoComplete()
		if e.cmdCursor < len(e.cmd) {
			// Delete char at cursor
			e.cmd = append(e.cmd[:e.cmdCursor], e.cmd[e.cmdCursor+1:]...)
			e.cmdHistoryIndex = -1
		}
		return false
	case KeyLeft, KeyCtrlB: // Ctrl+B = back (readline)
		if e.cmdAutoCompleteActive {
			e.cmdAutoCompleteLeft()
			e.updateCmdFromAutocomplete()
			return false
		}
		if e.cmdCursor > 0 {
			e.cmdCursor--
		}
		return false
	case KeyRight, KeyCtrlF: // Ctrl+F = forward (readline)
		if e.cmdAutoCompleteActive {
			e.cmdAutoCompleteRight()
			e.updateCmdFromAutocomplete()
			return false
		}
		if e.cmdCursor < len(e.cmd) {
			e.cmdCursor++
		}
		return false
	case KeyHome, KeyCtrlA: // Ctrl+A = beginning of line
		e.cmdCursor = 0
		return false
	case KeyEnd, KeyCtrlE: // Ctrl+E = end of line
		e.cmdCursor = len(e.cmd)
		return false
	case KeyUp, KeyCtrlP: // Ctrl+P = previous
		if e.cmdAutoCompleteActive {
			e.cmdAutoCompleteUp()
			e.updateCmdFromAutocomplete()
			return false
		}
		e.cmdHistoryUp()
		return false
	case KeyDown, KeyCtrlN: // Ctrl+N = next
		if e.cmdAutoCompleteActive {
			e.cmdAutoCompleteDown()
			e.updateCmdFromAutocomplete()
			return false
		}
		e.cmdHistoryDown()
		return false
	case KeyCtrlU: // Ctrl+U = clear line
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	case KeyCtrlK: // Ctrl+K = kill to end of line
		e.cmd = e.cmd[:e.cmdCursor]
		e.cmdHistoryIndex = -1
		return false
	case KeyCtrlW: // Ctrl+W = delete word backward
		if e.cmdCursor > 0 {
			// Find start of previous word
			i := e.cmdCursor - 1
			for i > 0 && e.cmd[i-1] == ' ' {
				i--
			}
			for i > 0 && e.cmd[i-1] != ' ' {
				i--
			}
			e.cmd = append(e.cmd[:i], e.cmd[e.cmdCursor:]...)
			e.cmdCursor = i
			e.cmdHistoryIndex = -1
		}
		return false
	case KeyRune:
		e.closeAutoComplete()
		// Insert char at cursor position
		e.cmd = append(e.cmd[:e.cmdCursor], append([]rune{ev.Rune()}, e.cmd[e.cmdCursor:]...)...)
		e.cmdCursor++
		e.cmdHistoryIndex = -1
		return false
	}
	return false
}

// cmdHistoryUp navigates to older command in history
func (e *Editor) cmdHistoryUp() {
	if len(e.cmdHistory) == 0 {
		return
	}

	// First time pressing up - save current prefix for filtering
	if e.cmdHistoryIndex == -1 {
		e.cmdHistoryPrefix = string(e.cmd)
		e.cmdHistoryIndex = len(e.cmdHistory)
	}

	// Find previous matching command
	for i := e.cmdHistoryIndex - 1; i >= 0; i-- {
		if strings.HasPrefix(e.cmdHistory[i], e.cmdHistoryPrefix) {
			e.cmdHistoryIndex = i
			e.cmd = []rune(e.cmdHistory[i])
			e.cmdCursor = len(e.cmd)
			return
		}
	}
}

// cmdHistoryDown navigates to newer command in history
func (e *Editor) cmdHistoryDown() {
	if e.cmdHistoryIndex == -1 {
		return
	}

	// Find next matching command
	for i := e.cmdHistoryIndex + 1; i < len(e.cmdHistory); i++ {
		if strings.HasPrefix(e.cmdHistory[i], e.cmdHistoryPrefix) {
			e.cmdHistoryIndex = i
			e.cmd = []rune(e.cmdHistory[i])
			e.cmdCursor = len(e.cmd)
			return
		}
	}

	// No more matches - restore original prefix
	e.cmdHistoryIndex = -1
	e.cmd = []rune(e.cmdHistoryPrefix)
	e.cmdCursor = len(e.cmd)
}

// LoadCmdHistory loads command history from file
func (e *Editor) LoadCmdHistory() {
	path := e.cmdHistoryPath
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return // File doesn't exist yet, that's ok
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line != "" {
			e.cmdHistory = append(e.cmdHistory, line)
		}
	}
}

// saveCmdHistory saves command history to file
func (e *Editor) saveCmdHistory() {
	path := e.cmdHistoryPath
	if path == "" {
		return
	}
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	// Keep only last 1000 commands
	history := e.cmdHistory
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}
	data := strings.Join(history, "\n")
	_ = os.WriteFile(path, []byte(data), 0644)
}

// filterCommands returns commands matching the given prefix
func filterCommands(prefix string) []CommandInfo {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return AvailableCommands
	}
	var result []CommandInfo
	for _, cmd := range AvailableCommands {
		if strings.HasPrefix(cmd.Name, prefix) {
			result = append(result, cmd)
		}
	}
	return result
}

// closeAutoComplete closes the command autocomplete popup
func (e *Editor) closeAutoComplete() {
	e.cmdAutoCompleteActive = false
	e.cmdAutoCompleteItems = nil
	e.cmdAutoCompleteIndex = -1
	e.cmdAutoCompleteCols = 0
	e.cmdAutoCompleteColGroups = nil
}

// cmdAutoCompleteUp moves selection up (previous command)
func (e *Editor) cmdAutoCompleteUp() {
	n := len(e.cmdAutoCompleteItems)
	if n == 0 {
		return
	}
	if e.cmdAutoCompleteIndex < 0 {
		e.cmdAutoCompleteIndex = n - 1
		return
	}
	e.cmdAutoCompleteIndex--
	if e.cmdAutoCompleteIndex < 0 {
		e.cmdAutoCompleteIndex = n - 1
	}
}

// cmdAutoCompleteDown moves selection down (next command)
func (e *Editor) cmdAutoCompleteDown() {
	n := len(e.cmdAutoCompleteItems)
	if n == 0 {
		return
	}
	if e.cmdAutoCompleteIndex < 0 {
		e.cmdAutoCompleteIndex = 0
		return
	}
	e.cmdAutoCompleteIndex++
	if e.cmdAutoCompleteIndex >= n {
		e.cmdAutoCompleteIndex = 0
	}
}

// findCmdPosition finds the column and position within column for a command index
func (e *Editor) findCmdPosition(cmdIdx int) (col int, posInCol int) {
	if len(e.cmdAutoCompleteColGroups) == 0 {
		return 0, cmdIdx
	}

	globalIdx := 0
	for colIdx, colGrps := range e.cmdAutoCompleteColGroups {
		for _, grp := range colGrps {
			for range grp.Commands {
				if globalIdx == cmdIdx {
					return colIdx, posInCol
				}
				globalIdx++
				posInCol++
			}
		}
		posInCol = 0
	}
	return 0, 0
}

// countCmdsInCol counts commands in a column
func (e *Editor) countCmdsInCol(colIdx int) int {
	if colIdx < 0 || colIdx >= len(e.cmdAutoCompleteColGroups) {
		return 0
	}
	count := 0
	for _, grp := range e.cmdAutoCompleteColGroups[colIdx] {
		count += len(grp.Commands)
	}
	return count
}

// firstCmdInCol returns the global command index of the first command in a column
func (e *Editor) firstCmdInCol(colIdx int) int {
	if colIdx < 0 || colIdx >= len(e.cmdAutoCompleteColGroups) {
		return 0
	}
	idx := 0
	for c := 0; c < colIdx; c++ {
		idx += e.countCmdsInCol(c)
	}
	return idx
}

// cmdAutoCompleteLeft moves selection to the previous column (same relative position)
func (e *Editor) cmdAutoCompleteLeft() {
	n := len(e.cmdAutoCompleteItems)
	if n == 0 {
		return
	}
	if e.cmdAutoCompleteIndex < 0 {
		e.cmdAutoCompleteIndex = 0
		return
	}
	cols := len(e.cmdAutoCompleteColGroups)
	if cols <= 1 {
		return
	}

	// Find current position
	currentCol, posInCol := e.findCmdPosition(e.cmdAutoCompleteIndex)

	// Move to previous column
	targetCol := currentCol - 1
	if targetCol < 0 {
		targetCol = cols - 1
	}

	// Find position in target column
	targetColCmds := e.countCmdsInCol(targetCol)
	if targetColCmds == 0 {
		return
	}

	// Clamp position to target column size
	targetPos := posInCol
	if targetPos >= targetColCmds {
		targetPos = targetColCmds - 1
	}

	e.cmdAutoCompleteIndex = e.firstCmdInCol(targetCol) + targetPos
}

// cmdAutoCompleteRight moves selection to the next column (same relative position)
func (e *Editor) cmdAutoCompleteRight() {
	n := len(e.cmdAutoCompleteItems)
	if n == 0 {
		return
	}
	if e.cmdAutoCompleteIndex < 0 {
		e.cmdAutoCompleteIndex = 0
		return
	}
	cols := len(e.cmdAutoCompleteColGroups)
	if cols <= 1 {
		return
	}

	// Find current position
	currentCol, posInCol := e.findCmdPosition(e.cmdAutoCompleteIndex)

	// Move to next column
	targetCol := currentCol + 1
	if targetCol >= cols {
		targetCol = 0
	}

	// Find position in target column
	targetColCmds := e.countCmdsInCol(targetCol)
	if targetColCmds == 0 {
		return
	}

	// Clamp position to target column size
	targetPos := posInCol
	if targetPos >= targetColCmds {
		targetPos = targetColCmds - 1
	}

	e.cmdAutoCompleteIndex = e.firstCmdInCol(targetCol) + targetPos
}

// updateCmdFromAutocomplete updates the command line with the selected autocomplete item
func (e *Editor) updateCmdFromAutocomplete() {
	if len(e.cmdAutoCompleteItems) == 0 || e.cmdAutoCompleteIndex < 0 || e.cmdAutoCompleteIndex >= len(e.cmdAutoCompleteItems) {
		return
	}
	selected := e.cmdAutoCompleteItems[e.cmdAutoCompleteIndex]
	e.cmd = []rune(commandInsertText(selected.Name))
	e.cmdCursor = len(e.cmd)
}

func commandInsertText(name string) string {
	idx := strings.Index(name, "<")
	if idx == -1 {
		return name
	}
	base := strings.TrimRight(name[:idx], " ")
	return base + " "
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
func (e *Editor) handleBranchPicker(ev EventKey) bool {
	switch keyString(ev) {
	case "esc", "ctrl+c":
		e.closeBranchPicker("")
		return false
	case "enter":
		if len(e.branchPickerItems) == 0 {
			e.closeBranchPicker("")
			return false
		}
		selection := e.branchPickerItems[e.branchPickerIndex]
		e.closeBranchPicker(selection)
		return false
	case "up", "k":
		e.branchPickerIndex--
	case "down", "j":
		e.branchPickerIndex++
	case "pgup":
		e.branchPickerIndex -= e.branchPickerPageSize()
	case "pgdn":
		e.branchPickerIndex += e.branchPickerPageSize()
	case "home":
		e.branchPickerIndex = 0
	case "end":
		e.branchPickerIndex = len(e.branchPickerItems) - 1
	default:
		return false
	}
	if e.branchPickerIndex < 0 {
		e.branchPickerIndex = 0
	}
	if e.branchPickerIndex >= len(e.branchPickerItems) {
		e.branchPickerIndex = len(e.branchPickerItems) - 1
		if e.branchPickerIndex < 0 {
			e.branchPickerIndex = 0
		}
	}
	return false
}
func (e *Editor) handleRefsPicker(ev EventKey) bool {
	switch keyString(ev) {
	case "esc", "ctrl+c", "q":
		e.closeRefsPicker(false)
		return true
	case "enter":
		e.closeRefsPicker(true)
		return true
	case "up", "k":
		e.refsPickerIndex--
	case "down", "j":
		e.refsPickerIndex++
	case "pgup", "ctrl+u":
		e.refsPickerIndex -= e.refsPickerPageSize()
	case "pgdn", "ctrl+d":
		e.refsPickerIndex += e.refsPickerPageSize()
	case "home", "g":
		e.refsPickerIndex = 0
	case "end", "G":
		e.refsPickerIndex = len(e.refsPickerItems) - 1
	default:
		return false // Not handled, let normal key handling proceed
	}
	// Clamp index
	if e.refsPickerIndex < 0 {
		e.refsPickerIndex = 0
	}
	if e.refsPickerIndex >= len(e.refsPickerItems) {
		e.refsPickerIndex = len(e.refsPickerItems) - 1
		if e.refsPickerIndex < 0 {
			e.refsPickerIndex = 0
		}
	}
	// Move cursor to selected reference (if same file)
	e.jumpToSelectedRef()
	return true // Key was handled
}

// jumpToSelectedRef moves cursor to the currently selected reference
func (e *Editor) jumpToSelectedRef() {
	if e.refsPickerIndex >= len(e.refsPickerItems) {
		return
	}
	loc := e.refsPickerItems[e.refsPickerIndex]
	currentAbs, _ := filepath.Abs(e.filename)
	if loc.Path == currentAbs || loc.Path == e.filename {
		e.cursor.Row = loc.StartLine
		e.cursor.Col = loc.StartCol
		e.ensureCursorVisible(e.viewHeightCached())
	}
}

// handleSidebarKey handles key events when sidebar is focused
func (e *Editor) handleSidebarKey(ev EventKey) bool {
	if e.sidebar == nil || !e.sidebar.Visible {
		logger.Debug("handleSidebarKey: sidebar nil or not visible")
		return false
	}

	if ev.Key() == KeyRune && ev.Rune() == ':' {
		e.sidebar.Focused = false
		e.mode = ModeCommand
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	}

	viewHeight := e.viewHeight - 1 // subtract header
	var fileTreeContent *SidebarFileTreeContent
	if content, ok := e.sidebar.Content.(*SidebarFileTreeContent); ok {
		fileTreeContent = content
	}
	action := e.sidebar.HandleKey(ev, viewHeight)
	if fileTreeContent != nil {
		e.updateFileTreePreview(fileTreeContent, action)
		e.updateFileTreeStatus(fileTreeContent, action)
		e.fileTreeShowHidden = fileTreeContent.ShowHidden()
		e.fileTreeShowIgnored = fileTreeContent.ShowIgnored()
		switch ev.Key() {
		case KeyF2:
			e.Notify(fileTreeContent.filterModeLabel())
		case KeyF3:
			if fileTreeContent.PreviewEnabled() {
				e.Notify("Files: preview on")
			} else {
				e.Notify("Files: preview off")
			}
		}
	}
	logger.Debug("handleSidebarKey", "action", action.Action, "branch", action.Branch, "mode", action.Mode, "provider", action.Provider, "model", action.Model)

	switch action.Action {
	case SidebarActionClose:
		logger.Debug("sidebar action: close")
		e.closeSidebar()
		return false

	case SidebarActionBackToMenu:
		logger.Debug("sidebar action: back to menu")
		if e.sidebar.MenuContent != nil {
			e.sidebar.MenuContent.SetAvailability(e.isGitRepo(), e.aiManager != nil)
			e.sidebar.SetContent(e.sidebar.MenuContent)
		}
		return false

	case SidebarActionFocusEditor:
		logger.Debug("sidebar action: focus editor")
		e.sidebar.Focused = false
		return false

	case SidebarActionSwitchMode:
		logger.Debug("sidebar action: switch mode", "mode", action.Mode)
		e.switchSidebarMode(action.Mode)
		return false

	case SidebarActionCheckoutBranch:
		logger.Debug("sidebar action: checkout branch", "branch", action.Branch)
		if action.Branch != "" {
			// Signal that we want to checkout this branch
			e.branchPickerRequested = false
			e.branchPickerSelection = action.Branch
			if e.sidebar.CloseOnSelect {
				e.closeSidebar()
			} else {
				e.sidebar.Focused = false
			}
		}
		return false

	case SidebarActionOpenFile:
		logger.Debug("sidebar action: open file", "path", action.Path)
		if action.Path != "" {
			if fileTreeContent != nil && isBinaryFile(action.Path) {
				e.Notify("Files: binary preview only")
				return false
			}
			e.sidebarOpenFilePath = action.Path
			e.clearFileTreePreview()
			if e.sidebar.CloseOnSelect {
				e.closeSidebar()
			} else {
				e.sidebar.Focused = false
			}
		}
		return false

	case SidebarActionOpenAIModels:
		logger.Debug("sidebar action: open AI models", "provider", action.Provider)
		e.openSidebarAIModels(action.Provider)
		return false

	case SidebarActionSetAIModel:
		logger.Debug("sidebar action: set AI model", "model", action.Model)
		e.selectSidebarAIModel(action.Model)
		return false

	case SidebarActionSwitchWorktree:
		logger.Debug("sidebar action: switch worktree", "path", action.Worktree)
		if action.Worktree != "" {
			e.worktreeSwitchSelection = action.Worktree
			if e.sidebar.CloseOnSelect {
				e.closeSidebar()
			} else {
				e.sidebar.Focused = false
			}
		}
		return false

	case SidebarActionRefresh:
		if action.Mode == SidebarModeWorktrees {
			e.refreshSidebarWorktrees()
		}
		return false
	}

	return false // continue running, don't quit
}

// switchSidebarMode switches sidebar to the specified mode
func (e *Editor) switchSidebarMode(mode SidebarMode) {
	if mode != SidebarModeFileTree {
		e.clearFileTreePreview()
	}
	switch mode {
	case SidebarModeMenu:
		if e.sidebar.MenuContent != nil {
			e.sidebar.SetContent(e.sidebar.MenuContent)
		}

	case SidebarModeBranches:
		e.openSidebarBranches()

	case SidebarModeAI:
		e.openSidebarAIProviders()

	case SidebarModeFileTree:
		e.openSidebarFileTree("")

	case SidebarModeRecentHistory:
		e.setStatus("Recent History: not implemented yet")

	case SidebarModeLocalChanges:
		e.openSidebarGitChanges()

	case SidebarModeWorktrees:
		e.openSidebarWorktrees()
	}
}

// openSidebar opens the sidebar with the menu
func (e *Editor) openSidebar() {
	logger.Debug("openSidebar called")
	if e.sidebar == nil {
		logger.Warn("openSidebar: sidebar is nil")
		return
	}

	// Close refs picker if open (mutual exclusion)
	if e.refsPickerActive {
		logger.Debug("openSidebar: closing refs picker")
		e.closeRefsPicker(false)
	}

	// Initialize menu content if not exists
	if e.sidebar.MenuContent == nil {
		logger.Debug("openSidebar: creating menu content", "gitRepo", e.isGitRepo())
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo(), e.aiManager != nil)
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo(), e.aiManager != nil)
	}

	e.sidebar.Open(e.sidebar.MenuContent)
	logger.Debug("openSidebar: sidebar opened")
}

// openSidebarBranches opens the sidebar directly in branches mode
func (e *Editor) openSidebarBranches() {
	logger.Debug("openSidebarBranches called")
	if e.sidebar == nil {
		logger.Warn("openSidebarBranches: sidebar is nil")
		return
	}
	if !e.isGitRepo() {
		e.setStatus("not a git repository")
		if e.sidebar.MenuContent != nil {
			e.sidebar.MenuContent.SetAvailability(false, e.aiManager != nil)
		}
		return
	}

	// Close refs picker if open (mutual exclusion)
	if e.refsPickerActive {
		logger.Debug("openSidebarBranches: closing refs picker")
		e.closeRefsPicker(false)
	}

	// Initialize menu content for later "back" navigation
	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo(), e.aiManager != nil)
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo(), e.aiManager != nil)
	}

	// Request branches - app will call ShowSidebarBranches
	e.sidebar.Open(NewSidebarLoadingContent("Branches", "Loading..."))
	e.setStatus("loading branches...")
	e.branchPickerRequested = true
	logger.Debug("openSidebarBranches: branch request set")
}

// openSidebarWorktrees opens the sidebar directly in worktrees mode
func (e *Editor) openSidebarWorktrees() {
	logger.Debug("openSidebarWorktrees called")
	if e.sidebar == nil {
		logger.Warn("openSidebarWorktrees: sidebar is nil")
		return
	}
	if !e.isGitRepo() {
		e.setStatus("not a git repository")
		if e.sidebar.MenuContent != nil {
			e.sidebar.MenuContent.SetAvailability(false, e.aiManager != nil)
		}
		return
	}

	// Close refs picker if open (mutual exclusion)
	if e.refsPickerActive {
		logger.Debug("openSidebarWorktrees: closing refs picker")
		e.closeRefsPicker(false)
	}

	// Initialize menu content for later "back" navigation
	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo(), e.aiManager != nil)
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo(), e.aiManager != nil)
	}

	e.sidebar.Open(NewSidebarLoadingContent("Worktree", "Loading..."))
	e.setStatus("loading worktrees...")
	e.worktreeListRequested = true
	logger.Debug("openSidebarWorktrees: list request set")
}

func (e *Editor) refreshSidebarWorktrees() {
	if e.sidebar == nil {
		return
	}
	if !e.isGitRepo() {
		e.setStatus("not a git repository")
		return
	}
	if e.sidebar.Visible && e.sidebar.Content != nil && e.sidebar.Content.Mode() == SidebarModeWorktrees {
		e.worktreeListRequested = true
		e.setStatus("loading worktrees...")
		return
	}
	e.openSidebarWorktrees()
}

func (e *Editor) requestWorktreeRefreshIfActive() {
	if e.sidebar == nil || !e.sidebar.Visible || e.sidebar.Content == nil {
		return
	}
	if e.sidebar.Content.Mode() == SidebarModeWorktrees {
		e.worktreeListRequested = true
	}
}

// openSidebarFileTree opens the sidebar in file tree mode.
func (e *Editor) openSidebarFileTree(path string) {
	logger.Debug("openSidebarFileTree called", "path", path)
	if e.sidebar == nil {
		logger.Warn("openSidebarFileTree: sidebar is nil")
		return
	}

	// Close refs picker if open (mutual exclusion)
	if e.refsPickerActive {
		logger.Debug("openSidebarFileTree: closing refs picker")
		e.closeRefsPicker(false)
	}

	// Initialize menu content for later "back" navigation
	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo(), e.aiManager != nil)
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo(), e.aiManager != nil)
	}

	startDir := strings.TrimSpace(path)
	if startDir == "" {
		if e.filename != "" {
			startDir = filepath.Dir(e.filename)
		} else if cwd, err := os.Getwd(); err == nil {
			startDir = cwd
		}
	}
	if startDir == "" {
		startDir = "."
	}
	if info, err := os.Stat(startDir); err == nil && !info.IsDir() {
		startDir = filepath.Dir(startDir)
	}

	content := NewSidebarFileTreeContent(startDir, e.fileTreeShowHidden, e.fileTreeShowIgnored)
	e.sidebar.Open(content)
}

// openSidebarGitChanges opens the sidebar in git changes mode.
func (e *Editor) openSidebarGitChanges() {
	logger.Debug("openSidebarGitChanges called")
	if e.sidebar == nil {
		logger.Warn("openSidebarGitChanges: sidebar is nil")
		return
	}
	if !e.isGitRepo() {
		e.setStatus("not a git repository")
		if e.sidebar.MenuContent != nil {
			e.sidebar.MenuContent.SetAvailability(false, e.aiManager != nil)
		}
		return
	}

	// Close refs picker if open (mutual exclusion)
	if e.refsPickerActive {
		logger.Debug("openSidebarGitChanges: closing refs picker")
		e.closeRefsPicker(false)
	}

	// Initialize menu content for later "back" navigation
	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo(), e.aiManager != nil)
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo(), e.aiManager != nil)
	}

	if err := e.refreshGitChangesIfStale(2 * time.Second); err != nil {
		logger.Error("openSidebarGitChanges: refresh failed", "error", err)
		e.setStatus(err.Error())
	}

	content := NewSidebarGitChangesContent(e)
	e.sidebar.Open(content)
}

// openSidebarAIProviders opens the sidebar in AI provider mode.
func (e *Editor) openSidebarAIProviders() {
	logger.Debug("openSidebarAIProviders called")
	if e.sidebar == nil {
		logger.Warn("openSidebarAIProviders: sidebar is nil")
		return
	}

	if e.aiManager == nil {
		e.setStatus("AI not configured")
		e.sidebar.Open(NewSidebarLoadingContent("AI", "AI not configured"))
		return
	}

	// Close refs picker if open (mutual exclusion)
	if e.refsPickerActive {
		logger.Debug("openSidebarAIProviders: closing refs picker")
		e.closeRefsPicker(false)
	}

	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo(), e.aiManager != nil)
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo(), e.aiManager != nil)
	}

	providers := e.aiManager.ListProviders()
	content := NewSidebarAIProvidersContent(providers, e.aiManager.ActiveName())
	if len(providers) == 0 {
		e.setStatus("AI: no providers configured")
	}
	e.sidebar.Open(content)
}

// openSidebarAIModels opens the sidebar for models of a provider.
func (e *Editor) openSidebarAIModels(provider string) {
	if e.sidebar == nil {
		return
	}
	if e.aiManager == nil {
		e.setStatus("AI not configured")
		return
	}
	if provider == "" {
		return
	}

	if err := e.aiManager.SetActive(provider); err != nil {
		e.setStatus("AI: " + err.Error())
		return
	}
	e.saveAIState()
	// Sync panel state after switching provider
	e.refreshAIPanelProviderState()

	providerLabel := provider
	if e.aiPanel != nil && e.aiPanel.ProviderName != "" {
		providerLabel = e.aiPanel.ProviderName
	}

	models, err := e.aiManager.ListModels()
	if err != nil {
		e.setStatus("AI: " + err.Error())
		return
	}
	if len(models) == 0 {
		e.setStatus("AI: no models available")
		return
	}

	content := NewSidebarAIModelsContent(provider, providerLabel, models, e.aiManager.CurrentModel())
	e.sidebar.SetContent(content)
	e.sidebar.Visible = true
	e.sidebar.Focused = true
}

func (e *Editor) selectSidebarAIModel(model string) {
	if e.aiManager == nil {
		e.setStatus("AI not configured")
		return
	}
	if model == "" {
		return
	}
	if err := e.aiManager.SetModel(model); err != nil {
		e.setStatus("AI: " + err.Error())
		return
	}
	e.saveAIState()
	e.syncAIPanelProviderState()
	e.setStatus("AI model: " + model)
	if content, ok := e.sidebar.Content.(*SidebarAIModelsContent); ok {
		content.SetCurrentModel(model)
	}
}

// closeSidebar closes the sidebar
func (e *Editor) closeSidebar() {
	logger.Debug("closeSidebar called")
	if e.sidebar == nil {
		return
	}
	e.sidebar.Close()
	e.clearFileTreePreview()
}

// toggleSidebar toggles the sidebar visibility
func (e *Editor) toggleSidebar() {
	logger.Debug("toggleSidebar called", "visible", e.sidebar != nil && e.sidebar.Visible, "focused", e.sidebar != nil && e.sidebar.Focused)
	if e.sidebar == nil {
		logger.Warn("toggleSidebar: sidebar is nil")
		return
	}
	if e.sidebar.Visible {
		e.closeSidebar()
	} else {
		e.openSidebar()
	}
}

func (e *Editor) toggleSidebarFocus() {
	if e.sidebar == nil {
		return
	}
	if !e.sidebar.Visible {
		e.openSidebar()
		return
	}
	e.sidebar.Focused = !e.sidebar.Focused
	if e.sidebar.Focused && e.aiPanel != nil {
		e.aiPanel.Focused = false
	}
	if !e.sidebar.Focused {
		e.clearFileTreePreview()
	}
}

func (e *Editor) exitCommandLine() {
	switch e.mode {
	case ModeCommand:
		e.mode = ModeNormal
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
	case ModeSearch:
		e.mode = ModeNormal
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.searchMatches = nil
		e.searchHistoryIndex = -1
	}
}

func (e *Editor) focusSidebar() {
	if e.sidebar == nil {
		return
	}
	if !e.sidebar.Visible {
		e.openSidebar()
		return
	}
	e.sidebar.Focused = true
	if e.aiPanel != nil {
		e.aiPanel.Focused = false
	}
	e.exitCommandLine()
}

func (e *Editor) toggleAIPanelFocus() {
	if e.aiPanel == nil {
		e.aiPanel = NewAIPanel()
	}
	if !e.aiPanel.Visible {
		e.aiPanel.Open()
		e.syncAIPanelProviderState()
	} else {
		e.aiPanel.Focused = !e.aiPanel.Focused
	}
	if e.aiPanel.Focused && e.sidebar != nil {
		e.sidebar.Focused = false
	}
}

func (e *Editor) focusAIPanel() {
	if e.aiPanel == nil {
		e.aiPanel = NewAIPanel()
	}
	if !e.aiPanel.Visible {
		e.aiPanel.Open()
		e.syncAIPanelProviderState()
	}
	e.aiPanel.Focused = true
	if e.sidebar != nil {
		e.sidebar.Focused = false
	}
	e.clearFileTreePreview()
	e.exitCommandLine()
}

func (e *Editor) focusEditor() {
	if e.sidebar != nil {
		e.sidebar.Focused = false
	}
	if e.aiPanel != nil {
		e.aiPanel.Focused = false
	}
	e.clearFileTreePreview()
	e.exitCommandLine()
}

func (e *Editor) focusCommandLine() {
	if e.sidebar != nil {
		e.sidebar.Focused = false
	}
	if e.aiPanel != nil {
		e.aiPanel.Focused = false
	}
	e.clearFileTreePreview()
	if e.mode == ModeCommand {
		return
	}
	if e.mode == ModeSearch {
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.searchMatches = nil
		e.searchHistoryIndex = -1
	}
	e.mode = ModeCommand
	e.cmd = e.cmd[:0]
	e.cmdCursor = 0
	e.cmdHistoryIndex = -1
}

type focusPane int

const (
	focusPaneSidebar focusPane = iota
	focusPaneEditor
	focusPaneAIPanel
)

func (e *Editor) focusablePanes() []focusPane {
	panes := make([]focusPane, 0, 3)
	if e.sidebar != nil && e.sidebar.Visible {
		panes = append(panes, focusPaneSidebar)
	}
	panes = append(panes, focusPaneEditor)
	if e.aiPanel != nil && e.aiPanel.Visible {
		panes = append(panes, focusPaneAIPanel)
	}
	return panes
}

func (e *Editor) currentPane() focusPane {
	if e.sidebar != nil && e.sidebar.Visible && e.sidebar.Focused {
		return focusPaneSidebar
	}
	if e.aiPanel != nil && e.aiPanel.Visible && e.aiPanel.Focused {
		return focusPaneAIPanel
	}
	return focusPaneEditor
}

func (e *Editor) focusPane(target focusPane) {
	switch target {
	case focusPaneSidebar:
		if e.sidebar == nil || !e.sidebar.Visible {
			return
		}
		e.focusSidebar()
	case focusPaneAIPanel:
		if e.aiPanel == nil || !e.aiPanel.Visible {
			return
		}
		e.focusAIPanel()
	default:
		e.focusEditor()
	}
}

func (e *Editor) focusPrevPane() {
	panes := e.focusablePanes()
	if len(panes) < 2 {
		return
	}
	current := e.currentPane()
	idx := 0
	found := false
	for i, pane := range panes {
		if pane == current {
			idx = i
			found = true
			break
		}
	}
	if !found {
		e.focusPane(panes[0])
		return
	}
	idx--
	if idx < 0 {
		idx = len(panes) - 1
	}
	e.focusPane(panes[idx])
}

func (e *Editor) focusNextPane() {
	panes := e.focusablePanes()
	if len(panes) < 2 {
		return
	}
	current := e.currentPane()
	idx := 0
	found := false
	for i, pane := range panes {
		if pane == current {
			idx = i
			found = true
			break
		}
	}
	if !found {
		e.focusPane(panes[0])
		return
	}
	idx++
	if idx >= len(panes) {
		idx = 0
	}
	e.focusPane(panes[idx])
}

// ShowSidebarBranches shows branches in the sidebar
func (e *Editor) ShowSidebarBranches(branches []string, current string) {
	logger.Debug("ShowSidebarBranches called", "count", len(branches), "current", current)
	if e.sidebar == nil {
		logger.Warn("ShowSidebarBranches: sidebar is nil")
		return
	}

	content := NewSidebarBranchesContent(branches, current)
	e.sidebar.SetContent(content)
	e.sidebar.Visible = true
	e.sidebar.Focused = true
}

// ShowSidebarWorktrees shows worktrees in the sidebar.
func (e *Editor) ShowSidebarWorktrees(worktrees []WorktreeInfo, activePath string) {
	logger.Debug("ShowSidebarWorktrees called", "count", len(worktrees), "active", activePath)
	if e.sidebar == nil {
		logger.Warn("ShowSidebarWorktrees: sidebar is nil")
		return
	}
	if content, ok := e.sidebar.Content.(*SidebarWorktreesContent); ok {
		content.UpdateWorktrees(worktrees, activePath)
		e.sidebar.SetContent(content)
	} else {
		content := NewSidebarWorktreesContent(worktrees, activePath)
		e.sidebar.SetContent(content)
	}
	e.sidebar.Visible = true
	e.sidebar.Focused = true
}

// isGitRepo returns true if current file is in a git repo
func (e *Editor) isGitRepo() bool {
	return e.gitBranch != ""
}

// IsSidebarBranchRequest returns true if sidebar requested branches
func (e *Editor) IsSidebarBranchRequest() bool {
	if e.sidebar == nil {
		return false
	}
	return e.branchPickerRequested
}

// ConsumeWorktreeListRequest consumes the worktree list request.
func (e *Editor) ConsumeWorktreeListRequest() bool {
	if !e.worktreeListRequested {
		return false
	}
	e.worktreeListRequested = false
	return true
}

// ConsumeSidebarBranchSelection consumes the branch selection from sidebar
func (e *Editor) ConsumeSidebarBranchSelection() string {
	if e.branchPickerSelection == "" {
		return ""
	}
	selection := e.branchPickerSelection
	e.branchPickerSelection = ""
	return selection
}

// ConsumeSidebarWorktreeSelection consumes the worktree selection from sidebar/commands.
func (e *Editor) ConsumeSidebarWorktreeSelection() string {
	if e.worktreeSwitchSelection == "" {
		return ""
	}
	selection := e.worktreeSwitchSelection
	e.worktreeSwitchSelection = ""
	return selection
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
