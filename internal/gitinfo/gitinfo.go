package gitinfo

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Branch(path string) string {
	gitDir, err := findGitDir(path)
	if err != nil || gitDir == "" {
		return ""
	}
	branch, err := readHead(gitDir)
	if err != nil {
		return ""
	}
	return branch
}

func Root(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		path = filepath.Dir(path)
	}
	pathAbs := path
	if abs, err := filepath.Abs(path); err == nil {
		pathAbs = abs
	}
	// Prefer git's own resolution so worktrees return the correct top-level.
	if out, err := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel").CombinedOutput(); err == nil {
		root := strings.TrimSpace(string(out))
		if root != "" {
			if normalized := normalizeRootPath(root, pathAbs); normalized != "" {
				return normalized
			}
			return root
		}
	}
	gitDir, err := findGitDir(path)
	if err != nil || gitDir == "" {
		return ""
	}
	return filepath.Dir(gitDir)
}

func ListBranches(path string) ([]string, string, error) {
	root := Root(path)
	if root == "" {
		return nil, "", errors.New("not a git repository")
	}
	out, err := exec.Command("git", "-C", root, "branch", "--format=%(refname:short)").CombinedOutput()
	if err != nil {
		return nil, "", errors.New(strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		branches = append(branches, line)
	}
	return branches, Branch(root), nil
}

func Checkout(path, branch string) error {
	root := Root(path)
	if root == "" {
		return errors.New("not a git repository")
	}
	out, err := exec.Command("git", "-C", root, "checkout", branch).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return errors.New(msg)
	}
	return nil
}

type Worktree struct {
	Path   string
	Branch string
}

func ListWorktrees(path string) ([]Worktree, string, error) {
	root := Root(path)
	if root == "" {
		return nil, "", errors.New("not a git repository")
	}
	out, err := exec.Command("git", "-C", root, "worktree", "list", "--porcelain").CombinedOutput()
	if err != nil {
		return nil, "", errors.New(strings.TrimSpace(string(out)))
	}
	lines := strings.Split(string(out), "\n")
	worktrees := make([]Worktree, 0, 8)
	currentPath := Root(path)
	if currentPath == "" {
		if abs, err := filepath.Abs(path); err == nil {
			currentPath = filepath.Clean(abs)
		}
	}
	activePath := ""
	var wt Worktree
	have := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if have {
				worktrees = append(worktrees, wt)
				wt = Worktree{}
				have = false
			}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			wt = Worktree{Path: strings.TrimSpace(strings.TrimPrefix(line, "worktree "))}
			have = true
			continue
		}
		if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			wt.Branch = filepath.Base(ref)
			continue
		}
		if strings.HasPrefix(line, "detached") {
			if wt.Branch == "" {
				wt.Branch = "detached"
			}
		}
	}
	if have {
		worktrees = append(worktrees, wt)
	}
	if currentPath != "" {
		for _, w := range worktrees {
			if w.Path == "" {
				continue
			}
			if abs, err := filepath.Abs(w.Path); err == nil {
				if filepath.Clean(abs) == currentPath {
					activePath = w.Path
					break
				}
			} else if filepath.Clean(w.Path) == currentPath {
				activePath = w.Path
				break
			}
		}
	}
	return worktrees, activePath, nil
}

func AddWorktree(path, name string) (string, error) {
	root := Root(path)
	if root == "" {
		return "", errors.New("not a git repository")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("worktree name required")
	}
	parent := filepath.Dir(root)
	target := filepath.Join(parent, name)
	args := []string{"worktree", "add", "-b", name, target, "HEAD"}
	if branchExists(root, name) {
		args = []string{"worktree", "add", target, name}
	}
	cmd := exec.Command("git")
	cmd.Args = append(cmd.Args, "-C", root)
	cmd.Args = append(cmd.Args, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", err
		}
		return "", errors.New(msg)
	}
	return target, nil
}

func RemoveWorktree(path, target string) error {
	root := Root(path)
	if root == "" {
		return errors.New("not a git repository")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("worktree path required")
	}
	out, err := exec.Command("git", "-C", root, "worktree", "remove", target).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return errors.New(msg)
	}
	return nil
}

func branchExists(root, name string) bool {
	name = strings.TrimSpace(name)
	if root == "" || name == "" {
		return false
	}
	err := exec.Command("git", "-C", root, "show-ref", "--verify", "--quiet", "refs/heads/"+name).Run()
	return err == nil
}

func normalizeRootPath(root, pathAbs string) string {
	if root == "" || pathAbs == "" {
		return ""
	}
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return ""
	}
	pathResolved, err := filepath.EvalSymlinks(pathAbs)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(rootResolved, pathResolved)
	if err != nil || rel == "" || strings.HasPrefix(rel, "..") {
		return ""
	}
	candidate := filepath.Clean(pathAbs)
	if rel == "." {
		return candidate
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		candidate = filepath.Dir(candidate)
	}
	return candidate
}

// MainBranch detects the main branch of the repository (main, master, etc.)
func MainBranch(path string) string {
	root := Root(path)
	if root == "" {
		return ""
	}

	// Try to get the default branch from remote origin
	out, err := exec.Command("git", "-C", root, "symbolic-ref", "refs/remotes/origin/HEAD").CombinedOutput()
	if err == nil {
		// Output is like "refs/remotes/origin/main"
		ref := strings.TrimSpace(string(out))
		if idx := strings.LastIndex(ref, "/"); idx >= 0 {
			return ref[idx+1:]
		}
	}

	// Fallback: check if main or master exists
	branches, _, err := ListBranches(path)
	if err != nil {
		return ""
	}
	for _, b := range branches {
		if b == "main" {
			return "main"
		}
	}
	for _, b := range branches {
		if b == "master" {
			return "master"
		}
	}

	return ""
}

func findGitDir(path string) (string, error) {
	start := path
	info, err := os.Stat(start)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		start = filepath.Dir(start)
	}
	for {
		gitPath := filepath.Join(start, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			if info.IsDir() {
				return gitPath, nil
			}
			if info.Mode().IsRegular() {
				data, err := os.ReadFile(gitPath)
				if err != nil {
					return "", err
				}
				line := strings.TrimSpace(string(data))
				const prefix = "gitdir:"
				if strings.HasPrefix(line, prefix) {
					dir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
					if !filepath.IsAbs(dir) {
						dir = filepath.Join(start, dir)
					}
					return dir, nil
				}
			}
		}
		parent := filepath.Dir(start)
		if parent == start {
			break
		}
		start = parent
	}
	return "", errors.New("git dir not found")
}

func readHead(gitDir string) (string, error) {
	headPath := filepath.Join(gitDir, "HEAD")
	f, err := os.Open(headPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return "", errors.New("empty HEAD")
	}
	line := strings.TrimSpace(scanner.Text())
	const refPrefix = "ref:"
	if strings.HasPrefix(line, refPrefix) {
		ref := strings.TrimSpace(strings.TrimPrefix(line, refPrefix))
		return filepath.Base(ref), nil
	}
	if len(line) >= 7 {
		return "detached:" + line[:7], nil
	}
	return "detached", nil
}
