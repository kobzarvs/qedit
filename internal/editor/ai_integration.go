package editor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// toggleAIPanel toggles the AI panel visibility.
func (e *Editor) toggleAIPanel() {
	if e.aiPanel == nil {
		e.aiPanel = NewAIPanel()
	}
	e.aiPanel.Toggle()

	// Update panel state from manager
	if e.aiPanel.Visible && e.aiManager != nil {
		e.syncAIPanelProviderState()
	}
}

// sendToAI sends the current context to the AI.
func (e *Editor) sendToAI() {
	if e.aiPanel == nil {
		e.aiPanel = NewAIPanel()
	}

	// Open panel if not visible
	if !e.aiPanel.Visible {
		e.aiPanel.Open()
	}

	if e.aiManager == nil {
		e.setStatus("AI not configured")
		return
	}

	// Build context
	ctx := e.buildAIContext()

	// If panel has no input, just focus it
	input := strings.TrimSpace(e.aiPanel.GetInput())
	if input == "" {
		e.aiPanel.Focused = true
		return
	}

	// Send to AI
	e.aiPanel.State = AIPanelStateWaiting
	e.aiPanel.autoScroll = true
	e.aiPanel.AddUserMessage(input)
	e.aiPanel.SaveInputToHistory()
	e.aiPanel.ClearInput()

	if err := e.aiManager.Send(ctx, input); err != nil {
		e.aiPanel.SetError(err)
		return
	}

	e.aiPanel.State = AIPanelStateStreaming
}

// buildAIContext builds the context for AI requests.
func (e *Editor) buildAIContext() AIContext {
	ctx := AIContext{
		FilePath:  e.filename,
		CursorRow: e.cursor.Row,
		CursorCol: e.cursor.Col,
	}

	// Detect language from filename
	if e.filename != "" {
		ext := strings.ToLower(filepath.Ext(e.filename))
		ctx.Language = extToLanguage(ext)
	}

	// Get content - selection or entire file
	if e.selectionActive {
		ctx.Content = e.getSelectionText()
		ctx.IsSelection = true
	} else {
		ctx.Content = e.Content()
		ctx.IsSelection = false
	}

	if e.aiPanel != nil {
		ctx.ReasoningLevel = e.aiPanel.ThinkingLevel
	}

	return ctx
}

// getSelectionText returns the selected text.
func (e *Editor) getSelectionText() string {
	start, end, ok := e.selectionRange()
	if !ok {
		return ""
	}

	var sb strings.Builder
	lineCount := e.LineCount()

	for row := start.Row; row <= end.Row; row++ {
		if row >= lineCount {
			break
		}
		line := e.text.Line(row)

		startCol := 0
		endCol := len(line)

		if row == start.Row {
			startCol = start.Col
		}
		if row == end.Row {
			endCol = end.Col
		}

		if startCol > len(line) {
			startCol = len(line)
		}
		if endCol > len(line) {
			endCol = len(line)
		}

		sb.WriteString(string(line[startCol:endCol]))
		if row < end.Row {
			sb.WriteRune('\n')
		}
	}

	return sb.String()
}

// applyAIEdit applies the current AI edit suggestion.
func (e *Editor) applyAIEdit() {
	if e.aiPanel == nil || e.aiPanel.CurrentEdit == nil {
		return
	}

	// TODO: Implement edit application through UndoManager
	e.aiPanel.CurrentEdit = nil
	e.setStatus("Edit applied")
}

// rejectAIEdit rejects the current AI edit suggestion.
func (e *Editor) rejectAIEdit() {
	if e.aiPanel == nil || e.aiPanel.CurrentEdit == nil {
		return
	}

	e.aiPanel.CurrentEdit = nil
	e.setStatus("Edit rejected")
}

// ProcessAIEvent processes an event from the AI manager.
func (e *Editor) ProcessAIEvent(event AIEvent) {
	if e.aiPanel == nil {
		return
	}

	switch event.Kind {
	case "text":
		e.aiPanel.State = AIPanelStateStreaming
		e.aiPanel.AppendToStreaming(event.Text)
	case "reasoning":
		e.aiPanel.State = AIPanelStateStreaming
		e.aiPanel.AppendToReasoning(event.Text)
	case "reasoning_done":
		e.aiPanel.FinalizeReasoning()
	case "error":
		e.aiPanel.SetError(event.Error)
	case "done":
		e.aiPanel.FinalizeReasoning()
		e.aiPanel.FinalizeStreaming()
	}
}

// extToLanguage converts file extension to language name.
func extToLanguage(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".jsx":
		return "javascript"
	case ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".sh", ".bash":
		return "bash"
	case ".zsh":
		return "zsh"
	case ".ps1":
		return "powershell"
	case ".sql":
		return "sql"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss", ".sass":
		return "scss"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".xml":
		return "xml"
	case ".md", ".markdown":
		return "markdown"
	default:
		return ""
	}
}

// IsAIPanelFocused returns true if the AI panel has focus.
func (e *Editor) IsAIPanelFocused() bool {
	return e.aiPanel != nil && e.aiPanel.Visible && e.aiPanel.Focused
}

// AIPanel returns the AI panel (for use by app).
func (e *Editor) AIPanel() *AIPanel {
	return e.aiPanel
}

// handleAIPanelKey handles keyboard input when AI panel is focused.
func (e *Editor) handleAIPanelKey(ev EventKey) bool {
	if e.aiPanel == nil {
		return false
	}

	key := ev.Key()
	r := ev.Rune()

	// Handle provider/model selection popups first
	if e.aiPanel.ProviderSelectActive {
		_ = e.handleAIPanelProviderPopup(ev)
		return false
	}
	if e.aiPanel.ModelSelectActive {
		_ = e.handleAIPanelModelPopup(ev)
		return false
	}

	switch key {
	case KeyEscape:
		// Unfocus AI panel, return to editor
		e.aiPanel.Focused = false
		return false

	case KeyEnter:
		// Send message if there's input
		if len(e.aiPanel.CurrentInput) > 0 {
			e.sendToAI()
		}
		return false

	case KeyBackspace, KeyBackspace2:
		// Delete character before cursor
		e.aiPanel.Backspace()
		return false

	case KeyDelete:
		// Delete character at cursor
		e.aiPanel.Delete()
		return false

	case KeyLeft:
		// Move cursor left
		e.aiPanel.MoveCursorLeft()
		return false

	case KeyRight:
		// Move cursor right
		e.aiPanel.MoveCursorRight()
		return false

	case KeyHome, KeyCtrlA:
		// Move cursor to start
		e.aiPanel.MoveCursorHome()
		return false

	case KeyEnd, KeyCtrlE:
		// Move cursor to end
		e.aiPanel.MoveCursorEnd()
		return false

	case KeyUp:
		// Navigate input history or scroll chat
		if len(e.aiPanel.CurrentInput) == 0 {
			e.aiPanel.HistoryUp()
		} else {
			// Scroll chat up
			e.aiPanel.ScrollUp(1)
		}
		return false

	case KeyDown:
		// Navigate input history or scroll chat
		if len(e.aiPanel.CurrentInput) == 0 {
			e.aiPanel.HistoryDown()
		} else {
			// Scroll chat down
			e.aiPanel.ScrollDown(1)
		}
		return false

	case KeyPgUp:
		// Scroll chat up by page
		e.aiPanel.ScrollUp(10)
		return false

	case KeyPgDn:
		// Scroll chat down by page
		e.aiPanel.ScrollDown(10)
		return false

	case KeyCtrlU:
		// Clear input line
		e.aiPanel.ClearInput()
		return false

	case KeyCtrlC:
		// Cancel current AI request
		if e.aiManager != nil {
			e.aiManager.Cancel()
		}
		e.aiPanel.State = AIPanelStateIdle
		return false

	case KeyTab:
		// Tab opens provider selection popup
		e.openProviderSelectPopup()
		return false

	case KeyBacktab:
		// Shift+Tab opens model selection popup
		e.openModelSelectPopup()
		return false

	case KeyCtrlO:
		e.toggleAIReasoning()
		return false

	case KeyCtrlT:
		e.toggleAIThinkingLevel()
		return false

	case KeyRune:
		// Insert character at cursor
		e.aiPanel.InsertRune(r)
		return false
	}

	return false
}

// openProviderSelectPopup opens the provider selection popup with current providers.
func (e *Editor) openProviderSelectPopup() {
	if e.aiManager == nil {
		return
	}

	providers := e.aiManager.ListProviders()
	items := make([]ProviderItem, len(providers))
	currentName := e.aiManager.ActiveName()

	for i, p := range providers {
		items[i] = ProviderItem{
			Name:        p.Name,
			DisplayName: p.DisplayName,
			Available:   p.Available,
			IsCurrent:   p.Name == currentName,
		}
	}

	e.aiPanel.OpenProviderSelect(items)
}

// openModelSelectPopup opens the model selection popup with current models.
func (e *Editor) openModelSelectPopup() {
	if e.aiManager == nil {
		return
	}

	models, err := e.aiManager.ListModels()
	if err != nil {
		return
	}

	e.aiPanel.OpenModelSelect(models, e.aiManager.CurrentModel())
}

// handleAIPanelProviderPopup handles keyboard input for provider selection popup.
func (e *Editor) handleAIPanelProviderPopup(ev EventKey) bool {
	key := ev.Key()

	switch key {
	case KeyEscape:
		e.aiPanel.CloseProviderSelect()
		return true

	case KeyEnter:
		// Select provider
		if e.aiPanel.ProviderSelectIndex >= 0 && e.aiPanel.ProviderSelectIndex < len(e.aiPanel.ProviderSelectItems) {
			selected := e.aiPanel.ProviderSelectItems[e.aiPanel.ProviderSelectIndex]
			if e.aiManager != nil {
				_ = e.aiManager.SetActive(selected.Name)
				e.aiPanel.ProviderName = selected.DisplayName
				e.aiPanel.ModelName = e.aiManager.CurrentModel()
				e.updateAIPanelStatus()
			}
		}
		e.aiPanel.CloseProviderSelect()
		return true

	case KeyUp:
		if e.aiPanel.ProviderSelectIndex > 0 {
			e.aiPanel.ProviderSelectIndex--
		}
		return true

	case KeyDown:
		if e.aiPanel.ProviderSelectIndex < len(e.aiPanel.ProviderSelectItems)-1 {
			e.aiPanel.ProviderSelectIndex++
		}
		return true

	case KeyTab:
		// Tab switches to model selection
		e.aiPanel.CloseProviderSelect()
		e.openModelSelectPopup()
		return true
	}

	return false
}

// handleAIPanelModelPopup handles keyboard input for model selection popup.
func (e *Editor) handleAIPanelModelPopup(ev EventKey) bool {
	key := ev.Key()

	switch key {
	case KeyEscape:
		e.aiPanel.CloseModelSelect()
		return true

	case KeyEnter:
		// Select model
		if e.aiPanel.ModelSelectIndex >= 0 && e.aiPanel.ModelSelectIndex < len(e.aiPanel.ModelSelectItems) {
			selected := e.aiPanel.ModelSelectItems[e.aiPanel.ModelSelectIndex]
			if e.aiManager != nil {
				_ = e.aiManager.SetModel(selected.ID)
				e.aiPanel.ModelName = selected.Name
			}
		}
		e.aiPanel.CloseModelSelect()
		return true

	case KeyUp:
		if e.aiPanel.ModelSelectIndex > 0 {
			e.aiPanel.ModelSelectIndex--
		}
		return true

	case KeyDown:
		if e.aiPanel.ModelSelectIndex < len(e.aiPanel.ModelSelectItems)-1 {
			e.aiPanel.ModelSelectIndex++
		}
		return true

	case KeyTab:
		// Tab switches to provider selection
		e.aiPanel.CloseModelSelect()
		e.openProviderSelectPopup()
		return true
	}

	return false
}

// editAIConversation opens the entire AI conversation in the editor for editing.
// The conversation is exported as markdown, edited, and then re-imported.
func (e *Editor) editAIConversation() {
	if e.aiPanel == nil {
		e.aiPanel = NewAIPanel()
	}

	// Export conversation to markdown
	markdown := e.aiPanel.ExportToMarkdown()

	// Create a temporary file with the conversation
	tempPath := filepath.Join(os.TempDir(), "ai_conversation_"+strconv.FormatInt(time.Now().Unix(), 10)+".md")
	if err := os.WriteFile(tempPath, []byte(markdown), 0644); err != nil {
		e.setStatus("AI edit: failed to create temp file: " + err.Error())
		return
	}

	// Open the file in the editor
	if err := e.OpenFile(tempPath); err != nil {
		e.setStatus("AI edit: failed to open: " + err.Error())
		return
	}

	// Mark this as a special AI edit buffer
	e.aiEditBufferPath = tempPath
	e.aiEditOriginalPanel = e.aiPanel
	e.setStatus("AI conversation opened for editing. Save to update (Ctrl+S), close to cancel (Ctrl+C)")
}

// saveAIEdit saves the edited conversation back to the AI panel.
func (e *Editor) saveAIEdit() bool {
	if e.aiEditBufferPath == "" || e.aiEditOriginalPanel == nil {
		return false
	}

	content := e.Content()
	if err := e.aiEditOriginalPanel.ImportFromMarkdown(content); err != nil {
		e.setStatus("AI edit: failed to import: " + err.Error())
		return false
	}

	// Clean up
	os.Remove(e.aiEditBufferPath)
	e.aiEditBufferPath = ""
	e.aiEditOriginalPanel = nil
	e.setStatus("AI conversation updated")
	return true
}

// cancelAIEdit cancels the AI conversation editing.
func (e *Editor) cancelAIEdit() {
	if e.aiEditBufferPath != "" {
		os.Remove(e.aiEditBufferPath)
		e.aiEditBufferPath = ""
	}
	e.aiEditOriginalPanel = nil
	e.setStatus("AI edit cancelled")
}

// toggleAIReasoning toggles the visibility of AI reasoning messages.
func (e *Editor) toggleAIReasoning() {
	if e.aiPanel == nil {
		e.aiPanel = NewAIPanel()
	}
	show := e.aiPanel.ToggleReasoning()
	if show {
		e.setStatus("AI reasoning: on")
	} else {
		e.setStatus("AI reasoning: off")
	}
}

// toggleAIThinkingLevel cycles through available AI thinking levels.
func (e *Editor) toggleAIThinkingLevel() {
	if e.aiPanel == nil {
		e.aiPanel = NewAIPanel()
	}
	model := ""
	if e.aiManager != nil {
		model = e.aiManager.CurrentModel()
	}
	levels := e.aiThinkingLevelsForModel(model)
	if len(levels) == 0 {
		e.setStatus("AI thinking: auto")
		return
	}
	newLevel := e.aiPanel.CycleThinkingLevel(levels)
	e.saveAIState()
	e.setStatus("AI thinking: " + newLevel)
}

// toggleAIPanelMaximize toggles the maximized state of the AI panel.
func (e *Editor) toggleAIPanelMaximize() {
	if e.aiPanel == nil {
		e.aiPanel = NewAIPanel()
	}
	max := e.aiPanel.ToggleMaximize()
	if max {
		e.setStatus("AI panel: maximized")
	} else {
		e.setStatus("AI panel: normal")
	}
}

// saveAIState saves the current AI configuration to the session store.
func (e *Editor) saveAIState() {
	if e.sessionStore == nil {
		return
	}
	state := AIState{}
	if e.aiManager != nil {
		state.Provider = e.aiManager.ActiveName()
		state.Model = e.aiManager.CurrentModel()
	}
	if e.aiPanel != nil {
		state.Thinking = e.aiPanel.ThinkingLevel
	}
	e.sessionStore.SetAIState(state)
}

// restoreAIState restores the AI configuration from the session store.
func (e *Editor) restoreAIState() {
	if e.sessionStore == nil {
		return
	}
	state, ok := e.sessionStore.GetAIState()
	if !ok {
		return
	}
	if state.Provider != "" {
		_ = e.aiManager.SetActive(state.Provider)
	}
	if state.Model != "" {
		_ = e.aiManager.SetModel(state.Model)
	}
	if state.Thinking != "" && e.aiPanel != nil {
		e.aiPanel.SetThinkingLevel(state.Thinking)
	}
}

// syncAIPanelProviderState updates the AI panel with current provider state.
func (e *Editor) syncAIPanelProviderState() {
	if e.aiPanel == nil || e.aiManager == nil {
		return
	}
	e.aiPanel.ProviderName = e.aiManager.ActiveName()
	e.aiPanel.ModelName = e.aiManager.CurrentModel()
	e.updateAIPanelStatus()
}

// refreshAIPanelProviderState refreshes and updates the AI panel provider state.
func (e *Editor) refreshAIPanelProviderState() {
	if e.aiPanel == nil || e.aiManager == nil {
		return
	}
	e.aiPanel.ProviderName = e.aiManager.ActiveName()
	e.aiPanel.ModelName = e.aiManager.CurrentModel()
	e.updateAIPanelStatus()
}

// aiThinkingLevelsForModel returns the thinking levels available for a given model.
func (e *Editor) aiThinkingLevelsForModel(model string) []string {
	if e.aiThinkingLevelsByModel != nil && model != "" {
		key := strings.ToLower(strings.TrimSpace(model))
		if levels, ok := e.aiThinkingLevelsByModel[key]; ok {
			return levels
		}
	}
	return e.aiThinkingLevels
}

func (e *Editor) updateAIPanelStatus() {
	if e.aiPanel == nil || e.aiManager == nil {
		return
	}
	active := e.aiManager.ActiveName()
	if active == "" {
		e.aiPanel.Status = AIProviderStatusOffline
		return
	}
	status := AIProviderStatusOffline
	providers := e.aiManager.ListProviders()
	for _, p := range providers {
		if p.Name != active {
			continue
		}
		status = AIProviderStatus(p.Status)
		if p.Available && status == AIProviderStatusOffline {
			status = AIProviderStatusOnline
		}
		break
	}
	e.aiPanel.Status = status
}
