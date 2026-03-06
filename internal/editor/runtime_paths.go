package editor

// normalizedPath resolves a path through the runtime file store when available.
func (e *Editor) normalizedPath(path string) string {
	if path == "" {
		return ""
	}
	if e.runtime.fileStore != nil {
		if abs, err := e.runtime.fileStore.Abs(path); err == nil {
			return abs
		}
	}
	return path
}
