package app

import (
	"errors"
	"testing"

	"github.com/kobzarvs/qedit/internal/editor"
)

var errTestAppNotExist = errors.New("missing path")

type testAppFileStore struct {
	absPaths   map[string]string
	dirEntries map[string][]editor.DirEntry
	stats      map[string]editor.FileMetadata
}

type testAppGitRuntime struct {
	root string
}

func (r testAppGitRuntime) Root(path string) string { return r.root }
func (r testAppGitRuntime) Branch(path string) string {
	return ""
}
func (r testAppGitRuntime) MainBranch(path string) string {
	return ""
}
func (r testAppGitRuntime) ListBranches(root string) ([]string, string, error) {
	return nil, "", nil
}
func (r testAppGitRuntime) ListWorktrees(root string) ([]editor.WorktreeInfo, string, error) {
	return nil, "", nil
}
func (r testAppGitRuntime) Checkout(root, branch string) error {
	return nil
}
func (r testAppGitRuntime) AddWorktree(root, name string) (string, error) {
	return "", nil
}
func (r testAppGitRuntime) RemoveWorktree(root, path string) error {
	return nil
}
func (r testAppGitRuntime) Changes(root string) ([]editor.GitFileChange, []editor.GitChangeHunk, error) {
	return nil, nil, nil
}

func (s testAppFileStore) Abs(path string) (string, error) {
	if abs, ok := s.absPaths[path]; ok {
		return abs, nil
	}
	return path, nil
}

func (s testAppFileStore) HomeDir() (string, error) {
	return "", nil
}

func (s testAppFileStore) Read(path string) ([]byte, error) {
	return nil, errTestAppNotExist
}

func (s testAppFileStore) ReadDir(path string) ([]editor.DirEntry, error) {
	if entries, ok := s.dirEntries[path]; ok {
		return append([]editor.DirEntry(nil), entries...), nil
	}
	return nil, errTestAppNotExist
}

func (s testAppFileStore) Write(path string, data []byte) error {
	return nil
}

func (s testAppFileStore) Stat(path string) (editor.FileMetadata, error) {
	if info, ok := s.stats[path]; ok {
		return info, nil
	}
	return editor.FileMetadata{}, errTestAppNotExist
}

func (s testAppFileStore) IsNotExist(err error) bool {
	return errors.Is(err, errTestAppNotExist)
}

func TestPickWorktreeFileUsesFileStore(t *testing.T) {
	fileStore := testAppFileStore{
		absPaths: map[string]string{
			"feature": "/repo-feature",
		},
		stats: map[string]editor.FileMetadata{
			"/repo-feature/go.mod": {Size: 10},
		},
	}

	got := pickWorktreeFile(testAppGitRuntime{}, fileStore, "feature", "")

	if got != "/repo-feature/go.mod" {
		t.Fatalf("pickWorktreeFile = %q, want %q", got, "/repo-feature/go.mod")
	}
}
