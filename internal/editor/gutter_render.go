package editor

import "fmt"

func (e *Editor) drawLineWithGutterAt(s Screen, x0, y, w, gutterWidth, lineIdx int) {
	line := e.text.Line(lineIdx)
	if markerKind, _ := parseConflictMarker(line); markerKind != conflictNone {
		e.drawConflictMarkerLine(s, y, x0+gutterWidth, x0+w)
		return
	}
	kind, _ := e.conflictLineInfo(lineIdx)
	if kind == conflictNone {
		kind = e.gitDiffLineKind(lineIdx)
	}
	diffWidth := 0
	if e.hasConflictBlocks() || e.gitDiffGutterActive() {
		diffWidth = 1
	}
	numWidth := gutterWidth - diffWidth
	if numWidth > 0 && e.display.lineNumberMode != LineNumberOff {
		// numWidth = 1 (leading space) + digits + 1 (trailing space)
		digits := numWidth - 2
		if digits < 1 {
			digits = 1
		}
		localBefore, remoteBefore := e.conflictLineOffsets(lineIdx)
		localNum := lineIdx + 1 - remoteBefore
		remoteNum := lineIdx + 1 - localBefore
		if localNum < 1 {
			localNum = 1
		}
		if remoteNum < 1 {
			remoteNum = 1
		}
		num := localNum
		if kind == conflictRemote {
			num = remoteNum
		}
		if e.display.lineNumberMode == LineNumberRelative && lineIdx != e.cursor.Row {
			cursorKind, _ := e.conflictLineInfo(e.cursor.Row)
			cursorLocalBefore, cursorRemoteBefore := e.conflictLineOffsets(e.cursor.Row)
			cursorLocalNum := e.cursor.Row + 1 - cursorRemoteBefore
			cursorRemoteNum := e.cursor.Row + 1 - cursorLocalBefore
			if cursorLocalNum < 1 {
				cursorLocalNum = 1
			}
			if cursorRemoteNum < 1 {
				cursorRemoteNum = 1
			}
			cursorNum := cursorLocalNum
			if cursorKind == conflictRemote {
				cursorNum = cursorRemoteNum
			}
			diff := num - cursorNum
			if diff < 0 {
				diff = -diff
			}
			num = diff
		}
		numStr := fmt.Sprintf("%*d", digits, num)
		style := e.styleLineNumber
		if lineIdx == e.cursor.Row {
			style = e.styleLineNumberActive
		}
		spaceStyle := e.styleMain
		if kind == conflictLocal || kind == conflictRemote {
			mergeStyle := e.styleMergeLocal
			if kind == conflictRemote {
				mergeStyle = e.styleMergeRemote
			}
			if mergeStyle != nil {
				fg, _, _ := style.Decompose()
				_, mergeBg, _ := mergeStyle.Decompose()
				style = style.Foreground(fg).Background(mergeBg)
				spaceStyle = style
			}
		}
		// Draw leading space
		if w > 0 {
			s.SetContent(x0, y, ' ', nil, spaceStyle)
		}
		// Draw number (right-aligned with leading spaces)
		for i, r := range numStr {
			x := 1 + i
			if x >= numWidth-1 || x >= w {
				break
			}
			s.SetContent(x0+x, y, r, nil, style)
		}
		// Draw trailing space
		if numWidth-1 < w {
			s.SetContent(x0+numWidth-1, y, ' ', nil, spaceStyle)
		}
	}
	if gutterWidth >= w {
		return
	}
	if diffWidth > 0 && gutterWidth > 0 {
		ch := ' '
		style := e.styleMain
		if sign, signStyle, ok := e.conflictSign(kind); ok {
			ch = sign
			if signStyle != nil {
				style = signStyle
			}
		}
		col := x0 + gutterWidth - 1
		if col < w {
			s.SetContent(col, y, ch, nil, style)
		}
	}
	selStart, selEnd, ok := e.selectionRangeForLine(lineIdx)
	if !ok {
		selStart = -1
		selEnd = -1
	}
	highlightActive := e.highlight.start >= 0 && lineIdx >= e.highlight.start && lineIdx <= e.highlight.end
	var spans []HighlightSpan
	if highlightActive {
		spans = e.highlight.spans[lineIdx]
	}
	e.drawLine(s, y, x0+w, x0+gutterWidth, line, e.display.tabWidth, selStart, selEnd, spans, highlightActive, e.searchMatches, lineIdx, e.searchMatchIndex, e.viewport.scrollX, kind)
}

func (e *Editor) renderFileTreePreview(s Screen, x0, viewHeight, width int) {
	if e.fileTreePreview.text == nil || width <= 0 || viewHeight <= 0 {
		return
	}
	if e.hasLanguageRuntime() && e.fileTreePreview.highlight.end >= 0 &&
		e.fileTreePreview.highlight.end < e.fileTreePreview.scroll+viewHeight-1 {
		e.updateFileTreePreviewHighlights()
	}
	lineCount := e.fileTreePreview.text.LineCount()
	gutterWidth := previewGutterWidth(lineCount, e.display.lineNumberMode)
	for y := 0; y < viewHeight; y++ {
		lineIdx := e.fileTreePreview.scroll + y
		if lineIdx >= lineCount {
			clearLineAt(s, x0, y, width, e.styleMain)
			continue
		}
		e.drawPreviewLineWithGutterAt(s, x0, y, width, gutterWidth, lineIdx)
	}
}

func (e *Editor) drawPreviewLineWithGutterAt(s Screen, x0, y, w, gutterWidth, lineIdx int) {
	line := e.fileTreePreview.text.Line(lineIdx)
	if gutterWidth >= w {
		return
	}
	if gutterWidth > 0 && e.display.lineNumberMode != LineNumberOff {
		numWidth := gutterWidth
		digits := numWidth - 2
		if digits < 1 {
			digits = 1
		}
		num := lineIdx + 1
		numStr := fmt.Sprintf("%*d", digits, num)
		style := e.styleLineNumber
		spaceStyle := e.styleMain
		// Draw leading space
		if w > 0 {
			s.SetContent(x0, y, ' ', nil, spaceStyle)
		}
		// Draw number (right-aligned with leading spaces)
		for i, r := range numStr {
			x := 1 + i
			if x >= numWidth-1 || x >= w {
				break
			}
			s.SetContent(x0+x, y, r, nil, style)
		}
		// Draw trailing space
		if numWidth-1 < w {
			s.SetContent(x0+numWidth-1, y, ' ', nil, spaceStyle)
		}
	}
	highlightActive := e.fileTreePreview.highlight.start >= 0 && lineIdx >= e.fileTreePreview.highlight.start && lineIdx <= e.fileTreePreview.highlight.end
	var spans []HighlightSpan
	if highlightActive {
		spans = e.fileTreePreview.highlight.spans[lineIdx]
	}
	e.drawLine(s, y, x0+w, x0+gutterWidth, line, e.display.tabWidth, -1, -1, spans, highlightActive, nil, lineIdx, 0, e.fileTreePreview.scrollX, conflictNone)
}
