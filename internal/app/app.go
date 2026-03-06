package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/gitinfo"
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

	openFileInEditor := func(path string) error {
		path = strings.TrimSpace(path)
		if path == "" {
			return nil
		}
		absPath := path
		if abs, err := filepath.Abs(path); err == nil {
			absPath = abs
		}
		if runtimeState.openPath == absPath {
			return nil
		}
		if err := ed.OpenFile(absPath); err != nil {
			return err
		}
		state := activateEditorFile(ed, s, ls, ts, langs, absPath, highlightMaxBytes)
		runtimeState.applyActiveFile(state)

		fileMonitor.Watch(absPath)
		ed.SetGitMainBranch("")
		syncEditorRepoState(ed, runtimeState.gitPath, sessionMgr)
		runtimeState.lastGitCheck = time.Now()
		return nil
	}

	switchToWorktree := func(targetPath string) {
		targetPath = strings.TrimSpace(targetPath)
		if targetPath == "" {
			return
		}
		if abs, err := filepath.Abs(targetPath); err == nil {
			targetPath = abs
		}
		candidate := pickWorktreeFile(targetPath, runtimeState.openPath)
		if candidate == "" {
			ed.SetGitBranch(gitinfo.Branch(targetPath))
			ed.SetGitRoot(gitinfo.Root(targetPath))
			runtimeState.gitPath = targetPath
			ed.SetStatusMessage("worktree switched (open a file)")
			return
		}
		if err := openFileInEditor(candidate); err != nil {
			ed.SetStatusMessage(err.Error())
			return
		}
		ed.SetGitBranch(gitinfo.Branch(candidate))
		ed.SetGitRoot(gitinfo.Root(candidate))
		ed.SetStatusMessage("worktree switched")
	}
	screen := ui.WrapScreen(s)
	ed.Render(screen)
	for {
		ev := s.PollEvent()
		isMouseScroll := false
		switch ev := ev.(type) {
		case *tcell.EventKey:
			// Emergency exit: allow Ctrl+C to quit even if editor state is stuck.
			if ev.Key() == tcell.KeyCtrlC {
				return nil
			}
			if ed.HandleKey(ui.WrapKey(ev)) {
				return nil
			}
		case *tcell.EventMouse:
			ed.HandleMouse(ui.WrapMouse(ev))
			isMouseScroll = true
		case *tcell.EventResize:
			s.Sync()
		case *tcell.EventInterrupt:
			// Layout updates are handled below.
		}
		if !isMouseScroll {
			ed.UpdateScroll()
		}
		if ed.ConsumeBranchPickerRequest() {
			showSidebarBranches(ed, runtimeState.gitPath)
		}
		if ed.ConsumeWorktreeListRequest() {
			showSidebarWorktrees(ed, runtimeState.gitPath)
		}
		// Handle sidebar branch selection (and legacy branch picker selection)
		if branch := ed.ConsumeSidebarBranchSelection(); branch != "" {
			logger.Debug("sidebar branch selected", "branch", branch)
			checkoutBranch(ed, runtimeState.gitPath, branch)
		} else if branch, ok := ed.ConsumeBranchSelection(); ok {
			checkoutBranch(ed, runtimeState.gitPath, branch)
		}
		if path := ed.ConsumeSidebarWorktreeSelection(); path != "" {
			logger.Debug("sidebar worktree selected", "path", path)
			if runtimeState.gitPath == "" {
				ed.SetStatusMessage("not a git repository")
			} else {
				switchToWorktree(path)
			}
		}
		if path, ok := ed.ConsumeSidebarOpenFile(); ok {
			err := openFileInEditor(path)
			if err != nil {
				ed.SetStatusMessage(err.Error())
			}
			ed.ApplyPendingGitDiffJump()
			if locPath, line, col, ok := ed.ConsumePendingOpenLocation(); ok {
				if err == nil && (locPath == "" || locPath == path) {
					ed.JumpToLocation(line, col)
				}
			}
		}
		if ed.ConsumeBufferSwitch() {
			path := ed.Filename()
			if path != runtimeState.openPath {
				// Update file watcher
				fileMonitor.Watch(path)
				state := activateEditorFile(ed, s, ls, ts, langs, path, highlightMaxBytes)
				runtimeState.applyActiveFile(state)

				// Update git info
				ed.SetGitBranch(gitinfo.Branch(path))
				gitRoot := gitinfo.Root(path)
				ed.SetGitRoot(gitRoot)
			}
		}
		now := time.Now()
		fileMonitor.ProcessWatcherEvents(now, fileChangeDebounce)
		fileMonitor.HandleAutoReloadResults()
		if runtimeState.openPath != "" {
			fileMonitor.PollExternalChange(now, filePollInterval)
		}
		runtimeState.lastChangeTick, runtimeState.lastHighlightStart, runtimeState.lastHighlightEnd = syncVisibleHighlights(
			ed,
			ts,
			runtimeState.openPath,
			runtimeState.langName,
			runtimeState.highlightEnabled,
			runtimeState.lastChangeTick,
			runtimeState.lastHighlightStart,
			runtimeState.lastHighlightEnd,
		)
		layoutRaw := keyboard.CurrentLayoutRaw()
		if layoutRaw != lastLayoutRaw {
			lastLayoutRaw = layoutRaw
			ed.SetKeyboardLayout(keyboard.CurrentLayout())
		}
		if runtimeState.gitPath != "" && time.Since(runtimeState.lastGitCheck) > 2*time.Second {
			runtimeState.lastGitCheck = time.Now()
			ed.SetGitBranch(gitinfo.Branch(runtimeState.gitPath))
		}
		if runtimeState.highlightExpected && !ed.HasHighlights() {
			if runtimeState.openPath != ed.Filename() {
				runtimeState.highlightExpected = false
			}
		}

		ed.Render(screen)
	}
}
