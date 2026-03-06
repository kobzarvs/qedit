package app

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/gitinfo"
	"github.com/kobzarvs/qedit/internal/logger"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/session"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

type editorRuntimeController struct {
	ed                *editor.Editor
	screen            tcell.Screen
	ls                *lsp.Manager
	ts                *treesitter.Engine
	langs             config.Languages
	highlightMaxBytes int64
	sessionMgr        *session.Manager
	fileMonitor       *externalFileMonitor
	state             *editorRuntimeState
}

func (c *editorRuntimeController) openFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	absPath := path
	if abs, err := filepath.Abs(path); err == nil {
		absPath = abs
	}
	if c.state.openPath == absPath {
		return nil
	}
	if err := c.ed.OpenFile(absPath); err != nil {
		return err
	}
	state := activateEditorFile(c.ed, c.screen, c.ls, c.ts, c.langs, absPath, c.highlightMaxBytes)
	c.state.applyActiveFile(state)

	c.fileMonitor.Watch(absPath)
	c.ed.SetGitMainBranch("")
	syncEditorRepoState(c.ed, c.state.gitPath, c.sessionMgr)
	c.state.lastGitCheck = time.Now()
	return nil
}

func (c *editorRuntimeController) switchToWorktree(targetPath string) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return
	}
	if abs, err := filepath.Abs(targetPath); err == nil {
		targetPath = abs
	}
	candidate := pickWorktreeFile(targetPath, c.state.openPath)
	if candidate == "" {
		c.ed.SetGitBranch(gitinfo.Branch(targetPath))
		c.ed.SetGitRoot(gitinfo.Root(targetPath))
		c.state.gitPath = targetPath
		c.ed.SetStatusMessage("worktree switched (open a file)")
		return
	}
	if err := c.openFile(candidate); err != nil {
		c.ed.SetStatusMessage(err.Error())
		return
	}
	c.ed.SetGitBranch(gitinfo.Branch(candidate))
	c.ed.SetGitRoot(gitinfo.Root(candidate))
	c.ed.SetStatusMessage("worktree switched")
}

func (c *editorRuntimeController) handleEditorRequests() {
	if c.ed.ConsumeBranchPickerRequest() {
		showSidebarBranches(c.ed, c.state.gitPath)
	}
	if c.ed.ConsumeWorktreeListRequest() {
		showSidebarWorktrees(c.ed, c.state.gitPath)
	}
	if branch := c.ed.ConsumeSidebarBranchSelection(); branch != "" {
		logger.Debug("sidebar branch selected", "branch", branch)
		checkoutBranch(c.ed, c.state.gitPath, branch)
	} else if branch, ok := c.ed.ConsumeBranchSelection(); ok {
		checkoutBranch(c.ed, c.state.gitPath, branch)
	}
	if path := c.ed.ConsumeSidebarWorktreeSelection(); path != "" {
		logger.Debug("sidebar worktree selected", "path", path)
		if c.state.gitPath == "" {
			c.ed.SetStatusMessage("not a git repository")
		} else {
			c.switchToWorktree(path)
		}
	}
	if path, ok := c.ed.ConsumeSidebarOpenFile(); ok {
		err := c.openFile(path)
		if err != nil {
			c.ed.SetStatusMessage(err.Error())
		}
		c.ed.ApplyPendingGitDiffJump()
		if locPath, line, col, ok := c.ed.ConsumePendingOpenLocation(); ok {
			if err == nil && (locPath == "" || locPath == path) {
				c.ed.JumpToLocation(line, col)
			}
		}
	}
	if c.ed.ConsumeBufferSwitch() {
		path := c.ed.Filename()
		if path != c.state.openPath {
			c.fileMonitor.Watch(path)
			state := activateEditorFile(c.ed, c.screen, c.ls, c.ts, c.langs, path, c.highlightMaxBytes)
			c.state.applyActiveFile(state)

			c.ed.SetGitBranch(gitinfo.Branch(path))
			c.ed.SetGitRoot(gitinfo.Root(path))
		}
	}
}
