package editor

// SidebarAIProvidersContent shows available AI providers.
type SidebarAIProvidersContent struct {
	providers []AIProviderInfo
	current   string
	index     int
}

func NewSidebarAIProvidersContent(providers []AIProviderInfo, current string) *SidebarAIProvidersContent {
	c := &SidebarAIProvidersContent{
		providers: providers,
		current:   current,
		index:     0,
	}
	for i, p := range providers {
		if p.Name == current {
			c.index = i
			break
		}
	}
	return c
}

func (c *SidebarAIProvidersContent) Mode() SidebarMode {
	return SidebarModeAI
}

func (c *SidebarAIProvidersContent) Title() string {
	return "AI Providers"
}

func (c *SidebarAIProvidersContent) Items() []SidebarItem {
	if len(c.providers) == 0 {
		return []SidebarItem{{Label: "No AI providers", Available: false}}
	}

	items := make([]SidebarItem, len(c.providers))
	for i, p := range c.providers {
		label := p.DisplayName
		if label == "" {
			label = p.Name
		}
		items[i] = SidebarItem{
			Label:      label,
			Available:  p.Available,
			Icon:       '⏺',
			ShowStatus: true,
			Status:     AIProviderStatus(p.Status),
		}
	}
	return items
}

func (c *SidebarAIProvidersContent) Index() int {
	return c.index
}

func (c *SidebarAIProvidersContent) SetIndex(i int) {
	if i >= 0 && i < len(c.providers) {
		c.index = i
	}
}

func (c *SidebarAIProvidersContent) HandleKey(ev EventKey) (bool, SidebarActionData) {
	return false, SidebarActionData{Action: SidebarActionNone}
}

func (c *SidebarAIProvidersContent) OnEnter() SidebarActionData {
	if c.index < 0 || c.index >= len(c.providers) {
		return SidebarActionData{Action: SidebarActionNone}
	}
	provider := c.providers[c.index]
	if provider.Name == "" {
		return SidebarActionData{Action: SidebarActionNone}
	}
	return SidebarActionData{
		Action:   SidebarActionOpenAIModels,
		Provider: provider.Name,
	}
}

func (c *SidebarAIProvidersContent) Available() bool {
	return true
}

func (c *SidebarAIProvidersContent) Refresh() error {
	return nil
}

// SidebarAIModelsContent shows models for a provider.
type SidebarAIModelsContent struct {
	providerName  string
	providerLabel string
	models        []AIModelInfo
	currentModel  string
	index         int
}

func NewSidebarAIModelsContent(providerName string, providerLabel string, models []AIModelInfo, currentModel string) *SidebarAIModelsContent {
	c := &SidebarAIModelsContent{
		providerName:  providerName,
		providerLabel: providerLabel,
		models:        models,
		currentModel:  currentModel,
		index:         0,
	}
	for i, m := range models {
		if m.ID == currentModel {
			c.index = i
			break
		}
	}
	return c
}

func (c *SidebarAIModelsContent) Mode() SidebarMode {
	return SidebarModeAI
}

func (c *SidebarAIModelsContent) Title() string {
	label := c.providerLabel
	if label == "" {
		label = c.providerName
	}
	if label == "" {
		return "AI Models"
	}
	return "AI: " + label
}

func (c *SidebarAIModelsContent) Items() []SidebarItem {
	if len(c.models) == 0 {
		return []SidebarItem{{Label: "No AI models", Available: false}}
	}

	items := make([]SidebarItem, len(c.models))
	for i, m := range c.models {
		label := m.Name
		if label == "" {
			label = m.ID
		}
		items[i] = SidebarItem{
			Label:     label,
			Available: true,
			IsCurrent: m.ID == c.currentModel,
		}
	}
	return items
}

func (c *SidebarAIModelsContent) Index() int {
	return c.index
}

func (c *SidebarAIModelsContent) SetIndex(i int) {
	if i >= 0 && i < len(c.models) {
		c.index = i
	}
}

func (c *SidebarAIModelsContent) HandleKey(ev EventKey) (bool, SidebarActionData) {
	key := ev.Key()
	r := ev.Rune()
	if key == KeyLeft || r == 'h' || key == KeyBackspace {
		return true, SidebarActionData{Action: SidebarActionSwitchMode, Mode: SidebarModeAI}
	}
	return false, SidebarActionData{Action: SidebarActionNone}
}

func (c *SidebarAIModelsContent) OnEnter() SidebarActionData {
	if c.index < 0 || c.index >= len(c.models) {
		return SidebarActionData{Action: SidebarActionNone}
	}
	model := c.models[c.index]
	if model.ID == "" {
		return SidebarActionData{Action: SidebarActionNone}
	}
	return SidebarActionData{
		Action: SidebarActionSetAIModel,
		Model:  model.ID,
	}
}

func (c *SidebarAIModelsContent) Available() bool {
	return true
}

func (c *SidebarAIModelsContent) Refresh() error {
	return nil
}

func (c *SidebarAIModelsContent) SetCurrentModel(model string) {
	c.currentModel = model
	for i, m := range c.models {
		if m.ID == model {
			c.index = i
			break
		}
	}
}
