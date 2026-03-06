package app

import (
	"path/filepath"
	"runtime"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/gitinfo"
	"github.com/kobzarvs/qedit/internal/integrations"
	"github.com/kobzarvs/qedit/internal/session"
	"github.com/kobzarvs/qedit/internal/treesitter"
	"github.com/kobzarvs/qedit/internal/ui"
)

func newEditorSessionStore(sessionMgr *session.Manager) editor.SessionStore {
	if sessionMgr == nil {
		return nil
	}
	return integrations.NewSessionStore(sessionMgr)
}

func editorHistoryPaths() (string, string) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", ""
	}
	return filepath.Join(dir, "history"), filepath.Join(dir, "search_history")
}

func newConfiguredEditor(cfg *config.Config, sessionStore editor.SessionStore, fileStore editor.FileStore, languageRuntime editor.LanguageRuntime) *editor.Editor {
	cmdHistoryPath, searchHistoryPath := editorHistoryPaths()
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

	ed.SetStyles(ui.StylesFromConfig(*cfg))
	runtimeServices := editor.RuntimeServices{
		Formatter:       integrations.GoFormatter{},
		Merger:          integrations.GitMerger{},
		SessionStore:    sessionStore,
		HistoryStore:    integrations.FileHistoryStore{},
		FileStore:       fileStore,
		UndoStore:       integrations.FileUndoStore{},
		LanguageRuntime: languageRuntime,
	}
	if runtime.GOOS == "darwin" {
		runtimeServices.SystemClipboard = integrations.MacClipboard{}
		runtimeServices.TerminalZoomer = integrations.TerminalZoomer{}
	}
	ed.ApplyRuntimeServices(runtimeServices)

	ed.LoadCmdHistory()
	ed.LoadSearchHistory()
	return ed
}

func persistEditorAutoReload(cfg *config.Config, enabled bool) error {
	if err := config.UpdateEditorAutoReloadOnChanges(enabled); err != nil {
		return err
	}
	if cfg != nil {
		cfg.Editor.AutoReloadOnChanges = enabled
	}
	return nil
}

func persistEditorSidebarWidth(cfg *config.Config, width string) error {
	if err := config.UpdateEditorSidebarWidth(width); err != nil {
		return err
	}
	if cfg != nil {
		cfg.Editor.SidebarWidth = width
	}
	return nil
}

func toEditorHighlightSpans(spans map[int][]treesitter.HighlightSpan) map[int][]editor.HighlightSpan {
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
}

func syncEditorRepoState(ed *editor.Editor, gitPath string, sessionMgr *session.Manager) {
	ed.SetGitBranch(gitinfo.Branch(gitPath))
	gitRoot, mainBranch := repoState(gitPath, sessionMgr)
	ed.SetGitRoot(gitRoot)
	if mainBranch != "" {
		ed.SetGitMainBranch(mainBranch)
	}
}

func repoState(gitPath string, sessionMgr *session.Manager) (string, string) {
	gitRoot := gitinfo.Root(gitPath)
	if gitRoot == "" {
		return "", ""
	}

	if sessionMgr != nil {
		if repoInfo, ok := sessionMgr.GetRepoInfo(gitRoot); ok && repoInfo.MainBranch != "" {
			return gitRoot, repoInfo.MainBranch
		}
	}

	mainBranch := gitinfo.MainBranch(gitPath)
	if mainBranch != "" && sessionMgr != nil {
		sessionMgr.SetRepoMainBranch(gitRoot, mainBranch)
	}
	return gitRoot, mainBranch
}
