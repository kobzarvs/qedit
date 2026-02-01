package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kobzarvs/qedit/internal/ai"
)

// OpenAIAPIConfig configures an OpenAI-compatible API provider.
type OpenAIAPIConfig struct {
	Name         string            // Unique identifier (e.g., "ollama")
	DisplayName  string            // Human-readable name (e.g., "Ollama (local)")
	BaseURL      string            // API base URL (e.g., "http://localhost:11434/v1")
	APIKey       string            // API key (empty for local providers)
	DefaultModel string            // Default model to use
	Headers      map[string]string // Additional headers
	Timeout      time.Duration     // Request timeout (0 = default 30s)
}

// OpenAIAPIProvider implements ai.Provider for OpenAI-compatible APIs.
type OpenAIAPIProvider struct {
	config       OpenAIAPIConfig
	httpClient   *http.Client
	currentModel string
	models       []ai.ModelInfo
	status       ai.ProviderStatus
	running      bool
	responses    chan ai.AIResponse
	cancelFunc   context.CancelFunc
	mu           sync.RWMutex
}

// NewOpenAIAPIProvider creates a new OpenAI API provider.
func NewOpenAIAPIProvider(config OpenAIAPIConfig) *OpenAIAPIProvider {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &OpenAIAPIProvider{
		config:       config,
		httpClient:   &http.Client{Timeout: timeout},
		currentModel: config.DefaultModel,
		status:       ai.StatusOffline,
		responses:    make(chan ai.AIResponse, 64),
	}
}

// Name returns the provider name.
func (p *OpenAIAPIProvider) Name() string {
	return p.config.Name
}

// DisplayName returns the human-readable name.
func (p *OpenAIAPIProvider) DisplayName() string {
	return p.config.DisplayName
}

// Type returns ProviderTypeAPI.
func (p *OpenAIAPIProvider) Type() ai.ProviderType {
	return ai.ProviderTypeAPI
}

// Available checks if the API is reachable.
func (p *OpenAIAPIProvider) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+"/models", nil)
	if err != nil {
		return false
	}
	p.addHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Status returns the current provider status.
func (p *OpenAIAPIProvider) Status() ai.ProviderStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// ListModels fetches available models from the API.
func (p *OpenAIAPIProvider) ListModels() ([]ai.ModelInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	p.addHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
		// Ollama format
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	var models []ai.ModelInfo

	// Standard OpenAI format
	for _, m := range result.Data {
		models = append(models, ai.ModelInfo{
			ID:   m.ID,
			Name: m.ID,
		})
	}

	// Ollama format fallback
	if len(models) == 0 {
		for _, m := range result.Models {
			models = append(models, ai.ModelInfo{
				ID:   m.Name,
				Name: m.Name,
			})
		}
	}

	p.mu.Lock()
	p.models = models
	p.mu.Unlock()

	return models, nil
}

// CurrentModel returns the selected model.
func (p *OpenAIAPIProvider) CurrentModel() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentModel
}

// SetModel sets the current model.
func (p *OpenAIAPIProvider) SetModel(model string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentModel = model
	return nil
}

// Start initializes the provider.
func (p *OpenAIAPIProvider) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.status = ai.StatusConnecting

	// Test connection and fetch models
	if !p.Available() {
		p.status = ai.StatusOffline
		return errors.New("API not reachable")
	}

	// Fetch models
	models, err := p.ListModels()
	if err != nil {
		p.status = ai.StatusError
		return err
	}

	// Set default model if not set
	if p.currentModel == "" && len(models) > 0 {
		p.currentModel = models[0].ID
	}

	p.running = true
	p.status = ai.StatusOnline
	return nil
}

// Stop shuts down the provider.
func (p *OpenAIAPIProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancelFunc != nil {
		p.cancelFunc()
		p.cancelFunc = nil
	}

	p.running = false
	p.status = ai.StatusOffline
	return nil
}

// IsRunning returns true if the provider is ready.
func (p *OpenAIAPIProvider) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Send sends a prompt to the API.
func (p *OpenAIAPIProvider) Send(ctx ai.EditorContext, prompt string) error {
	return p.SendWithHistory(ctx, prompt, nil)
}

// SendWithHistory sends a prompt with conversation history.
func (p *OpenAIAPIProvider) SendWithHistory(ctx ai.EditorContext, prompt string, history []ai.ChatMessage) error {
	p.mu.Lock()
	model := p.currentModel
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
	reqCtx, cancel := context.WithCancel(context.Background())
	p.cancelFunc = cancel
	p.mu.Unlock()

	if model == "" {
		return errors.New("no model selected")
	}

	// Build messages
	messages := make([]chatMessage, 0, len(history)+2)

	// System message with context
	systemMsg := buildSystemMessage(ctx)
	messages = append(messages, chatMessage{Role: "system", Content: systemMsg})

	// Add history
	for _, h := range history {
		messages = append(messages, chatMessage{Role: h.Role, Content: h.Content})
	}

	// Add user prompt
	messages = append(messages, chatMessage{Role: "user", Content: prompt})

	// Create request
	reqBody := chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(reqCtx, "POST", p.config.BaseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	p.addHeaders(req)

	// Send request
	go p.streamResponse(reqCtx, req)

	return nil
}

// Cancel cancels any in-progress request.
func (p *OpenAIAPIProvider) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancelFunc != nil {
		p.cancelFunc()
		p.cancelFunc = nil
	}
}

// Responses returns the response channel.
func (p *OpenAIAPIProvider) Responses() <-chan ai.AIResponse {
	return p.responses
}

// streamResponse handles streaming response from the API.
func (p *OpenAIAPIProvider) streamResponse(ctx context.Context, req *http.Request) {
	// Use a client without timeout for streaming
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return
		}
		p.sendError(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		p.sendError(fmt.Errorf("API error: %s - %s", resp.Status, string(body)))
		return
	}

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				p.sendDone()
				return
			}
			if ctx.Err() == context.Canceled {
				return
			}
			p.sendError(err)
			return
		}

		// Parse SSE line
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// SSE format: "data: {...}"
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		data := bytes.TrimPrefix(line, []byte("data: "))

		// Check for stream end
		if string(data) == "[DONE]" {
			p.sendDone()
			return
		}

		// Parse chunk
		var chunk chatChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			continue
		}

		// Extract content
		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			if content != "" {
				p.sendText(content)
			}
		}
	}
}

func (p *OpenAIAPIProvider) sendText(text string) {
	select {
	case p.responses <- ai.AIResponse{Kind: ai.ResponseKindText, Text: text}:
	default:
	}
}

func (p *OpenAIAPIProvider) sendError(err error) {
	select {
	case p.responses <- ai.AIResponse{Kind: ai.ResponseKindError, Error: err}:
	default:
	}
}

func (p *OpenAIAPIProvider) sendDone() {
	select {
	case p.responses <- ai.AIResponse{Kind: ai.ResponseKindDone}:
	default:
	}
}

func (p *OpenAIAPIProvider) addHeaders(req *http.Request) {
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}
	for k, v := range p.config.Headers {
		req.Header.Set(k, v)
	}
}

func buildSystemMessage(ctx ai.EditorContext) string {
	var sb strings.Builder

	sb.WriteString("You are a helpful AI coding assistant integrated into a code editor. ")
	sb.WriteString("Provide concise, helpful responses focused on the user's code and questions.\n\n")

	if ctx.FilePath != "" {
		fmt.Fprintf(&sb, "Current file: %s\n", ctx.FilePath)
	}
	if ctx.Language != "" {
		fmt.Fprintf(&sb, "Language: %s\n", ctx.Language)
	}

	if ctx.Content != "" {
		if ctx.IsSelection {
			sb.WriteString("\n--- Selected Code ---\n")
		} else {
			sb.WriteString("\n--- File Content ---\n")
		}
		sb.WriteString(ctx.Content)
		sb.WriteString("\n--- End ---\n")
	}

	sb.WriteString("\nWhen suggesting code changes, wrap them in markdown code blocks with the language specified.")

	return sb.String()
}

// OpenAI API types

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}
