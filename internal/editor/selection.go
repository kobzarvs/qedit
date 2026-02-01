package editor

// Selection tracks the active selection range.
// It is embedded in Editor to keep field access stable during refactor.
type Selection struct {
	selectionActive bool
	selectionStart  Cursor
	selectionEnd    Cursor
}
