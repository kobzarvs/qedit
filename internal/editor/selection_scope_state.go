package editor

type selectionScopeState struct {
	stack []NodeRange
	index int
}
