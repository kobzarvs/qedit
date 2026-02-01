package editor

// UndoManager owns undo/redo stacks and grouping state.
// It is embedded in Editor to keep field access stable during refactor.
type UndoManager struct {
	undo            []action
	redo            []action
	savePoint       int
	undoGroup       uint64
	lineUndoRow     int
	lineUndoContent []rune
	lineUndoValid   bool
}
