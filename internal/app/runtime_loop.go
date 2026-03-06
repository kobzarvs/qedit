package app

import (
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/platform/keyboard"
	"github.com/kobzarvs/qedit/internal/session"
	"github.com/kobzarvs/qedit/internal/treesitter"
	"github.com/kobzarvs/qedit/internal/ui"
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
	renderScreen       editor.Screen
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
	cfg *config.Config,
	screen tcell.Screen,
	ed *editor.Editor,
	ls *lsp.Manager,
	ts *treesitter.Engine,
	langs config.Languages,
	sessionMgr *session.Manager,
	fileStore editor.FileStore,
	opts editorRuntimeOptions,
) (*editorRuntime, error) {
	autoReloadRetries := opts.AutoReloadRetries
	if autoReloadRetries < 1 {
		autoReloadRetries = 1
	}

	rt := &editorRuntime{
		screen:             screen,
		renderScreen:       ui.WrapScreen(screen),
		ed:                 ed,
		ts:                 ts,
		state:              newEditorRuntimeState(ed),
		lastLayoutRaw:      keyboard.CurrentLayoutRaw(),
		fileChangeDebounce: opts.FileChangeDebounce,
		filePollInterval:   opts.FilePollInterval,
	}

	if opts.InitialPath != "" {
		state, err := openRuntimeFile(ed, screen, ls, ts, langs, fileStore, opts.InitialPath, opts.HighlightMaxBytes)
		if err != nil {
			return nil, err
		}
		rt.state.applyActiveFile(state)
	}
	if rt.state.gitPath == "" {
		if cwd := normalizeAppPath(fileStore, "."); cwd != "" {
			rt.state.gitPath = cwd
		}
	}

	rt.fileMonitor = newExternalFileMonitor(screen, ed, fileStore, opts.AutoReloadMaxBytes, autoReloadRetries, opts.AutoReloadStabilizeDelay)
	if rt.state.openPath != "" {
		rt.fileMonitor.Watch(rt.state.openPath)
	}

	ed.SetKeyboardLayout(keyboard.CurrentLayout())
	syncEditorRepoState(ed, rt.state.gitPath, sessionMgr)

	rt.controller = editorRuntimeController{
		ed:                ed,
		cfg:               cfg,
		screen:            screen,
		ls:                ls,
		ts:                ts,
		langs:             langs,
		highlightMaxBytes: opts.HighlightMaxBytes,
		sessionMgr:        sessionMgr,
		fileMonitor:       rt.fileMonitor,
		fileStore:         fileStore,
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

func (r *editorRuntime) Run() error {
	r.ed.Render(r.renderScreen)
	for {
		quit, isMouseScroll := r.HandleScreenEvent(r.screen.PollEvent())
		if quit {
			return nil
		}
		if !isMouseScroll {
			r.ed.UpdateScroll()
		}
		r.HandleRequests()
		r.Tick()
		r.ed.Render(r.renderScreen)
	}
}
