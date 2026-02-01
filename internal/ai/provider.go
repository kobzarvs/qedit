package ai

// Provider is the interface that all AI providers must implement.
type Provider interface {
	// Name returns the unique identifier for this provider (e.g., "claude", "ollama").
	Name() string

	// DisplayName returns a human-readable name (e.g., "Claude Code", "Ollama (local)").
	DisplayName() string

	// Type returns whether this is a CLI or API provider.
	Type() ProviderType

	// Available returns true if the provider is available (CLI in PATH or API reachable).
	Available() bool

	// Status returns the current status of the provider.
	Status() ProviderStatus

	// ListModels returns available models. Returns nil for CLI providers.
	ListModels() ([]ModelInfo, error)

	// CurrentModel returns the currently selected model.
	CurrentModel() string

	// SetModel selects a model. No-op for CLI providers.
	SetModel(model string) error

	// Start initializes the provider (starts subprocess or validates API connection).
	Start() error

	// Stop shuts down the provider.
	Stop() error

	// IsRunning returns true if the provider is ready to accept requests.
	IsRunning() bool

	// Send sends a prompt with context to the provider.
	// Responses are delivered through the Responses() channel.
	Send(ctx EditorContext, prompt string) error

	// SendWithHistory sends a prompt with conversation history.
	SendWithHistory(ctx EditorContext, prompt string, history []ChatMessage) error

	// Cancel cancels any in-progress request.
	Cancel()

	// Responses returns a channel for receiving responses.
	Responses() <-chan AIResponse
}
