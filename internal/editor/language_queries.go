package editor

func (e *Editor) hasLanguageRuntime() bool {
	return e.languageFeatures.HasAvailable(e)
}

func (e *Editor) languageGoto(method, path string, line, col int) ([]LSPLocation, error) {
	return e.languageFeatures.Goto(e, method, path, line, col)
}

func (e *Editor) languageNodeStack(path string, row, col int) []NodeRange {
	return e.languageFeatures.NodeStack(e, path, row, col)
}

func (e *Editor) languageHighlightRange(path string, startLine, endLine int) map[int][]HighlightSpan {
	return e.languageFeatures.HighlightRange(e, path, startLine, endLine)
}
