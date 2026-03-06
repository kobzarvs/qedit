package editor

func (e *Editor) hasSessionPersistence() bool {
	return e.runtime.persistence != nil && e.runtime.persistence.HasSessionState()
}

func (e *Editor) hasHistoryPersistence() bool {
	return e.runtime.persistence != nil && e.runtime.persistence.HasHistory()
}

func (e *Editor) hasUndoPersistence() bool {
	return e.runtime.persistence != nil && e.runtime.persistence.HasUndo()
}

func (e *Editor) persistenceFileState(path string) (FileState, bool) {
	return e.runtime.persistence.GetFileState(path)
}

func (e *Editor) setPersistenceFileState(path string, state FileState) {
	e.runtime.persistence.SetFileState(path, state)
}

func (e *Editor) stopPersistence() {
	e.runtime.persistence.Stop()
}

func (e *Editor) loadHistory(path string) ([]string, error) {
	return e.runtime.persistence.LoadHistory(path)
}

func (e *Editor) saveHistory(path string, entries []string) error {
	return e.runtime.persistence.SaveHistory(path, entries)
}

func (e *Editor) loadUndo(path string) ([]byte, error) {
	return e.runtime.persistence.LoadUndo(path)
}

func (e *Editor) saveUndo(path string, data []byte) error {
	return e.runtime.persistence.SaveUndo(path, data)
}

func (e *Editor) removeUndo(path string) error {
	return e.runtime.persistence.RemoveUndo(path)
}

func (e *Editor) isUndoNotExist(err error) bool {
	return e.runtime.persistence.IsUndoNotExist(err)
}
