package app

import "github.com/kobzarvs/qedit/internal/editor"

func normalizeAppPath(fs editor.FileStore, path string) string {
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
