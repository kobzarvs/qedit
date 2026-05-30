package editor

import (
	"strings"
	"testing"
)

type profileCommandExercise struct {
	profile string
	command string
	lines   []string
	cursor  Cursor
	keys    string
}

// profileCommandExercises lists minimal key scripts that must change editor state
// for every advertised vim/helix profile binding.
var profileCommandExercises = []profileCommandExercise{
	// --- vim ---
	{BehaviorProfileVim, "0", []string{"  abc"}, Cursor{Row: 0, Col: 5}, "0"},
	{BehaviorProfileVim, "^", []string{"  abc"}, Cursor{Row: 0, Col: 5}, "^"},
	{BehaviorProfileVim, "$", []string{"abc"}, Cursor{Row: 0, Col: 0}, "$"},
	{BehaviorProfileVim, "%", []string{"a(b)c"}, Cursor{Row: 0, Col: 1}, "%"},
	{BehaviorProfileVim, "(", []string{"One. Two."}, Cursor{Row: 0, Col: len([]rune("One. Two"))}, "("},
	{BehaviorProfileVim, ")", []string{"One. Two."}, Cursor{Row: 0, Col: 0}, ")"},
	{BehaviorProfileVim, "\"", []string{"abc"}, Cursor{Row: 0, Col: 0}, "\""},
	{BehaviorProfileVim, "\"_dd", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, `yyj"_dd`},
	{BehaviorProfileVim, "\"ap", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, `"ayyjdd"ap`},
	{BehaviorProfileVim, "\"ayy", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, `"ayy`},
	{BehaviorProfileVim, "@", []string{"one"}, Cursor{Row: 0, Col: 0}, `qa0iX<esc>jq@a`},
	{BehaviorProfileVim, "@@", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, `qa0iX<esc>jq@a@@`},
	{BehaviorProfileVim, "<<", []string{"  one"}, Cursor{Row: 0, Col: 0}, "><lt><lt>"},
	{BehaviorProfileVim, ">>", []string{"one"}, Cursor{Row: 0, Col: 0}, ">>"},
	{BehaviorProfileVim, "A", []string{"abc"}, Cursor{Row: 0, Col: 0}, "A!<esc>"},
	{BehaviorProfileVim, "B", []string{"one.two three"}, Cursor{Row: 0, Col: len([]rune("one.two three"))}, "B"},
	{BehaviorProfileVim, "C", []string{"abc def"}, Cursor{Row: 0, Col: 4}, "Cxy<esc>"},
	{BehaviorProfileVim, "D", []string{"abc def"}, Cursor{Row: 0, Col: 4}, "D"},
	{BehaviorProfileVim, "E", []string{"one.two three"}, Cursor{Row: 0, Col: 0}, "E"},
	{BehaviorProfileVim, "G", []string{"1", "2", "3"}, Cursor{Row: 0, Col: 0}, "G"},
	{BehaviorProfileVim, "I", []string{"  abc"}, Cursor{Row: 0, Col: 5}, "I*<esc>"},
	{BehaviorProfileVim, "J", []string{"hello", "world"}, Cursor{Row: 0, Col: 0}, "J"},
	{BehaviorProfileVim, "O", []string{"one", "three"}, Cursor{Row: 1, Col: 0}, "OTWO<esc>"},
	{BehaviorProfileVim, "P", []string{"abc"}, Cursor{Row: 0, Col: 0}, "yypP"},
	{BehaviorProfileVim, "R", []string{"abcdef"}, Cursor{Row: 0, Col: 1}, "RXY<esc>"},
	{BehaviorProfileVim, "S", []string{"one", "two", "three"}, Cursor{Row: 1, Col: 1}, "Schanged<esc>"},
	{BehaviorProfileVim, "U", []string{"abc"}, Cursor{Row: 0, Col: 1}, "lxU"},
	{BehaviorProfileVim, "V", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "V"},
	{BehaviorProfileVim, "W", []string{"one.two three"}, Cursor{Row: 0, Col: 0}, "W"},
	{BehaviorProfileVim, "X", []string{"abc"}, Cursor{Row: 0, Col: 2}, "X"},
	{BehaviorProfileVim, "Y", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "Y"},
	{BehaviorProfileVim, "a", []string{"abc"}, Cursor{Row: 0, Col: 0}, "aX<esc>"},
	{BehaviorProfileVim, "b", []string{"abc def"}, Cursor{Row: 0, Col: 4}, "b"},
	{BehaviorProfileVim, "c", []string{"abc"}, Cursor{Row: 0, Col: 0}, "c"},
	{BehaviorProfileVim, "cc", []string{"one", "two", "three"}, Cursor{Row: 1, Col: 1}, "ccchanged<esc>"},
	{BehaviorProfileVim, "ce", []string{"bad word"}, Cursor{Row: 0, Col: 0}, "cegood<esc>"},
	{BehaviorProfileVim, "cw", []string{"bad word"}, Cursor{Row: 0, Col: 0}, "cwgood<esc>"},
	{BehaviorProfileVim, "ctrl+a", []string{"version 2"}, Cursor{Row: 0, Col: 0}, "<ctrl+a>"},
	{BehaviorProfileVim, "ctrl+g", []string{"one", "two"}, Cursor{Row: 1, Col: 2}, "<ctrl+g>"},
	{BehaviorProfileVim, "ctrl+i", numberedLines(4), Cursor{Row: 0, Col: 0}, "G<ctrl+o><ctrl+i>"},
	{BehaviorProfileVim, "ctrl+r", []string{"abc"}, Cursor{Row: 0, Col: 1}, "xu<ctrl+r>"},
	{BehaviorProfileVim, "ctrl+o", numberedLines(4), Cursor{Row: 0, Col: 0}, "jG<ctrl+o>"},
	{BehaviorProfileVim, "ctrl+s", []string{"one"}, Cursor{Row: 0, Col: 0}, "j<ctrl+s>G<ctrl+o>"},
	{BehaviorProfileVim, "ctrl+w", []string{"one"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>"},
	{BehaviorProfileVim, "ctrl+wv", []string{"one"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>v"},
	{BehaviorProfileVim, "ctrl+ww", []string{"one"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>v<ctrl+w><ctrl+w>"},
	{BehaviorProfileVim, "ctrl+x", []string{"version 2"}, Cursor{Row: 0, Col: 0}, "<ctrl+x>"},
	{BehaviorProfileVim, "d", []string{"abc"}, Cursor{Row: 0, Col: 0}, "d"},
	{BehaviorProfileVim, "d$", []string{"one two"}, Cursor{Row: 0, Col: 4}, "d$"},
	{BehaviorProfileVim, "d0", []string{"  alpha beta"}, Cursor{Row: 0, Col: len([]rune("  alpha"))}, "d0"},
	{BehaviorProfileVim, "d10j", numberedLines(12), Cursor{Row: 0, Col: 0}, "d10j"},
	{BehaviorProfileVim, "dG", numberedLines(5), Cursor{Row: 1, Col: 0}, "dG"},
	{BehaviorProfileVim, "d^", []string{"  alpha beta"}, Cursor{Row: 0, Col: len([]rune("  alpha"))}, "d^"},
	{BehaviorProfileVim, "d)", []string{"One. Two."}, Cursor{Row: 0, Col: 0}, "d)"},
	{BehaviorProfileVim, "dd", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "dd"},
	{BehaviorProfileVim, "d}", []string{"one", "two", "", "three"}, Cursor{Row: 0, Col: 0}, "d}"},
	{BehaviorProfileVim, "dgg", numberedLines(5), Cursor{Row: 2, Col: 0}, "dgg"},
	{BehaviorProfileVim, "dw", []string{"one two"}, Cursor{Row: 0, Col: 0}, "dw"},
	{BehaviorProfileVim, "e", []string{"abc def"}, Cursor{Row: 0, Col: 0}, "e"},
	{BehaviorProfileVim, "F", []string{"abcabc"}, Cursor{Row: 0, Col: len([]rune("abcabc"))}, "Fbx"},
	{BehaviorProfileVim, "f", []string{"abcabc"}, Cursor{Row: 0, Col: 0}, "fc"},
	{BehaviorProfileVim, "gE", []string{"one.two three"}, Cursor{Row: 0, Col: len([]rune("one.two "))}, "gE"},
	{BehaviorProfileVim, "gg", numberedLines(5), Cursor{Row: 0, Col: 0}, "3gg"},
	{BehaviorProfileVim, "ge", []string{"one two"}, Cursor{Row: 0, Col: len([]rune("one "))}, "ge"},
	{BehaviorProfileVim, "gU", []string{"abc def"}, Cursor{Row: 0, Col: 0}, "gUe"},
	{BehaviorProfileVim, "gUe", []string{"abc def"}, Cursor{Row: 0, Col: 0}, "gUe"},
	{BehaviorProfileVim, "gUiw", []string{"alpha beta gamma"}, Cursor{Row: 0, Col: len([]rune("alpha b"))}, "gUiw"},
	{BehaviorProfileVim, "gu", []string{"ABC DEF"}, Cursor{Row: 0, Col: 0}, "guw"},
	{BehaviorProfileVim, "guw", []string{"ABC DEF"}, Cursor{Row: 0, Col: 0}, "guw"},
	{BehaviorProfileVim, "g~", []string{"aBc", "dEf"}, Cursor{Row: 0, Col: 0}, "g~~"},
	{BehaviorProfileVim, "g~~", []string{"aBc", "dEf"}, Cursor{Row: 0, Col: 0}, "g~~"},
	{BehaviorProfileVim, "h", []string{"abc"}, Cursor{Row: 0, Col: 2}, "h"},
	{BehaviorProfileVim, "i", []string{"abc"}, Cursor{Row: 0, Col: 0}, "iX<esc>"},
	{BehaviorProfileVim, "j", []string{"1", "2"}, Cursor{Row: 0, Col: 0}, "j"},
	{BehaviorProfileVim, "k", []string{"1", "2"}, Cursor{Row: 1, Col: 0}, "k"},
	{BehaviorProfileVim, "l", []string{"abc"}, Cursor{Row: 0, Col: 0}, "l"},
	{BehaviorProfileVim, "m", []string{"abc"}, Cursor{Row: 0, Col: 0}, "m"},
	{BehaviorProfileVim, "'", []string{"one", "  two", "three"}, Cursor{Row: 0, Col: 0}, "majj'a"},
	{BehaviorProfileVim, "`", []string{"abcdef"}, Cursor{Row: 0, Col: 0}, "ma2l$`a"},
	{BehaviorProfileVim, "o", []string{"one"}, Cursor{Row: 0, Col: 0}, "oTWO<esc>"},
	{BehaviorProfileVim, "p", []string{"abc"}, Cursor{Row: 0, Col: 0}, "yyp"},
	{BehaviorProfileVim, "q", []string{"one"}, Cursor{Row: 0, Col: 0}, "q"},
	{BehaviorProfileVim, "r", []string{"abc"}, Cursor{Row: 0, Col: 1}, "rZ"},
	{BehaviorProfileVim, "s", []string{"abc"}, Cursor{Row: 0, Col: 1}, "sZ<esc>"},
	{BehaviorProfileVim, "T", []string{"abcabc"}, Cursor{Row: 0, Col: len([]rune("abcabc"))}, "Tbx"},
	{BehaviorProfileVim, "t", []string{"abcabc"}, Cursor{Row: 0, Col: 0}, "tcx"},
	{BehaviorProfileVim, "u", []string{"abc"}, Cursor{Row: 0, Col: 1}, "xu"},
	{BehaviorProfileVim, "v", []string{"abc"}, Cursor{Row: 0, Col: 0}, "v"},
	{BehaviorProfileVim, "Vd", []string{"one", "two", "three"}, Cursor{Row: 0, Col: 0}, "Vjd"},
	{BehaviorProfileVim, "Vy", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "Vyj"},
	{BehaviorProfileVim, "w", []string{"one two"}, Cursor{Row: 0, Col: 0}, "w"},
	{BehaviorProfileVim, "x", []string{"abc"}, Cursor{Row: 0, Col: 0}, "x"},
	{BehaviorProfileVim, "y", []string{"abc"}, Cursor{Row: 0, Col: 0}, "y"},
	{BehaviorProfileVim, "yy", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "yy"},
	{BehaviorProfileVim, "{", []string{"one", "two", "", "three"}, Cursor{Row: 3, Col: 0}, "{"},
	{BehaviorProfileVim, "}", []string{"one", "two", "", "three"}, Cursor{Row: 0, Col: 0}, "}"},
	{BehaviorProfileVim, "~", []string{"aBc"}, Cursor{Row: 0, Col: 0}, "3~"},
	{BehaviorProfileVim, ":!", []string{"top"}, Cursor{Row: 0, Col: 0}, ":!printf ok<enter>"},
	{BehaviorProfileVim, ":profile", []string{""}, Cursor{Row: 0, Col: 0}, ":profile helix<enter>"},
	{BehaviorProfileVim, ":r", []string{"top"}, Cursor{Row: 0, Col: 0}, ":r read.txt<enter>"},
	{BehaviorProfileVim, ":s", []string{"foo foo"}, Cursor{Row: 0, Col: 0}, ":s/foo/bar/g<enter>"},
	{BehaviorProfileVim, ":tutor", []string{"draft"}, Cursor{Row: 0, Col: 0}, ":tutor vim<enter>"},
	{BehaviorProfileVim, ":'<,'>w", []string{"one", "two", "three"}, Cursor{Row: 0, Col: 0}, "Vj:w out.txt<enter>"},

	// --- helix ---
	{BehaviorProfileHelix, "A", []string{"abc"}, Cursor{Row: 0, Col: 0}, "A!<esc>"},
	{BehaviorProfileHelix, "B", []string{"one.two three"}, Cursor{Row: 0, Col: len([]rune("one.two three"))}, "B"},
	{BehaviorProfileHelix, "%", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "%d"},
	{BehaviorProfileHelix, "&", []string{"a = 1", "long = 2"}, Cursor{Row: 0, Col: 0}, "%s=<enter>&"},
	{BehaviorProfileHelix, "*", []string{"bat cat bat"}, Cursor{Row: 0, Col: 0}, "e*v"},
	{BehaviorProfileHelix, "(", []string{"one two three"}, Cursor{Row: 0, Col: 0}, "%sone|two|three<enter><alt+(>"},
	{BehaviorProfileHelix, ")", []string{"one two three"}, Cursor{Row: 0, Col: 0}, "%sone|two|three<enter><alt+)>"},
	{BehaviorProfileHelix, ",", []string{"one two one"}, Cursor{Row: 0, Col: 0}, "%sone<enter>,d"},
	{BehaviorProfileHelix, ";", []string{"one two"}, Cursor{Row: 0, Col: 0}, "w;"},
	{BehaviorProfileHelix, "<", []string{"one"}, Cursor{Row: 0, Col: 0}, "x><lt>"},
	{BehaviorProfileHelix, "E", []string{"bad.word next"}, Cursor{Row: 0, Col: 0}, "Ecgood<esc>"},
	{BehaviorProfileHelix, "G", []string{"1", "2", "3"}, Cursor{Row: 0, Col: 0}, "G"},
	{BehaviorProfileHelix, "I", []string{"  abc"}, Cursor{Row: 0, Col: 5}, "I*<esc>"},
	{BehaviorProfileHelix, "J", []string{"hello", "world"}, Cursor{Row: 0, Col: 0}, "J"},
	{BehaviorProfileHelix, "O", []string{"one", "three"}, Cursor{Row: 1, Col: 0}, "OTWO<esc>"},
	{BehaviorProfileHelix, "P", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "xyjP"},
	{BehaviorProfileHelix, "R", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "xyjR"},
	{BehaviorProfileHelix, ">", []string{"one"}, Cursor{Row: 0, Col: 0}, "x>"},
	{BehaviorProfileHelix, "U", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "duU"},
	{BehaviorProfileHelix, "W", []string{"one.two three"}, Cursor{Row: 0, Col: 0}, "Wd"},
	{BehaviorProfileHelix, "a", []string{"abc"}, Cursor{Row: 0, Col: 0}, "aX<esc>"},
	{BehaviorProfileHelix, "alt+(", []string{"one two three"}, Cursor{Row: 0, Col: 0}, "%sone|two|three<enter><alt+(>"},
	{BehaviorProfileHelix, "alt+)", []string{"one two three"}, Cursor{Row: 0, Col: 0}, "%sone|two|three<enter><alt+)>"},
	{BehaviorProfileHelix, "alt+,", []string{"one two one"}, Cursor{Row: 0, Col: 0}, "%sone<enter><alt+,>d"},
	{BehaviorProfileHelix, "alt+;", []string{"one. two. three"}, Cursor{Row: 0, Col: 0}, "%S\\. <enter><alt+;>;iX<esc>"},
	{BehaviorProfileHelix, "alt+`", []string{"aBc"}, Cursor{Row: 0, Col: 0}, "x<alt+`>"},
	{BehaviorProfileHelix, "alt+c", []string{"aa", "aa"}, Cursor{Row: 1, Col: 0}, "<alt+c>iX<esc>"},
	{BehaviorProfileHelix, "alt+i", []string{"abc"}, Cursor{Row: 0, Col: 0}, "x<alt+i>"},
	{BehaviorProfileHelix, "alt+o", []string{"abc"}, Cursor{Row: 0, Col: 0}, "x<alt+o>"},
	{BehaviorProfileHelix, "alt+s", []string{"one", "two", "three"}, Cursor{Row: 0, Col: 0}, "2x<alt+s>d"},
	{BehaviorProfileHelix, "b", []string{"abc def"}, Cursor{Row: 0, Col: 4}, "b"},
	{BehaviorProfileHelix, "C", []string{"aa", "aa"}, Cursor{Row: 0, Col: 0}, "CiX<esc>"},
	{BehaviorProfileHelix, "c", []string{"abc"}, Cursor{Row: 0, Col: 0}, "c"},
	{BehaviorProfileHelix, "ctrl+a", []string{"version 2"}, Cursor{Row: 0, Col: 0}, "<ctrl+a>"},
	{BehaviorProfileHelix, "ctrl+c", []string{"line"}, Cursor{Row: 0, Col: 0}, "<ctrl+c>"},
	{BehaviorProfileHelix, "ctrl+i", numberedLines(4), Cursor{Row: 0, Col: 0}, "G<ctrl+o><ctrl+i>"},
	{BehaviorProfileHelix, "ctrl+o", numberedLines(4), Cursor{Row: 0, Col: 0}, "jG<ctrl+o>"},
	{BehaviorProfileHelix, "ctrl+s", numberedLines(4), Cursor{Row: 0, Col: 0}, "j<ctrl+s>G<ctrl+o>"},
	{BehaviorProfileHelix, "ctrl+w", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>"},
	{BehaviorProfileHelix, "ctrl+wH", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>H"},
	{BehaviorProfileHelix, "ctrl+wJ", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>J"},
	{BehaviorProfileHelix, "ctrl+wK", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>K"},
	{BehaviorProfileHelix, "ctrl+wL", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>L"},
	{BehaviorProfileHelix, "ctrl+wh", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>h"},
	{BehaviorProfileHelix, "ctrl+wj", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>j"},
	{BehaviorProfileHelix, "ctrl+wk", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>k"},
	{BehaviorProfileHelix, "ctrl+wl", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>l"},
	{BehaviorProfileHelix, "ctrl+wns", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>ns"},
	{BehaviorProfileHelix, "ctrl+wnv", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>nv"},
	{BehaviorProfileHelix, "ctrl+wo", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>v<ctrl+w>o"},
	{BehaviorProfileHelix, "ctrl+wq", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>v<ctrl+w>q"},
	{BehaviorProfileHelix, "ctrl+ws", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>s"},
	{BehaviorProfileHelix, "ctrl+wt", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>v<ctrl+w>t"},
	{BehaviorProfileHelix, "ctrl+wv", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>v"},
	{BehaviorProfileHelix, "ctrl+ww", []string{"tutor"}, Cursor{Row: 0, Col: 0}, "<ctrl+w>v<ctrl+w><ctrl+w>"},
	{BehaviorProfileHelix, "ctrl+x", []string{"version 2"}, Cursor{Row: 0, Col: 0}, "<ctrl+x>"},
	{BehaviorProfileHelix, "d", []string{"abc"}, Cursor{Row: 0, Col: 1}, "d"},
	{BehaviorProfileHelix, "e", []string{"abc def"}, Cursor{Row: 0, Col: 0}, "e"},
	{BehaviorProfileHelix, "F", []string{"abcabc"}, Cursor{Row: 0, Col: len([]rune("abcabc"))}, "glFbd"},
	{BehaviorProfileHelix, "f", []string{"abcabc"}, Cursor{Row: 0, Col: 0}, "fcd"},
	{BehaviorProfileHelix, "g", []string{"abc"}, Cursor{Row: 0, Col: 0}, "g"},
	{BehaviorProfileHelix, "ge", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "ge"},
	{BehaviorProfileHelix, "gg", numberedLines(4), Cursor{Row: 2, Col: 1}, "gg"},
	{BehaviorProfileHelix, "gh", []string{"  abc"}, Cursor{Row: 0, Col: 3}, "gh"},
	{BehaviorProfileHelix, "gl", []string{"abc"}, Cursor{Row: 0, Col: 0}, "gl"},
	{BehaviorProfileHelix, "h", []string{"abc"}, Cursor{Row: 0, Col: 2}, "h"},
	{BehaviorProfileHelix, "i", []string{"abc"}, Cursor{Row: 0, Col: 1}, "iX<esc>"},
	{BehaviorProfileHelix, "j", []string{"1", "2"}, Cursor{Row: 0, Col: 0}, "j"},
	{BehaviorProfileHelix, "k", []string{"1", "2"}, Cursor{Row: 1, Col: 0}, "k"},
	{BehaviorProfileHelix, "l", []string{"abc"}, Cursor{Row: 0, Col: 0}, "l"},
	{BehaviorProfileHelix, "m", []string{"abc"}, Cursor{Row: 0, Col: 0}, "m"},
	{BehaviorProfileHelix, "ma", []string{"a(b)c"}, Cursor{Row: 0, Col: 2}, "ma"},
	{BehaviorProfileHelix, "md", []string{"a(b)c"}, Cursor{Row: 0, Col: 2}, "md("},
	{BehaviorProfileHelix, "mi", []string{"a(b)c"}, Cursor{Row: 0, Col: 2}, "mi(d"},
	{BehaviorProfileHelix, "mr", []string{"a(b)c"}, Cursor{Row: 0, Col: 2}, "mr(["},
	{BehaviorProfileHelix, "ms", []string{"abc"}, Cursor{Row: 0, Col: 1}, "vlms("},
	{BehaviorProfileHelix, "o", []string{"one"}, Cursor{Row: 0, Col: 0}, "oTWO<esc>"},
	{BehaviorProfileHelix, "p", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "xyp"},
	{BehaviorProfileHelix, "r", []string{"abc"}, Cursor{Row: 0, Col: 1}, "rZ"},
	{BehaviorProfileHelix, "S", []string{"one", "two", "three"}, Cursor{Row: 0, Col: 0}, "%S<enter>~"},
	{BehaviorProfileHelix, "s", []string{"I like apples and apples"}, Cursor{Row: 0, Col: 0}, "%sapples<enter>"},
	{BehaviorProfileHelix, "T", []string{"abcabc"}, Cursor{Row: 0, Col: len([]rune("abcabc"))}, "glTbd"},
	{BehaviorProfileHelix, "t", []string{"abcabc"}, Cursor{Row: 0, Col: 0}, "tcd"},
	{BehaviorProfileHelix, "u", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "duU"},
	{BehaviorProfileHelix, "v", []string{"abc"}, Cursor{Row: 0, Col: 0}, "v"},
	{BehaviorProfileHelix, "w", []string{"one two"}, Cursor{Row: 0, Col: 0}, "w"},
	{BehaviorProfileHelix, "x", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "x"},
	{BehaviorProfileHelix, "y", []string{"one", "two"}, Cursor{Row: 0, Col: 0}, "xyp"},
	{BehaviorProfileHelix, "`", []string{"AbC"}, Cursor{Row: 0, Col: 0}, "x`"},
	{BehaviorProfileHelix, "~", []string{"aBc"}, Cursor{Row: 0, Col: 0}, "x~"},
	{BehaviorProfileHelix, "2w", []string{"one two three"}, Cursor{Row: 0, Col: 0}, "2wd"},
	{BehaviorProfileHelix, "2x", []string{"one", "two", "three"}, Cursor{Row: 0, Col: 0}, "2xd"},
	{BehaviorProfileHelix, ":hs", []string{"tutor"}, Cursor{Row: 0, Col: 0}, ":hs hello2<enter>"},
	{BehaviorProfileHelix, ":vs", []string{"tutor"}, Cursor{Row: 0, Col: 0}, ":vs hello1<enter>"},
}

func TestAdvertisedProfileCommandsHaveConformanceCoverage(t *testing.T) {
	assertSameStringSet(t, "vim", advertisedVimProfileCommands, profileCommandExercisedNames(BehaviorProfileVim))
	assertSameStringSet(t, "helix", advertisedHelixProfileCommands, profileCommandExercisedNames(BehaviorProfileHelix))
}

func TestProfileCommandsRespondToKeySimulation(t *testing.T) {
	for _, ex := range profileCommandExercises {
		ex := ex
		t.Run(ex.profile+"/"+ex.command, func(t *testing.T) {
			e := newSimulatedProfileEditor(ex.profile, ex.lines...)
			if ex.profile == BehaviorProfileVim && ex.command == ":r" {
				store := &testFileStore{
					absPaths: map[string]string{"read.txt": "/tmp/read.txt"},
					readDataByPath: map[string][]byte{
						"/tmp/read.txt": []byte("alpha\nbeta\n"),
					},
				}
				e.SetFileStore(store)
			}
			if ex.profile == BehaviorProfileVim && ex.command == ":'<,'>w" {
				store := &testFileStore{absPaths: map[string]string{"out.txt": "/tmp/out.txt"}}
				e.SetFileStore(store)
			}
			e.cursor = ex.cursor
			before := profileKeySimulationFingerprint(e)
			pressKeyScript(t, e, ex.keys)
			after := profileKeySimulationFingerprint(e)
			if before == after {
				t.Fatalf("command %q keys %q did not change editor state", ex.command, ex.keys)
			}
		})
	}
}

func TestProfileCommandExercisesUseUniqueNames(t *testing.T) {
	seen := make(map[string]struct{}, len(profileCommandExercises))
	for _, ex := range profileCommandExercises {
		key := ex.profile + "\x00" + ex.command
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate exercise for %s %q", ex.profile, ex.command)
		}
		seen[key] = struct{}{}
		if strings.TrimSpace(ex.keys) == "" {
			t.Fatalf("empty key script for %s %q", ex.profile, ex.command)
		}
	}
}
