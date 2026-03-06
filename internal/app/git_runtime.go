package app

import (
	"path/filepath"
	"strings"

	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/logger"
)

func showSidebarBranches(ed *editor.Editor, gitRuntime editor.GitRuntime, gitPath string) {
	logger.Debug("branch picker requested")
	if gitPath == "" {
		logger.Debug("not a git repository")
		ed.SetStatusMessage("not a git repository")
		return
	}

	branches, current, err := gitRuntime.ListBranches(gitPath)
	if err != nil {
		logger.Error("failed to list branches", "error", err)
		ed.SetStatusMessage(err.Error())
		return
	}

	logger.Debug("showing sidebar branches", "count", len(branches), "current", current)
	ed.ShowSidebarBranches(branches, current)
}

func showSidebarWorktrees(ed *editor.Editor, gitRuntime editor.GitRuntime, fileStore editor.FileStore, gitPath string) {
	logger.Debug("worktree list requested")
	if gitPath == "" {
		logger.Debug("not a git repository")
		ed.SetStatusMessage("not a git repository")
		return
	}

	worktrees, active, err := gitRuntime.ListWorktrees(gitPath)
	if err != nil {
		logger.Error("failed to list worktrees", "error", err)
		ed.SetStatusMessage(err.Error())
		return
	}

	ed.ShowSidebarWorktrees(fileStore, worktrees, active)
}

func checkoutBranch(ed *editor.Editor, gitRuntime editor.GitRuntime, gitPath, branch string) {
	if gitPath == "" {
		ed.SetStatusMessage("not a git repository")
		return
	}
	if err := gitRuntime.Checkout(gitPath, branch); err != nil {
		logger.Error("failed to checkout branch", "branch", branch, "error", err)
		ed.SetStatusMessage(err.Error())
		return
	}
	ed.SetGitBranch(branch)
	ed.SetStatusMessage("checked out " + branch)
}

func pickWorktreeFile(gitRuntime editor.GitRuntime, fileStore editor.FileStore, targetRoot, openPath string) string {
	targetRoot = strings.TrimSpace(targetRoot)
	if targetRoot == "" {
		return ""
	}
	targetRoot = normalizeAppPath(fileStore, targetRoot)

	if openPath != "" {
		currentRoot := gitRuntime.Root(openPath)
		if currentRoot != "" {
			if rel, err := filepath.Rel(currentRoot, openPath); err == nil && !strings.HasPrefix(rel, "..") {
				candidate := filepath.Join(targetRoot, rel)
				if info, err := fileStore.Stat(candidate); err == nil && !info.IsDir {
					return candidate
				}
			}
		}
	}

	for _, name := range []string{"README.md", "README", "go.mod", "package.json"} {
		candidate := filepath.Join(targetRoot, name)
		if info, err := fileStore.Stat(candidate); err == nil && !info.IsDir {
			return candidate
		}
	}

	if entries, err := fileStore.ReadDir(targetRoot); err == nil {
		for _, entry := range entries {
			if entry.IsDir {
				continue
			}
			candidate := filepath.Join(targetRoot, entry.Name)
			if info, err := fileStore.Stat(candidate); err == nil && !info.IsDir {
				return candidate
			}
		}
		for _, entry := range entries {
			if !entry.IsDir || entry.Name == ".git" {
				continue
			}
			dirPath := filepath.Join(targetRoot, entry.Name)
			child, err := fileStore.ReadDir(dirPath)
			if err != nil {
				continue
			}
			for _, c := range child {
				if c.IsDir {
					continue
				}
				candidate := filepath.Join(dirPath, c.Name)
				if info, err := fileStore.Stat(candidate); err == nil && !info.IsDir {
					return candidate
				}
			}
		}
	}

	return ""
}
