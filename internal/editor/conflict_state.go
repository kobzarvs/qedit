package editor

type mergeReviewPane uint8

const (
	mergeReviewPaneLocal mergeReviewPane = iota
	mergeReviewPaneResult
	mergeReviewPaneRemote
)

type editorMergeReviewState struct {
	active bool
	pane   mergeReviewPane
}

type editorConflictState struct {
	blocks []conflictBlock
	dirty  bool
	review editorMergeReviewState
}
