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
	case editor.RuntimeRequestSaveFile:
		c.handleSaveFileRequest(req)
	case editor.RuntimeRequestReloadFile:
		c.handleReloadFileRequest(req)
	case editor.RuntimeRequestFormatBuffer:
		c.handleFormatBufferRequest(req)
	case editor.RuntimeRequestWriteClipboard:
		c.handleWriteClipboardRequest(req)
	case editor.RuntimeRequestReadClipboard:
		c.handleReadClipboardRequest(req)
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

func (c *editorRuntimeController) handleSaveFileRequest(req editor.RuntimeRequest) {
	if req.Path == "" {
		return
	}
	if err := c.fileStore.Write(req.Path, []byte(req.Content)); err != nil {
		c.ed.SetStatusMessage(err.Error())
		return
	}

	prevPath := normalizeAppPath(c.fileStore, c.state.openPath)
	savedPath := normalizeAppPath(c.fileStore, req.Path)
	c.ed.ApplySavedFile(savedPath)

	if savedPath != prevPath {
		c.activateCurrentEditorFile(savedPath, true)
	} else {
		c.state.openPath = savedPath
		c.fileMonitor.Watch(savedPath)
	}

	c.ed.SetStatusMessage("written")
	if req.QuitAfter {
		c.state.quitRequested = true
	}
}

func (c *editorRuntimeController) handleReloadFileRequest(req editor.RuntimeRequest) {
	if req.Path == "" {
		return
	}
	data, err := c.fileStore.Read(req.Path)
	if err != nil {
		c.ed.SetStatusMessage(err.Error())
		return
	}
	c.ed.ApplyReloadedContent(data)
	c.activateCurrentEditorFile(req.Path, false)
	c.ed.SetStatusMessage("reloaded")
}

func (c *editorRuntimeController) handleFormatBufferRequest(req editor.RuntimeRequest) {
	if c.formatter == nil {
		c.ed.SetStatusMessage("formatter unavailable")
		return
	}
	formatted, err := c.formatter.FormatGo(req.Content)
	if err != nil {
		c.ed.SetStatusMessage(err.Error())
		return
	}
	c.ed.ApplyFormattedContent(formatted)
	c.ed.SetStatusMessage("formatted")
}

func (c *editorRuntimeController) handleWriteClipboardRequest(req editor.RuntimeRequest) {
	if req.Content == "" {
		return
	}
	if c.clipboard == nil {
		if req.Notify {
			c.ed.SetStatusMessage("yanked (clipboard unavailable)")
		}
		return
	}
	if err := c.clipboard.Write(req.Content); err != nil {
		if req.Notify {
			c.ed.SetStatusMessage("yanked (clipboard unavailable)")
		}
		return
	}
	if req.Notify {
		c.ed.SetStatusMessage("yanked to clipboard")
	}
}

func (c *editorRuntimeController) handleReadClipboardRequest(req editor.RuntimeRequest) {
	if c.clipboard == nil {
		c.ed.SetStatusMessage("clipboard unavailable")
		return
	}
	text, err := c.clipboard.Read()
	if err != nil {
		c.ed.SetStatusMessage("clipboard unavailable")
		return
	}
	if text == "" {
		c.ed.SetStatusMessage("clipboard empty")
		return
	}
	c.ed.ApplyClipboardText(text, req.Before)
	c.ed.SetStatusMessage("pasted from clipboard")
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
