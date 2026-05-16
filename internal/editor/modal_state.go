package editor

type editorModalState struct {
	pendingAction    string
	selectMode       bool
	lastFindChar     rune
	lastFindForward  bool
	lastFindTill     bool
	gotoMode         bool
	matchMode        bool
	viewMode         bool
	windowMode       bool
	windowNewPending bool
	pendingKeys      string
	lastCommand      string
	spaceMenuActive  bool
}
