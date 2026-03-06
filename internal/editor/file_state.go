package editor

type editorFileState struct {
	snapshot             fileSnapshot
	diskContent          string
	externalChange       ExternalChange
	autoReloadOnChanges  bool
	autoReloadInProgress bool
}
