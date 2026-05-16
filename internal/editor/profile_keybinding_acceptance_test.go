package editor

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestVimProfileFullKeySimulationMatrix(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string
		cursor      Cursor
		keys        string
		wantContent string
		wantCursor  *Cursor
		wantMode    Mode
	}{
		{
			name:        "insert before cursor",
			lines:       []string{"one"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "iX<esc>",
			wantContent: "oXne",
			wantMode:    ModeNormal,
		},
		{
			name:        "append after cursor",
			lines:       []string{"one"},
			cursor:      Cursor{Row: 0, Col: 0},
			keys:        "aX<esc>",
			wantContent: "oXne",
			wantMode:    ModeNormal,
		},
		{
			name:        "append at line end",
			lines:       []string{"one"},
			keys:        "A!<esc>",
			wantContent: "one!",
			wantMode:    ModeNormal,
		},
		{
			name:        "insert at first nonblank",
			lines:       []string{"  one"},
			cursor:      Cursor{Row: 0, Col: 5},
			keys:        "I*<esc>",
			wantContent: "  *one",
			wantMode:    ModeNormal,
		},
		{
			name:        "open below",
			lines:       []string{"one"},
			keys:        "oTWO<esc>",
			wantContent: "one\nTWO",
			wantMode:    ModeNormal,
		},
		{
			name:        "open above",
			lines:       []string{"one", "three"},
			cursor:      Cursor{Row: 1, Col: 0},
			keys:        "OTWO<esc>",
			wantContent: "one\nTWO\nthree",
			wantMode:    ModeNormal,
		},
		{
			name:        "single replace",
			lines:       []string{"abc"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "rZ",
			wantContent: "aZc",
			wantMode:    ModeNormal,
		},
		{
			name:        "find char forward",
			lines:       []string{"abcabc"},
			keys:        "fcx",
			wantContent: "ababc",
			wantMode:    ModeNormal,
		},
		{
			name:        "till char forward",
			lines:       []string{"abcabc"},
			keys:        "tcx",
			wantContent: "acabc",
			wantMode:    ModeNormal,
		},
		{
			name:        "find char backward",
			lines:       []string{"abcabc"},
			keys:        "$Fbx",
			wantContent: "abcac",
			wantMode:    ModeNormal,
		},
		{
			name:        "till char backward",
			lines:       []string{"abcabc"},
			keys:        "$Tbx",
			wantContent: "abcab",
			wantMode:    ModeNormal,
		},
		{
			name:        "replace mode overwrites existing runes",
			lines:       []string{"abcdef"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "RXY<esc>",
			wantContent: "aXYdef",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete and change to line end",
			lines:       []string{"abc def"},
			cursor:      Cursor{Row: 0, Col: 4},
			keys:        "D",
			wantContent: "abc ",
			wantMode:    ModeNormal,
		},
		{
			name:        "change to line end enters insert",
			lines:       []string{"abc def"},
			cursor:      Cursor{Row: 0, Col: 4},
			keys:        "Cxyz<esc>",
			wantContent: "abc xyz",
			wantMode:    ModeNormal,
		},
		{
			name:        "substitute count chars",
			lines:       []string{"abcdef"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "3sZ<esc>",
			wantContent: "aZef",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete char before cursor",
			lines:       []string{"abc"},
			cursor:      Cursor{Row: 0, Col: 2},
			keys:        "X",
			wantContent: "ac",
			wantMode:    ModeNormal,
		},
		{
			name:        "join lines",
			lines:       []string{"hello", "world"},
			keys:        "J",
			wantContent: "hello world",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete word motion",
			lines:       []string{"one two"},
			keys:        "dw",
			wantContent: "two",
			wantMode:    ModeNormal,
		},
		{
			name:        "normal WORD forward treats punctuation as part of word",
			lines:       []string{"one.two three"},
			keys:        "W",
			wantContent: "one.two three",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: len([]rune("one.two "))}),
			wantMode:    ModeNormal,
		},
		{
			name:        "normal WORD backward treats punctuation as part of word",
			lines:       []string{"one.two three"},
			cursor:      Cursor{Row: 0, Col: len([]rune("one.two three"))},
			keys:        "B",
			wantContent: "one.two three",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: len([]rune("one.two "))}),
			wantMode:    ModeNormal,
		},
		{
			name:        "normal WORD end treats punctuation as part of word",
			lines:       []string{"one.two three"},
			keys:        "E",
			wantContent: "one.two three",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: len([]rune("one.two")) - 1}),
			wantMode:    ModeNormal,
		},
		{
			name:        "normal ge moves to previous word end",
			lines:       []string{"one two"},
			cursor:      Cursor{Row: 0, Col: len([]rune("one "))},
			keys:        "ge",
			wantContent: "one two",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: len([]rune("one")) - 1}),
			wantMode:    ModeNormal,
		},
		{
			name:        "normal gE moves to previous WORD end",
			lines:       []string{"one.two three"},
			cursor:      Cursor{Row: 0, Col: len([]rune("one.two "))},
			keys:        "gE",
			wantContent: "one.two three",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: len([]rune("one.two")) - 1}),
			wantMode:    ModeNormal,
		},
		{
			name:        "mark line jump moves to first nonblank on marked row",
			lines:       []string{"one", "  two", "three"},
			cursor:      Cursor{Row: 1, Col: len([]rune("  t"))},
			keys:        "maG'a",
			wantContent: "one\n  two\nthree",
			wantCursor:  ptrCursor(Cursor{Row: 1, Col: 2}),
			wantMode:    ModeNormal,
		},
		{
			name:        "mark exact jump restores marked column",
			lines:       []string{"abcdef"},
			cursor:      Cursor{Row: 0, Col: 2},
			keys:        "ma$`a",
			wantContent: "abcdef",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 2}),
			wantMode:    ModeNormal,
		},
		{
			name:        "delete to line mark uses linewise range",
			lines:       []string{"one", "two", "three"},
			keys:        "majjd'a",
			wantContent: "",
			wantMode:    ModeNormal,
		},
		{
			name:        "sentence forward motion",
			lines:       []string{"One. Two. Three."},
			keys:        ")",
			wantContent: "One. Two. Three.",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: len([]rune("One. "))}),
			wantMode:    ModeNormal,
		},
		{
			name:        "sentence backward motion",
			lines:       []string{"One. Two. Three."},
			cursor:      Cursor{Row: 0, Col: len([]rune("One. Two"))},
			keys:        "(",
			wantContent: "One. Two. Three.",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 0}),
			wantMode:    ModeNormal,
		},
		{
			name:        "delete sentence motion",
			lines:       []string{"One. Two."},
			keys:        "d)",
			wantContent: "Two.",
			wantMode:    ModeNormal,
		},
		{
			name:        "paragraph forward motion",
			lines:       []string{"one", "two", "", "three"},
			keys:        "}",
			wantContent: "one\ntwo\n\nthree",
			wantCursor:  ptrCursor(Cursor{Row: 2, Col: 0}),
			wantMode:    ModeNormal,
		},
		{
			name:        "paragraph backward motion",
			lines:       []string{"one", "two", "", "three"},
			cursor:      Cursor{Row: 3, Col: 0},
			keys:        "{",
			wantContent: "one\ntwo\n\nthree",
			wantCursor:  ptrCursor(Cursor{Row: 2, Col: 0}),
			wantMode:    ModeNormal,
		},
		{
			name:        "delete paragraph motion",
			lines:       []string{"one", "two", "", "three"},
			keys:        "d}",
			wantContent: "\nthree",
			wantMode:    ModeNormal,
		},
		{
			name:        "visual-line delete selected lines",
			lines:       []string{"one", "two", "three"},
			keys:        "Vjd",
			wantContent: "three",
			wantMode:    ModeNormal,
		},
		{
			name:        "visual-line yank and paste before",
			lines:       []string{"one", "two"},
			keys:        "VyjP",
			wantContent: "one\none\ntwo",
			wantMode:    ModeNormal,
		},
		{
			name:        "visual-line change selected lines",
			lines:       []string{"one", "two", "three"},
			keys:        "VjcX<esc>",
			wantContent: "X\nthree",
			wantMode:    ModeNormal,
		},
		{
			name:        "change through word end",
			lines:       []string{"bad word"},
			keys:        "cegood<esc>",
			wantContent: "good word",
			wantMode:    ModeNormal,
		},
		{
			name:        "change word uses vim cw semantics",
			lines:       []string{"bad word"},
			keys:        "cwgood<esc>",
			wantContent: "good word",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete to line end motion",
			lines:       []string{"one two"},
			cursor:      Cursor{Row: 0, Col: 4},
			keys:        "d$",
			wantContent: "one ",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete to absolute line start",
			lines:       []string{"  alpha beta"},
			cursor:      Cursor{Row: 0, Col: len([]rune("  alpha"))},
			keys:        "d0",
			wantContent: " beta",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete to first nonblank",
			lines:       []string{"  alpha beta"},
			cursor:      Cursor{Row: 0, Col: len([]rune("  alpha"))},
			keys:        "d^",
			wantContent: "   beta",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete counted motion waits and applies",
			lines:       numberedLines(12),
			keys:        "d10j",
			wantContent: "12",
			wantMode:    ModeNormal,
		},
		{
			name:        "operator count and motion count multiply",
			lines:       numberedLines(10),
			keys:        "2d3d",
			wantContent: "7\n8\n9\n10",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete to first line with gg motion",
			lines:       numberedLines(5),
			cursor:      Cursor{Row: 2, Col: 0},
			keys:        "dgg",
			wantContent: "4\n5",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete to last line with G motion",
			lines:       numberedLines(5),
			cursor:      Cursor{Row: 1, Col: 0},
			keys:        "dG",
			wantContent: "1",
			wantMode:    ModeNormal,
		},
		{
			name:        "change whole line keeps editable row",
			lines:       []string{"one", "two", "three"},
			cursor:      Cursor{Row: 1, Col: 1},
			keys:        "ccchanged<esc>",
			wantContent: "one\nchanged\nthree",
			wantMode:    ModeNormal,
		},
		{
			name:        "linewise yank and paste",
			lines:       []string{"one", "two"},
			keys:        "yyp",
			wantContent: "one\none\ntwo",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete line updates unnamed register",
			lines:       []string{"one", "two"},
			keys:        "ddp",
			wantContent: "two\none",
			wantMode:    ModeNormal,
		},
		{
			name:        "named register survives later unnamed delete",
			lines:       []string{"one", "two"},
			keys:        `"ayyjdd"ap`,
			wantContent: "one\none",
			wantMode:    ModeNormal,
		},
		{
			name:        "blackhole register preserves unnamed paste",
			lines:       []string{"one", "two"},
			keys:        `yyj"_ddp`,
			wantContent: "one\none",
			wantMode:    ModeNormal,
		},
		{
			name:        "Y yanks current line",
			lines:       []string{"one", "two"},
			keys:        "Yp",
			wantContent: "one\none\ntwo",
			wantMode:    ModeNormal,
		},
		{
			name:        "indent and unindent line",
			lines:       []string{"one"},
			keys:        ">><lt><lt>",
			wantContent: "one",
			wantMode:    ModeNormal,
		},
		{
			name:        "toggle case",
			lines:       []string{"aBc"},
			keys:        "3~",
			wantContent: "AbC",
			wantMode:    ModeNormal,
		},
		{
			name:        "lowercase operator over word motion",
			lines:       []string{"ABC DEF"},
			keys:        "guw",
			wantContent: "abc DEF",
			wantMode:    ModeNormal,
		},
		{
			name:        "uppercase operator through word end",
			lines:       []string{"abc def"},
			keys:        "gUe",
			wantContent: "ABC def",
			wantMode:    ModeNormal,
		},
		{
			name:        "toggle case operator over current line",
			lines:       []string{"aBc", "dEf"},
			keys:        "g~~",
			wantContent: "AbC\ndEf",
			wantMode:    ModeNormal,
		},
		{
			name:        "uppercase inner word text object",
			lines:       []string{"alpha beta gamma"},
			cursor:      Cursor{Row: 0, Col: len([]rune("alpha b"))},
			keys:        "gUiw",
			wantContent: "alpha BETA gamma",
			wantMode:    ModeNormal,
		},
		{
			name:        "match bracket",
			lines:       []string{"a(b)c"},
			keys:        "l%",
			wantContent: "a(b)c",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 3}),
			wantMode:    ModeNormal,
		},
		{
			name:        "undo and redo through real ctrl-r",
			lines:       []string{"abc"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "xu<ctrl+r>",
			wantContent: "ac",
			wantMode:    ModeNormal,
		},
		{
			name:        "increment number with ctrl-a",
			lines:       []string{"version 2"},
			keys:        "<ctrl+a>",
			wantContent: "version 3",
			wantMode:    ModeNormal,
		},
		{
			name:        "decrement number with ctrl-x",
			lines:       []string{"version 2"},
			keys:        "<ctrl+x>",
			wantContent: "version 1",
			wantMode:    ModeNormal,
		},
		{
			name:        "dot repeats single normal change",
			lines:       []string{"abcdef"},
			keys:        "x.",
			wantContent: "cdef",
			wantMode:    ModeNormal,
		},
		{
			name:        "dot repeats insert change",
			lines:       []string{"one", "two"},
			keys:        "iX<esc>j0.",
			wantContent: "Xone\nXtwo",
			wantMode:    ModeNormal,
		},
		{
			name:        "dot repeats operator change with inserted text",
			lines:       []string{"bad one", "bad two"},
			keys:        "cwgood<esc>j0.",
			wantContent: "good one\ngood two",
			wantMode:    ModeNormal,
		},
		{
			name:        "macro records and replays insert workflow",
			lines:       []string{"one", "two"},
			keys:        "qa0iX<esc>jq@a",
			wantContent: "Xone\nXtwo",
			wantMode:    ModeNormal,
		},
		{
			name:        "repeat last macro with @@",
			lines:       []string{"one", "two", "three"},
			keys:        "qa0iX<esc>jq@a@@",
			wantContent: "Xone\nXtwo\nXthree",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete inner word text object",
			lines:       []string{"alpha beta gamma"},
			cursor:      Cursor{Row: 0, Col: len([]rune("alpha b"))},
			keys:        "diw",
			wantContent: "alpha  gamma",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete around word text object",
			lines:       []string{"alpha beta gamma"},
			cursor:      Cursor{Row: 0, Col: len([]rune("alpha b"))},
			keys:        "daw",
			wantContent: "alpha gamma",
			wantMode:    ModeNormal,
		},
		{
			name:        "change inside quotes text object",
			lines:       []string{`say "old value" now`},
			cursor:      Cursor{Row: 0, Col: len([]rune(`say "old`))},
			keys:        `ci"new<esc>`,
			wantContent: `say "new" now`,
			wantMode:    ModeNormal,
		},
		{
			name:        "delete around parentheses text object",
			lines:       []string{"call(one, two) now"},
			cursor:      Cursor{Row: 0, Col: len([]rune("call(one"))},
			keys:        "da)",
			wantContent: "call now",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete inner paragraph text object",
			lines:       []string{"one", "two", "", "three"},
			keys:        "dip",
			wantContent: "\n\nthree",
			wantMode:    ModeNormal,
		},
		{
			name:        "goto count with gg",
			lines:       numberedLines(5),
			keys:        "3gg",
			wantContent: "1\n2\n3\n4\n5",
			wantCursor:  ptrCursor(Cursor{Row: 2, Col: 0}),
			wantMode:    ModeNormal,
		},
		{
			name:        "jumplist moves back and forward",
			lines:       numberedLines(4),
			keys:        "G<ctrl+o><ctrl+i>",
			wantContent: "1\n2\n3\n4",
			wantCursor:  ptrCursor(Cursor{Row: 3, Col: 0}),
			wantMode:    ModeNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newSimulatedProfileEditor(BehaviorProfileVim, tt.lines...)
			e.cursor = tt.cursor

			pressKeyScript(t, e, tt.keys)

			assertSimulatedResult(t, e, tt.wantContent, tt.wantCursor, tt.wantMode)
			if e.BehaviorProfile() != BehaviorProfileVim {
				t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileVim)
			}
		})
	}
}

func TestHelixProfileFullKeySimulationMatrix(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string
		cursor      Cursor
		keys        string
		wantContent string
		wantCursor  *Cursor
		wantMode    Mode
	}{
		{
			name:        "insert before cursor",
			lines:       []string{"abc"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "iX<esc>",
			wantContent: "aXbc",
			wantMode:    ModeNormal,
		},
		{
			name:        "append after cursor",
			lines:       []string{"abc"},
			keys:        "aX<esc>",
			wantContent: "aXbc",
			wantMode:    ModeNormal,
		},
		{
			name:        "append at line end",
			lines:       []string{"abc"},
			keys:        "A!<esc>",
			wantContent: "abc!",
			wantMode:    ModeNormal,
		},
		{
			name:        "insert at first nonblank",
			lines:       []string{"  abc"},
			cursor:      Cursor{Row: 0, Col: 5},
			keys:        "I*<esc>",
			wantContent: "  *abc",
			wantMode:    ModeNormal,
		},
		{
			name:        "open below",
			lines:       []string{"one"},
			keys:        "oTWO<esc>",
			wantContent: "one\nTWO",
			wantMode:    ModeNormal,
		},
		{
			name:        "open above",
			lines:       []string{"one", "three"},
			cursor:      Cursor{Row: 1, Col: 0},
			keys:        "OTWO<esc>",
			wantContent: "one\nTWO\nthree",
			wantMode:    ModeNormal,
		},
		{
			name:        "delete current char",
			lines:       []string{"abc"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "d",
			wantContent: "ac",
			wantMode:    ModeNormal,
		},
		{
			name:        "replace current char",
			lines:       []string{"abc"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "rZ",
			wantContent: "aZc",
			wantMode:    ModeNormal,
		},
		{
			name:        "find char forward selection delete",
			lines:       []string{"abcabc"},
			keys:        "fcd",
			wantContent: "abc",
			wantMode:    ModeNormal,
		},
		{
			name:        "till char forward selection delete",
			lines:       []string{"abcabc"},
			keys:        "tcd",
			wantContent: "cabc",
			wantMode:    ModeNormal,
		},
		{
			name:        "find char backward selection delete",
			lines:       []string{"abcabc"},
			keys:        "glFbd",
			wantContent: "abca",
			wantMode:    ModeNormal,
		},
		{
			name:        "till char backward selection delete",
			lines:       []string{"abcabc"},
			keys:        "glTbd",
			wantContent: "abcab",
			wantMode:    ModeNormal,
		},
		{
			name:        "join lines",
			lines:       []string{"hello", "world"},
			keys:        "J",
			wantContent: "hello world",
			wantMode:    ModeNormal,
		},
		{
			name:        "word selection delete",
			lines:       []string{"one two"},
			keys:        "wd",
			wantContent: "two",
			wantMode:    ModeNormal,
		},
		{
			name:        "collapse selection",
			lines:       []string{"one two"},
			keys:        "w;",
			wantContent: "one two",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 4}),
			wantMode:    ModeNormal,
		},
		{
			name:        "backward word selection delete",
			lines:       []string{"one two"},
			keys:        "glbd",
			wantContent: "one ",
			wantMode:    ModeNormal,
		},
		{
			name:        "word-end selection change",
			lines:       []string{"bad word"},
			keys:        "ecgood<esc>",
			wantContent: "good word",
			wantMode:    ModeNormal,
		},
		{
			name:        "counted word selection delete",
			lines:       []string{"one two three"},
			keys:        "2wd",
			wantContent: "three",
			wantMode:    ModeNormal,
		},
		{
			name:        "WORD selection delete treats punctuation as part of word",
			lines:       []string{"one.two three"},
			keys:        "Wd",
			wantContent: "three",
			wantMode:    ModeNormal,
		},
		{
			name:        "backward WORD selection delete",
			lines:       []string{"one.two three"},
			keys:        "glBd",
			wantContent: "one.two ",
			wantMode:    ModeNormal,
		},
		{
			name:        "WORD-end selection change",
			lines:       []string{"bad.word next"},
			keys:        "Ecgood<esc>",
			wantContent: "good next",
			wantMode:    ModeNormal,
		},
		{
			name:        "toggle selected text case",
			lines:       []string{"aBc"},
			keys:        "x~",
			wantContent: "AbC",
			wantMode:    ModeNormal,
		},
		{
			name:        "lowercase selected text",
			lines:       []string{"AbC"},
			keys:        "x`",
			wantContent: "abc",
			wantMode:    ModeNormal,
		},
		{
			name:        "uppercase selected text",
			lines:       []string{"aBc"},
			keys:        "x<alt+`>",
			wantContent: "ABC",
			wantMode:    ModeNormal,
		},
		{
			name:        "select regex creates multiple selections and delete applies to all",
			lines:       []string{"I like apples and apples"},
			keys:        "%sapples<enter>d",
			wantContent: "I like  and ",
			wantMode:    ModeNormal,
		},
		{
			name:        "select regex replace char applies to all selections",
			lines:       []string{"bob bib"},
			keys:        "%sb<enter>rB",
			wantContent: "BoB BiB",
			wantMode:    ModeNormal,
		},
		{
			name:        "select regex change replays inserted text across selections",
			lines:       []string{"I like apples and apples"},
			keys:        "%sapples<enter>coranges<esc>",
			wantContent: "I like oranges and oranges",
			wantMode:    ModeNormal,
		},
		{
			name:        "duplicate cursor below inserts at both cursors",
			lines:       []string{"aa", "aa"},
			keys:        "CiX<esc>",
			wantContent: "Xaa\nXaa",
			wantMode:    ModeNormal,
		},
		{
			name:        "duplicate cursor above inserts at both cursors",
			lines:       []string{"aa", "aa"},
			cursor:      Cursor{Row: 1, Col: 0},
			keys:        "<alt+c>iX<esc>",
			wantContent: "Xaa\nXaa",
			wantMode:    ModeNormal,
		},
		{
			name:        "split selections on lines and delete all pieces",
			lines:       []string{"one", "two", "three"},
			keys:        "2x<alt+s>d",
			wantContent: "three",
			wantMode:    ModeNormal,
		},
		{
			name:        "empty split regex prompt splits selected buffer by lines",
			lines:       []string{"one", "two", "three"},
			keys:        "%S<enter>~",
			wantContent: "ONE\nTWO\nTHREE",
			wantMode:    ModeNormal,
		},
		{
			name:        "split selection by regex and transform pieces",
			lines:       []string{"one,two,three"},
			keys:        "%S,<enter>~",
			wantContent: "ONE,TWO,THREE",
			wantMode:    ModeNormal,
		},
		{
			name:        "split selections collapse to multiple insert cursors",
			lines:       []string{"one. two. three"},
			keys:        "%S\\. <enter><alt+;>;iX<esc>",
			wantContent: "Xone. Xtwo. Xthree",
			wantMode:    ModeNormal,
		},
		{
			name:        "regex selections append at every selected range",
			lines:       []string{"foo foo"},
			keys:        "%sfoo<enter>aX<esc>",
			wantContent: "fooX fooX",
			wantMode:    ModeNormal,
		},
		{
			name:        "align regex selections",
			lines:       []string{"a = 1", "long = 2"},
			keys:        "%s=<enter>&",
			wantContent: "a    = 1\nlong = 2",
			wantMode:    ModeNormal,
		},
		{
			name:        "remove primary selection keeps remaining selections",
			lines:       []string{"one two one"},
			keys:        "%sone<enter><alt+,>d",
			wantContent: "one two ",
			wantMode:    ModeNormal,
		},
		{
			name:        "keep primary selection removes secondary selections",
			lines:       []string{"one two one"},
			keys:        "%sone<enter>,d",
			wantContent: " two one",
			wantMode:    ModeNormal,
		},
		{
			name:        "cycle selection contents forward",
			lines:       []string{"one two three"},
			keys:        "%sone|two|three<enter><alt+)>",
			wantContent: "three one two",
			wantMode:    ModeNormal,
		},
		{
			name:        "cycle selection contents backward",
			lines:       []string{"one two three"},
			keys:        "%sone|two|three<enter><alt+(>",
			wantContent: "two three one",
			wantMode:    ModeNormal,
		},
		{
			name:        "star search register and n add selection in select mode",
			lines:       []string{"bat cat bat"},
			keys:        "e*vnrB",
			wantContent: "BBB cat BBB",
			wantMode:    ModeNormal,
		},
		{
			name:        "match mode select inside pair then delete",
			lines:       []string{"a(b)c"},
			cursor:      Cursor{Row: 0, Col: 2},
			keys:        "mi(d",
			wantContent: "a()c",
			wantMode:    ModeNormal,
		},
		{
			name:        "match mode surround selection",
			lines:       []string{"abc"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "vlms(",
			wantContent: "a(b)c",
			wantMode:    ModeNormal,
		},
		{
			name:        "match mode delete surround",
			lines:       []string{"a(b)c"},
			cursor:      Cursor{Row: 0, Col: 2},
			keys:        "md(",
			wantContent: "abc",
			wantMode:    ModeNormal,
		},
		{
			name:        "match mode replace surround",
			lines:       []string{"a(b)c"},
			cursor:      Cursor{Row: 0, Col: 2},
			keys:        "mr([",
			wantContent: "a[b]c",
			wantMode:    ModeNormal,
		},
		{
			name:        "counted line selections delete whole lines",
			lines:       []string{"one", "two", "three"},
			keys:        "2xd",
			wantContent: "three",
			wantMode:    ModeNormal,
		},
		{
			name:        "linewise yank and paste",
			lines:       []string{"one", "two"},
			keys:        "xyp",
			wantContent: "one\none\ntwo",
			wantMode:    ModeNormal,
		},
		{
			name:        "linewise paste before",
			lines:       []string{"one", "two"},
			keys:        "xyjP",
			wantContent: "one\none\ntwo",
			wantMode:    ModeNormal,
		},
		{
			name:        "replace selection with yanked text",
			lines:       []string{"one", "two"},
			keys:        "xyjxR",
			wantContent: "one\none",
			wantMode:    ModeNormal,
		},
		{
			name:        "indent and unindent selected line",
			lines:       []string{"one"},
			keys:        "x><lt>",
			wantContent: "one",
			wantMode:    ModeNormal,
		},
		{
			name:        "goto line start",
			lines:       []string{"abc"},
			keys:        "glgh",
			wantContent: "abc",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 0}),
			wantMode:    ModeNormal,
		},
		{
			name:        "select all and delete",
			lines:       []string{"one", "two"},
			keys:        "%d",
			wantContent: "",
			wantMode:    ModeNormal,
		},
		{
			name:        "undo and redo through U",
			lines:       []string{"abc"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "duU",
			wantContent: "ac",
			wantMode:    ModeNormal,
		},
		{
			name:        "increment number with ctrl-a",
			lines:       []string{"version 2"},
			keys:        "<ctrl+a>",
			wantContent: "version 3",
			wantMode:    ModeNormal,
		},
		{
			name:        "decrement number with ctrl-x",
			lines:       []string{"version 2"},
			keys:        "<ctrl+x>",
			wantContent: "version 1",
			wantMode:    ModeNormal,
		},
		{
			name:        "ctrl-c toggles current line comment",
			lines:       []string{"line"},
			keys:        "<ctrl+c><ctrl+c>",
			wantContent: "line",
			wantMode:    ModeNormal,
		},
		{
			name:        "ctrl-c comments selected lines",
			lines:       []string{"one", "two", "three"},
			keys:        "2x<ctrl+c>",
			wantContent: "// one\n// two\nthree",
			wantMode:    ModeNormal,
		},
		{
			name:        "goto first line",
			lines:       numberedLines(4),
			cursor:      Cursor{Row: 2, Col: 1},
			keys:        "gg",
			wantContent: "1\n2\n3\n4",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 0}),
			wantMode:    ModeNormal,
		},
		{
			name:        "goto file end",
			lines:       []string{"one", "two"},
			keys:        "ge",
			wantContent: "one\ntwo",
			wantCursor:  ptrCursor(Cursor{Row: 1, Col: 3}),
			wantMode:    ModeNormal,
		},
		{
			name:        "goto line end",
			lines:       []string{"abc"},
			keys:        "gl",
			wantContent: "abc",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 3}),
			wantMode:    ModeNormal,
		},
		{
			name:        "goto last line",
			lines:       numberedLines(4),
			keys:        "G",
			wantContent: "1\n2\n3\n4",
			wantCursor:  ptrCursor(Cursor{Row: 3, Col: 0}),
			wantMode:    ModeNormal,
		},
		{
			name:        "jumplist returns after goto and goes forward",
			lines:       numberedLines(4),
			keys:        "G<ctrl+o><ctrl+i>",
			wantContent: "1\n2\n3\n4",
			wantCursor:  ptrCursor(Cursor{Row: 3, Col: 0}),
			wantMode:    ModeNormal,
		},
		{
			name:        "manual jumplist save returns with ctrl-o",
			lines:       numberedLines(4),
			cursor:      Cursor{Row: 1, Col: 0},
			keys:        "<ctrl+s>G<ctrl+o>",
			wantContent: "1\n2\n3\n4",
			wantCursor:  ptrCursor(Cursor{Row: 1, Col: 0}),
			wantMode:    ModeNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newSimulatedProfileEditor(BehaviorProfileHelix, tt.lines...)
			e.cursor = tt.cursor

			pressKeyScript(t, e, tt.keys)

			assertSimulatedResult(t, e, tt.wantContent, tt.wantCursor, tt.wantMode)
			if e.BehaviorProfile() != BehaviorProfileHelix {
				t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileHelix)
			}
		})
	}
}

func TestBasicProfileFullKeySimulationMatrix(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string
		cursor      Cursor
		keys        string
		wantContent string
		wantCursor  *Cursor
	}{
		{
			name:        "plain typing without modal transition",
			lines:       []string{""},
			keys:        "hello",
			wantContent: "hello",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 5}),
		},
		{
			name:        "insert-mode navigation and typing",
			lines:       []string{""},
			keys:        "abc<left><left>X<right>Y",
			wantContent: "aXbYc",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 4}),
		},
		{
			name:        "newline and backspace",
			lines:       []string{""},
			keys:        "abc<enter>de<backspace>",
			wantContent: "abc\nd",
			wantCursor:  ptrCursor(Cursor{Row: 1, Col: 1}),
		},
		{
			name:        "delete key removes char at cursor",
			lines:       []string{"abc"},
			cursor:      Cursor{Row: 0, Col: 1},
			keys:        "<del>",
			wantContent: "ac",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 1}),
		},
		{
			name:        "tab and shift tab indent current line",
			lines:       []string{"abc"},
			keys:        "<tab><shift+tab>",
			wantContent: "abc",
			wantCursor:  ptrCursor(Cursor{Row: 0, Col: 0}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newSimulatedProfileEditor(BehaviorProfileBasic, tt.lines...)
			e.cursor = tt.cursor

			pressKeyScript(t, e, tt.keys)

			assertSimulatedResult(t, e, tt.wantContent, tt.wantCursor, ModeInsert)
			if e.BehaviorProfile() != BehaviorProfileBasic {
				t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileBasic)
			}
		})
	}
}

func TestCommandLineCommandsThroughFullKeySimulation(t *testing.T) {
	t.Run("profile switch command", func(t *testing.T) {
		e := newSimulatedProfileEditor(BehaviorProfileBasic, "")

		pressKeyScript(t, e, "<alt+x>profile vim<enter>iX<esc>")

		if e.BehaviorProfile() != BehaviorProfileVim {
			t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileVim)
		}
		assertSimulatedResult(t, e, "X", nil, ModeNormal)
	})

	t.Run("substitute command", func(t *testing.T) {
		e := newSimulatedProfileEditor(BehaviorProfileVim, "foo foo")

		pressKeyScript(t, e, ":s/foo/bar/g<enter>")

		assertSimulatedResult(t, e, "bar bar", nil, ModeNormal)
	})

	t.Run("tutor command opens editable scratch tutorial", func(t *testing.T) {
		e := newSimulatedProfileEditor(BehaviorProfileVim, "draft")

		pressKeyScript(t, e, ":tutor vim<enter>")

		if e.document.filename != "" {
			t.Fatalf("filename = %q, want scratch buffer without file path", e.document.filename)
		}
		if !strings.Contains(e.Content(), "Welcome   to   the   VIM   Tutor") {
			t.Fatalf("tutorial content does not contain Vim tutor title")
		}
		if e.mode != ModeNormal {
			t.Fatalf("mode = %v, want normal", e.mode)
		}
	})
}

func TestHelixSplitRegexEscapedNewlineClearsTransientRegexError(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one", "two", "three")

	pressKeyScript(t, e, `%S\`)
	if !strings.HasPrefix(e.ui.statusMessage, "regex error:") {
		t.Fatalf("status after dangling backslash = %q, want transient regex error", e.ui.statusMessage)
	}
	pressKeyScript(t, e, `n`)
	if strings.HasPrefix(e.ui.statusMessage, "regex error:") {
		t.Fatalf("status after escaped newline = %q, want regex error cleared", e.ui.statusMessage)
	}
	pressKeyScript(t, e, "<enter>~")

	assertSimulatedResult(t, e, "ONE\nTWO\nTHREE", nil, ModeNormal)
}

func TestHelixCollapseSplitSelectionsKeepsMultipleCursors(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one", "two", "three")

	pressKeyScript(t, e, `%S\n<enter>;`)

	if e.selectionActive || len(e.selectionRanges) != 0 || e.modal.selectMode {
		t.Fatalf("selection active=%v ranges=%d selectMode=%v, want collapsed selections", e.selectionActive, len(e.selectionRanges), e.modal.selectMode)
	}
	want := []Cursor{
		{Row: 0, Col: 3},
		{Row: 1, Col: 3},
		{Row: 2, Col: 5},
	}
	if len(e.profile.helix.multiCursors) != len(want) {
		t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
	}
	for i := range want {
		if e.profile.helix.multiCursors[i] != want[i] {
			t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
		}
	}
}

func TestHelixCollapsedMultipleCursorsMoveTogether(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one", "two", "three")

	pressKeyScript(t, e, `%S\n<enter>;h`)

	want := []Cursor{
		{Row: 0, Col: 2},
		{Row: 1, Col: 2},
		{Row: 2, Col: 4},
	}
	if len(e.profile.helix.multiCursors) != len(want) {
		t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
	}
	for i := range want {
		if e.profile.helix.multiCursors[i] != want[i] {
			t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
		}
	}
}

func TestHelixMultipleSelectionHeadsMoveTogetherInNormalMode(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one", "two", "three")

	pressKeyScript(t, e, `%S\n<enter>h`)

	ranges := e.rawActiveSelectionRanges()
	want := []editorSelectionRange{
		{Start: Cursor{Row: 0, Col: 0}, End: Cursor{Row: 0, Col: 2}},
		{Start: Cursor{Row: 1, Col: 0}, End: Cursor{Row: 1, Col: 2}},
		{Start: Cursor{Row: 2, Col: 0}, End: Cursor{Row: 2, Col: 4}},
	}
	if len(ranges) != len(want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
	for i := range want {
		if ranges[i] != want[i] {
			t.Fatalf("ranges = %#v, want %#v", ranges, want)
		}
	}
	if e.cursor != want[0].End {
		t.Fatalf("cursor = %#v, want primary selection head %#v", e.cursor, want[0].End)
	}
}

func TestHelixMultipleSelectionHeadsArrowMoveTogetherInNormalMode(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one", "two", "three")

	pressKeyScript(t, e, `%S\n<enter><left>`)

	ranges := e.rawActiveSelectionRanges()
	want := []editorSelectionRange{
		{Start: Cursor{Row: 0, Col: 0}, End: Cursor{Row: 0, Col: 2}},
		{Start: Cursor{Row: 1, Col: 0}, End: Cursor{Row: 1, Col: 2}},
		{Start: Cursor{Row: 2, Col: 0}, End: Cursor{Row: 2, Col: 4}},
	}
	if len(ranges) != len(want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
	for i := range want {
		if ranges[i] != want[i] {
			t.Fatalf("ranges = %#v, want %#v", ranges, want)
		}
	}
}

func TestHelixMultipleSelectionHeadsWordMoveTogetherInNormalMode(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one two", "red blue")

	pressKeyScript(t, e, `%S\n<enter>b`)

	ranges := e.rawActiveSelectionRanges()
	want := []editorSelectionRange{
		{Start: Cursor{Row: 0, Col: 0}, End: Cursor{Row: 0, Col: 4}},
		{Start: Cursor{Row: 1, Col: 0}, End: Cursor{Row: 1, Col: 4}},
	}
	if len(ranges) != len(want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
	for i := range want {
		if ranges[i] != want[i] {
			t.Fatalf("ranges = %#v, want %#v", ranges, want)
		}
	}
}

func TestHelixCollapsedMultipleCursorsMoveTogetherInInsertMode(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one", "two", "three")

	pressKeyScript(t, e, `%S\n<enter>;i<left>`)

	want := []Cursor{
		{Row: 0, Col: 2},
		{Row: 1, Col: 2},
		{Row: 2, Col: 4},
	}
	if len(e.profile.helix.multiCursors) != len(want) {
		t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
	}
	for i := range want {
		if e.profile.helix.multiCursors[i] != want[i] {
			t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
		}
	}
	if e.mode != ModeInsert {
		t.Fatalf("mode = %v, want insert", e.mode)
	}
}

func TestHelixCommaClearsSecondaryCursors(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one", "two", "three")

	pressKeyScript(t, e, `%S\n<enter>;,`)

	if len(e.profile.helix.multiCursors) != 0 {
		t.Fatalf("multiCursors = %#v, want none", e.profile.helix.multiCursors)
	}
	if e.selectionActive || len(e.selectionRanges) != 0 || e.modal.selectMode {
		t.Fatalf("selection active=%v ranges=%d selectMode=%v, want no multi-cursor selection state", e.selectionActive, len(e.selectionRanges), e.modal.selectMode)
	}
	if e.cursor != (Cursor{Row: 0, Col: 3}) {
		t.Fatalf("cursor = %#v, want primary cursor", e.cursor)
	}
}

func TestHelixCommaKeepsOnlyPrimarySelection(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one two one")

	pressKeyScript(t, e, `%sone<enter>,`)

	ranges := e.rawActiveSelectionRanges()
	want := []editorSelectionRange{
		{Start: Cursor{Row: 0, Col: 3}, End: Cursor{Row: 0, Col: 0}},
	}
	if len(ranges) != len(want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
	for i := range want {
		if ranges[i] != want[i] {
			t.Fatalf("ranges = %#v, want %#v", ranges, want)
		}
	}
}

func TestHelixMultipleCursorsSurviveInsertEscapeAndMoveAgain(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one", "two")

	pressKeyScript(t, e, `%S\n<enter>;i!<esc>h`)

	assertSimulatedResult(t, e, "one!\ntwo!", ptrCursor(Cursor{Row: 0, Col: 3}), ModeNormal)
	want := []Cursor{
		{Row: 0, Col: 3},
		{Row: 1, Col: 3},
	}
	if len(e.profile.helix.multiCursors) != len(want) {
		t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
	}
	for i := range want {
		if e.profile.helix.multiCursors[i] != want[i] {
			t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
		}
	}
}

func TestHelixMultipleCursorsBackspaceOnSameLine(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "abcd")

	pressKeyScript(t, e, `%s[bd]<enter>;iX<backspace>`)

	assertSimulatedResult(t, e, "abcd", ptrCursor(Cursor{Row: 0, Col: 1}), ModeInsert)
	want := []Cursor{
		{Row: 0, Col: 1},
		{Row: 0, Col: 3},
	}
	if len(e.profile.helix.multiCursors) != len(want) {
		t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
	}
	for i := range want {
		if e.profile.helix.multiCursors[i] != want[i] {
			t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
		}
	}
}

func TestHelixMultipleCursorsDeleteInNormalMode(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "abcd")

	pressKeyScript(t, e, `%s[bd]<enter>;d`)

	assertSimulatedResult(t, e, "ac", ptrCursor(Cursor{Row: 0, Col: 1}), ModeNormal)
	want := []Cursor{
		{Row: 0, Col: 1},
		{Row: 0, Col: 2},
	}
	if len(e.profile.helix.multiCursors) != len(want) {
		t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
	}
	for i := range want {
		if e.profile.helix.multiCursors[i] != want[i] {
			t.Fatalf("multiCursors = %#v, want %#v", e.profile.helix.multiCursors, want)
		}
	}
}

func TestHelixSelectRegexPromptLivePreviewsSelections(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "foo bar foo")

	pressKeyScript(t, e, `%sfoo`)

	if e.mode != ModeSearch {
		t.Fatalf("mode = %v, want search prompt", e.mode)
	}
	ranges := e.activeSelectionRanges()
	want := []editorSelectionRange{
		{Start: Cursor{Row: 0, Col: 0}, End: Cursor{Row: 0, Col: 3}},
		{Start: Cursor{Row: 0, Col: 8}, End: Cursor{Row: 0, Col: 11}},
	}
	if len(ranges) != len(want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
	for i := range want {
		if ranges[i] != want[i] {
			t.Fatalf("ranges = %#v, want %#v", ranges, want)
		}
	}
}

func TestHelixSplitRegexPromptLivePreviewsSelections(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "one", "two", "three")

	pressKeyScript(t, e, `%S\n`)

	if e.mode != ModeSearch {
		t.Fatalf("mode = %v, want search prompt", e.mode)
	}
	ranges := e.activeSelectionRanges()
	want := []editorSelectionRange{
		{Start: Cursor{Row: 0, Col: 0}, End: Cursor{Row: 0, Col: 3}},
		{Start: Cursor{Row: 1, Col: 0}, End: Cursor{Row: 1, Col: 3}},
		{Start: Cursor{Row: 2, Col: 0}, End: Cursor{Row: 2, Col: 5}},
	}
	if len(ranges) != len(want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
	for i := range want {
		if ranges[i] != want[i] {
			t.Fatalf("ranges = %#v, want %#v", ranges, want)
		}
	}
}

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
