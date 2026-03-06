package app

import (
	"os"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/platform/keyboard"
	"github.com/kobzarvs/qedit/internal/session"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

type editorRuntimeOptions struct {
	InitialPath              string
	HighlightMaxBytes        int64
	AutoReloadMaxBytes       int64
	AutoReloadRetries        int
	AutoReloadStabilizeDelay time.Duration
	FileChangeDebounce       time.Duration
	FilePollInterval         time.Duration
}

type editorRuntime struct {
	screen             tcell.Screen
	ed                 *editor.Editor
	ts                 *treesitter.Engine
	fileMonitor        *externalFileMonitor
	controller         editorRuntimeController
	state              editorRuntimeState
	lastLayoutRaw      string
	fileChangeDebounce time.Duration
	filePollInterval   time.Duration
}

func newEditorRuntime(
	screen tcell.Screen,
	ed *editor.Editor,
	ls *lsp.Manager,
	ts *treesitter.Engine,
	langs config.Languages,
	sessionMgr *session.Manager,
	opts editorRuntimeOptions,
) (*editorRuntime, error) {
	autoReloadRetries := opts.AutoReloadRetries
	if autoReloadRetries < 1 {
		autoReloadRetries = 1
	}

	rt := &editorRuntime{
		screen:             screen,
		ed:                 ed,
		ts:                 ts,
		state:              newEditorRuntimeState(ed),
		lastLayoutRaw:      keyboard.CurrentLayoutRaw(),
		fileChangeDebounce: opts.FileChangeDebounce,
		filePollInterval:   opts.FilePollInterval,
	}

	if opts.InitialPath != "" {
		rt.state.openPath = opts.InitialPath
		if err := ed.OpenFile(rt.state.openPath); err != nil {
			return nil, err
		}
		rt.state.gitPath = rt.state.openPath
		content := ed.Content()
		ls.OpenFile(rt.state.openPath, content)
		rt.state.langName, rt.state.highlightEnabled = detectHighlightLanguage(rt.state.openPath, langs, opts.HighlightMaxBytes)
		rt.state.highlightExpected = rt.state.highlightEnabled && rt.state.langName != ""
	}
	if rt.state.gitPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			rt.state.gitPath = cwd
		}
	}

	rt.fileMonitor = newExternalFileMonitor(screen, ed, opts.AutoReloadMaxBytes, autoReloadRetries, opts.AutoReloadStabilizeDelay)
	if rt.state.openPath != "" {
		rt.fileMonitor.Watch(rt.state.openPath)
	}

	ed.SetKeyboardLayout(keyboard.CurrentLayout())
	syncEditorRepoState(ed, rt.state.gitPath, sessionMgr)
	wireEditorRuntimeCallbacks(ed, ts, ls)

	if rt.state.openPath != "" && rt.state.highlightEnabled && rt.state.langName != "" {
		if isAsyncParseLang(rt.state.langName) {
			ts.Parse(rt.state.openPath, rt.state.langName, ed.Content())
		} else if ts.ParseSync(rt.state.openPath, rt.state.langName, ed.Content()) {
			if _, end, ok := applyInitialScreenHighlights(ed, screen, ts, rt.state.openPath); ok {
				rt.state.lastHighlightStart = 0
				rt.state.lastHighlightEnd = end
			}
		} else {
			rt.state.highlightExpected = false
		}
	}

	rt.controller = editorRuntimeController{
		ed:                ed,
		screen:            screen,
		ls:                ls,
		ts:                ts,
		langs:             langs,
		highlightMaxBytes: opts.HighlightMaxBytes,
		sessionMgr:        sessionMgr,
		fileMonitor:       rt.fileMonitor,
		state:             &rt.state,
	}

	return rt, nil
}

func (r *editorRuntime) Close() {
	r.fileMonitor.Close()
}

func (r *editorRuntime) HandleScreenEvent(ev tcell.Event) (bool, bool) {
	return handleScreenEvent(r.screen, r.ed, ev)
}

func (r *editorRuntime) HandleRequests() {
	r.controller.handleEditorRequests()
}

func (r *editorRuntime) Tick() {
	runEditorRuntimeTick(
		r.ed,
		r.ts,
		r.fileMonitor,
		&r.state,
		r.fileChangeDebounce,
		r.filePollInterval,
		&r.lastLayoutRaw,
	)
}
