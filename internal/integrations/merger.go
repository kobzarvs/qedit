package integrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GitMerger performs three-way merges via `git merge-file -p`.
type GitMerger struct{}

func (GitMerger) Merge(base, local, remote string) (string, bool, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", false, err
	}
	tmpDir, err := os.MkdirTemp("", "qedit-merge-")
	if err != nil {
		return "", false, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	basePath := filepath.Join(tmpDir, "base")
	localPath := filepath.Join(tmpDir, "local")
	remotePath := filepath.Join(tmpDir, "remote")

	if err := os.WriteFile(basePath, []byte(base), 0o644); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(localPath, []byte(local), 0o644); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(remotePath, []byte(remote), 0o644); err != nil {
		return "", false, err
	}

	cmd := exec.Command(gitPath, "merge-file", "-p", localPath, basePath, remotePath)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return string(output), true, nil
		}
		return "", false, fmt.Errorf("merge-file failed: %w", err)
	}
	return string(output), false, nil
}
