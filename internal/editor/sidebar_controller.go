package editor

import (
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
		e.commandLine.text = e.commandLine.text[:0]
		e.commandLine.cursor = 0
		e.commandLine.historyIndex = -1
		return false
	}

	viewHeight := e.viewport.height - 1 // subtract header
	var fileTreeContent *SidebarFileTreeContent
	if content, ok := e.sidebar.Content.(*SidebarFileTreeContent); ok {
		fileTreeContent = content
	}
	action := e.sidebar.HandleKey(ev, viewHeight)
	if fileTreeContent != nil {
		e.updateFileTreePreview(fileTreeContent, action)
		e.updateFileTreeStatus(fileTreeContent, action)
		e.fileTree.showHidden = fileTreeContent.ShowHidden()
		e.fileTree.showIgnored = fileTreeContent.ShowIgnored()
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
		e.showSidebarMenu()
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
			e.enqueueRuntimeRequest(RuntimeRequest{
				Kind:  RuntimeRequestSelectBranch,
				Value: action.Branch,
			})
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
			if fileTreeContent != nil && isBinaryPath(fileTreeContent.fs, action.Path) {
				e.Notify("Files: binary preview only")
				return false
			}
			e.enqueueRuntimeRequest(RuntimeRequest{
				Kind: RuntimeRequestOpenFile,
				Path: action.Path,
				Line: -1,
				Col:  -1,
			})
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
			e.enqueueRuntimeRequest(RuntimeRequest{
				Kind: RuntimeRequestSwitchWorktree,
				Path: action.Worktree,
			})
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
		e.refreshSidebarMode(action.Mode)
		return false
	}

	return false
}

func (e *Editor) switchSidebarMode(mode SidebarMode) {
	if mode != SidebarModeFileTree {
		e.clearFileTreePreview()
	}
	e.openSidebarMode(mode)
}

func (e *Editor) openSidebar() {
	logger.Debug("openSidebar called")
	if e.sidebar == nil {
		logger.Warn("openSidebar: sidebar is nil")
		return
	}

	if e.refsPicker.active {
		logger.Debug("openSidebar: closing refs picker")
		e.closeRefsPicker(false)
	}

	e.showSidebarMenu()
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
		return
	}

	if e.refsPicker.active {
		logger.Debug("openSidebarBranches: closing refs picker")
		e.closeRefsPicker(false)
	}

	e.refreshSidebarMenu()

	e.sidebar.Open(NewSidebarLoadingContent("Branches", "Loading..."))
	e.setStatus("loading branches...")
	e.enqueueRuntimeRequest(RuntimeRequest{Kind: RuntimeRequestShowBranchPicker})
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
		return
	}

	if e.refsPicker.active {
		logger.Debug("openSidebarWorktrees: closing refs picker")
		e.closeRefsPicker(false)
	}

	e.refreshSidebarMenu()

	e.sidebar.Open(NewSidebarLoadingContent("Worktree", "Loading..."))
	e.setStatus("loading worktrees...")
	e.enqueueRuntimeRequest(RuntimeRequest{Kind: RuntimeRequestShowWorktrees})
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
		e.enqueueRuntimeRequest(RuntimeRequest{Kind: RuntimeRequestShowWorktrees})
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
		e.enqueueRuntimeRequest(RuntimeRequest{Kind: RuntimeRequestShowWorktrees})
	}
}

func (e *Editor) openSidebarFileTree(path string) {
	logger.Debug("openSidebarFileTree called", "path", path)
	if e.sidebar == nil {
		logger.Warn("openSidebarFileTree: sidebar is nil")
		return
	}

	if e.refsPicker.active {
		logger.Debug("openSidebarFileTree: closing refs picker")
		e.closeRefsPicker(false)
	}

	e.refreshSidebarMenu()
	e.enqueueRuntimeRequest(RuntimeRequest{
		Kind: RuntimeRequestShowFileTree,
		Path: path,
	})
}

func (e *Editor) ShowSidebarFileTree(fs FileStore, path string) {
	if e.sidebar == nil {
		logger.Warn("ShowSidebarFileTree: sidebar is nil")
		return
	}
	startDir := strings.TrimSpace(path)
	if startDir == "" {
		if e.document.filename != "" {
			startDir = filepath.Dir(e.document.filename)
		} else if cwd := e.normalizedPath("."); cwd != "" {
			startDir = cwd
		}
	}
	if startDir == "" {
		startDir = "."
	}
	if fs != nil {
		if info, err := fs.Stat(startDir); err == nil && !info.IsDir {
			startDir = filepath.Dir(startDir)
		}
	}

	content := NewSidebarFileTreeContent(fs, startDir, e.fileTree.showHidden, e.fileTree.showIgnored)
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
		return
	}

	if e.refsPicker.active {
		logger.Debug("openSidebarGitChanges: closing refs picker")
		e.closeRefsPicker(false)
	}

	e.refreshSidebarMenu()

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

func (e *Editor) ShowSidebarWorktrees(fs FileStore, worktrees []WorktreeInfo, activePath string) {
	logger.Debug("ShowSidebarWorktrees called", "count", len(worktrees), "active", activePath)
	if e.sidebar == nil {
		logger.Warn("ShowSidebarWorktrees: sidebar is nil")
		return
	}
	if content, ok := e.sidebar.Content.(*SidebarWorktreesContent); ok {
		content.fs = fs
		content.UpdateWorktrees(worktrees, activePath)
		e.sidebar.SetContent(content)
	} else {
		content := NewSidebarWorktreesContent(fs, worktrees, activePath)
		e.sidebar.SetContent(content)
	}
	e.sidebar.Visible = true
	e.sidebar.Focused = true
}

func (e *Editor) isGitRepo() bool {
	return e.git.branch != ""
}
