package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func (e *Editor) Render(s Screen) {
	w, h := s.Size()
	if w <= 0 || h <= 0 {
		return
	}

	statusY := h - 2
	cmdY := h - 1
	viewHeight := h - 2
	if h < 2 {
		statusY = h - 1
		cmdY = h - 1
	}
	if viewHeight < 0 {
		viewHeight = 0
	}
	e.viewHeight = viewHeight
	e.viewWidth = w

	// Calculate sidebar width (refs picker or new sidebar, mutually exclusive)
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
	editorX := sidebarWidth
	editorWidth := w - sidebarWidth

	gutterWidth := e.gutterWidth()
	if !e.freeScroll {
		e.ensureCursorVisible(viewHeight)
		e.ensureCursorVisibleHorizontal(editorWidth, gutterWidth)
	}

	s.SetStyle(e.styleMain)
	s.Clear()

	// Draw editor content (offset by sidebar)
	for y := 0; y < viewHeight; y++ {
		lineIdx := e.scroll + y
		if lineIdx >= len(e.lines) {
			clearLineAt(s, editorX, y, editorWidth, e.styleMain)
			continue
		}
		e.drawLineWithGutterAt(s, editorX, y, editorWidth, gutterWidth, lineIdx)
	}

	// Draw sidebar (new sidebar takes priority over refs picker)
	if e.sidebar != nil && e.sidebar.Visible && sidebarWidth > 0 {
		e.sidebar.Render(s, e.sidebarStyles, 0, 0, sidebarWidth, viewHeight)
	} else if e.refsPickerActive && sidebarWidth > 0 {
		e.renderRefsSidebar(s, sidebarWidth, viewHeight)
	}

	// Draw scroll indicator if recently scrolled
	e.renderScrollIndicator(s, w, viewHeight)

	var cx, cy int
	if statusY >= 0 && !e.zoomPendingRestore {
		e.renderStatusline(s, w, statusY)
	}
	if cmdY >= 0 && !e.zoomPendingRestore {
		cmdCursor := e.renderCommandline(s, w, cmdY)
		if e.mode == ModeCommand || e.mode == ModeSearch {
			cx = cmdCursor
			cy = cmdY
		}
		if e.mode == ModeCommand && e.cmdAutoCompleteActive {
			e.renderCommandAutocomplete(s, w, statusY)
		}
	}
	cursorVisible := true
	if e.mode != ModeCommand && e.mode != ModeSearch && e.mode != ModeBranchPicker {
		cy = e.cursor.Row - e.scroll
		if cy < 0 || cy >= viewHeight {
			cursorVisible = false
		}
		if e.cursor.Row >= 0 && e.cursor.Row < len(e.lines) {
			cx = editorX + gutterWidth + visualCol(e.lines[e.cursor.Row], e.cursor.Col, e.tabWidth) - e.scrollX
		}
		if cx < editorX+gutterWidth {
			cx = editorX + gutterWidth
		}
		if cx >= w {
			cx = w - 1
		}
	}

	if e.branchPickerActive {
		e.renderBranchPicker(s, w, viewHeight)
	}
	if e.spaceMenuActive {
		e.renderSpaceMenu(s, w, viewHeight)
	}
	if e.gotoMode {
		e.renderMenu(s, w, viewHeight, "Goto", GotoMenuItems)
	}
	if e.matchMode {
		e.renderMenu(s, w, viewHeight, "Match", MatchMenuItems)
	}
	if e.viewMode {
		e.renderMenu(s, w, viewHeight, "View", ViewMenuItems)
	}
	if e.windowMode {
		e.renderMenu(s, w, viewHeight, "Window", WindowMenuItems)
	}
	if e.keybindingsHelpActive {
		e.renderKeybindingsHelp(s, w, viewHeight)
	}
	sidebarFocused := e.sidebar != nil && e.sidebar.Visible && e.sidebar.Focused
	if e.mode == ModeBranchPicker || e.spaceMenuActive || e.keybindingsHelpActive || sidebarFocused || !cursorVisible {
		s.HideCursor()
		s.Show()
		return
	}
	cursorStyle := CursorStyleSteadyBlock
	if e.mode == ModeInsert || e.mode == ModeSearch || e.mode == ModeCommand {
		cursorStyle = CursorStyleSteadyBar
	}
	s.SetCursorStyle(cursorStyle)
	s.ShowCursor(cx, cy)
	s.Show()
}
func (e *Editor) renderStatusline(s Screen, w, y int) {
	mode := "NORMAL"
	if e.mode == ModeInsert {
		mode = "INSERT"
	} else if e.mode == ModeCommand {
		mode = "COMMAND"
	} else if e.mode == ModeBranchPicker {
		mode = "BRANCHES"
	} else if e.mode == ModeSearch {
		mode = "SEARCH"
	}
	name := e.filename
	if name == "" {
		name = "[No Name]"
	} else {
		// Show relative path from cwd if possible
		if cwd, err := os.Getwd(); err == nil {
			absName := name
			if !filepath.IsAbs(name) {
				absName, _ = filepath.Abs(name)
			}
			if rel, err := filepath.Rel(cwd, absName); err == nil && !strings.HasPrefix(rel, "..") {
				name = rel
			} else {
				name = filepath.Base(name)
			}
		} else {
			name = filepath.Base(name)
		}
	}
	flags := ""
	if e.dirty {
		flags += "[*]"
	}
	if e.externalChange != ExternalChangeNone {
		flags += "[!]"
	}

	status := fmt.Sprintf(" %s | %s %s", mode, name, flags)
	if e.statusMessage != "" {
		status = fmt.Sprintf(" %s | %s %s | %s ", mode, name, flags, e.statusMessage)
	}
	row := e.cursor.Row + 1
	col := 1
	if e.cursor.Row >= 0 && e.cursor.Row < len(e.lines) {
		col = visualCol(e.lines[e.cursor.Row], e.cursor.Col, e.tabWidth) + 1
	}

	// Build right part, tracking branch position for styling
	rightParts := []string{fmt.Sprintf(" Ln %d, Col %d", row, col)}
	branchText := ""
	if e.gitBranch != "" {
		branchText = formatGitBranch(e.gitBranchSymbol, e.gitBranch)
		rightParts = append(rightParts, branchText)
	}
	layoutText := ""
	if e.layoutName != "" {
		layoutText = e.layoutName + " "
		rightParts = append(rightParts, layoutText)
	}
	right := strings.Join(rightParts, " | ")

	line := composeStatusLine(status, right, w)
	lineStr := string(line)

	// Find branch position in the composed line (using rune indices)
	branchStart := -1
	branchEnd := -1
	if branchText != "" {
		branchRunes := []rune(branchText)
		if idx := strings.Index(lineStr, branchText); idx >= 0 {
			// Convert byte index to rune index
			branchStart = utf8.RuneCountInString(lineStr[:idx])
			branchEnd = branchStart + len(branchRunes)
		}
	}

	// Find layout position in the composed line (using rune indices)
	layoutStart := -1
	layoutEnd := -1
	if layoutText != "" {
		layoutRunes := []rune(layoutText)
		if idx := strings.Index(lineStr, layoutText); idx >= 0 {
			layoutStart = utf8.RuneCountInString(lineStr[:idx])
			layoutEnd = layoutStart + len(layoutRunes)
		}
	}

	// Choose branch style based on whether it's the main branch
	branchStyle := e.styleBranch
	if e.IsMainBranch() {
		branchStyle = e.styleMainBranch
	}

	// Choose layout style based on layout name
	layoutStyle := e.styleLayoutOther
	switch {
	case strings.HasPrefix(e.layoutName, "EN"):
		layoutStyle = e.styleLayoutUS
	case strings.HasPrefix(e.layoutName, "RU"):
		layoutStyle = e.styleLayoutRU
	}

	for x, r := range line {
		if x >= w {
			break
		}
		style := e.styleStatus
		if branchStart >= 0 && x >= branchStart && x < branchEnd {
			style = branchStyle
		} else if layoutStart >= 0 && x >= layoutStart && x < layoutEnd {
			style = layoutStyle
		}
		s.SetContent(x, y, r, nil, style)
	}
}
func (e *Editor) renderCommandline(s Screen, w, y int) int {
	var cmdRunes []rune
	var rightText string

	if e.mode == ModeSearch {
		// Search mode: show /query with match count
		prefix := '/'
		if !e.searchForward {
			prefix = '?'
		}
		cmdRunes = append([]rune{prefix}, e.searchQuery...)

		// Show match count on the right
		if len(e.searchMatches) > 0 {
			rightText = fmt.Sprintf(" [%d/%d] ", e.searchMatchIndex+1, len(e.searchMatches))
		} else if len(e.searchQuery) > 0 {
			rightText = " [no matches] "
		}
	} else if e.mode == ModeCommand {
		cmdRunes = append([]rune{':'}, e.cmd...)
	} else {
		cmdRunes = e.cmd
	}

	// Prepare right side: pending keys or last command (if not in search mode)
	// Check if "copied" message should be shown (within 2 seconds)
	const copiedMessageDuration = 2 * time.Second
	showCopiedMessage := time.Since(e.copiedMessageTime) < copiedMessageDuration && e.lastCommand == "y"
	checkmarkPos := -1 // position of ✓ in rightRunes for green coloring

	if rightText == "" {
		if e.pendingKeys != "" {
			// Show pending keys while waiting for next key (e.g., "g", "f")
			rightText = " " + e.pendingKeys + "_ "
		} else if showCopiedMessage {
			// Show "copied [✓] | y"
			rightText = " copied [✓] | y "
			checkmarkPos = 9 // position of ✓ in " copied [✓] | y "
		} else if e.lastCommand != "" {
			// Show last executed command (e.g., "gg", "fw")
			rightText = " " + e.lastCommand + " "
		} else if e.lastKeyCombo != "" {
			// Fallback to last key combo
			rightText = " " + e.lastKeyCombo + " "
		}
	}

	rightRunes := []rune(rightText)
	rightStart := w - len(rightRunes)
	if rightStart < 0 {
		rightStart = 0
		rightRunes = rightRunes[:w]
		// Adjust checkmark position if truncated
		if checkmarkPos >= len(rightRunes) {
			checkmarkPos = -1
		}
	}

	// Calculate available width for command
	availableWidth := rightStart
	if availableWidth < 0 {
		availableWidth = 0
	}

	// Calculate cursor position
	var cursorX int
	if e.mode == ModeCommand {
		cursorX = e.cmdCursor + 1 // +1 for ':' prefix
	} else if e.mode == ModeSearch {
		cursorX = e.searchCursor + 1 // +1 for '/' or '?' prefix
	} else {
		cursorX = len(cmdRunes)
	}

	// Handle scrolling if command is too long
	if len(cmdRunes) > availableWidth {
		// Ensure cursor is visible
		start := 0
		if cursorX > availableWidth-1 {
			start = cursorX - availableWidth + 1
		}
		if start > len(cmdRunes)-availableWidth {
			start = len(cmdRunes) - availableWidth
		}
		if start < 0 {
			start = 0
		}
		cmdRunes = cmdRunes[start:]
		cursorX = cursorX - start
	}

	// Style for green checkmark
	styleGreen := e.styleCommandCheckmark
	if styleGreen == nil {
		styleGreen = e.styleCommand
	}

	// Draw command line content
	for x := 0; x < w; x++ {
		if x < len(cmdRunes) {
			s.SetContent(x, y, cmdRunes[x], nil, e.styleCommand)
		} else if x >= rightStart && x-rightStart < len(rightRunes) {
			idx := x - rightStart
			style := e.styleCommand
			if idx == checkmarkPos {
				style = styleGreen
			}
			s.SetContent(x, y, rightRunes[idx], nil, style)
		} else {
			s.SetContent(x, y, ' ', nil, e.styleCommand)
		}
	}

	if cursorX < 0 {
		cursorX = 0
	}
	if cursorX >= w {
		cursorX = w - 1
	}
	return cursorX
}

// GroupInfo describes a group of commands for layout optimization
type GroupInfo struct {
	Name     string
	Commands []CommandInfo
	Size     int // len(Commands) + 1 for header
}

// groupCommands groups filtered commands by their Group field
func groupCommands(items []CommandInfo) []GroupInfo {
	var groups []GroupInfo
	var current *GroupInfo

	for _, cmd := range items {
		if current == nil || current.Name != cmd.Group {
			if current != nil {
				current.Size = len(current.Commands) + 1
				groups = append(groups, *current)
			}
			current = &GroupInfo{Name: cmd.Group}
		}
		current.Commands = append(current.Commands, cmd)
	}
	if current != nil {
		current.Size = len(current.Commands) + 1
		groups = append(groups, *current)
	}

	return groups
}

// distributeGroups places groups into columns without splitting them
func distributeGroups(groups []GroupInfo, height int) [][]GroupInfo {
	var columns [][]GroupInfo
	var currentCol []GroupInfo
	usedHeight := 0

	for _, g := range groups {
		if usedHeight+g.Size <= height {
			// Group fits in current column
			currentCol = append(currentCol, g)
			usedHeight += g.Size
		} else {
			// Start new column
			if len(currentCol) > 0 {
				columns = append(columns, currentCol)
			}
			currentCol = []GroupInfo{g}
			usedHeight = g.Size
		}
	}
	if len(currentCol) > 0 {
		columns = append(columns, currentCol)
	}

	return columns
}

// calculateOptimalLayout finds the best height and column distribution
// Returns: optimal height, column groups distribution
func calculateOptimalLayout(groups []GroupInfo, maxHeight int) (int, [][]GroupInfo) {
	if len(groups) == 0 {
		return 0, nil
	}

	// Minimum height = max group size
	minHeight := 0
	for _, g := range groups {
		if g.Size > minHeight {
			minHeight = g.Size
		}
	}
	if minHeight > maxHeight {
		minHeight = maxHeight
	}

	bestHeight := maxHeight
	bestCols := len(groups) // worst case: each group in own column
	var bestLayout [][]GroupInfo

	// Try heights from minimum to maximum
	for h := minHeight; h <= maxHeight; h++ {
		layout := distributeGroups(groups, h)
		cols := len(layout)

		// Better if fewer columns, or same columns but smaller height
		if cols < bestCols || (cols == bestCols && h < bestHeight) {
			bestCols = cols
			bestHeight = h
			bestLayout = layout
		}
	}

	return bestHeight, bestLayout
}

// renderCommandAutocomplete renders the command autocomplete popup above the command line
// Uses group-aware layout: groups are never split across columns
func (e *Editor) renderCommandAutocomplete(s Screen, w, statusY int) {
	if !e.cmdAutoCompleteActive || len(e.cmdAutoCompleteItems) == 0 {
		return
	}

	// Group commands
	groups := groupCommands(e.cmdAutoCompleteItems)
	if len(groups) == 0 {
		return
	}

	// Find maximum item width (for all commands across all groups)
	maxItemWidth := 0
	for _, grp := range groups {
		headerW := len(grp.Name) + 6 // "── Group ──"
		if headerW > maxItemWidth {
			maxItemWidth = headerW
		}
		for _, cmd := range grp.Commands {
			itemW := len(cmd.Name) + 3 + len(cmd.Description) // "name - desc"
			if itemW > maxItemWidth {
				maxItemWidth = itemW
			}
		}
	}

	// Calculate optimal height and layout
	maxH := statusY
	if maxH > 15 {
		maxH = 15
	}
	height, colGroups := calculateOptimalLayout(groups, maxH)
	if height == 0 || len(colGroups) == 0 {
		return
	}

	// Store layout for navigation
	cols := len(colGroups)
	e.cmdAutoCompleteCols = cols
	e.cmdAutoCompleteColGroups = colGroups

	// Calculate column width
	colWidth := maxItemWidth + 2
	if colWidth*cols > w {
		colWidth = w / cols
	}

	y0 := statusY - height
	if y0 < 0 {
		y0 = 0
		height = statusY
	}

	// Clear the popup area (only the menu width, not entire screen)
	menuWidth := cols * colWidth
	if menuWidth > w {
		menuWidth = w
	}
	for row := 0; row < height; row++ {
		y := y0 + row
		for x := 0; x < menuWidth; x++ {
			s.SetContent(x, y, ' ', nil, e.styleAutoComplete)
		}
	}

	// Track command index for selection highlighting
	cmdIdx := 0

	// Render each column
	for colIdx, colGrps := range colGroups {
		x0 := colIdx * colWidth
		row := 0

		for _, grp := range colGrps {
			// Render group header
			y := y0 + row
			headerStyle := e.styleAutoCompleteGroup
			headerText := "── " + grp.Name + " "
			x := x0 + 1
			for _, r := range headerText {
				if x >= x0+colWidth-1 || x >= w {
					break
				}
				s.SetContent(x, y, r, nil, headerStyle)
				x++
			}
			// Fill rest with dashes
			for x < x0+colWidth-1 && x < w {
				s.SetContent(x, y, '─', nil, headerStyle)
				x++
			}
			row++

			// Render commands in this group
			for _, cmd := range grp.Commands {
				y := y0 + row
				isSelected := cmdIdx == e.cmdAutoCompleteIndex

				style := e.styleAutoCompleteDescription
				hotkeyStyle := e.styleAutoCompleteHotkey
				if isSelected {
					style = e.styleSelection
					// Keep hotkey visible on selection - use selection bg with hotkey fg
					_, selBg, _ := e.styleSelection.Decompose()
					hotkeyFg, _, _ := e.styleAutoCompleteHotkey.Decompose()
					hotkeyStyle = e.styleAutoCompleteHotkey.Foreground(hotkeyFg).Background(selBg)
				}

				// Clear column area with style
				for x := x0; x < x0+colWidth && x < w; x++ {
					s.SetContent(x, y, ' ', nil, style)
				}

				// Render hotkey (command name) in hotkey color
				x := x0 + 1
				for _, r := range cmd.Name {
					if x >= x0+colWidth-1 || x >= w {
						break
					}
					s.SetContent(x, y, r, nil, hotkeyStyle)
					x++
				}

				// Render " - description" in description style
				descText := " - " + cmd.Description
				for _, r := range descText {
					if x >= x0+colWidth-1 || x >= w {
						break
					}
					s.SetContent(x, y, r, nil, style)
					x++
				}

				cmdIdx++
				row++
			}
		}
	}
}

const scrollIndicatorDuration = 1500 * time.Millisecond

func (e *Editor) renderScrollIndicator(s Screen, w, viewHeight int) {
	if viewHeight < 1 || w < 1 {
		return
	}

	// Check if scroll indicator should be visible
	elapsed := time.Since(e.lastScrollTime)
	if elapsed >= scrollIndicatorDuration {
		return
	}

	totalLines := len(e.lines)
	if totalLines <= viewHeight {
		return // No need for scroll indicator if all content fits
	}

	// Calculate thumb size (minimum 1 row)
	thumbSize := viewHeight * viewHeight / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}

	// Calculate thumb position
	maxScroll := totalLines - viewHeight
	if maxScroll < 1 {
		maxScroll = 1
	}
	thumbPos := e.scroll * (viewHeight - thumbSize) / maxScroll
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos+thumbSize > viewHeight {
		thumbPos = viewHeight - thumbSize
	}

	// Calculate opacity based on time elapsed (fade out effect)
	// 0-1000ms: full opacity, 1000-1500ms: fade out
	var thumbChar rune
	var trackChar rune
	fadeStart := 1000 * time.Millisecond
	if elapsed < fadeStart {
		thumbChar = '█'
		trackChar = '░'
	} else {
		// Fade out: use lighter characters
		fadeProgress := float64(elapsed-fadeStart) / float64(scrollIndicatorDuration-fadeStart)
		if fadeProgress < 0.33 {
			thumbChar = '▓'
			trackChar = '░'
		} else if fadeProgress < 0.66 {
			thumbChar = '▒'
			trackChar = ' '
		} else {
			thumbChar = '░'
			trackChar = ' '
		}
	}

	// Draw scroll indicator in the rightmost column
	x := w - 1
	style := e.styleScrollIndicator
	if style == nil {
		style = e.styleLineNumber
	}
	for y := 0; y < viewHeight; y++ {
		var ch rune
		if y >= thumbPos && y < thumbPos+thumbSize {
			ch = thumbChar
		} else {
			ch = trackChar
		}
		if ch != ' ' {
			s.SetContent(x, y, ch, nil, style)
		}
	}
}
func (e *Editor) drawLine(s Screen, y, w, startX int, line []rune, tabWidth int, selStart, selEnd int, spans []HighlightSpan, highlightActive bool, searchMatches []SearchMatch, lineIdx int, currentMatchIdx int, scrollX int) {
	col := 0 // visual column (accounting for tabs)
	if tabWidth < 1 {
		tabWidth = 1
	}
	fallbackStyle := e.styleMain
	if highlightActive {
		fallbackStyle = e.styleSyntaxUnknown
	}

	for idx, r := range line {
		// Calculate screen x from visual column and scrollX
		x := startX + col - scrollX
		if x >= w {
			break
		}

		// First determine the syntax-highlighted style
		activeStyle := fallbackStyle
		if kind, ok := highlightKindAt(spans, idx); ok {
			if style, ok := e.styleForHighlight(kind); ok {
				activeStyle = style
			}
		} else if highlightActive && !isWordRune(r) {
			activeStyle = e.styleMain
		}

		// Check for search match highlight
		isInMatch := false
		isCurrentMatch := false
		isMatchedChar := false // true if this char is one of the fuzzy-matched letters
		for i, match := range searchMatches {
			if match.Row == lineIdx && match.Length > 0 && idx >= match.Col && idx < match.Col+match.Length {
				isInMatch = true
				if i == currentMatchIdx {
					isCurrentMatch = true
					// Check if this char is in MatchedCols (relative to word start)
					relIdx := idx - match.Col
					for _, mc := range match.MatchedCols {
						if mc == relIdx {
							isMatchedChar = true
							break
						}
					}
					// If no MatchedCols, all chars are matched (exact match)
					if len(match.MatchedCols) == 0 {
						isMatchedChar = true
					}
				}
				break
			}
		}

		// Apply overlays: search match or selection
		if isCurrentMatch && isMatchedChar {
			// Current match, matched letter: yellow highlight
			activeStyle = e.styleSearchMatch
		} else if isCurrentMatch {
			// Current match, non-matched letter: selection background
			_, selBg, _ := e.styleSelection.Decompose()
			fg, _, _ := activeStyle.Decompose()
			activeStyle = activeStyle.Foreground(fg).Background(selBg)
		} else if isInMatch {
			// Other matches: selection background
			_, selBg, _ := e.styleSelection.Decompose()
			fg, _, _ := activeStyle.Decompose()
			activeStyle = activeStyle.Foreground(fg).Background(selBg)
		} else if selStart >= 0 && selEnd > selStart && idx >= selStart && idx < selEnd {
			// Selection: only change background, keep syntax foreground
			_, selBg, _ := e.styleSelection.Decompose()
			fg, _, _ := activeStyle.Decompose()
			activeStyle = activeStyle.Foreground(fg).Background(selBg)
		}
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			for i := 0; i < spaces; i++ {
				tx := startX + col - scrollX
				if tx >= startX && tx < w {
					s.SetContent(tx, y, ' ', nil, activeStyle)
				}
				col++
			}
			continue
		}
		if x >= startX {
			s.SetContent(x, y, r, nil, activeStyle)
		}
		col++
	}
	// Clear rest of line
	for x := startX + col - scrollX; x < w; x++ {
		if x >= startX {
			s.SetContent(x, y, ' ', nil, fallbackStyle)
		}
	}
}
func clearLineAt(s Screen, x0, y, w int, style Style) {
	for x := 0; x < w; x++ {
		s.SetContent(x0+x, y, ' ', nil, style)
	}
}
func composeStatusLine(left, right string, width int) []rune {
	if width <= 0 {
		return nil
	}
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if len(leftRunes)+len(rightRunes) > width {
		if len(rightRunes) >= width {
			rightRunes = rightRunes[len(rightRunes)-width:]
			leftRunes = nil
		} else {
			leftRunes = leftRunes[:width-len(rightRunes)]
		}
	}
	spaceCount := width - len(leftRunes) - len(rightRunes)
	if spaceCount < 0 {
		spaceCount = 0
	}
	line := make([]rune, 0, width)
	line = append(line, leftRunes...)
	for i := 0; i < spaceCount; i++ {
		line = append(line, ' ')
	}
	line = append(line, rightRunes...)
	return line
}
func formatGitBranch(symbol, branch string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		symbol = "git:"
	}
	if strings.HasSuffix(symbol, ":") || strings.HasSuffix(symbol, " ") {
		return symbol + branch
	}
	return symbol + " " + branch
}
func (e *Editor) drawLineWithGutterAt(s Screen, x0, y, w, gutterWidth, lineIdx int) {
	if gutterWidth > 0 {
		// gutterWidth = 1 (leading space) + digits + 1 (trailing space)
		digits := gutterWidth - 2
		if digits < 1 {
			digits = 1
		}
		num := lineIdx + 1
		if e.lineNumberMode == LineNumberRelative && lineIdx != e.cursor.Row {
			diff := lineIdx - e.cursor.Row
			if diff < 0 {
				diff = -diff
			}
			num = diff
		}
		numStr := fmt.Sprintf("%*d", digits, num)
		style := e.styleLineNumber
		if lineIdx == e.cursor.Row {
			style = e.styleLineNumberActive
		}
		// Draw leading space
		if w > 0 {
			s.SetContent(x0, y, ' ', nil, e.styleMain)
		}
		// Draw number (right-aligned with leading spaces)
		for i, r := range numStr {
			x := 1 + i
			if x >= gutterWidth-1 || x >= w {
				break
			}
			s.SetContent(x0+x, y, r, nil, style)
		}
		// Draw trailing space
		if gutterWidth-1 < w {
			s.SetContent(x0+gutterWidth-1, y, ' ', nil, e.styleMain)
		}
	}
	if gutterWidth >= w {
		return
	}
	selStart, selEnd, ok := e.selectionRangeForLine(lineIdx)
	if !ok {
		selStart = -1
		selEnd = -1
	}
	highlightActive := e.highlightStart >= 0 && lineIdx >= e.highlightStart && lineIdx <= e.highlightEnd
	var spans []HighlightSpan
	if highlightActive {
		spans = e.highlights[lineIdx]
	}
	e.drawLine(s, y, x0+w, x0+gutterWidth, e.lines[lineIdx], e.tabWidth, selStart, selEnd, spans, highlightActive, e.searchMatches, lineIdx, e.searchMatchIndex, e.scrollX)
}
func (e *Editor) renderBranchPicker(s Screen, w, viewHeight int) {
	if !e.branchPickerActive || len(e.branchPickerItems) == 0 {
		return
	}
	if w < 6 || viewHeight < 3 {
		return
	}
	title := "Select git branch"
	titleRunes := []rune(title)
	titleWidth := len(titleRunes) + 2
	maxItem := titleWidth
	for _, name := range e.branchPickerItems {
		l := len([]rune(name)) + 2 // "* " or "  " prefix for all branches
		if l > maxItem {
			maxItem = l
		}
	}
	boxWidth := maxItem + 4
	if boxWidth > w-2 {
		boxWidth = w - 2
	}
	if boxWidth < 8 {
		if w < 8 {
			boxWidth = w
		} else {
			boxWidth = 8
		}
	}
	listHeight := viewHeight - 2
	if listHeight < 1 {
		return
	}
	if listHeight > len(e.branchPickerItems) {
		listHeight = len(e.branchPickerItems)
	}
	boxHeight := listHeight + 2
	if boxHeight > viewHeight {
		boxHeight = viewHeight
		listHeight = boxHeight - 2
	}
	x0 := (w - boxWidth) / 2
	if x0 < 0 {
		x0 = 0
	}
	y0 := (viewHeight - boxHeight) / 2
	if y0 < 0 {
		y0 = 0
	}

	borderStyle := e.styleBoxBorder
	if borderStyle == nil {
		borderStyle = e.styleStatus
	}
	itemStyle := e.styleStatus
	selectedStyle := e.styleSelection
	innerWidth := boxWidth - 2

	topLeft := '┌'
	topRight := '┐'
	bottomLeft := '└'
	bottomRight := '┘'
	hLine := '─'
	vLine := '│'
	for x := 0; x < boxWidth; x++ {
		chTop := hLine
		chBottom := hLine
		if x == 0 {
			chTop = topLeft
			chBottom = bottomLeft
		} else if x == boxWidth-1 {
			chTop = topRight
			chBottom = bottomRight
		}
		s.SetContent(x0+x, y0, chTop, nil, borderStyle)
		s.SetContent(x0+x, y0+boxHeight-1, chBottom, nil, borderStyle)
	}
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, vLine, nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, vLine, nil, borderStyle)
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, itemStyle)
		}
	}
	if innerWidth > 0 {
		label := make([]rune, 0, innerWidth)
		label = append(label, ' ')
		label = append(label, titleRunes...)
		label = append(label, ' ')
		if len(label) > innerWidth {
			label = titleRunes
			if len(label) > innerWidth {
				label = label[:innerWidth]
			}
		}
		for i, r := range label {
			s.SetContent(x0+1+i, y0, r, nil, borderStyle)
		}
	}

	start := e.branchPickerIndex - listHeight/2
	maxStart := len(e.branchPickerItems) - listHeight
	if maxStart < 0 {
		maxStart = 0
	}
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}

	// Style for current branch marker (light green)
	markerStyle := e.styleBranchMarker
	if markerStyle == nil {
		markerStyle = e.styleMainBranch
	}
	_, markerBg, _ := itemStyle.Decompose()
	markerStyle = markerStyle.Background(markerBg)

	for i := 0; i < listHeight; i++ {
		idx := start + i
		if idx >= len(e.branchPickerItems) {
			break
		}
		branchName := e.branchPickerItems[idx]
		isCurrentBranch := branchName == e.gitBranch
		isMainBranch := branchName == "main" || branchName == "master" || branchName == e.gitMainBranch

		// Determine style - keep foreground, only change background when selected
		style := itemStyle
		if isMainBranch {
			style = e.styleMainBranch
		}
		if idx == e.branchPickerIndex {
			fg, _, _ := style.Decompose()
			_, selBg, _ := selectedStyle.Decompose()
			style = style.Foreground(fg).Background(selBg)
		}

		lineY := y0 + 1 + i
		// Clear line
		for x := 0; x < innerWidth; x++ {
			s.SetContent(x0+1+x, lineY, ' ', nil, style)
		}

		// Draw marker for current branch (or space for alignment)
		xOffset := 2
		if isCurrentBranch {
			currentMarkerStyle := markerStyle
			if idx == e.branchPickerIndex {
				_, selBg, _ := selectedStyle.Decompose()
				currentMarkerStyle = currentMarkerStyle.Background(selBg)
			}
			s.SetContent(x0+1, lineY, '*', nil, currentMarkerStyle)
		} else {
			s.SetContent(x0+1, lineY, ' ', nil, style)
		}
		s.SetContent(x0+2, lineY, ' ', nil, style)

		// Draw branch name
		runes := []rune(branchName)
		maxLen := innerWidth - xOffset
		if len(runes) > maxLen {
			runes = runes[:maxLen]
		}
		for j, r := range runes {
			s.SetContent(x0+1+xOffset+j, lineY, r, nil, style)
		}
	}
}
func (e *Editor) renderRefsSidebar(s Screen, sidebarWidth, viewHeight int) {
	if !e.refsPickerActive || len(e.refsPickerItems) == 0 {
		return
	}

	borderStyle := e.styleBoxBorder
	if borderStyle == nil {
		borderStyle = e.styleStatus
	}
	itemStyle := e.styleMain
	selectedStyle := e.styleSelection

	// Draw vertical separator on the right edge
	for y := 0; y < viewHeight; y++ {
		s.SetContent(sidebarWidth-1, y, '│', nil, borderStyle)
	}

	// Clear sidebar area
	innerWidth := sidebarWidth - 1
	for y := 0; y < viewHeight; y++ {
		for x := 0; x < innerWidth; x++ {
			s.SetContent(x, y, ' ', nil, itemStyle)
		}
	}

	// Draw title bar
	title := e.refsPickerTitle
	counter := fmt.Sprintf(" %d/%d", e.refsPickerIndex+1, len(e.refsPickerItems))
	titleLine := title + counter
	titleRunes := []rune(titleLine)
	for i, r := range titleRunes {
		if i < innerWidth {
			s.SetContent(i, 0, r, nil, borderStyle)
		}
	}
	// Fill rest of title line
	for x := len(titleRunes); x < innerWidth; x++ {
		s.SetContent(x, 0, ' ', nil, borderStyle)
	}

	// Draw list items starting from line 1
	listHeight := viewHeight - 1
	start := e.refsPickerIndex - listHeight/2
	maxStart := len(e.refsPickerItems) - listHeight
	if maxStart < 0 {
		maxStart = 0
	}
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}

	for i := 0; i < listHeight; i++ {
		idx := start + i
		lineY := 1 + i
		if idx >= len(e.refsPickerItems) {
			continue
		}
		loc := e.refsPickerItems[idx]

		// Format: filename:line
		displayPath := loc.Path
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, loc.Path); err == nil && !strings.HasPrefix(rel, "..") {
				displayPath = rel
			}
		}
		// Use just filename if path is too long
		if len(displayPath) > sidebarWidth-10 {
			displayPath = filepath.Base(loc.Path)
		}
		label := fmt.Sprintf("%s:%d", displayPath, loc.StartLine+1)
		labelRunes := []rune(label)

		style := itemStyle
		if idx == e.refsPickerIndex {
			style = selectedStyle
		}

		// Clear line
		for x := 0; x < innerWidth; x++ {
			s.SetContent(x, lineY, ' ', nil, style)
		}

		// Draw indicator
		if idx == e.refsPickerIndex {
			s.SetContent(0, lineY, '>', nil, style)
		}

		// Draw label (truncate if needed)
		maxLen := innerWidth - 2
		if len(labelRunes) > maxLen {
			labelRunes = labelRunes[:maxLen]
		}
		for j, r := range labelRunes {
			s.SetContent(2+j, lineY, r, nil, style)
		}
	}
}
func (e *Editor) renderSpaceMenu(s Screen, w, viewHeight int) {
	if !e.spaceMenuActive {
		return
	}
	if w < 20 || viewHeight < 5 {
		return
	}

	// Find the maximum label width
	maxLabelWidth := 0
	for _, item := range SpaceMenuItems {
		labelWidth := len(item.Label) + 6 // "x   Label"
		if labelWidth > maxLabelWidth {
			maxLabelWidth = labelWidth
		}
	}

	// Box dimensions
	boxWidth := maxLabelWidth + 4
	if boxWidth > w-4 {
		boxWidth = w - 4
	}
	innerWidth := boxWidth - 2

	listHeight := len(SpaceMenuItems)
	if listHeight > viewHeight-3 {
		listHeight = viewHeight - 3
	}
	boxHeight := listHeight + 2

	// Position at bottom right, above status line
	x0 := w - boxWidth - 1
	if x0 < 0 {
		x0 = 0
	}
	y0 := viewHeight - boxHeight
	if y0 < 0 {
		y0 = 0
	}

	borderStyle := e.styleBoxBorder
	if borderStyle == nil {
		borderStyle = e.styleStatus
	}
	itemStyle := e.styleCommand
	dimStyle := e.styleLineNumber // for unimplemented items

	// Draw border
	topLeft := '┌'
	topRight := '┐'
	bottomLeft := '└'
	bottomRight := '┘'
	hLine := '─'
	vLine := '│'

	// Top border with title
	title := "Space"
	titleRunes := []rune(title)

	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = topLeft
		} else if x == boxWidth-1 {
			ch = topRight
		}
		s.SetContent(x0+x, y0, ch, nil, borderStyle)
	}

	// Embed title in top border
	if len(titleRunes)+2 <= boxWidth-2 {
		for i, r := range titleRunes {
			s.SetContent(x0+1+i, y0, r, nil, borderStyle)
		}
	}

	// Bottom border
	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = bottomLeft
		} else if x == boxWidth-1 {
			ch = bottomRight
		}
		s.SetContent(x0+x, y0+boxHeight-1, ch, nil, borderStyle)
	}

	// Side borders and content
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, vLine, nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, vLine, nil, borderStyle)

		// Clear interior
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, itemStyle)
		}
	}

	// Draw menu items
	for i := 0; i < listHeight; i++ {
		if i >= len(SpaceMenuItems) {
			break
		}
		item := SpaceMenuItems[i]
		lineY := y0 + 1 + i

		// Choose style based on whether item is implemented
		style := itemStyle
		if !item.Implemented {
			style = dimStyle
		}

		// Clear line
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, lineY, ' ', nil, style)
		}

		// Format: " k   Label text"
		keyStr := string(item.Key)
		label := " " + keyStr + "   " + item.Label

		runes := []rune(label)
		if len(runes) > innerWidth {
			runes = runes[:innerWidth]
		}

		for j, r := range runes {
			s.SetContent(x0+1+j, lineY, r, nil, style)
		}
	}
}

// renderMenu renders a generic mode menu popup
func (e *Editor) renderMenu(s Screen, w, viewHeight int, title string, items []SpaceMenuItem) {
	if w < 20 || viewHeight < 5 {
		return
	}

	// Find the maximum label width
	maxLabelWidth := 0
	for _, item := range items {
		labelWidth := len(item.Label) + 6 // "x   Label"
		if labelWidth > maxLabelWidth {
			maxLabelWidth = labelWidth
		}
	}

	// Box dimensions
	boxWidth := maxLabelWidth + 4
	if boxWidth > w-4 {
		boxWidth = w - 4
	}
	innerWidth := boxWidth - 2

	listHeight := len(items)
	if listHeight > viewHeight-3 {
		listHeight = viewHeight - 3
	}
	boxHeight := listHeight + 2

	// Position at bottom right, above status line
	x0 := w - boxWidth - 1
	if x0 < 0 {
		x0 = 0
	}
	y0 := viewHeight - boxHeight
	if y0 < 0 {
		y0 = 0
	}

	borderStyle := e.styleBoxBorder
	if borderStyle == nil {
		borderStyle = e.styleStatus
	}
	itemStyle := e.styleCommand
	dimStyle := e.styleLineNumber

	// Draw border
	topLeft := '┌'
	topRight := '┐'
	bottomLeft := '└'
	bottomRight := '┘'
	hLine := '─'
	vLine := '│'

	// Top border with title
	titleRunes := []rune(title)

	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = topLeft
		} else if x == boxWidth-1 {
			ch = topRight
		}
		s.SetContent(x0+x, y0, ch, nil, borderStyle)
	}

	// Embed title in top border
	if len(titleRunes)+2 <= boxWidth-2 {
		for i, r := range titleRunes {
			s.SetContent(x0+1+i, y0, r, nil, borderStyle)
		}
	}

	// Bottom border
	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = bottomLeft
		} else if x == boxWidth-1 {
			ch = bottomRight
		}
		s.SetContent(x0+x, y0+boxHeight-1, ch, nil, borderStyle)
	}

	// Side borders and content
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, vLine, nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, vLine, nil, borderStyle)

		// Clear interior
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, itemStyle)
		}
	}

	// Draw menu items
	for i := 0; i < listHeight; i++ {
		if i >= len(items) {
			break
		}
		item := items[i]
		lineY := y0 + 1 + i

		// Choose style based on whether item is implemented
		style := itemStyle
		if !item.Implemented {
			style = dimStyle
		}

		// Clear line
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, lineY, ' ', nil, style)
		}

		// Format: " k   Label text"
		keyStr := string(item.Key)
		label := " " + keyStr + "   " + item.Label

		runes := []rune(label)
		if len(runes) > innerWidth {
			runes = runes[:innerWidth]
		}

		for j, r := range runes {
			s.SetContent(x0+1+j, lineY, r, nil, style)
		}
	}
}

// renderKeybindingsHelp renders a help popup showing all keybindings
func (e *Editor) renderKeybindingsHelp(s Screen, w, viewHeight int) {
	if w < 40 || viewHeight < 10 {
		return
	}

	// Keybinding entry with group info
	type keybinding struct {
		key    string
		action string
		desc   string
		group  string
	}

	// Action to group mapping
	actionGroups := map[string]string{
		// Navigation
		"move_left": "Navigation", "move_right": "Navigation", "move_up": "Navigation", "move_down": "Navigation",
		"word_left": "Navigation", "word_right": "Navigation", "word_forward": "Navigation", "word_backward": "Navigation", "word_end": "Navigation",
		"line_start": "Navigation", "line_end": "Navigation", "file_start": "Navigation", "file_end": "Navigation",
		"page_up": "Navigation", "page_down": "Navigation", "scroll_up": "Navigation", "scroll_down": "Navigation",
		// Editing
		"delete": "Editing", "change": "Editing", "yank": "Editing", "paste": "Editing", "paste_before": "Editing",
		"open_below": "Editing", "open_above": "Editing", "append": "Editing", "append_line_end": "Editing",
		"insert_line_start": "Editing", "join_lines": "Editing", "replace_char": "Editing", "delete_line": "Editing",
		"indent": "Editing", "unindent": "Editing", "insert_line_above": "Editing",
		// Selection
		"toggle_select": "Selection", "extend_line": "Selection", "collapse_selection": "Selection", "select_all": "Selection",
		// Search
		"search_forward": "Search", "search_backward": "Search", "search_next": "Search", "search_prev": "Search",
		"find_char": "Search", "find_char_backward": "Search", "till_char": "Search", "till_char_backward": "Search",
		// Modes
		"enter_insert": "Modes", "enter_command": "Modes", "goto_mode": "Modes", "match_mode": "Modes",
		"view_mode": "Modes", "space_mode": "Modes",
		// History
		"undo": "History", "redo": "History",
		// Other
		"quit": "Other", "branch_picker": "Other", "toggle_line_numbers": "Other",
	}

	// Action descriptions
	bindingDescs := map[string]string{
		"move_left": "Move cursor left", "move_right": "Move cursor right",
		"move_up": "Move cursor up", "move_down": "Move cursor down",
		"word_left": "Move to previous word", "word_right": "Move to next word",
		"word_forward": "Move to next word", "word_backward": "Move to previous word", "word_end": "Move to word end",
		"line_start": "Move to line start", "line_end": "Move to line end",
		"file_start": "Move to file start", "file_end": "Move to file end",
		"page_up": "Page up", "page_down": "Page down",
		"scroll_up": "Scroll up", "scroll_down": "Scroll down",
		"enter_insert": "Enter insert mode", "enter_command": "Enter command mode",
		"quit": "Quit editor", "undo": "Undo", "redo": "Redo",
		"delete": "Delete selection", "change": "Change (delete + insert)",
		"yank": "Yank (copy)", "paste": "Paste after", "paste_before": "Paste before",
		"open_below": "Open line below", "open_above": "Open line above",
		"append": "Append after cursor", "append_line_end": "Append at line end",
		"insert_line_start": "Insert at line start", "join_lines": "Join lines",
		"toggle_select": "Toggle select mode", "extend_line": "Extend to full line",
		"collapse_selection": "Collapse selection", "select_all": "Select all",
		"indent": "Indent", "unindent": "Unindent",
		"goto_mode": "Goto mode (g)", "match_mode": "Match mode (m)", "view_mode": "View mode (z)", "space_mode": "Space menu",
		"find_char": "Find char (f)", "find_char_backward": "Find char back (F)",
		"till_char": "Till char (t)", "till_char_backward": "Till char back (T)",
		"search_forward": "Search /", "search_backward": "Search ?",
		"search_next": "Next match (n)", "search_prev": "Prev match (N)",
		"replace_char": "Replace char (r)", "delete_line": "Delete line",
		"branch_picker": "Branch picker", "insert_line_above": "Insert line above",
		"toggle_line_numbers": "Toggle line numbers",
	}

	// Build bindings list grouped
	var allBindings []keybinding
	for key, action := range e.keymap.normal {
		desc := bindingDescs[action]
		if desc == "" {
			desc = action
		}
		group := actionGroups[action]
		if group == "" {
			group = "Other"
		}
		allBindings = append(allBindings, keybinding{key, action, desc, group})
	}

	// Sort by group, then action, then key (stable order)
	sort.Slice(allBindings, func(i, j int) bool {
		if allBindings[i].group != allBindings[j].group {
			return allBindings[i].group < allBindings[j].group
		}
		if allBindings[i].action != allBindings[j].action {
			return allBindings[i].action < allBindings[j].action
		}
		return allBindings[i].key < allBindings[j].key
	})

	// Apply filters (fuzzy match per column)
	filterKey := strings.ToLower(string(e.keybindingsHelpFilterKey))
	filterAct := strings.ToLower(string(e.keybindingsHelpFilterAct))
	filterDesc := strings.ToLower(string(e.keybindingsHelpFilterDesc))
	var filteredBindings []keybinding
	for _, b := range allBindings {
		matchKey := filterKey == "" || fuzzyMatch(filterKey, strings.ToLower(b.key))
		matchAct := filterAct == "" || fuzzyMatch(filterAct, strings.ToLower(b.action))
		matchDesc := filterDesc == "" || fuzzyMatch(filterDesc, strings.ToLower(b.desc))
		if matchKey && matchAct && matchDesc {
			filteredBindings = append(filteredBindings, b)
		}
	}

	// Build display list with group headers
	type displayRow struct {
		text       string
		isHeader   bool
		isGroupHdr bool
	}
	var rows []displayRow

	lastGroup := ""
	for _, b := range filteredBindings {
		if b.group != lastGroup {
			if lastGroup != "" {
				rows = append(rows, displayRow{"", false, false}) // blank line between groups
			}
			rows = append(rows, displayRow{b.group, false, true})
			lastGroup = b.group
		}
		keyCol := fmt.Sprintf("%-18s", b.key)
		actionCol := fmt.Sprintf("%-21s", b.action)
		rows = append(rows, displayRow{" " + keyCol + actionCol + b.desc, false, false})
	}

	// Box dimensions - wider
	boxWidth := w - 4
	if boxWidth > 100 {
		boxWidth = 100
	}
	boxHeight := viewHeight - 2
	innerWidth := boxWidth - 2
	// Header/filter row (1) + separator (1) = 2 fixed rows + borders (2) = 4
	listHeight := boxHeight - 4

	// Clamp scroll
	maxScroll := len(rows) - listHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if e.keybindingsHelpScroll > maxScroll {
		e.keybindingsHelpScroll = maxScroll
	}
	if e.keybindingsHelpScroll < 0 {
		e.keybindingsHelpScroll = 0
	}

	// Center popup
	x0 := (w - boxWidth) / 2
	y0 := (viewHeight - boxHeight) / 2

	borderStyle := e.styleBoxBorder
	if borderStyle == nil {
		borderStyle = e.styleStatus
	}
	contentStyle := e.styleCommand
	headerStyle := e.styleStatus

	// Draw border
	for x := 0; x < boxWidth; x++ {
		ch := '─'
		if x == 0 {
			ch = '┌'
		} else if x == boxWidth-1 {
			ch = '┐'
		}
		s.SetContent(x0+x, y0, ch, nil, borderStyle)
		ch = '─'
		if x == 0 {
			ch = '└'
		} else if x == boxWidth-1 {
			ch = '┘'
		}
		s.SetContent(x0+x, y0+boxHeight-1, ch, nil, borderStyle)
	}

	// Title centered
	title := "Keybindings"
	titleRunes := []rune(title)
	titleStart := (boxWidth - len(titleRunes)) / 2
	for i, r := range titleRunes {
		s.SetContent(x0+titleStart+i, y0, r, nil, borderStyle)
	}

	// Hints at bottom left
	hints := "Up,Down,Home,End,Tab,Esc"
	for i, r := range hints {
		if i+1 < boxWidth-1 {
			s.SetContent(x0+1+i, y0+boxHeight-1, r, nil, borderStyle)
		}
	}

	// Side borders and clear interior
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, '│', nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, '│', nil, borderStyle)
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, contentStyle)
		}
	}

	// Row 1: Column headers with filter inputs
	filterActiveStyle := e.styleFilterActive
	if filterActiveStyle == nil {
		filterActiveStyle = contentStyle
	}
	filterInactiveStyle := e.styleFilterInactive
	if filterInactiveStyle == nil {
		filterInactiveStyle = contentStyle
	}

	// Draw column headers with filters
	col := 1
	// Key column
	keyLabel := " Key "
	for _, r := range keyLabel {
		s.SetContent(x0+col, y0+1, r, nil, headerStyle)
		col++
	}
	// Key filter box [11 chars]
	keyFilter := string(e.keybindingsHelpFilterKey)
	keyFilterStyle := filterInactiveStyle
	if e.keybindingsHelpFilterFocus == 0 {
		keyFilterStyle = filterActiveStyle
		keyFilter += "_"
	}
	for i := 0; i < 11; i++ {
		ch := ' '
		if i < len(keyFilter) {
			ch = rune(keyFilter[i])
		}
		s.SetContent(x0+col+i, y0+1, ch, nil, keyFilterStyle)
	}
	col += 13

	// Action column
	actLabel := " Action "
	for _, r := range actLabel {
		s.SetContent(x0+col, y0+1, r, nil, headerStyle)
		col++
	}
	// Action filter box [10 chars]
	actFilter := string(e.keybindingsHelpFilterAct)
	actFilterStyle := filterInactiveStyle
	if e.keybindingsHelpFilterFocus == 1 {
		actFilterStyle = filterActiveStyle
		actFilter += "_"
	}
	for i := 0; i < 10; i++ {
		ch := ' '
		if i < len(actFilter) {
			ch = rune(actFilter[i])
		}
		s.SetContent(x0+col+i, y0+1, ch, nil, actFilterStyle)
	}
	col += 13

	// Description column
	descLabel := " Description "
	for _, r := range descLabel {
		s.SetContent(x0+col, y0+1, r, nil, headerStyle)
		col++
	}
	// Description filter box
	descFilter := string(e.keybindingsHelpFilterDesc)
	descFilterStyle := filterInactiveStyle
	if e.keybindingsHelpFilterFocus == 2 {
		descFilterStyle = filterActiveStyle
		descFilter += "_"
	}
	remainingWidth := innerWidth - col
	if remainingWidth < 0 {
		remainingWidth = 0
	}
	if remainingWidth > 15 {
		remainingWidth = 15
	}
	for i := 0; i < remainingWidth; i++ {
		ch := ' '
		if i < len(descFilter) {
			ch = rune(descFilter[i])
		}
		s.SetContent(x0+col+i, y0+1, ch, nil, descFilterStyle)
	}

	// Row 2: Separator
	for x := 1; x < boxWidth-1; x++ {
		s.SetContent(x0+x, y0+2, '─', nil, borderStyle)
	}

	// Draw scrollable content starting at row 3
	for i := 0; i < listHeight; i++ {
		idx := i + e.keybindingsHelpScroll
		if idx >= len(rows) {
			break
		}
		row := rows[idx]
		lineY := y0 + 3 + i

		if row.isGroupHdr {
			// Draw group name centered with full row background
			groupRunes := []rune(row.text)
			groupLen := len(groupRunes)
			leftPad := (innerWidth - groupLen) / 2
			if leftPad < 0 {
				leftPad = 0
			}
			for j := 0; j < innerWidth; j++ {
				ch := ' '
				if j >= leftPad && j < leftPad+groupLen {
					ch = groupRunes[j-leftPad]
				}
				s.SetContent(x0+1+j, lineY, ch, nil, contentStyle)
			}
		} else {
			runes := []rune(row.text)
			if len(runes) > innerWidth {
				runes = runes[:innerWidth]
			}
			for j := 0; j < innerWidth; j++ {
				ch := ' '
				if j < len(runes) {
					ch = runes[j]
				}
				s.SetContent(x0+1+j, lineY, ch, nil, contentStyle)
			}
		}
	}

	// Scroll indicator
	if len(rows) > listHeight {
		scrollInfo := fmt.Sprintf(" %d/%d ", e.keybindingsHelpScroll+1, max(1, len(rows)-listHeight+1))
		infoRunes := []rune(scrollInfo)
		startX := x0 + boxWidth - len(infoRunes) - 1
		for i, r := range infoRunes {
			s.SetContent(startX+i, y0+boxHeight-1, r, nil, borderStyle)
		}
	}
}
