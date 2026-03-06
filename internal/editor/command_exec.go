package editor

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func (e *Editor) execCommand(cmd string) bool {
	if cmd == "" {
		return false
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
