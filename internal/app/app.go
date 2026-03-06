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
	gitPath := ""
	var openPath string
	var langName string
	highlightEnabled := true
	highlightExpected := false
	if len(a.args) > 0 {
		openPath = a.args[0]
		if err := ed.OpenFile(openPath); err != nil {
			return err
		}
		gitPath = openPath
		content := ed.Content()
		ls.OpenFile(openPath, content)
		langName, highlightEnabled = detectHighlightLanguage(openPath, langs, highlightMaxBytes)
		highlightExpected = highlightEnabled && langName != ""
	}
	if gitPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			gitPath = cwd
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
	if openPath != "" {
		fileMonitor.Watch(openPath)
	}
	defer fileMonitor.Close()

	lastLayoutRaw := keyboard.CurrentLayoutRaw()
	ed.SetKeyboardLayout(keyboard.CurrentLayout())
	syncEditorRepoState(ed, gitPath, sessionMgr)
	wireEditorRuntimeCallbacks(ed, ts, ls)
	lastGitCheck := time.Now()
	lastChangeTick := ed.ChangeTick()
	lastHighlightStart := -1
	lastHighlightEnd := -1
	if openPath != "" && highlightEnabled && langName != "" {
		if isAsyncParseLang(langName) {
			ts.Parse(openPath, langName, ed.Content())
		} else if ts.ParseSync(openPath, langName, ed.Content()) {
			if _, end, ok := applyInitialScreenHighlights(ed, s, ts, openPath); ok {
				lastHighlightStart = 0
				lastHighlightEnd = end
			}
		} else {
			highlightExpected = false
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
		if openPath == absPath {
			return nil
		}
		if err := ed.OpenFile(absPath); err != nil {
			return err
		}
		state := activateEditorFile(ed, s, ls, ts, langs, absPath, highlightMaxBytes)
		openPath = state.openPath
		gitPath = state.gitPath
		langName = state.langName
		highlightEnabled = state.highlightEnabled
		highlightExpected = state.highlightExpected
		lastChangeTick = state.lastChangeTick
		lastHighlightStart = state.lastHighlightStart
		lastHighlightEnd = state.lastHighlightEnd

		fileMonitor.Watch(absPath)
		ed.SetGitMainBranch("")
		syncEditorRepoState(ed, gitPath, sessionMgr)
		lastGitCheck = time.Now()
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
		candidate := pickWorktreeFile(targetPath, openPath)
		if candidate == "" {
			ed.SetGitBranch(gitinfo.Branch(targetPath))
			ed.SetGitRoot(gitinfo.Root(targetPath))
			gitPath = targetPath
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
			showSidebarBranches(ed, gitPath)
		}
		if ed.ConsumeWorktreeListRequest() {
			showSidebarWorktrees(ed, gitPath)
		}
		// Handle sidebar branch selection (and legacy branch picker selection)
		if branch := ed.ConsumeSidebarBranchSelection(); branch != "" {
			logger.Debug("sidebar branch selected", "branch", branch)
			checkoutBranch(ed, gitPath, branch)
		} else if branch, ok := ed.ConsumeBranchSelection(); ok {
			checkoutBranch(ed, gitPath, branch)
		}
		if path := ed.ConsumeSidebarWorktreeSelection(); path != "" {
			logger.Debug("sidebar worktree selected", "path", path)
			if gitPath == "" {
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
			if path != openPath {
				// Update file watcher
				fileMonitor.Watch(path)
				state := activateEditorFile(ed, s, ls, ts, langs, path, highlightMaxBytes)
				openPath = state.openPath
				gitPath = state.gitPath
				langName = state.langName
				highlightEnabled = state.highlightEnabled
				highlightExpected = state.highlightExpected

				// Update git info
				ed.SetGitBranch(gitinfo.Branch(path))
				gitRoot := gitinfo.Root(path)
				ed.SetGitRoot(gitRoot)

				lastChangeTick = state.lastChangeTick
				lastHighlightStart = -1
				lastHighlightEnd = -1
			}
		}
		now := time.Now()
		fileMonitor.ProcessWatcherEvents(now, fileChangeDebounce)
		fileMonitor.HandleAutoReloadResults()
		if openPath != "" {
			fileMonitor.PollExternalChange(now, filePollInterval)
		}
		lastChangeTick, lastHighlightStart, lastHighlightEnd = syncVisibleHighlights(
			ed,
			ts,
			openPath,
			langName,
			highlightEnabled,
			lastChangeTick,
			lastHighlightStart,
			lastHighlightEnd,
		)
		layoutRaw := keyboard.CurrentLayoutRaw()
		if layoutRaw != lastLayoutRaw {
			lastLayoutRaw = layoutRaw
			ed.SetKeyboardLayout(keyboard.CurrentLayout())
		}
		if gitPath != "" && time.Since(lastGitCheck) > 2*time.Second {
			lastGitCheck = time.Now()
			ed.SetGitBranch(gitinfo.Branch(gitPath))
		}
		if highlightExpected && !ed.HasHighlights() {
			if openPath != ed.Filename() {
				highlightExpected = false
			}
		}

		ed.Render(screen)
	}
}
