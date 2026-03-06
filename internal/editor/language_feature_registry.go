package editor

type LanguageFeatureProvider struct {
	Name       string
	Available  func(*Editor) bool
	Supports   func(path string) bool
	NodeStack  func(*Editor, string, int, int) []NodeRange
	Goto       func(*Editor, string, string, int, int) ([]LSPLocation, error)
	Highlights func(*Editor, string, int, int) map[int][]HighlightSpan
}

type languageFeatureRegistry struct {
	providers []LanguageFeatureProvider
}

func newLanguageFeatureRegistry() languageFeatureRegistry {
	return languageFeatureRegistry{}
}

func (r *languageFeatureRegistry) Register(provider LanguageFeatureProvider) {
	r.providers = append(r.providers, provider)
}

func (r *languageFeatureRegistry) availableProvider(e *Editor, path string, want func(LanguageFeatureProvider) bool) (LanguageFeatureProvider, bool) {
	for i := len(r.providers) - 1; i >= 0; i-- {
		provider := r.providers[i]
		if provider.Available != nil && !provider.Available(e) {
			continue
		}
		if provider.Supports != nil && !provider.Supports(path) {
			continue
		}
		if want(provider) {
			return provider, true
		}
	}
	return LanguageFeatureProvider{}, false
}

func (r *languageFeatureRegistry) HasAvailable(e *Editor) bool {
	_, ok := r.availableProvider(e, "", func(LanguageFeatureProvider) bool { return true })
	return ok
}

func (r *languageFeatureRegistry) NodeStack(e *Editor, path string, row, col int) []NodeRange {
	provider, ok := r.availableProvider(e, path, func(p LanguageFeatureProvider) bool { return p.NodeStack != nil })
	if !ok {
		return nil
	}
	return provider.NodeStack(e, path, row, col)
}

func (r *languageFeatureRegistry) Goto(e *Editor, method, path string, line, col int) ([]LSPLocation, error) {
	provider, ok := r.availableProvider(e, path, func(p LanguageFeatureProvider) bool { return p.Goto != nil })
	if !ok {
		return nil, nil
	}
	return provider.Goto(e, method, path, line, col)
}

func (r *languageFeatureRegistry) HighlightRange(e *Editor, path string, startLine, endLine int) map[int][]HighlightSpan {
	provider, ok := r.availableProvider(e, path, func(p LanguageFeatureProvider) bool { return p.Highlights != nil })
	if !ok {
		return nil
	}
	return provider.Highlights(e, path, startLine, endLine)
}

func (e *Editor) RegisterLanguageFeature(provider LanguageFeatureProvider) {
	e.languageFeatures.Register(provider)
}

func (e *Editor) registerBuiltInLanguageFeatures() {
	e.RegisterLanguageFeature(LanguageFeatureProvider{
		Name: "runtime-language",
		Available: func(ed *Editor) bool {
			return ed.runtime.languageRuntime != nil
		},
		NodeStack: func(ed *Editor, path string, row, col int) []NodeRange {
			return ed.runtime.languageRuntime.NodeStack(path, row, col)
		},
		Goto: func(ed *Editor, method, path string, line, col int) ([]LSPLocation, error) {
			return ed.runtime.languageRuntime.Goto(method, path, line, col)
		},
		Highlights: func(ed *Editor, path string, startLine, endLine int) map[int][]HighlightSpan {
			return ed.runtime.languageRuntime.HighlightRange(path, startLine, endLine)
		},
	})
}
