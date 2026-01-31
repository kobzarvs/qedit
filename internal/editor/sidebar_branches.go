package editor

import "github.com/gdamore/tcell/v2"

// SidebarBranchesContent implements SidebarContent for git branches
type SidebarBranchesContent struct {
	branches []string
	current  string
	index    int
}

// NewSidebarBranchesContent creates a new branches content
func NewSidebarBranchesContent(branches []string, current string) *SidebarBranchesContent {
	b := &SidebarBranchesContent{
		branches: branches,
		current:  current,
		index:    0,
	}

	// Set index to current branch
	for i, branch := range branches {
		if branch == current {
			b.index = i
			break
		}
	}

	return b
}

// Mode returns the mode identifier
func (b *SidebarBranchesContent) Mode() SidebarMode {
	return SidebarModeBranches
}

// Title returns header text
func (b *SidebarBranchesContent) Title() string {
	return "Branches"
}

// Items returns the list to display
func (b *SidebarBranchesContent) Items() []SidebarItem {
	result := make([]SidebarItem, len(b.branches))
	for i, branch := range b.branches {
		result[i] = SidebarItem{
			Label:     branch,
			IsCurrent: branch == b.current,
			Available: true,
		}
	}
	return result
}

// Index returns current selection index
func (b *SidebarBranchesContent) Index() int {
	return b.index
}

// SetIndex sets the selection index
func (b *SidebarBranchesContent) SetIndex(i int) {
	if i >= 0 && i < len(b.branches) {
		b.index = i
	}
}

// HandleKey processes mode-specific keys
func (b *SidebarBranchesContent) HandleKey(ev *tcell.EventKey) (bool, SidebarActionData) {
	// No special keys for branches, navigation is handled by container
	return false, SidebarActionData{Action: SidebarActionNone}
}

// OnEnter called when Enter pressed - checkout selected branch
func (b *SidebarBranchesContent) OnEnter() SidebarActionData {
	if b.index < 0 || b.index >= len(b.branches) {
		return SidebarActionData{Action: SidebarActionNone}
	}

	branch := b.branches[b.index]
	if branch == b.current {
		// Already on this branch
		return SidebarActionData{Action: SidebarActionClose}
	}

	return SidebarActionData{
		Action: SidebarActionCheckoutBranch,
		Branch: branch,
	}
}

// Available returns true if we have branches
func (b *SidebarBranchesContent) Available() bool {
	return len(b.branches) > 0
}

// Refresh reloads content (noop - branches are set externally)
func (b *SidebarBranchesContent) Refresh() error {
	return nil
}

// UpdateBranches updates the branch list
func (b *SidebarBranchesContent) UpdateBranches(branches []string, current string) {
	b.branches = branches
	b.current = current

	// Preserve index if possible, otherwise reset
	if b.index >= len(branches) {
		b.index = 0
	}

	// Try to find current branch
	for i, branch := range branches {
		if branch == current {
			b.index = i
			break
		}
	}
}
