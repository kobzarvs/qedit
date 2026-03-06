package editor

type SidebarModeProvider struct {
	Mode      SidebarMode
	Label     string
	Hotkey    string
	Available func(*Editor) bool
	Open      func(*Editor)
	Refresh   func(*Editor)
}

type sidebarModeRegistry struct {
	order     []SidebarMode
	providers map[SidebarMode]SidebarModeProvider
}

func newSidebarModeRegistry() sidebarModeRegistry {
	return sidebarModeRegistry{
		providers: make(map[SidebarMode]SidebarModeProvider),
	}
}

func (r *sidebarModeRegistry) Register(provider SidebarModeProvider) {
	if r.providers == nil {
		r.providers = make(map[SidebarMode]SidebarModeProvider)
	}
	if _, exists := r.providers[provider.Mode]; !exists {
		r.order = append(r.order, provider.Mode)
	}
	r.providers[provider.Mode] = provider
}

func (r *sidebarModeRegistry) Provider(mode SidebarMode) (SidebarModeProvider, bool) {
	provider, ok := r.providers[mode]
	return provider, ok
}

func (r *sidebarModeRegistry) MenuItems(e *Editor) []SidebarMenuItem {
	items := make([]SidebarMenuItem, 0, len(r.order))
	for _, mode := range r.order {
		provider := r.providers[mode]
		if provider.Label == "" {
			continue
		}
		available := true
		if provider.Available != nil {
			available = provider.Available(e)
		}
		items = append(items, SidebarMenuItem{
			Label:     provider.Label,
			Mode:      provider.Mode,
			Hotkey:    provider.Hotkey,
			Available: available,
		})
	}
	return items
}

func (e *Editor) RegisterSidebarMode(provider SidebarModeProvider) {
	e.sidebarModes.Register(provider)
}

func (e *Editor) OpenSidebarMode(mode SidebarMode) {
	e.switchSidebarMode(mode)
}

func (e *Editor) registerBuiltInSidebarModes() {
	e.RegisterSidebarMode(SidebarModeProvider{
		Mode: SidebarModeMenu,
		Open: func(ed *Editor) {
			ed.showSidebarMenu()
		},
	})
	e.RegisterSidebarMode(SidebarModeProvider{
		Mode:   SidebarModeFileTree,
		Label:  "Project files",
		Hotkey: "Cmd+O",
		Open: func(ed *Editor) {
			ed.openSidebarFileTree("")
		},
	})
	e.RegisterSidebarMode(SidebarModeProvider{
		Mode:  SidebarModeBuffers,
		Label: "Open Buffers",
		Open: func(ed *Editor) {
			ed.openSidebarBuffers()
		},
	})
	e.RegisterSidebarMode(SidebarModeProvider{
		Mode:   SidebarModeBranches,
		Label:  "Branches",
		Hotkey: "Cmd+B",
		Available: func(ed *Editor) bool {
			return ed.isGitRepo()
		},
		Open: func(ed *Editor) {
			ed.openSidebarBranches()
		},
	})
	e.RegisterSidebarMode(SidebarModeProvider{
		Mode:  SidebarModeRecentHistory,
		Label: "Recent History",
		Available: func(*Editor) bool {
			return false
		},
		Open: func(ed *Editor) {
			ed.setStatus("Recent History: not implemented yet")
		},
	})
	e.RegisterSidebarMode(SidebarModeProvider{
		Mode:  SidebarModeLocalChanges,
		Label: "Git Changes",
		Available: func(ed *Editor) bool {
			return ed.isGitRepo()
		},
		Open: func(ed *Editor) {
			ed.openSidebarGitChanges()
		},
	})
	e.RegisterSidebarMode(SidebarModeProvider{
		Mode:   SidebarModeWorktrees,
		Label:  "Worktree",
		Hotkey: "Cmd+Shift+W",
		Available: func(ed *Editor) bool {
			return ed.isGitRepo()
		},
		Open: func(ed *Editor) {
			ed.openSidebarWorktrees()
		},
		Refresh: func(ed *Editor) {
			ed.refreshSidebarWorktrees()
		},
	})
}

func (e *Editor) refreshSidebarMenu() {
	if e.sidebar == nil {
		return
	}
	items := e.sidebarModes.MenuItems(e)
	if e.sidebar.MenuContent == nil {
		e.sidebar.MenuContent = NewSidebarMenuContent(items)
		return
	}
	e.sidebar.MenuContent.SetItems(items)
}

func (e *Editor) showSidebarMenu() {
	if e.sidebar == nil {
		return
	}
	e.refreshSidebarMenu()
	if e.sidebar.MenuContent != nil {
		e.sidebar.Open(e.sidebar.MenuContent)
	}
}

func (e *Editor) openSidebarMode(mode SidebarMode) {
	provider, ok := e.sidebarModes.Provider(mode)
	if !ok || provider.Open == nil {
		return
	}
	provider.Open(e)
}

func (e *Editor) refreshSidebarMode(mode SidebarMode) {
	provider, ok := e.sidebarModes.Provider(mode)
	if !ok {
		return
	}
	if provider.Refresh != nil {
		provider.Refresh(e)
		return
	}
	if provider.Open != nil {
		provider.Open(e)
	}
}
