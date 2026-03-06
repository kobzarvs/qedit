package app

import (
	"fmt"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

type editorLanguageRuntime struct {
	fileStore         editor.FileStore
	langs             config.Languages
	ts                *treesitter.Engine
	ls                *lsp.Manager
	highlightMaxBytes int64
}

func newEditorLanguageRuntime(fileStore editor.FileStore, langs config.Languages, ts *treesitter.Engine, ls *lsp.Manager, highlightMaxBytes int64) editor.LanguageRuntime {
	return editorLanguageRuntime{
		fileStore:         fileStore,
		langs:             langs,
		ts:                ts,
		ls:                ls,
		highlightMaxBytes: highlightMaxBytes,
	}
}

func (r editorLanguageRuntime) NodeStack(path string, row, col int) []editor.NodeRange {
	if r.ts == nil {
		return nil
	}
	stack := r.ts.GetNodeStackAt(path, row, col)
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
}

func (r editorLanguageRuntime) Goto(method, path string, line, col int) ([]editor.LSPLocation, error) {
	if r.ls == nil {
		return nil, fmt.Errorf("language runtime unavailable")
	}
	absPath := normalizeAppPath(r.fileStore, path)
	var locs []lsp.Location
	var err error
	switch method {
	case "definition":
		locs, err = r.ls.GotoDefinition(absPath, line, col)
	case "declaration":
		locs, err = r.ls.GotoDeclaration(absPath, line, col)
	case "typeDefinition":
		locs, err = r.ls.GotoTypeDefinition(absPath, line, col)
	case "references":
		locs, err = r.ls.FindReferences(absPath, line, col)
	case "implementation":
		locs, err = r.ls.GotoImplementation(absPath, line, col)
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
}

func (r editorLanguageRuntime) HighlightRange(path string, startLine, endLine int) map[int][]editor.HighlightSpan {
	if path == "" || startLine < 0 || endLine < startLine {
		return nil
	}
	if r.fileStore == nil || r.ts == nil {
		return nil
	}
	if info, err := r.fileStore.Stat(path); err != nil {
		return nil
	} else if r.highlightMaxBytes > 0 && info.Size > r.highlightMaxBytes {
		return nil
	}
	lang := r.langs.Match(path)
	if lang == nil {
		return nil
	}
	data, err := r.fileStore.Read(path)
	if err != nil {
		return nil
	}
	if !r.ts.ParseSync(path, lang.Name, string(data)) {
		return nil
	}
	return toEditorHighlightSpans(r.ts.Highlights(path, startLine, endLine))
}
