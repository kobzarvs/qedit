package editor

type refsPickerState struct {
	active     bool
	items      []LSPLocation
	index      int
	title      string
	fileCache  map[string][][]rune
	highlights map[string]map[int][]HighlightSpan
}
