package editor

import "testing"

type testGitRuntime struct {
	root      string
	branch    string
	main      string
	worktrees []WorktreeInfo
	active    string
	changes   []GitFileChange
	hunks     []GitChangeHunk
}

func (r testGitRuntime) Root(path string) string       { return r.root }
func (r testGitRuntime) Branch(path string) string     { return r.branch }
func (r testGitRuntime) MainBranch(path string) string { return r.main }
func (r testGitRuntime) ListBranches(root string) ([]string, string, error) {
	return []string{"main", "feature"}, r.branch, nil
}
func (r testGitRuntime) ListWorktrees(root string) ([]WorktreeInfo, string, error) {
	return append([]WorktreeInfo(nil), r.worktrees...), r.active, nil
}
func (r testGitRuntime) Checkout(root, branch string) error { return nil }
func (r testGitRuntime) AddWorktree(root, name string) (string, error) {
	return "/tmp/" + name, nil
}
func (r testGitRuntime) RemoveWorktree(root, path string) error { return nil }
func (r testGitRuntime) Changes(root string) ([]GitFileChange, []GitChangeHunk, error) {
	return append([]GitFileChange(nil), r.changes...), append([]GitChangeHunk(nil), r.hunks...), nil
}

func TestBuiltInGitFeatureUsesRuntimeForWorktrees(t *testing.T) {
	e := New(Options{})
	e.SetGitRoot("/repo")
	e.SetGitRuntime(testGitRuntime{
		worktrees: []WorktreeInfo{
			{Path: "/repo-main", Branch: "main"},
			{Path: "/repo-feature", Branch: "feature"},
		},
	})

	e.handleWorktreeCommand([]string{"switch", "feature"})

	req, ok := e.ConsumeRuntimeRequest()
	if !ok {
		t.Fatalf("expected switch worktree request")
	}
	if req.Kind != RuntimeRequestSwitchWorktree {
		t.Fatalf("request kind = %v, want %v", req.Kind, RuntimeRequestSwitchWorktree)
	}
	if req.Path != "/repo-feature" {
		t.Fatalf("request path = %q, want /repo-feature", req.Path)
	}
}

func TestRegisterGitFeatureOverridesBuiltInChanges(t *testing.T) {
	e := New(Options{})
	e.SetGitRoot("/repo")
	e.SetGitRuntime(testGitRuntime{
		changes: []GitFileChange{{Path: "runtime.go", AbsPath: "/repo/runtime.go"}},
		hunks:   []GitChangeHunk{{Path: "runtime.go", AbsPath: "/repo/runtime.go", StartLine: 1, EndLine: 1}},
	})
	e.RegisterGitFeature(GitFeatureProvider{
		Name:      "custom-git",
		Available: func(*Editor) bool { return true },
		Changes: func(*Editor, string) ([]GitFileChange, []GitChangeHunk, error) {
			return []GitFileChange{{Path: "custom.go", AbsPath: "/repo/custom.go"}}, []GitChangeHunk{{Path: "custom.go", AbsPath: "/repo/custom.go", StartLine: 2, EndLine: 3}}, nil
		},
	})

	if err := e.RefreshGitChanges(); err != nil {
		t.Fatalf("RefreshGitChanges returned error: %v", err)
	}
	if len(e.git.changes) != 1 || e.git.changes[0].Path != "custom.go" {
		t.Fatalf("git changes = %+v, want custom provider result", e.git.changes)
	}
	if len(e.git.changeHunks) != 1 || e.git.changeHunks[0].AbsPath != "/repo/custom.go" {
		t.Fatalf("git hunks = %+v, want custom provider result", e.git.changeHunks)
	}
}
