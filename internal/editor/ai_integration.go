package editor

import (
	"path/filepath"
	"strings"
)

// toggleAIPanel toggles the AI panel visibility.
func (e *Editor) toggleAIPanel() {
	if e.aiPanel == nil {
		e.aiPanel = NewAIPanel()
	}
	e.aiPanel.Toggle()

	// Update panel state from manager
	if e.aiPanel.Visible && e.aiManager != nil {
		e.aiPanel.ProviderName = e.aiManager.ActiveName()
		e.aiPanel.ModelName = e.aiManager.CurrentModel()
		// TODO: Update status from active provider
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
	case "error":
		e.aiPanel.SetError(event.Error)
	case "done":
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
		return e.handleAIPanelProviderPopup(ev)
	}
	if e.aiPanel.ModelSelectActive {
		return e.handleAIPanelModelPopup(ev)
	}

	switch key {
	case KeyEscape:
		// Unfocus AI panel, return to editor
		e.aiPanel.Focused = false
		return true

	case KeyEnter:
		// Send message if there's input
		if len(e.aiPanel.CurrentInput) > 0 {
			e.sendToAI()
		}
		return true

	case KeyBackspace, KeyBackspace2:
		// Delete character before cursor
		e.aiPanel.Backspace()
		return true

	case KeyDelete:
		// Delete character at cursor
		e.aiPanel.Delete()
		return true

	case KeyLeft:
		// Move cursor left
		e.aiPanel.MoveCursorLeft()
		return true

	case KeyRight:
		// Move cursor right
		e.aiPanel.MoveCursorRight()
		return true

	case KeyHome, KeyCtrlA:
		// Move cursor to start
		e.aiPanel.MoveCursorHome()
		return true

	case KeyEnd, KeyCtrlE:
		// Move cursor to end
		e.aiPanel.MoveCursorEnd()
		return true

	case KeyUp:
		// Navigate input history or scroll chat
		if len(e.aiPanel.CurrentInput) == 0 {
			e.aiPanel.HistoryUp()
		} else {
			// Scroll chat up
			e.aiPanel.ScrollUp(1)
		}
		return true

	case KeyDown:
		// Navigate input history or scroll chat
		if len(e.aiPanel.CurrentInput) == 0 {
			e.aiPanel.HistoryDown()
		} else {
			// Scroll chat down
			e.aiPanel.ScrollDown(1)
		}
		return true

	case KeyPgUp:
		// Scroll chat up by page
		e.aiPanel.ScrollUp(10)
		return true

	case KeyPgDn:
		// Scroll chat down by page
		e.aiPanel.ScrollDown(10)
		return true

	case KeyCtrlU:
		// Clear input line
		e.aiPanel.ClearInput()
		return true

	case KeyCtrlC:
		// Cancel current AI request
		if e.aiManager != nil {
			e.aiManager.Cancel()
		}
		e.aiPanel.State = AIPanelStateIdle
		return true

	case KeyTab:
		// Tab opens provider selection popup
		e.openProviderSelectPopup()
		return true

	case KeyRune:
		// Insert character at cursor
		e.aiPanel.InsertRune(r)
		return true
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
			if e.aiManager != nil && selected.Available {
				_ = e.aiManager.SetActive(selected.Name)
				e.aiPanel.ProviderName = selected.DisplayName
				e.aiPanel.ModelName = e.aiManager.CurrentModel()
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
