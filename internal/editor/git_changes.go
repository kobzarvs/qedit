package editor

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kobzarvs/qedit/internal/gitinfo"
)

// GitFileChange represents a changed file in the repo.
type GitFileChange struct {
	Path       string // repo-relative, slash-separated
	AbsPath    string
	Status     string
	Insertions int
	Deletions  int
	Binary     bool
	Staged     bool
	Unstaged   bool
}

// GitChangeHunk represents a changed hunk location in a file.
type GitChangeHunk struct {
	Path      string // repo-relative, slash-separated
	AbsPath   string
	StartLine int
	EndLine   int
}

// SetGitRoot sets the current git root for the editor.
func (e *Editor) SetGitRoot(root string) {
	root = strings.TrimSpace(root)
	if root == e.gitRoot {
		return
	}
	e.gitRoot = root
	e.gitChanges = nil
	e.gitChangeHunks = nil
	e.gitChangesVersion++
	e.gitChangesUpdated = time.Time{}
	if e.sidebar != nil {
		if content, ok := e.sidebar.Content.(*SidebarWorktreesContent); ok {
			content.SetCurrentPath(root)
		}
	}
}

// RefreshGitChanges reloads git change data.
func (e *Editor) RefreshGitChanges() error {
	root := e.gitRoot
	if root == "" {
		root = e.detectGitRoot()
		if root != "" {
			e.gitRoot = root
		}
	}
	if root == "" {
		e.gitChanges = nil
		e.gitChangeHunks = nil
		e.gitChangesVersion++
		e.gitChangesUpdated = time.Now()
		return nil
	}
	changes, hunks, err := gitinfo.Changes(root)
	if err != nil {
		return err
	}
	e.applyGitChanges(root, changes, hunks)
	return nil
}

func (e *Editor) refreshGitChangesIfStale(maxAge time.Duration) error {
	if e.gitChangesUpdated.IsZero() || time.Since(e.gitChangesUpdated) >= maxAge {
		return e.RefreshGitChanges()
	}
	return nil
}

func (e *Editor) detectGitRoot() string {
	if e.filename != "" {
		if root := gitinfo.Root(e.filename); root != "" {
			return root
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if root := gitinfo.Root(cwd); root != "" {
			return root
		}
	}
	return ""
}

func (e *Editor) applyGitChanges(root string, changes []gitinfo.FileChange, hunks []gitinfo.Hunk) {
	e.gitChanges = make([]GitFileChange, 0, len(changes))
	for _, ch := range changes {
		abs := filepath.Join(root, filepath.FromSlash(ch.Path))
		e.gitChanges = append(e.gitChanges, GitFileChange{
			Path:       ch.Path,
			AbsPath:    abs,
			Status:     ch.Status,
			Insertions: ch.Insertions,
			Deletions:  ch.Deletions,
			Binary:     ch.Binary,
			Staged:     ch.Staged,
			Unstaged:   ch.Unstaged,
		})
	}
	sort.Slice(e.gitChanges, func(i, j int) bool {
		return e.gitChanges[i].Path < e.gitChanges[j].Path
	})

	e.gitChangeHunks = make([]GitChangeHunk, 0, len(hunks))
	for _, h := range hunks {
		abs := filepath.Join(root, filepath.FromSlash(h.Path))
		e.gitChangeHunks = append(e.gitChangeHunks, GitChangeHunk{
			Path:      h.Path,
			AbsPath:   abs,
			StartLine: h.StartLine,
			EndLine:   h.EndLine,
		})
	}
	sort.Slice(e.gitChangeHunks, func(i, j int) bool {
		if e.gitChangeHunks[i].AbsPath != e.gitChangeHunks[j].AbsPath {
			return e.gitChangeHunks[i].AbsPath < e.gitChangeHunks[j].AbsPath
		}
		return e.gitChangeHunks[i].StartLine < e.gitChangeHunks[j].StartLine
	})

	e.gitChangesUpdated = time.Now()
	e.gitChangesVersion++
}

func (e *Editor) gotoGitChange(forward bool) {
	if err := e.refreshGitChangesIfStale(2 * time.Second); err != nil {
		e.setStatus(err.Error())
		e.pendingGitDiffJump = false
		return
	}
	if len(e.gitChangeHunks) == 0 {
		e.setStatus("no git changes")
		e.gitDiffHighlight = nil
		e.pendingGitDiffJump = false
		return
	}
	e.pendingGitDiffJump = true
	currentPath := e.filename
	if currentPath != "" {
		if abs, err := filepath.Abs(currentPath); err == nil {
			currentPath = abs
		}
	}
	target := e.findGitChangeHunk(currentPath, e.cursor.Row, forward)
	if target == nil {
		e.setStatus("no git changes")
		e.gitDiffHighlight = nil
		e.pendingGitDiffJump = false
		return
	}
	if target.AbsPath == "" {
		e.setStatus("no git changes")
		e.gitDiffHighlight = nil
		e.pendingGitDiffJump = false
		return
	}
	e.setGitDiffHighlight(target)
	e.mode = ModeMerge
	e.highlightGitChangeInSidebar(target.AbsPath)
	if currentPath != "" && target.AbsPath == currentPath {
		e.JumpToLocation(target.StartLine, 0)
		e.pendingGitDiffJump = false
		return
	}
	e.requestOpenLocation(target.AbsPath, target.StartLine, 0)
}

func (e *Editor) setGitDiffHighlight(hunk *GitChangeHunk) {
	if hunk == nil {
		e.gitDiffHighlight = nil
		return
	}
	copy := *hunk
	e.gitDiffHighlight = &copy
}

func (e *Editor) gitDiffGutterActive() bool {
	if e.mode != ModeMerge {
		return false
	}
	return e.gitDiffHasCurrentFileHunks()
}

func (e *Editor) gitDiffLineKind(lineIdx int) conflictLineKind {
	if e.mode != ModeMerge {
		return conflictNone
	}
	if len(e.gitChangeHunks) == 0 {
		return conflictNone
	}
	currentPath := e.filename
	if abs, err := filepath.Abs(currentPath); err == nil {
		currentPath = abs
	}
	for _, h := range e.gitChangeHunks {
		if h.AbsPath < currentPath {
			continue
		}
		if h.AbsPath > currentPath {
			break
		}
		if lineIdx >= h.StartLine && lineIdx <= h.EndLine {
			return conflictRemote
		}
	}
	return conflictNone
}

func (e *Editor) gitDiffHasCurrentFileHunks() bool {
	if len(e.gitChangeHunks) == 0 || e.filename == "" {
		return false
	}
	currentPath := e.filename
	if abs, err := filepath.Abs(currentPath); err == nil {
		currentPath = abs
	}
	for _, h := range e.gitChangeHunks {
		if h.AbsPath < currentPath {
			continue
		}
		if h.AbsPath > currentPath {
			break
		}
		return true
	}
	return false
}

func (e *Editor) gitDiffHighlightAppliesToCurrentFile() bool {
	hunk := e.gitDiffHighlight
	if hunk == nil || hunk.AbsPath == "" || e.filename == "" {
		return false
	}
	currentPath := e.filename
	if abs, err := filepath.Abs(currentPath); err == nil {
		currentPath = abs
	}
	return currentPath == hunk.AbsPath
}

func (e *Editor) highlightGitChangeInSidebar(absPath string) {
	if absPath == "" || e.sidebar == nil || !e.sidebar.Visible {
		return
	}
	content, ok := e.sidebar.Content.(*SidebarGitChangesContent)
	if !ok {
		return
	}
	content.SetCurrentPath(absPath)
	listHeight := e.viewHeightCached() - 1
	if listHeight < 1 {
		listHeight = 1
	}
	e.sidebar.EnsureVisible(listHeight)
}

func (e *Editor) applyPendingGitDiffJump() {
	if !e.pendingGitDiffJump {
		return
	}
	e.pendingGitDiffJump = false
	e.mode = ModeMerge
}

// ApplyPendingGitDiffJump applies deferred diff mode after opening a file.
func (e *Editor) ApplyPendingGitDiffJump() {
	e.applyPendingGitDiffJump()
}

func (e *Editor) findGitChangeHunk(currentPath string, cursorRow int, forward bool) *GitChangeHunk {
	hunks := e.gitChangeHunks
	if len(hunks) == 0 {
		return nil
	}
	if forward {
		if currentPath != "" {
			for i := range hunks {
				h := &hunks[i]
				if h.AbsPath == currentPath && h.StartLine > cursorRow {
					return h
				}
			}
			for i := range hunks {
				h := &hunks[i]
				if h.AbsPath > currentPath {
					return h
				}
			}
		}
		return &hunks[0]
	}
	if currentPath != "" {
		for i := len(hunks) - 1; i >= 0; i-- {
			h := &hunks[i]
			if h.AbsPath == currentPath && h.StartLine < cursorRow {
				return h
			}
		}
		for i := len(hunks) - 1; i >= 0; i-- {
			h := &hunks[i]
			if h.AbsPath < currentPath {
				return h
			}
		}
	}
	return &hunks[len(hunks)-1]
}

func (e *Editor) requestOpenLocation(path string, line, col int) {
	if path == "" {
		return
	}
	e.pendingOpenLocation = true
	e.pendingOpenPath = path
	e.pendingOpenLine = line
	e.pendingOpenCol = col
	e.sidebarOpenFilePath = path
}

func (e *Editor) ConsumePendingOpenLocation() (string, int, int, bool) {
	if !e.pendingOpenLocation {
		return "", 0, 0, false
	}
	path := e.pendingOpenPath
	line := e.pendingOpenLine
	col := e.pendingOpenCol
	e.pendingOpenLocation = false
	e.pendingOpenPath = ""
	e.pendingOpenLine = 0
	e.pendingOpenCol = 0
	return path, line, col, true
}

func (e *Editor) JumpToLocation(line, col int) {
	lineCount := e.LineCount()
	if lineCount == 0 {
		return
	}
	if line < 0 {
		line = 0
	}
	if line >= lineCount {
		line = lineCount - 1
	}
	if col < 0 {
		col = 0
	}
	if col > e.lineLen(line) {
		col = e.lineLen(line)
	}
	e.cursor.Row = line
	e.cursor.Col = col
	e.selectionActive = false
	e.freeScroll = false
	e.ensureCursorVisible(e.viewHeightCached())
	e.ensureCursorVisibleHorizontal(e.viewWidth, e.gutterWidth())
}
