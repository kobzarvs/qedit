package editor

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SidebarGitChangesContent implements SidebarContent for git changes.
type SidebarGitChangesContent struct {
	editor        *Editor
	viewTree      bool
	index         int
	items         []SidebarItem
	lastVersion   uint64
	lastViewTree  bool
	lastGitRoot   string
	lastItemCount int
	currentPath   string
	lastCurrent   string
}

func NewSidebarGitChangesContent(ed *Editor) *SidebarGitChangesContent {
	return &SidebarGitChangesContent{
		editor: ed,
	}
}

func (c *SidebarGitChangesContent) Mode() SidebarMode {
	return SidebarModeLocalChanges
}

func (c *SidebarGitChangesContent) Title() string {
	if c.viewTree {
		return "Git Changes (tree)"
	}
	return "Git Changes (flat)"
}

func (c *SidebarGitChangesContent) Items() []SidebarItem {
	c.ensureItems()
	return c.items
}

func (c *SidebarGitChangesContent) Index() int {
	return c.index
}

func (c *SidebarGitChangesContent) SetIndex(i int) {
	if i >= 0 && i < len(c.items) {
		c.index = i
	}
}

func (c *SidebarGitChangesContent) SetCurrentPath(path string) {
	if path == "" {
		c.currentPath = ""
		c.ensureItems()
		return
	}
	if c.editor != nil {
		path = c.editor.normalizedPath(path)
	}
	c.currentPath = filepath.Clean(path)
	c.ensureItems()
}

func (c *SidebarGitChangesContent) HandleKey(ev EventKey) (bool, SidebarActionData) {
	key := ev.Key()
	r := ev.Rune()
	switch {
	case r == 't':
		c.viewTree = !c.viewTree
		c.ensureItems()
		return true, SidebarActionData{Action: SidebarActionNone}
	case r == 'r':
		if err := c.editor.RefreshGitChanges(); err != nil {
			c.editor.setStatus(err.Error())
		}
		c.ensureItems()
		return true, SidebarActionData{Action: SidebarActionNone}
	case key == KeyRight || r == 'l':
		return true, c.OnEnter()
	}
	return false, SidebarActionData{Action: SidebarActionNone}
}

func (c *SidebarGitChangesContent) OnEnter() SidebarActionData {
	c.ensureItems()
	if c.index < 0 || c.index >= len(c.items) {
		return SidebarActionData{Action: SidebarActionNone}
	}
	item := c.items[c.index]
	if item.Path == "" || item.IsDir || !item.Available {
		return SidebarActionData{Action: SidebarActionNone}
	}
	line, col, ok := c.editor.prepareGitChangeOpen(item.Path)
	if !ok {
		return SidebarActionData{Action: SidebarActionOpenFile, Path: item.Path}
	}
	return SidebarActionData{
		Action:      SidebarActionOpenFile,
		Path:        item.Path,
		Line:        line,
		Col:         col,
		HasLocation: true,
	}
}

func (c *SidebarGitChangesContent) Available() bool {
	return c.editor != nil && c.editor.isGitRepo()
}

func (c *SidebarGitChangesContent) Refresh() error {
	if err := c.editor.RefreshGitChanges(); err != nil {
		return err
	}
	c.ensureItems()
	return nil
}

func (c *SidebarGitChangesContent) ensureItems() {
	if c.editor == nil {
		c.items = []SidebarItem{{Label: "No changes", Available: false}}
		c.index = 0
		return
	}
	_ = c.editor.refreshGitChangesIfStale(2 * time.Second)
	version := c.editor.git.changesVersion
	root := c.editor.git.root
	currentPath := strings.TrimSpace(c.currentPath)
	if currentPath != "" {
		currentPath = filepath.Clean(currentPath)
	}
	if version == c.lastVersion && c.viewTree == c.lastViewTree && root == c.lastGitRoot && c.lastItemCount == len(c.editor.git.changes) && currentPath == c.lastCurrent {
		return
	}
	c.lastVersion = version
	c.lastViewTree = c.viewTree
	c.lastGitRoot = root
	c.lastItemCount = len(c.editor.git.changes)
	c.lastCurrent = currentPath

	changes := c.editor.git.changes
	if len(changes) == 0 {
		c.items = []SidebarItem{{Label: "No changes", Available: false}}
		c.index = 0
		return
	}
	staged, unstaged := splitGitChanges(changes)
	if c.viewTree {
		c.items = buildGitChangesTreeGroups(staged, unstaged, root, c.editor.sidebarStyles, currentPath)
	} else {
		c.items = buildGitChangesFlatGroups(staged, unstaged, c.editor.sidebarStyles, currentPath)
	}
	if currentPath != "" {
		for i, item := range c.items {
			if item.Path != "" && !item.IsDir && item.Path == currentPath {
				c.index = i
				break
			}
		}
	}
	if c.index >= len(c.items) {
		c.index = len(c.items) - 1
		if c.index < 0 {
			c.index = 0
		}
	}
}

func buildGitChangesFlatItems(changes []GitFileChange, styles SidebarStyles, widths gitChangeColumnWidths, currentPath string) []SidebarItem {
	items := make([]SidebarItem, 0, len(changes))
	sorted := append([]GitFileChange(nil), changes...)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i].Path) < strings.ToLower(sorted[j].Path)
	})
	for _, ch := range sorted {
		label := " " + filepath.Base(filepath.FromSlash(ch.Path))
		isCurrent := currentPath != "" && currentPath == ch.AbsPath
		items = append(items, SidebarItem{
			Label:         label,
			Path:          ch.AbsPath,
			IsDir:         false,
			IsCurrent:     isCurrent,
			Available:     true,
			Icon:          statusRune(ch.Status),
			IconStyle:     gitChangeIconStyle(ch, styles),
			Hotkey:        formatGitChangeCounts(ch),
			RightSegments: gitChangeSegments(ch, styles, widths),
		})
	}
	return items
}

type gitChangeNode struct {
	name     string
	relPath  string
	absPath  string
	isDir    bool
	change   *GitFileChange
	children map[string]*gitChangeNode
}

func buildGitChangesTreeItems(changes []GitFileChange, root string, styles SidebarStyles, widths gitChangeColumnWidths, currentPath string) []SidebarItem {
	tree := &gitChangeNode{isDir: true, children: make(map[string]*gitChangeNode)}
	for i := range changes {
		ch := &changes[i]
		parts := strings.Split(filepath.ToSlash(ch.Path), "/")
		node := tree
		var relParts []string
		for idx, part := range parts {
			if part == "" {
				continue
			}
			relParts = append(relParts, part)
			child := node.children[part]
			if child == nil {
				child = &gitChangeNode{
					name:     part,
					relPath:  filepath.ToSlash(filepath.Join(relParts...)),
					isDir:    idx < len(parts)-1,
					children: make(map[string]*gitChangeNode),
				}
				if child.isDir {
					child.absPath = filepath.Join(root, filepath.FromSlash(child.relPath))
				} else {
					child.absPath = ch.AbsPath
				}
				node.children[part] = child
			}
			node = child
		}
		node.isDir = false
		node.change = ch
		node.absPath = ch.AbsPath
	}

	var items []SidebarItem
	appendTreeItems(&items, tree, 0, styles, widths, currentPath)
	return items
}

func appendTreeItems(items *[]SidebarItem, node *gitChangeNode, depth int, styles SidebarStyles, widths gitChangeColumnWidths, currentPath string) {
	if node == nil {
		return
	}
	// Sort children: dirs first, then files.
	var dirs, files []*gitChangeNode
	for _, child := range node.children {
		if child.isDir {
			dirs = append(dirs, child)
		} else {
			files = append(files, child)
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].name) < strings.ToLower(dirs[j].name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].name) < strings.ToLower(files[j].name)
	})

	indent := strings.Repeat("  ", depth)
	for _, dir := range dirs {
		*items = append(*items, SidebarItem{
			Label:     indent + dir.name + "/",
			Path:      dir.absPath,
			IsDir:     true,
			Available: true,
		})
		appendTreeItems(items, dir, depth+1, styles, widths, currentPath)
	}
	for _, file := range files {
		ch := file.change
		isCurrent := currentPath != "" && currentPath == file.absPath
		item := SidebarItem{
			Label:     indent + " " + file.name,
			Path:      file.absPath,
			IsDir:     false,
			IsCurrent: isCurrent,
			Available: true,
		}
		if ch != nil {
			item.Icon = statusRune(ch.Status)
			item.IconStyle = gitChangeIconStyle(*ch, styles)
			item.Hotkey = formatGitChangeCounts(*ch)
			item.RightSegments = gitChangeSegments(*ch, styles, widths)
		}
		*items = append(*items, item)
	}
}

func formatGitChangeCounts(ch GitFileChange) string {
	if ch.Binary {
		return "bin"
	}
	return "+" + strconv.Itoa(ch.Insertions) + " -" + strconv.Itoa(ch.Deletions)
}

func gitChangeSegments(ch GitFileChange, styles SidebarStyles, widths gitChangeColumnWidths) []SidebarTextSegment {
	if ch.Binary {
		return nil
	}
	addStyle := styles.DiffAdd
	delStyle := styles.DiffDel
	if addStyle == nil {
		addStyle = styles.Hotkey
	}
	if delStyle == nil {
		delStyle = styles.Hotkey
	}
	if widths.addDigits == 0 && widths.delDigits == 0 {
		widths.addDigits = countDigits(ch.Insertions)
		widths.delDigits = countDigits(ch.Deletions)
	}
	addText := formatGitChangeColumn(ch.Insertions, '+', widths.addDigits)
	delText := formatGitChangeColumn(ch.Deletions, '-', widths.delDigits)
	return []SidebarTextSegment{
		{Text: addText, Style: addStyle},
		{Text: " ", Style: nil},
		{Text: delText, Style: delStyle},
	}
}

func gitChangeIconStyle(ch GitFileChange, styles SidebarStyles) Style {
	addStyle := styles.DiffAdd
	delStyle := styles.DiffDel
	if addStyle == nil {
		addStyle = styles.Hotkey
	}
	if delStyle == nil {
		delStyle = styles.Hotkey
	}
	switch ch.Status {
	case "A", "?":
		return addStyle
	case "D":
		return delStyle
	case "M":
		if ch.Deletions > ch.Insertions {
			return delStyle
		}
		return addStyle
	default:
		if ch.Insertions > ch.Deletions {
			return addStyle
		}
		if ch.Deletions > ch.Insertions {
			return delStyle
		}
	}
	return nil
}

type gitChangeColumnWidths struct {
	addDigits int
	delDigits int
}

func gitChangeColumnWidthsFor(changes []GitFileChange) gitChangeColumnWidths {
	var widths gitChangeColumnWidths
	for _, ch := range changes {
		if ch.Binary {
			continue
		}
		widths.addDigits = max(widths.addDigits, countDigits(ch.Insertions))
		widths.delDigits = max(widths.delDigits, countDigits(ch.Deletions))
	}
	return widths
}

func countDigits(n int) int {
	if n <= 0 {
		return 1
	}
	return len(strconv.Itoa(n))
}

func formatGitChangeColumn(count int, sign rune, digits int) string {
	if digits <= 0 {
		return ""
	}
	width := 1 + digits
	if count == 0 {
		return strings.Repeat(" ", width)
	}
	return fmt.Sprintf("%*s", width, fmt.Sprintf("%c%d", sign, count))
}

func splitGitChanges(changes []GitFileChange) (staged, unstaged []GitFileChange) {
	for _, ch := range changes {
		if ch.Staged {
			staged = append(staged, ch)
		}
		if ch.Unstaged || (!ch.Staged && !ch.Unstaged) {
			unstaged = append(unstaged, ch)
		}
	}
	return staged, unstaged
}

func buildGitChangesFlatGroups(staged, unstaged []GitFileChange, styles SidebarStyles, currentPath string) []SidebarItem {
	var items []SidebarItem
	if len(staged) > 0 {
		widths := gitChangeColumnWidthsFor(staged)
		items = append(items, SidebarItem{Label: "Staged", Available: false})
		items = append(items, buildGitChangesFlatItems(staged, styles, widths, currentPath)...)
	}
	if len(unstaged) > 0 {
		widths := gitChangeColumnWidthsFor(unstaged)
		if len(items) > 0 {
			items = append(items, SidebarItem{Label: "", Available: false})
		}
		items = append(items, SidebarItem{Label: "Unstaged", Available: false})
		items = append(items, buildGitChangesFlatItems(unstaged, styles, widths, currentPath)...)
	}
	return items
}

func buildGitChangesTreeGroups(staged, unstaged []GitFileChange, root string, styles SidebarStyles, currentPath string) []SidebarItem {
	var items []SidebarItem
	if len(staged) > 0 {
		widths := gitChangeColumnWidthsFor(staged)
		items = append(items, SidebarItem{Label: "Staged", Available: false})
		items = append(items, buildGitChangesTreeItems(staged, root, styles, widths, currentPath)...)
	}
	if len(unstaged) > 0 {
		widths := gitChangeColumnWidthsFor(unstaged)
		if len(items) > 0 {
			items = append(items, SidebarItem{Label: "", Available: false})
		}
		items = append(items, SidebarItem{Label: "Unstaged", Available: false})
		items = append(items, buildGitChangesTreeItems(unstaged, root, styles, widths, currentPath)...)
	}
	return items
}

func statusRune(status string) rune {
	if status == "" {
		return 0
	}
	return []rune(status)[0]
}
