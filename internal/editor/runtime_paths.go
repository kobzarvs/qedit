package editor

// normalizedPath resolves a path through the runtime file store when available.
func (e *Editor) normalizedPath(path string) string {
	return normalizedPathWithStore(e.runtime.fileStore, path)
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
