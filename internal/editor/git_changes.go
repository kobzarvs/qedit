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
	if root == e.git.root {
		return
	}
	e.git.root = root
	e.git.changes = nil
	e.git.changeHunks = nil
	e.git.changesVersion++
	e.git.changesUpdated = time.Time{}
	if e.sidebar != nil {
		if content, ok := e.sidebar.Content.(*SidebarWorktreesContent); ok {
			content.SetCurrentPath(root)
		}
	}
}

// RefreshGitChanges reloads git change data.
func (e *Editor) RefreshGitChanges() error {
	root := e.git.root
	if root == "" {
		root = e.detectGitRoot()
		if root != "" {
			e.git.root = root
		}
	}
	if root == "" {
		e.git.changes = nil
		e.git.changeHunks = nil
		e.git.changesVersion++
		e.git.changesUpdated = time.Now()
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
	if e.git.changesUpdated.IsZero() || time.Since(e.git.changesUpdated) >= maxAge {
		return e.RefreshGitChanges()
	}
	return nil
}

func (e *Editor) detectGitRoot() string {
	if e.document.filename != "" {
		if root := gitinfo.Root(e.document.filename); root != "" {
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
	e.git.changes = make([]GitFileChange, 0, len(changes))
	for _, ch := range changes {
		abs := filepath.Join(root, filepath.FromSlash(ch.Path))
		e.git.changes = append(e.git.changes, GitFileChange{
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
	sort.Slice(e.git.changes, func(i, j int) bool {
		return e.git.changes[i].Path < e.git.changes[j].Path
	})

	e.git.changeHunks = make([]GitChangeHunk, 0, len(hunks))
	for _, h := range hunks {
		abs := filepath.Join(root, filepath.FromSlash(h.Path))
		e.git.changeHunks = append(e.git.changeHunks, GitChangeHunk{
			Path:      h.Path,
			AbsPath:   abs,
			StartLine: h.StartLine,
			EndLine:   h.EndLine,
		})
	}
	sort.Slice(e.git.changeHunks, func(i, j int) bool {
		if e.git.changeHunks[i].AbsPath != e.git.changeHunks[j].AbsPath {
			return e.git.changeHunks[i].AbsPath < e.git.changeHunks[j].AbsPath
		}
		return e.git.changeHunks[i].StartLine < e.git.changeHunks[j].StartLine
	})

	e.git.changesUpdated = time.Now()
	e.git.changesVersion++
}

func (e *Editor) gotoGitChange(forward bool) {
	if err := e.refreshGitChangesIfStale(2 * time.Second); err != nil {
		e.setStatus(err.Error())
		e.git.pendingDiffJump = false
		return
	}
	if len(e.git.changeHunks) == 0 {
		e.setStatus("no git changes")
		e.git.diffHighlight = nil
		e.git.pendingDiffJump = false
		return
	}
	e.git.pendingDiffJump = true
	currentPath := e.document.filename
	if currentPath != "" {
		if abs, err := filepath.Abs(currentPath); err == nil {
			currentPath = abs
		}
	}
	target := e.findGitChangeHunk(currentPath, e.cursor.Row, forward)
	if target == nil {
		e.setStatus("no git changes")
		e.git.diffHighlight = nil
		e.git.pendingDiffJump = false
		return
	}
	if target.AbsPath == "" {
		e.setStatus("no git changes")
		e.git.diffHighlight = nil
		e.git.pendingDiffJump = false
		return
	}
	e.setGitDiffHighlight(target)
	e.mode = ModeMerge
	e.highlightGitChangeInSidebar(target.AbsPath)
	if currentPath != "" && target.AbsPath == currentPath {
		e.JumpToLocation(target.StartLine, 0)
		e.git.pendingDiffJump = false
		return
	}
	e.requestOpenLocation(target.AbsPath, target.StartLine, 0)
}

func (e *Editor) setGitDiffHighlight(hunk *GitChangeHunk) {
	if hunk == nil {
		e.git.diffHighlight = nil
		return
	}
	copy := *hunk
	e.git.diffHighlight = &copy
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
	if len(e.git.changeHunks) == 0 {
		return conflictNone
	}
	currentPath := e.document.filename
	if abs, err := filepath.Abs(currentPath); err == nil {
		currentPath = abs
	}
	for _, h := range e.git.changeHunks {
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
	if len(e.git.changeHunks) == 0 || e.document.filename == "" {
		return false
	}
	currentPath := e.document.filename
	if abs, err := filepath.Abs(currentPath); err == nil {
		currentPath = abs
	}
	for _, h := range e.git.changeHunks {
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
	hunk := e.git.diffHighlight
	if hunk == nil || hunk.AbsPath == "" || e.document.filename == "" {
		return false
	}
	currentPath := e.document.filename
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
	if !e.git.pendingDiffJump {
		return
	}
	e.git.pendingDiffJump = false
	e.mode = ModeMerge
}

// ApplyPendingGitDiffJump applies deferred diff mode after opening a file.
func (e *Editor) ApplyPendingGitDiffJump() {
	e.applyPendingGitDiffJump()
}

func (e *Editor) findGitChangeHunk(currentPath string, cursorRow int, forward bool) *GitChangeHunk {
	hunks := e.git.changeHunks
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
	e.requests.openLocation.active = true
	e.requests.openLocation.path = path
	e.requests.openLocation.line = line
	e.requests.openLocation.col = col
	e.requests.sidebarOpenFilePath = path
}

func (e *Editor) ConsumePendingOpenLocation() (string, int, int, bool) {
	if !e.requests.openLocation.active {
		return "", 0, 0, false
	}
	path := e.requests.openLocation.path
	line := e.requests.openLocation.line
	col := e.requests.openLocation.col
	e.requests.openLocation = openLocationRequest{}
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
	e.interaction.freeScroll = false
	e.ensureCursorVisible(e.viewHeightCached())
	e.ensureCursorVisibleHorizontal(e.viewport.width, e.gutterWidth())
}
