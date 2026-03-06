package editor

type editorZoomState struct {
	pendingRestore bool
	animating      bool
	savedScroll    int
	savedScrollX   int
}
