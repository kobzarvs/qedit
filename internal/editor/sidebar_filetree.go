package editor

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SidebarFileTreeContent implements SidebarContent for file tree navigation.
type SidebarFileTreeContent struct {
	dir            string
	projectRoot    string
	showHidden     bool
	showIgnored    bool
	gitRoot        string
	ignorePatterns []gitignorePattern
	previewMode    bool
	items          []SidebarItem
	index          int
}

func NewSidebarFileTreeContent(dir string, showHidden, showIgnored bool) *SidebarFileTreeContent {
	c := &SidebarFileTreeContent{
		dir:         dir,
		showHidden:  showHidden,
		showIgnored: showIgnored,
		index:       0,
	}
	c.gitRoot = findGitRoot(dir)
	if c.gitRoot != "" {
		c.projectRoot = c.gitRoot
		c.ignorePatterns = loadGitignore(c.gitRoot)
	} else if cwd, err := os.Getwd(); err == nil {
		c.projectRoot = cwd
	} else {
		c.projectRoot = dir
	}
	_ = c.loadDir(dir)
	return c
}

func (c *SidebarFileTreeContent) Mode() SidebarMode {
	return SidebarModeFileTree
}

func (c *SidebarFileTreeContent) Title() string {
	return c.dir
}

func (c *SidebarFileTreeContent) Items() []SidebarItem {
	return c.items
}

func (c *SidebarFileTreeContent) Index() int {
	return c.index
}

func (c *SidebarFileTreeContent) SetIndex(i int) {
	if i >= 0 && i < len(c.items) {
		c.index = i
	}
}

func (c *SidebarFileTreeContent) HandleKey(ev EventKey) (bool, SidebarActionData) {
	key := ev.Key()
	r := ev.Rune()
	switch {
	case key == KeyLeft || key == KeyBackspace || key == KeyBackspace2:
		c.goUp()
		return true, SidebarActionData{Action: SidebarActionNone}
	case key == KeyRight || r == 'l':
		return true, c.enter()
	case r == 'a' || r == '.':
		c.toggleHidden()
		return true, SidebarActionData{Action: SidebarActionNone}
	case r == 'h':
		c.toggleIgnored()
		return true, SidebarActionData{Action: SidebarActionNone}
	case r == 'v':
		c.togglePreview()
		return true, SidebarActionData{Action: SidebarActionNone}
	case key == KeyHome && ev.Modifiers()&ModMeta != 0:
		c.goToProjectRoot()
		return true, SidebarActionData{Action: SidebarActionNone}
	case key == KeyEscape:
		return true, SidebarActionData{Action: SidebarActionBackToMenu}
	}
	return false, SidebarActionData{Action: SidebarActionNone}
}

func (c *SidebarFileTreeContent) OnEnter() SidebarActionData {
	return c.enter()
}

func (c *SidebarFileTreeContent) Available() bool {
	return true
}

func (c *SidebarFileTreeContent) Refresh() error {
	return c.loadDir(c.dir)
}

func (c *SidebarFileTreeContent) PreviewEnabled() bool {
	return c.previewMode
}

func (c *SidebarFileTreeContent) ShowHidden() bool {
	return c.showHidden
}

func (c *SidebarFileTreeContent) ShowIgnored() bool {
	return c.showIgnored
}

func (c *SidebarFileTreeContent) SelectedPath() (string, bool) {
	item, ok := c.currentItem()
	if !ok {
		return "", false
	}
	return item.Path, item.IsDir
}

func (c *SidebarFileTreeContent) togglePreview() {
	c.previewMode = !c.previewMode
}

func (c *SidebarFileTreeContent) toggleHidden() {
	c.showHidden = !c.showHidden
	c.reloadPreserveSelection(c.currentPath())
}

func (c *SidebarFileTreeContent) toggleIgnored() {
	c.showIgnored = !c.showIgnored
	c.reloadPreserveSelection(c.currentPath())
}

func (c *SidebarFileTreeContent) enter() SidebarActionData {
	item, ok := c.currentItem()
	if !ok {
		return SidebarActionData{Action: SidebarActionNone}
	}
	if item.IsDir {
		if item.Label == ".." {
			c.goUp()
			return SidebarActionData{Action: SidebarActionNone}
		}
		_ = c.loadDir(item.Path)
		c.index = 0
		return SidebarActionData{Action: SidebarActionNone}
	}
	return SidebarActionData{Action: SidebarActionOpenFile, Path: item.Path}
}

func (c *SidebarFileTreeContent) goUp() {
	if c.dir == "" {
		return
	}
	parent := filepath.Dir(c.dir)
	if parent == c.dir {
		return
	}
	prev := c.dir
	_ = c.loadDir(parent)
	c.selectByPath(prev)
}

func (c *SidebarFileTreeContent) goToProjectRoot() {
	if c.projectRoot == "" {
		return
	}
	_ = c.loadDir(c.projectRoot)
	c.index = 0
}

func (c *SidebarFileTreeContent) loadDir(dir string) error {
	if dir == "" {
		return nil
	}
	absDir, err := filepath.Abs(dir)
	if err == nil {
		dir = absDir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.items = []SidebarItem{{Label: "Error: " + err.Error(), Available: false}}
		c.index = 0
		return err
	}
	c.dir = dir

	var parentItems []SidebarItem
	if parent := filepath.Dir(dir); parent != dir {
		parentItems = append(parentItems, SidebarItem{
			Label:     "..",
			Path:      parent,
			IsDir:     true,
			Available: true,
		})
	}

	var dirs []SidebarItem
	var files []SidebarItem
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" {
			continue
		}
		isDir := entry.IsDir()
		fullPath := filepath.Join(dir, name)
		isHidden := strings.HasPrefix(name, ".")
		isIgnored := false
		if c.gitRoot != "" {
			if rel, err := filepath.Rel(c.gitRoot, fullPath); err == nil {
				isIgnored = matchesGitignore(c.ignorePatterns, rel, isDir)
			}
		}
		if isHidden && !c.showHidden {
			continue
		}
		if isIgnored && !c.showIgnored {
			continue
		}
		item := SidebarItem{
			Label:     name,
			Path:      fullPath,
			IsDir:     isDir,
			IsHidden:  isHidden,
			IsIgnored: isIgnored,
			Available: true,
		}
		if isDir {
			dirs = append(dirs, item)
		} else {
			files = append(files, item)
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Label) < strings.ToLower(dirs[j].Label)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Label) < strings.ToLower(files[j].Label)
	})

	c.items = append(parentItems, dirs...)
	c.items = append(c.items, files...)
	if c.index >= len(c.items) {
		if len(c.items) > 0 {
			c.index = len(c.items) - 1
		} else {
			c.index = 0
		}
	}
	return nil
}

func (c *SidebarFileTreeContent) reloadPreserveSelection(path string) {
	_ = c.loadDir(c.dir)
	c.selectByPath(path)
}

func (c *SidebarFileTreeContent) selectByPath(path string) {
	if path == "" {
		return
	}
	for i, item := range c.items {
		if item.Path == path {
			c.index = i
			return
		}
	}
	c.index = 0
}

func (c *SidebarFileTreeContent) currentItem() (SidebarItem, bool) {
	if c.index < 0 || c.index >= len(c.items) {
		return SidebarItem{}, false
	}
	return c.items[c.index], true
}

func (c *SidebarFileTreeContent) currentPath() string {
	item, ok := c.currentItem()
	if !ok {
		return ""
	}
	return item.Path
}
