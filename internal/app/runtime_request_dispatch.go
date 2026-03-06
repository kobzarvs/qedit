package app

import (
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/logger"
)

func (c *editorRuntimeController) handleEditorRequests() {
	for {
		req, ok := c.ed.ConsumeRuntimeRequest()
		if !ok {
			return
		}
		c.dispatchRuntimeRequest(req)
	}
}

func (c *editorRuntimeController) dispatchRuntimeRequest(req editor.RuntimeRequest) {
	switch req.Kind {
	case editor.RuntimeRequestShowBranchPicker:
		c.handleShowBranchPickerRequest()
	case editor.RuntimeRequestShowWorktrees:
		c.handleShowWorktreesRequest()
	case editor.RuntimeRequestSelectBranch:
		c.handleSelectBranchRequest(req)
	case editor.RuntimeRequestSwitchWorktree:
		c.handleSwitchWorktreeRequest(req)
	case editor.RuntimeRequestOpenFile:
		c.handleOpenFileRequest(req)
	case editor.RuntimeRequestBufferSwitched:
		c.handleBufferSwitchedRequest()
	case editor.RuntimeRequestPersistAutoReload:
		c.handlePersistAutoReloadRequest(req)
	case editor.RuntimeRequestPersistSidebarWidth:
		c.handlePersistSidebarWidthRequest(req)
	}
}

func (c *editorRuntimeController) handleShowBranchPickerRequest() {
	showSidebarBranches(c.ed, c.state.gitPath)
}

func (c *editorRuntimeController) handleShowWorktreesRequest() {
	showSidebarWorktrees(c.ed, c.state.gitPath)
}

func (c *editorRuntimeController) handleSelectBranchRequest(req editor.RuntimeRequest) {
	if req.Value == "" {
		return
	}
	logger.Debug("branch selected", "branch", req.Value)
	checkoutBranch(c.ed, c.state.gitPath, req.Value)
}

func (c *editorRuntimeController) handleSwitchWorktreeRequest(req editor.RuntimeRequest) {
	if req.Path == "" {
		return
	}
	logger.Debug("worktree selected", "path", req.Path)
	if c.state.gitPath == "" {
		c.ed.SetStatusMessage("not a git repository")
		return
	}
	c.switchToWorktree(req.Path)
}

func (c *editorRuntimeController) handleOpenFileRequest(req editor.RuntimeRequest) {
	if req.Path == "" {
		return
	}
	if err := c.openFile(req.Path); err != nil {
		c.ed.SetStatusMessage(err.Error())
		return
	}
	c.ed.ApplyPendingGitDiffJump()
	if req.Line >= 0 {
		c.ed.JumpToLocation(req.Line, req.Col)
	}
}

func (c *editorRuntimeController) handleBufferSwitchedRequest() {
	path := c.ed.Filename()
	if path == c.state.openPath {
		return
	}
	c.activateExistingBuffer(path)
}

func (c *editorRuntimeController) handlePersistAutoReloadRequest(req editor.RuntimeRequest) {
	if err := persistEditorAutoReload(c.cfg, req.Bool); err != nil {
		c.ed.SetAutoReloadOnChanges(req.PrevBool)
		c.ed.SetStatusMessage("config write failed: " + err.Error())
		return
	}
	c.ed.SetAutoReloadOnChanges(req.Bool)
}

func (c *editorRuntimeController) handlePersistSidebarWidthRequest(req editor.RuntimeRequest) {
	if req.Value == "" {
		return
	}
	if err := persistEditorSidebarWidth(c.cfg, req.Value); err != nil {
		c.ed.SetSidebarWidthConfig(req.PrevValue)
		c.ed.SetStatusMessage("config write failed: " + err.Error())
		return
	}
	c.ed.SetSidebarWidthConfig(req.Value)
}
