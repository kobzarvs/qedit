package editor

import (
	"strings"
	"time"
)

// AIPanelState represents the state of the AI panel.
type AIPanelState int

const (
	AIPanelStateIdle AIPanelState = iota
	AIPanelStateWaiting
	AIPanelStateStreaming
)

// aiLineStyle represents the style kind for AI panel display lines.
type aiLineStyle int

const (
	aiLineStyleLabel aiLineStyle = iota
	aiLineStyleUser
	aiLineStyleAssistant
	aiLineStyleReasoning
)

// AIPanelMessage represents a message in the AI chat.
type AIPanelMessage struct {
	Role      string    // "user", "assistant"
	Content   string    // Message content
	Timestamp time.Time // When the message was sent/received
}

// aiDisplayLine represents a single display line in the AI panel.
type aiDisplayLine struct {
	text      string
	styleKind aiLineStyle
	highlight bool
}

// AIPanel manages the AI chat panel state.
type AIPanel struct {
	// Visibility
	Visible bool
	Focused bool

	// Dimensions
	Width int // Panel width in columns

	// Provider state
	ProviderName string
	ModelName    string
	Status       AIProviderStatus

	// Chat state
	Messages           []AIPanelMessage
	CurrentInput       []rune // Current prompt input
	InputCursor        int    // Cursor position in input
	StreamingText      string // Currently streaming response text
	StreamingReasoning string // Currently streaming reasoning text
	State              AIPanelState
	ShowReasoning      bool
	ThinkingLevel      string
	Maximized          bool

	// Scroll state
	Scroll         int // Scroll position in chat history
	InputScroll    int // Horizontal scroll in input
	InputHistory   []string
	HistoryIndex   int // -1 = not browsing history
	lastScrollTime time.Time

	// Model selection popup
	ModelSelectActive bool
	ModelSelectItems  []AIModelInfo
	ModelSelectIndex  int

	// Provider selection popup
	ProviderSelectActive bool
	ProviderSelectItems  []ProviderItem
	ProviderSelectIndex  int

	// Current edit suggestion
	CurrentEdit *AIEdit

	// Render cache fields (used by ai_render.go)
	contentVersion     int
	renderCacheVersion int
	renderCacheWidth   int
	renderLines        []aiDisplayLine
	renderHighlights   map[int][]HighlightSpan
	autoScroll         bool
}

// ProviderItem represents a provider in the selection list.
type ProviderItem struct {
	Name        string
	DisplayName string
	Available   bool
	IsCurrent   bool
}

// NewAIPanel creates a new AI panel.
func NewAIPanel() *AIPanel {
	return &AIPanel{
		Visible:       false,
		Focused:       false,
		Width:         50,
		Messages:      make([]AIPanelMessage, 0),
		CurrentInput:  make([]rune, 0),
		State:         AIPanelStateIdle,
		ShowReasoning: true,
		HistoryIndex:  -1,
		autoScroll:    true,
	}
}

// Toggle toggles the panel visibility.
func (p *AIPanel) Toggle() {
	if p.Visible {
		p.Close()
	} else {
		p.Open()
	}
}

// Open opens the panel.
func (p *AIPanel) Open() {
	p.Visible = true
	p.Focused = true
}

// Close closes the panel.
func (p *AIPanel) Close() {
	p.Visible = false
	p.Focused = false
}

// FocusToggle toggles focus between panel and editor.
func (p *AIPanel) FocusToggle() {
	if p.Visible {
		p.Focused = !p.Focused
	}
}

// AddUserMessage adds a user message to the chat.
func (p *AIPanel) AddUserMessage(content string) {
	p.Messages = append(p.Messages, AIPanelMessage{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	})
	p.markDirty()
}

// AddAssistantMessage adds an assistant message to the chat.
func (p *AIPanel) AddAssistantMessage(content string) {
	p.Messages = append(p.Messages, AIPanelMessage{
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
	})
	p.markDirty()
}

// AddReasoningMessage adds a reasoning/thinking message to the chat.
func (p *AIPanel) AddReasoningMessage(content string) {
	p.Messages = append(p.Messages, AIPanelMessage{
		Role:      "reasoning",
		Content:   content,
		Timestamp: time.Now(),
	})
	p.markDirty()
}

// AppendToStreaming appends text to the current streaming response.
func (p *AIPanel) AppendToStreaming(text string) {
	p.StreamingText += text
	p.markDirty()
}

// AppendToReasoning appends text to the current streaming reasoning.
func (p *AIPanel) AppendToReasoning(text string) {
	p.StreamingReasoning += text
	p.markDirty()
}

// FinalizeStreaming converts streaming text to a message.
func (p *AIPanel) FinalizeStreaming() {
	if p.StreamingText != "" {
		p.AddAssistantMessage(p.StreamingText)
		p.StreamingText = ""
	}
	p.State = AIPanelStateIdle
	p.markDirty()
}

// FinalizeReasoning converts streaming reasoning text to a message.
func (p *AIPanel) FinalizeReasoning() {
	if p.StreamingReasoning != "" {
		p.AddReasoningMessage(p.StreamingReasoning)
		p.StreamingReasoning = ""
	}
	p.markDirty()
}

// ToggleReasoning toggles the visibility of reasoning messages.
func (p *AIPanel) ToggleReasoning() bool {
	p.ShowReasoning = !p.ShowReasoning
	p.markDirty()
	return p.ShowReasoning
}

// ToggleMaximize toggles the maximized state of the AI panel.
func (p *AIPanel) ToggleMaximize() bool {
	p.Maximized = !p.Maximized
	p.markDirty()
	return p.Maximized
}

// SetThinkingLevel sets the thinking level for the AI.
func (p *AIPanel) SetThinkingLevel(level string) bool {
	level = strings.TrimSpace(strings.ToLower(level))
	if level == "" {
		return false
	}
	p.ThinkingLevel = level
	p.markDirty()
	return true
}

// ClearInput clears the input field.
func (p *AIPanel) ClearInput() {
	p.CurrentInput = p.CurrentInput[:0]
	p.InputCursor = 0
}

// GetInput returns the current input as a string.
func (p *AIPanel) GetInput() string {
	return string(p.CurrentInput)
}

// SetInput sets the input field content.
func (p *AIPanel) SetInput(text string) {
	p.CurrentInput = []rune(text)
	p.InputCursor = len(p.CurrentInput)
}

// InsertRune inserts a rune at the cursor position.
func (p *AIPanel) InsertRune(r rune) {
	p.CurrentInput = append(p.CurrentInput[:p.InputCursor], append([]rune{r}, p.CurrentInput[p.InputCursor:]...)...)
	p.InputCursor++
}

// Backspace deletes the character before the cursor.
func (p *AIPanel) Backspace() {
	if p.InputCursor > 0 {
		p.CurrentInput = append(p.CurrentInput[:p.InputCursor-1], p.CurrentInput[p.InputCursor:]...)
		p.InputCursor--
	}
}

// Delete deletes the character at the cursor.
func (p *AIPanel) Delete() {
	if p.InputCursor < len(p.CurrentInput) {
		p.CurrentInput = append(p.CurrentInput[:p.InputCursor], p.CurrentInput[p.InputCursor+1:]...)
	}
}

// MoveCursorLeft moves the cursor left.
func (p *AIPanel) MoveCursorLeft() {
	if p.InputCursor > 0 {
		p.InputCursor--
	}
}

// MoveCursorRight moves the cursor right.
func (p *AIPanel) MoveCursorRight() {
	if p.InputCursor < len(p.CurrentInput) {
		p.InputCursor++
	}
}

// MoveCursorHome moves the cursor to the beginning.
func (p *AIPanel) MoveCursorHome() {
	p.InputCursor = 0
}

// MoveCursorEnd moves the cursor to the end.
func (p *AIPanel) MoveCursorEnd() {
	p.InputCursor = len(p.CurrentInput)
}

// SaveInputToHistory saves the current input to history.
func (p *AIPanel) SaveInputToHistory() {
	input := strings.TrimSpace(string(p.CurrentInput))
	if input == "" {
		return
	}
	// Don't add if same as last entry
	if len(p.InputHistory) > 0 && p.InputHistory[len(p.InputHistory)-1] == input {
		return
	}
	p.InputHistory = append(p.InputHistory, input)
	// Keep last 100 entries
	if len(p.InputHistory) > 100 {
		p.InputHistory = p.InputHistory[len(p.InputHistory)-100:]
	}
}

// HistoryUp navigates to older input.
func (p *AIPanel) HistoryUp() {
	if len(p.InputHistory) == 0 {
		return
	}
	if p.HistoryIndex == -1 {
		p.HistoryIndex = len(p.InputHistory)
	}
	if p.HistoryIndex > 0 {
		p.HistoryIndex--
		p.SetInput(p.InputHistory[p.HistoryIndex])
	}
}

// HistoryDown navigates to newer input.
func (p *AIPanel) HistoryDown() {
	if p.HistoryIndex == -1 {
		return
	}
	p.HistoryIndex++
	if p.HistoryIndex >= len(p.InputHistory) {
		p.HistoryIndex = -1
		p.ClearInput()
	} else {
		p.SetInput(p.InputHistory[p.HistoryIndex])
	}
}

// ScrollUp scrolls the chat history up.
func (p *AIPanel) ScrollUp(lines int) {
	p.autoScroll = false
	p.lastScrollTime = time.Now()
	p.Scroll -= lines
	if p.Scroll < 0 {
		p.Scroll = 0
	}
}

// ScrollDown scrolls the chat history down.
func (p *AIPanel) ScrollDown(lines int) {
	p.autoScroll = false
	p.lastScrollTime = time.Now()
	p.Scroll += lines
	// Will be clamped during render
}

// OpenProviderSelect opens the provider selection popup.
func (p *AIPanel) OpenProviderSelect(items []ProviderItem) {
	p.ProviderSelectActive = true
	p.ProviderSelectItems = items
	p.ProviderSelectIndex = 0
	// Find current provider
	for i, item := range items {
		if item.IsCurrent {
			p.ProviderSelectIndex = i
			break
		}
	}
}

// CloseProviderSelect closes the provider selection popup.
func (p *AIPanel) CloseProviderSelect() {
	p.ProviderSelectActive = false
	p.ProviderSelectItems = nil
}

// OpenModelSelect opens the model selection popup.
func (p *AIPanel) OpenModelSelect(models []AIModelInfo, currentModel string) {
	p.ModelSelectActive = true
	p.ModelSelectItems = models
	p.ModelSelectIndex = 0
	// Find current model
	for i, m := range models {
		if m.ID == currentModel {
			p.ModelSelectIndex = i
			break
		}
	}
}

// CloseModelSelect closes the model selection popup.
func (p *AIPanel) CloseModelSelect() {
	p.ModelSelectActive = false
	p.ModelSelectItems = nil
}

// ClearChat clears the chat history.
func (p *AIPanel) ClearChat() {
	p.Messages = p.Messages[:0]
	p.StreamingText = ""
	p.Scroll = 0
	p.markDirty()
}

// SetError sets an error message.
func (p *AIPanel) SetError(err error) {
	if err != nil {
		p.AddAssistantMessage("Error: " + err.Error())
	}
	p.State = AIPanelStateIdle
	p.StreamingText = ""
	p.markDirty()
}

// SetState sets the panel state.
func (p *AIPanel) SetState(state AIPanelState) {
	p.State = state
	p.markDirty()
}

// CycleThinkingLevel cycles through the provided thinking levels and returns the new level.
// If the current level is not in the list, it returns the first level.
func (p *AIPanel) CycleThinkingLevel(levels []string) string {
	if len(levels) == 0 {
		return p.ThinkingLevel
	}
	current := p.ThinkingLevel
	for i, level := range levels {
		if level == current {
			// Return the next level, wrapping around
			p.ThinkingLevel = levels[(i+1)%len(levels)]
			p.markDirty()
			return p.ThinkingLevel
		}
	}
	// Current level not found, set to first level
	p.ThinkingLevel = levels[0]
	p.markDirty()
	return p.ThinkingLevel
}

// markDirty invalidates the render cache by incrementing the content version.
func (p *AIPanel) markDirty() {
	p.contentVersion++
}

// ExportToMarkdown exports the entire conversation as markdown text.
// This includes all messages and reasoning sections formatted as markdown.
func (p *AIPanel) ExportToMarkdown() string {
	if p == nil {
		return ""
	}

	var sb strings.Builder

	// First finalize any streaming content
	if p.StreamingReasoning != "" {
		p.AddReasoningMessage(p.StreamingReasoning)
		p.StreamingReasoning = ""
	}
	if p.StreamingText != "" {
		p.AddAssistantMessage(p.StreamingText)
		p.StreamingText = ""
	}

	for i, msg := range p.Messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}

		switch msg.Role {
		case "user":
			sb.WriteString("## User\n\n")
			sb.WriteString(msg.Content)
		case "assistant":
			sb.WriteString("## Assistant\n\n")
			sb.WriteString(msg.Content)
		case "reasoning":
			sb.WriteString("## Thinking\n\n")
			sb.WriteString("```\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n```")
		}
	}

	return sb.String()
}

// ImportFromMarkdown imports a conversation from markdown text.
// It parses sections marked with ## User, ## Assistant, and ## Thinking.
func (p *AIPanel) ImportFromMarkdown(text string) error {
	if p == nil {
		return nil
	}

	// Clear existing messages
	p.Messages = p.Messages[:0]

	lines := strings.Split(text, "\n")
	var currentRole string
	var currentContent strings.Builder
	var inFence bool
	var fenceMarker string

	flushMessage := func() {
		if currentRole != "" && currentContent.Len() > 0 {
			content := strings.TrimSpace(currentContent.String())
			if content != "" {
				switch currentRole {
				case "user":
					p.AddUserMessage(content)
				case "assistant":
					p.AddAssistantMessage(content)
				case "reasoning":
					p.AddReasoningMessage(content)
				}
			}
		}
		currentContent.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for section headers
		if strings.HasPrefix(trimmed, "## ") {
			flushMessage()
			section := strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			switch section {
			case "user":
				currentRole = "user"
			case "assistant":
				currentRole = "assistant"
			case "thinking", "reasoning":
				currentRole = "reasoning"
			default:
				// Unknown section, treat as assistant
				currentRole = "assistant"
			}
			continue
		}

		// Handle code fences (for thinking sections)
		if isFenceLine(line) && currentRole == "reasoning" {
			trimmedLeft := strings.TrimLeft(line, " \t")
			marker := trimmedLeft[:3]
			if !inFence {
				inFence = true
				fenceMarker = marker
				continue // Skip the opening fence line
			} else if strings.HasPrefix(trimmedLeft, fenceMarker) {
				inFence = false
				fenceMarker = ""
				continue // Skip the closing fence line
			}
		}

		// Accumulate content
		if currentRole != "" {
			if currentContent.Len() > 0 {
				currentContent.WriteString("\n")
			}
			currentContent.WriteString(line)
		}
	}

	flushMessage()
	return nil
}
