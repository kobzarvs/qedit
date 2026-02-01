package editor

// Buffer holds the editable text and cursor position.
// It is embedded in Editor to keep field access stable during refactor.
type Buffer struct {
	text   *TextBuffer
	cursor Cursor
}
