package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
		KeymapNormal:         cfg.Keymap.Normal,
		KeymapInsert:         cfg.Keymap.Insert,
		CmdHistoryPath:       cmdHistoryPath,
		SearchHistoryPath:    searchHistoryPath,
		SessionStore:         sessionStore,
	})
	defer ed.Shutdown()
	ed.SetStyles(ui.StylesFromConfig(cfg))
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
	var (
		fileWatcher       *fsnotify.Watcher
		fileEvents        chan time.Time
		pendingFileChange bool
		lastFileEvent     time.Time
		lastFileCheck     time.Time
		watchedPath       string
	)
	if openPath != "" {
		absPath := openPath
		if abs, err := filepath.Abs(openPath); err == nil {
			absPath = abs
		}
		watchedPath = absPath
		watchedDir := filepath.Dir(absPath)
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			logger.Warn("file watcher unavailable", "error", err)
		} else if err := watcher.Add(watchedDir); err != nil {
			_ = watcher.Close()
			logger.Warn("file watcher disabled", "error", err)
		} else {
			fileWatcher = watcher
			fileEvents = make(chan time.Time, 1)
			go func() {
				sendEvent := func(ts time.Time) {
					select {
					case fileEvents <- ts:
						return
					default:
					}
					select {
					case <-fileEvents:
					default:
					}
					select {
					case fileEvents <- ts:
					default:
					}
				}
				for {
					select {
					case ev, ok := <-watcher.Events:
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
					case err, ok := <-watcher.Errors:
						if !ok {
							return
						}
						logger.Warn("file watcher error", "error", err)
					}
				}
			}()
		}
	}
	if fileWatcher != nil {
		defer func() { _ = fileWatcher.Close() }()
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
		if change == editor.ExternalChangeModified && !ed.IsDirty() {
			autoReload := true
			if name := ed.Filename(); name != "" {
				if info, err := os.Stat(name); err == nil && info.Size() > autoReloadMaxBytes {
					autoReload = false
				}
			}
			if autoReload {
				if err := ed.ReloadFromDisk(false); err != nil {
					ed.SetStatusMessage("reload failed: " + err.Error())
				} else {
					ed.SetStatusMessage("reloaded from disk")
				}
				return
			}
			if ed.ExternalChange() != change {
				ed.SetExternalChange(change)
				ed.SetStatusMessage("file changed on disk (>8MB, use :e to reload)")
			}
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
		if ts.ParseSync(openPath, langName, ed.Content()) {
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
	screen := ui.WrapScreen(s)
	ed.Render(screen)
	for {
		ev := s.PollEvent()
		isMouseScroll := false
		switch ev := ev.(type) {
		case *tcell.EventKey:
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
		} else if openPath != "" && time.Since(lastFileCheck) > filePollInterval {
			lastFileCheck = time.Now()
			applyExternalChange()
		}
		if openPath != "" && highlightEnabled && langName != "" {
			tick := ed.ChangeTick()
			changed := tick != lastChangeTick
			if changed {
				lastChangeTick = tick
				if edit, ok := ed.ConsumeLastEdit(); ok {
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
			if changed || start != lastHighlightStart || end != lastHighlightEnd {
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
			continue
		}
		ed.Render(screen)
	}
}
