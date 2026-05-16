package editor

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func (e *Editor) execCommand(cmd string) bool {
	if cmd == "" {
		return false
	}
	if strings.HasPrefix(cmd, "!") {
		return e.executeShellCommand(strings.TrimSpace(cmd[1:]))
	}
	if strings.HasPrefix(cmd, "'<,'>") {
		return e.executeVisualRangeCommand(strings.TrimSpace(strings.TrimPrefix(cmd, "'<,'>")))
	}
	if handled, quit := e.executeSubstituteCommand(cmd); handled {
		return quit
	}
	fields := strings.Fields(cmd)
	name := fields[0]
	args := fields[1:]

	if def, ok := e.commands.Lookup(name); ok && def.Handle != nil {
		return def.Handle(e, args)
	}

	// Check if command is a line number
	if lineNum, err := strconv.Atoi(name); err == nil && lineNum > 0 {
		e.gotoLineNumber(lineNum)
		return false
	}
	e.setStatus("unknown command: " + name)
	return false
}

func (e *Editor) executeShellCommand(cmd string) bool {
	if cmd == "" {
		e.setStatus("shell command required")
		return false
	}
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		e.setStatus("shell: " + text)
		return false
	}
	if text == "" {
		e.setStatus("shell command completed")
		return false
	}
	e.setStatus("shell: " + firstStatusLine(text))
	return false
}

func firstStatusLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

func (e *Editor) executeVisualRangeCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		e.setStatus("visual range command required")
		return false
	}
	switch fields[0] {
	case "w", "write":
		return e.executeVisualWriteCommand(fields[1:])
	default:
		e.setStatus("unsupported visual range command: " + fields[0])
		return false
	}
}

func (e *Editor) executeVisualWriteCommand(args []string) bool {
	if len(args) == 0 {
		e.setStatus("file name required")
		return false
	}
	if e.workspaceFileStore() == nil {
		e.setStatus("file store unavailable")
		return false
	}
	start, end, ok := e.selectionRange()
	if !ok {
		e.setStatus("no visual selection")
		return false
	}
	path := strings.Join(args, " ")
	if abs, err := e.workspaceAbs(path); err == nil {
		path = abs
	}
	data := []byte(e.textForRange(start, end))
	if err := e.workspaceWrite(path, data); err != nil {
		e.setStatus(err.Error())
		return false
	}
	if e.profile.name == BehaviorProfileVim && e.profile.vim.visual {
		e.exitVimVisualMode(true)
	}
	e.setStatus(fmt.Sprintf("written %d bytes to %s", len(data), path))
	return false
}

func (e *Editor) textForRange(start, end Cursor) string {
	if cursorLess(end, start) {
		start, end = end, start
	}
	var b strings.Builder
	for row := start.Row; row <= end.Row; row++ {
		if row < 0 || row >= e.LineCount() {
			continue
		}
		if row > start.Row {
			b.WriteByte('\n')
		}
		line := e.line(row)
		startCol := 0
		endCol := len(line)
		if row == start.Row {
			startCol = start.Col
		}
		if row == end.Row {
			endCol = end.Col
		}
		if startCol < 0 {
			startCol = 0
		}
		if endCol < startCol {
			endCol = startCol
		}
		if endCol > len(line) {
			endCol = len(line)
		}
		b.WriteString(string(line[startCol:endCol]))
	}
	return b.String()
}

func (e *Editor) executeSubstituteCommand(cmd string) (bool, bool) {
	startRow, endRow, body, ok := e.parseSubstituteRange(cmd)
	if !ok {
		return false, false
	}
	if !looksLikeSubstituteCommand(body) {
		e.setStatus("usage: s/old/new/[g]")
		return true, false
	}
	oldText, newText, flags, ok := parseSubstituteBody(body[1:])
	if !ok || oldText == "" {
		e.setStatus("usage: s/old/new/[g]")
		return true, false
	}
	global := strings.Contains(flags, "g")
	replacements := 0
	e.startUndoGroup()
	for row := startRow; row <= endRow && row < e.LineCount(); row++ {
		line := string(e.line(row))
		var next string
		count := 0
		if global {
			count = strings.Count(line, oldText)
			next = strings.ReplaceAll(line, oldText, newText)
		} else if strings.Contains(line, oldText) {
			count = 1
			next = strings.Replace(line, oldText, newText, 1)
		} else {
			next = line
		}
		if count == 0 {
			continue
		}
		replacements += count
		start := Cursor{Row: row, Col: 0}
		end := Cursor{Row: row, Col: len([]rune(line))}
		deleted := e.deleteTextRange(start, end)
		e.appendUndo(action{kind: actionInsertText, pos: start, text: deleted})
		inserted := [][]rune{[]rune(next)}
		endPos := e.insertTextAt(start, inserted)
		e.appendUndo(action{kind: actionDeleteText, pos: start, endPos: endPos, text: inserted})
	}
	e.finishUndoGroup()
	if replacements == 0 {
		e.setStatus("pattern not found: " + oldText)
		return true, false
	}
	e.change.lastEdit.Valid = false
	e.setStatus(fmt.Sprintf("%d substitutions", replacements))
	return true, false
}

func (e *Editor) parseSubstituteRange(cmd string) (int, int, string, bool) {
	body := strings.TrimSpace(cmd)
	if body == "" {
		return 0, 0, "", false
	}
	startRow, endRow := e.cursor.Row, e.cursor.Row
	if strings.HasPrefix(body, "%") {
		startRow = 0
		endRow = e.LineCount() - 1
		body = body[1:]
	} else if body[0] >= '0' && body[0] <= '9' {
		idx := strings.IndexByte(body, 's')
		if idx <= 0 {
			return 0, 0, "", false
		}
		rangeText := body[:idx]
		parts := strings.Split(rangeText, ",")
		if len(parts) > 2 {
			return 0, 0, "", false
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil || start < 1 {
			return 0, 0, "", false
		}
		end := start
		if len(parts) == 2 {
			end, err = strconv.Atoi(parts[1])
			if err != nil || end < start {
				return 0, 0, "", false
			}
		}
		startRow = start - 1
		endRow = end - 1
		body = body[idx:]
	}
	if !looksLikeSubstituteCommand(body) {
		return 0, 0, "", false
	}
	if startRow < 0 {
		startRow = 0
	}
	if endRow >= e.LineCount() {
		endRow = e.LineCount() - 1
	}
	return startRow, endRow, body, true
}

func looksLikeSubstituteCommand(body string) bool {
	if len(body) < 2 || body[0] != 's' {
		return false
	}
	delim := body[1]
	if delim == ' ' || delim == '\t' {
		return false
	}
	if delim >= '0' && delim <= '9' {
		return false
	}
	if delim >= 'A' && delim <= 'Z' {
		return false
	}
	if delim >= 'a' && delim <= 'z' {
		return false
	}
	return true
}

func parseSubstituteBody(body string) (string, string, string, bool) {
	runes := []rune(body)
	if len(runes) == 0 {
		return "", "", "", false
	}
	delim := runes[0]
	parts := make([][]rune, 0, 3)
	current := make([]rune, 0)
	escaped := false
	for _, r := range runes[1:] {
		if escaped {
			current = append(current, r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == delim && len(parts) < 2 {
			parts = append(parts, current)
			current = make([]rune, 0)
			continue
		}
		current = append(current, r)
	}
	if escaped {
		current = append(current, '\\')
	}
	if len(parts) != 2 {
		return "", "", "", false
	}
	return string(parts[0]), string(parts[1]), string(current), true
}

func (e *Editor) executeWriteCommand(args []string, quitAfter bool) bool {
	path := ""
	if len(args) > 0 {
		path = strings.Join(args, " ")
	}
	if err := e.queueSaveRequest(path, quitAfter); err != nil {
		e.setStatus(err.Error())
		return false
	}
	return false
}

func (e *Editor) executeReloadCommand(args []string, force bool) bool {
	if len(args) > 0 {
		e.setStatus("edit file not supported")
		return false
	}
	if err := e.queueReloadRequest(force); err != nil {
		e.setStatus(err.Error())
		return false
	}
	return false
}

func (e *Editor) executeReadCommand(args []string) bool {
	target := strings.TrimSpace(strings.Join(args, " "))
	if target == "" {
		e.setStatus("usage: r {file|!cmd}")
		return false
	}
	var data []byte
	var label string
	var err error
	if strings.HasPrefix(target, "!") {
		cmd := strings.TrimSpace(target[1:])
		if cmd == "" {
			e.setStatus("shell command required")
			return false
		}
		data, err = exec.Command("sh", "-c", cmd).Output()
		label = "!" + cmd
	} else {
		path := target
		if e.workspaceFileStore() != nil {
			if abs, absErr := e.workspaceAbs(target); absErr == nil {
				path = abs
			}
		}
		if e.workspaceFileStore() == nil {
			e.setStatus("file store unavailable")
			return false
		}
		data, err = e.workspaceRead(path)
		label = path
	}
	if err != nil {
		e.setStatus(err.Error())
		return false
	}
	inserted := e.insertReadTextBelowCursor(string(data))
	e.setStatus(fmt.Sprintf("read %d lines from %s", inserted, label))
	return false
}

func (e *Editor) insertReadTextBelowCursor(text string) int {
	text = strings.TrimRight(text, "\n")
	if text == "" || e.LineCount() == 0 {
		return 0
	}
	lines := splitRunesByNewline([]rune("\n" + text))
	if len(lines) == 0 {
		return 0
	}
	pos := Cursor{Row: e.cursor.Row, Col: e.lineLen(e.cursor.Row)}
	e.startUndoGroup()
	endPos := e.insertTextAt(pos, lines)
	e.appendUndo(action{kind: actionDeleteText, pos: pos, endPos: endPos, text: lines})
	e.finishUndoGroup()
	e.cursor = Cursor{Row: pos.Row + 1, Col: 0}
	e.change.lastEdit.Valid = false
	return len(lines) - 1
}

func (e *Editor) executeQuitCommand(force bool) bool {
	if force {
		return true
	}
	if e.buffers != nil && e.buffers.HasDirtyBuffers() {
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
		if e.buffers.HasDirtyBuffers() {
			e.setStatus("unsaved changes in open buffers (use :q!)")
			return false
		}
	} else if e.document.dirty {
		e.setStatus("unsaved changes (use :q!)")
		return false
	}
	return true
}

func (e *Editor) executeLineNumberCommand(args []string) bool {
	if len(args) == 0 {
		e.toggleLineNumbers()
		return false
	}
	switch strings.ToLower(args[0]) {
	case "off":
		e.display.lineNumberMode = LineNumberOff
		e.setStatus("line numbers off")
	case "abs", "absolute":
		e.display.lineNumberMode = LineNumberAbsolute
		e.setStatus("line numbers absolute")
	case "rel", "relative":
		e.display.lineNumberMode = LineNumberRelative
		e.setStatus("line numbers relative")
	default:
		e.setStatus("unknown line number mode")
	}
	return false
}

func (e *Editor) executeSetCommand(args []string) bool {
	if len(args) == 0 {
		ignore := "noignorecase"
		if e.searchIgnoreCase {
			ignore = "ignorecase"
		}
		highlight := "nohlsearch"
		if e.searchHighlight {
			highlight = "hlsearch"
		}
		e.setStatus(ignore + " " + highlight)
		return false
	}

	var changedSearch bool
	var status []string
	for _, arg := range args {
		opt := strings.ToLower(strings.TrimSpace(arg))
		switch opt {
		case "ic", "ignorecase":
			e.searchIgnoreCase = true
			changedSearch = true
			status = append(status, "ignorecase")
		case "noic", "noignorecase":
			e.searchIgnoreCase = false
			changedSearch = true
			status = append(status, "noignorecase")
		case "invic", "invignorecase":
			e.searchIgnoreCase = !e.searchIgnoreCase
			changedSearch = true
			if e.searchIgnoreCase {
				status = append(status, "ignorecase")
			} else {
				status = append(status, "noignorecase")
			}
		case "hls", "hlsearch":
			e.searchHighlight = true
			status = append(status, "hlsearch")
		case "nohls", "nohlsearch":
			e.searchHighlight = false
			e.searchMatches = nil
			e.searchMatchIndex = 0
			status = append(status, "nohlsearch")
		case "invhls", "invhlsearch":
			e.searchHighlight = !e.searchHighlight
			if e.searchHighlight {
				status = append(status, "hlsearch")
			} else {
				e.searchMatches = nil
				e.searchMatchIndex = 0
				status = append(status, "nohlsearch")
			}
		case "is", "incsearch":
			status = append(status, "incsearch")
		case "nois", "noincsearch":
			status = append(status, "noincsearch")
		default:
			e.setStatus("unknown option: " + arg)
			return false
		}
	}
	if changedSearch {
		e.searchMatches = nil
		e.searchMatchIndex = 0
	}
	e.setStatus(strings.Join(status, " "))
	return false
}

func (e *Editor) executeNoHLSearchCommand() bool {
	e.searchMatches = nil
	e.searchMatchIndex = 0
	e.setStatus("nohlsearch")
	return false
}

func (e *Editor) executeTreeCommand(args []string) bool {
	path := ""
	if len(args) > 0 {
		path = strings.Join(args, " ")
	}
	e.openSidebarFileTree(path)
	return false
}

func (e *Editor) executeSidebarWidthCommand(args []string) bool {
	if len(args) == 0 {
		if e.sidebar != nil {
			e.setStatus("sidebar width: " + e.sidebar.WidthConfig)
		}
		return false
	}
	if e.sidebar != nil {
		prevWidth := e.sidebar.WidthConfig
		e.sidebar.WidthConfig = args[0]
		e.enqueueRuntimeRequest(RuntimeRequest{
			Kind:      RuntimeRequestPersistSidebarWidth,
			Value:     args[0],
			PrevValue: prevWidth,
		})
		e.setStatus("sidebar width set to " + args[0])
	}
	return false
}

func (e *Editor) executeAutoReloadCommand(args []string) bool {
	if len(args) == 0 {
		e.setStatus("auto-reload-on-changes=" + boolToFlag(e.file.autoReloadOnChanges))
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
	if enabled == e.file.autoReloadOnChanges {
		e.setStatus("auto-reload-on-changes=" + boolToFlag(e.file.autoReloadOnChanges))
		return false
	}
	prevEnabled := e.file.autoReloadOnChanges
	e.file.autoReloadOnChanges = enabled
	e.enqueueRuntimeRequest(RuntimeRequest{
		Kind:     RuntimeRequestPersistAutoReload,
		Bool:     enabled,
		PrevBool: prevEnabled,
	})
	e.setStatus("auto-reload-on-changes=" + boolToFlag(e.file.autoReloadOnChanges))
	return false
}

func (e *Editor) executeProfileCommand(args []string) bool {
	if len(args) == 0 {
		e.setStatus("profile=" + e.BehaviorProfile())
		return false
	}
	if len(args) > 1 {
		e.setStatus("usage: profile basic|helix|vim")
		return false
	}
	next := strings.ToLower(strings.TrimSpace(args[0]))
	prev := e.BehaviorProfile()
	if next == prev {
		e.setStatus("profile=" + prev)
		return false
	}
	if !e.SetBehaviorProfile(next) {
		e.setStatus("unknown profile: " + next)
		return false
	}
	e.enqueueRuntimeRequest(RuntimeRequest{
		Kind:      RuntimeRequestPersistProfile,
		Value:     next,
		PrevValue: prev,
	})
	e.setStatus("profile=" + e.BehaviorProfile())
	return false
}

func (e *Editor) handleWorktreeCommand(args []string) bool {
	root := e.git.root
	if root == "" {
		root = e.detectGitRoot()
	}
	if root == "" {
		e.setStatus("not a git repository")
		return false
	}
	if len(args) == 0 {
		e.openSidebarWorktrees()
		return false
	}
	switch strings.ToLower(args[0]) {
	case "list":
		e.openSidebarWorktrees()
		return false
	case "refresh":
		e.refreshSidebarWorktrees()
		return false
	case "new":
		name := strings.TrimSpace(strings.Join(args[1:], " "))
		if name == "" {
			e.setStatus("usage: worktree new <name>")
			return false
		}
		path, err := e.gitAddWorktree(root, name)
		if err != nil {
			e.setStatus(err.Error())
			return false
		}
		e.setStatus("worktree created: " + name)
		e.requestWorktreeRefreshIfActive()
		e.enqueueRuntimeRequest(RuntimeRequest{
			Kind: RuntimeRequestSwitchWorktree,
			Path: path,
		})
		return false
	case "switch":
		target := strings.TrimSpace(strings.Join(args[1:], " "))
		if target == "" {
			e.setStatus("usage: worktree switch <name>")
			return false
		}
		path, err := e.resolveWorktreeTarget(root, target)
		if err != nil {
			e.setStatus(err.Error())
			return false
		}
		e.enqueueRuntimeRequest(RuntimeRequest{
			Kind: RuntimeRequestSwitchWorktree,
			Path: path,
		})
		return false
	case "remove":
		target := strings.TrimSpace(strings.Join(args[1:], " "))
		if target == "" {
			e.setStatus("usage: worktree remove <name>")
			return false
		}
		path, err := e.resolveWorktreeTarget(root, target)
		if err != nil {
			e.setStatus(err.Error())
			return false
		}
		if err := e.gitRemoveWorktree(root, path); err != nil {
			e.setStatus(err.Error())
			return false
		}
		e.setStatus("worktree removed: " + filepath.Base(path))
		e.requestWorktreeRefreshIfActive()
		return false
	default:
		e.setStatus("usage: worktree [list|new|switch|remove|refresh]")
		return false
	}
}

func (e *Editor) resolveWorktreeTarget(root, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errors.New("worktree name required")
	}
	worktrees, _, err := e.gitListWorktrees(root)
	if err != nil {
		return "", err
	}
	targetAbs := target
	if abs := e.normalizedPath(target); abs != "" {
		targetAbs = abs
	}
	for _, wt := range worktrees {
		if filepath.Clean(wt.Path) == filepath.Clean(target) || filepath.Clean(wt.Path) == filepath.Clean(targetAbs) {
			return wt.Path, nil
		}
	}
	for _, wt := range worktrees {
		if wt.Branch == target {
			return wt.Path, nil
		}
	}
	for _, wt := range worktrees {
		if filepath.Base(wt.Path) == target {
			return wt.Path, nil
		}
	}
	return "", fmt.Errorf("worktree not found: %s", target)
}

func (e *Editor) prefillCommand(text string) {
	e.mode = ModeCommand
	e.commandLine.text = []rune(text)
	e.commandLine.cursor = len(e.commandLine.text)
	e.commandLine.historyIndex = -1
	e.closeAutoComplete()
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
	e.interaction.freeScroll = false
	e.viewport.scrollX = 0
	e.primeHugeRowsAround(e.cursor.Row)
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
