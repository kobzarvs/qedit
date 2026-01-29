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
