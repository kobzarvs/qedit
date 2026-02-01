package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func (e *Editor) execAction(action string) bool {
	if e.actionHook != nil {
		e.actionHook(action)
	}
	switch action {
	case actionMoveLeft:
		e.moveLeft()
	case actionMoveRight:
		e.moveRight()
	case actionMoveUp:
		e.moveUp()
	case actionMoveDown:
		e.moveDown()
	case actionWordLeft:
		e.moveWordLeft()
	case actionWordRight:
		e.moveWordRight()
	case actionLineStart:
		e.moveLineStart()
	case actionLineEnd:
		e.moveLineEnd()
	case actionFileStart:
		e.moveFileStart()
	case actionFileEnd:
		e.moveFileEnd()
	case actionPageUp:
		e.pageUp()
	case actionPageDown:
		e.pageDown()
	case actionMoveLineUp:
		e.moveLineUp()
	case actionMoveLineDown:
		e.moveLineDown()
	case actionToggleLineNumbers:
		e.toggleLineNumbers()
	case actionBranchPicker:
		e.openSidebarBranches()
	case actionToggleSidebar:
		e.toggleSidebar()
	case actionEnterInsert:
		e.mode = ModeInsert
		e.saveLineState()
	case actionEnterNormal:
		e.mode = ModeNormal
	case actionEnterCommand:
		e.mode = ModeCommand
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
	case actionMergeMode:
		return e.enterMergeMode()
	case actionQuit:
		return true
	case actionBackspace:
		e.backspace()
	case actionNewline:
		e.insertNewline()
	case actionInsertTab:
		e.insertTab()
	case actionUndo:
		e.Undo()
		return false // Don't clear selection - undo may restore it
	case actionRedo:
		e.Redo()
		return false // Don't clear selection
	case actionDeleteLine:
		e.deleteLine()
	case actionDeleteChar:
		e.deleteChar()
	case actionDeleteWordLeft:
		e.deleteWordLeft()
	case actionDeleteWordRight:
		e.deleteWordRight()
	case actionInsertLineBelow:
		e.insertLineBelow()
	case actionUndoLine:
		e.undoLine()
	case actionScrollUp:
		e.scrollViewUp()
	case actionScrollDown:
		e.scrollViewDown()
	case actionIndent:
		e.indentSelection()
		return false // Don't clear selection
	case actionUnindent:
		e.unindentSelection()
		return false // Don't clear selection
	case actionSelectAll:
		e.selectAll()
		return false // Don't clear selection

	// Helix-style motions
	case actionWordForward:
		e.wordForward()
	case actionWordBackward:
		e.wordBackward()
	case actionWordEnd:
		e.wordEnd()
	case actionGotoMode:
		e.gotoMode = true
		e.pendingKeys = "g"
		return false // Don't clear selection, wait for next key
	case actionGotoLine:
		e.gotoLastLine()
	case actionGotoLinePrompt:
		e.mode = ModeCommand
		e.cmd = []rune{}
		e.cmdCursor = 0
		e.setStatus("goto line:")
	case actionGotoFirstLine:
		e.gotoFirstLine()
	case actionGotoFileEnd:
		e.gotoFileEnd()
	case actionFindChar:
		e.setPendingFindChar(action)
		e.pendingKeys = "f"
		return false // Don't clear selection, wait for char input
	case actionFindCharBackward:
		e.setPendingFindChar(action)
		e.pendingKeys = "F"
		return false
	case actionTillChar:
		e.setPendingFindChar(action)
		e.pendingKeys = "t"
		return false
	case actionTillCharBackward:
		e.setPendingFindChar(action)
		e.pendingKeys = "T"
		return false

	// Helix-style editing
	case actionDelete:
		e.helixDelete()
	case actionChange:
		e.helixChange()
		return false // Don't clear selection (entering insert mode)
	case actionYank:
		e.yankSelection()
		return false // Don't clear selection yet (yank preserves for visual feedback)
	case actionPaste:
		e.pasteAfter()
	case actionPasteBefore:
		e.pasteBefore()
	case actionOpenBelow:
		e.openBelow()
		return false // Entering insert mode
	case actionOpenAbove:
		e.openAbove()
		return false // Entering insert mode
	case actionAppend:
		e.appendMode()
		return false // Entering insert mode
	case actionAppendLineEnd:
		e.appendLineEnd()
		return false // Entering insert mode
	case actionInsertLineStart:
		e.insertLineStart()
		return false // Entering insert mode
	case actionReplaceChar:
		e.setPendingFindChar(action)
		e.pendingKeys = "r"
		return false // Wait for char input
	case actionJoinLines:
		e.joinLinesCmd()

	// Helix-style selection
	case actionToggleSelect:
		e.toggleSelectMode()
		return false // Don't clear selection
	case actionExtendLine:
		e.extendLine()
		return false // Don't clear selection
	case actionCollapseSelection:
		e.collapseSelection()
	case actionFlipSelection:
		e.flipSelection()
		return false // Don't clear selection

	// Space mode
	case actionSpaceMode:
		e.spaceMenuActive = true
		e.pendingKeys = "SPC"
		return false

	// Match mode
	case actionMatchMode:
		e.matchMode = true
		e.pendingKeys = "m"
		return false

	// View mode
	case actionViewMode:
		e.viewMode = true
		e.pendingKeys = "z"
		return false

	// Search
	case actionSearchForward:
		e.enterSearchMode(true, false, false) // exact search
		return false
	case actionSearchBackward:
		e.enterSearchMode(false, false, false) // exact search
		return false
	case actionSearchFuzzy:
		e.enterSearchMode(true, true, false) // fuzzy search
		return false
	case actionSearchRegex:
		e.enterSearchMode(true, false, true) // regex search
		return false
	case actionSearchNext:
		e.searchNext()
	case actionSearchPrev:
		e.searchPrev()

	// Special
	case actionInsertLineAbove:
		e.insertLineAboveCursor()

	// Terminal zoom
	case actionTerminalZoomIn:
		if e.zoomPendingRestore {
			return false // already zoomed, ignore
		}
		// Save current scroll positions for restore
		e.zoomSavedScroll = e.scroll
		e.zoomSavedScrollX = e.scrollX
		e.zoomWithAnimation(true, 20) // zoom in with scroll animation
		e.zoomPendingRestore = true
		return false

	// Selection scope
	case actionExpandSelection:
		e.expandSelection()
		return false
	case actionShrinkSelection:
		e.shrinkSelection()
		return false

	// File operations
	case actionSave:
		if err := e.Save(""); err != nil {
			e.setStatus(err.Error())
		} else {
			e.setStatus("saved " + e.filename)
		}
		return false

	// AI integration
	case actionAIPanel:
		e.toggleAIPanel()
		return false
	case actionAISend:
		e.sendToAI()
		return false
	case actionAIApply:
		e.applyAIEdit()
		return false
	case actionAIReject:
		e.rejectAIEdit()
		return false
	}
	if !e.selectMode {
		e.clearSelection()
	}
	return false
}
func (e *Editor) execCommand(cmd string) bool {
	if cmd == "" {
		return false
	}
	fields := strings.Fields(cmd)
	name := fields[0]
	args := fields[1:]

	switch name {
	case "w":
		path := ""
		if len(args) > 0 {
			path = strings.Join(args, " ")
		}
		if err := e.Save(path); err != nil {
			e.setStatus(err.Error())
			return false
		}
		e.setStatus("written")
		return false
	case "e", "edit":
		if len(args) > 0 {
			e.setStatus("edit file not supported")
			return false
		}
		if err := e.ReloadFromDisk(false); err != nil {
			e.setStatus(err.Error())
			return false
		}
		e.setStatus("reloaded")
		return false
	case "e!", "edit!":
		if len(args) > 0 {
			e.setStatus("edit file not supported")
			return false
		}
		if err := e.ReloadFromDisk(true); err != nil {
			e.setStatus(err.Error())
			return false
		}
		e.setStatus("reloaded")
		return false
	case "q":
		if e.dirty {
			e.setStatus("unsaved changes (use :q!)")
			return false
		}
		return true
	case "q!":
		return true
	case "wq", "x":
		path := ""
		if len(args) > 0 {
			path = strings.Join(args, " ")
		}
		if err := e.Save(path); err != nil {
			e.setStatus(err.Error())
			return false
		}
		return true
	case "ln":
		if len(args) == 0 {
			e.toggleLineNumbers()
			return false
		}
		switch strings.ToLower(args[0]) {
		case "off":
			e.lineNumberMode = LineNumberOff
			e.setStatus("line numbers off")
		case "abs", "absolute":
			e.lineNumberMode = LineNumberAbsolute
			e.setStatus("line numbers absolute")
		case "rel", "relative":
			e.lineNumberMode = LineNumberRelative
			e.setStatus("line numbers relative")
		default:
			e.setStatus("unknown line number mode")
		}
		return false
	case "fmt":
		if err := e.FormatCurrent(); err != nil {
			e.setStatus(err.Error())
			return false
		}
		e.setStatus("formatted")
		return false
	case "sidebar":
		e.toggleSidebar()
		return false
	case "sidew":
		// :sidew - show current width, :sidew 30 - set width
		if len(args) == 0 {
			if e.sidebar != nil {
				e.setStatus("sidebar width: " + e.sidebar.WidthConfig)
			}
		} else {
			if e.sidebar != nil {
				e.sidebar.WidthConfig = args[0]
				e.setStatus("sidebar width set to " + args[0])
			}
		}
		return false
	case "autoreload", "auto-reload", "auto-reload-on-changes":
		if len(args) == 0 {
			e.setStatus("auto-reload-on-changes=" + boolToFlag(e.autoReloadOnChanges))
			return false
		}
		if len(args) > 1 {
			e.setStatus("usage: auto-reload-on-changes 1|0")
			return false
		}
		enabled, ok := parseBoolArg(args[0])
		if !ok {
			e.setStatus("auto-reload-on-changes expects 1|0")
			return false
		}
		if enabled == e.autoReloadOnChanges {
			e.setStatus("auto-reload-on-changes=" + boolToFlag(e.autoReloadOnChanges))
			return false
		}
		if e.autoReloadConfigHook != nil {
			if err := e.autoReloadConfigHook(enabled); err != nil {
				e.setStatus("config write failed: " + err.Error())
				return false
			}
		}
		e.autoReloadOnChanges = enabled
		e.setStatus("auto-reload-on-changes=" + boolToFlag(e.autoReloadOnChanges))
		return false
	case "merge":
		if len(args) > 0 {
			e.setStatus("merge takes no arguments")
			return false
		}
		return e.enterMergeMode()
	case "ide":
		e.toggleAIPanel()
		return false
	case "ai":
		return e.handleAICommand(args)
	default:
		// Check if command is a line number
		if lineNum, err := strconv.Atoi(name); err == nil && lineNum > 0 {
			e.gotoLineNumber(lineNum)
			return false
		}
		e.setStatus("unknown command: " + name)
		return false
	}
}

// handleAICommand handles :ai subcommands
func (e *Editor) handleAICommand(args []string) bool {
	if e.aiManager == nil {
		e.setStatus("AI not configured")
		return false
	}

	if len(args) == 0 {
		// :ai - show current provider and model
		name := e.aiManager.ActiveName()
		model := e.aiManager.CurrentModel()
		if name == "" {
			e.setStatus("AI: no provider active")
		} else {
			e.setStatus(fmt.Sprintf("AI: %s / %s", name, model))
		}
		return false
	}

	switch args[0] {
	case "list":
		// :ai list - show available providers
		providers := e.aiManager.ListProviders()
		if len(providers) == 0 {
			e.setStatus("AI: no providers configured")
			return false
		}
		var names []string
		for _, p := range providers {
			status := "offline"
			if p.Available {
				status = "online"
			}
			if p.Name == e.aiManager.ActiveName() {
				names = append(names, "*"+p.Name+"("+status+")")
			} else {
				names = append(names, p.Name+"("+status+")")
			}
		}
		e.setStatus("AI providers: " + strings.Join(names, ", "))
		return false

	case "model":
		if len(args) < 2 {
			// :ai model - show available models
			models, err := e.aiManager.ListModels()
			if err != nil {
				e.setStatus("AI: " + err.Error())
				return false
			}
			if len(models) == 0 {
				e.setStatus("AI: no models available")
				return false
			}
			var names []string
			current := e.aiManager.CurrentModel()
			for _, m := range models {
				if m.ID == current {
					names = append(names, "*"+m.ID)
				} else {
					names = append(names, m.ID)
				}
			}
			e.setStatus("Models: " + strings.Join(names, ", "))
			return false
		}
		// :ai model <name> - set model
		model := args[1]
		if err := e.aiManager.SetModel(model); err != nil {
			e.setStatus("AI: " + err.Error())
			return false
		}
		e.setStatus("AI model: " + model)
		if e.aiPanel != nil {
			e.aiPanel.ModelName = model
		}
		return false

	default:
		// :ai <provider> - switch provider
		provider := args[0]
		if err := e.aiManager.SetActive(provider); err != nil {
			e.setStatus("AI: " + err.Error())
			return false
		}
		e.setStatus("AI provider: " + provider)
		if e.aiPanel != nil {
			e.aiPanel.ProviderName = provider
			e.aiPanel.ModelName = e.aiManager.CurrentModel()
		}
		return false
	}
}
func (e *Editor) gotoLineNumber(lineNum int) {
	if lineNum < 1 {
		lineNum = 1
	}
	lineCount := e.LineCount()
	if lineNum > lineCount {
		lineNum = lineCount
	}
	e.cursor.Row = lineNum - 1
	e.cursor.Col = 0
	e.selectionActive = false
	e.freeScroll = false
	e.scrollX = 0
	e.setStatus(fmt.Sprintf("line %d", lineNum))
}

func parseBoolArg(value string) (bool, bool) {
	if value == "" {
		return false, false
	}
	val := strings.ToLower(strings.TrimSpace(value))
	switch val {
	case "on", "yes":
		return true, true
	case "off", "no":
		return false, true
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return false, false
	}
	return b, true
}

func boolToFlag(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
func (e *Editor) Save(path string) error {
	if path == "" {
		if e.filename == "" {
			return errors.New("no file name")
		}
		path = e.filename
	}
	data := []byte(e.Content())
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	e.filename = path
	e.savePoint = len(e.undo)
	e.externalChange = ExternalChangeNone
	e.diskContent = e.Content()
	e.updateDirty()
	_ = e.syncFileSnapshot()
	_ = e.SaveUndoHistory()
	e.saveSessionState()
	return nil
}
func (e *Editor) FormatGo() error {
	src := e.Content()
	if e.formatter == nil {
		return errors.New("formatter unavailable")
	}
	formatted, err := e.formatter.FormatGo(src)
	if err != nil {
		return err
	}
	if formatted == src {
		return nil
	}
	e.replaceBuffer(formatted, true)
	return nil
}
func (e *Editor) FormatCurrent() error {
	if isMarkdownFile(e.filename) {
		return e.FormatMarkdownTables()
	}
	if isGoFile(e.filename) {
		return e.FormatGo()
	}
	if e.filename == "" && looksLikeGo(e.Content()) {
		return e.FormatGo()
	}
	return errors.New("format not supported")
}
func isGoFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".go"
}
func looksLikeGo(src string) bool {
	src = strings.TrimLeftFunc(src, unicode.IsSpace)
	return strings.HasPrefix(src, "package ")
}
func isMarkdownFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	default:
		return false
	}
}
func (e *Editor) FormatMarkdownTables() error {
	src := e.Content()
	lines := strings.Split(src, "\n")
	if len(lines) == 0 {
		return nil
	}
	changed := false
	i := 0
	inFence := false
	for i < len(lines) {
		if isFenceLine(lines[i]) {
			inFence = !inFence
			i++
			continue
		}
		if inFence {
			i++
			continue
		}
		if !lineHasPipe(lines[i]) {
			i++
			continue
		}
		sepIdx := findTableSeparator(lines, i)
		if sepIdx != -1 && sepIdx > 0 {
			start := sepIdx - 1
			end := sepIdx + 1
			for end < len(lines) && lineHasPipe(lines[end]) && strings.TrimSpace(lines[end]) != "" {
				end++
			}
			if start < 0 || start >= end {
				i++
				continue
			}
			prefix := leadingWhitespace(lines[start])
			block := lines[start:end]
			formatted := formatMarkdownTableBlock(block, prefix)
			if formatted == nil {
				i = end
				continue
			}
			for j := start; j < end; j++ {
				if lines[j] != formatted[j-start] {
					lines[j] = formatted[j-start]
					changed = true
				}
			}
			i = end
			continue
		}

		start := i
		end := i + 1
		for end < len(lines) && lineHasPipe(lines[end]) && strings.TrimSpace(lines[end]) != "" {
			end++
		}
		if end-start < 2 {
			i++
			continue
		}
		block := lines[start:end]
		if !isPipeTableBlock(block) {
			i++
			continue
		}
		prefix := leadingWhitespace(lines[start])
		formatted := formatMarkdownTableBlockNoSeparator(block, prefix)
		if formatted == nil {
			i = end
			continue
		}
		for j := start; j < end; j++ {
			if lines[j] != formatted[j-start] {
				lines[j] = formatted[j-start]
				changed = true
			}
		}
		i = end
	}
	if !changed {
		return nil
	}
	e.replaceBuffer(strings.Join(lines, "\n"), true)
	return nil
}
func isFenceLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}
func lineHasPipe(line string) bool {
	return strings.Contains(line, "|")
}
func findTableSeparator(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		if isTableSeparator(lines[i]) {
			return i
		}
		if strings.TrimSpace(lines[i]) == "" && i > start {
			return -1
		}
	}
	return -1
}
func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if !strings.Contains(trimmed, "-") || !strings.Contains(trimmed, "|") {
		return false
	}
	if strings.HasPrefix(trimmed, "|") {
		trimmed = trimmed[1:]
	}
	if strings.HasSuffix(trimmed, "|") {
		trimmed = trimmed[:len(trimmed)-1]
	}
	parts := strings.Split(trimmed, "|")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		cell := strings.TrimSpace(part)
		if cell == "" {
			continue
		}
		if !onlyTableSepChars(cell) {
			return false
		}
		if strings.Count(cell, "-") < 3 {
			return false
		}
	}
	return true
}
func onlyTableSepChars(cell string) bool {
	for _, r := range cell {
		if r != '-' && r != ':' {
			return false
		}
	}
	return true
}
func leadingWhitespace(line string) string {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[:i]
}
func formatMarkdownTableBlock(lines []string, prefix string) []string {
	if len(lines) < 2 {
		return nil
	}
	sepIdx := -1
	for i, line := range lines {
		if isTableSeparator(line) {
			sepIdx = i
			break
		}
	}
	if sepIdx <= 0 {
		return nil
	}
	rows := make([][]string, len(lines))
	maxCols := 0
	for i, line := range lines {
		rows[i] = splitTableRow(line)
		if len(rows[i]) > maxCols {
			maxCols = len(rows[i])
		}
	}
	if maxCols == 0 {
		return nil
	}
	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], "")
		}
	}
	widths := make([]int, maxCols)
	for i, row := range rows {
		if i == sepIdx {
			continue
		}
		for c, cell := range row {
			w := runeLen(cell)
			if w > widths[c] {
				widths[c] = w
			}
		}
	}
	aligns := make([]tableAlign, maxCols)
	for c, cell := range rows[sepIdx] {
		s := strings.TrimSpace(cell)
		if strings.HasPrefix(s, ":") {
			aligns[c].left = true
		}
		if strings.HasSuffix(s, ":") {
			aligns[c].right = true
		}
	}
	out := make([]string, len(lines))
	for i, row := range rows {
		cells := make([]string, maxCols)
		if i == sepIdx {
			for c := 0; c < maxCols; c++ {
				width := widths[c]
				if width < 3 {
					width = 3
				}
				switch {
				case aligns[c].left && aligns[c].right:
					dashes := strings.Repeat("-", max(1, width-2))
					cells[c] = ":" + dashes + ":"
				case aligns[c].left:
					dashes := strings.Repeat("-", max(1, width-1))
					cells[c] = ":" + dashes
				case aligns[c].right:
					dashes := strings.Repeat("-", max(1, width-1))
					cells[c] = dashes + ":"
				default:
					cells[c] = strings.Repeat("-", width)
				}
			}
		} else {
			for c, cell := range row {
				padding := widths[c] - runeLen(cell)
				if padding < 0 {
					padding = 0
				}
				cells[c] = cell + strings.Repeat(" ", padding)
			}
		}
		out[i] = prefix + "| " + strings.Join(cells, " | ") + " |"
	}
	return out
}
func formatMarkdownTableBlockNoSeparator(lines []string, prefix string) []string {
	if len(lines) < 2 {
		return nil
	}
	rows := make([][]string, len(lines))
	maxCols := 0
	for i, line := range lines {
		rows[i] = splitTableRow(line)
		if len(rows[i]) > maxCols {
			maxCols = len(rows[i])
		}
	}
	if maxCols < 2 {
		return nil
	}
	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], "")
		}
	}
	widths := make([]int, maxCols)
	for _, row := range rows {
		for c, cell := range row {
			w := runeLen(cell)
			if w > widths[c] {
				widths[c] = w
			}
		}
	}
	out := make([]string, len(lines))
	for i, row := range rows {
		cells := make([]string, maxCols)
		for c, cell := range row {
			padding := widths[c] - runeLen(cell)
			if padding < 0 {
				padding = 0
			}
			cells[c] = cell + strings.Repeat(" ", padding)
		}
		out[i] = prefix + "| " + strings.Join(cells, " | ") + " |"
	}
	return out
}
func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "|") {
		trimmed = trimmed[1:]
	}
	if strings.HasSuffix(trimmed, "|") {
		trimmed = trimmed[:len(trimmed)-1]
	}
	parts := strings.Split(trimmed, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}
func runeLen(s string) int {
	return len([]rune(s))
}

func isPipeTableBlock(lines []string) bool {
	if len(lines) < 2 {
		return false
	}
	colCount := -1
	for _, line := range lines {
		row := splitTableRow(line)
		if len(row) < 2 {
			return false
		}
		if colCount == -1 {
			colCount = len(row)
		} else if len(row) != colCount {
			return false
		}
	}
	return true
}

// sendTerminalZoomStep sends a single zoom command to the terminal via the zoomer.
func (e *Editor) sendTerminalZoomStep(zoomIn bool) {
	if e.terminalZoomer == nil {
		return
	}
	_ = e.terminalZoomer.ZoomStep(zoomIn)
}

// zoomWithAnimation performs zoom with synchronized scroll animation.
// For zoom in: scrolls to center cursor. For zoom out: scrolls back to saved position.
func (e *Editor) zoomWithAnimation(zoomIn bool, steps int) {
	if e.zoomAnimating {
		return // already animating, ignore
	}
	e.zoomAnimating = true
	defer func() { e.zoomAnimating = false }()

	// Calculate start and target scroll positions
	startScroll := e.scroll
	startScrollX := e.scrollX

	var targetScroll, targetScrollX int
	if zoomIn {
		// Target: center cursor on screen
		// Use current viewport, will be approximate but close enough
		targetScroll = e.cursor.Row - e.viewHeight/2
		targetScrollX = e.cursor.Col - e.viewWidth/2

		// Clamp targets
		if targetScroll < 0 {
			targetScroll = 0
		}
		maxScroll := e.LineCount() - e.viewHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if targetScroll > maxScroll {
			targetScroll = maxScroll
		}
		if targetScrollX < 0 {
			targetScrollX = 0
		}
	} else {
		// Target: restore saved scroll positions
		targetScroll = e.zoomSavedScroll
		targetScrollX = e.zoomSavedScrollX
	}

	// Perform animated zoom + scroll
	for i := 1; i <= steps; i++ {
		// Send one zoom step
		e.sendTerminalZoomStep(zoomIn)

		// Interpolate scroll position (linear)
		progress := float64(i) / float64(steps)
		e.scroll = startScroll + int(float64(targetScroll-startScroll)*progress)
		e.scrollX = startScrollX + int(float64(targetScrollX-startScrollX)*progress)

		// Small delay between steps for smoothness
		time.Sleep(15 * time.Millisecond)
	}

	// Ensure final position is exact
	e.scroll = targetScroll
	e.scrollX = targetScrollX
}
