package plugins

import "github.com/kobzarvs/qedit/internal/editor"

const ProfileSidebarMode editor.SidebarMode = 1000

type ProfileSidebarPlugin struct{}

func NewProfileSidebarPlugin() Plugin {
	return ProfileSidebarPlugin{}
}

func (ProfileSidebarPlugin) ID() string {
	return "profiles-sidebar"
}

func (ProfileSidebarPlugin) Register(ed *editor.Editor) error {
	ed.RegisterSidebarMode(editor.SidebarModeProvider{
		Mode:  ProfileSidebarMode,
		Label: "Profiles",
		Open: func(ed *editor.Editor) {
			if ed == nil {
				return
			}
			ed.ShowSidebarContent(newProfileSidebarContent(ed))
		},
	})
	ed.RegisterCommand(editor.CommandDefinition{
		Names: []string{"profiles"},
		Entries: []editor.CommandInfo{
			{Name: "profiles", Description: "open behavior profiles sidebar", Group: editor.CmdGroupView},
		},
		Handle: func(ed *editor.Editor, args []string) bool {
			ed.OpenSidebarMode(ProfileSidebarMode)
			return false
		},
	})
	return nil
}

type profileSidebarContent struct {
	editor   *editor.Editor
	profiles []string
	index    int
}

func newProfileSidebarContent(ed *editor.Editor) *profileSidebarContent {
	items := []string{
		editor.BehaviorProfileBasic,
		editor.BehaviorProfileHelix,
		editor.BehaviorProfileVim,
	}
	content := &profileSidebarContent{
		editor:   ed,
		profiles: items,
	}
	for i, profile := range items {
		if ed.BehaviorProfile() == profile {
			content.index = i
			break
		}
	}
	return content
}

func (c *profileSidebarContent) Mode() editor.SidebarMode { return ProfileSidebarMode }

func (c *profileSidebarContent) Title() string { return "Profiles" }

func (c *profileSidebarContent) Items() []editor.SidebarItem {
	items := make([]editor.SidebarItem, 0, len(c.profiles))
	for _, profile := range c.profiles {
		items = append(items, editor.SidebarItem{
			Label:     profile,
			Available: true,
			IsCurrent: c.editor.BehaviorProfile() == profile,
		})
	}
	return items
}

func (c *profileSidebarContent) Index() int { return c.index }

func (c *profileSidebarContent) SetIndex(i int) {
	if i >= 0 && i < len(c.profiles) {
		c.index = i
	}
}

func (c *profileSidebarContent) HandleKey(ev editor.EventKey) (bool, editor.SidebarActionData) {
	return false, editor.SidebarActionData{Action: editor.SidebarActionNone}
}

func (c *profileSidebarContent) OnEnter() editor.SidebarActionData {
	if c.index < 0 || c.index >= len(c.profiles) {
		return editor.SidebarActionData{Action: editor.SidebarActionNone}
	}
	next := c.profiles[c.index]
	if !c.editor.SetBehaviorProfile(next) {
		return editor.SidebarActionData{Action: editor.SidebarActionNone}
	}
	c.editor.SetStatusMessage("profile=" + c.editor.BehaviorProfile())
	return editor.SidebarActionData{Action: editor.SidebarActionClose}
}

func (c *profileSidebarContent) Available() bool { return true }

func (c *profileSidebarContent) Refresh() error { return nil }
