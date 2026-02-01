package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/editor"
)

type keyAdapter struct {
	ev *tcell.EventKey
}

// WrapKey adapts a tcell key event to the editor EventKey interface.
func WrapKey(ev *tcell.EventKey) editor.EventKey {
	return keyAdapter{ev: ev}
}

func (k keyAdapter) Key() editor.Key {
	return toEditorKey(k.ev.Key())
}

func (k keyAdapter) Rune() rune {
	return k.ev.Rune()
}

func (k keyAdapter) Modifiers() editor.ModMask {
	return toEditorModMask(k.ev.Modifiers())
}

type mouseAdapter struct {
	ev *tcell.EventMouse
}

// WrapMouse adapts a tcell mouse event to the editor EventMouse interface.
func WrapMouse(ev *tcell.EventMouse) editor.EventMouse {
	return mouseAdapter{ev: ev}
}

func (m mouseAdapter) Buttons() editor.MouseButton {
	return toEditorButtons(m.ev.Buttons())
}

func (m mouseAdapter) Position() (x, y int) {
	return m.ev.Position()
}

func toEditorKey(key tcell.Key) editor.Key {
	switch key {
	case tcell.KeyRune:
		return editor.KeyRune
	case tcell.KeyEscape:
		return editor.KeyEscape
	case tcell.KeyEnter:
		return editor.KeyEnter
	case tcell.KeyTab:
		return editor.KeyTab
	case tcell.KeyBacktab:
		return editor.KeyBacktab
	case tcell.KeyBackspace:
		return editor.KeyBackspace
	case tcell.KeyBackspace2:
		return editor.KeyBackspace2
	case tcell.KeyDelete:
		return editor.KeyDelete
	case tcell.KeyUp:
		return editor.KeyUp
	case tcell.KeyDown:
		return editor.KeyDown
	case tcell.KeyLeft:
		return editor.KeyLeft
	case tcell.KeyRight:
		return editor.KeyRight
	case tcell.KeyPgUp:
		return editor.KeyPgUp
	case tcell.KeyPgDn:
		return editor.KeyPgDn
	case tcell.KeyHome:
		return editor.KeyHome
	case tcell.KeyEnd:
		return editor.KeyEnd
	case tcell.KeyCtrlA:
		return editor.KeyCtrlA
	case tcell.KeyCtrlB:
		return editor.KeyCtrlB
	case tcell.KeyCtrlC:
		return editor.KeyCtrlC
	case tcell.KeyCtrlD:
		return editor.KeyCtrlD
	case tcell.KeyCtrlE:
		return editor.KeyCtrlE
	case tcell.KeyCtrlF:
		return editor.KeyCtrlF
	case tcell.KeyCtrlG:
		return editor.KeyCtrlG
	case tcell.KeyCtrlH:
		return editor.KeyCtrlH
	case tcell.KeyCtrlI:
		return editor.KeyCtrlI
	case tcell.KeyCtrlJ:
		return editor.KeyCtrlJ
	case tcell.KeyCtrlK:
		return editor.KeyCtrlK
	case tcell.KeyCtrlL:
		return editor.KeyCtrlL
	case tcell.KeyCtrlM:
		return editor.KeyCtrlM
	case tcell.KeyCtrlN:
		return editor.KeyCtrlN
	case tcell.KeyCtrlO:
		return editor.KeyCtrlO
	case tcell.KeyCtrlP:
		return editor.KeyCtrlP
	case tcell.KeyCtrlQ:
		return editor.KeyCtrlQ
	case tcell.KeyCtrlR:
		return editor.KeyCtrlR
	case tcell.KeyCtrlS:
		return editor.KeyCtrlS
	case tcell.KeyCtrlT:
		return editor.KeyCtrlT
	case tcell.KeyCtrlU:
		return editor.KeyCtrlU
	case tcell.KeyCtrlV:
		return editor.KeyCtrlV
	case tcell.KeyCtrlW:
		return editor.KeyCtrlW
	case tcell.KeyCtrlX:
		return editor.KeyCtrlX
	case tcell.KeyCtrlY:
		return editor.KeyCtrlY
	case tcell.KeyCtrlZ:
		return editor.KeyCtrlZ
	default:
		return editor.KeyUnknown
	}
}

func toEditorModMask(mask tcell.ModMask) editor.ModMask {
	var out editor.ModMask
	if mask&tcell.ModCtrl != 0 {
		out |= editor.ModCtrl
	}
	if mask&tcell.ModShift != 0 {
		out |= editor.ModShift
	}
	if mask&tcell.ModAlt != 0 {
		out |= editor.ModAlt
	}
	if mask&tcell.ModMeta != 0 {
		out |= editor.ModMeta
	}
	return out
}

func toEditorButtons(mask tcell.ButtonMask) editor.MouseButton {
	var out editor.MouseButton
	if mask&tcell.Button1 != 0 {
		out |= editor.Button1
	}
	if mask&tcell.WheelUp != 0 {
		out |= editor.WheelUp
	}
	if mask&tcell.WheelDown != 0 {
		out |= editor.WheelDown
	}
	if mask&tcell.WheelLeft != 0 {
		out |= editor.WheelLeft
	}
	if mask&tcell.WheelRight != 0 {
		out |= editor.WheelRight
	}
	return out
}
