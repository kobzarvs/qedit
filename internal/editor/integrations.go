package editor

import "time"

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

type FileMetadata struct {
	ModTime time.Time
	Size    int64
	IsDir   bool
}

// FileStore provides current-buffer filesystem operations.
type FileStore interface {
	Abs(path string) (string, error)
	Read(path string) ([]byte, error)
	Write(path string, data []byte) error
	Stat(path string) (FileMetadata, error)
	IsNotExist(err error) bool
}

// HistoryStore persists line-based editor histories by path.
type HistoryStore interface {
	Load(path string) ([]string, error)
	Save(path string, entries []string) error
}

// UndoStore persists serialized undo history for a file path.
type UndoStore interface {
	Load(path string) ([]byte, error)
	Save(path string, data []byte) error
	Remove(path string) error
	IsNotExist(err error) bool
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

func (e *Editor) SetFileStore(s FileStore) {
	e.runtime.fileStore = s
}

func (e *Editor) SetUndoStore(s UndoStore) {
	e.runtime.undoStore = s
}
