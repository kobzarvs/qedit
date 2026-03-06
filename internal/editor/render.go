package editor

import (
	"strings"
	"time"
)

func (e *Editor) Render(s Screen) {
	w, h := s.Size()
	if w <= 0 || h <= 0 {
		return
	}
	now := time.Now()
	notificationActive := e.notificationActive(now)
	showTopMessage := e.showTopStatusMessage(w) || notificationActive

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
	e.ensureConflictBlocks()

	// Calculate sidebar width (refs picker or new sidebar, mutually exclusive)
	sidebarWidth := 0
	if e.sidebar != nil && e.sidebar.Visible {
		sidebarWidth = e.sidebar.CalculateWidth(w)
	} else if e.refsPicker.active && len(e.refsPicker.items) > 0 {
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
	if !e.interaction.freeScroll {
		e.ensureCursorVisible(viewHeight)
		e.ensureCursorVisibleHorizontal(editorWidth, gutterWidth)
	}

	s.SetStyle(e.styleMain)
	s.Clear()

	// Draw editor content (offset by sidebar)
	if editorWidth > 0 {
		if e.fileTreePreviewVisible() {
			e.renderFileTreePreview(s, editorX, viewHeight, editorWidth)
		} else {
			for y := 0; y < viewHeight; y++ {
				lineIdx := e.scroll + y
				if lineIdx >= e.LineCount() {
					clearLineAt(s, editorX, y, editorWidth, e.styleMain)
					continue
				}
				e.drawLineWithGutterAt(s, editorX, y, editorWidth, gutterWidth, lineIdx)
			}
		}
	}

	// Draw sidebar (new sidebar takes priority over refs picker)
	if e.sidebar != nil && e.sidebar.Visible && sidebarWidth > 0 {
		e.sidebar.Render(s, e.sidebarStyles, 0, 0, sidebarWidth, viewHeight)
	} else if e.refsPicker.active && sidebarWidth > 0 {
		e.renderRefsSidebar(s, sidebarWidth, viewHeight)
	}

	// Draw scroll indicator if recently scrolled
	e.renderScrollIndicator(s, w, viewHeight)

	var cx, cy int
	if statusY >= 0 && !e.zoomPendingRestore {
		e.renderStatusline(s, w, statusY, showTopMessage)
	}
	if cmdY >= 0 && !e.zoomPendingRestore {
		cmdCursor := e.renderCommandline(s, w, cmdY)
		if e.mode == ModeCommand || e.mode == ModeSearch {
			cx = cmdCursor
			cy = cmdY
		}
		if e.mode == ModeCommand && e.cmdAutoComplete.active {
			e.renderCommandAutocomplete(s, w, statusY)
		}
	}
	cursorVisible := true
	if e.mode != ModeCommand && e.mode != ModeSearch && e.mode != ModeBranchPicker {
		cy = e.cursor.Row - e.scroll
		if cy < 0 || cy >= viewHeight {
			cursorVisible = false
		}
		if e.cursor.Row >= 0 && e.cursor.Row < e.LineCount() {
			cx = editorX + gutterWidth + visualCol(e.text.Line(e.cursor.Row), e.cursor.Col, e.tabWidth) - e.scrollX
		}
		if cx < editorX+gutterWidth {
			cx = editorX + gutterWidth
		}
		maxCursorX := w - 1
		if maxCursorX < editorX+gutterWidth {
			maxCursorX = editorX + gutterWidth
		}
		if cx > maxCursorX {
			cx = maxCursorX
		}
		if cx >= w {
			cx = w - 1
		}
	}

	if e.branchPicker.active {
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
	if e.keybindingsHelp.active {
		e.renderKeybindingsHelp(s, w, viewHeight)
	}
	if showTopMessage {
		if notificationActive {
			e.renderNotification(s, w, now)
		} else {
			e.renderTopStatusMessage(s, w)
		}
	}
	sidebarFocused := e.sidebar != nil && e.sidebar.Visible && e.sidebar.Focused
	if e.mode == ModeBranchPicker || e.spaceMenuActive || e.keybindingsHelp.active || sidebarFocused || !cursorVisible {
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

const scrollIndicatorDuration = 1500 * time.Millisecond

func (e *Editor) renderScrollIndicator(s Screen, w, viewHeight int) {
	if viewHeight < 1 || w < 1 {
		return
	}
	if e.fileTreePreviewVisible() {
		return
	}

	e.drawScrollIndicator(s, w-1, 0, viewHeight, e.LineCount(), e.scroll, e.interaction.lastScrollTime)
}

func (e *Editor) drawScrollIndicator(s Screen, x, y, height int, totalLines int, scroll int, lastScroll time.Time) {
	if height < 1 || totalLines <= height {
		return
	}
	if lastScroll.IsZero() {
		return
	}
	elapsed := time.Since(lastScroll)
	if elapsed >= scrollIndicatorDuration {
		return
	}

	// Calculate thumb size (minimum 1 row)
	thumbSize := height * height / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}

	// Calculate thumb position
	maxScroll := totalLines - height
	if maxScroll < 1 {
		maxScroll = 1
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	thumbPos := scroll * (height - thumbSize) / maxScroll
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos+thumbSize > height {
		thumbPos = height - thumbSize
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

	style := e.styleScrollIndicator
	if style == nil {
		style = e.styleLineNumber
	}
	for row := 0; row < height; row++ {
		var ch rune
		if row >= thumbPos && row < thumbPos+thumbSize {
			ch = thumbChar
		} else {
			ch = trackChar
		}
		if ch != ' ' {
			s.SetContent(x, y+row, ch, nil, style)
		}
	}
}
func (e *Editor) drawLine(s Screen, y, w, startX int, line []rune, tabWidth int, selStart, selEnd int, spans []HighlightSpan, highlightActive bool, searchMatches []SearchMatch, lineIdx int, currentMatchIdx int, scrollX int, conflictKind conflictLineKind) {
	col := 0 // visual column (accounting for tabs)
	if tabWidth < 1 {
		tabWidth = 1
	}
	fallbackStyle := e.styleMain
	if highlightActive {
		fallbackStyle = e.styleSyntaxUnknown
	}
	conflictBg, conflictActive := e.conflictBackground(conflictKind)
	if conflictActive {
		fg, _, _ := fallbackStyle.Decompose()
		fallbackStyle = fallbackStyle.Foreground(fg).Background(conflictBg)
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
		if conflictActive {
			fg, _, _ := activeStyle.Decompose()
			activeStyle = activeStyle.Foreground(fg).Background(conflictBg)
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
