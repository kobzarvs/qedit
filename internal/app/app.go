package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gdamore/tcell/v2"
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/gitinfo"
	"github.com/kobzarvs/qedit/internal/integrations"
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
	var sessionStore editor.SessionStore
	if sessionMgr != nil {
		sessionStore = integrations.NewSessionStore(sessionMgr)
	}
	cmdHistoryPath := ""
	searchHistoryPath := ""
	if dir, err := config.ConfigDir(); err == nil {
		cmdHistoryPath = filepath.Join(dir, "history")
		searchHistoryPath = filepath.Join(dir, "search_history")
	}

	highlightMaxBytes := cfg.Editor.HighlightMaxBytes
	ed := editor.New(editor.Options{
		TabWidth:             cfg.Editor.TabWidth,
		LineNumbers:          cfg.Editor.LineNumbers,
		GitBranchSymbol:      cfg.Editor.GitBranchSymbol,
		SidebarWidth:         cfg.Editor.SidebarWidth,
		SidebarMinWidth:      cfg.Editor.SidebarMinWidth,
		SidebarMaxWidth:      cfg.Editor.SidebarMaxWidth,
		SidebarCloseOnSelect: cfg.Editor.SidebarCloseOnSelect,
		FileTreeShowHidden:   cfg.Editor.FileTreeShowHidden,
		FileTreeShowIgnored:  cfg.Editor.FileTreeShowIgnored,
		AutoReloadOnChanges:  cfg.Editor.AutoReloadOnChanges,
		KeymapNormal:         cfg.Keymap.Normal,
		KeymapInsert:         cfg.Keymap.Insert,
		CmdHistoryPath:       cmdHistoryPath,
		SearchHistoryPath:    searchHistoryPath,
		SessionStore:         sessionStore,
	})
	defer ed.Shutdown()
	ed.SetStyles(ui.StylesFromConfig(cfg))
	ed.SetHighlightRangeFunc(func(path string, startLine, endLine int) map[int][]editor.HighlightSpan {
		if path == "" || startLine < 0 || endLine < startLine {
			return nil
		}
		if info, err := os.Stat(path); err != nil {
			return nil
		} else if highlightMaxBytes > 0 && info.Size() > highlightMaxBytes {
			return nil
		}
		lang := langs.Match(path)
		if lang == nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !ts.ParseSync(path, lang.Name, string(data)) {
			return nil
		}
		spans := ts.Highlights(path, startLine, endLine)
		if spans == nil {
			return nil
		}
		editorSpans := make(map[int][]editor.HighlightSpan, len(spans))
		for line, lineSpans := range spans {
			dst := make([]editor.HighlightSpan, len(lineSpans))
			for i, span := range lineSpans {
				dst[i] = editor.HighlightSpan{
					StartCol: span.StartCol,
					EndCol:   span.EndCol,
					Kind:     span.Kind,
				}
			}
			editorSpans[line] = dst
		}
		return editorSpans
	})
	ed.SetAutoReloadConfigHook(func(enabled bool) error {
		if err := config.UpdateEditorAutoReloadOnChanges(enabled); err != nil {
			return err
		}
		cfg.Editor.AutoReloadOnChanges = enabled
		return nil
	})
	ed.SetSidebarWidthConfigHook(func(width string) error {
		if err := config.UpdateEditorSidebarWidth(width); err != nil {
			return err
		}
		cfg.Editor.SidebarWidth = width
		return nil
	})
	ed.SetFormatter(integrations.GoFormatter{})
	if runtime.GOOS == "darwin" {
		ed.SetClipboard(integrations.MacClipboard{})
		ed.SetTerminalZoomer(integrations.TerminalZoomer{})
	}

	ed.LoadCmdHistory()
	ed.LoadSearchHistory()
	gitPath := ""
	var openPath string
	var langName string
	highlightEnabled := true
	highlightExpected := false
	isAsyncParseLang := func(name string) bool {
		switch name {
		case "typescript", "typescriptreact", "javascript", "javascriptreact", "vue":
			return true
		default:
			return false
		}
	}
	if len(a.args) > 0 {
		openPath = a.args[0]
		if err := ed.OpenFile(openPath); err != nil {
			return err
		}
		gitPath = openPath
		if highlightMaxBytes > 0 {
			if info, err := os.Stat(openPath); err == nil && info.Size() > highlightMaxBytes {
				highlightEnabled = false
			}
		}
		content := ed.Content()
		ls.OpenFile(openPath, content)
		if highlightEnabled {
			if lang := langs.Match(openPath); lang != nil {
				langName = lang.Name
			}
		}
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
	type autoReloadResult struct {
		seq  uint64
		data []byte
		err  error
	}
	var (
		fileWatcher       *fsnotify.Watcher
		fileEvents        chan time.Time
		pendingFileChange bool
		lastFileEvent     time.Time
		lastFileCheck     time.Time
		watchedPath       string
		autoReloadSeq     uint64
		autoReloadActive  bool
		autoReloadPending bool
		autoReloadResults chan autoReloadResult
	)
	resetFileWatcher := func(path string) {
		if fileWatcher != nil {
			_ = fileWatcher.Close()
			fileWatcher = nil
		}
		fileEvents = nil
		watchedPath = ""
		pendingFileChange = false
		lastFileEvent = time.Time{}

		if path == "" {
			return
		}
		absPath := path
		if abs, err := filepath.Abs(path); err == nil {
			absPath = abs
		}
		watchedPath = absPath
		watchedDir := filepath.Dir(absPath)
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			logger.Warn("file watcher unavailable", "error", err)
			return
		}
		if err := watcher.Add(watchedDir); err != nil {
			_ = watcher.Close()
			logger.Warn("file watcher disabled", "error", err)
			return
		}
		fileWatcher = watcher
		fileEvents = make(chan time.Time, 1)
		go func(w *fsnotify.Watcher, events chan time.Time) {
			sendEvent := func(ts time.Time) {
				select {
				case events <- ts:
					return
				default:
				}
				select {
				case <-events:
				default:
				}
				select {
				case events <- ts:
				default:
				}
			}
			for {
				select {
				case ev, ok := <-w.Events:
					if !ok {
						return
					}
					if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
						continue
					}
					if filepath.Clean(ev.Name) != watchedPath {
						continue
					}
					sendEvent(time.Now())
					_ = s.PostEvent(tcell.NewEventInterrupt(nil))
				case err, ok := <-w.Errors:
					if !ok {
						return
					}
					logger.Warn("file watcher error", "error", err)
				}
			}
		}(watcher, fileEvents)
	}
	if openPath != "" {
		resetFileWatcher(openPath)
	}
	if fileWatcher != nil {
		defer func() { _ = fileWatcher.Close() }()
	}
	if openPath != "" {
		autoReloadResults = make(chan autoReloadResult, 1)
	}
	readFileStable := func(path string, attempts int, delay time.Duration) ([]byte, error) {
		var lastErr error
		for i := 0; i < attempts; i++ {
			infoBefore, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			time.Sleep(delay)
			infoMid, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			if !infoBefore.ModTime().Equal(infoMid.ModTime()) || infoBefore.Size() != infoMid.Size() {
				lastErr = fmt.Errorf("file changed during read")
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				lastErr = err
				time.Sleep(delay)
				continue
			}
			infoAfter, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			if infoMid.ModTime().Equal(infoAfter.ModTime()) && infoMid.Size() == infoAfter.Size() && int64(len(data)) == infoAfter.Size() {
				return data, nil
			}
			lastErr = fmt.Errorf("file changed during read")
			time.Sleep(delay)
		}
		return nil, lastErr
	}
	startAutoReload := func() {
		if watchedPath == "" {
			return
		}
		if autoReloadActive {
			autoReloadPending = true
			return
		}
		autoReloadActive = true
		autoReloadPending = false
		autoReloadSeq++
		seq := autoReloadSeq
		ed.SetAutoReloadInProgress(true)
		go func(path string, currentSeq uint64) {
			data, err := readFileStable(path, autoReloadRetries, autoReloadStabilizeDelay)
			select {
			case autoReloadResults <- autoReloadResult{seq: currentSeq, data: data, err: err}:
			default:
				select {
				case <-autoReloadResults:
				default:
				}
				autoReloadResults <- autoReloadResult{seq: currentSeq, data: data, err: err}
			}
			_ = s.PostEvent(tcell.NewEventInterrupt(nil))
		}(watchedPath, seq)
	}
	applyExternalChange := func() {
		change, err := ed.CheckExternalChange()
		if err != nil {
			logger.Error("file change check failed", "error", err)
			return
		}
		if change == editor.ExternalChangeNone {
			if ed.ExternalChange() != editor.ExternalChangeNone {
				ed.ClearExternalChange()
			}
			return
		}
		if change == editor.ExternalChangeModified {
			if ed.AutoReloadOnChanges() {
				ed.ClearExternalChange()
				startAutoReload()
				return
			}
			largeFile := false
			if name := ed.Filename(); name != "" {
				if info, err := os.Stat(name); err == nil && info.Size() > autoReloadMaxBytes {
					largeFile = true
				}
			}
			if ed.ExternalChange() != change {
				ed.SetExternalChange(change)
				msg := "file changed on disk (use :e to reload)"
				if ed.HasLocalChanges() {
					msg = "file changed on disk (use :e! to reload)"
				}
				if largeFile {
					msg = "large file changed on disk (use :e to reload)"
					if ed.HasLocalChanges() {
						msg = "large file changed on disk (use :e! to reload)"
					}
				}
				ed.SetStatusMessage(msg)
			}
			ed.MarkExternalDirty()
			return
		}
		if ed.ExternalChange() != change {
			ed.SetExternalChange(change)
			switch change {
			case editor.ExternalChangeDeleted:
				ed.SetStatusMessage("file deleted on disk")
			default:
				ed.SetStatusMessage("file changed on disk (use :e! to reload)")
			}
		}
	}

	lastLayoutRaw := keyboard.CurrentLayoutRaw()
	ed.SetKeyboardLayout(keyboard.CurrentLayout())
	ed.SetGitBranch(gitinfo.Branch(gitPath))

	// Determine main branch (from session cache or git)
	gitRoot := gitinfo.Root(gitPath)
	ed.SetGitRoot(gitRoot)
	if gitRoot != "" {
		sm := sessionMgr
		var mainBranch string
		// Try session cache first
		if sm != nil {
			if repoInfo, ok := sm.GetRepoInfo(gitRoot); ok && repoInfo.MainBranch != "" {
				mainBranch = repoInfo.MainBranch
			}
		}
		// If not cached, detect synchronously (fast for local repos)
		if mainBranch == "" {
			mainBranch = gitinfo.MainBranch(gitPath)
			// Save to cache for next time
			if mainBranch != "" && sm != nil {
				sm.SetRepoMainBranch(gitRoot, mainBranch)
			}
		}
		if mainBranch != "" {
			ed.SetGitMainBranch(mainBranch)
		}
	}

	// Wire up tree-sitter node stack callback for expand/shrink selection
	ed.SetNodeStackFunc(func(path string, row, col int) []editor.NodeRange {
		stack := ts.GetNodeStackAt(path, row, col)
		if stack == nil {
			return nil
		}
		result := make([]editor.NodeRange, len(stack))
		for i, nr := range stack {
			result[i] = editor.NodeRange{
				StartRow: nr.StartRow,
				StartCol: nr.StartCol,
				EndRow:   nr.EndRow,
				EndCol:   nr.EndCol,
			}
		}
		return result
	})

	// Wire up LSP goto callback for definition, references, etc.
	ed.SetLSPGotoFunc(func(method, path string, line, col int) ([]editor.LSPLocation, error) {
		// Ensure we use absolute path (same as LSP OpenFile)
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}
		var locs []lsp.Location
		switch method {
		case "definition":
			locs, err = ls.GotoDefinition(absPath, line, col)
		case "declaration":
			locs, err = ls.GotoDeclaration(absPath, line, col)
		case "typeDefinition":
			locs, err = ls.GotoTypeDefinition(absPath, line, col)
		case "references":
			locs, err = ls.FindReferences(absPath, line, col)
		case "implementation":
			locs, err = ls.GotoImplementation(absPath, line, col)
		default:
			return nil, fmt.Errorf("unknown LSP method: %s", method)
		}
		if err != nil {
			return nil, err
		}
		result := make([]editor.LSPLocation, len(locs))
		for i, loc := range locs {
			result[i] = editor.LSPLocation{
				Path:      lsp.URIToPath(loc.URI),
				StartLine: loc.Range.Start.Line,
				StartCol:  loc.Range.Start.Character,
				EndLine:   loc.Range.End.Line,
				EndCol:    loc.Range.End.Character,
			}
		}
		return result, nil
	})
	lastGitCheck := time.Now()
	lastChangeTick := ed.ChangeTick()
	lastHighlightStart := -1
	lastHighlightEnd := -1
	if openPath != "" && highlightEnabled && langName != "" {
		if isAsyncParseLang(langName) {
			ts.Parse(openPath, langName, ed.Content())
		} else if ts.ParseSync(openPath, langName, ed.Content()) {
			_, h := s.Size()
			viewHeight := h - 2
			if viewHeight < 0 {
				viewHeight = 0
			}
			end := viewHeight - 1
			if end < 0 {
				end = 0
			}
			lineCount := ed.LineCount()
			if lineCount > 0 && end >= lineCount {
				end = lineCount - 1
			}
			spans := ts.Highlights(openPath, 0, end)
			if spans != nil {
				editorSpans := make(map[int][]editor.HighlightSpan, len(spans))
				for line, lineSpans := range spans {
					dst := make([]editor.HighlightSpan, len(lineSpans))
					for i, span := range lineSpans {
						dst[i] = editor.HighlightSpan{
							StartCol: span.StartCol,
							EndCol:   span.EndCol,
							Kind:     span.Kind,
						}
					}
					editorSpans[line] = dst
				}
				ed.SetHighlights(0, end, editorSpans)
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
		openPath = absPath
		gitPath = absPath
		highlightEnabled = true
		if highlightMaxBytes > 0 {
			if info, err := os.Stat(absPath); err == nil && info.Size() > highlightMaxBytes {
				highlightEnabled = false
			}
		}
		content := ed.Content()
		ls.OpenFile(absPath, content)
		langName = ""
		if highlightEnabled {
			if lang := langs.Match(absPath); lang != nil {
				langName = lang.Name
			}
		}
		highlightExpected = highlightEnabled && langName != ""

		lastChangeTick = ed.ChangeTick()
		lastHighlightStart = -1
		lastHighlightEnd = -1
		if highlightEnabled && langName != "" {
			if isAsyncParseLang(langName) {
				ts.Parse(absPath, langName, ed.Content())
			} else if ts.ParseSync(absPath, langName, ed.Content()) {
				_, h := s.Size()
				viewHeight := h - 2
				if viewHeight < 0 {
					viewHeight = 0
				}
				end := viewHeight - 1
				if end < 0 {
					end = 0
				}
				lineCount := ed.LineCount()
				if lineCount > 0 && end >= lineCount {
					end = lineCount - 1
				}
				spans := ts.Highlights(absPath, 0, end)
				if spans != nil {
					editorSpans := make(map[int][]editor.HighlightSpan, len(spans))
					for line, lineSpans := range spans {
						dst := make([]editor.HighlightSpan, len(lineSpans))
						for i, span := range lineSpans {
							dst[i] = editor.HighlightSpan{
								StartCol: span.StartCol,
								EndCol:   span.EndCol,
								Kind:     span.Kind,
							}
						}
						editorSpans[line] = dst
					}
					ed.SetHighlights(0, end, editorSpans)
					lastHighlightStart = 0
					lastHighlightEnd = end
				}
			} else {
				highlightExpected = false
				ed.SetHighlights(-1, -1, nil)
			}
		} else {
			ed.SetHighlights(-1, -1, nil)
		}

		resetFileWatcher(absPath)
		if autoReloadResults == nil {
			autoReloadResults = make(chan autoReloadResult, 1)
		}
		ed.SetGitBranch(gitinfo.Branch(gitPath))
		ed.SetGitMainBranch("")
		gitRoot := gitinfo.Root(gitPath)
		ed.SetGitRoot(gitRoot)
		if gitRoot != "" {
			sm := sessionMgr
			var mainBranch string
			if sm != nil {
				if repoInfo, ok := sm.GetRepoInfo(gitRoot); ok && repoInfo.MainBranch != "" {
					mainBranch = repoInfo.MainBranch
				}
			}
			if mainBranch == "" {
				mainBranch = gitinfo.MainBranch(gitPath)
				if mainBranch != "" && sm != nil {
					sm.SetRepoMainBranch(gitRoot, mainBranch)
				}
			}
			if mainBranch != "" {
				ed.SetGitMainBranch(mainBranch)
			}
		}
		lastGitCheck = time.Now()
		return nil
	}

	pickWorktreeFile := func(targetRoot string) string {
		targetRoot = strings.TrimSpace(targetRoot)
		if targetRoot == "" {
			return ""
		}
		if abs, err := filepath.Abs(targetRoot); err == nil {
			targetRoot = abs
		}
		// Try to open the same relative file in the target worktree.
		if openPath != "" {
			currentRoot := gitinfo.Root(openPath)
			if currentRoot != "" {
				if rel, err := filepath.Rel(currentRoot, openPath); err == nil && !strings.HasPrefix(rel, "..") {
					candidate := filepath.Join(targetRoot, rel)
					if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
						return candidate
					}
				}
			}
		}
		// Try common entry files.
		for _, name := range []string{"README.md", "README", "go.mod", "package.json"} {
			candidate := filepath.Join(targetRoot, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
		// Try first regular file in root.
		if entries, err := os.ReadDir(targetRoot); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				candidate := filepath.Join(targetRoot, entry.Name())
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					return candidate
				}
			}
			// Try one level deep.
			for _, entry := range entries {
				if !entry.IsDir() || entry.Name() == ".git" {
					continue
				}
				dirPath := filepath.Join(targetRoot, entry.Name())
				child, err := os.ReadDir(dirPath)
				if err != nil {
					continue
				}
				for _, c := range child {
					if c.IsDir() {
						continue
					}
					candidate := filepath.Join(dirPath, c.Name())
					if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
						return candidate
					}
				}
			}
		}
		return ""
	}

	switchToWorktree := func(targetPath string) {
		targetPath = strings.TrimSpace(targetPath)
		if targetPath == "" {
			return
		}
		if abs, err := filepath.Abs(targetPath); err == nil {
			targetPath = abs
		}
		candidate := pickWorktreeFile(targetPath)
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
			logger.Debug("branch picker requested")
			if gitPath == "" {
				logger.Debug("not a git repository")
				ed.SetStatusMessage("not a git repository")
			} else {
				branches, current, err := gitinfo.ListBranches(gitPath)
				if err != nil {
					logger.Error("failed to list branches", "error", err)
					ed.SetStatusMessage(err.Error())
				} else {
					logger.Debug("showing sidebar branches", "count", len(branches), "current", current)
					ed.ShowSidebarBranches(branches, current)
				}
			}
		}
		if ed.ConsumeWorktreeListRequest() {
			logger.Debug("worktree list requested")
			if gitPath == "" {
				logger.Debug("not a git repository")
				ed.SetStatusMessage("not a git repository")
			} else {
				worktrees, active, err := gitinfo.ListWorktrees(gitPath)
				if err != nil {
					logger.Error("failed to list worktrees", "error", err)
					ed.SetStatusMessage(err.Error())
				} else {
					items := make([]editor.WorktreeInfo, 0, len(worktrees))
					for _, wt := range worktrees {
						items = append(items, editor.WorktreeInfo{
							Path:   wt.Path,
							Branch: wt.Branch,
						})
					}
					ed.ShowSidebarWorktrees(items, active)
				}
			}
		}
		// Handle sidebar branch selection (and legacy branch picker selection)
		if branch := ed.ConsumeSidebarBranchSelection(); branch != "" {
			logger.Debug("sidebar branch selected", "branch", branch)
			if gitPath == "" {
				ed.SetStatusMessage("not a git repository")
			} else if err := gitinfo.Checkout(gitPath, branch); err != nil {
				logger.Error("failed to checkout branch", "branch", branch, "error", err)
				ed.SetStatusMessage(err.Error())
			} else {
				ed.SetGitBranch(branch)
				ed.SetStatusMessage("checked out " + branch)
			}
		} else if branch, ok := ed.ConsumeBranchSelection(); ok {
			if gitPath == "" {
				ed.SetStatusMessage("not a git repository")
			} else if err := gitinfo.Checkout(gitPath, branch); err != nil {
				ed.SetStatusMessage(err.Error())
			} else {
				ed.SetGitBranch(branch)
				ed.SetStatusMessage("checked out " + branch)
			}
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
				openPath = path
				gitPath = path

				// Update file watcher
				resetFileWatcher(path)

				// LSP: inform about open file
				content := ed.Content()
				ls.OpenFile(path, content)

				// Re-detect language
				langName = ""
				highlightEnabled = true
				if highlightMaxBytes > 0 {
					if info, err := os.Stat(path); err == nil && info.Size() > highlightMaxBytes {
						highlightEnabled = false
					}
				}
				if highlightEnabled {
					if lang := langs.Match(path); lang != nil {
						langName = lang.Name
					}
				}
				highlightExpected = highlightEnabled && langName != ""

				// Re-parse tree-sitter
				if highlightEnabled && langName != "" {
					if isAsyncParseLang(langName) {
						ts.Parse(path, langName, ed.Content())
					} else if ts.ParseSync(path, langName, ed.Content()) {
						_, h := s.Size()
						viewHeight := h - 2
						if viewHeight < 0 {
							viewHeight = 0
						}
						end := viewHeight - 1
						if end < 0 {
							end = 0
						}
						lineCount := ed.LineCount()
						if lineCount > 0 && end >= lineCount {
							end = lineCount - 1
						}
						spans := ts.Highlights(path, 0, end)
						if spans != nil {
							editorSpans := make(map[int][]editor.HighlightSpan, len(spans))
							for line, lineSpans := range spans {
								dst := make([]editor.HighlightSpan, len(lineSpans))
								for i, span := range lineSpans {
									dst[i] = editor.HighlightSpan{
										StartCol: span.StartCol,
										EndCol:   span.EndCol,
										Kind:     span.Kind,
									}
								}
								editorSpans[line] = dst
							}
							ed.SetHighlights(0, end, editorSpans)
						}
					} else {
						highlightExpected = false
						ed.SetHighlights(-1, -1, nil)
					}
				} else {
					ed.SetHighlights(-1, -1, nil)
				}

				// Update git info
				ed.SetGitBranch(gitinfo.Branch(path))
				gitRoot := gitinfo.Root(path)
				ed.SetGitRoot(gitRoot)

				lastChangeTick = ed.ChangeTick()
				lastHighlightStart = -1
				lastHighlightEnd = -1

				if autoReloadResults == nil {
					autoReloadResults = make(chan autoReloadResult, 1)
				}
			}
		}
		if fileWatcher != nil {
			for {
				select {
				case ts := <-fileEvents:
					pendingFileChange = true
					if ts.After(lastFileEvent) {
						lastFileEvent = ts
					}
				default:
					goto doneFileEvents
				}
			}
		doneFileEvents:
			if pendingFileChange && time.Since(lastFileEvent) >= fileChangeDebounce {
				pendingFileChange = false
				applyExternalChange()
			}
		}
		if autoReloadActive {
			select {
			case res := <-autoReloadResults:
				if res.seq == autoReloadSeq {
					autoReloadActive = false
					ed.SetAutoReloadInProgress(false)
					if res.err != nil {
						ed.SetExternalChange(editor.ExternalChangeModified)
						ed.SetStatusMessage("auto reload failed: " + res.err.Error())
					} else if conflict, err := ed.MergeExternalContent(string(res.data)); err != nil {
						ed.SetExternalChange(editor.ExternalChangeModified)
						ed.SetStatusMessage("auto reload failed: " + err.Error())
					} else if conflict {
						ed.SetStatusMessage("auto reload merged with conflicts")
					} else {
						ed.SetStatusMessage("auto reload complete")
					}
				}
				if autoReloadPending && ed.AutoReloadOnChanges() {
					autoReloadPending = false
					startAutoReload()
				}
			default:
			}
		}
		if openPath != "" && time.Since(lastFileCheck) > filePollInterval {
			lastFileCheck = time.Now()
			applyExternalChange()
		}
		// Drain tree-sitter async parse events
		tsParsed := false
	drainTS:
		for {
			select {
			case evt := <-ts.Events():
				if evt.Kind == "parsed" && evt.Path == openPath {
					tsParsed = true
				}
			default:
				break drainTS
			}
		}
		if openPath != "" && highlightEnabled && langName != "" {
			tick := ed.ChangeTick()
			changed := tick != lastChangeTick
			asyncChanged := false
			if changed {
				lastChangeTick = tick
				if isAsyncParseLang(langName) {
					// Adjust existing highlights to match the edit before async reparse
					if edit, ok := ed.PeekLastEdit(); ok {
						ed.AdjustHighlights(edit.StartRow, edit.OldEndRow, edit.NewEndRow)
					}
					ts.Parse(openPath, langName, ed.Content())
					asyncChanged = true
				} else if edit, ok := ed.ConsumeLastEdit(); ok {
					tsEdit := sitter.EditInput{
						StartIndex:  uint32(edit.StartByte),
						OldEndIndex: uint32(edit.OldEndByte),
						NewEndIndex: uint32(edit.NewEndByte),
						StartPoint: sitter.Point{
							Row:    uint32(edit.StartRow),
							Column: uint32(edit.StartColBytes),
						},
						OldEndPoint: sitter.Point{
							Row:    uint32(edit.OldEndRow),
							Column: uint32(edit.OldEndColBytes),
						},
						NewEndPoint: sitter.Point{
							Row:    uint32(edit.NewEndRow),
							Column: uint32(edit.NewEndColBytes),
						},
					}
					ts.ParseSyncEdit(openPath, langName, ed.Content(), &tsEdit)
				} else {
					ts.ParseSync(openPath, langName, ed.Content())
				}
			}
			start, end := ed.VisibleRange()
			if asyncChanged && !tsParsed {
				// Async reparse queued but not done yet — keep current highlights as-is
			} else if changed || tsParsed || start != lastHighlightStart || end != lastHighlightEnd {
				spans := ts.Highlights(openPath, start, end)
				if spans != nil {
					editorSpans := make(map[int][]editor.HighlightSpan, len(spans))
					for line, lineSpans := range spans {
						dst := make([]editor.HighlightSpan, len(lineSpans))
						for i, span := range lineSpans {
							dst[i] = editor.HighlightSpan{
								StartCol: span.StartCol,
								EndCol:   span.EndCol,
								Kind:     span.Kind,
							}
						}
						editorSpans[line] = dst
					}
					ed.SetHighlights(start, end, editorSpans)
					lastHighlightStart = start
					lastHighlightEnd = end
				} else {
					ed.SetHighlights(-1, -1, nil)
					lastHighlightStart = -1
					lastHighlightEnd = -1
				}
			}
		} else if openPath != "" {
			ed.SetHighlights(-1, -1, nil)
		}
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
