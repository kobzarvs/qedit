package editor

import (
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
	for col := contentX; col < contentX+contentW; col++ {
		s.SetContent(col, sepY, '─', nil, e.sidebarStyles.Border)
	}

	// Chat area: y+2 to y+h-4 (leaving space for input)
	chatY := y + 2
	inputHeight := 3
	chatH := h - 2 - inputHeight
	if chatH > 0 {
		e.renderAIPanelChat(s, contentX, chatY, contentW, chatH, panel)
	}

	// Input separator
	inputSepY := y + h - inputHeight
	for col := contentX; col < contentX+contentW; col++ {
		s.SetContent(col, inputSepY, '─', nil, e.sidebarStyles.Border)
	}

	// Input area
	inputY := y + h - inputHeight + 1
	e.renderAIPanelInput(s, contentX, inputY, contentW, inputHeight-1, panel)

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
	// Format: "Provider: [name ▼] Model: [name ▼]"
	providerLabel := "Provider: "
	providerName := panel.ProviderName
	if providerName == "" {
		providerName = "(none)"
	}

	modelLabel := " Model: "
	modelName := panel.ModelName
	if modelName == "" {
		modelName = "(none)"
	}

	// Status indicator
	statusChar := '○' // offline
	statusStyle := e.sidebarStyles.Hidden
	switch panel.Status {
	case AIProviderStatusOnline:
		statusChar = '●'
		statusStyle = e.styleBranchMarker
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

	// Draw provider
	for _, r := range providerLabel {
		if col >= x+w {
			break
		}
		s.SetContent(col, y, r, nil, e.sidebarStyles.Hidden)
		col++
	}
	for _, r := range providerName {
		if col >= x+w {
			break
		}
		style := e.sidebarStyles.Base
		if panel.Focused {
			style = e.styleAutoCompleteHotkey
		}
		s.SetContent(col, y, r, nil, style)
		col++
	}

	// Draw model label and name
	for _, r := range modelLabel {
		if col >= x+w {
			break
		}
		s.SetContent(col, y, r, nil, e.sidebarStyles.Hidden)
		col++
	}

	// Truncate model name if needed
	remaining := x + w - col - 1
	modelRunes := []rune(modelName)
	if len(modelRunes) > remaining && remaining > 3 {
		modelRunes = append(modelRunes[:remaining-3], '.', '.', '.')
	}
	for _, r := range modelRunes {
		if col >= x+w {
			break
		}
		style := e.sidebarStyles.Base
		if panel.Focused {
			style = e.styleAutoCompleteHotkey
		}
		s.SetContent(col, y, r, nil, style)
		col++
	}
}

func (e *Editor) renderAIPanelChat(s Screen, x, y, w, h int, panel *AIPanel) {
	if h <= 0 {
		return
	}

	// Build display lines from messages and streaming text
	type displayLine struct {
		text    string
		isUser  bool
		isLabel bool
	}
	var lines []displayLine

	for _, msg := range panel.Messages {
		// Add role label
		if msg.Role == "user" {
			lines = append(lines, displayLine{text: "You:", isUser: true, isLabel: true})
		} else {
			lines = append(lines, displayLine{text: "AI:", isUser: false, isLabel: true})
		}

		// Wrap message content
		wrapped := wrapText(msg.Content, w-2)
		for _, line := range wrapped {
			lines = append(lines, displayLine{text: line, isUser: msg.Role == "user"})
		}
		// Empty line between messages
		lines = append(lines, displayLine{text: ""})
	}

	// Add streaming text if any
	if panel.State == AIPanelStateStreaming && panel.StreamingText != "" {
		lines = append(lines, displayLine{text: "AI:", isLabel: true})
		wrapped := wrapText(panel.StreamingText, w-2)
		for _, line := range wrapped {
			lines = append(lines, displayLine{text: line})
		}
		// Add blinking cursor indicator
		lines = append(lines, displayLine{text: "▌"})
	} else if panel.State == AIPanelStateWaiting {
		lines = append(lines, displayLine{text: "AI: thinking...", isLabel: true})
	}

	// Calculate scroll
	maxScroll := len(lines) - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	// Auto-scroll to bottom when streaming
	if panel.State == AIPanelStateStreaming || panel.State == AIPanelStateWaiting {
		panel.Scroll = maxScroll
	}
	if panel.Scroll > maxScroll {
		panel.Scroll = maxScroll
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
		style := e.sidebarStyles.Base
		if line.isLabel {
			style = e.styleAutoCompleteHotkey
		} else if line.isUser {
			style = e.sidebarStyles.Dir
		}

		col := x + 1
		for _, r := range line.text {
			if col >= x+w-1 {
				break
			}
			s.SetContent(col, row, r, nil, style)
			col++
		}
	}
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

	col := x
	for _, r := range prompt {
		if col >= x+w {
			break
		}
		s.SetContent(col, y, r, nil, e.styleAutoCompleteHotkey)
		col++
	}

	// Draw input text
	inputStartX := col
	inputW := w - 2

	input := panel.CurrentInput
	cursor := panel.InputCursor

	// Handle scrolling if input is too long
	visibleStart := 0
	if cursor > inputW-1 {
		visibleStart = cursor - inputW + 1
	}

	for i := 0; i < inputW && visibleStart+i < len(input); i++ {
		r := input[visibleStart+i]
		s.SetContent(inputStartX+i, y, r, nil, e.sidebarStyles.Base)
	}

	// Clear rest of input area
	for i := len(input) - visibleStart; i < inputW; i++ {
		if inputStartX+i < x+w {
			s.SetContent(inputStartX+i, y, ' ', nil, e.sidebarStyles.Base)
		}
	}

	// Draw help hint on second line if space
	if h > 1 {
		hint := "Enter=send Esc=close Tab=switch"
		hintCol := x + 1
		for _, r := range hint {
			if hintCol >= x+w-1 {
				break
			}
			s.SetContent(hintCol, y+1, r, nil, e.sidebarStyles.Hidden)
			hintCol++
		}
	}
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
func wrapText(text string, width int) []string {
	if width <= 0 {
		return nil
	}

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}

		// Simple word wrapping
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		currentLine := words[0]
		for _, word := range words[1:] {
			if len(currentLine)+1+len(word) <= width {
				currentLine += " " + word
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}
		lines = append(lines, currentLine)
	}

	return lines
}
