package editor

type editorFileState struct {
	snapshot             fileSnapshot
	diskContent          string
	diskContentValid     bool
	externalChange       ExternalChange
	autoReloadOnChanges  bool
	autoReloadInProgress bool
	readOnly             bool
}
