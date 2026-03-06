package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/gitinfo"
	"github.com/kobzarvs/qedit/internal/integrations"
	"github.com/kobzarvs/qedit/internal/lsp"
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

func newConfiguredEditor(cfg *config.Config, langs config.Languages, ts *treesitter.Engine, sessionStore editor.SessionStore) *editor.Editor {
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
	ed.SetHighlightRangeFunc(newHighlightRangeFunc(langs, ts, cfg.Editor.HighlightMaxBytes))
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
	ed.SetHistoryStore(integrations.FileHistoryStore{})
	if runtime.GOOS == "darwin" {
		ed.SetClipboard(integrations.MacClipboard{})
		ed.SetTerminalZoomer(integrations.TerminalZoomer{})
	}

	ed.LoadCmdHistory()
	ed.LoadSearchHistory()
	return ed
}

func newHighlightRangeFunc(langs config.Languages, ts *treesitter.Engine, highlightMaxBytes int64) editor.HighlightRangeFunc {
	return func(path string, startLine, endLine int) map[int][]editor.HighlightSpan {
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
		return toEditorHighlightSpans(ts.Highlights(path, startLine, endLine))
	}
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

func wireEditorRuntimeCallbacks(ed *editor.Editor, ts *treesitter.Engine, ls *lsp.Manager) {
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

	ed.SetLSPGotoFunc(func(method, path string, line, col int) ([]editor.LSPLocation, error) {
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
}
