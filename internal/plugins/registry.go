package plugins

import "github.com/kobzarvs/qedit/internal/editor"

type Plugin interface {
	ID() string
	Register(*editor.Editor) error
}

type Registry struct {
	plugins []Plugin
}

func NewRegistry(plugins ...Plugin) *Registry {
	return &Registry{plugins: append([]Plugin(nil), plugins...)}
}

func (r *Registry) Register(plugin Plugin) {
	if r == nil || plugin == nil {
		return
	}
	r.plugins = append(r.plugins, plugin)
}

func (r *Registry) Apply(ed *editor.Editor) error {
	if r == nil {
		return nil
	}
	for _, plugin := range r.plugins {
		if err := plugin.Register(ed); err != nil {
			return err
		}
	}
	return nil
}
