package app

import (
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
	fileStore         editor.FileStore
}

func (c *editorRuntimeController) openFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	absPath := normalizeAppPath(c.fileStore, path)
	if c.state.openPath == absPath {
		return nil
	}
	if err := c.ed.OpenFile(absPath); err != nil {
		return err
	}
	state := activateEditorFile(c.ed, c.screen, c.ls, c.ts, c.langs, c.fileStore, absPath, c.highlightMaxBytes)
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
	targetPath = normalizeAppPath(c.fileStore, targetPath)
	candidate := pickWorktreeFile(c.fileStore, targetPath, c.state.openPath)
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
	for {
		req, ok := c.ed.ConsumeRuntimeRequest()
		if !ok {
			return
		}
		switch req.Kind {
		case editor.RuntimeRequestShowBranchPicker:
			showSidebarBranches(c.ed, c.state.gitPath)
		case editor.RuntimeRequestShowWorktrees:
			showSidebarWorktrees(c.ed, c.state.gitPath)
		case editor.RuntimeRequestSelectBranch:
			if req.Value != "" {
				logger.Debug("branch selected", "branch", req.Value)
				checkoutBranch(c.ed, c.state.gitPath, req.Value)
			}
		case editor.RuntimeRequestSwitchWorktree:
			if req.Path == "" {
				continue
			}
			logger.Debug("worktree selected", "path", req.Path)
			if c.state.gitPath == "" {
				c.ed.SetStatusMessage("not a git repository")
			} else {
				c.switchToWorktree(req.Path)
			}
		case editor.RuntimeRequestOpenFile:
			if req.Path == "" {
				continue
			}
			err := c.openFile(req.Path)
			if err != nil {
				c.ed.SetStatusMessage(err.Error())
				continue
			}
			c.ed.ApplyPendingGitDiffJump()
			if req.Line >= 0 {
				c.ed.JumpToLocation(req.Line, req.Col)
			}
		case editor.RuntimeRequestBufferSwitched:
			path := c.ed.Filename()
			if path != c.state.openPath {
				c.fileMonitor.Watch(path)
				state := activateEditorFile(c.ed, c.screen, c.ls, c.ts, c.langs, c.fileStore, path, c.highlightMaxBytes)
				c.state.applyActiveFile(state)

				c.ed.SetGitBranch(gitinfo.Branch(path))
				c.ed.SetGitRoot(gitinfo.Root(path))
			}
		}
	}
}
