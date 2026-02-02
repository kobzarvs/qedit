package integrations

import (
	"sync"

	"github.com/kobzarvs/qedit/internal/ai"
	"github.com/kobzarvs/qedit/internal/editor"
)

// AIManagerAdapter adapts ai.Manager to editor.AIManager interface.
type AIManagerAdapter struct {
	mgr        *ai.Manager
	events     chan editor.AIEvent
	eventsOnce sync.Once
	closeOnce  sync.Once
}

// NewAIManager creates a new AI manager adapter.
func NewAIManager(mgr *ai.Manager) *AIManagerAdapter {
	if mgr == nil {
		return nil
	}
	return &AIManagerAdapter{
		mgr:    mgr,
		events: make(chan editor.AIEvent, 100),
	}
}

func (a *AIManagerAdapter) ActiveName() string {
	if a == nil || a.mgr == nil || a.mgr.Active() == nil {
		return ""
	}
	return a.mgr.Active().Name()
}

func (a *AIManagerAdapter) SetActive(name string) error {
	if a == nil || a.mgr == nil {
		return nil
	}
	return a.mgr.SetActive(name)
}

func (a *AIManagerAdapter) ListProviders() []editor.AIProviderInfo {
	if a == nil || a.mgr == nil {
		return nil
	}
	providers := a.mgr.List()
	result := make([]editor.AIProviderInfo, len(providers))
	for i, p := range providers {
		status := int(p.Status())
		available := status != int(ai.StatusOffline)
		result[i] = editor.AIProviderInfo{
			Name:        p.Name(),
			DisplayName: p.DisplayName(),
			Available:   available,
			Status:      status,
			ModelName:   p.CurrentModel(),
		}
	}
	return result
}

func (a *AIManagerAdapter) ListAvailableProviders() []editor.AIProviderInfo {
	if a == nil || a.mgr == nil {
		return nil
	}
	providers := a.mgr.List()
	result := make([]editor.AIProviderInfo, 0, len(providers))
	for _, p := range providers {
		status := int(p.Status())
		if status == int(ai.StatusOffline) {
			continue
		}
		result = append(result, editor.AIProviderInfo{
			Name:        p.Name(),
			DisplayName: p.DisplayName(),
			Available:   true,
			Status:      status,
			ModelName:   p.CurrentModel(),
		})
	}
	return result
}

func (a *AIManagerAdapter) ListModels() ([]editor.AIModelInfo, error) {
	if a == nil || a.mgr == nil || a.mgr.Active() == nil {
		return nil, nil
	}
	models, err := a.mgr.Active().ListModels()
	if err != nil {
		return nil, err
	}
	result := make([]editor.AIModelInfo, len(models))
	for i, m := range models {
		result[i] = editor.AIModelInfo{
			ID:   m.ID,
			Name: m.Name,
		}
	}
	return result, nil
}

func (a *AIManagerAdapter) CurrentModel() string {
	if a == nil || a.mgr == nil || a.mgr.Active() == nil {
		return ""
	}
	return a.mgr.Active().CurrentModel()
}

func (a *AIManagerAdapter) SetModel(model string) error {
	if a == nil || a.mgr == nil || a.mgr.Active() == nil {
		return nil
	}
	return a.mgr.Active().SetModel(model)
}

func (a *AIManagerAdapter) Send(ctx editor.AIContext, prompt string) error {
	if a == nil || a.mgr == nil {
		return nil
	}
	aiCtx := ai.EditorContext{
		FilePath:       ctx.FilePath,
		Content:        ctx.Content,
		IsSelection:    ctx.IsSelection,
		CursorRow:      ctx.CursorRow,
		CursorCol:      ctx.CursorCol,
		Language:       ctx.Language,
		ReasoningLevel: ctx.ReasoningLevel,
	}
	return a.mgr.Send(aiCtx, prompt)
}

func (a *AIManagerAdapter) Cancel() {
	if a == nil || a.mgr == nil || a.mgr.Active() == nil {
		return
	}
	a.mgr.Active().Cancel()
}

func (a *AIManagerAdapter) Events() <-chan editor.AIEvent {
	if a == nil || a.mgr == nil {
		ch := make(chan editor.AIEvent)
		close(ch)
		return ch
	}

	// Start the forwarding goroutine only once
	a.eventsOnce.Do(func() {
		go func() {
			defer a.closeOnce.Do(func() {
				close(a.events)
			})
			for resp := range a.mgr.Events() {
				kind := string(resp.Kind)
				select {
				case a.events <- editor.AIEvent{
					Kind:  kind,
					Text:  resp.Text,
					Error: resp.Error,
				}:
				default:
					// Channel full, drop event to prevent blocking
				}
			}
		}()
	})

	return a.events
}

func (a *AIManagerAdapter) Start() error {
	// Nothing to start - providers start on demand
	return nil
}

func (a *AIManagerAdapter) Stop() {
	if a == nil || a.mgr == nil {
		return
	}
	a.mgr.Stop()
}
