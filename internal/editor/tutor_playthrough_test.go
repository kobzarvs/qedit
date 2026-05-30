package editor

import "testing"

// Tutor playthrough tests exercise the embedded Vim and Helix tutorials end-to-end:
// :tutor opens the real scratch buffer, navigation uses hjkl, edits use
// pressKeyScript (HandleKey only). Expectations are checked against testdata/*.json.

func TestVimBeginnerTutorPlaythrough(t *testing.T) {
	_, expects := loadVimBeginnerTutor(t)
	e := openTutorViaKeys(t, BehaviorProfileVim)

	tutorPlaythroughEditAt(t, e, "The ccow jumpedd ovverr thhe mooon.", 4, "x")
	tutorPlaythroughAssertLine(t, e, "The cow jumpedd ovverr thhe mooon.")
	tutorPlaythroughEditAt(t, e, "jumpedd", 6, "x")
	tutorPlaythroughEditAt(t, e, "ovverr", 2, "x")
	tutorPlaythroughEditAt(t, e, "overr", 4, "x")
	tutorPlaythroughEditAt(t, e, "thhe", 2, "x")
	tutorPlaythroughEditAt(t, e, "mooon", 2, "x")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 105))

	tutorPlaythroughEditLineAt(t, e, "There is text misng this .", len([]rune("There is ")), "isome <esc>")
	tutorPlaythroughEditLineAt(t, e, "There is some text misng this .", len([]rune("There is some text mis")), "isi<esc>")
	tutorPlaythroughEditLineAt(t, e, "There is some text missing this .", len([]rune("There is some text missing ")), "ifrom <esc>")
	tutorPlaythroughEditLineAt(t, e, "There is some text missing from this .", len([]rune("There is some text missing from this ")), "iline<esc>")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 126))

	tutorPlaythroughEditLineAt(t, e, "There is some text missing from th", 0, "Ais line.<esc>")
	tutorPlaythroughEditLineAt(t, e, "There is also some text miss", 0, "Aing here.<esc>")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 146))
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 148))

	tutorPlaythroughEditLineAt(t, e, "There are a some words fun that don't belong paper in this sentence.", len([]rune("There are ")), "dw")
	tutorPlaythroughEditLineAt(t, e, "There are some words fun that don't belong paper in this sentence.", len([]rune("There are some words ")), "dw")
	tutorPlaythroughEditLineAt(t, e, "There are some words that don't belong paper in this sentence.", len([]rune("There are some words that don't belong ")), "dw")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 222))

	tutorPlaythroughEditAt(t, e, ". end", 1, "d$")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 238))

	tutorPlaythroughEditAt(t, e, "ABC", 0, "d2w")
	tutorPlaythroughEditAt(t, e, "FGHI", 0, "d4w")
	tutorPlaythroughEditAt(t, e, "Q", 0, "d3w")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 297))

	tutorPlaythroughEditAt(t, e, "2)  Mud is fun,", 0, "dd")
	tutorPlaythroughEditAt(t, e, "4)  I have a car,", 0, "2dd")
	tutorPlaythroughAssertLine(t, e, "1)  Roses are red,")
	tutorPlaythroughAssertLine(t, e, "3)  Violets are blue,")
	tutorPlaythroughAssertLine(t, e, "6)  Sugar is sweet")
	tutorPlaythroughAssertLine(t, e, "7)  And so are you.")
	tutorPlaythroughAssertNoLine(t, e, "2)  Mud is fun,")
	tutorPlaythroughAssertNoLine(t, e, "4)  I have a car,")

	tutorPlaythroughEditLineAt(t, e, "Fiix the errors oon thhis line and reeplace them witth undo.", 1, "x")
	pressKeyScript(t, e, "u")
	tutorPlaythroughAssertLine(t, e, "Fiix the errors oon thhis line and reeplace them witth undo.")
	tutorPlaythroughEditLineAt(t, e, "Fiix the errors oon thhis line and reeplace them witth undo.", 1, "x")
	tutorPlaythroughEditLineAt(t, e, "Fix the errors oon thhis line and reeplace them witth undo.", len([]rune("Fix the errors o")), "x")
	tutorPlaythroughEditLineAt(t, e, "Fix the errors on thhis line and reeplace them witth undo.", len([]rune("Fix the errors on th")), "x")
	tutorPlaythroughEditLineAt(t, e, "Fix the errors on this line and reeplace them witth undo.", len([]rune("Fix the errors on this line and r")), "x")
	tutorPlaythroughEditLineAt(t, e, "Fix the errors on this line and replace them witth undo.", len([]rune("Fix the errors on this line and replace them wit")), "x")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 334))
	pressKeyScript(t, e, "Uu<ctrl+r>u")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 334))

	tutorPlaythroughEditAt(t, e, "Whan", 2, "re")
	tutorPlaythroughEditAt(t, e, "lime", 2, "rn")
	tutorPlaythroughEditAt(t, e, "tuoed", 1, "ry")
	tutorPlaythroughEditAt(t, e, "tyoed", 2, "rp")
	tutorPlaythroughEditAt(t, e, "presswd", 5, "re")
	tutorPlaythroughEditAt(t, e, "wrojg", 3, "rn")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 391))

	tutorPlaythroughEditLineAt(t, e, "This lubw has a few wptfd that mrrf changing usf the change operator.", len([]rune("This l")), "ceine<esc>")
	tutorPlaythroughEditLineAt(t, e, "This line has a few wptfd that mrrf changing usf the change operator.", len([]rune("This line has a few ")), "cewords<esc>")
	tutorPlaythroughEditLineAt(t, e, "This line has a few words that mrrf changing usf the change operator.", len([]rune("This line has a few words that ")), "ceneed<esc>")
	tutorPlaythroughEditLineAt(t, e, "This line has a few words that need changing usf the change operator.", len([]rune("This line has a few words that need changing ")), "ceusing<esc>")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 413))

	tutorPlaythroughEditAt(t, e, "some help", 0, "c$to be corrected using the c$ command.<esc>")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 434))

	tutorPlaythroughEditAt(t, e, "Usually thee best time", 0, ":s/thee/the/g<enter>")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 543))

	tutorPlaythroughEditAt(t, e, "This li will allow", len([]rune("This li"))-1, "ane<esc>")
	tutorPlaythroughEditAt(t, e, "pract appendi", len([]rune("pract"))-1, "aice<esc>")
	tutorPlaythroughEditAt(t, e, "appendi text", len([]rune("appendi"))-1, "ang<esc>")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 761))

	tutorPlaythroughEditAt(t, e, "xxx gives you xxx", 0, "R456<esc>")
	tutorPlaythroughEditAt(t, e, "xxx.", 0, "R579<esc>")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 782))

	// Lesson 6.4: yank text after "a)" through before "first", paste on "b)" line, append "second", then yank "item."
	tutorPlaythroughEditLineAt(t, e, "a) This is the first item.", len([]rune("a)")), "v12ly")
	tutorPlaythroughGotoLineExact(t, e, "b)", 1)
	pressKeyScript(t, e, "$pasecond<esc>")
	tutorPlaythroughEditLineAt(t, e, "a) This is the first item.", len([]rune("a) This is the first")), "v6ly")
	tutorPlaythroughGotoLineExact(t, e, "b) This is the second", 1)
	pressKeyScript(t, e, "$p")
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 809))
	tutorPlaythroughAssertLine(t, e, tutorExpectedLine(t, expects, 810))

	playthroughExpects := loadVimTutorPlaythroughExpectations(t)
	assertVimTutorPlaythroughExpectations(t, e, playthroughExpects)
	if e.BehaviorProfile() != BehaviorProfileVim {
		t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileVim)
	}
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want %v", e.mode, ModeNormal)
	}
}

func TestHelixTutorPlaythrough(t *testing.T) {
	lines, expects := loadHelixTutorExpectations(t)
	e := openTutorViaKeys(t, BehaviorProfileHelix)

	tutorPlaythroughEditAt(t, e, "Thhiss senttencee haass exxtra charracterss.", 1, "d")
	tutorPlaythroughEditAt(t, e, "Thiss", 3, "d")
	tutorPlaythroughEditAt(t, e, "senttencee", 4, "d")
	tutorPlaythroughEditAt(t, e, "sentencee", 8, "d")
	tutorPlaythroughEditAt(t, e, "haass", 1, "d")
	tutorPlaythroughEditAt(t, e, "hass", 3, "d")
	tutorPlaythroughEditAt(t, e, "exxtra", 2, "d")
	tutorPlaythroughEditAt(t, e, "charracterss", 4, "d")
	tutorPlaythroughEditAt(t, e, "characterss", 10, "d")
	tutorPlaythroughAssertLine(t, e, helixMarkedExpectedLine(t, lines, 104))

	tutorPlaythroughEditLineAt(t, e, " --> Th stce misg so.", len([]rune(" --> Th")), "iis<esc>")
	tutorPlaythroughEditLineAt(t, e, " --> This stce misg so.", len([]rune(" --> This s")), "ien<esc>")
	tutorPlaythroughEditLineAt(t, e, " --> This sentce misg so.", len([]rune(" --> This sent")), "ien<esc>")
	tutorPlaythroughEditLineAt(t, e, " --> This sentence misg so.", len([]rune(" --> This sentence ")), "iis <esc>")
	tutorPlaythroughEditLineAt(t, e, " --> This sentence is misg so.", len([]rune(" --> This sentence is mis")), "isin<esc>")
	tutorPlaythroughEditLineAt(t, e, " --> This sentence is missing so.", len([]rune(" --> This sentence is missing so")), "ime text<esc>")
	tutorPlaythroughAssertLine(t, e, helixMarkedExpectedLine(t, lines, 129))

	tutorPlaythroughEditLineAt(t, e, " --> This sentence is miss", 0, "Aing some text.<esc>")
	tutorPlaythroughAssertLine(t, e, helixMarkedExpectedLine(t, lines, 201))

	tutorPlaythroughEditLineAt(t, e, " --> What is the best editor?", 0, "oHelix is the best editor.<esc>")
	tutorPlaythroughAssertLine(t, e, " Helix is the best editor.")

	tutorPlaythroughEditAt(t, e, "pencil", 0, "wd")
	tutorPlaythroughEditAt(t, e, "vacuum", 0, "wd")
	tutorPlaythroughEditAt(t, e, "the it", 0, "wd")
	tutorPlaythroughAssertLine(t, e, helixMarkedExpectedLine(t, lines, 264))

	tutorPlaythroughEditAt(t, e, "paper", 0, "ecsentence<esc>")
	tutorPlaythroughEditAt(t, e, "heavy", 0, "ecincorrect<esc>")
	tutorPlaythroughEditAt(t, e, "behind", 0, "ecin<esc>")
	tutorPlaythroughAssertLine(t, e, helixMarkedExpectedLine(t, lines, 330))

	tutorPlaythroughEditLineAt(t, e, " --> Remove the FOO BAR distracting words BAZ BIZ from this line.", len([]rune(" --> Remove the ")), "v2wd")
	tutorPlaythroughEditLineAt(t, e, " --> Remove the distracting words BAZ BIZ from this line.", len([]rune(" --> Remove the distracting words ")), "v2wd")
	tutorPlaythroughAssertLine(t, e, " --> Remove the distracting words from this line.")

	tutorPlaythroughEditAt(t, e, "2) Mud is fun,", 0, "xd")
	tutorPlaythroughEditAt(t, e, "4) I have a car,", 0, "2xd")
	tutorPlaythroughAssertLine(t, e, " --> 1) Roses are red,")
	tutorPlaythroughAssertLine(t, e, " --> 3) Violets are blue,")
	tutorPlaythroughAssertLine(t, e, " --> 6) Sugar is sweet,")
	tutorPlaythroughAssertLine(t, e, " --> 7) And so are you.")

	tutorPlaythroughEditAt(t, e, "Fiix", 1, "d")
	pressKeyScript(t, e, "u")
	tutorPlaythroughAssertLine(t, e, " --> Fiix the errors on thhis line and reeplace them witth undo.")
	tutorPlaythroughEditAt(t, e, "Fiix", 1, "d")
	tutorPlaythroughEditAt(t, e, "thhis", 2, "d")
	tutorPlaythroughEditAt(t, e, "reeplace", 1, "d")
	tutorPlaythroughEditAt(t, e, "witth", 2, "d")
	tutorPlaythroughAssertLine(t, e, " --> Fix the errors on this line and replace them with undo.")

	tutorPlaythroughEditLineAt(t, e, " --> 1 banana 2 3 4", len([]rune(" --> 1 ")), "wy")
	tutorPlaythroughEditLineAt(t, e, " --> 1 banana 2 3 4", len([]rune(" --> 1 banana 2")), "p")
	tutorPlaythroughEditLineAt(t, e, " --> 1 banana 2 banana 3 4", len([]rune(" --> 1 banana 2 banana 3")), "p")
	tutorPlaythroughAssertLine(t, e, helixMarkedExpectedLine(t, lines, 482))

	tutorPlaythroughEditLineOccurrenceAt(t, e, " --> Fix th two nes at same ime.", 1, len([]rune(" --> Fix th")), "Ciese<esc>")
	tutorPlaythroughEditLineOccurrenceAt(t, e, " --> Fix these two nes at same ime.", 1, len([]rune(" --> Fix these two ")), "Cili<esc>")
	tutorPlaythroughEditLineOccurrenceAt(t, e, " --> Fix these two lines at same ime.", 1, len([]rune(" --> Fix these two lines at same ")), "Cit<esc>")
	tutorPlaythroughEditLineOccurrenceAt(t, e, " --> Fix these two lines at same time.", 1, len([]rune(" --> Fix these two lines at ")), "Cithe <esc>")
	tutorPlaythroughAssertLineCount(t, e, " --> Fix these two lines at the same time.", 2)
	pressKeyScript(t, e, ",")

	tutorPlaythroughEditLineAt(t, e, " --> I like to eat apples since my favorite fruit is apples.", 0, "xsapples<enter>coranges<esc>,")
	tutorPlaythroughAssertLine(t, e, " --> I like to eat oranges since my favorite fruit is oranges.")

	tutorPlaythroughEditLineAt(t, e, " --> This  sentence has   some      extra spaces.", 0, "xs  +<enter>c <esc>")
	tutorPlaythroughAssertLine(t, e, " --> This sentence has some extra spaces.")

	tutorPlaythroughEditLineAt(t, e, " --> 97) lorem", len([]rune(" --> ")), "4CW&<esc>")
	tutorPlaythroughAssertLine(t, e, " --> 97)  lorem")
	tutorPlaythroughAssertLine(t, e, " --> 100) sit")

	pressKeyScript(t, e, "<esc>,")
	tutorPlaythroughEditLineAt(t, e, " --> -----[Free this sentence of its brackets!]-----", len([]rune(" --> ")), "f[d")
	tutorPlaythroughEditLineAt(t, e, " --> Free this sentence of its brackets!]-----", len([]rune(" --> Free this sentence of its brackets!]----")), "F]d")
	tutorPlaythroughAssertLine(t, e, " --> Free this sentence of its brackets!")
	tutorPlaythroughEditLineAt(t, e, " --> ------Free this sentence of its dashes!------", len([]rune(" --> ")), "tFd")
	tutorPlaythroughEditLineAt(t, e, " --> Free this sentence of its dashes!------", len([]rune(" --> Free this sentence of its dashes!-----")), "T!d")
	tutorPlaythroughAssertLine(t, e, " --> Free this sentence of its dashes!")

	tutorPlaythroughEditLineAt(t, e, " |=======|------|", len([]rune(" |")), "t|r-")
	tutorPlaythroughAssertLine(t, e, " |-------|------|")

	tutorPlaythroughEditLineAt(t, e, " --> I like watermelons because oranges are refreshing.", len([]rune(" --> I like ")), "wy")
	tutorPlaythroughEditLineAt(t, e, " --> I like watermelons because oranges are refreshing.", len([]rune(" --> I like watermelons because ")), "wR")
	tutorPlaythroughAssertLine(t, e, " --> I like watermelons because watermelons are refreshing.")

	tutorPlaythroughEditLineAt(t, e, " --> This sentence", 0, "4xJ")
	tutorPlaythroughAssertLine(t, e, " --> This sentence is spilling over onto other lines.")

	tutorPlaythroughEditLineAt(t, e, "    are indented", 0, ">")
	tutorPlaythroughAssertLine(t, e, "\t    are indented")
	tutorPlaythroughEditLineAt(t, e, "         very poorly.", 0, "<lt>")
	tutorPlaythroughAssertLine(t, e, "     very poorly.")

	tutorPlaythroughEditLineAt(t, e, " --> 2) Added point.", len([]rune(" --> ")), "<ctrl+a>")
	tutorPlaythroughAssertLine(t, e, " --> 3) Added point.")
	tutorPlaythroughEditLineAt(t, e, " --> 3) Another point.", len([]rune(" --> ")), "<ctrl+a>")
	tutorPlaythroughAssertLine(t, e, " --> 4) Another point.")
	tutorPlaythroughEditLineAt(t, e, " --> 6) Last point.", len([]rune(" --> ")), "<ctrl+x>")
	tutorPlaythroughAssertLine(t, e, " --> 5) Last point.")

	tutorPlaythroughEditLineAt(t, e, " --> Everybody wants to be a bat,", len([]rune(" --> Everybody wants to be a ")), "e*vnccat<esc>")
	tutorPlaythroughAssertLine(t, e, " --> Everybody wants to be a cat,")
	tutorPlaythroughAssertLine(t, e, " --> because a cat's the only cat")

	tutorPlaythroughEditLineAt(t, e, " --> How much would would a wouldchuck chuck", 0, "2xswould<enter>)<alt+,>cwood<esc>")
	tutorPlaythroughAssertLine(t, e, " --> How much wood would a woodchuck chuck")
	tutorPlaythroughAssertLine(t, e, " --> if a woodchuck could chuck wood?")

	tutorPlaythroughEditLineAt(t, e, " --> Jumping through the water,", 0, "2xsthrough|water|know<enter><alt+)>")
	tutorPlaythroughAssertLine(t, e, " --> Jumping know the through,")
	tutorPlaythroughAssertLine(t, e, " --> daring to water.")

	tutorPlaythroughEditLineAt(t, e, " --> thIs sENtencE hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> ")), "~")
	tutorPlaythroughEditLineAt(t, e, " --> ThIs sENtencE hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> Th")), "~")
	tutorPlaythroughEditLineAt(t, e, " --> This sENtencE hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> This s")), "~")
	tutorPlaythroughEditLineAt(t, e, " --> This seNtencE hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> This se")), "~")
	tutorPlaythroughEditLineAt(t, e, " --> This sentencE hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> This sentenc")), "~")
	tutorPlaythroughEditLineAt(t, e, " --> This sentence hAs MIS-cApitalIsed leTTerS.", len([]rune(" --> This sentence h")), "~")
	tutorPlaythroughEditAt(t, e, "MIS-cApitalIsed", 0, "W`")
	tutorPlaythroughEditAt(t, e, "leTTerS", 0, "e`")
	tutorPlaythroughAssertLine(t, e, " --> This sentence has mis-capitalised letters.")
	tutorPlaythroughEditLineAt(t, e, " --> this SENTENCE SHOULD all be in LOWERCASE.", 0, "x`")
	tutorPlaythroughAssertLine(t, e, " --> this sentence should all be in lowercase.")
	tutorPlaythroughEditLineAt(t, e, " --> THIS sentence should ALL BE IN uppercase!", 0, "x<alt+`>")
	tutorPlaythroughAssertLine(t, e, " --> THIS SENTENCE SHOULD ALL BE IN UPPERCASE!")

	tutorPlaythroughGotoLineExact(t, e, "these are sentences. some sentences don't start with uppercase", 1)
	pressKeyScript(t, e, "2xS\\. |! <enter><alt+;>;<alt+`>")
	tutorPlaythroughAssertLine(t, e, "These are sentences. Some sentences don't start with uppercase")
	tutorPlaythroughAssertLine(t, e, "letters! That is not good grammar. You can fix this.")

	tutorPlaythroughEditLineAt(t, e, " --> Comment me please", 0, "<ctrl+c>")
	tutorPlaythroughAssertLine(t, e, " // --> Comment me please")
	tutorPlaythroughEditLineAt(t, e, " // --> Comment me please", 0, "<ctrl+c>")
	tutorPlaythroughAssertLine(t, e, " --> Comment me please")

	tutorPlaythroughEditLineAt(t, e, " --> What are you doing?!", 0, "4x<ctrl+c>")
	tutorPlaythroughAssertLine(t, e, " // --> What are you doing?!")
	tutorPlaythroughAssertLine(t, e, " // --> Enough! Uncomment me now!")
	tutorPlaythroughEditLineAt(t, e, " // --> What are you doing?!", 0, "4x<ctrl+c>")
	tutorPlaythroughAssertLine(t, e, " --> What are you doing?!")
	tutorPlaythroughAssertLine(t, e, " --> Enough! Uncomment me now!")

	tutorPlaythroughEditLineAt(t, e, " --> so, select all of this, and surround it with ()", len([]rune(" --> so, ")), "v4ems(")
	tutorPlaythroughAssertLine(t, e, " --> so, (select all of this), and surround it with ()")
	tutorPlaythroughEditLineAt(t, e, " --> delete (the x pair of parentheses) from within!", len([]rune(" --> delete (the ")), "md(")
	tutorPlaythroughAssertLine(t, e, " --> delete the x pair of parentheses from within!")
	tutorPlaythroughEditLineAt(t, e, " --> replace the (pair from x within), with something else", len([]rune(" --> replace the (pair from ")), "mr([")
	tutorPlaythroughAssertLine(t, e, " --> replace the [pair from x within], with something else")

	assertHelixTutorDocumentedExpectations(t, e, expects)
	tutorPlaythroughHelixWindowChapter(t, e)
	if e.BehaviorProfile() != BehaviorProfileHelix {
		t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileHelix)
	}
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want %v", e.mode, ModeNormal)
	}
	_ = lines // reference lines for helixExpectedLine assertions above
}
