package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
		isCurrentBranch := branchName == e.git.branch
		isMainBranch := branchName == "main" || branchName == "master" || branchName == e.git.mainBranch

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
