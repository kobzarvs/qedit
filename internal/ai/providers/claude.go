package providers

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/kobzarvs/qedit/internal/ai"
)

// ClaudeProvider implements AI provider for Claude Code CLI.
type ClaudeProvider struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	running   bool
	responses chan ai.AIResponse
	cancel    chan struct{}

	binaryPath string // Path to claude binary (empty = search in PATH)
}

// NewClaudeProvider creates a new Claude Code CLI provider.
func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{
		responses: make(chan ai.AIResponse, 100),
		cancel:    make(chan struct{}),
	}
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) DisplayName() string {
	return "Claude Code"
}

func (p *ClaudeProvider) Type() ai.ProviderType {
	return ai.ProviderTypeCLI
}

func (p *ClaudeProvider) Available() bool {
	// Check if claude binary is available
	binary := p.binaryPath
	if binary == "" {
		binary = "claude"
	}
	_, err := exec.LookPath(binary)
	return err == nil
}

func (p *ClaudeProvider) Status() ai.ProviderStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.Available() {
		return ai.StatusOffline
	}
	if p.running {
		return ai.StatusOnline
	}
	return ai.StatusOffline
}

func (p *ClaudeProvider) ListModels() ([]ai.ModelInfo, error) {
	// Claude Code CLI uses the model configured in the CLI itself
	// We can't change it from here, so return a single "default" model
	return []ai.ModelInfo{
		{ID: "default", Name: "Claude (CLI default)"},
	}, nil
}

func (p *ClaudeProvider) CurrentModel() string {
	return "default"
}

func (p *ClaudeProvider) SetModel(model string) error {
	// Claude Code CLI model is configured externally
	return nil
}

func (p *ClaudeProvider) Start() error {
	// Don't start persistently - we'll start per-request
	return nil
}

func (p *ClaudeProvider) startProcess(prompt string) error {
	p.mu.Lock()

	// Kill any existing process
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}

	binary := p.binaryPath
	if binary == "" {
		binary = "claude"
	}

	// Reset cancel channel
	select {
	case <-p.cancel:
		// Channel was closed, create new one
		p.cancel = make(chan struct{})
	default:
	}

	// Start claude with the prompt directly as argument
	// Using -p for print mode (non-interactive, outputs to stdout)
	p.cmd = exec.Command(binary, "-p", prompt)

	var err error
	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := p.cmd.Start(); err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to start claude: %w", err)
	}

	p.running = true
	p.mu.Unlock()

	// Start reading stdout in goroutine
	go p.readOutput()

	// Wait for process to complete in background
	go func() {
		p.cmd.Wait()
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	return nil
}

func (p *ClaudeProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	// Signal cancel
	close(p.cancel)

	// Close stdin to signal EOF
	if p.stdin != nil {
		p.stdin.Close()
	}

	// Wait for process to exit
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}

	p.running = false
	return nil
}

func (p *ClaudeProvider) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *ClaudeProvider) Send(ctx ai.EditorContext, prompt string) error {
	// Build the full prompt with context
	var fullPrompt strings.Builder

	if ctx.FilePath != "" {
		fmt.Fprintf(&fullPrompt, "File: %s\n", ctx.FilePath)
		if ctx.Language != "" {
			fmt.Fprintf(&fullPrompt, "Language: %s\n", ctx.Language)
		}
	}

	if ctx.IsSelection {
		fullPrompt.WriteString("Selected code:\n```\n")
	} else if ctx.Content != "" {
		fullPrompt.WriteString("File content:\n```\n")
	}

	if ctx.Content != "" {
		fullPrompt.WriteString(ctx.Content)
		fullPrompt.WriteString("\n```\n\n")
	}

	fullPrompt.WriteString(prompt)

	// Start claude process with the prompt
	return p.startProcess(fullPrompt.String())
}

func (p *ClaudeProvider) SendWithHistory(ctx ai.EditorContext, prompt string, history []ai.ChatMessage) error {
	// Claude CLI doesn't support history in the same way as API
	// Just send the prompt
	return p.Send(ctx, prompt)
}

func (p *ClaudeProvider) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Send Ctrl+C equivalent or restart the process
	if p.cmd != nil && p.cmd.Process != nil {
		// Kill and restart
		p.cmd.Process.Kill()
		p.running = false
	}
}

func (p *ClaudeProvider) Responses() <-chan ai.AIResponse {
	return p.responses
}

// readOutput reads the output from claude CLI.
func (p *ClaudeProvider) readOutput() {
	// Read in chunks for streaming effect
	buf := make([]byte, 256)

	for {
		select {
		case <-p.cancel:
			return
		default:
		}

		n, err := p.stdout.Read(buf)
		if n > 0 {
			text := string(buf[:n])
			select {
			case p.responses <- ai.AIResponse{
				Kind: ai.ResponseKindText,
				Text: text,
			}:
			case <-p.cancel:
				return
			}
		}

		if err != nil {
			if err != io.EOF {
				select {
				case p.responses <- ai.AIResponse{
					Kind:  ai.ResponseKindError,
					Error: fmt.Errorf("read error: %w", err),
				}:
				case <-p.cancel:
				}
			}
			break
		}
	}

	// Read any stderr output
	stderrBuf := make([]byte, 4096)
	if n, _ := p.stderr.Read(stderrBuf); n > 0 {
		errText := strings.TrimSpace(string(stderrBuf[:n]))
		if errText != "" {
			select {
			case p.responses <- ai.AIResponse{
				Kind:  ai.ResponseKindError,
				Error: fmt.Errorf("claude: %s", errText),
			}:
			case <-p.cancel:
			}
			return
		}
	}

	// Signal done
	select {
	case p.responses <- ai.AIResponse{
		Kind: ai.ResponseKindDone,
	}:
	case <-p.cancel:
	}
}
