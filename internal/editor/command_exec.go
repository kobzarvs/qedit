package editor

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kobzarvs/qedit/internal/gitinfo"
)

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
		// Update buffer manager with saved state
		if e.buffers != nil && e.buffers.Count() > 0 {
			bs := e.snapshotBufferState()
			e.buffers.UpdateActive(bs)
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
		if e.buffers != nil && e.buffers.HasDirtyBuffers() {
			// Update active buffer state first
			bs := e.snapshotBufferState()
			e.buffers.UpdateActive(bs)
			if e.buffers.HasDirtyBuffers() {
				e.setStatus("unsaved changes in open buffers (use :q!)")
				return false
			}
		} else if e.dirty {
			e.setStatus("unsaved changes (use :q!)")
			return false
		}
		return true
	case "q!":
		return true
	case "bn":
		e.gotoNextBuffer()
		return false
	case "bp":
		e.gotoPrevBuffer()
		return false
	case "bc":
		e.closeCurrentBuffer(false)
		return false
	case "bc!":
		e.closeCurrentBuffer(true)
		return false
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
	case "tree":
		path := ""
		if len(args) > 0 {
			path = strings.Join(args, " ")
		}
		e.openSidebarFileTree(path)
		return false
	case "sidew":
		// :sidew - show current width, :sidew 30 - set width
		if len(args) == 0 {
			if e.sidebar != nil {
				e.setStatus("sidebar width: " + e.sidebar.WidthConfig)
			}
		} else {
			if e.sidebar != nil {
				if e.runtime.sidebarWidthConfigHook != nil {
					if err := e.runtime.sidebarWidthConfigHook(args[0]); err != nil {
						e.setStatus("config write failed: " + err.Error())
						return false
					}
				}
				e.sidebar.WidthConfig = args[0]
				e.setStatus("sidebar width set to " + args[0])
			}
		}
		return false
	case "sidebar-focus":
		e.toggleSidebarFocus()
		return false
	case "focus-editor":
		e.focusEditor()
		return false
	case "autoreload", "auto-reload", "auto-reload-on-changes":
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
		if e.runtime.autoReloadConfigHook != nil {
			if err := e.runtime.autoReloadConfigHook(enabled); err != nil {
				e.setStatus("config write failed: " + err.Error())
				return false
			}
		}
		e.file.autoReloadOnChanges = enabled
		e.setStatus("auto-reload-on-changes=" + boolToFlag(e.file.autoReloadOnChanges))
		return false
	case "merge":
		if len(args) > 0 {
			e.setStatus("merge takes no arguments")
			return false
		}
		return e.enterMergeMode()
	case "worktree", "worktrees":
		return e.handleWorktreeCommand(args)
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
		path, err := gitinfo.AddWorktree(root, name)
		if err != nil {
			e.setStatus(err.Error())
			return false
		}
		e.setStatus("worktree created: " + name)
		e.requestWorktreeRefreshIfActive()
		e.requests.worktreeSelection = path
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
		e.requests.worktreeSelection = path
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
		if err := gitinfo.RemoveWorktree(root, path); err != nil {
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
	worktrees, _, err := gitinfo.ListWorktrees(root)
	if err != nil {
		return "", err
	}
	targetAbs := target
	if abs, err := filepath.Abs(target); err == nil {
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
	e.cmd = []rune(text)
	e.cmdCursor = len(e.cmd)
	e.cmdHistoryIndex = -1
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
