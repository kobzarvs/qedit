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
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	case KeyCtrlC:
		e.closeAutoComplete()
		e.mode = ModeNormal
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	case KeyTab:
		prefix := string(e.cmd)
		if !e.cmdAutoCompleteActive {
			e.cmdAutoCompleteItems = filterCommands(prefix)
			if len(e.cmdAutoCompleteItems) == 0 {
				return false
			}
			e.cmdAutoCompleteActive = true
			e.cmdAutoCompleteIndex = -1
			e.cmdAutoCompleteCols = 1 // Will be recalculated on render
		} else {
			// Tab moves to next item (down in column, then next column top)
			if e.cmdAutoCompleteIndex < 0 {
				e.cmdAutoCompleteIndex = 0
			} else {
				e.cmdAutoCompleteIndex++
				if e.cmdAutoCompleteIndex >= len(e.cmdAutoCompleteItems) {
					e.cmdAutoCompleteIndex = 0
				}
			}
		}
		e.updateCmdFromAutocomplete()
		return false
	case KeyBacktab:
		if e.cmdAutoCompleteActive {
			// Shift+Tab moves to previous item (up in column, then prev column bottom)
			if e.cmdAutoCompleteIndex < 0 {
				e.cmdAutoCompleteIndex = len(e.cmdAutoCompleteItems) - 1
			} else {
				e.cmdAutoCompleteIndex--
				if e.cmdAutoCompleteIndex < 0 {
					e.cmdAutoCompleteIndex = len(e.cmdAutoCompleteItems) - 1
				}
			}
			e.updateCmdFromAutocomplete()
		}
		return false
	case KeyEnter:
		e.closeAutoComplete()
		cmd := strings.TrimSpace(string(e.cmd))
		e.mode = ModeNormal
		// Add to history if not empty and different from last
		if cmd != "" && (len(e.cmdHistory) == 0 || e.cmdHistory[len(e.cmdHistory)-1] != cmd) {
			e.cmdHistory = append(e.cmdHistory, cmd)
			e.saveCmdHistory()
		}
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return e.execCommand(cmd)
	case KeyBackspace, KeyBackspace2:
		e.closeAutoComplete()
		if e.cmdCursor > 0 && len(e.cmd) > 0 {
			// Delete char before cursor
			e.cmd = append(e.cmd[:e.cmdCursor-1], e.cmd[e.cmdCursor:]...)
			e.cmdCursor--
			e.cmdHistoryIndex = -1
		}
		return false
	case KeyDelete:
		e.closeAutoComplete()
		if e.cmdCursor < len(e.cmd) {
			// Delete char at cursor
			e.cmd = append(e.cmd[:e.cmdCursor], e.cmd[e.cmdCursor+1:]...)
			e.cmdHistoryIndex = -1
		}
		return false
	case KeyLeft, KeyCtrlB: // Ctrl+B = back (readline)
		if e.cmdAutoCompleteActive {
			e.cmdAutoCompleteLeft()
			e.updateCmdFromAutocomplete()
			return false
		}
		if e.cmdCursor > 0 {
			e.cmdCursor--
		}
		return false
	case KeyRight, KeyCtrlF: // Ctrl+F = forward (readline)
		if e.cmdAutoCompleteActive {
			e.cmdAutoCompleteRight()
			e.updateCmdFromAutocomplete()
			return false
		}
		if e.cmdCursor < len(e.cmd) {
			e.cmdCursor++
		}
		return false
	case KeyHome, KeyCtrlA: // Ctrl+A = beginning of line
		e.cmdCursor = 0
		return false
	case KeyEnd, KeyCtrlE: // Ctrl+E = end of line
		e.cmdCursor = len(e.cmd)
		return false
	case KeyUp, KeyCtrlP: // Ctrl+P = previous
		if e.cmdAutoCompleteActive {
			e.cmdAutoCompleteUp()
			e.updateCmdFromAutocomplete()
			return false
		}
		e.cmdHistoryUp()
		return false
	case KeyDown, KeyCtrlN: // Ctrl+N = next
		if e.cmdAutoCompleteActive {
			e.cmdAutoCompleteDown()
			e.updateCmdFromAutocomplete()
			return false
		}
		e.cmdHistoryDown()
		return false
	case KeyCtrlU: // Ctrl+U = clear line
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	case KeyCtrlK: // Ctrl+K = kill to end of line
		e.cmd = e.cmd[:e.cmdCursor]
		e.cmdHistoryIndex = -1
		return false
	case KeyCtrlW: // Ctrl+W = delete word backward
		if e.cmdCursor > 0 {
			// Find start of previous word
			i := e.cmdCursor - 1
			for i > 0 && e.cmd[i-1] == ' ' {
				i--
			}
			for i > 0 && e.cmd[i-1] != ' ' {
				i--
			}
			e.cmd = append(e.cmd[:i], e.cmd[e.cmdCursor:]...)
			e.cmdCursor = i
			e.cmdHistoryIndex = -1
		}
		return false
	case KeyRune:
		e.closeAutoComplete()
		// Insert char at cursor position
		e.cmd = append(e.cmd[:e.cmdCursor], append([]rune{ev.Rune()}, e.cmd[e.cmdCursor:]...)...)
		e.cmdCursor++
		e.cmdHistoryIndex = -1
		return false
	}
	return false
}

// cmdHistoryUp navigates to older command in history
func (e *Editor) cmdHistoryUp() {
	if len(e.cmdHistory) == 0 {
		return
	}

	// First time pressing up - save current prefix for filtering
	if e.cmdHistoryIndex == -1 {
		e.cmdHistoryPrefix = string(e.cmd)
		e.cmdHistoryIndex = len(e.cmdHistory)
	}

	// Find previous matching command
	for i := e.cmdHistoryIndex - 1; i >= 0; i-- {
		if strings.HasPrefix(e.cmdHistory[i], e.cmdHistoryPrefix) {
			e.cmdHistoryIndex = i
			e.cmd = []rune(e.cmdHistory[i])
			e.cmdCursor = len(e.cmd)
			return
		}
	}
}

// cmdHistoryDown navigates to newer command in history
func (e *Editor) cmdHistoryDown() {
	if e.cmdHistoryIndex == -1 {
		return
	}

	// Find next matching command
	for i := e.cmdHistoryIndex + 1; i < len(e.cmdHistory); i++ {
		if strings.HasPrefix(e.cmdHistory[i], e.cmdHistoryPrefix) {
			e.cmdHistoryIndex = i
			e.cmd = []rune(e.cmdHistory[i])
			e.cmdCursor = len(e.cmd)
			return
		}
	}

	// No more matches - restore original prefix
	e.cmdHistoryIndex = -1
	e.cmd = []rune(e.cmdHistoryPrefix)
	e.cmdCursor = len(e.cmd)
}

// LoadCmdHistory loads command history from file
func (e *Editor) LoadCmdHistory() {
	path := e.cmdHistoryPath
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
			e.cmdHistory = append(e.cmdHistory, line)
		}
	}
}

// saveCmdHistory saves command history to file
func (e *Editor) saveCmdHistory() {
	path := e.cmdHistoryPath
	if path == "" {
		return
	}
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	// Keep only last 1000 commands
	history := e.cmdHistory
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
	e.cmdAutoCompleteActive = false
	e.cmdAutoCompleteItems = nil
	e.cmdAutoCompleteIndex = -1
	e.cmdAutoCompleteCols = 0
	e.cmdAutoCompleteColGroups = nil
}

// cmdAutoCompleteUp moves selection up (previous command)
func (e *Editor) cmdAutoCompleteUp() {
	n := len(e.cmdAutoCompleteItems)
	if n == 0 {
		return
	}
	if e.cmdAutoCompleteIndex < 0 {
		e.cmdAutoCompleteIndex = n - 1
		return
	}
	e.cmdAutoCompleteIndex--
	if e.cmdAutoCompleteIndex < 0 {
		e.cmdAutoCompleteIndex = n - 1
	}
}

// cmdAutoCompleteDown moves selection down (next command)
func (e *Editor) cmdAutoCompleteDown() {
	n := len(e.cmdAutoCompleteItems)
	if n == 0 {
		return
	}
	if e.cmdAutoCompleteIndex < 0 {
		e.cmdAutoCompleteIndex = 0
		return
	}
	e.cmdAutoCompleteIndex++
	if e.cmdAutoCompleteIndex >= n {
		e.cmdAutoCompleteIndex = 0
	}
}

// findCmdPosition finds the column and position within column for a command index
func (e *Editor) findCmdPosition(cmdIdx int) (col int, posInCol int) {
	if len(e.cmdAutoCompleteColGroups) == 0 {
		return 0, cmdIdx
	}

	globalIdx := 0
	for colIdx, colGrps := range e.cmdAutoCompleteColGroups {
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
	if colIdx < 0 || colIdx >= len(e.cmdAutoCompleteColGroups) {
		return 0
	}
	count := 0
	for _, grp := range e.cmdAutoCompleteColGroups[colIdx] {
		count += len(grp.Commands)
	}
	return count
}

// firstCmdInCol returns the global command index of the first command in a column
func (e *Editor) firstCmdInCol(colIdx int) int {
	if colIdx < 0 || colIdx >= len(e.cmdAutoCompleteColGroups) {
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
	n := len(e.cmdAutoCompleteItems)
	if n == 0 {
		return
	}
	if e.cmdAutoCompleteIndex < 0 {
		e.cmdAutoCompleteIndex = 0
		return
	}
	cols := len(e.cmdAutoCompleteColGroups)
	if cols <= 1 {
		return
	}

	// Find current position
	currentCol, posInCol := e.findCmdPosition(e.cmdAutoCompleteIndex)

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

	e.cmdAutoCompleteIndex = e.firstCmdInCol(targetCol) + targetPos
}

// cmdAutoCompleteRight moves selection to the next column (same relative position)
func (e *Editor) cmdAutoCompleteRight() {
	n := len(e.cmdAutoCompleteItems)
	if n == 0 {
		return
	}
	if e.cmdAutoCompleteIndex < 0 {
		e.cmdAutoCompleteIndex = 0
		return
	}
	cols := len(e.cmdAutoCompleteColGroups)
	if cols <= 1 {
		return
	}

	// Find current position
	currentCol, posInCol := e.findCmdPosition(e.cmdAutoCompleteIndex)

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

	e.cmdAutoCompleteIndex = e.firstCmdInCol(targetCol) + targetPos
}

// updateCmdFromAutocomplete updates the command line with the selected autocomplete item
func (e *Editor) updateCmdFromAutocomplete() {
	if len(e.cmdAutoCompleteItems) == 0 || e.cmdAutoCompleteIndex < 0 || e.cmdAutoCompleteIndex >= len(e.cmdAutoCompleteItems) {
		return
	}
	selected := e.cmdAutoCompleteItems[e.cmdAutoCompleteIndex]
	e.cmd = []rune(commandInsertText(selected.Name))
	e.cmdCursor = len(e.cmd)
}

func commandInsertText(name string) string {
	idx := strings.Index(name, "<")
	if idx == -1 {
		return name
	}
	base := strings.TrimRight(name[:idx], " ")
	return base + " "
}
