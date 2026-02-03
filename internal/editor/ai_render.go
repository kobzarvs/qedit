package editor

import (
	"strconv"
	"strings"
)

// renderAIPanel renders the AI panel on the right side of the screen.
func (e *Editor) renderAIPanel(s Screen, x, y, w, h int) {
	if e.aiPanel == nil || !e.aiPanel.Visible || w <= 0 || h <= 0 {
		return
	}

	panel := e.aiPanel

	// Fill background
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			s.SetContent(col, row, ' ', nil, e.sidebarStyles.Base)
		}
	}

	// Draw left border
	for row := y; row < y+h; row++ {
		s.SetContent(x, row, '│', nil, e.sidebarStyles.Border)
	}

	contentX := x + 1
	contentW := w - 1
	if contentW <= 0 {
		return
	}

	// Header: Provider and Model
	headerY := y
	e.renderAIPanelHeader(s, contentX, headerY, contentW, panel)

	// Separator
	sepY := y + 1
	if h < 2 {
		return
	}
	for col := contentX; col < contentX+contentW; col++ {
		s.SetContent(col, sepY, '─', nil, e.sidebarStyles.Border)
	}

	// Chat area: y+2 to y+h-4 (leaving space for input)
	chatY := y + 2
	inputAreaHeight := 4
	maxInputArea := h - 2
	if maxInputArea <= 0 {
		return
	}
	if inputAreaHeight > maxInputArea {
		inputAreaHeight = maxInputArea
	}
	inputHeight := inputAreaHeight + 1
	chatH := h - 2 - inputHeight
	if chatH > 0 {
		e.renderAIPanelChat(s, contentX, chatY, contentW, chatH, panel)
	}

	// Input separator
	inputSepY := y + h - inputHeight
	if inputSepY >= y && inputSepY < y+h {
		for col := contentX; col < contentX+contentW; col++ {
			s.SetContent(col, inputSepY, '─', nil, e.sidebarStyles.Border)
		}
	}

	// Input area
	inputY := y + h - inputHeight + 1
	e.renderAIPanelInput(s, contentX, inputY, contentW, inputAreaHeight, panel)

	// Model selection popup
	if panel.ModelSelectActive {
		e.renderAIPanelModelSelect(s, contentX, y+2, contentW, chatH, panel)
	}

	// Provider selection popup
	if panel.ProviderSelectActive {
		e.renderAIPanelProviderSelect(s, contentX, y+2, contentW, chatH, panel)
	}
}

func (e *Editor) renderAIPanelHeader(s Screen, x, y, w int, panel *AIPanel) {
	providerName := panel.ProviderName
	if providerName == "" {
		providerName = "(none)"
	}

	modelName := panel.ModelName
	if modelName == "" {
		modelName = "(none)"
	}

	thinking := panel.ThinkingLevel
	if thinking == "" {
		thinking = "auto"
	}
	thinkText := "think:" + thinking

	// Status indicator
	statusChar := '○' // offline
	statusStyle := e.sidebarStyles.Hidden
	switch panel.Status {
	case AIProviderStatusOnline:
		statusChar = '●'
		statusStyle = e.styleAIStatusOnline
	case AIProviderStatusConnecting:
		statusChar = '◐'
		statusStyle = e.sidebarStyles.Hidden
	case AIProviderStatusError:
		statusChar = '○'
		statusStyle = e.styleSearchMatch
	}

	// Draw status indicator
	col := x
	s.SetContent(col, y, statusChar, nil, statusStyle)
	col++
	s.SetContent(col, y, ' ', nil, e.sidebarStyles.Base)
	col++

	providerStyle := e.styleAIHeader
	if panel.Focused {
		providerStyle = e.styleAutoCompleteHotkey
	}
	modelStyle := providerStyle
	thinkingStyle := e.styleAIReasoning
	if thinkingStyle == nil {
		thinkingStyle = e.sidebarStyles.Hidden
	}
	separatorStyle := e.sidebarStyles.Hidden
	separator := " | "

	segments := []struct {
		text  string
		style Style
	}{
		{text: providerName, style: providerStyle},
		{text: modelName, style: modelStyle},
		{text: thinkText, style: thinkingStyle},
	}

	for i, segment := range segments {
		if col >= x+w {
			break
		}
		if i > 0 {
			for _, r := range separator {
				if col >= x+w {
					return
				}
				s.SetContent(col, y, r, nil, separatorStyle)
				col++
			}
		}
		remaining := x + w - col
		if remaining <= 0 {
			break
		}
		text := truncateWithEllipsis(segment.text, remaining)
		for _, r := range text {
			if col >= x+w {
				return
			}
			s.SetContent(col, y, r, nil, segment.style)
			col++
		}
		if len(text) < len([]rune(segment.text)) {
			break
		}
	}
}

func (e *Editor) renderAIPanelChat(s Screen, x, y, w, h int, panel *AIPanel) {
	if h <= 0 {
		return
	}

	lines, highlights := e.aiPanelRenderLines(panel, w)

	// Calculate scroll
	maxScroll := len(lines) - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	// Auto-scroll to bottom when streaming unless user scrolled
	if panel.State == AIPanelStateStreaming || panel.State == AIPanelStateWaiting {
		if panel.autoScroll {
			panel.Scroll = maxScroll
		}
	} else if panel.Scroll > maxScroll {
		panel.Scroll = maxScroll
	}
	if panel.Scroll >= maxScroll {
		panel.autoScroll = true
	}

	// Render visible lines
	for i := 0; i < h; i++ {
		lineIdx := panel.Scroll + i
		row := y + i

		// Clear line
		for col := x; col < x+w; col++ {
			s.SetContent(col, row, ' ', nil, e.sidebarStyles.Base)
		}

		if lineIdx >= len(lines) {
			continue
		}

		line := lines[lineIdx]
		style := e.aiPanelLineStyle(line)
		lineSpans := highlights[lineIdx]
		e.drawAIPanelLine(s, x+1, row, w-2, line, style, lineSpans)
	}

	e.drawScrollIndicator(s, x+w-1, y, h, len(lines), panel.Scroll, panel.lastScrollTime)
}

func (e *Editor) renderAIPanelInput(s Screen, x, y, w, h int, panel *AIPanel) {
	if h <= 0 {
		return
	}

	// Draw prompt indicator
	prompt := "> "
	if panel.State == AIPanelStateStreaming || panel.State == AIPanelStateWaiting {
		prompt = "⟳ "
	}

	promptRunes := []rune(prompt)
	promptWidth := len(promptRunes)
	showLineNumbers := panel.Focused
	lineNumWidth := 0
	gutterWidth := 0

	inputW := w - promptWidth
	if inputW < 1 {
		return
	}

	input := panel.CurrentInput
	cursor := panel.InputCursor
	lines, cursorLine, cursorCol := wrapInputLines(input, cursor, inputW)

	if showLineNumbers {
		maxLine := len(lines)
		if maxLine < 1 {
			maxLine = 1
		}
		lineNumWidth = len(strconv.Itoa(maxLine))
		gutterWidth = lineNumWidth + 1
		inputW = w - promptWidth - gutterWidth
		if inputW < 1 {
			inputW = 1
		}
		lines, cursorLine, cursorCol = wrapInputLines(input, cursor, inputW)
		if maxLine = len(lines); maxLine < 1 {
			maxLine = 1
		}
		if newWidth := len(strconv.Itoa(maxLine)); newWidth != lineNumWidth {
			lineNumWidth = newWidth
			gutterWidth = lineNumWidth + 1
			inputW = w - promptWidth - gutterWidth
			if inputW < 1 {
				inputW = 1
			}
			lines, cursorLine, cursorCol = wrapInputLines(input, cursor, inputW)
		}
	}

	if len(lines) == 0 {
		lines = []string{""}
		cursorLine = 0
		cursorCol = 0
	}

	start := 0
	if cursorLine >= h {
		start = cursorLine - h + 1
	}
	if start < 0 {
		start = 0
	}

	lineNumStyle := e.sidebarStyles.Hidden
	for i := 0; i < h; i++ {
		row := y + i
		// Clear line
		for col := x; col < x+w; col++ {
			s.SetContent(col, row, ' ', nil, e.sidebarStyles.Base)
		}

		lineIdx := start + i
		if lineIdx >= len(lines) {
			continue
		}
		col := x
		if showLineNumbers {
			num := strconv.Itoa(lineIdx + 1)
			for j := 0; j < lineNumWidth-len(num); j++ {
				if col >= x+w {
					break
				}
				s.SetContent(col, row, ' ', nil, lineNumStyle)
				col++
			}
			for _, r := range num {
				if col >= x+w {
					break
				}
				s.SetContent(col, row, r, nil, lineNumStyle)
				col++
			}
			if col < x+gutterWidth {
				s.SetContent(col, row, ' ', nil, lineNumStyle)
				col++
			}
		}
		if lineIdx == 0 {
			for _, r := range promptRunes {
				if col >= x+w {
					break
				}
				s.SetContent(col, row, r, nil, e.styleAutoCompleteHotkey)
				col++
			}
		} else {
			for j := 0; j < promptWidth; j++ {
				if col >= x+w {
					break
				}
				s.SetContent(col, row, ' ', nil, e.sidebarStyles.Base)
				col++
			}
		}
		for _, r := range []rune(lines[lineIdx]) {
			if col >= x+w {
				break
			}
			s.SetContent(col, row, r, nil, e.sidebarStyles.Base)
			col++
		}
	}

	if panel.Focused {
		if cursorLine >= start && cursorLine < start+h {
			e.aiInputCursorX = x + gutterWidth + promptWidth + cursorCol
			e.aiInputCursorY = y + (cursorLine - start)
			if e.aiInputCursorX >= x+w {
				e.aiInputCursorX = x + w - 1
			}
			e.aiInputCursorVisible = true
		}
	}

	if len(input) == 0 && h > 1 {
		hint := "Enter=send Esc=cancel Tab=providers Shift+Tab=models Ctrl+O=reason Ctrl+T=think"
		hintRow := y + h - 1
		hintCol := x + gutterWidth + promptWidth
		for _, r := range hint {
			if hintCol >= x+w-1 {
				break
			}
			s.SetContent(hintCol, hintRow, r, nil, e.sidebarStyles.Hidden)
			hintCol++
		}
	}
}

func wrapInputLines(input []rune, cursor int, width int) ([]string, int, int) {
	if width <= 0 {
		return nil, 0, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(input) {
		cursor = len(input)
	}

	lines := []string{""}
	lineIdx := 0
	col := 0
	cursorLine := 0
	cursorCol := 0

	for i, r := range input {
		if i == cursor {
			cursorLine = lineIdx
			cursorCol = col
		}
		if r == '\n' {
			lines = append(lines, "")
			lineIdx++
			col = 0
			continue
		}
		if col == width {
			lines = append(lines, "")
			lineIdx++
			col = 0
		}
		lines[lineIdx] += string(r)
		col++
	}
	if cursor == len(input) {
		cursorLine = lineIdx
		cursorCol = col
	}

	return lines, cursorLine, cursorCol
}

func aiInputIndexForLineCol(lines []string, lineIdx int, col int) int {
	if lineIdx < 0 {
		return 0
	}
	if lineIdx >= len(lines) {
		lineIdx = len(lines) - 1
		if lineIdx < 0 {
			return 0
		}
	}
	if col < 0 {
		col = 0
	}
	idx := 0
	for i := 0; i < lineIdx; i++ {
		idx += len([]rune(lines[i]))
	}
	lineRunes := []rune(lines[lineIdx])
	if col > len(lineRunes) {
		col = len(lineRunes)
	}
	idx += col
	return idx
}

func (e *Editor) aiPanelRenderLines(panel *AIPanel, width int) ([]aiDisplayLine, map[int][]HighlightSpan) {
	contentWidth := width - 2
	if contentWidth < 1 {
		return nil, nil
	}

	if panel.renderCacheWidth == contentWidth && panel.renderCacheVersion == panel.contentVersion {
		return panel.renderLines, panel.renderHighlights
	}

	lines := buildAIPanelLines(panel, contentWidth)
	var highlights map[int][]HighlightSpan
	highlightReady := panel.State == AIPanelStateIdle && panel.StreamingText == "" && panel.StreamingReasoning == ""
	if highlightReady && e.aiMarkdownHighlight != nil && hasHighlightableLines(lines) {
		markdownText := joinAIPanelLines(lines)
		highlights = e.aiMarkdownHighlight(markdownText)
	}

	panel.renderLines = lines
	panel.renderHighlights = highlights
	panel.renderCacheVersion = panel.contentVersion
	panel.renderCacheWidth = contentWidth

	return lines, highlights
}

func buildAIPanelLines(panel *AIPanel, width int) []aiDisplayLine {
	var lines []aiDisplayLine

	appendMessage := func(label string, style aiLineStyle, content string, highlight bool) {
		lines = append(lines, aiDisplayLine{text: label, styleKind: aiLineStyleLabel, highlight: false})
		wrapped := wrapText(content, width)
		for _, line := range wrapped {
			lines = append(lines, aiDisplayLine{text: line, styleKind: style, highlight: highlight})
		}
		lines = append(lines, aiDisplayLine{text: "", styleKind: aiLineStyleAssistant})
	}

	for _, msg := range panel.Messages {
		switch msg.Role {
		case "user":
			appendMessage("You:", aiLineStyleUser, msg.Content, false)
		case "reasoning":
			if !panel.ShowReasoning {
				continue
			}
			appendMessage("Think:", aiLineStyleReasoning, msg.Content, false)
		default:
			appendMessage("AI:", aiLineStyleAssistant, msg.Content, true)
		}
	}

	if panel.ShowReasoning && panel.StreamingReasoning != "" {
		lines = append(lines, aiDisplayLine{text: "Think:", styleKind: aiLineStyleLabel})
		wrapped := wrapText(panel.StreamingReasoning, width)
		for _, line := range wrapped {
			lines = append(lines, aiDisplayLine{text: line, styleKind: aiLineStyleReasoning, highlight: false})
		}
		if panel.StreamingText != "" {
			lines = append(lines, aiDisplayLine{text: "", styleKind: aiLineStyleAssistant})
		}
	}

	if panel.State == AIPanelStateStreaming && panel.StreamingText != "" {
		lines = append(lines, aiDisplayLine{text: "AI:", styleKind: aiLineStyleLabel})
		wrapped := wrapText(panel.StreamingText, width)
		for _, line := range wrapped {
			lines = append(lines, aiDisplayLine{text: line, styleKind: aiLineStyleAssistant, highlight: true})
		}
		lines = append(lines, aiDisplayLine{text: "▌", styleKind: aiLineStyleAssistant})
	} else if panel.State == AIPanelStateWaiting {
		lines = append(lines, aiDisplayLine{text: "AI: thinking...", styleKind: aiLineStyleLabel})
	}

	return lines
}

func hasHighlightableLines(lines []aiDisplayLine) bool {
	for _, line := range lines {
		if line.highlight {
			return true
		}
	}
	return false
}

func joinAIPanelLines(lines []aiDisplayLine) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line.text)
	}
	return b.String()
}

func (e *Editor) aiPanelLineStyle(line aiDisplayLine) Style {
	switch line.styleKind {
	case aiLineStyleLabel:
		return e.styleAutoCompleteHotkey
	case aiLineStyleUser:
		if e.styleAIUser != nil {
			return e.styleAIUser
		}
		return e.sidebarStyles.Dir
	case aiLineStyleReasoning:
		if e.styleAIThinking != nil {
			return e.styleAIThinking
		}
		if e.styleAIReasoning != nil {
			return e.styleAIReasoning
		}
		return e.styleSyntaxComment
	default:
		if e.styleAIAssistant != nil {
			return e.styleAIAssistant
		}
		return e.sidebarStyles.Base
	}
}

func (e *Editor) drawAIPanelLine(s Screen, x, y, w int, line aiDisplayLine, baseStyle Style, spans []HighlightSpan) {
	col := 0
	for idx, r := range []rune(line.text) {
		if col >= w {
			break
		}
		style := baseStyle
		if line.highlight && len(spans) > 0 {
			if kind, ok := highlightKindAt(spans, idx); ok {
				if hlStyle, ok := e.styleForHighlight(kind); ok {
					fg, _, _ := hlStyle.Decompose()
					_, baseBg, _ := baseStyle.Decompose()
					style = hlStyle.Foreground(fg).Background(baseBg)
				}
			}
		}
		s.SetContent(x+col, y, r, nil, style)
		col++
	}
}

func truncateWithEllipsis(text string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(append(runes[:max-3], '.', '.', '.'))
}

func (e *Editor) renderAIPanelModelSelect(s Screen, x, y, w, h int, panel *AIPanel) {
	items := panel.ModelSelectItems
	if len(items) == 0 {
		return
	}

	// Calculate popup dimensions
	popupH := len(items) + 2
	if popupH > h {
		popupH = h
	}
	popupW := w - 2
	if popupW > 40 {
		popupW = 40
	}

	// Center popup
	popupX := x + (w-popupW)/2
	popupY := y + 1

	// Draw border
	e.drawPopupBox(s, popupX, popupY, popupW, popupH, "Select Model")

	// Draw items
	listH := popupH - 2
	for i := 0; i < listH; i++ {
		if i >= len(items) {
			break
		}
		item := items[i]
		row := popupY + 1 + i
		isSelected := i == panel.ModelSelectIndex

		style := e.sidebarStyles.Base
		if isSelected {
			style = e.styleSelection
		}

		// Clear row
		for col := popupX + 1; col < popupX+popupW-1; col++ {
			s.SetContent(col, row, ' ', nil, style)
		}

		// Draw model name
		col := popupX + 2
		name := item.ID
		if len(name) > popupW-4 {
			name = name[:popupW-7] + "..."
		}
		for _, r := range name {
			if col >= popupX+popupW-1 {
				break
			}
			s.SetContent(col, row, r, nil, style)
			col++
		}
	}
}

func (e *Editor) renderAIPanelProviderSelect(s Screen, x, y, w, h int, panel *AIPanel) {
	items := panel.ProviderSelectItems
	if len(items) == 0 {
		return
	}

	// Calculate popup dimensions
	popupH := len(items) + 2
	if popupH > h {
		popupH = h
	}
	popupW := w - 2
	if popupW > 40 {
		popupW = 40
	}

	// Center popup
	popupX := x + (w-popupW)/2
	popupY := y + 1

	// Draw border
	e.drawPopupBox(s, popupX, popupY, popupW, popupH, "Select Provider")

	// Draw items
	listH := popupH - 2
	for i := 0; i < listH; i++ {
		if i >= len(items) {
			break
		}
		item := items[i]
		row := popupY + 1 + i
		isSelected := i == panel.ProviderSelectIndex

		style := e.sidebarStyles.Base
		if !item.Available {
			style = e.sidebarStyles.Hidden
		}
		if isSelected {
			style = e.styleSelection
		}

		// Clear row
		for col := popupX + 1; col < popupX+popupW-1; col++ {
			s.SetContent(col, row, ' ', nil, style)
		}

		// Draw indicator
		col := popupX + 1
		if item.IsCurrent {
			s.SetContent(col, row, '*', nil, style)
		}
		col += 2

		// Draw provider name
		name := item.DisplayName
		if len(name) > popupW-6 {
			name = name[:popupW-9] + "..."
		}
		for _, r := range name {
			if col >= popupX+popupW-1 {
				break
			}
			s.SetContent(col, row, r, nil, style)
			col++
		}

		// Draw availability indicator
		if !item.Available {
			indicator := " (offline)"
			for _, r := range indicator {
				if col >= popupX+popupW-1 {
					break
				}
				s.SetContent(col, row, r, nil, style)
				col++
			}
		}
	}
}

func (e *Editor) drawPopupBox(s Screen, x, y, w, h int, title string) {
	style := e.styleBoxBorder
	if style == nil {
		style = e.sidebarStyles.Border
	}

	// Draw border
	s.SetContent(x, y, '┌', nil, style)
	s.SetContent(x+w-1, y, '┐', nil, style)
	s.SetContent(x, y+h-1, '└', nil, style)
	s.SetContent(x+w-1, y+h-1, '┘', nil, style)

	for col := x + 1; col < x+w-1; col++ {
		s.SetContent(col, y, '─', nil, style)
		s.SetContent(col, y+h-1, '─', nil, style)
	}
	for row := y + 1; row < y+h-1; row++ {
		s.SetContent(x, row, '│', nil, style)
		s.SetContent(x+w-1, row, '│', nil, style)
	}

	// Draw title
	if title != "" {
		titleRunes := []rune(title)
		titleX := x + 1
		for i, r := range titleRunes {
			if titleX+i >= x+w-1 {
				break
			}
			s.SetContent(titleX+i, y, r, nil, style)
		}
	}

	// Fill interior
	for row := y + 1; row < y+h-1; row++ {
		for col := x + 1; col < x+w-1; col++ {
			s.SetContent(col, row, ' ', nil, e.sidebarStyles.Base)
		}
	}
}

// wrapText wraps text to fit within the given width.
// It preserves fenced code blocks (``` or ~~~) without word-wrapping.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return nil
	}

	var lines []string
	inFence := false
	fenceMarker := "" // tracks "```" or "~~~"
	fenceContent := []string{}

	flushFence := func() {
		if len(fenceContent) > 0 {
			lines = append(lines, fenceContent...)
			fenceContent = fenceContent[:0]
		}
	}

	for _, paragraph := range strings.Split(text, "\n") {
		// Check if this line is a fence line
		if isFenceLine(paragraph) {
			trimmedLeft := strings.TrimLeft(paragraph, " \t")
			marker := trimmedLeft[:3] // "```" or "~~~"

			if !inFence {
				// Opening fence - flush any pending content first
				flushFence()
				inFence = true
				fenceMarker = marker
				fenceContent = append(fenceContent, paragraph)
			} else if strings.HasPrefix(trimmedLeft, fenceMarker) {
				// Closing fence (must match same marker type)
				fenceContent = append(fenceContent, paragraph)
				flushFence()
				inFence = false
				fenceMarker = ""
			} else {
				// Inside fence but line looks like a fence (e.g., demonstrating markdown)
				fenceContent = append(fenceContent, paragraph)
			}
			continue
		}

		// If inside a fence, accumulate content without wrapping
		if inFence {
			fenceContent = append(fenceContent, paragraph)
			continue
		}

		// Outside fence: apply word wrapping
		wrapped := wrapLinePreserve(paragraph, width)
		lines = append(lines, wrapped...)
	}

	// Handle case where fence wasn't closed (streaming content)
	if inFence && len(fenceContent) > 0 {
		lines = append(lines, fenceContent...)
	}

	return lines
}

func wrapLinePreserve(line string, width int) []string {
	if line == "" {
		return []string{""}
	}

	runes := []rune(line)
	if len(runes) <= width {
		return []string{line}
	}

	indentLen := 0
	for indentLen < len(runes) {
		r := runes[indentLen]
		if r != ' ' && r != '\t' {
			break
		}
		indentLen++
	}
	indent := string(runes[:indentLen])

	var lines []string
	start := 0
	first := true
	for start < len(runes) {
		avail := width
		if !first && indentLen > 0 && indentLen < width {
			avail = width - indentLen
		}
		if avail <= 0 {
			avail = width
		}
		if len(runes)-start <= avail {
			chunk := string(runes[start:])
			if !first && indentLen > 0 {
				chunk = indent + strings.TrimLeft(chunk, " \t")
			}
			lines = append(lines, chunk)
			break
		}

		breakAt := start + avail
		for i := start + avail; i > start; i-- {
			if runes[i-1] == ' ' || runes[i-1] == '\t' {
				breakAt = i - 1
				break
			}
		}
		if breakAt <= start {
			breakAt = start + avail
		}
		chunk := string(runes[start:breakAt])
		if !first && indentLen > 0 {
			chunk = indent + strings.TrimLeft(chunk, " \t")
		}
		lines = append(lines, chunk)
		start = breakAt
		for start < len(runes) && (runes[start] == ' ' || runes[start] == '\t') {
			start++
		}
		first = false
	}

	return lines
}
