package editor

type editorViewportState struct {
	scroll  int
	scrollX int
	// height/width are the active pane's visible area (split-aware).
	height int
	width  int
	// layoutW/layoutH are the full editor content area used to lay out the window tree.
	layoutW int
	layoutH int
}
