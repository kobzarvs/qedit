package editor

import (
	"testing"
	"time"
)

func TestSidebarGitChangesOnEnterPreparesDiffJump(t *testing.T) {
	e := newTestEditor("one", "two")
	e.git.root = "/repo"
	e.git.changeHunks = []GitChangeHunk{
		{Path: "a.go", AbsPath: "/repo/a.go", StartLine: 3, EndLine: 4},
	}
	e.git.changes = []GitFileChange{
		{Path: "a.go", AbsPath: "/repo/a.go"},
	}
	e.git.changesUpdated = time.Now()
	content := NewSidebarGitChangesContent(e)
	items := content.Items()
	for i, item := range items {
		if item.Path == "/repo/a.go" {
			content.SetIndex(i)
			break
		}
	}

	action := content.OnEnter()

	if action.Action != SidebarActionOpenFile {
		t.Fatalf("action = %v, want %v", action.Action, SidebarActionOpenFile)
	}
	if action.Path != "/repo/a.go" {
		t.Fatalf("path = %q, want %q", action.Path, "/repo/a.go")
	}
	if !action.HasLocation || action.Line != 3 || action.Col != 0 {
		t.Fatalf("location = (%v,%d,%d), want (true,3,0)", action.HasLocation, action.Line, action.Col)
	}
	if !e.git.pendingDiffJump {
		t.Fatalf("pendingDiffJump = false, want true")
	}
	if e.git.diffHighlight == nil || e.git.diffHighlight.AbsPath != "/repo/a.go" {
		t.Fatalf("diffHighlight = %+v, want /repo/a.go", e.git.diffHighlight)
	}
}
