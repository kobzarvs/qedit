package app

import (
	"time"
)

func (c *editorRuntimeController) applyActivatedFile(state activeFileState) {
	if state.openPath == "" {
		return
	}
	c.state.applyActiveFile(state)
	c.fileMonitor.Watch(state.openPath)
}

func (c *editorRuntimeController) refreshActiveRepoState(gitPath string, resetMainBranch bool) {
	if resetMainBranch {
		c.ed.SetGitMainBranch("")
	}
	syncEditorRepoState(c.ed, gitPath, c.sessionMgr, c.gitRuntime)
	c.state.lastGitCheck = time.Now()
}

func (c *editorRuntimeController) activateOpenFile(path string) error {
	state, err := openRuntimeFile(c.ed, c.screen, c.ls, c.ts, c.langs, c.fileStore, path, c.highlightMaxBytes)
	if err != nil {
		return err
	}
	c.applyActivatedFile(state)
	c.refreshActiveRepoState(state.gitPath, true)
	return nil
}

func (c *editorRuntimeController) activateExistingBuffer(path string) {
	path = normalizeAppPath(c.fileStore, path)
	if path == "" {
		return
	}
	state := activateEditorFile(c.ed, c.screen, c.ls, c.ts, c.langs, c.fileStore, path, c.highlightMaxBytes)
	c.applyActivatedFile(state)
	c.ed.SetGitBranch(c.gitRuntime.Branch(path))
	c.ed.SetGitRoot(c.gitRuntime.Root(path))
	c.state.lastGitCheck = time.Now()
}

func (c *editorRuntimeController) activateCurrentEditorFile(path string, resetMainBranch bool) {
	path = normalizeAppPath(c.fileStore, path)
	if path == "" {
		return
	}
	state := activateEditorFile(c.ed, c.screen, c.ls, c.ts, c.langs, c.fileStore, path, c.highlightMaxBytes)
	c.applyActivatedFile(state)
	c.refreshActiveRepoState(state.gitPath, resetMainBranch)
}
