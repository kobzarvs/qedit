package app

import (
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

func launchHugeHighlightJob(ts *treesitter.Engine, state *editorRuntimeState, path string, visibleStart, visibleEnd, prefetchStart, prefetchEnd, colStart, colEnd int) {
	if ts == nil || state == nil || path == "" || visibleStart < 0 || visibleEnd < visibleStart {
		return
	}
	if state.highlightJobCancel != nil {
		close(state.highlightJobCancel)
		state.highlightJobCancel = nil
	}
	state.highlightJobSeq++
	seq := state.highlightJobSeq
	state.highlightJobActive = true
	state.highlightJobStart = visibleStart
	state.highlightJobEnd = visibleEnd
	cancel := make(chan struct{})
	state.highlightJobCancel = cancel
	results := state.highlightResults
	go func() {
		sendRange := func(start, end int) bool {
			select {
			case <-cancel:
				return false
			default:
			}
			spans := toEditorHighlightSpans(ts.HighlightsWindow(path, start, end, colStart, colEnd))
			select {
			case <-cancel:
				return false
			case results <- asyncHighlightResult{
				seq:   seq,
				path:  path,
				start: start,
				end:   end,
				spans: spans,
			}:
				return true
			}
		}
		if !sendRange(visibleStart, visibleEnd) {
			return
		}
		if prefetchStart >= 0 && prefetchEnd >= prefetchStart && (prefetchStart != visibleStart || prefetchEnd != visibleEnd) {
			if !sendRange(prefetchStart, prefetchEnd) {
				return
			}
		}
		select {
		case <-cancel:
		case results <- asyncHighlightResult{seq: seq, path: path, done: true}:
		}
	}()
}

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

func rangeCovers(coverStart, coverEnd, start, end int) bool {
	return coverStart >= 0 && coverEnd >= coverStart && coverStart <= start && coverEnd >= end
}

func hugeHighlightTargetRange(ed *editor.Editor, start, end, lastVisibleStart int) (int, int, int, int) {
	if ed == nil {
		return start, end, start, end
	}
	if end < start {
		end = start
	}
	lineCount := ed.LineCount()
	if lineCount > 0 && end >= lineCount {
		end = lineCount - 1
	}
	visibleLines := end - start + 1
	if visibleLines < 1 {
		visibleLines = 1
	}
	prefetchStart := start
	prefetchEnd := end
	if lastVisibleStart >= 0 {
		switch {
		case start > lastVisibleStart:
			prefetchEnd = end + visibleLines
			if lineCount > 0 && prefetchEnd >= lineCount {
				prefetchEnd = lineCount - 1
			}
		case start < lastVisibleStart:
			prefetchStart = start - visibleLines
			if prefetchStart < 0 {
				prefetchStart = 0
			}
		}
	}
	return start, end, prefetchStart, prefetchEnd
}

func syncVisibleHighlights(
	ed *editor.Editor,
	ts *treesitter.Engine,
	state *editorRuntimeState,
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
	if ed.HugeFileMode() {
		if state != nil {
			if tsParsed {
				state.highlightParsed = true
			}
		drain:
			for {
				select {
				case result := <-state.highlightResults:
					if result.seq != state.highlightJobSeq || result.path != openPath {
						continue
					}
					if result.done {
						state.highlightJobActive = false
						state.highlightJobStart = -1
						state.highlightJobEnd = -1
						state.highlightJobCancel = nil
						continue
					}
					ed.MergeHighlights(result.start, result.end, result.spans)
					lastHighlightStart = result.start
					lastHighlightEnd = result.end
				default:
					break drain
				}
			}
		}
		if tick != lastChangeTick {
			lastChangeTick = tick
		}
		if ed.IsDirty() {
			if state != nil {
				state.highlightParsed = false
				state.highlightJobSeq++
				state.highlightJobActive = false
				state.highlightJobStart = -1
				state.highlightJobEnd = -1
				if state.highlightJobCancel != nil {
					close(state.highlightJobCancel)
					state.highlightJobCancel = nil
				}
			}
			ed.SetHighlights(-1, -1, nil)
			return lastChangeTick, -1, -1
		}
		start, end := ed.VisibleRange()
		if state != nil {
			defer func() {
				state.lastVisibleStart = start
			}()
		}
		if state == nil || !state.highlightParsed {
			return lastChangeTick, lastHighlightStart, lastHighlightEnd
		}
		if ed.HighlightsCover(start, end) {
			return lastChangeTick, lastHighlightStart, lastHighlightEnd
		}
		visibleStart, visibleEnd, prefetchStart, prefetchEnd := hugeHighlightTargetRange(ed, start, end, state.lastVisibleStart)
		if state.highlightJobActive && rangeCovers(state.highlightJobStart, state.highlightJobEnd, start, end) {
			return lastChangeTick, lastHighlightStart, lastHighlightEnd
		}
		if ts == nil {
			return lastChangeTick, lastHighlightStart, lastHighlightEnd
		}
		if state.highlightJobActive && state.highlightJobStart == visibleStart && state.highlightJobEnd == visibleEnd {
			return lastChangeTick, lastHighlightStart, lastHighlightEnd
		}
		colStart, colEnd := ed.HighlightWindowCols()
		launchHugeHighlightJob(ts, state, openPath, visibleStart, visibleEnd, prefetchStart, prefetchEnd, colStart, colEnd)
		return lastChangeTick, lastHighlightStart, lastHighlightEnd
	}

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
