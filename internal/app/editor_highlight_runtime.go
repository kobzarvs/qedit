package app

import (
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

func drainParsedEvent(ts *treesitter.Engine, path string) bool {
	parsed := false
drain:
	for {
		select {
		case evt := <-ts.Events():
			if evt.Kind == "parsed" && evt.Path == path {
				parsed = true
			}
		default:
			break drain
		}
	}
	return parsed
}

func syncVisibleHighlights(
	ed *editor.Editor,
	ts *treesitter.Engine,
	openPath string,
	langName string,
	highlightEnabled bool,
	lastChangeTick uint64,
	lastHighlightStart int,
	lastHighlightEnd int,
) (uint64, int, int) {
	if openPath == "" {
		return lastChangeTick, lastHighlightStart, lastHighlightEnd
	}
	if !highlightEnabled || langName == "" {
		ed.SetHighlights(-1, -1, nil)
		return lastChangeTick, lastHighlightStart, lastHighlightEnd
	}

	tsParsed := drainParsedEvent(ts, openPath)
	tick := ed.ChangeTick()
	changed := tick != lastChangeTick
	asyncChanged := false
	if changed {
		lastChangeTick = tick
		if isAsyncParseLang(langName) {
			// Adjust current visible highlights before async reparse completes.
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
		return lastChangeTick, lastHighlightStart, lastHighlightEnd
	}
	if changed || tsParsed || start != lastHighlightStart || end != lastHighlightEnd {
		if applyHighlightRange(ed, ts, openPath, start, end) {
			return lastChangeTick, start, end
		}
		ed.SetHighlights(-1, -1, nil)
		return lastChangeTick, -1, -1
	}
	return lastChangeTick, lastHighlightStart, lastHighlightEnd
}
