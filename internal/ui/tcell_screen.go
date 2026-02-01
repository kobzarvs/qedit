package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/editor"
)

type screenAdapter struct {
	screen tcell.Screen
}

// WrapScreen adapts a tcell screen to the editor Screen interface.
func WrapScreen(screen tcell.Screen) editor.Screen {
	return screenAdapter{screen: screen}
}

func (s screenAdapter) Size() (w, h int) {
	return s.screen.Size()
}

func (s screenAdapter) SetStyle(style editor.Style) {
	s.screen.SetStyle(toTCellStyle(style))
}

func (s screenAdapter) Clear() {
	s.screen.Clear()
}

func (s screenAdapter) SetContent(x, y int, ch rune, comb []rune, style editor.Style) {
	s.screen.SetContent(x, y, ch, comb, toTCellStyle(style))
}

func (s screenAdapter) Show() {
	s.screen.Show()
}

func (s screenAdapter) ShowCursor(x, y int) {
	s.screen.ShowCursor(x, y)
}

func (s screenAdapter) HideCursor() {
	s.screen.HideCursor()
}

func (s screenAdapter) SetCursorStyle(style editor.CursorStyle) {
	switch style {
	case editor.CursorStyleSteadyBar:
		s.screen.SetCursorStyle(tcell.CursorStyleSteadyBar)
	default:
		s.screen.SetCursorStyle(tcell.CursorStyleSteadyBlock)
	}
}
