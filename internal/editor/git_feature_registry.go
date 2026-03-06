package editor

import "errors"

type GitFeatureProvider struct {
	Name           string
	Available      func(*Editor) bool
	Root           func(*Editor, string) string
	Branch         func(*Editor, string) string
	MainBranch     func(*Editor, string) string
	ListBranches   func(*Editor, string) ([]string, string, error)
	ListWorktrees  func(*Editor, string) ([]WorktreeInfo, string, error)
	Checkout       func(*Editor, string, string) error
	AddWorktree    func(*Editor, string, string) (string, error)
	RemoveWorktree func(*Editor, string, string) error
	Changes        func(*Editor, string) ([]GitFileChange, []GitChangeHunk, error)
}

type gitFeatureRegistry struct {
	providers []GitFeatureProvider
}

func newGitFeatureRegistry() gitFeatureRegistry {
	return gitFeatureRegistry{}
}

func (r *gitFeatureRegistry) Register(provider GitFeatureProvider) {
	r.providers = append(r.providers, provider)
}

func (r *gitFeatureRegistry) availableProvider(e *Editor, want func(GitFeatureProvider) bool) (GitFeatureProvider, bool) {
	for i := len(r.providers) - 1; i >= 0; i-- {
		provider := r.providers[i]
		if provider.Available != nil && !provider.Available(e) {
			continue
		}
		if want(provider) {
			return provider, true
		}
	}
	return GitFeatureProvider{}, false
}

func (r *gitFeatureRegistry) Root(e *Editor, path string) string {
	provider, ok := r.availableProvider(e, func(p GitFeatureProvider) bool { return p.Root != nil })
	if !ok {
		return ""
	}
	return provider.Root(e, path)
}

func (r *gitFeatureRegistry) Branch(e *Editor, path string) string {
	provider, ok := r.availableProvider(e, func(p GitFeatureProvider) bool { return p.Branch != nil })
	if !ok {
		return ""
	}
	return provider.Branch(e, path)
}

func (r *gitFeatureRegistry) MainBranch(e *Editor, path string) string {
	provider, ok := r.availableProvider(e, func(p GitFeatureProvider) bool { return p.MainBranch != nil })
	if !ok {
		return ""
	}
	return provider.MainBranch(e, path)
}

func (r *gitFeatureRegistry) ListBranches(e *Editor, root string) ([]string, string, error) {
	provider, ok := r.availableProvider(e, func(p GitFeatureProvider) bool { return p.ListBranches != nil })
	if !ok {
		return nil, "", errors.New("git runtime unavailable")
	}
	return provider.ListBranches(e, root)
}

func (r *gitFeatureRegistry) ListWorktrees(e *Editor, root string) ([]WorktreeInfo, string, error) {
	provider, ok := r.availableProvider(e, func(p GitFeatureProvider) bool { return p.ListWorktrees != nil })
	if !ok {
		return nil, "", errors.New("git runtime unavailable")
	}
	return provider.ListWorktrees(e, root)
}

func (r *gitFeatureRegistry) Checkout(e *Editor, root, branch string) error {
	provider, ok := r.availableProvider(e, func(p GitFeatureProvider) bool { return p.Checkout != nil })
	if !ok {
		return errors.New("git runtime unavailable")
	}
	return provider.Checkout(e, root, branch)
}

func (r *gitFeatureRegistry) AddWorktree(e *Editor, root, name string) (string, error) {
	provider, ok := r.availableProvider(e, func(p GitFeatureProvider) bool { return p.AddWorktree != nil })
	if !ok {
		return "", errors.New("git runtime unavailable")
	}
	return provider.AddWorktree(e, root, name)
}

func (r *gitFeatureRegistry) RemoveWorktree(e *Editor, root, path string) error {
	provider, ok := r.availableProvider(e, func(p GitFeatureProvider) bool { return p.RemoveWorktree != nil })
	if !ok {
		return errors.New("git runtime unavailable")
	}
	return provider.RemoveWorktree(e, root, path)
}

func (r *gitFeatureRegistry) Changes(e *Editor, root string) ([]GitFileChange, []GitChangeHunk, error) {
	provider, ok := r.availableProvider(e, func(p GitFeatureProvider) bool { return p.Changes != nil })
	if !ok {
		return nil, nil, errors.New("git runtime unavailable")
	}
	return provider.Changes(e, root)
}

func (e *Editor) RegisterGitFeature(provider GitFeatureProvider) {
	e.gitFeatures.Register(provider)
}

func (e *Editor) registerBuiltInGitFeatures() {
	e.RegisterGitFeature(GitFeatureProvider{
		Name: "runtime-git",
		Available: func(ed *Editor) bool {
			return ed.runtime.gitRuntime != nil
		},
		Root: func(ed *Editor, path string) string {
			return ed.runtime.gitRuntime.Root(path)
		},
		Branch: func(ed *Editor, path string) string {
			return ed.runtime.gitRuntime.Branch(path)
		},
		MainBranch: func(ed *Editor, path string) string {
			return ed.runtime.gitRuntime.MainBranch(path)
		},
		ListBranches: func(ed *Editor, root string) ([]string, string, error) {
			return ed.runtime.gitRuntime.ListBranches(root)
		},
		ListWorktrees: func(ed *Editor, root string) ([]WorktreeInfo, string, error) {
			return ed.runtime.gitRuntime.ListWorktrees(root)
		},
		Checkout: func(ed *Editor, root, branch string) error {
			return ed.runtime.gitRuntime.Checkout(root, branch)
		},
		AddWorktree: func(ed *Editor, root, name string) (string, error) {
			return ed.runtime.gitRuntime.AddWorktree(root, name)
		},
		RemoveWorktree: func(ed *Editor, root, path string) error {
			return ed.runtime.gitRuntime.RemoveWorktree(root, path)
		},
		Changes: func(ed *Editor, root string) ([]GitFileChange, []GitChangeHunk, error) {
			return ed.runtime.gitRuntime.Changes(root)
		},
	})
}

func (e *Editor) gitRoot(path string) string {
	return e.gitFeatures.Root(e, path)
}

func (e *Editor) gitBranch(path string) string {
	return e.gitFeatures.Branch(e, path)
}

func (e *Editor) gitMainBranch(path string) string {
	return e.gitFeatures.MainBranch(e, path)
}

func (e *Editor) gitListBranches(root string) ([]string, string, error) {
	return e.gitFeatures.ListBranches(e, root)
}

func (e *Editor) gitListWorktrees(root string) ([]WorktreeInfo, string, error) {
	return e.gitFeatures.ListWorktrees(e, root)
}

func (e *Editor) gitCheckout(root, branch string) error {
	return e.gitFeatures.Checkout(e, root, branch)
}

func (e *Editor) gitAddWorktree(root, name string) (string, error) {
	return e.gitFeatures.AddWorktree(e, root, name)
}

func (e *Editor) gitRemoveWorktree(root, path string) error {
	return e.gitFeatures.RemoveWorktree(e, root, path)
}

func (e *Editor) gitChanges(root string) ([]GitFileChange, []GitChangeHunk, error) {
	return e.gitFeatures.Changes(e, root)
}
