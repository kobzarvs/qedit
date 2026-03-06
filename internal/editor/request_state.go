package editor

type openLocationRequest struct {
	active bool
	path   string
	line   int
	col    int
}

type editorRequestState struct {
	branchPickerRequested bool
	branchSelection       string
	worktreeListRequested bool
	worktreeSelection     string
	sidebarOpenFilePath   string
	openLocation          openLocationRequest
	bufferSwitched        bool
}
