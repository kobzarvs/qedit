package editor

import (
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
)

func TestDefaultNormalHotkeysTriggerActions(t *testing.T) {
	cfg := config.Default()
	keys := sortedKeys(cfg.Keymap.Normal)
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			if shouldSkipHotkey(key, cfg.Keymap.Normal) {
				t.Skip("skipping zoom hotkey")
			}
			e := newTestEditor("one", "two", "three")
			var got []string
			e.actionHook = func(action string) {
				got = append(got, action)
			}
			_ = e.HandleKey(eventForKeyString(t, key))
			if len(got) == 0 {
				t.Fatalf("no action executed for %q", key)
			}
			if len(got) > 1 {
				t.Fatalf("multiple actions executed for %q: %v", key, got)
			}
			want := expectedActionForKey(key, cfg.Keymap.Normal)
			if got[0] != want {
				t.Fatalf("action = %q, want %q", got[0], want)
			}
		})
	}
}

func TestDefaultInsertHotkeysTriggerActions(t *testing.T) {
	cfg := config.Default()
	keys := sortedKeys(cfg.Keymap.Insert)
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			if shouldSkipHotkey(key, cfg.Keymap.Insert) {
				t.Skip("skipping zoom hotkey")
			}
			e := newTestEditor("one", "two", "three")
			e.mode = ModeInsert
			var got []string
			e.actionHook = func(action string) {
				got = append(got, action)
			}
			_ = e.HandleKey(eventForKeyString(t, key))
			if len(got) == 0 {
				t.Fatalf("no action executed for %q", key)
			}
			if len(got) > 1 {
				t.Fatalf("multiple actions executed for %q: %v", key, got)
			}
			want := expectedActionForKey(key, cfg.Keymap.Insert)
			if got[0] != want {
				t.Fatalf("action = %q, want %q", got[0], want)
			}
		})
	}
}

func expectedActionForKey(key string, keymap map[string]string) string {
	if key == "cmd+home" {
		if _, ok := keymap["cmd+left"]; ok {
			key = "cmd+left"
		}
	}
	if key == "cmd+end" {
		if _, ok := keymap["cmd+right"]; ok {
			key = "cmd+right"
		}
	}
	return keymap[key]
}

func shouldSkipHotkey(key string, keymap map[string]string) bool {
	return expectedActionForKey(key, keymap) == actionTerminalZoomIn
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func eventForKeyString(t *testing.T, key string) *tcell.EventKey {
	t.Helper()
	parts := strings.Split(key, "+")
	base := parts[len(parts)-1]
	var mod tcell.ModMask
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "ctrl":
			mod |= tcell.ModCtrl
		case "cmd":
			mod |= tcell.ModMeta
		case "alt":
			mod |= tcell.ModAlt
		case "shift":
			mod |= tcell.ModShift
		default:
			t.Fatalf("unknown modifier %q in %q", part, key)
		}
	}

	if mod&tcell.ModCtrl != 0 && base != "home" && base != "end" {
		if r := []rune(base); len(r) == 1 {
			if ctrlKey := ctrlKeyForRune(r[0]); ctrlKey != 0 {
				return tcell.NewEventKey(ctrlKey, 0, 0)
			}
		}
	}

	switch base {
	case "left":
		return tcell.NewEventKey(tcell.KeyLeft, 0, mod)
	case "right":
		return tcell.NewEventKey(tcell.KeyRight, 0, mod)
	case "up":
		return tcell.NewEventKey(tcell.KeyUp, 0, mod)
	case "down":
		return tcell.NewEventKey(tcell.KeyDown, 0, mod)
	case "home":
		return tcell.NewEventKey(tcell.KeyHome, 0, mod)
	case "end":
		return tcell.NewEventKey(tcell.KeyEnd, 0, mod)
	case "pgup":
		return tcell.NewEventKey(tcell.KeyPgUp, 0, mod)
	case "pgdn":
		return tcell.NewEventKey(tcell.KeyPgDn, 0, mod)
	case "enter":
		return tcell.NewEventKey(tcell.KeyEnter, 0, mod)
	case "backspace":
		return tcell.NewEventKey(tcell.KeyBackspace, 0, mod)
	case "del":
		return tcell.NewEventKey(tcell.KeyDelete, 0, mod)
	case "tab":
		return tcell.NewEventKey(tcell.KeyTab, 0, mod)
	case "esc":
		return tcell.NewEventKey(tcell.KeyEscape, 0, mod)
	case "space":
		return tcell.NewEventKey(tcell.KeyRune, ' ', mod)
	}

	if r := []rune(base); len(r) == 1 {
		return tcell.NewEventKey(tcell.KeyRune, r[0], mod)
	}

	t.Fatalf("unsupported key %q", key)
	return nil
}

func ctrlKeyForRune(r rune) tcell.Key {
	switch unicode.ToLower(r) {
	case 'a':
		return tcell.KeyCtrlA
	case 'b':
		return tcell.KeyCtrlB
	case 'c':
		return tcell.KeyCtrlC
	case 'd':
		return tcell.KeyCtrlD
	case 'e':
		return tcell.KeyCtrlE
	case 'f':
		return tcell.KeyCtrlF
	case 'g':
		return tcell.KeyCtrlG
	case 'h':
		return tcell.KeyCtrlH
	case 'i':
		return tcell.KeyCtrlI
	case 'j':
		return tcell.KeyCtrlJ
	case 'k':
		return tcell.KeyCtrlK
	case 'l':
		return tcell.KeyCtrlL
	case 'm':
		return tcell.KeyCtrlM
	case 'n':
		return tcell.KeyCtrlN
	case 'o':
		return tcell.KeyCtrlO
	case 'p':
		return tcell.KeyCtrlP
	case 'q':
		return tcell.KeyCtrlQ
	case 'r':
		return tcell.KeyCtrlR
	case 's':
		return tcell.KeyCtrlS
	case 't':
		return tcell.KeyCtrlT
	case 'u':
		return tcell.KeyCtrlU
	case 'v':
		return tcell.KeyCtrlV
	case 'w':
		return tcell.KeyCtrlW
	case 'x':
		return tcell.KeyCtrlX
	case 'y':
		return tcell.KeyCtrlY
	case 'z':
		return tcell.KeyCtrlZ
	}
	return 0
}
