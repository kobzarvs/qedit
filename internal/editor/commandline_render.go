package editor

import (
	"fmt"
	"time"
)

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
		cmdRunes = append([]rune{':'}, e.commandLine.text...)
	} else {
		cmdRunes = e.commandLine.text
	}

	// Prepare right side: pending keys or last command (if not in search mode)
	// Check if "copied" message should be shown (within 2 seconds)
	const copiedMessageDuration = 2 * time.Second
	showCopiedMessage := time.Since(e.ui.copiedMessageTime) < copiedMessageDuration && e.modal.lastCommand == "y"
	checkmarkPos := -1 // position of ✓ in rightRunes for green coloring

	if rightText == "" {
		if e.modal.pendingKeys != "" {
			// Show pending keys while waiting for next key (e.g., "g", "f")
			rightText = " " + e.modal.pendingKeys + "_ "
		} else if showCopiedMessage {
			// Show "copied [✓] | y"
			rightText = " copied [✓] | y "
			checkmarkPos = 9 // position of ✓ in " copied [✓] | y "
		} else if e.modal.lastCommand != "" {
			// Show last executed command (e.g., "gg", "fw")
			rightText = " " + e.modal.lastCommand + " "
		} else if e.ui.lastKeyCombo != "" {
			// Fallback to last key combo
			rightText = " " + e.ui.lastKeyCombo + " "
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
		cursorX = e.commandLine.cursor + 1 // +1 for ':' prefix
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

// distributeGroupsBalanced packs groups into at most maxCols columns (greedy by height).
func distributeGroupsBalanced(groups []GroupInfo, maxCols int) [][]GroupInfo {
	if len(groups) == 0 {
		return nil
	}
	if maxCols <= 0 {
		maxCols = maxCmdAutocompleteColumns
	}
	if maxCols > len(groups) {
		maxCols = len(groups)
	}

	columns := make([][]GroupInfo, maxCols)
	colHeights := make([]int, maxCols)
	for _, g := range groups {
		minIdx := 0
		for i := 1; i < maxCols; i++ {
			if colHeights[i] < colHeights[minIdx] {
				minIdx = i
			}
		}
		columns[minIdx] = append(columns[minIdx], g)
		colHeights[minIdx] += g.Size
	}

	var result [][]GroupInfo
	for _, col := range columns {
		if len(col) > 0 {
			result = append(result, col)
		}
	}
	return result
}

// columnContentHeight returns the tallest column height in rows.
func columnContentHeight(columns [][]GroupInfo) int {
	maxH := 0
	for _, col := range columns {
		h := 0
		for _, g := range col {
			h += g.Size
		}
		if h > maxH {
			maxH = h
		}
	}
	return maxH
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

// calculateOptimalLayout finds viewport height and column distribution (at most maxCols columns).
// Returns: visible viewport height, column groups distribution.
func calculateOptimalLayout(groups []GroupInfo, maxHeight int, maxCols int) (int, [][]GroupInfo) {
	if len(groups) == 0 {
		return 0, nil
	}
	if maxCols <= 0 {
		maxCols = maxCmdAutocompleteColumns
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
	bestCols := len(groups) + 1
	var bestLayout [][]GroupInfo

	// Try heights from minimum to maximum; prefer fewer columns within maxCols.
	for h := minHeight; h <= maxHeight; h++ {
		layout := distributeGroups(groups, h)
		cols := len(layout)
		if cols > maxCols {
			continue
		}

		if cols < bestCols || (cols == bestCols && h < bestHeight) {
			bestCols = cols
			bestHeight = h
			bestLayout = layout
		}
	}

	if bestLayout != nil {
		return bestHeight, bestLayout
	}

	// Fallback: pack into maxCols columns and scroll if content is taller than viewport.
	layout := distributeGroupsBalanced(groups, maxCols)
	return maxHeight, layout
}

// renderCommandAutocomplete renders the command autocomplete popup above the command line
// Uses group-aware layout: groups are never split across columns
func (e *Editor) renderCommandAutocomplete(s Screen, w, statusY int) {
	if !e.cmdAutoComplete.active || len(e.cmdAutoComplete.items) == 0 {
		return
	}

	// Group commands
	groups := groupCommands(e.cmdAutoComplete.items)
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

	// Calculate optimal height and layout (at most 3 columns; scroll if needed).
	maxH := statusY
	if maxH < 1 {
		return
	}
	visibleHeight, colGroups := calculateOptimalLayout(groups, maxH, maxCmdAutocompleteColumns)
	if visibleHeight == 0 || len(colGroups) == 0 {
		return
	}

	contentHeight := columnContentHeight(colGroups)
	if contentHeight < visibleHeight {
		visibleHeight = contentHeight
	}

	// Store layout for navigation
	cols := len(colGroups)
	e.cmdAutoComplete.cols = cols
	e.cmdAutoComplete.colGroups = colGroups
	e.cmdAutoComplete.visibleHeight = visibleHeight
	e.cmdAutoComplete.contentHeight = contentHeight
	e.clampCmdAutoCompleteScroll()
	e.ensureCmdAutoCompleteVisible()

	scroll := e.cmdAutoComplete.scroll

	// Calculate column width
	colWidth := maxItemWidth + 2
	if colWidth*cols > w {
		colWidth = w / cols
	}
	if colWidth < 8 {
		colWidth = 8
	}

	y0 := statusY - visibleHeight
	if y0 < 0 {
		y0 = 0
		visibleHeight = statusY
		e.cmdAutoComplete.visibleHeight = visibleHeight
		e.clampCmdAutoCompleteScroll()
	}

	// Clear the popup area (only the menu width, not entire screen)
	menuWidth := cols * colWidth
	if menuWidth > w {
		menuWidth = w
	}
	for row := 0; row < visibleHeight; row++ {
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
			if row >= scroll && row-scroll < visibleHeight {
				y := y0 + (row - scroll)
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
			}
			row++

			// Render commands in this group
			for _, cmd := range grp.Commands {
				if row < scroll {
					cmdIdx++
					row++
					continue
				}
				if row-scroll >= visibleHeight {
					cmdIdx++
					row++
					continue
				}
				y := y0 + (row - scroll)
				isSelected := cmdIdx == e.cmdAutoComplete.index

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

	// Scroll indicator when content overflows viewport
	if contentHeight > visibleHeight {
		maxScroll := contentHeight - visibleHeight
		scrollInfo := fmt.Sprintf(" %d/%d ", scroll+1, maxScroll+1)
		infoRunes := []rune(scrollInfo)
		startX := menuWidth - len(infoRunes)
		if startX < 0 {
			startX = 0
		}
		if startX+len(infoRunes) > w {
			infoRunes = infoRunes[:w-startX]
		}
		indicatorY := y0 + visibleHeight - 1
		if indicatorY >= 0 && indicatorY < statusY {
			for i, r := range infoRunes {
				s.SetContent(startX+i, indicatorY, r, nil, e.styleAutoCompleteGroup)
			}
		}
	}
}
