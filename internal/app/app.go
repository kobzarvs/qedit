package app

import (
	"os"
	"runtime"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/logger"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/platform/keyboard"
	"github.com/kobzarvs/qedit/internal/session"
	"github.com/kobzarvs/qedit/internal/treesitter"
	"github.com/kobzarvs/qedit/internal/ui"
)

// App is the top-level runtime for qedit.
type App struct {
	args []string
}

func New(args []string) *App {
	return &App{args: args}
}

func (a *App) Run() error {
	runtime.LockOSThread()
	logger.Debug("app.Run started")

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		return err
	}
	logger.Debug("config loaded")
	langs, err := config.LoadLanguages()
	if err != nil {
		return err
	}

	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := s.Init(); err != nil {
		return err
	}
	s.EnableMouse()
	defer s.Fini()

	ls := lsp.NewManager(langs)
	if err := ls.Start(); err != nil {
		return err
	}
	defer func() { _ = ls.Stop() }()

	ts := treesitter.New(langs)
	if err := ts.Start(); err != nil {
		return err
	}
	defer func() { _ = ts.Stop() }()

	stopLayout := make(chan struct{})
	defer close(stopLayout)
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopLayout:
				return
			case <-ticker.C:
				_ = s.PostEvent(tcell.NewEventInterrupt(nil))
			}
		}
	}()

	sessionMgr, _ := session.NewManager()
	highlightMaxBytes := cfg.Editor.HighlightMaxBytes
	sessionStore := newEditorSessionStore(sessionMgr)
	ed := newConfiguredEditor(&cfg, langs, ts, sessionStore)
	defer ed.Shutdown()
	runtimeState := newEditorRuntimeState(ed)
	if len(a.args) > 0 {
		runtimeState.openPath = a.args[0]
		if err := ed.OpenFile(runtimeState.openPath); err != nil {
			return err
		}
		runtimeState.gitPath = runtimeState.openPath
		content := ed.Content()
		ls.OpenFile(runtimeState.openPath, content)
		runtimeState.langName, runtimeState.highlightEnabled = detectHighlightLanguage(runtimeState.openPath, langs, highlightMaxBytes)
		runtimeState.highlightExpected = runtimeState.highlightEnabled && runtimeState.langName != ""
	}
	if runtimeState.gitPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			runtimeState.gitPath = cwd
		}
	}

	const (
		fileChangeDebounce = 300 * time.Millisecond
		autoReloadMaxBytes = int64(8 << 20)
		filePollInterval   = time.Second
	)
	autoReloadStabilizeDelay := time.Duration(cfg.Editor.AutoReloadStabilizeMS) * time.Millisecond
	if autoReloadStabilizeDelay < 0 {
		autoReloadStabilizeDelay = 0
	}
	autoReloadRetries := cfg.Editor.AutoReloadMaxRetries
	if autoReloadRetries < 1 {
		autoReloadRetries = 1
	}
	fileMonitor := newExternalFileMonitor(s, ed, autoReloadMaxBytes, autoReloadRetries, autoReloadStabilizeDelay)
	if runtimeState.openPath != "" {
		fileMonitor.Watch(runtimeState.openPath)
	}
	defer fileMonitor.Close()

	lastLayoutRaw := keyboard.CurrentLayoutRaw()
	ed.SetKeyboardLayout(keyboard.CurrentLayout())
	syncEditorRepoState(ed, runtimeState.gitPath, sessionMgr)
	wireEditorRuntimeCallbacks(ed, ts, ls)
	if runtimeState.openPath != "" && runtimeState.highlightEnabled && runtimeState.langName != "" {
		if isAsyncParseLang(runtimeState.langName) {
			ts.Parse(runtimeState.openPath, runtimeState.langName, ed.Content())
		} else if ts.ParseSync(runtimeState.openPath, runtimeState.langName, ed.Content()) {
			if _, end, ok := applyInitialScreenHighlights(ed, s, ts, runtimeState.openPath); ok {
				runtimeState.lastHighlightStart = 0
				runtimeState.lastHighlightEnd = end
			}
		} else {
			runtimeState.highlightExpected = false
		}
	}
	controller := editorRuntimeController{
		ed:                ed,
		screen:            s,
		ls:                ls,
		ts:                ts,
		langs:             langs,
		highlightMaxBytes: highlightMaxBytes,
		sessionMgr:        sessionMgr,
		fileMonitor:       fileMonitor,
		state:             &runtimeState,
	}
	screen := ui.WrapScreen(s)
	ed.Render(screen)
	for {
		quit, isMouseScroll := handleScreenEvent(s, ed, s.PollEvent())
		if quit {
			return nil
		}
		if !isMouseScroll {
			ed.UpdateScroll()
		}
		controller.handleEditorRequests()
		runEditorRuntimeTick(
			ed,
			ts,
			fileMonitor,
			&runtimeState,
			fileChangeDebounce,
			filePollInterval,
			&lastLayoutRaw,
		)

		ed.Render(screen)
	}
}
