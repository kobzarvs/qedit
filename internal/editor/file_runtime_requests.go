package editor

import "errors"

func (e *Editor) resolveSavePath(path string) (string, error) {
	if path == "" {
		if e.document.filename == "" {
			return "", errors.New("no file name")
		}
		path = e.document.filename
	}
	return path, nil
}

func (e *Editor) prepareSave(path string) (string, []byte, error) {
	path, err := e.resolveSavePath(path)
	if err != nil {
		return "", nil, err
	}
	if e.hugeFileActive() {
		return path, nil, nil
	}
	return path, []byte(e.Content()), nil
}

func (e *Editor) ApplySavedFile(path string) {
	e.clearGitDiffPreview()
	e.document.filename = path
	e.document.title = ""
	e.savePoint = len(e.undo)
	e.file.externalChange = ExternalChangeNone
	e.file.diskContent = e.Content()
	e.file.diskContentValid = true
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
	if e.hugeFileActive() {
		e.enqueueRuntimeRequest(RuntimeRequest{
			Kind:      RuntimeRequestSaveHugeFile,
			Path:      targetPath,
			QuitAfter: quitAfter,
		})
		return nil
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
	if e.hugeFileActive() {
		return "", errors.New("reload is unavailable in huge file mode")
	}
	if e.document.filename == "" {
		return "", errors.New("no file name")
	}
	if e.HasLocalChanges() && !force {
		return "", errors.New("unsaved changes (use :e!)")
	}
	return e.document.filename, nil
}

func (e *Editor) ApplyReloadedContent(data []byte) {
	e.clearGitDiffPreview()
	e.file.externalChange = ExternalChangeNone
	e.replaceBuffer(string(data), false)
	e.selectionActive = false
	e.file.diskContent = e.Content()
	e.file.diskContentValid = true
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
