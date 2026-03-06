package app

import (
	"runtime"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/logger"
	"github.com/kobzarvs/qedit/internal/lsp"
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
	sessionStore := newEditorSessionStore(sessionMgr)
	ed := newConfiguredEditor(&cfg, langs, ts, sessionStore)
	defer ed.Shutdown()
	autoReloadStabilizeDelay := time.Duration(cfg.Editor.AutoReloadStabilizeMS) * time.Millisecond
	if autoReloadStabilizeDelay < 0 {
		autoReloadStabilizeDelay = 0
	}
	rt, err := newEditorRuntime(s, ed, ls, ts, langs, sessionMgr, editorRuntimeOptions{
		InitialPath:              firstArg(a.args),
		HighlightMaxBytes:        cfg.Editor.HighlightMaxBytes,
		AutoReloadMaxBytes:       int64(8 << 20),
		AutoReloadRetries:        cfg.Editor.AutoReloadMaxRetries,
		AutoReloadStabilizeDelay: autoReloadStabilizeDelay,
		FileChangeDebounce:       300 * time.Millisecond,
		FilePollInterval:         time.Second,
	})
	if err != nil {
		return err
	}
	defer rt.Close()
	screen := ui.WrapScreen(s)
	ed.Render(screen)
	for {
		quit, isMouseScroll := rt.HandleScreenEvent(s.PollEvent())
		if quit {
			return nil
		}
		if !isMouseScroll {
			ed.UpdateScroll()
		}
		rt.HandleRequests()
		rt.Tick()

		ed.Render(screen)
	}
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
