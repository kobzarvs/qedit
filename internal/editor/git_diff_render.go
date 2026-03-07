package editor

import "fmt"

func (e *Editor) renderGitDiffPreview(s Screen, x0, viewHeight, width int) {
	if !e.gitDiffPreviewActive() || viewHeight <= 0 || width <= 0 {
		return
	}
	for y := 0; y < viewHeight; y++ {
		lineIdx := e.viewport.scroll + y
		if lineIdx >= len(e.git.diffPreview.lines) {
			clearLineAt(s, x0, y, width, e.styleMain)
			continue
		}
		e.drawGitDiffPreviewLineAt(s, x0, y, width, lineIdx)
	}
}

func (e *Editor) drawGitDiffPreviewLineAt(s Screen, x0, y, width, lineIdx int) {
	line := e.git.diffPreview.lines[lineIdx]
	gutterWidth := e.gitDiffPreviewGutterWidth()
	if gutterWidth > width {
		gutterWidth = width
	}

	numStyle := e.styleLineNumber
	if line.kind == conflictLocal || line.kind == conflictRemote {
		mergeStyle := e.styleMergeLocal
		if line.kind == conflictRemote {
			mergeStyle = e.styleMergeRemote
		}
		if mergeStyle != nil {
			fg, _, _ := numStyle.Decompose()
			_, mergeBg, _ := mergeStyle.Decompose()
			numStyle = numStyle.Foreground(fg).Background(mergeBg)
		}
	}

	oldText := fmt.Sprintf("%*s", e.git.diffPreview.oldDigits, "")
	if line.oldLine > 0 {
		oldText = fmt.Sprintf("%*d", e.git.diffPreview.oldDigits, line.oldLine)
	}
	newText := fmt.Sprintf("%*s", e.git.diffPreview.newDigits, "")
	if line.newLine > 0 {
		newText = fmt.Sprintf("%*d", e.git.diffPreview.newDigits, line.newLine)
	}

	col := x0
	for _, r := range " " + oldText + " " + " " + newText + " " {
		if col >= x0+gutterWidth-1 || col >= x0+width {
			break
		}
		s.SetContent(col, y, r, nil, numStyle)
		col++
	}
	sign := line.sign
	signStyle := e.styleLineNumber
	if mark, style, ok := e.conflictSign(line.kind); ok {
		sign = mark
		signStyle = style
	}
	if x0+gutterWidth-1 < x0+width {
		s.SetContent(x0+gutterWidth-1, y, sign, nil, signStyle)
	}

	spans, highlightActive := e.gitDiffPreviewHighlight(line)
	e.drawLine(s, y, x0+width, x0+gutterWidth, line.text, e.display.tabWidth, -1, -1, spans, highlightActive, nil, lineIdx, e.searchMatchIndex, e.viewport.scrollX, line.kind)
}

func (e *Editor) gitDiffPreviewHighlight(line gitDiffPreviewLine) ([]HighlightSpan, bool) {
	if line.actualLine < 0 || e.highlight.start < 0 || line.actualLine < e.highlight.start || line.actualLine > e.highlight.end {
		return nil, false
	}
	return e.highlight.spans[line.actualLine], true
}
