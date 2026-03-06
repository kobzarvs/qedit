package app

import (
	"runtime"
	"time"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/integrations"
	"github.com/kobzarvs/qedit/internal/logger"
	"github.com/kobzarvs/qedit/internal/session"
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
	services, err := newAppServices(langs)
	if err != nil {
		return err
	}
	defer services.Close()

	sessionMgr, _ := session.NewManager()
	sessionStore := newEditorSessionStore(sessionMgr)
	fileStore := integrations.FileStore{}
	ed := newConfiguredEditor(&cfg, langs, services.ts, sessionStore, fileStore)
	defer ed.Shutdown()
	autoReloadStabilizeDelay := time.Duration(cfg.Editor.AutoReloadStabilizeMS) * time.Millisecond
	if autoReloadStabilizeDelay < 0 {
		autoReloadStabilizeDelay = 0
	}
	rt, err := newEditorRuntime(services.screen, ed, services.ls, services.ts, langs, sessionMgr, fileStore, editorRuntimeOptions{
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
	return rt.Run()
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
