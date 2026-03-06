package editor

func (e *Editor) hasLanguageRuntime() bool {
	return e.runtime.languageRuntime != nil
}

func (e *Editor) languageGoto(method, path string, line, col int) ([]LSPLocation, error) {
	if e.runtime.languageRuntime == nil {
		return nil, nil
	}
	return e.runtime.languageRuntime.Goto(method, path, line, col)
}

func (e *Editor) languageNodeStack(path string, row, col int) []NodeRange {
	if e.runtime.languageRuntime == nil {
		return nil
	}
	return e.runtime.languageRuntime.NodeStack(path, row, col)
}

func (e *Editor) languageHighlightRange(path string, startLine, endLine int) map[int][]HighlightSpan {
	if e.runtime.languageRuntime == nil {
		return nil
	}
	return e.runtime.languageRuntime.HighlightRange(path, startLine, endLine)
}
