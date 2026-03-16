package editor

import (
	"strings"
	"testing"
	"time"
)

func TestGotoGitChangeUsesActualCursorRowInDiffPreview(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"index 1111111..2222222 100644",
		"--- a/a.go",
		"+++ b/a.go",
		"@@ -10,2 +10,2 @@",
		"-old10",
		"-old11",
		"+new10",
		"+new11",
		"@@ -12 +12 @@",
		"-old12",
		"+new12",
	}, "\n")

	content := strings.Join([]string{
		"l01", "l02", "l03", "l04", "l05", "l06", "l07", "l08", "l09",
		"new10", "new11", "new12", "tail",
	}, "\n")
	preview, ok := buildGitDiffPreview("/repo/a.go", patch, content, false)
	if !ok {
		t.Fatalf("buildGitDiffPreview returned ok=false")
	}

	e := newTestEditor(
		"l01", "l02", "l03", "l04", "l05", "l06", "l07", "l08", "l09",
		"new10", "new11", "new12", "tail",
	)
	e.document.filename = "/repo/a.go"
	e.mode = ModeMerge
	e.git.changeHunks = []GitChangeHunk{
		{Path: "a.go", AbsPath: "/repo/a.go", StartLine: 9, EndLine: 10},
		{Path: "a.go", AbsPath: "/repo/a.go", StartLine: 11, EndLine: 11},
	}
	e.git.changesVersion = 1
	preview.path = "/repo/a.go"
	preview.changesVersion = e.git.changesVersion
	preview.active = true
	e.git.diffPreview = preview
	e.git.changesUpdated = time.Now()

	currentRow := -1
	for i, line := range e.git.diffPreview.lines {
		if line.sign == '+' && string(line.text) == "new10" {
			currentRow = i
			break
		}
	}
	if currentRow < 0 {
		t.Fatalf("preview row for new10 not found")
	}
	e.cursor.Row = currentRow

	e.gotoGitChange(true)

	if e.git.diffHighlight == nil {
		t.Fatalf("diffHighlight = nil, want second hunk")
	}
	if e.git.diffHighlight.StartLine != 11 || e.git.diffHighlight.EndLine != 11 {
		t.Fatalf("diffHighlight = (%d,%d), want (11,11)", e.git.diffHighlight.StartLine, e.git.diffHighlight.EndLine)
	}
	line, ok := e.gitDiffPreviewLine(e.cursor.Row)
	if !ok {
		t.Fatalf("no preview line at cursor row %d", e.cursor.Row)
	}
	if line.sign != '+' || string(line.text) != "new12" {
		t.Fatalf("cursor line = (%q,%q), want ('+','new12')", string(line.sign), string(line.text))
	}
}

func TestGotoGitChangeNavigatesDeleteAndAddSectionsSeparately(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"index 1111111..2222222 100644",
		"--- a/a.go",
		"+++ b/a.go",
		"@@ -2,2 +2,2 @@",
		"-old",
		"-older",
		"+new",
		"+newer",
	}, "\n")

	content := strings.Join([]string{"keep", "new", "newer", "tail"}, "\n")
	preview, ok := buildGitDiffPreview("/repo/a.go", patch, content, false)
	if !ok {
		t.Fatalf("buildGitDiffPreview returned ok=false")
	}

	e := newTestEditor("keep", "new", "newer", "tail")
	e.document.filename = "/repo/a.go"
	e.mode = ModeMerge
	e.git.changeHunks = []GitChangeHunk{
		{Path: "a.go", AbsPath: "/repo/a.go", StartLine: 1, EndLine: 1, Sign: '-'},
		{Path: "a.go", AbsPath: "/repo/a.go", StartLine: 1, EndLine: 2, Sign: '+'},
	}
	e.git.changesVersion = 1
	preview.path = "/repo/a.go"
	preview.changesVersion = e.git.changesVersion
	preview.active = true
	e.git.diffPreview = preview
	e.git.changesUpdated = time.Now()
	e.git.diffHighlight = &GitChangeHunk{Path: "a.go", AbsPath: "/repo/a.go", StartLine: 1, EndLine: 1, Sign: '-'}

	deleteRow := -1
	for i, line := range e.git.diffPreview.lines {
		if line.sign == '-' {
			deleteRow = i
			break
		}
	}
	if deleteRow < 0 {
		t.Fatalf("delete preview row not found")
	}
	e.cursor.Row = deleteRow

	e.gotoGitChange(true)

	if e.git.diffHighlight == nil {
		t.Fatalf("diffHighlight = nil, want add section")
	}
	if e.git.diffHighlight.Sign != '+' || e.git.diffHighlight.StartLine != 1 || e.git.diffHighlight.EndLine != 2 {
		t.Fatalf("diffHighlight = (%q,%d,%d), want ('+',1,2)", string(e.git.diffHighlight.Sign), e.git.diffHighlight.StartLine, e.git.diffHighlight.EndLine)
	}
	line, ok := e.gitDiffPreviewLine(e.cursor.Row)
	if !ok {
		t.Fatalf("no preview line at cursor row %d", e.cursor.Row)
	}
	if line.sign != '+' || string(line.text) != "new" {
		t.Fatalf("cursor line = (%q,%q), want ('+','new')", string(line.sign), string(line.text))
	}

	e.gotoGitChange(false)

	if e.git.diffHighlight == nil {
		t.Fatalf("diffHighlight = nil after backward navigation")
	}
	if e.git.diffHighlight.Sign != '-' || e.git.diffHighlight.StartLine != 1 || e.git.diffHighlight.EndLine != 1 {
		t.Fatalf("diffHighlight = (%q,%d,%d), want ('-',1,1)", string(e.git.diffHighlight.Sign), e.git.diffHighlight.StartLine, e.git.diffHighlight.EndLine)
	}
	line, ok = e.gitDiffPreviewLine(e.cursor.Row)
	if !ok {
		t.Fatalf("no preview line at cursor row %d after backward navigation", e.cursor.Row)
	}
	if line.sign != '-' || string(line.text) != "old" {
		t.Fatalf("cursor line = (%q,%q), want ('-','old')", string(line.sign), string(line.text))
	}
}
