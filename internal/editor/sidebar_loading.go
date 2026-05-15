package editor

// SidebarLoadingContent shows a non-interactive loading placeholder.
type SidebarLoadingContent struct {
	title string
	label string
}

func NewSidebarLoadingContent(title, label string) *SidebarLoadingContent {
	return &SidebarLoadingContent{
		title: title,
		label: label,
	}
}

func (l *SidebarLoadingContent) Mode() SidebarMode {
	return SidebarModeBranches
}

func (l *SidebarLoadingContent) Title() string {
	return l.title
}

func (l *SidebarLoadingContent) Items() []SidebarItem {
	return []SidebarItem{
		{
			Label:     l.label,
			Available: false,
		},
	}
}

func (l *SidebarLoadingContent) Index() int {
	return 0
}

func (l *SidebarLoadingContent) SetIndex(i int) {
	// No-op: non-interactive placeholder.
}

func (l *SidebarLoadingContent) HandleKey(ev EventKey) (bool, SidebarActionData) {
	return false, SidebarActionData{Action: SidebarActionNone}
}

func (l *SidebarLoadingContent) OnEnter() SidebarActionData {
	return SidebarActionData{Action: SidebarActionNone}
}

func (l *SidebarLoadingContent) Available() bool {
	return true
}

func (l *SidebarLoadingContent) Refresh() error {
	return nil
}
