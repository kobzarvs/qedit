package editor

import (
	"regexp"
	"strings"
	"testing"
)

func TestVimTutorStrictUserKeyScript(t *testing.T) {
	_, expects := loadVimBeginnerTutor(t)
	e := newSimulatedProfileEditor(BehaviorProfileVim, "scratch")

	pressKeyScript(t, e, ":tutor vim<enter>")

	tutorStrictEditAt(t, e, "The ccow jumpedd ovverr thhe mooon.", 4, "x")
	tutorStrictAssertLine(t, e, "The cow jumpedd ovverr thhe mooon.")
	tutorStrictEditAt(t, e, "jumpedd", 6, "x")
	tutorStrictEditAt(t, e, "ovverr", 2, "x")
	tutorStrictEditAt(t, e, "overr", 4, "x")
	tutorStrictEditAt(t, e, "thhe", 2, "x")
	tutorStrictEditAt(t, e, "mooon", 2, "x")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 105))

	tutorStrictEditLineAt(t, e, "There is text misng this .", len([]rune("There is ")), "isome <esc>")
	tutorStrictEditLineAt(t, e, "There is some text misng this .", len([]rune("There is some text mis")), "isi<esc>")
	tutorStrictEditLineAt(t, e, "There is some text missing this .", len([]rune("There is some text missing ")), "ifrom <esc>")
	tutorStrictEditLineAt(t, e, "There is some text missing from this .", len([]rune("There is some text missing from this ")), "iline<esc>")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 126))

	tutorStrictEditLineAt(t, e, "There is some text missing from th", 0, "Ais line.<esc>")
	tutorStrictEditLineAt(t, e, "There is also some text miss", 0, "Aing here.<esc>")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 146))
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 148))

	tutorStrictEditLineAt(t, e, "There are a some words fun that don't belong paper in this sentence.", len([]rune("There are ")), "dw")
	tutorStrictEditLineAt(t, e, "There are some words fun that don't belong paper in this sentence.", len([]rune("There are some words ")), "dw")
	tutorStrictEditLineAt(t, e, "There are some words that don't belong paper in this sentence.", len([]rune("There are some words that don't belong ")), "dw")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 222))

	tutorStrictEditAt(t, e, ". end", 1, "d$")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 238))

	tutorStrictEditAt(t, e, "ABC", 0, "d2w")
	tutorStrictEditAt(t, e, "FGHI", 0, "d4w")
	tutorStrictEditAt(t, e, "Q", 0, "d3w")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 297))

	tutorStrictEditAt(t, e, "2)  Mud is fun,", 0, "dd")
	tutorStrictEditAt(t, e, "4)  I have a car,", 0, "2dd")
	tutorStrictAssertLine(t, e, "1)  Roses are red,")
	tutorStrictAssertLine(t, e, "3)  Violets are blue,")
	tutorStrictAssertLine(t, e, "6)  Sugar is sweet")
	tutorStrictAssertLine(t, e, "7)  And so are you.")
	tutorStrictAssertNoLine(t, e, "2)  Mud is fun,")
	tutorStrictAssertNoLine(t, e, "4)  I have a car,")

	tutorStrictEditLineAt(t, e, "Fiix the errors oon thhis line and reeplace them witth undo.", 1, "x")
	pressKeyScript(t, e, "u")
	tutorStrictAssertLine(t, e, "Fiix the errors oon thhis line and reeplace them witth undo.")
	tutorStrictEditLineAt(t, e, "Fiix the errors oon thhis line and reeplace them witth undo.", 1, "x")
	tutorStrictEditLineAt(t, e, "Fix the errors oon thhis line and reeplace them witth undo.", len([]rune("Fix the errors o")), "x")
	tutorStrictEditLineAt(t, e, "Fix the errors on thhis line and reeplace them witth undo.", len([]rune("Fix the errors on th")), "x")
	tutorStrictEditLineAt(t, e, "Fix the errors on this line and reeplace them witth undo.", len([]rune("Fix the errors on this line and r")), "x")
	tutorStrictEditLineAt(t, e, "Fix the errors on this line and replace them witth undo.", len([]rune("Fix the errors on this line and replace them wit")), "x")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 334))
	pressKeyScript(t, e, "Uu<ctrl+r>u")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 334))

	tutorStrictEditAt(t, e, "Whan", 2, "re")
	tutorStrictEditAt(t, e, "lime", 2, "rn")
	tutorStrictEditAt(t, e, "tuoed", 1, "ry")
	tutorStrictEditAt(t, e, "tyoed", 2, "rp")
	tutorStrictEditAt(t, e, "presswd", 5, "re")
	tutorStrictEditAt(t, e, "wrojg", 3, "rn")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 391))

	tutorStrictEditLineAt(t, e, "This lubw has a few wptfd that mrrf changing usf the change operator.", len([]rune("This l")), "ceine<esc>")
	tutorStrictEditLineAt(t, e, "This line has a few wptfd that mrrf changing usf the change operator.", len([]rune("This line has a few ")), "cewords<esc>")
	tutorStrictEditLineAt(t, e, "This line has a few words that mrrf changing usf the change operator.", len([]rune("This line has a few words that ")), "ceneed<esc>")
	tutorStrictEditLineAt(t, e, "This line has a few words that need changing usf the change operator.", len([]rune("This line has a few words that need changing ")), "ceusing<esc>")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 413))

	tutorStrictEditAt(t, e, "some help", 0, "c$to be corrected using the c$ command.<esc>")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 434))

	tutorStrictEditAt(t, e, "Usually thee best time", 0, ":s/thee/the/g<enter>")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 543))

	tutorStrictEditAt(t, e, "This li will allow", len([]rune("This li"))-1, "ane<esc>")
	tutorStrictEditAt(t, e, "pract appendi", len([]rune("pract"))-1, "aice<esc>")
	tutorStrictEditAt(t, e, "appendi text", len([]rune("appendi"))-1, "ang<esc>")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 761))

	tutorStrictEditAt(t, e, "xxx gives you xxx", 0, "R456<esc>")
	tutorStrictEditAt(t, e, "xxx.", 0, "R579<esc>")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 782))

	tutorStrictEditLineAt(t, e, "a) This is the first item.", len([]rune("a)")), "v12ly")
	tutorStrictEditLineAt(t, e, "b)", 0, "$pA second<esc>")
	tutorStrictEditLineAt(t, e, "a) This is the first item.", len([]rune("a) This is the first")), "v6ly")
	tutorStrictEditLineAt(t, e, "b) This is the second", 0, "$p")
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 809))
	tutorStrictAssertLine(t, e, tutorExpectedLine(t, expects, 810))
}

func TestHelixTutorStrictUserKeyScript(t *testing.T) {
	lines := loadHelixTutor(t)
	e := newSimulatedProfileEditor(BehaviorProfileHelix, "scratch")

	pressKeyScript(t, e, ":tutor helix<enter>")

	tutorStrictEditAt(t, e, "Thhiss senttencee haass exxtra charracterss.", 1, "d")
	tutorStrictEditAt(t, e, "Thiss", 3, "d")
	tutorStrictEditAt(t, e, "senttencee", 4, "d")
	tutorStrictEditAt(t, e, "sentencee", 8, "d")
	tutorStrictEditAt(t, e, "haass", 1, "d")
	tutorStrictEditAt(t, e, "hass", 3, "d")
	tutorStrictEditAt(t, e, "exxtra", 2, "d")
	tutorStrictEditAt(t, e, "charracterss", 4, "d")
	tutorStrictEditAt(t, e, "characterss", 10, "d")
	tutorStrictAssertLine(t, e, " --> "+helixExpectedLine(t, lines, 104))

	tutorStrictEditLineAt(t, e, " --> Th stce misg so.", len([]rune(" --> Th")), "iis<esc>")
	tutorStrictEditLineAt(t, e, " --> This stce misg so.", len([]rune(" --> This s")), "ien<esc>")
	tutorStrictEditLineAt(t, e, " --> This sentce misg so.", len([]rune(" --> This sent")), "ien<esc>")
	tutorStrictEditLineAt(t, e, " --> This sentence misg so.", len([]rune(" --> This sentence ")), "iis <esc>")
	tutorStrictEditLineAt(t, e, " --> This sentence is misg so.", len([]rune(" --> This sentence is mis")), "isin<esc>")
	tutorStrictEditLineAt(t, e, " --> This sentence is missing so.", len([]rune(" --> This sentence is missing so")), "ime text<esc>")
	tutorStrictAssertLine(t, e, " --> "+helixExpectedLine(t, lines, 129))

	tutorStrictEditLineAt(t, e, " --> This sentence is miss", 0, "Aing some text.<esc>")
	tutorStrictAssertLine(t, e, " --> "+helixExpectedLine(t, lines, 201))

	tutorStrictEditLineAt(t, e, " --> What is the best editor?", 0, "oHelix is the best editor.<esc>")
	tutorStrictAssertLine(t, e, " Helix is the best editor.")

	tutorStrictEditAt(t, e, "pencil", 0, "wd")
	tutorStrictEditAt(t, e, "vacuum", 0, "wd")
	tutorStrictEditAt(t, e, "the it", 0, "wd")
	tutorStrictAssertLine(t, e, " --> "+helixExpectedLine(t, lines, 264))

	tutorStrictEditAt(t, e, "paper", 0, "ecsentence<esc>")
	tutorStrictEditAt(t, e, "heavy", 0, "ecincorrect<esc>")
	tutorStrictEditAt(t, e, "behind", 0, "ecin<esc>")
	tutorStrictAssertLine(t, e, " --> "+helixExpectedLine(t, lines, 330))

	tutorStrictEditLineAt(t, e, " --> Remove the FOO BAR distracting words BAZ BIZ from this line.", len([]rune(" --> Remove the ")), "v2wd")
	tutorStrictEditLineAt(t, e, " --> Remove the distracting words BAZ BIZ from this line.", len([]rune(" --> Remove the distracting words ")), "v2wd")
	tutorStrictAssertLine(t, e, " --> Remove the distracting words from this line.")

	tutorStrictEditAt(t, e, "2) Mud is fun,", 0, "xd")
	tutorStrictEditAt(t, e, "4) I have a car,", 0, "2xd")
	tutorStrictAssertLine(t, e, " --> 1) Roses are red,")
	tutorStrictAssertLine(t, e, " --> 3) Violets are blue,")
	tutorStrictAssertLine(t, e, " --> 6) Sugar is sweet,")
	tutorStrictAssertLine(t, e, " --> 7) And so are you.")

	tutorStrictEditAt(t, e, "Fiix", 1, "d")
	pressKeyScript(t, e, "u")
	tutorStrictAssertLine(t, e, " --> Fiix the errors on thhis line and reeplace them witth undo.")
	tutorStrictEditAt(t, e, "Fiix", 1, "d")
	tutorStrictEditAt(t, e, "thhis", 2, "d")
	tutorStrictEditAt(t, e, "reeplace", 1, "d")
	tutorStrictEditAt(t, e, "witth", 2, "d")
	tutorStrictAssertLine(t, e, " --> Fix the errors on this line and replace them with undo.")

	tutorStrictEditLineAt(t, e, " --> 1 banana 2 3 4", len([]rune(" --> 1 ")), "wy")
	tutorStrictEditLineAt(t, e, " --> 1 banana 2 3 4", len([]rune(" --> 1 banana 2")), "p")
	tutorStrictEditLineAt(t, e, " --> 1 banana 2 banana 3 4", len([]rune(" --> 1 banana 2 banana 3")), "p")
	tutorStrictAssertLine(t, e, " --> "+helixExpectedLine(t, lines, 482))

	tutorStrictEditLineOccurrenceAt(t, e, " --> Fix th two nes at same ime.", 1, len([]rune(" --> Fix th")), "Ciese<esc>")
	tutorStrictEditLineOccurrenceAt(t, e, " --> Fix these two nes at same ime.", 1, len([]rune(" --> Fix these two ")), "Cili<esc>")
	tutorStrictEditLineOccurrenceAt(t, e, " --> Fix these two lines at same ime.", 1, len([]rune(" --> Fix these two lines at same ")), "Cit<esc>")
	tutorStrictEditLineOccurrenceAt(t, e, " --> Fix these two lines at same time.", 1, len([]rune(" --> Fix these two lines at ")), "Cithe <esc>")
	tutorStrictAssertLineCount(t, e, " --> Fix these two lines at the same time.", 2)

	tutorStrictEditLineAt(t, e, " --> I like to eat apples since my favorite fruit is apples.", 0, "xsapples<enter>coranges<esc>")
	tutorStrictAssertLine(t, e, " --> I like to eat oranges since my favorite fruit is oranges.")

	tutorStrictEditLineAt(t, e, " --> This  sentence has   some      extra spaces.", 0, "xs  +<enter>c <esc>")
	tutorStrictAssertLine(t, e, " --> This sentence has some extra spaces.")

	tutorStrictEditLineAt(t, e, " --> 97) lorem", len([]rune(" --> ")), "4CW&")
	tutorStrictAssertLine(t, e, " --> 97)  lorem")
	tutorStrictAssertLine(t, e, " --> 100) sit")

	tutorStrictEditLineAt(t, e, " --> -----[Free this sentence of its brackets!]-----", len([]rune(" --> ")), "f[d")
	tutorStrictEditLineAt(t, e, " --> Free this sentence of its brackets!]-----", len([]rune(" --> Free this sentence of its brackets!]----")), "F]d")
	tutorStrictAssertLine(t, e, " --> Free this sentence of its brackets!")
	tutorStrictEditLineAt(t, e, " --> ------Free this sentence of its dashes!------", len([]rune(" --> ")), "tFd")
	tutorStrictEditLineAt(t, e, " --> Free this sentence of its dashes!------", len([]rune(" --> Free this sentence of its dashes!-----")), "T!d")
	tutorStrictAssertLine(t, e, " --> Free this sentence of its dashes!")

	tutorStrictEditLineAt(t, e, " |=======|------|", len([]rune(" |")), "t|r-")
	tutorStrictAssertLine(t, e, " |-------|------|")

	tutorStrictEditLineAt(t, e, " --> I like watermelons because oranges are refreshing.", len([]rune(" --> I like ")), "wy")
	tutorStrictEditLineAt(t, e, " --> I like watermelons because oranges are refreshing.", len([]rune(" --> I like watermelons because ")), "wR")
	tutorStrictAssertLine(t, e, " --> I like watermelons because watermelons are refreshing.")

	tutorStrictEditLineAt(t, e, " --> This sentence", 0, "4xJ")
	tutorStrictAssertLine(t, e, " --> This sentence is spilling over onto other lines.")

	tutorStrictEditLineAt(t, e, "    are indented", 0, ">")
	tutorStrictAssertLine(t, e, "\t    are indented")
	tutorStrictEditLineAt(t, e, "         very poorly.", 0, "<lt>")
	tutorStrictAssertLine(t, e, "     very poorly.")

	tutorStrictEditLineAt(t, e, " --> 2) Added point.", len([]rune(" --> ")), "<ctrl+a>")
	tutorStrictAssertLine(t, e, " --> 3) Added point.")
	tutorStrictEditLineAt(t, e, " --> 3) Another point.", len([]rune(" --> ")), "<ctrl+a>")
	tutorStrictAssertLine(t, e, " --> 4) Another point.")
	tutorStrictEditLineAt(t, e, " --> 6) Last point.", len([]rune(" --> ")), "<ctrl+x>")
	tutorStrictAssertLine(t, e, " --> 5) Last point.")

	tutorStrictEditLineAt(t, e, " --> Everybody wants to be a bat,", len([]rune(" --> Everybody wants to be a ")), "e*vnccat<esc>")
	tutorStrictAssertLine(t, e, " --> Everybody wants to be a cat,")
	tutorStrictAssertLine(t, e, " --> because a cat's the only cat")

	tutorStrictEditLineAt(t, e, " --> How much would would a wouldchuck chuck", 0, "2xswould<enter>)<alt+,>cwood<esc>")
	tutorStrictAssertLine(t, e, " --> How much wood would a woodchuck chuck")
	tutorStrictAssertLine(t, e, " --> if a woodchuck could chuck wood?")

	tutorStrictEditLineAt(t, e, " --> Jumping through the water,", 0, "2xsthrough|water|know<enter><alt+)>")
	tutorStrictAssertLine(t, e, " --> Jumping know the through,")
	tutorStrictAssertLine(t, e, " --> daring to water.")

	tutorStrictEditLineAt(t, e, " --> thIs sENtencE hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> ")), "~")
	tutorStrictEditLineAt(t, e, " --> ThIs sENtencE hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> Th")), "~")
	tutorStrictEditLineAt(t, e, " --> This sENtencE hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> This s")), "~")
	tutorStrictEditLineAt(t, e, " --> This seNtencE hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> This se")), "~")
	tutorStrictEditLineAt(t, e, " --> This sentencE hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> This sentenc")), "~")
	tutorStrictEditLineAt(t, e, " --> This sentence hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> This sentence h")), "~")
	tutorStrictEditAt(t, e, "MIS-cApitalIsed", 0, "W`")
	tutorStrictEditAt(t, e, "leTTerS", 0, "e`")
	tutorStrictAssertLine(t, e, " --> This sentence has mis-capitalised letters.")
	tutorStrictEditLineAt(t, e, " --> this SENTENCE SHOULD all be in LOWERCASE.", 0, "x`")
	tutorStrictAssertLine(t, e, " --> this sentence should all be in lowercase.")
	tutorStrictEditLineAt(t, e, " --> THIS sentence should ALL BE IN uppercase!", 0, "x<alt+`>")
	tutorStrictAssertLine(t, e, " --> THIS SENTENCE SHOULD ALL BE IN UPPERCASE!")

	tutorStrictSearch(t, e, "these are sentences. some sentences don't start with uppercase")
	pressKeyScript(t, e, "2xS\\. |! <enter><alt+;>;<alt+`>")
	tutorStrictAssertLine(t, e, "These are sentences. Some sentences don't start with uppercase")
	tutorStrictAssertLine(t, e, "letters! That is not good grammar. You can fix this.")

	tutorStrictEditLineAt(t, e, " --> Comment me please", 0, "<ctrl+c>")
	tutorStrictAssertLine(t, e, " // --> Comment me please")
	tutorStrictEditLineAt(t, e, " // --> Comment me please", 0, "<ctrl+c>")
	tutorStrictAssertLine(t, e, " --> Comment me please")

	tutorStrictEditLineAt(t, e, " --> What are you doing?!", 0, "4x<ctrl+c>")
	tutorStrictAssertLine(t, e, " // --> What are you doing?!")
	tutorStrictAssertLine(t, e, " // --> Enough! Uncomment me now!")
	tutorStrictEditLineAt(t, e, " // --> What are you doing?!", 0, "4x<ctrl+c>")
	tutorStrictAssertLine(t, e, " --> What are you doing?!")
	tutorStrictAssertLine(t, e, " --> Enough! Uncomment me now!")

	tutorStrictEditLineAt(t, e, " --> so, select all of this, and surround it with ()", len([]rune(" --> so, ")), "v4ems(")
	tutorStrictAssertLine(t, e, " --> so, (select all of this), and surround it with ()")
	tutorStrictEditLineAt(t, e, " --> delete (the x pair of parentheses) from within!", len([]rune(" --> delete (the ")), "md(")
	tutorStrictAssertLine(t, e, " --> delete the x pair of parentheses from within!")
	tutorStrictEditLineAt(t, e, " --> replace the (pair from x within), with something else", len([]rune(" --> replace the (pair from ")), "mr([")
	tutorStrictAssertLine(t, e, " --> replace the [pair from x within], with something else")
}

func tutorStrictEditAt(t *testing.T, e *Editor, needle string, offset int, script string) {
	t.Helper()
	tutorStrictSearch(t, e, needle)
	tutorStrictMoveRight(t, e, offset)
	pressKeyScript(t, e, script)
	if e.mode == ModeInsert {
		t.Fatalf("script %q left editor in insert mode", script)
	}
}

func tutorStrictEditLineAt(t *testing.T, e *Editor, line string, offset int, script string) {
	t.Helper()
	tutorStrictSearchLine(t, e, line)
	tutorStrictMoveRight(t, e, offset)
	pressKeyScript(t, e, script)
	if e.mode == ModeInsert {
		t.Fatalf("script %q left editor in insert mode", script)
	}
}

func tutorStrictEditLineOccurrenceAt(t *testing.T, e *Editor, line string, occurrence int, offset int, script string) {
	t.Helper()
	tutorStrictSearchLineOccurrence(t, e, line, occurrence)
	tutorStrictMoveRight(t, e, offset)
	pressKeyScript(t, e, script)
	if e.mode == ModeInsert {
		t.Fatalf("script %q left editor in insert mode", script)
	}
}

func tutorStrictSearch(t *testing.T, e *Editor, needle string) {
	t.Helper()
	pressKeyScript(t, e, "<esc>gg/")
	tutorStrictTypeLiteral(t, e, needle)
	if got := string(e.searchQuery); got != needle {
		t.Fatalf("search query after typing = %q, want %q", got, needle)
	}
	if matches := tutorStrictExactMatchCount(e, needle); matches != 1 {
		t.Fatalf("search for %q has %d exact matches before confirmation, want 1", needle, matches)
	}
	pressKeyScript(t, e, "<enter>")
	if e.mode != ModeNormal {
		t.Fatalf("search for %q left mode = %v, want normal", needle, e.mode)
	}
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		t.Fatalf("search for %q left cursor row out of range: %+v", needle, e.cursor)
	}
	line := string(e.line(e.cursor.Row))
	if e.cursor.Col < 0 || e.cursor.Col > len([]rune(line)) {
		t.Fatalf("search for %q left cursor col out of range: line=%q cursor=%+v", needle, line, e.cursor)
	}
	got := string([]rune(line)[e.cursor.Col:])
	if !strings.HasPrefix(got, needle) {
		t.Fatalf("search for %q landed at line %q col %d", needle, line, e.cursor.Col)
	}
}

func tutorStrictSearchLine(t *testing.T, e *Editor, line string) {
	t.Helper()
	pattern := "^" + regexp.QuoteMeta(line) + "$"
	pressKeyScript(t, e, "<esc>gg<cmd+e>")
	tutorStrictTypeLiteral(t, e, pattern)
	if got := string(e.searchQuery); got != pattern {
		t.Fatalf("search query after typing = %q, want %q", got, pattern)
	}
	if matches := len(e.searchMatches); matches != 1 {
		t.Fatalf("line search for %q has %d matches before confirmation, want 1", line, matches)
	}
	pressKeyScript(t, e, "<enter>")
	if e.mode != ModeNormal {
		t.Fatalf("line search for %q left mode = %v, want normal", line, e.mode)
	}
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		t.Fatalf("line search for %q left cursor row out of range: %+v", line, e.cursor)
	}
	if got := string(e.line(e.cursor.Row)); got != line {
		t.Fatalf("line search for %q landed at line %q col %d", line, got, e.cursor.Col)
	}
	if e.cursor.Col != 0 {
		t.Fatalf("line search for %q landed at col %d, want 0", line, e.cursor.Col)
	}
}

func tutorStrictSearchLineOccurrence(t *testing.T, e *Editor, line string, occurrence int) {
	t.Helper()
	if occurrence < 1 {
		t.Fatalf("line search for %q requested invalid occurrence %d", line, occurrence)
	}
	pattern := "^" + regexp.QuoteMeta(line) + "$"
	pressKeyScript(t, e, "<esc>gg<cmd+e>")
	tutorStrictTypeLiteral(t, e, pattern)
	if got := string(e.searchQuery); got != pattern {
		t.Fatalf("search query after typing = %q, want %q", got, pattern)
	}
	if matches := len(e.searchMatches); matches < occurrence {
		t.Fatalf("line search for %q has %d matches before confirmation, want at least %d", line, matches, occurrence)
	}
	pressKeyScript(t, e, "<enter>")
	for i := 1; i < occurrence; i++ {
		pressKeyScript(t, e, "n")
	}
	if e.mode != ModeNormal {
		t.Fatalf("line search for %q left mode = %v, want normal", line, e.mode)
	}
	if e.cursor.Row < 0 || e.cursor.Row >= e.LineCount() {
		t.Fatalf("line search for %q left cursor row out of range: %+v", line, e.cursor)
	}
	if got := string(e.line(e.cursor.Row)); got != line {
		t.Fatalf("line search for %q landed at line %q col %d", line, got, e.cursor.Col)
	}
	if e.cursor.Col != 0 {
		t.Fatalf("line search for %q landed at col %d, want 0", line, e.cursor.Col)
	}
}

func tutorStrictExactMatchCount(e *Editor, needle string) int {
	matches := 0
	for row := 0; row < e.LineCount(); row++ {
		if strings.Contains(string(e.line(row)), needle) {
			matches++
		}
	}
	return matches
}

func tutorStrictTypeLiteral(t *testing.T, e *Editor, text string) {
	t.Helper()
	for _, r := range text {
		e.HandleKey(keyRune(r))
	}
}

func tutorStrictMoveRight(t *testing.T, e *Editor, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		e.HandleKey(keyRune('l'))
	}
}

func tutorStrictAssertLine(t *testing.T, e *Editor, line string) {
	t.Helper()
	if !tutorStrictHasLine(e, line) {
		t.Fatalf("tutorial buffer does not contain line %q", line)
	}
}

func tutorStrictAssertNoLine(t *testing.T, e *Editor, line string) {
	t.Helper()
	if tutorStrictHasLine(e, line) {
		t.Fatalf("tutorial buffer still contains line %q", line)
	}
}

func tutorStrictAssertLineCount(t *testing.T, e *Editor, line string, want int) {
	t.Helper()
	got := 0
	for row := 0; row < e.LineCount(); row++ {
		if string(e.line(row)) == line {
			got++
		}
	}
	if got != want {
		t.Fatalf("tutorial buffer contains line %q %d times, want %d", line, got, want)
	}
}

func tutorStrictHasLine(e *Editor, line string) bool {
	for row := 0; row < e.LineCount(); row++ {
		if string(e.line(row)) == line {
			return true
		}
	}
	return false
}
