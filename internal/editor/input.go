package editor

import (
	"fmt"
	"strings"
)

func (e *Editor) HandleKey(ev EventKey) bool {
	e.freeScroll = false
	if e.mode != ModeCommand && e.mode != ModeSearch && e.statusMessage != "" && !e.autoReloadInProgress {
		e.statusMessage = ""
	}
	// Track last key combination for display
	e.lastKeyCombo = keyStringDisplay(ev)

	if handled, quit := e.handleGlobalFocusHotkeys(ev); handled {
		return quit
	}

	// Handle sidebar if focused
	if e.sidebar != nil && e.sidebar.Visible && e.sidebar.Focused {
		return e.handleSidebarKey(ev)
	}

	switch e.mode {
	case ModeInsert:
		return e.handleInsert(ev)
	case ModeCommand:
		return e.handleCommand(ev)
	case ModeBranchPicker:
		return e.handleBranchPicker(ev)
	case ModeSearch:
		return e.handleSearch(ev)
	case ModeMerge:
		return e.handleMerge(ev)
	default:
		return e.handleNormal(ev)
	}
}

func (e *Editor) handleGlobalFocusHotkeys(ev EventKey) (bool, bool) {
	if ev.Modifiers()&ModAlt == 0 {
		return false, false
	}

	key := keyStringForMap(ev, e.keymap.normal)
	action, ok := e.keymap.normal[key]
	if !ok {
		return false, false
	}

	switch action {
	case actionToggleSidebar,
		actionToggleSidebarFocus,
		actionFocusEditor,
		actionFocusPrevPane,
		actionFocusNextPane,
		actionFocusSidebar,
		actionFocusCommandLine:
		return true, e.execAction(action)
	default:
		return false, false
	}
}

// yankToSystemClipboard copies selection to system clipboard
func (e *Editor) yankToSystemClipboard() {
	// First yank to internal clipboard
	e.yankSelection()

	// Then copy to system clipboard if available
	if len(e.clipboard) == 0 {
		return
	}

	// Build text from clipboard
	var sb strings.Builder
	for i, line := range e.clipboard {
		if i > 0 {
			sb.WriteRune('\n')
		}
		sb.WriteString(string(line))
	}

	// Try to copy to system clipboard
	if e.systemClipboard == nil {
		e.setStatus("yanked (clipboard unavailable)")
		return
	}
	if err := e.systemClipboard.Write(sb.String()); err != nil {
		e.setStatus("yanked (clipboard unavailable)")
		return
	}
	e.setStatus("yanked to clipboard")
}

// pasteFromSystemClipboard pastes from system clipboard
func (e *Editor) pasteFromSystemClipboard(before bool) {
	// Try to get from system clipboard
	if e.systemClipboard == nil {
		e.setStatus("clipboard unavailable")
		return
	}
	text, err := e.systemClipboard.Read()
	if err != nil {
		e.setStatus("clipboard unavailable")
		return
	}

	if text == "" {
		e.setStatus("clipboard empty")
		return
	}

	// Parse into lines
	lines := strings.Split(text, "\n")
	e.clipboard = make([][]rune, len(lines))
	for i, line := range lines {
		e.clipboard[i] = []rune(line)
	}

	// Paste
	if before {
		e.pasteBefore()
	} else {
		e.pasteAfter()
	}
	e.setStatus("pasted from clipboard")
}

func (e *Editor) handleInsert(ev EventKey) bool {
	if e.handleSelectionMove(ev) {
		return false
	}
	key := keyStringForMap(ev, e.keymap.insert)
	if key != "" {
		if action, ok := e.keymap.insert[key]; ok {
			return e.execAction(action)
		}
	}
	if ev.Key() == KeyRune {
		e.clearSelection()
		e.insertRune(ev.Rune())
	}
	return false
}
func (e *Editor) handleSelectionMove(ev EventKey) bool {
	if ev.Modifiers()&ModShift == 0 {
		return false
	}
	// Don't handle if Alt is pressed - let keymap handle alt+shift combinations
	if ev.Modifiers()&ModAlt != 0 {
		return false
	}
	switch ev.Key() {
	case KeyLeft:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveWordLeft)
		} else {
			e.extendSelection(e.moveLeft)
		}
		return true
	case KeyRight:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveWordRight)
		} else {
			e.extendSelection(e.moveRight)
		}
		return true
	case KeyUp:
		e.extendSelection(e.moveUp)
		return true
	case KeyDown:
		e.extendSelection(e.moveDown)
		return true
	case KeyPgUp:
		e.extendSelection(e.pageUp)
		return true
	case KeyPgDn:
		e.extendSelection(e.pageDown)
		return true
	case KeyHome:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveFileStart)
			return true
		}
		e.extendSelection(e.moveLineStart)
		return true
	case KeyEnd:
		if ev.Modifiers()&ModMeta != 0 {
			e.extendSelection(e.moveFileEnd)
			return true
		}
		e.extendSelection(e.moveLineEnd)
		return true
	}
	return false
}

// ConsumeSidebarOpenFile consumes the file path selected from sidebar.
func (e *Editor) ConsumeSidebarOpenFile() (string, bool) {
	if e.sidebarOpenFilePath == "" {
		return "", false
	}
	path := e.sidebarOpenFilePath
	e.sidebarOpenFilePath = ""
	return path, true
}

func keyString(ev EventKey) string {
	if ev.Key() == KeyUnknown {
		return ""
	}
	// Handle function keys with optional modifiers.
	if ev.Key() >= KeyF1 && ev.Key() <= KeyF12 {
		keyNum := int(ev.Key()-KeyF1) + 1
		base := fmt.Sprintf("f%d", keyNum)
		var parts []string
		if ev.Modifiers()&ModMeta != 0 {
			parts = append(parts, "cmd")
		}
		if ev.Modifiers()&ModCtrl != 0 {
			parts = append(parts, "ctrl")
		}
		if ev.Modifiers()&ModShift != 0 {
			parts = append(parts, "shift")
		}
		if ev.Modifiers()&ModAlt != 0 {
			parts = append(parts, "alt")
		}
		if len(parts) == 0 {
			return base
		}
		parts = append(parts, base)
		return strings.Join(parts, "+")
	}
	// Handle alt+shift+arrow combinations first
	if ev.Modifiers()&ModAlt != 0 && ev.Modifiers()&ModShift != 0 {
		switch ev.Key() {
		case KeyUp:
			return "alt+shift+up"
		case KeyDown:
			return "alt+shift+down"
		case KeyLeft:
			return "alt+shift+left"
		case KeyRight:
			return "alt+shift+right"
		}
	}
	// Handle alt+arrow combinations
	if ev.Modifiers()&ModAlt != 0 {
		switch ev.Key() {
		case KeyUp:
			return "alt+up"
		case KeyDown:
			return "alt+down"
		case KeyLeft:
			return "alt+left"
		case KeyRight:
			return "alt+right"
		}
		if ev.Key() == KeyRune {
			r := ev.Rune()
			if ev.Modifiers()&ModShift != 0 {
				if r == ' ' {
					return "alt+shift+space"
				}
				return "alt+shift+" + strings.ToLower(string(r))
			}
			if r == ' ' {
				return "alt+space"
			}
			return "alt+" + strings.ToLower(string(r))
		}
	}
	if ev.Modifiers()&ModCtrl != 0 {
		switch ev.Key() {
		case KeyHome:
			return "ctrl+home"
		case KeyEnd:
			return "ctrl+end"
		}
	}
	if ev.Modifiers()&ModMeta != 0 {
		if ev.Key() == KeyRune {
			r := ev.Rune()
			if ev.Modifiers()&ModShift != 0 {
				if r == ' ' {
					return "cmd+shift+space"
				}
				return "cmd+shift+" + strings.ToLower(string(r))
			}
			if r == ' ' {
				return "cmd+space"
			}
			return "cmd+" + strings.ToLower(string(r))
		}
		switch ev.Key() {
		case KeyBackspace, KeyBackspace2:
			return "cmd+backspace"
		case KeyDelete:
			return "cmd+del"
		case KeyEnter:
			return "cmd+enter"
		case KeyLeft:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+left"
			}
			return "cmd+left"
		case KeyRight:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+right"
			}
			return "cmd+right"
		case KeyUp:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+up"
			}
			return "cmd+up"
		case KeyDown:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+down"
			}
			return "cmd+down"
		case KeyHome:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+home"
			}
			return "cmd+home"
		case KeyEnd:
			if ev.Modifiers()&ModShift != 0 {
				return "cmd+shift+end"
			}
			return "cmd+end"
		}
	}
	if ev.Key() == KeyRune {
		r := ev.Rune()
		if r == ' ' {
			return "space"
		}
		return string(r)
	}
	// Check Tab before ctrlKeyName since KeyTab == KeyCtrlI (0x09)
	switch ev.Key() {
	case KeyTab:
		if ev.Modifiers()&ModShift != 0 {
			return "shift+tab"
		}
		return "tab"
	case KeyBacktab:
		return "shift+tab"
	}
	if name := ctrlKeyName(ev.Key()); name != "" {
		return name
	}
	switch ev.Key() {
	case KeyUp:
		return "up"
	case KeyDown:
		return "down"
	case KeyLeft:
		return "left"
	case KeyRight:
		return "right"
	case KeyPgUp:
		return "pgup"
	case KeyPgDn:
		return "pgdn"
	case KeyHome:
		return "home"
	case KeyEnd:
		return "end"
	case KeyBackspace, KeyBackspace2:
		return "backspace"
	case KeyEnter:
		if ev.Modifiers()&ModShift != 0 {
			return "shift+enter"
		}
		return "enter"
	case KeyDelete:
		return "del"
	case KeyEscape:
		return "esc"
	}
	return ""
}
func keyStringDisplay(ev EventKey) string {
	var parts []string

	// Build modifier prefix in order: CMD, CTRL, SHIFT, ALT
	if ev.Modifiers()&ModMeta != 0 {
		parts = append(parts, "CMD")
	}
	if ev.Modifiers()&ModCtrl != 0 {
		parts = append(parts, "CTRL")
	}
	if ev.Modifiers()&ModShift != 0 {
		parts = append(parts, "SHIFT")
	}
	if ev.Modifiers()&ModAlt != 0 {
		parts = append(parts, "ALT")
	}

	// Get key name
	var keyName string
	if ev.Key() == KeyRune {
		r := ev.Rune()
		if r == ' ' {
			keyName = "SPACE"
		} else {
			keyName = strings.ToUpper(string(r))
		}
	} else {
		switch ev.Key() {
		case KeyUnknown:
			keyName = ""
		case KeyUp:
			keyName = "UP"
		case KeyDown:
			keyName = "DOWN"
		case KeyLeft:
			keyName = "LEFT"
		case KeyRight:
			keyName = "RIGHT"
		case KeyPgUp:
			keyName = "PGUP"
		case KeyPgDn:
			keyName = "PGDN"
		case KeyHome:
			keyName = "HOME"
		case KeyEnd:
			keyName = "END"
		case KeyBackspace, KeyBackspace2:
			keyName = "BKSP"
		case KeyEnter:
			keyName = "ENTER"
		case KeyDelete:
			keyName = "DEL"
		case KeyEscape:
			keyName = "ESC"
		case KeyTab:
			keyName = "TAB"
		case KeyCtrlA:
			keyName = "A"
		case KeyCtrlB:
			keyName = "B"
		case KeyCtrlC:
			keyName = "C"
		case KeyCtrlD:
			keyName = "D"
		case KeyCtrlE:
			keyName = "E"
		case KeyCtrlF:
			keyName = "F"
		case KeyCtrlG:
			keyName = "G"
		case KeyCtrlH:
			keyName = "H"
		case KeyCtrlI:
			keyName = "I"
		case KeyCtrlJ:
			keyName = "J"
		case KeyCtrlK:
			keyName = "K"
		case KeyCtrlL:
			keyName = "L"
		case KeyCtrlM:
			keyName = "M"
		case KeyCtrlN:
			keyName = "N"
		case KeyCtrlO:
			keyName = "O"
		case KeyCtrlP:
			keyName = "P"
		case KeyCtrlQ:
			keyName = "Q"
		case KeyCtrlR:
			keyName = "R"
		case KeyCtrlS:
			keyName = "S"
		case KeyCtrlT:
			keyName = "T"
		case KeyCtrlU:
			keyName = "U"
		case KeyCtrlV:
			keyName = "V"
		case KeyCtrlW:
			keyName = "W"
		case KeyCtrlX:
			keyName = "X"
		case KeyCtrlY:
			keyName = "Y"
		case KeyCtrlZ:
			keyName = "Z"
		case KeyF1:
			keyName = "F1"
		case KeyF2:
			keyName = "F2"
		case KeyF3:
			keyName = "F3"
		case KeyF4:
			keyName = "F4"
		case KeyF5:
			keyName = "F5"
		case KeyF6:
			keyName = "F6"
		case KeyF7:
			keyName = "F7"
		case KeyF8:
			keyName = "F8"
		case KeyF9:
			keyName = "F9"
		case KeyF10:
			keyName = "F10"
		case KeyF11:
			keyName = "F11"
		case KeyF12:
			keyName = "F12"
		default:
			keyName = fmt.Sprintf("KEY%d", ev.Key())
		}
	}

	if keyName != "" {
		parts = append(parts, keyName)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "-")
}
func keyStringForMap(ev EventKey, keymap map[string]string) string {
	if ev.Modifiers()&ModMeta != 0 {
		switch ev.Key() {
		case KeyHome:
			if _, ok := keymap["cmd+left"]; ok {
				return "cmd+left"
			}
		case KeyEnd:
			if _, ok := keymap["cmd+right"]; ok {
				return "cmd+right"
			}
		}
	}
	return keyString(ev)
}
func ctrlKeyName(key Key) string {
	switch key {
	case KeyCtrlA:
		return "ctrl+a"
	case KeyCtrlB:
		return "ctrl+b"
	case KeyCtrlC:
		return "ctrl+c"
	case KeyCtrlD:
		return "ctrl+d"
	case KeyCtrlE:
		return "ctrl+e"
	case KeyCtrlF:
		return "ctrl+f"
	case KeyCtrlG:
		return "ctrl+g"
	case KeyCtrlH:
		return "ctrl+h"
	case KeyCtrlI:
		return "ctrl+i"
	case KeyCtrlJ:
		return "ctrl+j"
	case KeyCtrlK:
		return "ctrl+k"
	case KeyCtrlL:
		return "ctrl+l"
	case KeyCtrlM:
		return "ctrl+m"
	case KeyCtrlN:
		return "ctrl+n"
	case KeyCtrlO:
		return "ctrl+o"
	case KeyCtrlP:
		return "ctrl+p"
	case KeyCtrlQ:
		return "ctrl+q"
	case KeyCtrlR:
		return "ctrl+r"
	case KeyCtrlS:
		return "ctrl+s"
	case KeyCtrlT:
		return "ctrl+t"
	case KeyCtrlU:
		return "ctrl+u"
	case KeyCtrlV:
		return "ctrl+v"
	case KeyCtrlW:
		return "ctrl+w"
	case KeyCtrlX:
		return "ctrl+x"
	case KeyCtrlY:
		return "ctrl+y"
	case KeyCtrlZ:
		return "ctrl+z"
	}
	return ""
}
