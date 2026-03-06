package editor

// Clipboard provides access to system clipboard.
type Clipboard interface {
	Read() (string, error)
	Write(text string) error
}

// Formatter formats source code (e.g., Go via gofmt).
type Formatter interface {
	FormatGo(src string) (string, error)
}

// TerminalZoomer sends zoom commands to the terminal.
type TerminalZoomer interface {
	ZoomStep(zoomIn bool) error
}

// FileState stores persisted editor state for a file.
type FileState struct {
	CursorRow int
	CursorCol int
	ScrollY   int
	ScrollX   int
	Mode      string

	SelectionActive   bool
	SelectionStartRow int
	SelectionStartCol int
	SelectionEndRow   int
	SelectionEndCol   int
}

// SessionStore persists and restores editor file state.
type SessionStore interface {
	GetFileState(path string) (FileState, bool)
	SetFileState(path string, state FileState)
	Stop()
}

// HistoryStore persists line-based editor histories by path.
type HistoryStore interface {
	Load(path string) ([]string, error)
	Save(path string, entries []string) error
}

func (e *Editor) SetClipboard(c Clipboard) {
	e.runtime.systemClipboard = c
}

func (e *Editor) SetFormatter(f Formatter) {
	e.runtime.formatter = f
}

func (e *Editor) SetTerminalZoomer(z TerminalZoomer) {
	e.runtime.terminalZoomer = z
}

func (e *Editor) SetSessionStore(s SessionStore) {
	e.runtime.sessionStore = s
}

func (e *Editor) SetHistoryStore(s HistoryStore) {
	e.runtime.historyStore = s
}
