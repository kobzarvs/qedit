package editor

import (
	"os"
	"path/filepath"
	"strings"
)

// SidebarWorktreesContent implements SidebarContent for git worktrees.
type SidebarWorktreesContent struct {
	worktrees  []WorktreeInfo
	activePath string
	index      int
}

// NewSidebarWorktreesContent creates a new worktrees content.
func NewSidebarWorktreesContent(worktrees []WorktreeInfo, activePath string) *SidebarWorktreesContent {
	w := &SidebarWorktreesContent{
		worktrees:  append([]WorktreeInfo(nil), worktrees...),
		activePath: normalizeWorktreePath(activePath),
		index:      0,
	}
	w.syncIndex()
	return w
}

func (w *SidebarWorktreesContent) Mode() SidebarMode {
	return SidebarModeWorktrees
}

func (w *SidebarWorktreesContent) Title() string {
	return "Worktree"
}

func (w *SidebarWorktreesContent) Items() []SidebarItem {
	items := make([]SidebarItem, 0, len(w.worktrees))
	for _, wt := range w.worktrees {
		branch := strings.TrimSpace(wt.Branch)
		if branch == "" {
			branch = "detached"
		}
		path := strings.TrimSpace(wt.Path)
		isCurrent := w.isCurrentPath(path)
		label := branch
		if path != "" {
			displayPath := worktreeDisplayPath(path, w.activePath)
			if displayPath != "" {
				label = branch + "  " + displayPath
			}
		}
		items = append(items, SidebarItem{
			Label:     label,
			Available: true,
			IsCurrent: isCurrent,
		})
	}
	if len(items) == 0 {
		items = []SidebarItem{{Label: "No worktrees", Available: false}}
	}
	return items
}

func (w *SidebarWorktreesContent) Index() int {
	return w.index
}

func (w *SidebarWorktreesContent) SetIndex(i int) {
	if i >= 0 && i < len(w.worktrees) {
		w.index = i
	}
}

func (w *SidebarWorktreesContent) HandleKey(ev EventKey) (bool, SidebarActionData) {
	key := ev.Key()
	r := ev.Rune()
	switch {
	case r == 'r':
		return true, SidebarActionData{Action: SidebarActionRefresh, Mode: SidebarModeWorktrees}
	case key == KeyRight || r == 'l':
		return true, w.OnEnter()
	}
	return false, SidebarActionData{Action: SidebarActionNone}
}

func (w *SidebarWorktreesContent) OnEnter() SidebarActionData {
	if w.index < 0 || w.index >= len(w.worktrees) {
		return SidebarActionData{Action: SidebarActionNone}
	}
	wt := w.worktrees[w.index]
	if wt.Path == "" {
		return SidebarActionData{Action: SidebarActionNone}
	}
	if w.isCurrentPath(wt.Path) {
		return SidebarActionData{Action: SidebarActionClose}
	}
	return SidebarActionData{
		Action:   SidebarActionSwitchWorktree,
		Worktree: wt.Path,
	}
}

func (w *SidebarWorktreesContent) Available() bool {
	return len(w.worktrees) > 0
}

func (w *SidebarWorktreesContent) Refresh() error {
	return nil
}

func (w *SidebarWorktreesContent) SetCurrentPath(path string) {
	w.activePath = normalizeWorktreePath(path)
	w.syncIndex()
}

func (w *SidebarWorktreesContent) SetCurrentBranch(branch string) {
	branch = strings.TrimSpace(branch)
	if branch == "" || w.activePath == "" {
		return
	}
	for i, wt := range w.worktrees {
		if w.isCurrentPath(wt.Path) {
			w.worktrees[i].Branch = branch
			return
		}
	}
}

func (w *SidebarWorktreesContent) UpdateWorktrees(worktrees []WorktreeInfo, activePath string) {
	w.worktrees = append(w.worktrees[:0], worktrees...)
	w.activePath = normalizeWorktreePath(activePath)
	w.syncIndex()
}

func (w *SidebarWorktreesContent) syncIndex() {
	if len(w.worktrees) == 0 {
		w.index = 0
		return
	}
	for i, wt := range w.worktrees {
		if w.isCurrentPath(wt.Path) {
			w.index = i
			return
		}
	}
	if w.index >= len(w.worktrees) {
		w.index = len(w.worktrees) - 1
		if w.index < 0 {
			w.index = 0
		}
	}
}

func (w *SidebarWorktreesContent) isCurrentPath(path string) bool {
	if w.activePath == "" || strings.TrimSpace(path) == "" {
		return false
	}
	return normalizeWorktreePath(path) == w.activePath
}

func normalizeWorktreePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

func worktreeDisplayPath(path, activePath string) string {
	path = normalizeWorktreePath(path)
	if path == "" {
		return ""
	}
	if activePath != "" {
		base := filepath.Dir(activePath)
		if base != "" {
			if rel, err := filepath.Rel(base, path); err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
				return rel
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		home = filepath.Clean(home)
		if path == home {
			return "~"
		}
		prefix := home + string(filepath.Separator)
		if strings.HasPrefix(path, prefix) {
			return "~" + string(filepath.Separator) + strings.TrimPrefix(path, prefix)
		}
	}
	return path
}
