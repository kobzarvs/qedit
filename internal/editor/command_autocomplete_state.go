package editor

type commandAutocompleteState struct {
	active        bool
	items         []CommandInfo
	index         int
	cols          int
	colGroups     [][]GroupInfo
	scroll        int // first visible row (shared across columns)
	visibleHeight int // viewport rows above the status line
	contentHeight int // tallest column in rows
}

const maxCmdAutocompleteColumns = 3
