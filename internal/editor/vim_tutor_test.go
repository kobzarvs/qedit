package editor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

type vimTutorExpectations struct {
	Expect map[string]json.RawMessage `json:"expect"`
}

func TestVimProfilePassesBeginnerTutorExercises(t *testing.T) {
	lines, expects := loadVimBeginnerTutor(t)

	t.Run("delete stray characters with x", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 105))

		vimDeleteCharAt(t, e, "ccow", 0)
		vimDeleteCharAt(t, e, "jumpedd", 6)
		vimDeleteCharAt(t, e, "ovverr", 2)
		vimDeleteCharAt(t, e, "overr", 4)
		vimDeleteCharAt(t, e, "thhe", 2)
		vimDeleteCharAt(t, e, "mooon", 2)

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 105))
	})

	t.Run("insert missing text with i", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 126))

		vimInsertBefore(t, e, "text", 0, "some ")
		vimInsertBefore(t, e, "ng", 0, "si")
		vimInsertBefore(t, e, "this", 0, "from ")
		vimInsertBefore(t, e, ".", 0, "line")

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 126))
	})

	t.Run("append line endings with A", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 146), tutorLine(t, lines, 148))

		e.cursor = Cursor{Row: 0, Col: 0}
		vimPressRunes(t, e, "A")
		vimPressRunes(t, e, "is line.")
		vimPressKey(t, e, "esc")
		e.cursor = Cursor{Row: 1, Col: 0}
		vimPressRunes(t, e, "A")
		vimPressRunes(t, e, "ing here.")
		vimPressKey(t, e, "esc")

		assertTutorContent(t, e, strings.Join([]string{
			tutorExpectedLine(t, expects, 146),
			tutorExpectedLine(t, expects, 148),
		}, "\n"))
	})

	t.Run("delete words with dw", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 222))

		vimDeleteWordAt(t, e, "a some")
		vimDeleteWordAt(t, e, "fun")
		vimDeleteWordAt(t, e, "paper")

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 222))
	})

	t.Run("delete to end of line with d$", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 238))

		vimPlaceOn(t, e, 0, ". end", 1)
		vimPressRunes(t, e, "d$")

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 238))
	})

	t.Run("delete counted motions", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 297))

		vimPlaceOn(t, e, 0, "ABC", 0)
		vimPressRunes(t, e, "d2w")
		vimPlaceOn(t, e, 0, "FGHI", 0)
		vimPressRunes(t, e, "d4w")
		vimPlaceOn(t, e, 0, "Q", 0)
		vimPressRunes(t, e, "d3w")

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 297))
	})

	t.Run("delete whole lines with dd", func(t *testing.T) {
		e := newVimTutorEditor(
			tutorLine(t, lines, 311),
			tutorLine(t, lines, 312),
			tutorLine(t, lines, 313),
			tutorLine(t, lines, 314),
			tutorLine(t, lines, 315),
			tutorLine(t, lines, 316),
			tutorLine(t, lines, 317),
		)

		e.cursor = Cursor{Row: 1, Col: 0}
		vimPressRunes(t, e, "dd")
		e.cursor = Cursor{Row: 2, Col: 0}
		vimPressRunes(t, e, "2dd")

		assertTutorContent(t, e, strings.Join([]string{
			"1)  Roses are red,",
			"3)  Violets are blue,",
			"6)  Sugar is sweet",
			"7)  And so are you.",
		}, "\n"))
	})

	t.Run("undo line with U and redo with ctrl-r", func(t *testing.T) {
		original := tutorLine(t, lines, 334)
		want := tutorExpectedLine(t, expects, 334)
		e := newVimTutorEditor(original)

		vimDeleteCharAt(t, e, "Fiix", 1)
		vimPressRunes(t, e, "u")
		assertTutorContent(t, e, original)

		vimDeleteCharAt(t, e, "Fiix", 1)
		vimDeleteCharAt(t, e, "oon", 1)
		vimDeleteCharAt(t, e, "thhis", 2)
		vimDeleteCharAt(t, e, "reeplace", 1)
		vimDeleteCharAt(t, e, "witth", 2)
		assertTutorContent(t, e, want)

		vimPressRunes(t, e, "U")
		assertTutorContent(t, e, original)
		vimPressRunes(t, e, "u")
		assertTutorContent(t, e, want)
		vimPressKey(t, e, "ctrl+r")
		assertTutorContent(t, e, original)
		vimPressRunes(t, e, "u")
		assertTutorContent(t, e, want)
	})

	t.Run("replace single characters with r", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 391))

		vimReplaceCharAt(t, e, "Whan", 2, 'e')
		vimReplaceCharAt(t, e, "lime", 2, 'n')
		vimReplaceCharAt(t, e, "tuoed", 1, 'y')
		vimReplaceCharAt(t, e, "tyoed", 2, 'p')
		vimReplaceCharAt(t, e, "presswd", 5, 'e')
		vimReplaceCharAt(t, e, "wrojg", 3, 'n')

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 391))
	})

	t.Run("change words with ce", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 413))

		vimChangeTo(t, e, "lubw", 1, "ine")
		vimChangeTo(t, e, "wptfd", 0, "words")
		vimChangeTo(t, e, "mrrf", 0, "need")
		vimChangeTo(t, e, "usf", 0, "using")

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 413))
	})

	t.Run("change to end of line with c$", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 434))

		vimPlaceOn(t, e, 0, "some help", 0)
		vimPressRunes(t, e, "c$")
		vimPressRunes(t, e, "to be corrected using the c$ command.")
		vimPressKey(t, e, "esc")

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 434))
	})

	t.Run("substitute current line with :s", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 543))

		vimPressRunes(t, e, ":s/thee/the/g")
		vimPressKey(t, e, "enter")

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 543))
	})

	t.Run("append after cursor with a", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 761))

		vimAppendAfter(t, e, "li", 1, "ne")
		vimAppendAfter(t, e, "pract", 4, "ice")
		vimAppendAfter(t, e, "appendi", 6, "ng")

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 761))
	})

	t.Run("replace multiple characters with R", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 782))

		vimReplaceTextAt(t, e, "xxx", 0, "456")
		vimReplaceTextAt(t, e, "xxx", 0, "579")

		assertTutorContent(t, e, tutorExpectedLine(t, expects, 782))
	})

	t.Run("copy and paste with visual yanks", func(t *testing.T) {
		e := newVimTutorEditor(tutorLine(t, lines, 809), tutorLine(t, lines, 810))

		e.cursor = Cursor{Row: 0, Col: 2}
		vimPressRunes(t, e, "v13ly")
		vimPressRunes(t, e, "j$p")
		vimPressRunes(t, e, "a")
		vimPressRunes(t, e, "second")
		vimPressKey(t, e, "esc")
		vimPlaceOn(t, e, 0, " item.", 0)
		vimPressRunes(t, e, "v6ly")
		e.cursor = Cursor{Row: 1, Col: e.lineLen(1)}
		vimPressRunes(t, e, "p")

		assertTutorContent(t, e, strings.Join([]string{
			tutorExpectedLine(t, expects, 809),
			tutorExpectedLine(t, expects, 810),
		}, "\n"))
	})
}

func loadVimBeginnerTutor(t *testing.T) ([]string, vimTutorExpectations) {
	t.Helper()
	tutorPath := filepath.Join("testdata", "vim-01-beginner.tutor")
	rawTutor, err := os.ReadFile(tutorPath)
	if err != nil {
		t.Fatalf("read %s: %v", tutorPath, err)
	}
	expectPath := filepath.Join("testdata", "vim-01-beginner.tutor.json")
	rawExpect, err := os.ReadFile(expectPath)
	if err != nil {
		t.Fatalf("read %s: %v", expectPath, err)
	}
	var expects vimTutorExpectations
	if err := json.Unmarshal(rawExpect, &expects); err != nil {
		t.Fatalf("parse %s: %v", expectPath, err)
	}
	if len(expects.Expect) == 0 {
		t.Fatalf("%s has no expectations", expectPath)
	}
	text := strings.ReplaceAll(string(rawTutor), "\r\n", "\n")
	return strings.Split(strings.TrimRight(text, "\n"), "\n"), expects
}

func tutorLine(t *testing.T, lines []string, lineNumber int) string {
	t.Helper()
	index := lineNumber - 1
	if index < 0 || index >= len(lines) {
		t.Fatalf("tutor line %d out of range", lineNumber)
	}
	return lines[index]
}

func tutorExpectedLine(t *testing.T, expects vimTutorExpectations, lineNumber int) string {
	t.Helper()
	raw, ok := expects.Expect[strconv.Itoa(lineNumber)]
	if !ok {
		t.Fatalf("missing tutor expectation for line %d", lineNumber)
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("line %d expectation is not a string: %v", lineNumber, err)
	}
	return got
}

func newVimTutorEditor(lines ...string) *Editor {
	e := newTestEditor(lines...)
	e.SetBehaviorProfile(BehaviorProfileVim)
	e.mode = ModeNormal
	return e
}

func vimPressRunes(t *testing.T, e *Editor, keys string) {
	t.Helper()
	for _, r := range keys {
		e.HandleKey(wrapKey(tcellEventRune(r)))
	}
}

func vimPressKey(t *testing.T, e *Editor, key string) {
	t.Helper()
	e.HandleKey(eventForKeyString(t, key))
}

func tcellEventRune(r rune) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, r, 0)
}

func vimPlaceOn(t *testing.T, e *Editor, row int, needle string, offset int) {
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

func vimDeleteCharAt(t *testing.T, e *Editor, needle string, offset int) {
	t.Helper()
	vimPlaceOn(t, e, 0, needle, offset)
	vimPressRunes(t, e, "x")
}

func vimInsertBefore(t *testing.T, e *Editor, needle string, offset int, text string) {
	t.Helper()
	vimPlaceOn(t, e, 0, needle, offset)
	vimPressRunes(t, e, "i")
	vimPressRunes(t, e, text)
	vimPressKey(t, e, "esc")
}

func vimDeleteWordAt(t *testing.T, e *Editor, needle string) {
	t.Helper()
	vimPlaceOn(t, e, 0, needle, 0)
	vimPressRunes(t, e, "dw")
}

func vimReplaceCharAt(t *testing.T, e *Editor, needle string, offset int, replacement rune) {
	t.Helper()
	vimPlaceOn(t, e, 0, needle, offset)
	vimPressRunes(t, e, "r")
	vimPressRunes(t, e, string(replacement))
}

func vimChangeTo(t *testing.T, e *Editor, needle string, offset int, replacement string) {
	t.Helper()
	vimPlaceOn(t, e, 0, needle, offset)
	vimPressRunes(t, e, "ce")
	vimPressRunes(t, e, replacement)
	vimPressKey(t, e, "esc")
}

func vimAppendAfter(t *testing.T, e *Editor, needle string, offset int, text string) {
	t.Helper()
	vimPlaceOn(t, e, 0, needle, offset)
	vimPressRunes(t, e, "a")
	vimPressRunes(t, e, text)
	vimPressKey(t, e, "esc")
}

func vimReplaceTextAt(t *testing.T, e *Editor, needle string, offset int, replacement string) {
	t.Helper()
	vimPlaceOn(t, e, 0, needle, offset)
	vimPressRunes(t, e, "R")
	vimPressRunes(t, e, replacement)
	vimPressKey(t, e, "esc")
}

func assertTutorContent(t *testing.T, e *Editor, want string) {
	t.Helper()
	if got := e.Content(); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	if e.BehaviorProfile() != BehaviorProfileVim {
		t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileVim)
	}
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want %v", e.mode, ModeNormal)
	}
}
