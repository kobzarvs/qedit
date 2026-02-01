package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/editor"
)

type tcellStyle struct {
	style tcell.Style
}

func (s tcellStyle) Decompose() (editor.Color, editor.Color, editor.AttrMask) {
	fg, bg, attrs := s.style.Decompose()
	return editor.Color(fg), editor.Color(bg), editor.AttrMask(attrs)
}

func (s tcellStyle) Foreground(c editor.Color) editor.Style {
	return tcellStyle{style: s.style.Foreground(tcell.Color(c))}
}

func (s tcellStyle) Background(c editor.Color) editor.Style {
	return tcellStyle{style: s.style.Background(tcell.Color(c))}
}

func wrapStyle(style tcell.Style) editor.Style {
	return tcellStyle{style: style}
}

func toTCellStyle(style editor.Style) tcell.Style {
	if style == nil {
		return tcell.StyleDefault
	}
	switch s := style.(type) {
	case tcellStyle:
		return s.style
	case *tcellStyle:
		return s.style
	default:
		fg, bg, attrs := style.Decompose()
		return tcell.StyleDefault.
			Foreground(tcell.Color(fg)).
			Background(tcell.Color(bg)).
			Attributes(tcell.AttrMask(attrs))
	}
}
