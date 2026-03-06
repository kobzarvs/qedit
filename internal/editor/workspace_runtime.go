package editor

func (e *Editor) workspaceFileStore() FileStore {
	if e.runtime.workspace == nil || !e.runtime.workspace.HasFileStore() {
		return nil
	}
	return e.runtime.workspace
}

func (e *Editor) hasWorkspaceFormatter() bool {
	return e.runtime.workspace != nil && e.runtime.workspace.HasFormatter()
}

func (e *Editor) hasWorkspaceMerger() bool {
	return e.runtime.workspace != nil && e.runtime.workspace.HasMerger()
}

func (e *Editor) workspaceAbs(path string) (string, error) {
	return e.runtime.workspace.Abs(path)
}

func (e *Editor) workspaceRead(path string) ([]byte, error) {
	return e.runtime.workspace.Read(path)
}

func (e *Editor) workspaceWrite(path string, data []byte) error {
	return e.runtime.workspace.Write(path, data)
}

func (e *Editor) workspaceStat(path string) (FileMetadata, error) {
	return e.runtime.workspace.Stat(path)
}

func (e *Editor) workspaceIsNotExist(err error) bool {
	return e.runtime.workspace.IsNotExist(err)
}

func (e *Editor) workspaceFormatGo(src string) (string, error) {
	return e.runtime.workspace.FormatGo(src)
}

func (e *Editor) workspaceMerge(base, local, remote string) (string, bool, error) {
	return e.runtime.workspace.Merge(base, local, remote)
}
