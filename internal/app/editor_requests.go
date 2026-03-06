package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/session"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

type editorRuntimeController struct {
	ed                *editor.Editor
	cfg               *config.Config
	screen            tcell.Screen
	clipboard         editor.Clipboard
	formatter         editor.Formatter
	ls                *lsp.Manager
	ts                *treesitter.Engine
	langs             config.Languages
	highlightMaxBytes int64
	sessionMgr        *session.Manager
	fileMonitor       *externalFileMonitor
	state             *editorRuntimeState
	fileStore         editor.FileStore
	gitRuntime        editor.GitRuntime
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
	return c.activateOpenFile(absPath)
}

func (c *editorRuntimeController) switchToWorktree(targetPath string) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return
	}
	targetPath = normalizeAppPath(c.fileStore, targetPath)
	candidate := pickWorktreeFile(c.gitRuntime, c.fileStore, targetPath, c.state.openPath)
	if candidate == "" {
		c.ed.SetGitBranch(c.gitRuntime.Branch(targetPath))
		c.ed.SetGitRoot(c.gitRuntime.Root(targetPath))
		c.state.gitPath = targetPath
		c.ed.SetStatusMessage("worktree switched (open a file)")
		return
	}
	if err := c.openFile(candidate); err != nil {
		c.ed.SetStatusMessage(err.Error())
		return
	}
	c.ed.SetGitBranch(c.gitRuntime.Branch(candidate))
	c.ed.SetGitRoot(c.gitRuntime.Root(candidate))
	c.ed.SetStatusMessage("worktree switched")
}
