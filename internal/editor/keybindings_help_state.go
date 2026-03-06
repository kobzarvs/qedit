package editor

type keybindingsHelpState struct {
	active      bool
	scroll      int
	filterKey   []rune
	filterAct   []rune
	filterDesc  []rune
	filterFocus int
}
