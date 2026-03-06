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

// Merger performs a three-way merge for local/base/remote content.
type Merger interface {
	Merge(base, local, remote string) (string, bool, error)
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

type DirEntry struct {
	Name  string
	IsDir bool
}

// LanguageRuntime provides editor language-aware runtime queries.
type LanguageRuntime interface {
	NodeStack(path string, row, col int) []NodeRange
	Goto(method, path string, line, col int) ([]LSPLocation, error)
	HighlightRange(path string, startLine, endLine int) map[int][]HighlightSpan
}

// FileStore provides editor filesystem operations.
type FileStore interface {
	Abs(path string) (string, error)
	HomeDir() (string, error)
	Read(path string) ([]byte, error)
	ReadDir(path string) ([]DirEntry, error)
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

func (e *Editor) SetMerger(m Merger) {
	e.runtime.merger = m
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
	if e.buffers != nil {
		e.buffers.SetPathNormalizer(e.normalizedPath)
	}
}

func (e *Editor) SetUndoStore(s UndoStore) {
	e.runtime.undoStore = s
}

func (e *Editor) SetLanguageRuntime(r LanguageRuntime) {
	e.runtime.languageRuntime = r
}
