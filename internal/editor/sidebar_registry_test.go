package editor

import "testing"

type testSidebarContent struct {
	mode SidebarMode
}

func (c testSidebarContent) Mode() SidebarMode    { return c.mode }
func (c testSidebarContent) Title() string        { return "Test" }
func (c testSidebarContent) Items() []SidebarItem { return nil }
func (c testSidebarContent) Index() int           { return 0 }
func (c testSidebarContent) SetIndex(i int)       {}
func (c testSidebarContent) HandleKey(ev EventKey) (bool, SidebarActionData) {
	return false, SidebarActionData{}
}
func (c testSidebarContent) OnEnter() SidebarActionData { return SidebarActionData{} }
func (c testSidebarContent) Available() bool            { return true }
func (c testSidebarContent) Refresh() error             { return nil }

func TestSidebarMenuUsesRegistryAvailability(t *testing.T) {
	e := newTestEditor("hello")

	e.openSidebar()

	menu, ok := e.sidebar.Content.(*SidebarMenuContent)
	if !ok {
		t.Fatalf("sidebar content = %T, want *SidebarMenuContent", e.sidebar.Content)
	}

	var branches SidebarMenuItem
	found := false
	for _, item := range menu.items {
		if item.Mode == SidebarModeBranches {
			branches = item
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("branches item not found")
	}
	if branches.Available {
		t.Fatalf("branches item available = true, want false without git repo")
	}

	e.SetGitBranch("main")
	e.openSidebar()

	menu, ok = e.sidebar.Content.(*SidebarMenuContent)
	if !ok {
		t.Fatalf("sidebar content = %T, want *SidebarMenuContent", e.sidebar.Content)
	}
	for _, item := range menu.items {
		if item.Mode == SidebarModeBranches {
			if !item.Available {
				t.Fatalf("branches item available = false, want true with git repo")
			}
			return
		}
	}
	t.Fatalf("branches item not found after git state update")
}

func TestRegisterSidebarModeAddsMenuItemAndDispatch(t *testing.T) {
	e := newTestEditor("hello")
	customMode := SidebarMode(1000)
	opened := false

	e.RegisterSidebarMode(SidebarModeProvider{
		Mode:   customMode,
		Label:  "Custom View",
		Hotkey: "Cmd+K",
		Open: func(ed *Editor) {
			opened = true
			ed.sidebar.Open(testSidebarContent{mode: customMode})
		},
	})

	e.openSidebar()

	menu, ok := e.sidebar.Content.(*SidebarMenuContent)
	if !ok {
		t.Fatalf("sidebar content = %T, want *SidebarMenuContent", e.sidebar.Content)
	}

	found := false
	for _, item := range menu.items {
		if item.Mode == customMode {
			found = true
			if item.Label != "Custom View" {
				t.Fatalf("label = %q, want %q", item.Label, "Custom View")
			}
			if item.Hotkey != "Cmd+K" {
				t.Fatalf("hotkey = %q, want %q", item.Hotkey, "Cmd+K")
			}
		}
	}
	if !found {
		t.Fatalf("custom sidebar mode not found in menu")
	}

	e.switchSidebarMode(customMode)

	if !opened {
		t.Fatalf("custom mode opener was not called")
	}
	if e.sidebar.Content.Mode() != customMode {
		t.Fatalf("sidebar mode = %v, want %v", e.sidebar.Content.Mode(), customMode)
	}
}
