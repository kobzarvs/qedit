package ai

import (
	"errors"
	"sync"
)

var (
	ErrNoProvider       = errors.New("no AI provider available")
	ErrProviderNotFound = errors.New("AI provider not found")
	ErrNoActiveProvider = errors.New("no active AI provider")
)

// Manager manages AI providers and routes requests.
type Manager struct {
	mu        sync.RWMutex
	providers map[string]Provider
	active    string       // Name of the active provider
	events    chan AIResponse
}

// NewManager creates a new AI manager.
func NewManager() *Manager {
	return &Manager{
		providers: make(map[string]Provider),
		events:    make(chan AIResponse, 64),
	}
}

// Register adds a provider to the manager.
func (m *Manager) Register(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p

	// Set as active if it's the first provider
	if m.active == "" {
		m.active = p.Name()
	}
}

// Unregister removes a provider from the manager.
func (m *Manager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.providers[name]; ok {
		p.Stop()
		delete(m.providers, name)
	}

	// Clear active if this was the active provider
	if m.active == name {
		m.active = ""
		// Pick another provider if available
		for n := range m.providers {
			m.active = n
			break
		}
	}
}

// SetActive sets the active provider by name.
func (m *Manager) SetActive(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.providers[name]; !ok {
		return ErrProviderNotFound
	}
	m.active = name
	return nil
}

// Active returns the currently active provider.
func (m *Manager) Active() Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.active == "" {
		return nil
	}
	return m.providers[m.active]
}

// ActiveName returns the name of the active provider.
func (m *Manager) ActiveName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// Get returns a provider by name.
func (m *Manager) Get(name string) (Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[name]
	return p, ok
}

// List returns all registered providers.
func (m *Manager) List() []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Provider, 0, len(m.providers))
	for _, p := range m.providers {
		result = append(result, p)
	}
	return result
}

// ListAvailable returns providers that are available.
func (m *Manager) ListAvailable() []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Provider
	for _, p := range m.providers {
		if p.Available() {
			result = append(result, p)
		}
	}
	return result
}

// Send sends a request through the active provider.
func (m *Manager) Send(ctx EditorContext, prompt string) error {
	p := m.Active()
	if p == nil {
		return ErrNoActiveProvider
	}

	// Start provider if not running
	if !p.IsRunning() {
		if err := p.Start(); err != nil {
			return err
		}
	}

	// Forward responses to manager's event channel
	go m.forwardResponses(p)

	return p.Send(ctx, prompt)
}

// SendWithHistory sends a request with conversation history.
func (m *Manager) SendWithHistory(ctx EditorContext, prompt string, history []ChatMessage) error {
	p := m.Active()
	if p == nil {
		return ErrNoActiveProvider
	}

	// Start provider if not running
	if !p.IsRunning() {
		if err := p.Start(); err != nil {
			return err
		}
	}

	// Forward responses to manager's event channel
	go m.forwardResponses(p)

	return p.SendWithHistory(ctx, prompt, history)
}

// Cancel cancels any in-progress request on the active provider.
func (m *Manager) Cancel() {
	if p := m.Active(); p != nil {
		p.Cancel()
	}
}

// Events returns the channel for receiving AI responses.
func (m *Manager) Events() <-chan AIResponse {
	return m.events
}

// forwardResponses forwards responses from a provider to the manager's event channel.
func (m *Manager) forwardResponses(p Provider) {
	for resp := range p.Responses() {
		select {
		case m.events <- resp:
		default:
			// Drop if channel is full (shouldn't happen with proper consumption)
		}
	}
}

// Stop stops all providers.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.providers {
		p.Stop()
	}
}

// StartActive starts the active provider.
func (m *Manager) StartActive() error {
	p := m.Active()
	if p == nil {
		return ErrNoActiveProvider
	}
	return p.Start()
}
