package editor

import (
	"path/filepath"
	"strings"
)

// normalizedPath resolves a path through the runtime workspace when available.
func (e *Editor) normalizedPath(path string) string {
	return normalizedPathWithStore(e.runtime.workspace, path)
}

func normalizedPathWithStore(fs FileStore, path string) string {
	if path == "" {
		return ""
	}
	if fs != nil {
		if abs, err := fs.Abs(path); err == nil {
			return abs
		}
	}
	return path
}

func (e *Editor) relativePathFromWorkingDir(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	cwd := e.normalizedPath(".")
	if cwd == "" {
		return "", false
	}
	absPath := e.normalizedPath(path)
	if absPath == "" {
		return "", false
	}
	rel, err := filepath.Rel(cwd, absPath)
	if err != nil || rel == "" || rel == "." || rel == ".." {
		return "", false
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "..\\") {
		return "", false
	}
	return rel, true
}
