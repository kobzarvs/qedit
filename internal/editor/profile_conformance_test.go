package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var advertisedVimProfileCommands = []string{
	"0",
	"^",
	"$",
	"%",
	"(",
	")",
	"\"",
	"\"_dd",
	"\"ap",
	"\"ayy",
	"@",
	"@@",
	"<<",
	">>",
	"A",
	"B",
	"C",
	"D",
	"E",
	"G",
	"I",
	"J",
	"O",
	"P",
	"R",
	"S",
	"U",
	"V",
	"W",
	"X",
	"Y",
	"a",
	"b",
	"c",
	"cc",
	"ce",
	"cw",
	"ctrl+a",
	"ctrl+g",
	"ctrl+i",
	"ctrl+r",
	"ctrl+o",
	"ctrl+s",
	"ctrl+w",
	"ctrl+wv",
	"ctrl+ww",
	"ctrl+x",
	"d",
	"d$",
	"d0",
	"d10j",
	"dG",
	"d^",
	"d)",
	"dd",
	"d}",
	"dgg",
	"dw",
	"e",
	"F",
	"f",
	"gE",
	"gg",
	"ge",
	"gU",
	"gUe",
	"gUiw",
	"gu",
	"guw",
	"g~",
	"g~~",
	"h",
	"i",
	"j",
	"k",
	"l",
	"m",
	"'",
	"`",
	"o",
	"p",
	"q",
	"r",
	"s",
	"T",
	"t",
	"u",
	"v",
	"Vd",
	"Vy",
	"w",
	"x",
	"y",
	"yy",
	"{",
	"}",
	"~",
	":!",
	":profile",
	":r",
	":s",
	":tutor",
	":'<,'>w",
}

var advertisedHelixProfileCommands = []string{
	"A",
	"B",
	"%",
	"&",
	"*",
	"(",
	")",
	",",
	";",
	"<",
	"E",
	"G",
	"I",
	"J",
	"O",
	"P",
	"R",
	">",
	"U",
	"W",
	"a",
	"alt+(",
	"alt+)",
	"alt+,",
	"alt+;",
	"alt+`",
	"alt+c",
	"alt+i",
	"alt+o",
	"alt+s",
	"b",
	"C",
	"c",
	"ctrl+a",
	"ctrl+c",
	"ctrl+i",
	"ctrl+o",
	"ctrl+s",
	"ctrl+w",
	"ctrl+wH",
	"ctrl+wJ",
	"ctrl+wK",
	"ctrl+wL",
	"ctrl+wh",
	"ctrl+wj",
	"ctrl+wk",
	"ctrl+wl",
	"ctrl+wns",
	"ctrl+wnv",
	"ctrl+wo",
	"ctrl+wq",
	"ctrl+ws",
	"ctrl+wt",
	"ctrl+wv",
	"ctrl+ww",
	"ctrl+x",
	"d",
	"e",
	"F",
	"f",
	"g",
	"ge",
	"gg",
	"gh",
	"gl",
	"h",
	"i",
	"j",
	"k",
	"l",
	"m",
	"ma",
	"md",
	"mi",
	"mr",
	"ms",
	"o",
	"p",
	"r",
	"S",
	"s",
	"T",
	"t",
	"u",
	"v",
	"w",
	"x",
	"y",
	"`",
	"~",
	"2w",
	"2x",
	":hs",
	":vs",
}

func TestVimProfileMatchesRealVimForCoreEditingSequences(t *testing.T) {
	vimPath, err := exec.LookPath("vim")
	if err != nil {
		t.Skip("vim binary is not available for oracle comparison")
	}

	tests := []struct {
		name      string
		initial   string
		qeditKeys string
		vimNormal string
	}{
		{"insert before cursor", "one", "liX<esc>", "liX"},
		{"append after cursor", "one", "aX<esc>", "aX"},
		{"append line end", "one", "A!<esc>", "A!"},
		{"insert first nonblank", "  one", "$I*<esc>", "$I*"},
		{"open below", "one", "oTWO<esc>", "oTWO"},
		{"open above", "one\nthree", "jOTWO<esc>", "jOTWO"},
		{"delete char", "abc", "lx", "lx"},
		{"delete previous char", "abc", "llX", "llX"},
		{"replace char", "abc", "lrZ", "lrZ"},
		{"find char forward", "abcabc", "fcx", "fcx"},
		{"till char forward", "abcabc", "tcx", "tcx"},
		{"find char backward", "abcabc", "$Fbx", "$Fbx"},
		{"till char backward", "abcabc", "$Tbx", "$Tbx"},
		{"replace mode", "abcdef", "lRXY<esc>", "lRXY"},
		{"delete to line end command", "abc def", "wD", "wD"},
		{"change to line end command", "abc def", "wCxyz<esc>", "wCxyz"},
		{"substitute one char", "abc", "lsZ<esc>", "lsZ"},
		{"substitute counted chars", "abcdef", "l3sZ<esc>", "l3sZ"},
		{"join lines", "hello\nworld", "J", "J"},
		{"delete word", "one two", "dw", "dw"},
		{"delete WORD", "one.two three", "dW", "dW"},
		{"delete sentence", "One. Two.", "d)", "d)"},
		{"delete paragraph", "one\ntwo\n\nthree", "d}", "d}"},
		{"visual line delete", "one\ntwo\nthree", "Vjd", "Vjd"},
		{"change through word end", "bad word", "cegood<esc>", "cegood"},
		{"change word", "bad word", "cwgood<esc>", "cwgood"},
		{"uppercase through word end", "abc def", "gUe", "gUe"},
		{"lowercase word", "ABC DEF", "guw", "guw"},
		{"toggle line", "aBc\ndEf", "g~~", "g~~"},
		{"delete to line end motion", "one two", "wd$", "wd$"},
		{"delete to line start motion", "  alpha beta", "wed0", "wed0"},
		{"delete to first nonblank motion", "  alpha beta", "wed^", "wed^"},
		{"delete counted motion", strings.Join(numberedLines(12), "\n"), "d10j", "d10j"},
		{"operator and motion count multiply", strings.Join(numberedLines(10), "\n"), "2d3d", "2d3d"},
		{"delete to first line", strings.Join(numberedLines(5), "\n"), "jjdgg", "jjdgg"},
		{"delete to last line", strings.Join(numberedLines(5), "\n"), "jdG", "jdG"},
		{"delete to mark line", "one\ntwo\nthree", "majjd'a", "majjd'a"},
		{"named register paste", "one\ntwo", `"ayyjdd"ap`, `"ayyjdd"ap`},
		{"blackhole delete preserves unnamed", "one\ntwo", `yyj"_ddp`, `yyj"_ddp`},
		{"change whole line", "one\ntwo\nthree", "jccchanged<esc>", "jccchanged"},
		{"linewise yank paste", "one\ntwo", "yyp", "yyp"},
		{"Y yank line paste", "one\ntwo", "Yp", "Yp"},
		{"indent and unindent", "one", ">><lt><lt>", ">><<"},
		{"toggle case", "aBc", "3~", "3~"},
		{"normal undo", "abc", "lxu", "lxu"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newSimulatedProfileEditor(BehaviorProfileVim, splitConformanceText(tt.initial)...)

			pressKeyScript(t, e, tt.qeditKeys)
			want := runVimNormalOracle(t, vimPath, tt.initial, tt.vimNormal)

			if got := e.Content(); got != want {
				t.Fatalf("qedit content = %q, real vim content = %q", got, want)
			}
		})
	}
}

func assertSameStringSet(t *testing.T, label string, want, got []string) {
	t.Helper()
	want = sortedUniqueStrings(want)
	got = sortedUniqueStrings(got)
	if strings.Join(want, "\n") == strings.Join(got, "\n") {
		return
	}
	t.Fatalf("%s conformance coverage mismatch\nmissing: %v\nextra: %v", label, missingStrings(want, got), missingStrings(got, want))
}

func sortedUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func missingStrings(want, got []string) []string {
	gotSet := make(map[string]struct{}, len(got))
	for _, value := range got {
		gotSet[value] = struct{}{}
	}
	var missing []string
	for _, value := range want {
		if _, ok := gotSet[value]; !ok {
			missing = append(missing, value)
		}
	}
	return missing
}

func splitConformanceText(text string) []string {
	return strings.Split(text, "\n")
}

func runVimNormalOracle(t *testing.T, vimPath, initial, normalKeys string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "oracle.txt")
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("write oracle input: %v", err)
	}
	cmd := exec.Command(vimPath,
		"-Nu", "NONE",
		"-N",
		"-n",
		"-es",
		path,
		"-c", "set nomore",
		"-c", "call cursor(1, 1)",
		"-c", "normal! "+normalKeys,
		"-c", "wq!",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run vim oracle for %q: %v\n%s", normalKeys, err, string(out))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read oracle output: %v", err)
	}
	return strings.TrimSuffix(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
}
