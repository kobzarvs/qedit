package editor

import (
	_ "embed"
	"strings"
)

//go:embed testdata/vim-01-beginner.tutor
var vimTutorText string

//go:embed testdata/helix-tutor
var helixTutorText string

type tutorSpec struct {
	name    string
	profile string
	title   string
	text    string
}

func (e *Editor) openTutor(name string) bool {
	spec, ok := e.resolveTutor(name)
	if !ok {
		e.setStatus("usage: tutor [vim|helix]")
		return false
	}
	e.openScratchBuffer(spec.title, spec.text)
	e.SetBehaviorProfile(spec.profile)
	e.mode = ModeNormal
	e.setStatus("tutor: " + spec.name)
	return false
}

func (e *Editor) resolveTutor(name string) (tutorSpec, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = e.BehaviorProfile()
		if name == BehaviorProfileBasic {
			name = BehaviorProfileHelix
		}
	}
	switch name {
	case BehaviorProfileVim, "vi", "vimtutor":
		return tutorSpec{
			name:    BehaviorProfileVim,
			profile: BehaviorProfileVim,
			title:   "[Tutor: Vim]",
			text:    strings.TrimRight(vimTutorText, "\n"),
		}, true
	case BehaviorProfileHelix, "hx":
		return tutorSpec{
			name:    BehaviorProfileHelix,
			profile: BehaviorProfileHelix,
			title:   "[Tutor: Helix]",
			text:    strings.TrimRight(helixTutorText, "\n"),
		}, true
	default:
		return tutorSpec{}, false
	}
}

func (e *Editor) openScratchBuffer(title, text string) {
	e.saveCurrentBufferForNewBuffer()

	e.text = NewTextBufferFromString(text)
	e.huge = editorHugeFileState{}
	e.clearGitDiffPreview()
	e.cursor = Cursor{}
	e.file.diskContent = text
	e.file.readOnly = false
	e.resetConflictBlocks()
	e.viewport.scroll = 0
	e.viewport.scrollX = 0
	e.mode = ModeNormal
	e.document.filename = ""
	e.document.title = title
	e.commandLine.text = e.commandLine.text[:0]
	e.commandLine.cursor = 0
	e.commandLine.historyIndex = -1
	e.undo = nil
	e.redo = nil
	e.savePoint = 0
	e.undoGroup = 0
	e.lineUndoValid = false
	e.lineUndoContent = nil
	e.change.tick = 0
	e.change.lastEdit.Valid = false
	e.highlight = editorHighlightState{start: -1, end: -1}
	e.selectionActive = false
	e.clipboard = editorClipboardState{}
	e.selectionScope = selectionScopeState{}
	e.searchMatches = nil
	e.searchMatchIndex = 0
	e.file.externalChange = ExternalChangeNone
	e.updateDirty()

	if e.buffers != nil {
		bs := e.snapshotBufferState()
		newIdx := e.buffers.Add(bs)
		e.buffers.SetActive(newIdx)
		e.setActiveWindowBufferIndex(newIdx)
	}
}

func (e *Editor) saveCurrentBufferForNewBuffer() {
	if e.buffers == nil {
		return
	}
	if e.buffers.Count() > 0 {
		bs := e.snapshotBufferState()
		e.buffers.UpdateActive(bs)
		return
	}
	if e.currentBufferWorthKeeping() {
		e.buffers.Add(e.snapshotBufferState())
	}
}

func (e *Editor) currentBufferWorthKeeping() bool {
	if e.document.filename != "" || e.document.title != "" || e.document.dirty {
		return true
	}
	return e.Content() != ""
}
