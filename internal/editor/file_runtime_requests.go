package editor

import "errors"

func (e *Editor) prepareSave(path string) (string, []byte, error) {
	if path == "" {
		if e.document.filename == "" {
			return "", nil, errors.New("no file name")
		}
		path = e.document.filename
	}
	return path, []byte(e.Content()), nil
}

func (e *Editor) ApplySavedFile(path string) {
	e.document.filename = path
	e.savePoint = len(e.undo)
	e.file.externalChange = ExternalChangeNone
	e.file.diskContent = e.Content()
	e.updateDirty()
	_ = e.syncFileSnapshot()
	_ = e.SaveUndoHistory()
	e.saveSessionState()
	if e.buffers != nil && e.buffers.Count() > 0 {
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
	}
}

func (e *Editor) queueSaveRequest(path string, quitAfter bool) error {
	targetPath, data, err := e.prepareSave(path)
	if err != nil {
		return err
	}
	e.enqueueRuntimeRequest(RuntimeRequest{
		Kind:      RuntimeRequestSaveFile,
		Path:      targetPath,
		Content:   string(data),
		QuitAfter: quitAfter,
	})
	return nil
}

func (e *Editor) prepareReload(force bool) (string, error) {
	if e.document.filename == "" {
		return "", errors.New("no file name")
	}
	if e.HasLocalChanges() && !force {
		return "", errors.New("unsaved changes (use :e!)")
	}
	return e.document.filename, nil
}

func (e *Editor) ApplyReloadedContent(data []byte) {
	e.file.externalChange = ExternalChangeNone
	e.replaceBuffer(string(data), false)
	e.selectionActive = false
	e.file.diskContent = e.Content()
	_ = e.syncFileSnapshot()
	_ = e.LoadUndoHistory()
	if e.buffers != nil && e.buffers.Count() > 0 {
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
	}
}

func (e *Editor) queueReloadRequest(force bool) error {
	path, err := e.prepareReload(force)
	if err != nil {
		return err
	}
	e.enqueueRuntimeRequest(RuntimeRequest{
		Kind:  RuntimeRequestReloadFile,
		Path:  path,
		Force: force,
	})
	return nil
}
