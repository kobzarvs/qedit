package gitinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func TestBranchAndRoot(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	runGit(t, dir, "init")

	branch := Branch(dir)
	if branch == "" {
		t.Fatalf("Branch empty")
	}
	root := Root(dir)
	if root != dir {
		t.Fatalf("Root = %q, want %q", root, dir)
	}
}

func TestListBranchesAndCheckout(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "init")
	runGit(t, dir, "branch", "dev")

	branches, current, err := ListBranches(dir)
	if err != nil {
		t.Fatalf("ListBranches error: %v", err)
	}
	if current == "" {
		t.Fatalf("current branch empty")
	}
	foundDev := false
	for _, b := range branches {
		if b == "dev" {
			foundDev = true
			break
		}
	}
	if !foundDev {
		t.Fatalf("dev branch not found in %v", branches)
	}

	if err := Checkout(dir, "dev"); err != nil {
		t.Fatalf("Checkout error: %v", err)
	}
	if got := Branch(dir); got != "dev" {
		t.Fatalf("Branch = %q, want %q", got, "dev")
	}
}

func TestListBranchesNotRepo(t *testing.T) {
	dir := t.TempDir()
	_, _, err := ListBranches(dir)
	if err == nil {
		t.Fatalf("expected error for non-repo")
	}
}
