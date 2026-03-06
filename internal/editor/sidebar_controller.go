package editor

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kobzarvs/qedit/internal/logger"
)

func (e *Editor) handleSidebarKey(ev EventKey) bool {
	if e.sidebar == nil {
		return false
	}

	if ev.Key() == KeyRune && ev.Rune() == ':' {
		e.sidebar.Focused = false
		e.mode = ModeCommand
		e.cmd = e.cmd[:0]
		e.cmdCursor = 0
		e.cmdHistoryIndex = -1
		return false
	}

	viewHeight := e.viewHeight - 1 // subtract header
	var fileTreeContent *SidebarFileTreeContent
	if content, ok := e.sidebar.Content.(*SidebarFileTreeContent); ok {
		fileTreeContent = content
	}
	action := e.sidebar.HandleKey(ev, viewHeight)
	if fileTreeContent != nil {
		e.updateFileTreePreview(fileTreeContent, action)
		e.updateFileTreeStatus(fileTreeContent, action)
		e.fileTreeShowHidden = fileTreeContent.ShowHidden()
		e.fileTreeShowIgnored = fileTreeContent.ShowIgnored()
		switch ev.Key() {
		case KeyF2:
			e.Notify(fileTreeContent.filterModeLabel())
		case KeyF3:
			if fileTreeContent.PreviewEnabled() {
				e.Notify("Files: preview on")
			} else {
				e.Notify("Files: preview off")
			}
		}
	}
	logger.Debug("handleSidebarKey", "action", action.Action, "branch", action.Branch, "mode", action.Mode)

	switch action.Action {
	case SidebarActionClose:
		logger.Debug("sidebar action: close")
		e.closeSidebar()
		return false

	case SidebarActionBackToMenu:
		logger.Debug("sidebar action: back to menu")
		if e.sidebar.MenuContent != nil {
			e.sidebar.MenuContent.SetAvailability(e.isGitRepo())
			e.sidebar.SetContent(e.sidebar.MenuContent)
		}
		return false

	case SidebarActionFocusEditor:
		logger.Debug("sidebar action: focus editor")
		e.sidebar.Focused = false
		return false

	case SidebarActionSwitchMode:
		logger.Debug("sidebar action: switch mode", "mode", action.Mode)
		e.switchSidebarMode(action.Mode)
		return false

	case SidebarActionCheckoutBranch:
		logger.Debug("sidebar action: checkout branch", "branch", action.Branch)
		if action.Branch != "" {
			e.branchPickerRequested = false
			e.branchPickerSelection = action.Branch
			if e.sidebar.CloseOnSelect {
				e.closeSidebar()
			} else {
				e.sidebar.Focused = false
			}
		}
		return false

	case SidebarActionOpenFile:
		logger.Debug("sidebar action: open file", "path", action.Path)
		if action.Path != "" {
			if fileTreeContent != nil && isBinaryFile(action.Path) {
				e.Notify("Files: binary preview only")
				return false
			}
			e.sidebarOpenFilePath = action.Path
			e.clearFileTreePreview()
			if e.sidebar.CloseOnSelect {
				e.closeSidebar()
			} else {
				e.sidebar.Focused = false
			}
		}
		return false

	case SidebarActionSwitchWorktree:
		logger.Debug("sidebar action: switch worktree", "path", action.Worktree)
		if action.Worktree != "" {
			e.worktreeSwitchSelection = action.Worktree
			if e.sidebar.CloseOnSelect {
				e.closeSidebar()
			} else {
				e.sidebar.Focused = false
			}
		}
		return false

	case SidebarActionSwitchBuffer:
		logger.Debug("sidebar action: switch buffer", "index", action.BufferIndex)
		e.switchToBuffer(action.BufferIndex)
		if e.sidebar.CloseOnSelect {
			e.closeSidebar()
		} else {
			e.sidebar.Focused = false
		}
		return false

	case SidebarActionRefresh:
		if action.Mode == SidebarModeWorktrees {
			e.refreshSidebarWorktrees()
		}
		return false
	}

	return false
}

func (e *Editor) switchSidebarMode(mode SidebarMode) {
	if mode != SidebarModeFileTree {
		e.clearFileTreePreview()
	}
	switch mode {
	case SidebarModeMenu:
		if e.sidebar.MenuContent != nil {
			e.sidebar.SetContent(e.sidebar.MenuContent)
		}

	case SidebarModeBranches:
		e.openSidebarBranches()

	case SidebarModeFileTree:
		e.openSidebarFileTree("")

	case SidebarModeRecentHistory:
		e.setStatus("Recent History: not implemented yet")

	case SidebarModeLocalChanges:
		e.openSidebarGitChanges()

	case SidebarModeWorktrees:
		e.openSidebarWorktrees()

	case SidebarModeBuffers:
		e.openSidebarBuffers()
	}
}

func (e *Editor) openSidebar() {
	logger.Debug("openSidebar called")
	if e.sidebar == nil {
		logger.Warn("openSidebar: sidebar is nil")
		return
	}

	if e.refsPickerActive {
		logger.Debug("openSidebar: closing refs picker")
		e.closeRefsPicker(false)
	}

	if e.sidebar.MenuContent == nil {
		logger.Debug("openSidebar: creating menu content", "gitRepo", e.isGitRepo())
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo())
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo())
	}

	e.sidebar.Open(e.sidebar.MenuContent)
	logger.Debug("openSidebar: sidebar opened")
}

func (e *Editor) openSidebarBranches() {
	logger.Debug("openSidebarBranches called")
	if e.sidebar == nil {
		logger.Warn("openSidebarBranches: sidebar is nil")
		return
	}
	if !e.isGitRepo() {
		e.setStatus("not a git repository")
		if e.sidebar.MenuContent != nil {
			e.sidebar.MenuContent.SetAvailability(false)
		}
		return
	}

	if e.refsPickerActive {
		logger.Debug("openSidebarBranches: closing refs picker")
		e.closeRefsPicker(false)
	}

	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo())
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo())
	}

	e.sidebar.Open(NewSidebarLoadingContent("Branches", "Loading..."))
	e.setStatus("loading branches...")
	e.branchPickerRequested = true
	logger.Debug("openSidebarBranches: branch request set")
}

func (e *Editor) openSidebarWorktrees() {
	logger.Debug("openSidebarWorktrees called")
	if e.sidebar == nil {
		logger.Warn("openSidebarWorktrees: sidebar is nil")
		return
	}
	if !e.isGitRepo() {
		e.setStatus("not a git repository")
		if e.sidebar.MenuContent != nil {
			e.sidebar.MenuContent.SetAvailability(false)
		}
		return
	}

	if e.refsPickerActive {
		logger.Debug("openSidebarWorktrees: closing refs picker")
		e.closeRefsPicker(false)
	}

	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo())
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo())
	}

	e.sidebar.Open(NewSidebarLoadingContent("Worktree", "Loading..."))
	e.setStatus("loading worktrees...")
	e.worktreeListRequested = true
	logger.Debug("openSidebarWorktrees: list request set")
}

func (e *Editor) refreshSidebarWorktrees() {
	if e.sidebar == nil {
		return
	}
	if !e.isGitRepo() {
		e.setStatus("not a git repository")
		return
	}
	if e.sidebar.Visible && e.sidebar.Content != nil && e.sidebar.Content.Mode() == SidebarModeWorktrees {
		e.worktreeListRequested = true
		e.setStatus("loading worktrees...")
		return
	}
	e.openSidebarWorktrees()
}

func (e *Editor) requestWorktreeRefreshIfActive() {
	if e.sidebar == nil || !e.sidebar.Visible || e.sidebar.Content == nil {
		return
	}
	if e.sidebar.Content.Mode() == SidebarModeWorktrees {
		e.worktreeListRequested = true
	}
}

func (e *Editor) openSidebarFileTree(path string) {
	logger.Debug("openSidebarFileTree called", "path", path)
	if e.sidebar == nil {
		logger.Warn("openSidebarFileTree: sidebar is nil")
		return
	}

	if e.refsPickerActive {
		logger.Debug("openSidebarFileTree: closing refs picker")
		e.closeRefsPicker(false)
	}

	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo())
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo())
	}

	startDir := strings.TrimSpace(path)
	if startDir == "" {
		if e.filename != "" {
			startDir = filepath.Dir(e.filename)
		} else if cwd, err := os.Getwd(); err == nil {
			startDir = cwd
		}
	}
	if startDir == "" {
		startDir = "."
	}
	if info, err := os.Stat(startDir); err == nil && !info.IsDir() {
		startDir = filepath.Dir(startDir)
	}

	content := NewSidebarFileTreeContent(startDir, e.fileTreeShowHidden, e.fileTreeShowIgnored)
	e.sidebar.Open(content)
}

func (e *Editor) openSidebarGitChanges() {
	logger.Debug("openSidebarGitChanges called")
	if e.sidebar == nil {
		logger.Warn("openSidebarGitChanges: sidebar is nil")
		return
	}
	if !e.isGitRepo() {
		e.setStatus("not a git repository")
		if e.sidebar.MenuContent != nil {
			e.sidebar.MenuContent.SetAvailability(false)
		}
		return
	}

	if e.refsPickerActive {
		logger.Debug("openSidebarGitChanges: closing refs picker")
		e.closeRefsPicker(false)
	}

	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(e.isGitRepo())
	} else {
		e.sidebar.MenuContent.SetAvailability(e.isGitRepo())
	}

	if err := e.refreshGitChangesIfStale(2 * time.Second); err != nil {
		logger.Error("openSidebarGitChanges: refresh failed", "error", err)
		e.setStatus(err.Error())
	}

	content := NewSidebarGitChangesContent(e)
	e.sidebar.Open(content)
}

func (e *Editor) closeSidebar() {
	logger.Debug("closeSidebar called")
	if e.sidebar == nil {
		return
	}
	e.sidebar.Close()
	e.clearFileTreePreview()
}

func (e *Editor) toggleSidebar() {
	logger.Debug("toggleSidebar called", "visible", e.sidebar != nil && e.sidebar.Visible, "focused", e.sidebar != nil && e.sidebar.Focused)
	if e.sidebar == nil {
		logger.Warn("toggleSidebar: sidebar is nil")
		return
	}
	if e.sidebar.Visible {
		if e.sidebar.Focused {
			e.closeSidebar()
		} else {
			e.sidebar.Focused = true
		}
	} else {
		if e.sidebar.Content != nil {
			e.sidebar.Visible = true
			e.sidebar.Focused = true
		} else {
			e.openSidebar()
		}
	}
}

func (e *Editor) toggleSidebarFocus() {
	if e.sidebar == nil {
		return
	}
	if !e.sidebar.Visible {
		e.openSidebar()
		return
	}
	e.sidebar.Focused = !e.sidebar.Focused
	if !e.sidebar.Focused {
		e.clearFileTreePreview()
	}
}

func (e *Editor) ShowSidebarBranches(branches []string, current string) {
	logger.Debug("ShowSidebarBranches called", "count", len(branches), "current", current)
	if e.sidebar == nil {
		logger.Warn("ShowSidebarBranches: sidebar is nil")
		return
	}

	content := NewSidebarBranchesContent(branches, current)
	e.sidebar.SetContent(content)
	e.sidebar.Visible = true
	e.sidebar.Focused = true
}

func (e *Editor) ShowSidebarWorktrees(worktrees []WorktreeInfo, activePath string) {
	logger.Debug("ShowSidebarWorktrees called", "count", len(worktrees), "active", activePath)
	if e.sidebar == nil {
		logger.Warn("ShowSidebarWorktrees: sidebar is nil")
		return
	}
	if content, ok := e.sidebar.Content.(*SidebarWorktreesContent); ok {
		content.UpdateWorktrees(worktrees, activePath)
		e.sidebar.SetContent(content)
	} else {
		content := NewSidebarWorktreesContent(worktrees, activePath)
		e.sidebar.SetContent(content)
	}
	e.sidebar.Visible = true
	e.sidebar.Focused = true
}

func (e *Editor) isGitRepo() bool {
	return e.git.branch != ""
}

func (e *Editor) IsSidebarBranchRequest() bool {
	if e.sidebar == nil {
		return false
	}
	return e.branchPickerRequested
}

func (e *Editor) ConsumeWorktreeListRequest() bool {
	if !e.worktreeListRequested {
		return false
	}
	e.worktreeListRequested = false
	return true
}

func (e *Editor) ConsumeSidebarBranchSelection() string {
	if e.branchPickerSelection == "" {
		return ""
	}
	selection := e.branchPickerSelection
	e.branchPickerSelection = ""
	return selection
}

func (e *Editor) ConsumeSidebarWorktreeSelection() string {
	if e.worktreeSwitchSelection == "" {
		return ""
	}
	selection := e.worktreeSwitchSelection
	e.worktreeSwitchSelection = ""
	return selection
}
