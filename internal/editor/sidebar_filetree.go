package editor

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
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
	searchQuery    []rune
	searchPrevPath string
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
		if absRoot, err := filepath.Abs(c.gitRoot); err == nil {
			c.gitRoot = absRoot
		}
		c.projectRoot = c.gitRoot
		c.ignorePatterns = loadGitignore(c.gitRoot)
	} else if cwd, err := os.Getwd(); err == nil {
		c.projectRoot = cwd
	} else {
		c.projectRoot = dir
	}
	if c.projectRoot != "" {
		if absRoot, err := filepath.Abs(c.projectRoot); err == nil {
			c.projectRoot = absRoot
		}
	}
	_ = c.loadDir(dir)
	return c
}

func (c *SidebarFileTreeContent) Mode() SidebarMode {
	return SidebarModeFileTree
}

func (c *SidebarFileTreeContent) Title() string {
	if c.searchActive() {
		return "Find: " + string(c.searchQuery)
	}
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
	if c.searchActive() {
		switch {
		case key == KeyEscape:
			c.clearSearch()
			return true, SidebarActionData{Action: SidebarActionNone}
		case key == KeyBackspace || key == KeyBackspace2 || key == KeyLeft:
			c.deleteSearchRune()
			return true, SidebarActionData{Action: SidebarActionNone}
		case key == KeyRight:
			return true, c.enter()
		case key == KeyRune && ev.Modifiers() == 0 && unicode.IsPrint(r):
			c.appendSearchRune(r)
			return true, SidebarActionData{Action: SidebarActionNone}
		}
	}
	switch {
	case key == KeyLeft || key == KeyBackspace || key == KeyBackspace2:
		c.goUp()
		return true, SidebarActionData{Action: SidebarActionNone}
	case key == KeyRight:
		return true, c.enter()
	case key == KeyF2:
		c.cycleFilterMode()
		return true, SidebarActionData{Action: SidebarActionNone}
	case key == KeyF3:
		c.togglePreview()
		return true, SidebarActionData{Action: SidebarActionNone}
	case key == KeyHome && ev.Modifiers()&ModMeta != 0:
		c.goToProjectRoot()
		return true, SidebarActionData{Action: SidebarActionNone}
	case key == KeyEscape:
		return true, SidebarActionData{Action: SidebarActionBackToMenu}
	case key == KeyRune && ev.Modifiers() == 0 && c.isSearchStartRune(r):
		c.appendSearchRune(r)
		return true, SidebarActionData{Action: SidebarActionNone}
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
	preserve := c.currentPath()
	if c.searchActive() {
		c.loadSearchResults(preserve)
		return
	}
	c.reloadPreserveSelection(preserve)
}

func (c *SidebarFileTreeContent) toggleIgnored() {
	c.showIgnored = !c.showIgnored
	preserve := c.currentPath()
	if c.searchActive() {
		c.loadSearchResults(preserve)
		return
	}
	c.reloadPreserveSelection(preserve)
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
		if c.searchActive() {
			c.searchQuery = nil
			c.searchPrevPath = ""
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
		isDir := entry.IsDir()
		fullPath := filepath.Join(dir, name)
		if name == ".git" {
			item := SidebarItem{
				Label:     name,
				Path:      fullPath,
				IsDir:     isDir,
				IsHidden:  false,
				IsIgnored: false,
				Available: true,
			}
			if isDir {
				dirs = append(dirs, item)
			} else {
				files = append(files, item)
			}
			continue
		}
		isIgnored := false
		if c.gitRoot != "" {
			if rel, err := filepath.Rel(c.gitRoot, fullPath); err == nil {
				rel = strings.TrimPrefix(rel, "./")
				rel = strings.TrimPrefix(rel, ".\\")
				isIgnored = matchesGitignore(c.ignorePatterns, rel, isDir)
			}
		} else {
			rel, err := filepath.Rel(c.projectRoot, fullPath)
			if err == nil {
				rel = strings.TrimPrefix(rel, "./")
				rel = strings.TrimPrefix(rel, ".\\")
				isIgnored = matchesGitignore(c.ignorePatterns, rel, isDir)
			}
		}
		if isIgnored && !c.showIgnored {
			continue
		}
		item := SidebarItem{
			Label:     name,
			Path:      fullPath,
			IsDir:     isDir,
			IsHidden:  false,
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

func (c *SidebarFileTreeContent) selectedRelativePath() (string, bool) {
	item, ok := c.currentItem()
	if !ok || !item.Available || item.Path == "" {
		return "", false
	}
	root := c.projectRoot
	if root == "" {
		root = c.dir
	}
	if root != "" {
		if rel, err := filepath.Rel(root, item.Path); err == nil && rel != "" {
			return rel, true
		}
	}
	return item.Path, true
}

func (c *SidebarFileTreeContent) searchActive() bool {
	return len(c.searchQuery) > 0
}

func (c *SidebarFileTreeContent) isSearchStartRune(r rune) bool {
	if !unicode.IsPrint(r) {
		return false
	}
	if r == ' ' {
		return false
	}
	return true
}

func matchIndices(label string, query []rune) []int {
	if label == "" || len(query) == 0 {
		return nil
	}
	labelRunes := []rune(label)
	queryRunes := make([]rune, len(query))
	for i, r := range query {
		queryRunes[i] = unicode.ToLower(r)
	}
	lowerRunes := make([]rune, len(labelRunes))
	for i, r := range labelRunes {
		lowerRunes[i] = unicode.ToLower(r)
	}
	if len(queryRunes) > len(lowerRunes) {
		return nil
	}
	marks := make([]bool, len(lowerRunes))
	for i := 0; i <= len(lowerRunes)-len(queryRunes); i++ {
		match := true
		for j, qr := range queryRunes {
			if lowerRunes[i+j] != qr {
				match = false
				break
			}
		}
		if match {
			for j := 0; j < len(queryRunes); j++ {
				marks[i+j] = true
			}
		}
	}
	var indices []int
	for i, marked := range marks {
		if marked {
			indices = append(indices, i)
		}
	}
	return indices
}

func (c *SidebarFileTreeContent) cycleFilterMode() {
	c.showIgnored = !c.showIgnored
	preserve := c.currentPath()
	if c.searchActive() {
		c.loadSearchResults(preserve)
		return
	}
	c.reloadPreserveSelection(preserve)
}

func (c *SidebarFileTreeContent) filterModeLabel() string {
	if c.showIgnored {
		return "Files: .gitignore"
	}
	return "Files: project files"
}

func (c *SidebarFileTreeContent) appendSearchRune(r rune) {
	if !c.searchActive() {
		c.searchPrevPath = c.currentPath()
	}
	c.searchQuery = append(c.searchQuery, r)
	c.loadSearchResults(c.currentPath())
}

func (c *SidebarFileTreeContent) deleteSearchRune() {
	if len(c.searchQuery) == 0 {
		return
	}
	c.searchQuery = c.searchQuery[:len(c.searchQuery)-1]
	if len(c.searchQuery) == 0 {
		c.clearSearch()
		return
	}
	c.loadSearchResults(c.currentPath())
}

func (c *SidebarFileTreeContent) clearSearch() {
	c.searchQuery = nil
	prev := c.searchPrevPath
	c.searchPrevPath = ""
	_ = c.loadDir(c.dir)
	c.selectByPath(prev)
}

func (c *SidebarFileTreeContent) loadSearchResults(preservePath string) {
	query := strings.ToLower(string(c.searchQuery))
	if query == "" {
		return
	}
	root := c.projectRoot
	if root == "" {
		root = c.dir
	}
	if root == "" {
		c.items = []SidebarItem{{Label: "No matches", Available: false}}
		c.index = 0
		return
	}
	var items []SidebarItem
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		name := d.Name()
		isDir := d.IsDir()
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		if name == ".git" && isDir {
			if !strings.Contains(strings.ToLower(rel), query) && !strings.Contains(strings.ToLower(name), query) {
				return filepath.SkipDir
			}
		}
		isIgnored := false
		if c.gitRoot != "" {
			if rel, err := filepath.Rel(c.gitRoot, path); err == nil {
				rel = strings.TrimPrefix(rel, "./")
				rel = strings.TrimPrefix(rel, ".\\")
				isIgnored = matchesGitignore(c.ignorePatterns, rel, isDir)
			}
		} else {
			if rel, err := filepath.Rel(c.projectRoot, path); err == nil {
				rel = strings.TrimPrefix(rel, "./")
				rel = strings.TrimPrefix(rel, ".\\")
				isIgnored = matchesGitignore(c.ignorePatterns, rel, isDir)
			}
		}
		if isIgnored && !c.showIgnored {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}
		nameLower := strings.ToLower(name)
		relLower := strings.ToLower(rel)
		if !strings.Contains(nameLower, query) && !strings.Contains(relLower, query) {
			if name == ".git" && isDir {
				return filepath.SkipDir
			}
			return nil
		}
		label := name
		if isDir && !strings.HasSuffix(label, "/") {
			label += "/"
		}
		matchIndices := matchIndices(name, c.searchQuery)
		items = append(items, SidebarItem{
			Label:        label,
			Path:         path,
			IsDir:        isDir,
			IsHidden:     false,
			IsIgnored:    isIgnored,
			Available:    true,
			MatchIndices: matchIndices,
		})
		if name == ".git" && isDir {
			return filepath.SkipDir
		}
		if isDir {
			return nil
		}
		return nil
	})
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Label) < strings.ToLower(items[j].Label)
	})
	if len(items) == 0 {
		c.items = []SidebarItem{{Label: "No matches", Available: false}}
		c.index = 0
		return
	}
	c.items = items
	if preservePath != "" {
		c.selectByPath(preservePath)
	}
	if c.index < 0 || c.index >= len(c.items) {
		c.index = 0
	}
}
