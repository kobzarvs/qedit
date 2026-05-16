package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelixProfilePassesCoreTutorExercises(t *testing.T) {
	lines := loadHelixTutor(t)

	t.Run("delete stray characters with d", func(t *testing.T) {
		e := newHelixTutorEditor(helixExerciseLine(t, lines, 103))

		helixDeleteCharAt(t, e, "Thhiss", 1)
		helixDeleteCharAt(t, e, "Thiss", 3)
		helixDeleteCharAt(t, e, "senttencee", 4)
		helixDeleteCharAt(t, e, "sentencee", 8)
		helixDeleteCharAt(t, e, "haass", 1)
		helixDeleteCharAt(t, e, "hass", 3)
		helixDeleteCharAt(t, e, "exxtra", 2)
		helixDeleteCharAt(t, e, "charracterss", 4)
		helixDeleteCharAt(t, e, "characterss", 10)

		assertHelixContent(t, e, helixExpectedLine(t, lines, 104))
	})

	t.Run("insert missing text with i", func(t *testing.T) {
		e := newHelixTutorEditor(helixExerciseLine(t, lines, 128))

		helixInsertBefore(t, e, " stce", 0, "is")
		helixInsertBefore(t, e, "tce", 0, "en")
		helixInsertBefore(t, e, "ce", 0, "en")
		helixInsertBefore(t, e, "misg", 0, "is ")
		helixInsertBefore(t, e, "g so", 0, "sin")
		helixInsertBefore(t, e, ".", 0, "me text")

		assertHelixContent(t, e, helixExpectedLine(t, lines, 129))
	})

	t.Run("append line endings with A", func(t *testing.T) {
		e := newHelixTutorEditor(helixExerciseLine(t, lines, 200))

		helixPressRunes(t, e, "A")
		helixPressRunes(t, e, "ing some text.")
		helixPressKey(t, e, "esc")

		assertHelixContent(t, e, helixExpectedLine(t, lines, 201))
	})

	t.Run("delete word selections with w", func(t *testing.T) {
		e := newHelixTutorEditor(helixExerciseLine(t, lines, 263))

		helixDeleteWordSelectionAt(t, e, "pencil")
		helixDeleteWordSelectionAt(t, e, "vacuum")
		helixDeleteWordSelectionAt(t, e, "the it")

		assertHelixContent(t, e, helixExpectedLine(t, lines, 264))
	})

	t.Run("change word selections with e and c", func(t *testing.T) {
		e := newHelixTutorEditor(helixExerciseLine(t, lines, 329))

		helixChangeWordAt(t, e, "paper", "sentence")
		helixChangeWordAt(t, e, "heavy", "incorrect")
		helixChangeWordAt(t, e, "behind", "in")

		assertHelixContent(t, e, helixExpectedLine(t, lines, 330))
	})

	t.Run("delete select-mode counted word ranges", func(t *testing.T) {
		e := newHelixTutorEditor(helixExerciseLine(t, lines, 372))

		helixPlaceOn(t, e, 0, "FOO", 0)
		helixPressRunes(t, e, "v2wd")
		helixPlaceOn(t, e, 0, "BAZ", 0)
		helixPressRunes(t, e, "v2wd")

		assertHelixContent(t, e, "Remove the distracting words from this line.")
	})

	t.Run("delete whole-line selections with x", func(t *testing.T) {
		e := newHelixTutorEditor(
			helixExerciseLine(t, lines, 390),
			helixExerciseLine(t, lines, 391),
			helixExerciseLine(t, lines, 392),
			helixExerciseLine(t, lines, 393),
			helixExerciseLine(t, lines, 394),
			helixExerciseLine(t, lines, 395),
			helixExerciseLine(t, lines, 396),
		)

		e.cursor = Cursor{Row: 1, Col: 0}
		helixPressRunes(t, e, "xd")
		e.cursor = Cursor{Row: 2, Col: 0}
		helixPressRunes(t, e, "2xd")

		assertHelixContent(t, e, strings.Join([]string{
			"1) Roses are red,",
			"3) Violets are blue,",
			"6) Sugar is sweet,",
			"7) And so are you.",
		}, "\n"))
	})

	t.Run("undo and redo fixes", func(t *testing.T) {
		e := newHelixTutorEditor(helixExerciseLine(t, lines, 458))
		want := "Fix the errors on this line and replace them with undo."

		helixDeleteCharAt(t, e, "Fiix", 1)
		helixPressRunes(t, e, "u")
		assertHelixContent(t, e, helixExerciseLine(t, lines, 458))

		helixDeleteCharAt(t, e, "Fiix", 1)
		helixDeleteCharAt(t, e, "thhis", 2)
		helixDeleteCharAt(t, e, "reeplace", 1)
		helixDeleteCharAt(t, e, "witth", 2)
		assertHelixContent(t, e, want)

		helixPressRunes(t, e, "u")
		if e.Content() == want {
			t.Fatalf("content still fixed after undo")
		}
		helixPressRunes(t, e, "U")
		assertHelixContent(t, e, want)
	})

	t.Run("yank and paste selections", func(t *testing.T) {
		e := newHelixTutorEditor(helixExerciseLine(t, lines, 481))

		helixPlaceOn(t, e, 0, "banana", 0)
		helixPressRunes(t, e, "wy")
		helixPlaceOn(t, e, 0, " 3", 0)
		helixPressRunes(t, e, "p")
		helixPlaceOn(t, e, 0, " 4", 0)
		helixPressRunes(t, e, "p")

		assertHelixContent(t, e, helixExpectedLine(t, lines, 482))
	})
}

func TestHelixProfilePassesWindowTutorExercises(t *testing.T) {
	t.Run("new splits and movement", func(t *testing.T) {
		e := newHelixTutorEditor("tutor")

		helixPressKey(t, e, "ctrl+w")
		helixPressRunes(t, e, "nv")
		if got := e.windowCount(); got != 2 {
			t.Fatalf("window count after ctrl-w nv = %d, want 2", got)
		}
		if got := e.BufferCount(); got != 2 {
			t.Fatalf("buffer count after ctrl-w nv = %d, want 2", got)
		}
		rightID := e.windows.activeID

		helixPressKey(t, e, "ctrl+w")
		helixPressRunes(t, e, "ns")
		if got := e.windowCount(); got != 3 {
			t.Fatalf("window count after ctrl-w ns = %d, want 3", got)
		}
		if got := e.BufferCount(); got != 3 {
			t.Fatalf("buffer count after ctrl-w ns = %d, want 3", got)
		}

		helixPressKey(t, e, "ctrl+w")
		helixPressRunes(t, e, "h")
		if e.windows.activeID == rightID {
			t.Fatalf("ctrl-w h did not leave the right split")
		}
		helixPressKey(t, e, "ctrl+w")
		helixPressRunes(t, e, "w")
		if got := e.windowCount(); got != 3 {
			t.Fatalf("window count after ctrl-w w = %d, want 3", got)
		}
	})

	t.Run("current-buffer split close and only", func(t *testing.T) {
		e := newHelixTutorEditor("tutor")

		helixPressKey(t, e, "ctrl+w")
		helixPressRunes(t, e, "v")
		helixPressKey(t, e, "ctrl+w")
		helixPressRunes(t, e, "s")
		if got := e.windowCount(); got != 3 {
			t.Fatalf("window count after current-buffer splits = %d, want 3", got)
		}

		helixPressKey(t, e, "ctrl+w")
		helixPressRunes(t, e, "q")
		if got := e.windowCount(); got != 2 {
			t.Fatalf("window count after ctrl-w q = %d, want 2", got)
		}

		helixPressKey(t, e, "ctrl+w")
		helixPressRunes(t, e, "o")
		if got := e.windowCount(); got != 1 {
			t.Fatalf("window count after ctrl-w o = %d, want 1", got)
		}
	})

	t.Run("command splits and swap transpose", func(t *testing.T) {
		e := newHelixTutorEditor("tutor")

		helixPressRunes(t, e, ":vs hello1")
		helixPressKey(t, e, "enter")
		helixPressRunes(t, e, ":hs hello2")
		helixPressKey(t, e, "enter")
		if got := e.windowCount(); got != 3 {
			t.Fatalf("window count after :vs/:hs = %d, want 3", got)
		}
		if got := e.BufferCount(); got != 3 {
			t.Fatalf("buffer count after :vs/:hs = %d, want 3", got)
		}

		helixPressKey(t, e, "ctrl+w")
		helixPressRunes(t, e, "K")
		if !strings.HasSuffix(e.document.filename, "hello2") {
			t.Fatalf("active file after ctrl-w K = %q, want hello2", e.document.filename)
		}
		helixPressKey(t, e, "ctrl+w")
		helixPressRunes(t, e, "t")
		if parent := e.activeWindowLeaf().parent; parent == nil || parent.axis != editorWindowVertical {
			t.Fatalf("active parent axis = %#v, want vertical after transpose", parent)
		}
	})
}

func loadHelixTutor(t *testing.T) []string {
	t.Helper()
	path := filepath.Join("testdata", "helix-tutor")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

func helixExerciseLine(t *testing.T, lines []string, lineNumber int) string {
	t.Helper()
	line := strings.TrimSpace(tutorLine(t, lines, lineNumber))
	return strings.TrimSpace(strings.TrimPrefix(line, "-->"))
}

func helixExpectedLine(t *testing.T, lines []string, lineNumber int) string {
	t.Helper()
	return strings.TrimSpace(tutorLine(t, lines, lineNumber))
}

func newHelixTutorEditor(lines ...string) *Editor {
	e := newTestEditor(lines...)
	e.SetBehaviorProfile(BehaviorProfileHelix)
	e.mode = ModeNormal
	return e
}

func helixPressRunes(t *testing.T, e *Editor, keys string) {
	t.Helper()
	for _, r := range keys {
		e.HandleKey(wrapKey(tcellEventRune(r)))
	}
}

func helixPressKey(t *testing.T, e *Editor, key string) {
	t.Helper()
	e.HandleKey(eventForKeyString(t, key))
}

func helixPlaceOn(t *testing.T, e *Editor, row int, needle string, offset int) {
	t.Helper()
	if row < 0 || row >= e.LineCount() {
		t.Fatalf("row %d out of range", row)
	}
	col := strings.Index(string(e.line(row)), needle)
	if col < 0 {
		t.Fatalf("line %d %q does not contain %q", row, string(e.line(row)), needle)
	}
	e.cursor = Cursor{Row: row, Col: col + offset}
}

func helixDeleteCharAt(t *testing.T, e *Editor, needle string, offset int) {
	t.Helper()
	helixPlaceOn(t, e, 0, needle, offset)
	helixPressRunes(t, e, "d")
}

func helixInsertBefore(t *testing.T, e *Editor, needle string, offset int, text string) {
	t.Helper()
	helixPlaceOn(t, e, 0, needle, offset)
	helixPressRunes(t, e, "i")
	helixPressRunes(t, e, text)
	helixPressKey(t, e, "esc")
}

func helixDeleteWordSelectionAt(t *testing.T, e *Editor, needle string) {
	t.Helper()
	helixPlaceOn(t, e, 0, needle, 0)
	helixPressRunes(t, e, "wd")
}

func helixChangeWordAt(t *testing.T, e *Editor, needle string, replacement string) {
	t.Helper()
	helixPlaceOn(t, e, 0, needle, 0)
	helixPressRunes(t, e, "ec")
	helixPressRunes(t, e, replacement)
	helixPressKey(t, e, "esc")
}

func assertHelixContent(t *testing.T, e *Editor, want string) {
	t.Helper()
	if got := e.Content(); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	if e.BehaviorProfile() != BehaviorProfileHelix {
		t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileHelix)
	}
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want %v", e.mode, ModeNormal)
	}
}
