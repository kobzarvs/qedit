package editor

import "errors"

type FormatterProvider struct {
	Name     string
	Supports func(path, content string) bool
	Format   func(*Editor, string, string) error
}

type formatterRegistry struct {
	providers []FormatterProvider
}

func newFormatterRegistry() formatterRegistry {
	return formatterRegistry{}
}

func (r *formatterRegistry) Register(provider FormatterProvider) {
	r.providers = append(r.providers, provider)
}

func (r *formatterRegistry) Format(e *Editor, path, content string) error {
	for i := len(r.providers) - 1; i >= 0; i-- {
		provider := r.providers[i]
		if provider.Supports == nil || provider.Format == nil {
			continue
		}
		if !provider.Supports(path, content) {
			continue
		}
		return provider.Format(e, path, content)
	}
	return errors.New("format not supported")
}

func (e *Editor) RegisterFormatter(provider FormatterProvider) {
	e.formatters.Register(provider)
}

func (e *Editor) registerBuiltInFormatters() {
	e.RegisterFormatter(FormatterProvider{
		Name: "markdown-tables",
		Supports: func(path, content string) bool {
			return isMarkdownFile(path)
		},
		Format: func(ed *Editor, path, content string) error {
			if err := ed.FormatMarkdownTables(); err != nil {
				return err
			}
			ed.setStatus("formatted")
			return nil
		},
	})
	e.RegisterFormatter(FormatterProvider{
		Name: "go",
		Supports: func(path, content string) bool {
			return isGoFile(path) || (path == "" && looksLikeGo(content))
		},
		Format: func(ed *Editor, path, content string) error {
			ed.enqueueRuntimeRequest(RuntimeRequest{
				Kind:    RuntimeRequestFormatBuffer,
				Path:    path,
				Content: content,
			})
			return nil
		},
	})
	e.RegisterFormatter(FormatterProvider{
		Name: "javascript",
		Supports: func(path, content string) bool {
			return isPrettierFile(path)
		},
		Format: func(ed *Editor, path, content string) error {
			ed.enqueueRuntimeRequest(RuntimeRequest{
				Kind:    RuntimeRequestFormatBuffer,
				Path:    path,
				Content: content,
			})
			return nil
		},
	})
}
