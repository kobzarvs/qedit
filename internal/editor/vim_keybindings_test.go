package editor

import (
	"strconv"
	"strings"
	"testing"
)

func TestVimOperatorPendingCountWaitsForMotion(t *testing.T) {
	e := newVimTutorEditor(numberedLines(12)...)

	vimPressRunes(t, e, "d10")

	if got := e.Content(); got != strings.Join(numberedLines(12), "\n") {
		t.Fatalf("content changed while waiting for motion: %q", got)
	}
	if e.profile.vim.operator != "d" || e.profile.vim.count != "10" {
		t.Fatalf("pending operator=%q count=%q, want d/10", e.profile.vim.operator, e.profile.vim.count)
	}
	if e.modal.pendingKeys != "d10" {
		t.Fatalf("pending keys = %q, want d10", e.modal.pendingKeys)
	}

	vimPressRunes(t, e, "j")

	if got, want := e.Content(), "12"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestVimRepeatedOperatorAcceptsCountAfterOperator(t *testing.T) {
	e := newVimTutorEditor(numberedLines(12)...)

	vimPressRunes(t, e, "d10d")

	if got, want := e.Content(), strings.Join([]string{"11", "12"}, "\n"); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestVimCountBeforeAndAfterOperatorMultiply(t *testing.T) {
	e := newVimTutorEditor(numberedLines(10)...)

	vimPressRunes(t, e, "2d3d")

	if got, want := e.Content(), strings.Join([]string{"7", "8", "9", "10"}, "\n"); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestVimGotoMotionsWorkInNormalAndOperatorPending(t *testing.T) {
	e := newVimTutorEditor(numberedLines(5)...)

	vimPressRunes(t, e, "3gg")
	if e.cursor.Row != 2 {
		t.Fatalf("cursor row = %d, want 2", e.cursor.Row)
	}

	vimPressRunes(t, e, "dgg")
	if got, want := e.Content(), strings.Join([]string{"4", "5"}, "\n"); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestVimLineStartMotionsWorkInOperatorPending(t *testing.T) {
	e := newVimTutorEditor("  alpha beta")
	e.cursor = Cursor{Row: 0, Col: len([]rune("  alpha"))}

	vimPressRunes(t, e, "d0")
	if got, want := e.Content(), " beta"; got != want {
		t.Fatalf("d0 content = %q, want %q", got, want)
	}

	e = newVimTutorEditor("  alpha beta")
	e.cursor = Cursor{Row: 0, Col: len([]rune("  alpha"))}
	vimPressRunes(t, e, "d^")
	if got, want := e.Content(), "   beta"; got != want {
		t.Fatalf("d^ content = %q, want %q", got, want)
	}
}

func TestVimNativeNormalModeCommandsDoNotDependOnHelixKeymap(t *testing.T) {
	e := New(Options{
		Profile:      BehaviorProfileVim,
		KeymapNormal: map[string]string{},
		KeymapInsert: map[string]string{"esc": actionEnterNormal},
	})
	e.text = NewTextBufferFromString("abc def")

	vimPressRunes(t, e, "dw")

	if got, want := e.Content(), "def"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestVimDeleteChangeSubstituteAndBackspaceCommands(t *testing.T) {
	e := newVimTutorEditor("abc def")
	e.cursor = Cursor{Row: 0, Col: 4}
	vimPressRunes(t, e, "D")
	if got, want := e.Content(), "abc "; got != want {
		t.Fatalf("D content = %q, want %q", got, want)
	}

	e = newVimTutorEditor("abc def")
	e.cursor = Cursor{Row: 0, Col: 4}
	vimPressRunes(t, e, "Cxyz")
	vimPressKey(t, e, "esc")
	if got, want := e.Content(), "abc xyz"; got != want {
		t.Fatalf("C content = %q, want %q", got, want)
	}

	e = newVimTutorEditor("abc")
	e.cursor = Cursor{Row: 0, Col: 1}
	vimPressRunes(t, e, "sZ")
	vimPressKey(t, e, "esc")
	if got, want := e.Content(), "aZc"; got != want {
		t.Fatalf("s content = %q, want %q", got, want)
	}

	e = newVimTutorEditor("abcdef")
	e.cursor = Cursor{Row: 0, Col: 1}
	vimPressRunes(t, e, "3sZ")
	vimPressKey(t, e, "esc")
	if got, want := e.Content(), "aZef"; got != want {
		t.Fatalf("3s content = %q, want %q", got, want)
	}

	e = newVimTutorEditor("abc")
	e.cursor = Cursor{Row: 0, Col: 2}
	vimPressRunes(t, e, "X")
	if got, want := e.Content(), "ac"; got != want {
		t.Fatalf("X content = %q, want %q", got, want)
	}
}

func TestVimChangeWholeLineKeepsEditableLine(t *testing.T) {
	e := newVimTutorEditor("one", "two", "three")
	e.cursor = Cursor{Row: 1, Col: 1}

	vimPressRunes(t, e, "ccchanged")
	vimPressKey(t, e, "esc")

	if got, want := e.Content(), strings.Join([]string{"one", "changed", "three"}, "\n"); got != want {
		t.Fatalf("cc content = %q, want %q", got, want)
	}

	e = newVimTutorEditor("one", "two", "three")
	e.cursor = Cursor{Row: 1, Col: 1}
	vimPressRunes(t, e, "Schanged")
	vimPressKey(t, e, "esc")

	if got, want := e.Content(), strings.Join([]string{"one", "changed", "three"}, "\n"); got != want {
		t.Fatalf("S content = %q, want %q", got, want)
	}
}

func TestVimTutorCommandModeFileAndShellCommands(t *testing.T) {
	t.Run("read file below cursor", func(t *testing.T) {
		e := newVimTutorEditor("top")
		store := &testFileStore{
			absPaths: map[string]string{"read.txt": "/tmp/read.txt"},
			readDataByPath: map[string][]byte{
				"/tmp/read.txt": []byte("alpha\nbeta\n"),
			},
		}
		e.SetFileStore(store)

		vimPressRunes(t, e, ":r read.txt")
		vimPressKey(t, e, "enter")

		if got, want := e.Content(), "top\nalpha\nbeta"; got != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
	})

	t.Run("read shell output below cursor", func(t *testing.T) {
		e := newVimTutorEditor("top")

		vimPressRunes(t, e, ":r !printf 'cmdline\\n'")
		vimPressKey(t, e, "enter")

		if got, want := e.Content(), "top\ncmdline"; got != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
	})

	t.Run("shell command reports output", func(t *testing.T) {
		e := newVimTutorEditor("top")

		vimPressRunes(t, e, ":!printf ok")
		vimPressKey(t, e, "enter")

		if got, want := e.ui.statusMessage, "shell: ok"; got != want {
			t.Fatalf("status = %q, want %q", got, want)
		}
	})

	t.Run("visual range write", func(t *testing.T) {
		e := newVimTutorEditor("one", "two", "three")
		store := &testFileStore{absPaths: map[string]string{"out.txt": "/tmp/out.txt"}}
		e.SetFileStore(store)

		vimPressRunes(t, e, "Vj:w out.txt")
		vimPressKey(t, e, "enter")

		if store.writtenPath != "/tmp/out.txt" {
			t.Fatalf("written path = %q, want /tmp/out.txt", store.writtenPath)
		}
		if got, want := string(store.writtenData), "one\ntwo\n"; got != want {
			t.Fatalf("written data = %q, want %q", got, want)
		}
		if e.profile.vim.visual || e.selectionActive {
			t.Fatalf("visual=%v selection=%v, want cleared", e.profile.vim.visual, e.selectionActive)
		}
	})
}

func TestVimTutorStatusAndWindowKeys(t *testing.T) {
	t.Run("ctrl-g reports file location", func(t *testing.T) {
		e := newVimTutorEditor("one", "two")
		e.document.filename = "file.txt"
		e.cursor = Cursor{Row: 1, Col: 2}

		vimPressKey(t, e, "ctrl+g")

		if !strings.Contains(e.ui.statusMessage, `"file.txt"`) ||
			!strings.Contains(e.ui.statusMessage, "line 2 of 2") ||
			!strings.Contains(e.ui.statusMessage, "col 3") {
			t.Fatalf("status = %q, want file/line/col info", e.ui.statusMessage)
		}
	})

	t.Run("ctrl-w ctrl-w cycles windows", func(t *testing.T) {
		e := newVimTutorEditor("one")

		vimPressKey(t, e, "ctrl+w")
		vimPressRunes(t, e, "v")
		rightID := e.windows.activeID
		vimPressKey(t, e, "ctrl+w")
		vimPressKey(t, e, "ctrl+w")

		if got := e.windowCount(); got != 2 {
			t.Fatalf("window count = %d, want 2", got)
		}
		if e.windows.activeID == rightID {
			t.Fatalf("ctrl-w ctrl-w did not cycle to another window")
		}
	})
}

func numberedLines(count int) []string {
	lines := make([]string, count)
	for i := range lines {
		lines[i] = strconv.Itoa(i + 1)
	}
	return lines
}
