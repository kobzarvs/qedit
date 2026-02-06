package editor

import (
	"path/filepath"
)

// SidebarBuffersContent implements SidebarContent for the open buffers list.
type SidebarBuffersContent struct {
	editor *Editor
	index  int
	items  []SidebarItem
}

// NewSidebarBuffersContent creates a new buffers sidebar content.
func NewSidebarBuffersContent(ed *Editor) *SidebarBuffersContent {
	c := &SidebarBuffersContent{editor: ed}
	c.buildItems()
	return c
}

func (c *SidebarBuffersContent) buildItems() {
	if c.editor.buffers == nil {
		c.items = nil
		return
	}
	infos := c.editor.buffers.Items()
	c.items = make([]SidebarItem, len(infos))
	for i, info := range infos {
		label := info.Filename
		if label == "" {
			label = "[No Name]"
		} else {
			// Show relative path from cwd if possible
			if cwd, err := filepath.Abs("."); err == nil {
				if rel, err := filepath.Rel(cwd, info.Filename); err == nil && len(rel) < len(info.Filename) {
					label = rel
				}
			}
		}
		var icon rune
		if info.Dirty {
			icon = '*'
		}
		c.items[i] = SidebarItem{
			Label:     label,
			IsCurrent: info.Active,
			Icon:      icon,
			Available: true,
		}
	}
}

func (c *SidebarBuffersContent) Mode() SidebarMode {
	return SidebarModeBuffers
}

func (c *SidebarBuffersContent) Title() string {
	return "Open Buffers"
}

func (c *SidebarBuffersContent) Items() []SidebarItem {
	c.buildItems()
	return c.items
}

func (c *SidebarBuffersContent) Index() int {
	return c.index
}

func (c *SidebarBuffersContent) SetIndex(i int) {
	if i >= 0 && i < len(c.items) {
		c.index = i
	}
}

func (c *SidebarBuffersContent) HandleKey(ev EventKey) (bool, SidebarActionData) {
	r := ev.Rune()
	switch {
	case r == 'd':
		// Close buffer (with dirty check)
		if c.index < 0 || c.index >= len(c.items) {
			return true, SidebarActionData{Action: SidebarActionNone}
		}
		infos := c.editor.buffers.Items()
		if c.index >= len(infos) {
			return true, SidebarActionData{Action: SidebarActionNone}
		}
		if infos[c.index].Dirty {
			c.editor.setStatus("buffer has unsaved changes (use D to force close)")
			return true, SidebarActionData{Action: SidebarActionNone}
		}
		c.editor.closeBufferAtIndex(c.index)
		if c.index >= len(c.editor.buffers.Items()) && c.index > 0 {
			c.index--
		}
		c.buildItems()
		return true, SidebarActionData{Action: SidebarActionNone}

	case r == 'D':
		// Force close buffer
		if c.index < 0 || c.index >= len(c.items) {
			return true, SidebarActionData{Action: SidebarActionNone}
		}
		c.editor.closeBufferAtIndex(c.index)
		if c.index >= len(c.editor.buffers.Items()) && c.index > 0 {
			c.index--
		}
		c.buildItems()
		return true, SidebarActionData{Action: SidebarActionNone}
	}
	return false, SidebarActionData{Action: SidebarActionNone}
}

func (c *SidebarBuffersContent) OnEnter() SidebarActionData {
	if c.index < 0 || c.index >= len(c.items) {
		return SidebarActionData{Action: SidebarActionNone}
	}
	return SidebarActionData{
		Action:      SidebarActionSwitchBuffer,
		BufferIndex: c.index,
	}
}

func (c *SidebarBuffersContent) Available() bool {
	return c.editor.buffers != nil && c.editor.buffers.Count() > 0
}

func (c *SidebarBuffersContent) Refresh() error {
	c.buildItems()
	return nil
}
