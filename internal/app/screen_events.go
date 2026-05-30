package app

import (
	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/ui"
)

func handleScreenEvent(screen tcell.Screen, ed *editor.Editor, ev tcell.Event) (bool, bool) {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		if ed.HandleKey(ui.WrapKey(ev)) {
			return true, false
		}
	case *tcell.EventMouse:
		ed.HandleMouse(ui.WrapMouse(ev))
		return false, true
	case *tcell.EventResize:
		screen.Sync()
	case *tcell.EventInterrupt:
		// Layout updates are handled by the caller after the event.
	}
	return false, false
}
