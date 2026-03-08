package integrations

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoFormatter formats Go source using gofmt.
type GoFormatter struct{}

func (GoFormatter) FormatGo(src string) (string, error) {
	cmd := exec.Command("gofmt")
	cmd.Stdin = strings.NewReader(src)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", errors.New(msg)
		}
		return "", err
	}
	return out.String(), nil
}

func FormatPrettier(path, src string) (string, error) {
	cmd, err := resolvePrettierCommand(path)
	if err != nil {
		return "", err
	}
	cmd.Args = append(cmd.Args, "--stdin-filepath", path)
	cmd.Stdin = strings.NewReader(src)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", errors.New(msg)
		}
		return "", err
	}
	return out.String(), nil
}

func resolvePrettierCommand(path string) (*exec.Cmd, error) {
	for _, dir := range prettierSearchDirs(path) {
		bin := filepath.Join(dir, "node_modules", ".bin", "prettier")
		if isExecutableFile(bin) {
			cmd := exec.Command(bin)
			cmd.Dir = dir
			return cmd, nil
		}
		nodeScript := filepath.Join(dir, "node_modules", "prettier", "bin", "prettier.cjs")
		if isRegularFile(nodeScript) {
			if nodePath, err := exec.LookPath("node"); err == nil {
				cmd := exec.Command(nodePath, nodeScript)
				cmd.Dir = dir
				return cmd, nil
			}
		}
	}
	if prettierPath, err := exec.LookPath("prettier"); err == nil {
		cmd := exec.Command(prettierPath)
		if dir := filepath.Dir(path); dir != "" {
			cmd.Dir = dir
		}
		return cmd, nil
	}
	if bunPath, err := exec.LookPath("bun"); err == nil {
		cmd := exec.Command(bunPath, "x", "prettier")
		if dir := filepath.Dir(path); dir != "" {
			cmd.Dir = dir
		}
		return cmd, nil
	}
	if npxPath, err := exec.LookPath("npx"); err == nil {
		cmd := exec.Command(npxPath, "--yes", "prettier")
		if dir := filepath.Dir(path); dir != "" {
			cmd.Dir = dir
		}
		return cmd, nil
	}
	return nil, errors.New("prettier not found")
}

func prettierSearchDirs(path string) []string {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	var dirs []string
	for {
		dirs = append(dirs, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}
