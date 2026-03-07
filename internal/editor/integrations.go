package editor

import "io"

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

// GitRuntime provides editor git-aware runtime queries and operations.
type GitRuntime interface {
	Root(path string) string
	Branch(path string) string
	MainBranch(path string) string
	ListBranches(root string) ([]string, string, error)
	ListWorktrees(root string) ([]WorktreeInfo, string, error)
	Checkout(root, branch string) error
	AddWorktree(root, name string) (string, error)
	RemoveWorktree(root, path string) error
	Changes(root string) ([]GitFileChange, []GitChangeHunk, error)
	Diff(root, path string) (string, error)
}

// WorkspaceRuntime provides file, merge, and formatting operations.
type WorkspaceRuntime interface {
	HasFileStore() bool
	HasFormatter() bool
	HasMerger() bool
	Abs(path string) (string, error)
	HomeDir() (string, error)
	Open(path string) (io.ReadSeekCloser, error)
	Read(path string) ([]byte, error)
	ReadDir(path string) ([]DirEntry, error)
	Write(path string, data []byte) error
	Stat(path string) (FileMetadata, error)
	IsNotExist(err error) bool
	FormatGo(src string) (string, error)
	Merge(base, local, remote string) (string, bool, error)
}

// PersistenceRuntime provides editor state persistence.
type PersistenceRuntime interface {
	HasSessionState() bool
	HasHistory() bool
	HasUndo() bool
	GetFileState(path string) (FileState, bool)
	SetFileState(path string, state FileState)
	Stop()
	LoadHistory(path string) ([]string, error)
	SaveHistory(path string, entries []string) error
	LoadUndo(path string) ([]byte, error)
	SaveUndo(path string, data []byte) error
	RemoveUndo(path string) error
	IsUndoNotExist(err error) bool
}

// RuntimeServices groups external editor runtime dependencies.
type RuntimeServices struct {
	SystemClipboard    Clipboard
	TerminalZoomer     TerminalZoomer
	WorkspaceRuntime   WorkspaceRuntime
	Formatter          Formatter
	Merger             Merger
	PersistenceRuntime PersistenceRuntime
	SessionStore       SessionStore
	HistoryStore       HistoryStore
	FileStore          FileStore
	UndoStore          UndoStore
	LanguageRuntime    LanguageRuntime
	GitRuntime         GitRuntime
}

// FileStore provides editor filesystem operations.
type FileStore interface {
	Abs(path string) (string, error)
	HomeDir() (string, error)
	Open(path string) (io.ReadSeekCloser, error)
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

type storeBackedPersistenceRuntime struct {
	sessionStore SessionStore
	historyStore HistoryStore
	undoStore    UndoStore
}

type storeBackedWorkspaceRuntime struct {
	fileStore FileStore
	formatter Formatter
	merger    Merger
}

func NewStoreBackedWorkspaceRuntime(fileStore FileStore, formatter Formatter, merger Merger) WorkspaceRuntime {
	return &storeBackedWorkspaceRuntime{
		fileStore: fileStore,
		formatter: formatter,
		merger:    merger,
	}
}

func NewStoreBackedPersistenceRuntime(sessionStore SessionStore, historyStore HistoryStore, undoStore UndoStore) PersistenceRuntime {
	return &storeBackedPersistenceRuntime{
		sessionStore: sessionStore,
		historyStore: historyStore,
		undoStore:    undoStore,
	}
}

func (r *storeBackedWorkspaceRuntime) HasFileStore() bool {
	return r != nil && r.fileStore != nil
}

func (r *storeBackedWorkspaceRuntime) HasFormatter() bool {
	return r != nil && r.formatter != nil
}

func (r *storeBackedWorkspaceRuntime) HasMerger() bool {
	return r != nil && r.merger != nil
}

func (r *storeBackedWorkspaceRuntime) Abs(path string) (string, error) {
	if r == nil || r.fileStore == nil {
		return path, nil
	}
	return r.fileStore.Abs(path)
}

func (r *storeBackedWorkspaceRuntime) HomeDir() (string, error) {
	if r == nil || r.fileStore == nil {
		return "", nil
	}
	return r.fileStore.HomeDir()
}

func (r *storeBackedWorkspaceRuntime) Open(path string) (io.ReadSeekCloser, error) {
	if r == nil || r.fileStore == nil {
		return nil, nil
	}
	return r.fileStore.Open(path)
}

func (r *storeBackedWorkspaceRuntime) Read(path string) ([]byte, error) {
	if r == nil || r.fileStore == nil {
		return nil, nil
	}
	return r.fileStore.Read(path)
}

func (r *storeBackedWorkspaceRuntime) ReadDir(path string) ([]DirEntry, error) {
	if r == nil || r.fileStore == nil {
		return nil, nil
	}
	return r.fileStore.ReadDir(path)
}

func (r *storeBackedWorkspaceRuntime) Write(path string, data []byte) error {
	if r == nil || r.fileStore == nil {
		return nil
	}
	return r.fileStore.Write(path, data)
}

func (r *storeBackedWorkspaceRuntime) Stat(path string) (FileMetadata, error) {
	if r == nil || r.fileStore == nil {
		return FileMetadata{}, nil
	}
	return r.fileStore.Stat(path)
}

func (r *storeBackedWorkspaceRuntime) IsNotExist(err error) bool {
	if r == nil || r.fileStore == nil {
		return false
	}
	return r.fileStore.IsNotExist(err)
}

func (r *storeBackedWorkspaceRuntime) FormatGo(src string) (string, error) {
	if r == nil || r.formatter == nil {
		return "", nil
	}
	return r.formatter.FormatGo(src)
}

func (r *storeBackedWorkspaceRuntime) Merge(base, local, remote string) (string, bool, error) {
	if r == nil || r.merger == nil {
		return "", false, nil
	}
	return r.merger.Merge(base, local, remote)
}

func (r *storeBackedPersistenceRuntime) GetFileState(path string) (FileState, bool) {
	if r == nil || r.sessionStore == nil {
		return FileState{}, false
	}
	return r.sessionStore.GetFileState(path)
}

func (r *storeBackedPersistenceRuntime) HasSessionState() bool {
	return r != nil && r.sessionStore != nil
}

func (r *storeBackedPersistenceRuntime) HasHistory() bool {
	return r != nil && r.historyStore != nil
}

func (r *storeBackedPersistenceRuntime) HasUndo() bool {
	return r != nil && r.undoStore != nil
}

func (r *storeBackedPersistenceRuntime) SetFileState(path string, state FileState) {
	if r == nil || r.sessionStore == nil {
		return
	}
	r.sessionStore.SetFileState(path, state)
}

func (r *storeBackedPersistenceRuntime) Stop() {
	if r == nil || r.sessionStore == nil {
		return
	}
	r.sessionStore.Stop()
}

func (r *storeBackedPersistenceRuntime) LoadHistory(path string) ([]string, error) {
	if r == nil || r.historyStore == nil {
		return nil, nil
	}
	return r.historyStore.Load(path)
}

func (r *storeBackedPersistenceRuntime) SaveHistory(path string, entries []string) error {
	if r == nil || r.historyStore == nil {
		return nil
	}
	return r.historyStore.Save(path, entries)
}

func (r *storeBackedPersistenceRuntime) LoadUndo(path string) ([]byte, error) {
	if r == nil || r.undoStore == nil {
		return nil, nil
	}
	return r.undoStore.Load(path)
}

func (r *storeBackedPersistenceRuntime) SaveUndo(path string, data []byte) error {
	if r == nil || r.undoStore == nil {
		return nil
	}
	return r.undoStore.Save(path, data)
}

func (r *storeBackedPersistenceRuntime) RemoveUndo(path string) error {
	if r == nil || r.undoStore == nil {
		return nil
	}
	return r.undoStore.Remove(path)
}

func (r *storeBackedPersistenceRuntime) IsUndoNotExist(err error) bool {
	if r == nil || r.undoStore == nil {
		return false
	}
	return r.undoStore.IsNotExist(err)
}

func (e *Editor) ensureStoreBackedPersistenceRuntime() *storeBackedPersistenceRuntime {
	if runtime, ok := e.runtime.persistence.(*storeBackedPersistenceRuntime); ok {
		return runtime
	}
	runtime := &storeBackedPersistenceRuntime{}
	e.runtime.persistence = runtime
	return runtime
}

func (e *Editor) ensureStoreBackedWorkspaceRuntime() *storeBackedWorkspaceRuntime {
	if runtime, ok := e.runtime.workspace.(*storeBackedWorkspaceRuntime); ok {
		return runtime
	}
	runtime := &storeBackedWorkspaceRuntime{}
	e.runtime.workspace = runtime
	return runtime
}

func (e *Editor) SetClipboard(c Clipboard) {
	e.runtime.systemClipboard = c
}

func (e *Editor) SetFormatter(f Formatter) {
	e.ensureStoreBackedWorkspaceRuntime().formatter = f
}

func (e *Editor) SetMerger(m Merger) {
	e.ensureStoreBackedWorkspaceRuntime().merger = m
}

func (e *Editor) SetTerminalZoomer(z TerminalZoomer) {
	e.runtime.terminalZoomer = z
}

func (e *Editor) SetWorkspaceRuntime(r WorkspaceRuntime) {
	e.runtime.workspace = r
}

func (e *Editor) SetPersistenceRuntime(r PersistenceRuntime) {
	e.runtime.persistence = r
}

func (e *Editor) SetSessionStore(s SessionStore) {
	e.ensureStoreBackedPersistenceRuntime().sessionStore = s
}

func (e *Editor) SetHistoryStore(s HistoryStore) {
	e.ensureStoreBackedPersistenceRuntime().historyStore = s
}

func (e *Editor) SetFileStore(s FileStore) {
	e.ensureStoreBackedWorkspaceRuntime().fileStore = s
	if e.buffers != nil {
		e.buffers.SetPathNormalizer(e.normalizedPath)
	}
}

func (e *Editor) SetUndoStore(s UndoStore) {
	e.ensureStoreBackedPersistenceRuntime().undoStore = s
}

func (e *Editor) SetLanguageRuntime(r LanguageRuntime) {
	e.runtime.languageRuntime = r
}

func (e *Editor) SetGitRuntime(r GitRuntime) {
	e.runtime.gitRuntime = r
}

func (e *Editor) ApplyRuntimeServices(services RuntimeServices) {
	if services.SystemClipboard != nil {
		e.runtime.systemClipboard = services.SystemClipboard
	}
	if services.WorkspaceRuntime != nil {
		e.runtime.workspace = services.WorkspaceRuntime
	}
	if services.Formatter != nil {
		e.ensureStoreBackedWorkspaceRuntime().formatter = services.Formatter
	}
	if services.Merger != nil {
		e.ensureStoreBackedWorkspaceRuntime().merger = services.Merger
	}
	if services.TerminalZoomer != nil {
		e.runtime.terminalZoomer = services.TerminalZoomer
	}
	if services.PersistenceRuntime != nil {
		e.runtime.persistence = services.PersistenceRuntime
	}
	if services.SessionStore != nil {
		e.ensureStoreBackedPersistenceRuntime().sessionStore = services.SessionStore
	}
	if services.HistoryStore != nil {
		e.ensureStoreBackedPersistenceRuntime().historyStore = services.HistoryStore
	}
	if services.FileStore != nil {
		e.ensureStoreBackedWorkspaceRuntime().fileStore = services.FileStore
		if e.buffers != nil {
			e.buffers.SetPathNormalizer(e.normalizedPath)
		}
	}
	if services.UndoStore != nil {
		e.ensureStoreBackedPersistenceRuntime().undoStore = services.UndoStore
	}
	if services.LanguageRuntime != nil {
		e.runtime.languageRuntime = services.LanguageRuntime
	}
	if services.GitRuntime != nil {
		e.runtime.gitRuntime = services.GitRuntime
	}
}
