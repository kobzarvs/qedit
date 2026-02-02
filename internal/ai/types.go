package ai

// ProviderType distinguishes between CLI and API providers.
type ProviderType int

const (
	ProviderTypeCLI ProviderType = iota // subprocess (claude, codex)
	ProviderTypeAPI                     // HTTP API (OpenAI-compatible)
)

// EditorContext contains information about the current editor state.
type EditorContext struct {
	FilePath       string // Path to the current file
	Content        string // Entire file or selection content
	IsSelection    bool   // true if Content is a selection, false if entire file
	CursorRow      int    // 0-based row
	CursorCol      int    // 0-based column
	Language       string // "go", "python", etc.
	ReasoningLevel string // reasoning setting (off/low/medium/high/on)
}

// ResponseKind indicates the type of response.
type ResponseKind string

const (
	ResponseKindText          ResponseKind = "text"           // Streaming text
	ResponseKindReasoning     ResponseKind = "reasoning"      // Streaming reasoning
	ResponseKindReasoningDone ResponseKind = "reasoning_done" // Reasoning stream completed
	ResponseKindEdit          ResponseKind = "edit"           // Proposed edit
	ResponseKindError         ResponseKind = "error"          // Error occurred
	ResponseKindDone          ResponseKind = "done"           // Stream completed
)

// AIResponse represents a response from an AI provider.
type AIResponse struct {
	Kind  ResponseKind // Type of response
	Text  string       // Text content (for text/error)
	Edit  *AIEdit      // Proposed edit (for edit)
	Error error        // Error details (for error)
}

// AIEdit represents a proposed code edit.
type AIEdit struct {
	StartLine int    // 0-based start line
	EndLine   int    // 0-based end line (exclusive)
	NewText   string // Replacement text
}

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role    string // "system", "user", "assistant"
	Content string // Message content
}

// ProviderStatus represents the current status of a provider.
type ProviderStatus int

const (
	StatusOffline ProviderStatus = iota
	StatusConnecting
	StatusOnline
	StatusError
)

func (s ProviderStatus) String() string {
	switch s {
	case StatusOffline:
		return "Offline"
	case StatusConnecting:
		return "Connecting"
	case StatusOnline:
		return "Online"
	case StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

// ModelInfo contains information about an available model.
type ModelInfo struct {
	ID          string // Model identifier (e.g., "llama3:8b")
	Name        string // Display name (e.g., "Llama 3 8B")
	Description string // Optional description
}
