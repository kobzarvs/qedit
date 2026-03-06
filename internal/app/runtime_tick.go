package app

import (
	"time"

	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/gitinfo"
	"github.com/kobzarvs/qedit/internal/platform/keyboard"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

func runEditorRuntimeTick(
	ed *editor.Editor,
	ts *treesitter.Engine,
	fileMonitor *externalFileMonitor,
	state *editorRuntimeState,
	fileChangeDebounce time.Duration,
	filePollInterval time.Duration,
	lastLayoutRaw *string,
) {
	now := time.Now()
	fileMonitor.ProcessWatcherEvents(now, fileChangeDebounce)
	fileMonitor.HandleAutoReloadResults()
	if state.openPath != "" {
		fileMonitor.PollExternalChange(now, filePollInterval)
	}

	state.lastChangeTick, state.lastHighlightStart, state.lastHighlightEnd = syncVisibleHighlights(
		ed,
		ts,
		state.openPath,
		state.langName,
		state.highlightEnabled,
		state.lastChangeTick,
		state.lastHighlightStart,
		state.lastHighlightEnd,
	)

	layoutRaw := keyboard.CurrentLayoutRaw()
	if layoutRaw != *lastLayoutRaw {
		*lastLayoutRaw = layoutRaw
		ed.SetKeyboardLayout(keyboard.CurrentLayout())
	}

	if state.gitPath != "" && now.Sub(state.lastGitCheck) > 2*time.Second {
		state.lastGitCheck = now
		ed.SetGitBranch(gitinfo.Branch(state.gitPath))
	}

	if state.highlightExpected && !ed.HasHighlights() && state.openPath != ed.Filename() {
		state.highlightExpected = false
	}
}
