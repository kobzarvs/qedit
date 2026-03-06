package editor

type editorConflictState struct {
	blocks []conflictBlock
	dirty  bool
}
