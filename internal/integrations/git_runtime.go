package integrations

import (
	"path/filepath"

	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/gitinfo"
)

type GitInfoRuntime struct{}

func (GitInfoRuntime) Root(path string) string {
	return gitinfo.Root(path)
}

func (GitInfoRuntime) Branch(path string) string {
	return gitinfo.Branch(path)
}

func (GitInfoRuntime) MainBranch(path string) string {
	return gitinfo.MainBranch(path)
}

func (GitInfoRuntime) ListBranches(root string) ([]string, string, error) {
	return gitinfo.ListBranches(root)
}

func (GitInfoRuntime) ListWorktrees(root string) ([]editor.WorktreeInfo, string, error) {
	worktrees, active, err := gitinfo.ListWorktrees(root)
	if err != nil {
		return nil, "", err
	}
	items := make([]editor.WorktreeInfo, 0, len(worktrees))
	for _, wt := range worktrees {
		items = append(items, editor.WorktreeInfo{
			Path:   wt.Path,
			Branch: wt.Branch,
		})
	}
	return items, active, nil
}

func (GitInfoRuntime) Checkout(root, branch string) error {
	return gitinfo.Checkout(root, branch)
}

func (GitInfoRuntime) AddWorktree(root, name string) (string, error) {
	return gitinfo.AddWorktree(root, name)
}

func (GitInfoRuntime) RemoveWorktree(root, path string) error {
	return gitinfo.RemoveWorktree(root, path)
}

func (GitInfoRuntime) Changes(root string) ([]editor.GitFileChange, []editor.GitChangeHunk, error) {
	changes, hunks, err := gitinfo.Changes(root)
	if err != nil {
		return nil, nil, err
	}
	editorChanges := make([]editor.GitFileChange, 0, len(changes))
	for _, ch := range changes {
		editorChanges = append(editorChanges, editor.GitFileChange{
			Path:       ch.Path,
			AbsPath:    filepath.Join(root, filepath.FromSlash(ch.Path)),
			Status:     ch.Status,
			Insertions: ch.Insertions,
			Deletions:  ch.Deletions,
			Binary:     ch.Binary,
			Staged:     ch.Staged,
			Unstaged:   ch.Unstaged,
		})
	}
	editorHunks := make([]editor.GitChangeHunk, 0, len(hunks))
	for _, h := range hunks {
		editorHunks = append(editorHunks, editor.GitChangeHunk{
			Path:      h.Path,
			AbsPath:   filepath.Join(root, filepath.FromSlash(h.Path)),
			StartLine: h.StartLine,
			EndLine:   h.EndLine,
		})
	}
	return editorChanges, editorHunks, nil
}
