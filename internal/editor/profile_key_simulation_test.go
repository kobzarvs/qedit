package editor

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func newSimulatedProfileEditor(profile string, lines ...string) *Editor {
	e := newTestEditor(lines...)
	e.SetBehaviorProfile(profile)
	if profile == BehaviorProfileBasic {
		e.mode = ModeInsert
	} else {
		e.mode = ModeNormal
	}
	return e
}

func pressKeyScript(t *testing.T, e *Editor, script string) {
	t.Helper()
	for len(script) > 0 {
		if script[0] == '<' {
			end := strings.IndexByte(script, '>')
			if end < 0 {
				t.Fatalf("unterminated key token in %q", script)
			}
			token := script[1:end]
			switch token {
			case "lt":
				e.HandleKey(keyRune('<'))
			default:
				e.HandleKey(eventForKeyString(t, token))
			}
			script = script[end+1:]
			continue
		}
		r, size := utf8.DecodeRuneInString(script)
		if r == utf8.RuneError && size == 0 {
			return
		}
		e.HandleKey(keyRune(r))
		script = script[size:]
	}
}

func assertSimulatedResult(t *testing.T, e *Editor, wantContent string, wantCursor *Cursor, wantMode Mode) {
	t.Helper()
	if got := e.Content(); got != wantContent {
		t.Fatalf("content = %q, want %q", got, wantContent)
	}
	if wantCursor != nil && e.cursor != *wantCursor {
		t.Fatalf("cursor = %+v, want %+v", e.cursor, *wantCursor)
	}
	if e.mode != wantMode {
		t.Fatalf("mode = %v, want %v", e.mode, wantMode)
	}
}

func ptrCursor(cursor Cursor) *Cursor {
	return &cursor
}

func profileKeySimulationFingerprint(e *Editor) string {
	return fmt.Sprintf(
		"mode=%d content=%q cursor=%+v pending=%q pendingAction=%q vimOp=%q vimCnt=%q helixCnt=%q visual=%v select=%v windows=%d status=%q jumps=%d/%d registers=%d clipboard=%q",
		e.mode,
		e.Content(),
		e.cursor,
		e.modal.pendingKeys,
		e.modal.pendingAction,
		e.profile.vim.operator,
		e.profile.vim.count,
		e.profile.helix.count,
		e.profile.vim.visual,
		e.selectionActive,
		e.windowCount(),
		e.ui.statusMessage,
		len(e.profile.jumps),
		e.profile.jumpIndex,
		len(e.profile.vim.registers),
		e.clipboard.lines,
	)
}

func profileCommandExercisedNames(profile string) []string {
	names := make([]string, 0, len(profileCommandExercises))
	for _, ex := range profileCommandExercises {
		if ex.profile == profile {
			names = append(names, ex.command)
		}
	}
	return names
}
