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
	"github.com/kobzarvs/qedit/internal/logger"
)

// LMStudioProvider implements ai.Provider for the LM Studio v1 REST API.
type LMStudioProvider struct {
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

// NewLMStudioProvider creates a new LM Studio REST API provider.
func NewLMStudioProvider(config OpenAIAPIConfig) *LMStudioProvider {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &LMStudioProvider{
		config:       config,
		httpClient:   &http.Client{Timeout: timeout},
		currentModel: config.DefaultModel,
		status:       ai.StatusOffline,
		responses:    make(chan ai.AIResponse, 64),
	}
}

func (p *LMStudioProvider) Name() string {
	return p.config.Name
}

func (p *LMStudioProvider) DisplayName() string {
	return p.config.DisplayName
}

func (p *LMStudioProvider) Type() ai.ProviderType {
	return ai.ProviderTypeAPI
}

func (p *LMStudioProvider) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.apiURL("/models"), nil)
	if err != nil {
		return false
	}
	p.addHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.setStatus(ai.StatusOffline)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		p.setStatus(ai.StatusOnline)
		return true
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		p.setStatus(ai.StatusError)
		return false
	}
	p.setStatus(ai.StatusOffline)
	return false
}

func (p *LMStudioProvider) Status() ai.ProviderStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *LMStudioProvider) ListModels() ([]ai.ModelInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.apiURL("/models"), nil)
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
		Models []struct {
			Type           string `json:"type"`
			Key            string `json:"key"`
			DisplayName    string `json:"display_name"`
			LoadedInstance []struct {
				ID string `json:"id"`
			} `json:"loaded_instances"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	models := make([]ai.ModelInfo, 0, len(result.Models))
	firstLoaded := ""
	for _, m := range result.Models {
		if m.Type != "llm" {
			continue
		}
		name := m.DisplayName
		if name == "" {
			name = m.Key
		}
		models = append(models, ai.ModelInfo{ID: m.Key, Name: name})
		if firstLoaded == "" && len(m.LoadedInstance) > 0 {
			firstLoaded = m.Key
		}
	}

	p.mu.Lock()
	p.models = models
	if p.currentModel == "" {
		switch {
		case p.config.DefaultModel != "":
			p.currentModel = p.config.DefaultModel
		case firstLoaded != "":
			p.currentModel = firstLoaded
		case len(models) > 0:
			p.currentModel = models[0].ID
		}
	}
	p.mu.Unlock()

	return models, nil
}

func (p *LMStudioProvider) CurrentModel() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentModel
}

func (p *LMStudioProvider) SetModel(model string) error {
	if model == "" {
		return errors.New("model is required")
	}
	p.mu.Lock()
	p.currentModel = model
	p.mu.Unlock()
	return nil
}

func (p *LMStudioProvider) Start() error {
	p.setStatus(ai.StatusConnecting)
	if !p.Available() {
		return errors.New("API not reachable")
	}
	if _, err := p.ListModels(); err != nil {
		p.setStatus(ai.StatusError)
		return err
	}
	p.mu.Lock()
	p.running = true
	p.status = ai.StatusOnline
	p.mu.Unlock()
	return nil
}

func (p *LMStudioProvider) Stop() error {
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

func (p *LMStudioProvider) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

func (p *LMStudioProvider) Send(ctx ai.EditorContext, prompt string) error {
	return p.SendWithHistory(ctx, prompt, nil)
}

func (p *LMStudioProvider) SendWithHistory(ctx ai.EditorContext, prompt string, history []ai.ChatMessage) error {
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

	reqBody := struct {
		Model        string `json:"model"`
		Input        string `json:"input"`
		SystemPrompt string `json:"system_prompt,omitempty"`
		Reasoning    string `json:"reasoning,omitempty"`
		Stream       bool   `json:"stream"`
	}{
		Model:        model,
		Input:        prompt,
		SystemPrompt: buildSystemMessage(ctx),
		Reasoning:    normalizeReasoning(ctx.ReasoningLevel),
		Stream:       true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(reqCtx, "POST", p.apiURL("/chat"), bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	p.addHeaders(req)

	go p.streamResponse(reqCtx, req)

	return nil
}

func (p *LMStudioProvider) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancelFunc != nil {
		p.cancelFunc()
		p.cancelFunc = nil
	}
}

func (p *LMStudioProvider) Responses() <-chan ai.AIResponse {
	return p.responses
}

func (p *LMStudioProvider) streamResponse(ctx context.Context, req *http.Request) {
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

	logger.Debug("lmstudio: stream connected", "status", resp.Status)

	reader := bufio.NewReader(resp.Body)
	var eventType string
	dataLines := make([]string, 0, 2)
	streamed := false
	reasoningStreamed := false
	doneSent := false

	flush := func() {
		if eventType == "" {
			if len(dataLines) > 0 {
				logger.Debug("lmstudio: sse data without event", "bytes", len(strings.Join(dataLines, "\n")))
			}
			dataLines = dataLines[:0]
			return
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		currentEvent := eventType
		eventType = ""

		logger.Debug("lmstudio: sse event", "event", currentEvent, "bytes", len(data))
		switch currentEvent {
		case "reasoning.delta":
			var payload struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				logger.Debug("lmstudio: reasoning.delta decode failed", "error", err)
				return
			}
			if payload.Content != "" {
				reasoningStreamed = true
				logger.Debug("lmstudio: reasoning.delta", "chars", len(payload.Content))
				p.sendReasoning(payload.Content)
			}
		case "reasoning.end":
			if reasoningStreamed {
				p.sendReasoningDone()
			}
		case "message.delta":
			var payload struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				logger.Debug("lmstudio: message.delta decode failed", "error", err)
				return
			}
			if payload.Content != "" {
				streamed = true
				logger.Debug("lmstudio: message.delta", "chars", len(payload.Content))
				p.sendText(payload.Content)
			}
		case "error":
			var payload struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				logger.Debug("lmstudio: error event decode failed", "error", err)
				return
			}
			if payload.Error.Message != "" {
				logger.Warn("lmstudio: stream error", "message", payload.Error.Message)
				p.sendError(errors.New(payload.Error.Message))
			}
		case "chat.end":
			if !streamed || !reasoningStreamed {
				var payload struct {
					Result struct {
						Output []struct {
							Type    string `json:"type"`
							Content string `json:"content"`
						} `json:"output"`
					} `json:"result"`
				}
				if err := json.Unmarshal([]byte(data), &payload); err != nil {
					logger.Debug("lmstudio: chat.end decode failed", "error", err)
				} else {
					if !reasoningStreamed {
						var rb strings.Builder
						for _, item := range payload.Result.Output {
							if item.Type != "reasoning" || item.Content == "" {
								continue
							}
							if rb.Len() > 0 {
								rb.WriteString("\n")
							}
							rb.WriteString(item.Content)
						}
						if rb.Len() > 0 {
							logger.Debug("lmstudio: chat.end reasoning", "chars", rb.Len())
							p.sendReasoning(rb.String())
							p.sendReasoningDone()
						}
					}
					if !streamed {
						var sb strings.Builder
						messageCount := 0
						for _, item := range payload.Result.Output {
							if item.Type != "message" || item.Content == "" {
								continue
							}
							if sb.Len() > 0 {
								sb.WriteString("\n")
							}
							sb.WriteString(item.Content)
							messageCount++
						}
						if sb.Len() > 0 {
							logger.Debug("lmstudio: chat.end messages", "count", messageCount, "chars", sb.Len())
							p.sendText(sb.String())
						}
					}
				}
			}
			if !doneSent {
				doneSent = true
				p.sendDone()
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if eventType != "" {
					flush()
				}
				if !doneSent {
					doneSent = true
					p.sendDone()
				}
				return
			}
			if ctx.Err() == context.Canceled {
				return
			}
			p.sendError(err)
			return
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
}

func (p *LMStudioProvider) addHeaders(req *http.Request) {
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}
	for k, v := range p.config.Headers {
		req.Header.Set(k, v)
	}
}

func (p *LMStudioProvider) apiURL(path string) string {
	base := strings.TrimRight(p.config.BaseURL, "/")
	if strings.HasSuffix(base, "/api/v1") {
		return base + path
	}
	return base + "/api/v1" + path
}

func normalizeReasoning(level string) string {
	level = strings.TrimSpace(strings.ToLower(level))
	if level == "" || level == "auto" {
		return ""
	}
	return level
}

func (p *LMStudioProvider) setStatus(status ai.ProviderStatus) {
	p.mu.Lock()
	p.status = status
	p.mu.Unlock()
}

func (p *LMStudioProvider) sendText(text string) {
	select {
	case p.responses <- ai.AIResponse{Kind: ai.ResponseKindText, Text: text}:
	default:
	}
}

func (p *LMStudioProvider) sendReasoning(text string) {
	select {
	case p.responses <- ai.AIResponse{Kind: ai.ResponseKindReasoning, Text: text}:
	default:
	}
}

func (p *LMStudioProvider) sendReasoningDone() {
	select {
	case p.responses <- ai.AIResponse{Kind: ai.ResponseKindReasoningDone}:
	default:
	}
}

func (p *LMStudioProvider) sendError(err error) {
	select {
	case p.responses <- ai.AIResponse{Kind: ai.ResponseKindError, Error: err}:
	default:
	}
}

func (p *LMStudioProvider) sendDone() {
	select {
	case p.responses <- ai.AIResponse{Kind: ai.ResponseKindDone}:
	default:
	}
}
