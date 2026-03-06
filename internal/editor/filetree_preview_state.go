package editor

type fileTreePreviewState struct {
	active    bool
	path      string
	text      *TextBuffer
	scroll    int
	scrollX   int
	binary    bool
	highlight editorHighlightState
}
