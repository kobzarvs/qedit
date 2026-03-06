package editor

import (
	"os"
	"path/filepath"
	"strings"
)

func (e *Editor) handleCommand(ev EventKey) bool {
	switch ev.Key() {
	case KeyEscape:
		e.closeAutoComplete()
		e.mode = ModeNormal
		e.commandLine.text = e.commandLine.text[:0]
		e.commandLine.cursor = 0
		e.commandLine.historyIndex = -1
		return false
	case KeyCtrlC:
		e.closeAutoComplete()
		e.mode = ModeNormal
		e.commandLine.text = e.commandLine.text[:0]
		e.commandLine.cursor = 0
		e.commandLine.historyIndex = -1
		return false
	case KeyTab:
		prefix := string(e.commandLine.text)
		if !e.cmdAutoComplete.active {
			e.cmdAutoComplete.items = filterCommands(prefix)
			if len(e.cmdAutoComplete.items) == 0 {
				return false
			}
			e.cmdAutoComplete.active = true
			e.cmdAutoComplete.index = -1
			e.cmdAutoComplete.cols = 1 // Will be recalculated on render
		} else {
			// Tab moves to next item (down in column, then next column top)
			if e.cmdAutoComplete.index < 0 {
				e.cmdAutoComplete.index = 0
			} else {
				e.cmdAutoComplete.index++
				if e.cmdAutoComplete.index >= len(e.cmdAutoComplete.items) {
					e.cmdAutoComplete.index = 0
				}
			}
		}
		e.updateCmdFromAutocomplete()
		return false
	case KeyBacktab:
		if e.cmdAutoComplete.active {
			// Shift+Tab moves to previous item (up in column, then prev column bottom)
			if e.cmdAutoComplete.index < 0 {
				e.cmdAutoComplete.index = len(e.cmdAutoComplete.items) - 1
			} else {
				e.cmdAutoComplete.index--
				if e.cmdAutoComplete.index < 0 {
					e.cmdAutoComplete.index = len(e.cmdAutoComplete.items) - 1
				}
			}
			e.updateCmdFromAutocomplete()
		}
		return false
	case KeyEnter:
		e.closeAutoComplete()
		cmd := strings.TrimSpace(string(e.commandLine.text))
		e.mode = ModeNormal
		// Add to history if not empty and different from last
		if cmd != "" && (len(e.commandLine.history) == 0 || e.commandLine.history[len(e.commandLine.history)-1] != cmd) {
			e.commandLine.history = append(e.commandLine.history, cmd)
			e.saveCmdHistory()
		}
		e.commandLine.text = e.commandLine.text[:0]
		e.commandLine.cursor = 0
		e.commandLine.historyIndex = -1
		return e.execCommand(cmd)
	case KeyBackspace, KeyBackspace2:
		e.closeAutoComplete()
		if e.commandLine.cursor > 0 && len(e.commandLine.text) > 0 {
			// Delete char before cursor
			e.commandLine.text = append(e.commandLine.text[:e.commandLine.cursor-1], e.commandLine.text[e.commandLine.cursor:]...)
			e.commandLine.cursor--
			e.commandLine.historyIndex = -1
		}
		return false
	case KeyDelete:
		e.closeAutoComplete()
		if e.commandLine.cursor < len(e.commandLine.text) {
			// Delete char at cursor
			e.commandLine.text = append(e.commandLine.text[:e.commandLine.cursor], e.commandLine.text[e.commandLine.cursor+1:]...)
			e.commandLine.historyIndex = -1
		}
		return false
	case KeyLeft, KeyCtrlB: // Ctrl+B = back (readline)
		if e.cmdAutoComplete.active {
			e.cmdAutoCompleteLeft()
			e.updateCmdFromAutocomplete()
			return false
		}
		if e.commandLine.cursor > 0 {
			e.commandLine.cursor--
		}
		return false
	case KeyRight, KeyCtrlF: // Ctrl+F = forward (readline)
		if e.cmdAutoComplete.active {
			e.cmdAutoCompleteRight()
			e.updateCmdFromAutocomplete()
			return false
		}
		if e.commandLine.cursor < len(e.commandLine.text) {
			e.commandLine.cursor++
		}
		return false
	case KeyHome, KeyCtrlA: // Ctrl+A = beginning of line
		e.commandLine.cursor = 0
		return false
	case KeyEnd, KeyCtrlE: // Ctrl+E = end of line
		e.commandLine.cursor = len(e.commandLine.text)
		return false
	case KeyUp, KeyCtrlP: // Ctrl+P = previous
		if e.cmdAutoComplete.active {
			e.cmdAutoCompleteUp()
			e.updateCmdFromAutocomplete()
			return false
		}
		e.cmdHistoryUp()
		return false
	case KeyDown, KeyCtrlN: // Ctrl+N = next
		if e.cmdAutoComplete.active {
			e.cmdAutoCompleteDown()
			e.updateCmdFromAutocomplete()
			return false
		}
		e.cmdHistoryDown()
		return false
	case KeyCtrlU: // Ctrl+U = clear line
		e.commandLine.text = e.commandLine.text[:0]
		e.commandLine.cursor = 0
		e.commandLine.historyIndex = -1
		return false
	case KeyCtrlK: // Ctrl+K = kill to end of line
		e.commandLine.text = e.commandLine.text[:e.commandLine.cursor]
		e.commandLine.historyIndex = -1
		return false
	case KeyCtrlW: // Ctrl+W = delete word backward
		if e.commandLine.cursor > 0 {
			// Find start of previous word
			i := e.commandLine.cursor - 1
			for i > 0 && e.commandLine.text[i-1] == ' ' {
				i--
			}
			for i > 0 && e.commandLine.text[i-1] != ' ' {
				i--
			}
			e.commandLine.text = append(e.commandLine.text[:i], e.commandLine.text[e.commandLine.cursor:]...)
			e.commandLine.cursor = i
			e.commandLine.historyIndex = -1
		}
		return false
	case KeyRune:
		e.closeAutoComplete()
		// Insert char at cursor position
		e.commandLine.text = append(e.commandLine.text[:e.commandLine.cursor], append([]rune{ev.Rune()}, e.commandLine.text[e.commandLine.cursor:]...)...)
		e.commandLine.cursor++
		e.commandLine.historyIndex = -1
		return false
	}
	return false
}

// cmdHistoryUp navigates to older command in history
func (e *Editor) cmdHistoryUp() {
	if len(e.commandLine.history) == 0 {
		return
	}

	// First time pressing up - save current prefix for filtering
	if e.commandLine.historyIndex == -1 {
		e.commandLine.historyPrefix = string(e.commandLine.text)
		e.commandLine.historyIndex = len(e.commandLine.history)
	}

	// Find previous matching command
	for i := e.commandLine.historyIndex - 1; i >= 0; i-- {
		if strings.HasPrefix(e.commandLine.history[i], e.commandLine.historyPrefix) {
			e.commandLine.historyIndex = i
			e.commandLine.text = []rune(e.commandLine.history[i])
			e.commandLine.cursor = len(e.commandLine.text)
			return
		}
	}
}

// cmdHistoryDown navigates to newer command in history
func (e *Editor) cmdHistoryDown() {
	if e.commandLine.historyIndex == -1 {
		return
	}

	// Find next matching command
	for i := e.commandLine.historyIndex + 1; i < len(e.commandLine.history); i++ {
		if strings.HasPrefix(e.commandLine.history[i], e.commandLine.historyPrefix) {
			e.commandLine.historyIndex = i
			e.commandLine.text = []rune(e.commandLine.history[i])
			e.commandLine.cursor = len(e.commandLine.text)
			return
		}
	}

	// No more matches - restore original prefix
	e.commandLine.historyIndex = -1
	e.commandLine.text = []rune(e.commandLine.historyPrefix)
	e.commandLine.cursor = len(e.commandLine.text)
}

// LoadCmdHistory loads command history from file
func (e *Editor) LoadCmdHistory() {
	path := e.commandLine.historyPath
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return // File doesn't exist yet, that's ok
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line != "" {
			e.commandLine.history = append(e.commandLine.history, line)
		}
	}
}

// saveCmdHistory saves command history to file
func (e *Editor) saveCmdHistory() {
	path := e.commandLine.historyPath
	if path == "" {
		return
	}
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	// Keep only last 1000 commands
	history := e.commandLine.history
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}
	data := strings.Join(history, "\n")
	_ = os.WriteFile(path, []byte(data), 0644)
}

// filterCommands returns commands matching the given prefix
func filterCommands(prefix string) []CommandInfo {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return AvailableCommands
	}
	var result []CommandInfo
	for _, cmd := range AvailableCommands {
		if strings.HasPrefix(cmd.Name, prefix) {
			result = append(result, cmd)
		}
	}
	return result
}

// closeAutoComplete closes the command autocomplete popup
func (e *Editor) closeAutoComplete() {
	e.cmdAutoComplete = commandAutocompleteState{index: -1}
}

// cmdAutoCompleteUp moves selection up (previous command)
func (e *Editor) cmdAutoCompleteUp() {
	n := len(e.cmdAutoComplete.items)
	if n == 0 {
		return
	}
	if e.cmdAutoComplete.index < 0 {
		e.cmdAutoComplete.index = n - 1
		return
	}
	e.cmdAutoComplete.index--
	if e.cmdAutoComplete.index < 0 {
		e.cmdAutoComplete.index = n - 1
	}
}

// cmdAutoCompleteDown moves selection down (next command)
func (e *Editor) cmdAutoCompleteDown() {
	n := len(e.cmdAutoComplete.items)
	if n == 0 {
		return
	}
	if e.cmdAutoComplete.index < 0 {
		e.cmdAutoComplete.index = 0
		return
	}
	e.cmdAutoComplete.index++
	if e.cmdAutoComplete.index >= n {
		e.cmdAutoComplete.index = 0
	}
}

// findCmdPosition finds the column and position within column for a command index
func (e *Editor) findCmdPosition(cmdIdx int) (col int, posInCol int) {
	if len(e.cmdAutoComplete.colGroups) == 0 {
		return 0, cmdIdx
	}

	globalIdx := 0
	for colIdx, colGrps := range e.cmdAutoComplete.colGroups {
		for _, grp := range colGrps {
			for range grp.Commands {
				if globalIdx == cmdIdx {
					return colIdx, posInCol
				}
				globalIdx++
				posInCol++
			}
		}
		posInCol = 0
	}
	return 0, 0
}

// countCmdsInCol counts commands in a column
func (e *Editor) countCmdsInCol(colIdx int) int {
	if colIdx < 0 || colIdx >= len(e.cmdAutoComplete.colGroups) {
		return 0
	}
	count := 0
	for _, grp := range e.cmdAutoComplete.colGroups[colIdx] {
		count += len(grp.Commands)
	}
	return count
}

// firstCmdInCol returns the global command index of the first command in a column
func (e *Editor) firstCmdInCol(colIdx int) int {
	if colIdx < 0 || colIdx >= len(e.cmdAutoComplete.colGroups) {
		return 0
	}
	idx := 0
	for c := 0; c < colIdx; c++ {
		idx += e.countCmdsInCol(c)
	}
	return idx
}

// cmdAutoCompleteLeft moves selection to the previous column (same relative position)
func (e *Editor) cmdAutoCompleteLeft() {
	n := len(e.cmdAutoComplete.items)
	if n == 0 {
		return
	}
	if e.cmdAutoComplete.index < 0 {
		e.cmdAutoComplete.index = 0
		return
	}
	cols := len(e.cmdAutoComplete.colGroups)
	if cols <= 1 {
		return
	}

	// Find current position
	currentCol, posInCol := e.findCmdPosition(e.cmdAutoComplete.index)

	// Move to previous column
	targetCol := currentCol - 1
	if targetCol < 0 {
		targetCol = cols - 1
	}

	// Find position in target column
	targetColCmds := e.countCmdsInCol(targetCol)
	if targetColCmds == 0 {
		return
	}

	// Clamp position to target column size
	targetPos := posInCol
	if targetPos >= targetColCmds {
		targetPos = targetColCmds - 1
	}

	e.cmdAutoComplete.index = e.firstCmdInCol(targetCol) + targetPos
}

// cmdAutoCompleteRight moves selection to the next column (same relative position)
func (e *Editor) cmdAutoCompleteRight() {
	n := len(e.cmdAutoComplete.items)
	if n == 0 {
		return
	}
	if e.cmdAutoComplete.index < 0 {
		e.cmdAutoComplete.index = 0
		return
	}
	cols := len(e.cmdAutoComplete.colGroups)
	if cols <= 1 {
		return
	}

	// Find current position
	currentCol, posInCol := e.findCmdPosition(e.cmdAutoComplete.index)

	// Move to next column
	targetCol := currentCol + 1
	if targetCol >= cols {
		targetCol = 0
	}

	// Find position in target column
	targetColCmds := e.countCmdsInCol(targetCol)
	if targetColCmds == 0 {
		return
	}

	// Clamp position to target column size
	targetPos := posInCol
	if targetPos >= targetColCmds {
		targetPos = targetColCmds - 1
	}

	e.cmdAutoComplete.index = e.firstCmdInCol(targetCol) + targetPos
}

// updateCmdFromAutocomplete updates the command line with the selected autocomplete item
func (e *Editor) updateCmdFromAutocomplete() {
	if len(e.cmdAutoComplete.items) == 0 || e.cmdAutoComplete.index < 0 || e.cmdAutoComplete.index >= len(e.cmdAutoComplete.items) {
		return
	}
	selected := e.cmdAutoComplete.items[e.cmdAutoComplete.index]
	e.commandLine.text = []rune(commandInsertText(selected.Name))
	e.commandLine.cursor = len(e.commandLine.text)
}

func commandInsertText(name string) string {
	idx := strings.Index(name, "<")
	if idx == -1 {
		return name
	}
	base := strings.TrimRight(name[:idx], " ")
	return base + " "
}
