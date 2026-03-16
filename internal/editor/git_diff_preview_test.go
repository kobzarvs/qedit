package editor

import (
	"strings"
	"testing"
)

type previewTestGitRuntime struct {
	patch string
}

func (r previewTestGitRuntime) Root(path string) string       { return "/repo" }
func (r previewTestGitRuntime) Branch(path string) string     { return "" }
func (r previewTestGitRuntime) MainBranch(path string) string { return "" }
func (r previewTestGitRuntime) ListBranches(root string) ([]string, string, error) {
	return nil, "", nil
}
func (r previewTestGitRuntime) ListWorktrees(root string) ([]WorktreeInfo, string, error) {
	return nil, "", nil
}
func (r previewTestGitRuntime) Checkout(root, branch string) error            { return nil }
func (r previewTestGitRuntime) AddWorktree(root, name string) (string, error) { return "", nil }
func (r previewTestGitRuntime) RemoveWorktree(root, path string) error        { return nil }
func (r previewTestGitRuntime) Changes(root string) ([]GitFileChange, []GitChangeHunk, error) {
	return nil, nil, nil
}
func (r previewTestGitRuntime) Diff(root, path string) (string, error) { return r.patch, nil }

func TestBuildGitDiffPreviewIncludesDeletedLines(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"index 1111111..2222222 100644",
		"--- a/a.go",
		"+++ b/a.go",
		"@@ -1,3 +1,3 @@",
		" keep",
		"-old",
		"+new",
		" tail",
	}, "\n")

	content := strings.Join([]string{"keep", "new", "tail"}, "\n")
	preview, ok := buildGitDiffPreview("/repo/a.go", patch, content, false)
	if !ok {
		t.Fatalf("buildGitDiffPreview returned ok=false")
	}
	if len(preview.lines) != 4 {
		t.Fatalf("preview lines = %d, want 4", len(preview.lines))
	}
	if preview.lines[1].sign != '-' || string(preview.lines[1].text) != "old" {
		t.Fatalf("deleted line = (%q,%q), want ('-','old')", string(preview.lines[1].sign), string(preview.lines[1].text))
	}
	if preview.lines[1].oldLine != 2 || preview.lines[1].newLine != 0 {
		t.Fatalf("deleted line numbers = (%d,%d), want (2,0)", preview.lines[1].oldLine, preview.lines[1].newLine)
	}
	if preview.lines[2].sign != '+' || string(preview.lines[2].text) != "new" {
		t.Fatalf("added line = (%q,%q), want ('+','new')", string(preview.lines[2].sign), string(preview.lines[2].text))
	}
	if preview.lines[2].oldLine != 0 || preview.lines[2].newLine != 2 {
		t.Fatalf("added line numbers = (%d,%d), want (0,2)", preview.lines[2].oldLine, preview.lines[2].newLine)
	}
}

func TestActivateGitDiffPreviewAnchorsDeletionOnlyHunk(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"index 1111111..2222222 100644",
		"--- a/a.go",
		"+++ b/a.go",
		"@@ -2,2 +2,0 @@",
		"-old",
		"-older",
	}, "\n")

	content := strings.Join([]string{"keep", "tail"}, "\n")
	preview, ok := buildGitDiffPreview("/repo/a.go", patch, content, false)
	if !ok {
		t.Fatalf("buildGitDiffPreview returned ok=false")
	}

	e := newTestEditor("keep", "tail")
	e.document.filename = "/repo/a.go"
	e.mode = ModeMerge
	e.git.diffPreview = preview
	e.git.diffHighlight = &GitChangeHunk{AbsPath: "/repo/a.go", StartLine: 1, EndLine: 1, Sign: '-'}

	if !e.activateGitDiffPreview() {
		t.Fatalf("activateGitDiffPreview returned false")
	}
	line, ok := e.gitDiffPreviewLine(e.cursor.Row)
	if !ok {
		t.Fatalf("no preview line at cursor row %d", e.cursor.Row)
	}
	if line.sign != '-' || string(line.text) != "old" {
		t.Fatalf("cursor line = (%q,%q), want ('-','old')", string(line.sign), string(line.text))
	}
}

func TestActivateGitDiffPreviewAnchorsReplacementToFirstAddedLine(t *testing.T) {
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
	e.git.diffPreview = preview
	e.git.diffHighlight = &GitChangeHunk{AbsPath: "/repo/a.go", StartLine: 1, EndLine: 2, Sign: '+'}

	if !e.activateGitDiffPreview() {
		t.Fatalf("activateGitDiffPreview returned false")
	}
	line, ok := e.gitDiffPreviewLine(e.cursor.Row)
	if !ok {
		t.Fatalf("no preview line at cursor row %d", e.cursor.Row)
	}
	if line.sign != '+' || string(line.text) != "new" {
		t.Fatalf("cursor line = (%q,%q), want ('+','new')", string(line.sign), string(line.text))
	}
}

func TestRenderSnapshotGitDiffPreviewShowsDeletedLines(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"index 1111111..2222222 100644",
		"--- a/a.go",
		"+++ b/a.go",
		"@@ -2,3 +2,3 @@",
		" top",
		" keep",
		"-old",
		"+new",
		" tail",
		" bottom",
	}, "\n")

	preview, ok := buildGitDiffPreview("/repo/a.go", patch, "", false)
	if !ok {
		t.Fatalf("buildGitDiffPreview returned ok=false")
	}

	e := newTestEditor("top", "keep", "new", "tail", "bottom")
	e.document.filename = "/repo/a.go"
	e.mode = ModeMerge
	e.git.changeHunks = []GitChangeHunk{{AbsPath: "/repo/a.go", StartLine: 1, EndLine: 3}}
	e.git.diffPreview = preview
	e.git.diffPreview.active = true

	got := renderSnapshot(t, e, 40, 9)

	if !strings.Contains(got, "-old") {
		t.Fatalf("snapshot does not show deleted line:\n%s", got)
	}
	if !strings.Contains(got, "+new") {
		t.Fatalf("snapshot does not show added line:\n%s", got)
	}
	if strings.Contains(got, "@@ -") {
		t.Fatalf("snapshot still shows hunk header:\n%s", got)
	}
	if !strings.Contains(got, "top") || !strings.Contains(got, "bottom") {
		t.Fatalf("snapshot does not show unchanged file context:\n%s", got)
	}
}

func TestEnsureGitDiffPreviewPreservesActiveCursorOnRefresh(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"index 1111111..2222222 100644",
		"--- a/a.go",
		"+++ b/a.go",
		"@@ -2,3 +2,3 @@",
		" top",
		" keep",
		"-old",
		"+new",
		" tail",
		" bottom",
	}, "\n")

	e := newTestEditor("top", "keep", "new", "tail", "bottom")
	e.document.filename = "/repo/a.go"
	e.mode = ModeMerge
	e.SetGitRoot("/repo")
	e.SetGitRuntime(previewTestGitRuntime{patch: patch})
	e.git.changeHunks = []GitChangeHunk{{AbsPath: "/repo/a.go", StartLine: 1, EndLine: 3}}
	e.git.diffHighlight = &GitChangeHunk{AbsPath: "/repo/a.go", StartLine: 1, EndLine: 3}
	e.git.changesVersion = 1

	if !e.activateGitDiffPreview() {
		t.Fatalf("activateGitDiffPreview returned false")
	}
	e.cursor.Row = 4
	beforeActual, beforeCol := e.gitDiffPreviewActualCursor()

	e.git.changesVersion = 2
	if !e.ensureGitDiffPreview() {
		t.Fatalf("ensureGitDiffPreview returned false after refresh")
	}

	afterActual, afterCol := e.gitDiffPreviewActualCursor()
	if !e.git.diffPreview.active {
		t.Fatalf("diff preview became inactive after refresh")
	}
	if beforeActual != afterActual || beforeCol != afterCol {
		t.Fatalf("cursor actual position changed after refresh: before=(%d,%d) after=(%d,%d)", beforeActual, beforeCol, afterActual, afterCol)
	}
}

func TestEnsureGitDiffPreviewPreservesDeletedLineCursorOnRefresh(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"index 1111111..2222222 100644",
		"--- a/a.go",
		"+++ b/a.go",
		"@@ -1,3 +1,3 @@",
		" keep",
		"-old",
		"+new",
		" tail",
	}, "\n")

	e := newTestEditor("keep", "new", "tail")
	e.document.filename = "/repo/a.go"
	e.mode = ModeMerge
	e.SetGitRoot("/repo")
	e.SetGitRuntime(previewTestGitRuntime{patch: patch})
	e.git.changeHunks = []GitChangeHunk{{AbsPath: "/repo/a.go", StartLine: 0, EndLine: 1}}
	e.git.diffHighlight = &GitChangeHunk{AbsPath: "/repo/a.go", StartLine: 0, EndLine: 2}
	e.git.changesVersion = 1

	if !e.activateGitDiffPreview() {
		t.Fatalf("activateGitDiffPreview returned false")
	}
	deletedRow := -1
	for i, line := range e.git.diffPreview.lines {
		if line.sign == '-' && string(line.text) == "old" {
			deletedRow = i
			break
		}
	}
	if deletedRow < 0 {
		t.Fatalf("deleted preview row not found")
	}
	e.cursor.Row = deletedRow
	line, ok := e.gitDiffPreviewLine(e.cursor.Row)
	if !ok {
		t.Fatalf("no preview line at cursor row %d", e.cursor.Row)
	}
	if line.sign != '-' || string(line.text) != "old" {
		t.Fatalf("cursor line before refresh = (%q,%q), want ('-','old')", string(line.sign), string(line.text))
	}

	e.git.changesVersion = 2
	if !e.ensureGitDiffPreview() {
		t.Fatalf("ensureGitDiffPreview returned false after refresh")
	}

	line, ok = e.gitDiffPreviewLine(e.cursor.Row)
	if !ok {
		t.Fatalf("no preview line at cursor row %d after refresh", e.cursor.Row)
	}
	if line.sign != '-' || string(line.text) != "old" {
		t.Fatalf("cursor line after refresh = (%q,%q), want ('-','old')", string(line.sign), string(line.text))
	}
}

func TestEnterMergeModeFallsBackToGitDiffPreview(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"index 1111111..2222222 100644",
		"--- a/a.go",
		"+++ b/a.go",
		"@@ -2,3 +2,3 @@",
		" top",
		" keep",
		"-old",
		"+new",
		" tail",
		" bottom",
	}, "\n")

	e := newTestEditor("top", "keep", "new", "tail", "bottom")
	e.document.filename = "/repo/a.go"
	e.SetGitRoot("/repo")
	e.SetGitRuntime(previewTestGitRuntime{patch: patch})
	e.applyGitChanges(
		[]GitFileChange{{Path: "a.go", AbsPath: "/repo/a.go", Status: "M", Unstaged: true}},
		[]GitChangeHunk{{AbsPath: "/repo/a.go", StartLine: 1, EndLine: 3}},
	)

	e.enterMergeMode()

	if e.mode != ModeMerge {
		t.Fatalf("mode = %v, want %v", e.mode, ModeMerge)
	}
	if !e.gitDiffPreviewActive() {
		t.Fatalf("expected git diff preview active after enterMergeMode")
	}
}
