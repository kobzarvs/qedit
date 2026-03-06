package editor

type editorProfileState struct {
	name string
	vim  vimProfileState
}

type vimProfileState struct {
	visual        bool
	operator      string
	count         string
	operatorCount string
	pendingGoto   bool
}
