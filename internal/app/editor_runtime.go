package app

import (
	"path/filepath"
	"runtime"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/integrations"
	"github.com/kobzarvs/qedit/internal/plugins"
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

func newEditorFormatter() editor.Formatter {
	return integrations.GoFormatter{}
}

func newEditorMerger() editor.Merger {
	return integrations.GitMerger{}
}

func newEditorGitRuntime() editor.GitRuntime {
	return integrations.GitInfoRuntime{}
}

func newEditorClipboard() editor.Clipboard {
	if runtime.GOOS == "darwin" {
		return integrations.MacClipboard{}
	}
	return nil
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
		Profile:              cfg.Editor.Profile,
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
		WorkspaceRuntime: editor.NewStoreBackedWorkspaceRuntime(
			fileStore,
			newEditorFormatter(),
			newEditorMerger(),
		),
		PersistenceRuntime: editor.NewStoreBackedPersistenceRuntime(
			sessionStore,
			integrations.FileHistoryStore{},
			integrations.FileUndoStore{},
		),
		LanguageRuntime: languageRuntime,
		GitRuntime:      newEditorGitRuntime(),
	}
	if clipboard := newEditorClipboard(); clipboard != nil {
		runtimeServices.SystemClipboard = clipboard
		runtimeServices.TerminalZoomer = integrations.TerminalZoomer{}
	}
	ed.ApplyRuntimeServices(runtimeServices)
	if err := plugins.NewRegistry(
		plugins.NewProfileSidebarPlugin(),
	).Apply(ed); err != nil {
		ed.SetStatusMessage("plugin init failed: " + err.Error())
	}

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

func persistEditorProfile(cfg *config.Config, profile string) error {
	if err := config.UpdateEditorProfile(profile); err != nil {
		return err
	}
	if cfg != nil {
		cfg.Editor.Profile = profile
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

func syncEditorRepoState(ed *editor.Editor, gitPath string, sessionMgr *session.Manager, gitRuntime editor.GitRuntime) {
	ed.SetGitBranch(gitRuntime.Branch(gitPath))
	gitRoot, mainBranch := repoState(gitPath, sessionMgr, gitRuntime)
	ed.SetGitRoot(gitRoot)
	if mainBranch != "" {
		ed.SetGitMainBranch(mainBranch)
	}
}

func repoState(gitPath string, sessionMgr *session.Manager, gitRuntime editor.GitRuntime) (string, string) {
	gitRoot := gitRuntime.Root(gitPath)
	if gitRoot == "" {
		return "", ""
	}

	if sessionMgr != nil {
		if repoInfo, ok := sessionMgr.GetRepoInfo(gitRoot); ok && repoInfo.MainBranch != "" {
			return gitRoot, repoInfo.MainBranch
		}
	}

	mainBranch := gitRuntime.MainBranch(gitPath)
	if mainBranch != "" && sessionMgr != nil {
		sessionMgr.SetRepoMainBranch(gitRoot, mainBranch)
	}
	return gitRoot, mainBranch
}
