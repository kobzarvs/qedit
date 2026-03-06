package editor

type commandAutocompleteState struct {
	active    bool
	items     []CommandInfo
	index     int
	cols      int
	colGroups [][]GroupInfo
}
