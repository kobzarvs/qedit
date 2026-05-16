package editor

type editorProfileState struct {
	name      string
	helix     helixProfileState
	vim       vimProfileState
	jumps     []Cursor
	jumpIndex int
}

type helixProfileState struct {
	count                  string
	selectionPromptRanges  []editorSelectionRange
	multiChangeActive      bool
	multiChangeRanges      []editorSelectionRange
	multiChangePrimary     int
	multiChangeStart       Cursor
	multiChangeOriginalEnd Cursor
	multiCursors           []Cursor
	surroundReplaceOld     rune
}

type vimProfileState struct {
	visual                  bool
	visualLine              bool
	visualAnchor            Cursor
	replace                 bool
	operator                string
	operatorStart           Cursor
	count                   string
	operatorCount           string
	pendingGoto             bool
	pendingTextObject       bool
	pendingTextObjectAround bool
	pendingRegister         bool
	registerName            rune
	registers               map[rune]editorClipboardState
	marks                   map[rune]Cursor
	macros                  map[rune][]vimRepeatKey
	macroRecording          bool
	macroRegister           rune
	replayingMacro          bool
	lastMacroRegister       rune
	repeatKeys              []vimRepeatKey
	recordingRepeat         []vimRepeatKey
	recordingRepeatActive   bool
	replayingRepeat         bool
}

type vimRepeatKey struct {
	key  Key
	r    rune
	mods ModMask
}

type vimRepeatEventKey struct {
	key  Key
	r    rune
	mods ModMask
}

func (k vimRepeatEventKey) Key() Key {
	return k.key
}

func (k vimRepeatEventKey) Rune() rune {
	return k.r
}

func (k vimRepeatEventKey) Modifiers() ModMask {
	return k.mods
}
