package app

import (
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

const invalidHighlightParseVersion = ^uint64(0)

func launchHugeHighlightJob(ts *treesitter.Engine, state *editorRuntimeState, path string, visibleStart, visibleEnd, prefetchStart, prefetchEnd, colStart, colEnd, totalLines int) {
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
	state.highlightJobColStart = colStart
	state.highlightJobColEnd = colEnd
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
		// 1. Visible range — highest priority.
		if !sendRange(visibleStart, visibleEnd) {
			return
		}
		// 2. Prefetch range in scroll direction.
		if prefetchStart >= 0 && prefetchEnd >= prefetchStart && (prefetchStart != visibleStart || prefetchEnd != visibleEnd) {
			if !sendRange(prefetchStart, prefetchEnd) {
				return
			}
		}
		// 3. Background expansion — cover the rest of the file in chunks.
		//    Small chunks so the opMu lock is released frequently, allowing
		//    cancel to take effect and new jobs to start quickly.
		if totalLines > 0 {
			const chunkSize = 20
			coveredStart := prefetchStart
			if coveredStart < 0 || coveredStart > visibleStart {
				coveredStart = visibleStart
			}
			coveredEnd := prefetchEnd
			if coveredEnd < visibleEnd {
				coveredEnd = visibleEnd
			}
			for coveredStart > 0 || coveredEnd < totalLines-1 {
				// Expand forward.
				if coveredEnd < totalLines-1 {
					chunkStart := coveredEnd + 1
					chunkEnd := chunkStart + chunkSize - 1
					if chunkEnd >= totalLines {
						chunkEnd = totalLines - 1
					}
					if !sendRange(chunkStart, chunkEnd) {
						return
					}
					coveredEnd = chunkEnd
				}
				// Expand backward.
				if coveredStart > 0 {
					chunkEnd := coveredStart - 1
					chunkStart := chunkEnd - chunkSize + 1
					if chunkStart < 0 {
						chunkStart = 0
					}
					if !sendRange(chunkStart, chunkEnd) {
						return
					}
					coveredStart = chunkStart
				}
			}
		}
		select {
		case <-cancel:
		case results <- asyncHighlightResult{seq: seq, path: path, done: true}:
		}
	}()
}

func drainParsedEvent(ts *treesitter.Engine, path string) (uint64, bool) {
	parsed := false
	version := uint64(0)
drain:
	for {
		select {
		case evt := <-ts.Events():
			if evt.Kind == "parsed" && evt.Path == path {
				parsed = true
				version = evt.Version
			}
		default:
			break drain
		}
	}
	return version, parsed
}

func rangeCovers(coverStart, coverEnd, start, end int) bool {
	return coverStart >= 0 && coverEnd >= coverStart && coverStart <= start && coverEnd >= end
}

func resetHugeHighlightJob(state *editorRuntimeState) {
	if state == nil {
		return
	}
	state.highlightParsed = false
	state.highlightJobSeq++
	state.highlightJobActive = false
	state.highlightJobStart = -1
	state.highlightJobEnd = -1
	state.highlightJobColStart = -1
	state.highlightJobColEnd = -1
	if state.highlightJobCancel != nil {
		close(state.highlightJobCancel)
		state.highlightJobCancel = nil
	}
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

	parsedVersion, parsedEvent := drainParsedEvent(ts, openPath)
	tick := ed.ChangeTick()
	asyncLang := isAsyncParseLang(langName)
	if ed.HugeFileMode() {
		changed := tick != lastChangeTick
		if state != nil {
			if parsedEvent && (parsedVersion == state.highlightParseVersion || (state.highlightParseVersion == 0 && !ed.IsDirty())) {
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
						state.highlightJobColStart = -1
						state.highlightJobColEnd = -1
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
		if changed {
			lastChangeTick = tick
		}
		if ed.IsDirty() {
			if state == nil || ts == nil || !asyncLang {
				resetHugeHighlightJob(state)
				ed.SetHighlights(-1, -1, nil)
				return lastChangeTick, -1, -1
			}
			if changed || state.highlightParseVersion == 0 {
				resetHugeHighlightJob(state)
				content, ok := ed.HighlightContent(state.highlightMaxBytes)
				if !ok {
					state.highlightParseVersion = invalidHighlightParseVersion
					ed.SetHighlights(-1, -1, nil)
					return lastChangeTick, -1, -1
				}
				state.highlightParseVersion = ts.Parse(openPath, langName, content)
				state.highlightParsed = false
				ed.SetHighlights(-1, -1, nil)
				return lastChangeTick, -1, -1
			}
			if !state.highlightParsed {
				return lastChangeTick, lastHighlightStart, lastHighlightEnd
			}
		}
		start, end := ed.HighlightVisibleRange()
		if state != nil {
			defer func() {
				state.lastVisibleStart = start
			}()
		}
		if state == nil || !state.highlightParsed {
			return lastChangeTick, lastHighlightStart, lastHighlightEnd
		}
		colStart, colEnd := ed.HighlightWindowCols()
		linesCovered := ed.HighlightsCover(start, end)
		columnsQueried := ed.HighlightsColumnsCover(colStart, colEnd)
		colsCovered := linesCovered && columnsQueried && ed.HighlightsColumnsHaveSpans(start, end, colStart, colEnd)
		if linesCovered && colsCovered {
			return lastChangeTick, lastHighlightStart, lastHighlightEnd
		}
		visibleStart, visibleEnd, prefetchStart, prefetchEnd := hugeHighlightTargetRange(ed, start, end, state.lastVisibleStart)
		jobCoversWindow := state.highlightJobActive &&
			rangeCovers(state.highlightJobStart, state.highlightJobEnd, start, end) &&
			rangeCovers(state.highlightJobColStart, state.highlightJobColEnd, colStart, colEnd)
		if jobCoversWindow {
			return lastChangeTick, lastHighlightStart, lastHighlightEnd
		}
		if ts == nil {
			return lastChangeTick, lastHighlightStart, lastHighlightEnd
		}
		if state.highlightJobActive &&
			state.highlightJobStart == visibleStart &&
			state.highlightJobEnd == visibleEnd &&
			rangeCovers(state.highlightJobColStart, state.highlightJobColEnd, colStart, colEnd) {
			return lastChangeTick, lastHighlightStart, lastHighlightEnd
		}
		// Add overscan so horizontal scrolling within the window doesn't re-query.
		const colOverscan = 1024
		queryColStart := colStart - colOverscan
		if queryColStart < 0 {
			queryColStart = 0
		}
		queryColEnd := colEnd + colOverscan
		ed.SetHighlightColumns(queryColStart, queryColEnd)
		launchHugeHighlightJob(ts, state, openPath, visibleStart, visibleEnd, prefetchStart, prefetchEnd, queryColStart, queryColEnd, ed.LineCount())
		return lastChangeTick, lastHighlightStart, lastHighlightEnd
	}

	changed := tick != lastChangeTick
	asyncChanged := false
	if asyncLang && parsedEvent && state != nil && parsedVersion == state.highlightParseVersion {
		state.highlightParsed = true
	}
	if changed {
		lastChangeTick = tick
		if asyncLang {
			// Adjust current visible highlights before async reparse completes.
			if edit, ok := ed.PeekLastEdit(); ok {
				ed.AdjustHighlights(edit.StartRow, edit.OldEndRow, edit.NewEndRow)
			} else {
				ed.SetHighlights(-1, -1, nil)
				lastHighlightStart = -1
				lastHighlightEnd = -1
			}
			parseVersion := ts.Parse(openPath, langName, ed.Content())
			if state != nil {
				state.highlightParseVersion = parseVersion
				state.highlightParsed = false
			}
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

	start, end := ed.HighlightVisibleRange()
	if asyncChanged {
		return lastChangeTick, lastHighlightStart, lastHighlightEnd
	}
	tsParsed := parsedEvent && (state == nil || state.highlightParseVersion == 0 || parsedVersion == state.highlightParseVersion)
	if asyncLang && state != nil && !state.highlightParsed {
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
