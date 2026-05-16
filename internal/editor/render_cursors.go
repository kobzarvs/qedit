package editor

import "strings"

func (e *Editor) helixVirtualCursorColsForLine(row int) map[int]bool {
	if e.BehaviorProfile() != BehaviorProfileHelix {
		return nil
	}
	cols := make(map[int]bool)
	add := func(pos Cursor) {
		if pos.Row != row || pos == e.cursor {
			return
		}
		if pos.Col < 0 {
			pos.Col = 0
		}
		if maxCol := e.lineLen(row); pos.Col > maxCol {
			pos.Col = maxCol
		}
		cols[pos.Col] = true
	}
	for _, pos := range e.profile.helix.multiCursors {
		add(pos)
	}
	for _, r := range e.selectionRanges {
		add(e.visibleSelectionHead(r))
	}
	if len(cols) == 0 {
		return nil
	}
	return cols
}

func (e *Editor) visibleSelectionHead(r editorSelectionRange) Cursor {
	head := r.End
	if head.Row > r.Start.Row && head.Col == 0 {
		row := head.Row - 1
		return Cursor{Row: row, Col: e.lineLen(row)}
	}
	return head
}

func (e *Editor) virtualCursorStyle(base Style) Style {
	fg, bg, _ := base.Decompose()
	if fg == bg {
		mainFg, _, _ := e.styleMain.Decompose()
		_, selBg, _ := e.styleSelection.Decompose()
		return base.Foreground(mainFg).Background(selBg)
	}
	return base.Foreground(bg).Background(fg)
}

func expandRegexEscapedNewlines(pattern string) string {
	return strings.ReplaceAll(pattern, `\n`, "\n")
}

func (e *Editor) clearRegexStatus() {
	if strings.HasPrefix(e.ui.statusMessage, "regex error:") {
		e.ui.statusMessage = ""
	}
}
