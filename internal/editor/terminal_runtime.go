package editor

func (e *Editor) hasTerminalZoomer() bool {
	return e.runtime.terminalZoomer != nil
}

func (e *Editor) terminalZoomStep(zoomIn bool) {
	if e.runtime.terminalZoomer == nil {
		return
	}
	_ = e.runtime.terminalZoomer.ZoomStep(zoomIn)
}
