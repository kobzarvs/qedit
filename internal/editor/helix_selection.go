package editor

import (
	"regexp"
	"sort"
	"unicode/utf8"
)

func (e *Editor) enterHelixSelectionPrompt(action string) {
	e.profile.helix.selectionPromptRanges = cloneSelectionRanges(e.selectionSearchRanges())
	e.mode = ModeSearch
	e.searchQuery = e.searchQuery[:0]
	e.searchCursor = 0
	e.searchMatches = nil
	e.searchMatchIndex = 0
	e.searchForward = true
	e.searchFuzzy = false
	e.searchRegex = true
	e.searchHistoryIndex = -1
	e.searchConfirmAction = action
}

func (e *Editor) applyHelixSelectionPattern(action, pattern string) {
	defer func() {
		e.profile.helix.selectionPromptRanges = nil
	}()
	if pattern == "" {
		if action == actionSplitRegex {
			e.clearRegexStatus()
			e.splitSelectionRangesByLine(e.helixPromptSearchRanges())
		}
		return
	}
	pattern = expandRegexEscapedNewlines(pattern)
	re, err := regexp.Compile(pattern)
	if err != nil {
		e.setStatus("regex error: " + err.Error())
		return
	}
	e.clearRegexStatus()
	switch action {
	case actionSelectRegex:
		e.setSelectionRangesOrStatus(e.regexSelectionRanges(re))
	case actionSplitRegex:
		e.setSelectionRangesOrStatus(e.regexSplitRanges(re))
	}
}

func (e *Editor) previewHelixSelectionPattern() {
	action := e.searchConfirmAction
	if action != actionSelectRegex && action != actionSplitRegex {
		return
	}
	pattern := string(e.searchQuery)
	if pattern == "" {
		e.setSelectionRanges(e.helixPromptSearchRanges(), 0)
		return
	}
	pattern = expandRegexEscapedNewlines(pattern)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return
	}
	e.clearRegexStatus()
	switch action {
	case actionSelectRegex:
		e.setSelectionRanges(e.regexSelectionRanges(re), 0)
	case actionSplitRegex:
		e.setSelectionRanges(e.regexSplitRanges(re), 0)
	}
}

func (e *Editor) setSelectionRangesOrStatus(ranges []editorSelectionRange) {
	if len(ranges) == 0 {
		e.setStatus("no selections")
		return
	}
	e.setSelectionRanges(ranges, 0)
}

func (e *Editor) regexSelectionRanges(re *regexp.Regexp) []editorSelectionRange {
	ranges := e.helixPromptSearchRanges()
	var out []editorSelectionRange
	content := []rune(e.Content())
	for _, r := range ranges {
		startIdx := e.text.IndexForCursor(r.Start)
		endIdx := e.text.IndexForCursor(r.End)
		if startIdx < 0 || endIdx > len(content) || startIdx >= endIdx {
			continue
		}
		segment := string(content[startIdx:endIdx])
		for _, match := range re.FindAllStringIndex(segment, -1) {
			startRune := runeOffsetForByte(segment, match[0])
			endRune := runeOffsetForByte(segment, match[1])
			if endRune <= startRune {
				continue
			}
			start := e.text.CursorForIndex(startIdx + startRune)
			end := e.text.CursorForIndex(startIdx + endRune)
			out = append(out, editorSelectionRange{
				Start: end,
				End:   start,
			})
		}
	}
	return out
}

func (e *Editor) selectRegexInSelections(re *regexp.Regexp) {
	e.setSelectionRangesOrStatus(e.regexSelectionRanges(re))
}

func (e *Editor) regexSplitRanges(re *regexp.Regexp) []editorSelectionRange {
	ranges := e.helixPromptSearchRanges()
	var out []editorSelectionRange
	content := []rune(e.Content())
	for _, r := range ranges {
		startIdx := e.text.IndexForCursor(r.Start)
		endIdx := e.text.IndexForCursor(r.End)
		if startIdx < 0 || endIdx > len(content) || startIdx >= endIdx {
			continue
		}
		segment := string(content[startIdx:endIdx])
		prev := 0
		for _, match := range re.FindAllStringIndex(segment, -1) {
			matchStart := runeOffsetForByte(segment, match[0])
			matchEnd := runeOffsetForByte(segment, match[1])
			if matchStart > prev {
				out = append(out, editorSelectionRange{
					Start: e.text.CursorForIndex(startIdx + prev),
					End:   e.text.CursorForIndex(startIdx + matchStart),
				})
			}
			if matchEnd > prev {
				prev = matchEnd
			}
		}
		if prev < len([]rune(segment)) {
			out = append(out, editorSelectionRange{
				Start: e.text.CursorForIndex(startIdx + prev),
				End:   e.text.CursorForIndex(endIdx),
			})
		}
	}
	return out
}

func (e *Editor) splitSelectionsByRegex(re *regexp.Regexp) {
	e.setSelectionRangesOrStatus(e.regexSplitRanges(re))
}

func (e *Editor) selectionSearchRanges() []editorSelectionRange {
	ranges := e.activeSelectionRanges()
	if len(ranges) > 0 {
		return ranges
	}
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		return nil
	}
	return []editorSelectionRange{{
		Start: Cursor{Row: e.cursor.Row, Col: 0},
		End:   Cursor{Row: e.cursor.Row, Col: e.lineLen(e.cursor.Row)},
	}}
}

func (e *Editor) helixPromptSearchRanges() []editorSelectionRange {
	if len(e.profile.helix.selectionPromptRanges) > 0 {
		return cloneSelectionRanges(e.profile.helix.selectionPromptRanges)
	}
	return e.selectionSearchRanges()
}

func (e *Editor) splitSelectionsByLine() {
	e.splitSelectionRangesByLine(e.selectionSearchRanges())
}

func (e *Editor) splitSelectionRangesByLine(ranges []editorSelectionRange) {
	var out []editorSelectionRange
	for _, r := range ranges {
		start, end, ok := r.normalized()
		if !ok {
			continue
		}
		endRow := end.Row
		if end.Col == 0 && endRow > start.Row {
			endRow--
		}
		for row := start.Row; row <= endRow && row < e.LineCount(); row++ {
			out = append(out, editorSelectionRange{
				Start: Cursor{Row: row, Col: 0},
				End:   e.linewiseSelectionEnd(row),
			})
		}
	}
	e.setSelectionRanges(out, 0)
}

func (e *Editor) alignSelections() {
	ranges := e.rawActiveSelectionRanges()
	if len(ranges) < 2 {
		return
	}
	maxCol := 0
	for _, r := range ranges {
		if r.End.Col > maxCol {
			maxCol = r.End.Col
		}
	}
	sort.SliceStable(ranges, func(i, j int) bool {
		if ranges[i].End.Row != ranges[j].End.Row {
			return ranges[i].End.Row > ranges[j].End.Row
		}
		return ranges[i].End.Col > ranges[j].End.Col
	})
	e.startUndoGroup()
	for _, r := range ranges {
		missing := maxCol - r.End.Col
		if missing <= 0 || r.End.Row < 0 || r.End.Row >= e.LineCount() {
			continue
		}
		pos := r.End
		for i := 0; i < missing; i++ {
			if e.insertRuneAt(pos, ' ') {
				e.appendUndo(action{kind: actionDeleteRune, pos: pos, r: ' '})
				pos.Col++
			}
		}
	}
	e.finishUndoGroup()
	e.change.lastEdit.Valid = false
}

func (e *Editor) cyclePrimarySelection(delta int) {
	if len(e.selectionRanges) == 0 {
		return
	}
	next := e.primarySelection + delta
	for next < 0 {
		next += len(e.selectionRanges)
	}
	next %= len(e.selectionRanges)
	e.setSelectionRanges(e.selectionRanges, next)
}

func (e *Editor) keepPrimarySelection() {
	if len(e.profile.helix.multiCursors) > 1 {
		e.profile.helix.multiCursors = nil
		e.clearSelection()
		e.modal.selectMode = false
		return
	}
	if len(e.selectionRanges) <= 1 {
		return
	}
	primary := e.primarySelection
	if primary < 0 || primary >= len(e.selectionRanges) {
		primary = 0
	}
	e.setSelectionRanges([]editorSelectionRange{e.selectionRanges[primary]}, 0)
}

func (e *Editor) removePrimarySelection() {
	if len(e.selectionRanges) <= 1 {
		e.clearSelection()
		e.modal.selectMode = false
		return
	}
	next := append([]editorSelectionRange(nil), e.selectionRanges[:e.primarySelection]...)
	next = append(next, e.selectionRanges[e.primarySelection+1:]...)
	primary := e.primarySelection
	if primary >= len(next) {
		primary = len(next) - 1
	}
	e.setSelectionRanges(next, primary)
}

func (e *Editor) cycleSelectionContents(direction int) {
	ranges := normalizedSelectionRanges(e.selectionRanges)
	if len(ranges) < 2 {
		return
	}
	sort.SliceStable(ranges, func(i, j int) bool {
		return cursorLess(ranges[i].Start, ranges[j].Start)
	})
	texts := make([][][]rune, len(ranges))
	for i, r := range ranges {
		texts[i] = e.collectDeletedText(r.Start, r.End)
	}
	replacements := make([][][]rune, len(ranges))
	for i := range ranges {
		from := i - direction
		for from < 0 {
			from += len(texts)
		}
		from %= len(texts)
		replacements[i] = texts[from]
	}
	type rangeReplacement struct {
		r           editorSelectionRange
		replacement [][]rune
	}
	items := make([]rangeReplacement, len(ranges))
	for i, r := range ranges {
		items[i] = rangeReplacement{r: r, replacement: replacements[i]}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].r.Start.Row != items[j].r.Start.Row {
			return items[i].r.Start.Row > items[j].r.Start.Row
		}
		return items[i].r.Start.Col > items[j].r.Start.Col
	})
	e.startUndoGroup()
	for _, item := range items {
		deleted := e.deleteTextRange(item.r.Start, item.r.End)
		if len(deleted) > 0 {
			e.appendUndo(action{kind: actionInsertText, pos: item.r.Start, text: deleted})
		}
		if len(item.replacement) > 0 {
			endPos := e.insertTextAt(item.r.Start, item.replacement)
			e.appendUndo(action{kind: actionDeleteText, pos: item.r.Start, endPos: endPos, text: item.replacement})
		}
	}
	e.finishUndoGroup()
	e.clearSelection()
	e.modal.selectMode = false
	e.change.lastEdit.Valid = false
}

func runeOffsetForByte(s string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset >= len(s) {
		return utf8.RuneCountInString(s)
	}
	return utf8.RuneCountInString(s[:byteOffset])
}
