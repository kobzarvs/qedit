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
	GetAIState() (AIState, bool)
	SetAIState(state AIState)
	Stop()
}

// AIProviderInfo contains information about an AI provider.
type AIProviderInfo struct {
	Name        string
	DisplayName string
	Available   bool
	Status      int // 0=offline, 1=connecting, 2=online, 3=error
	ModelName   string
}

// AIState stores persisted AI selection.
type AIState struct {
	Provider string
	Model    string
	Thinking string
}

// AIModelInfo contains information about an AI model.
type AIModelInfo struct {
	ID   string
	Name string
}

// AIManager manages AI providers and communication.
type AIManager interface {
	// Provider management
	ActiveName() string
	SetActive(name string) error
	ListProviders() []AIProviderInfo
	ListAvailableProviders() []AIProviderInfo

	// Model management
	ListModels() ([]AIModelInfo, error)
	CurrentModel() string
	SetModel(model string) error

	// Communication
	Send(ctx AIContext, prompt string) error
	Cancel()
	Events() <-chan AIEvent

	// Lifecycle
	Start() error
	Stop()
}

// AIContext contains context for AI requests.
type AIContext struct {
	FilePath       string
	Content        string
	IsSelection    bool
	CursorRow      int
	CursorCol      int
	Language       string
	ReasoningLevel string
}

// AIEvent represents an event from an AI provider.
type AIEvent struct {
	Kind  string // "text", "reasoning", "reasoning_done", "edit", "error", "done"
	Text  string
	Error error
}

// AIProviderStatus represents the status of an AI provider.
type AIProviderStatus int

const (
	AIProviderStatusOffline AIProviderStatus = iota
	AIProviderStatusConnecting
	AIProviderStatusOnline
	AIProviderStatusError
)

// AIEdit represents a code edit suggestion from AI.
type AIEdit struct {
	StartLine int
	EndLine   int
	NewText   string
}

func (e *Editor) SetClipboard(c Clipboard) {
	e.systemClipboard = c
}

func (e *Editor) SetFormatter(f Formatter) {
	e.formatter = f
}

func (e *Editor) SetTerminalZoomer(z TerminalZoomer) {
	e.terminalZoomer = z
}

func (e *Editor) SetSessionStore(s SessionStore) {
	e.sessionStore = s
}

func (e *Editor) SetAIManager(m AIManager) {
	e.aiManager = m
	if e.aiPanel == nil {
		e.aiPanel = NewAIPanel()
	}
	e.restoreAIState()
	e.syncAIPanelProviderState()
}
