package editor

import "strings"

func (e *Editor) searchFromPrimarySelection() {
	start, end, ok := e.selectionRange()
	if !ok {
		return
	}
	text := e.collectDeletedText(start, end)
	if len(text) != 1 || len(text[0]) == 0 {
		return
	}
	e.lastSearchQuery = string(text[0])
	e.searchRegex = false
	e.searchFuzzy = false
	e.modal.selectMode = false
}

func (e *Editor) addSearchSelection(forward bool) {
	if e.lastSearchQuery == "" || e.text == nil {
		return
	}
	currentStart, currentEnd, ok := e.selectionRange()
	if !ok {
		return
	}
	contentRunes := []rune(e.Content())
	content := string(contentRunes)
	query := e.lastSearchQuery
	var matchStart int
	if forward {
		from := e.text.IndexForCursor(currentEnd)
		haystack := string(contentRunes[from:])
		idx := strings.Index(haystack, query)
		if idx < 0 {
			idx = strings.Index(content, query)
			if idx < 0 {
				return
			}
			matchStart = runeOffsetForByte(content, idx)
		} else {
			matchStart = from + runeOffsetForByte(haystack, idx)
		}
	} else {
		to := e.text.IndexForCursor(currentStart)
		haystack := string(contentRunes[:to])
		idx := strings.LastIndex(haystack, query)
		if idx < 0 {
			idx = strings.LastIndex(content, query)
			if idx < 0 {
				return
			}
		}
		matchStart = runeOffsetForByte(content, idx)
	}
	matchEnd := matchStart + len([]rune(query))
	next := editorSelectionRange{
		Start: e.text.CursorForIndex(matchStart),
		End:   e.text.CursorForIndex(matchEnd),
	}
	ranges := e.activeSelectionRanges()
	for _, r := range ranges {
		if r.Start == next.Start && r.End == next.End {
			return
		}
	}
	ranges = append(ranges, next)
	e.setSelectionRanges(ranges, len(ranges)-1)
}
