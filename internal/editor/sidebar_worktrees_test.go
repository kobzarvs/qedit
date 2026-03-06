package editor

import "testing"

func TestSidebarWorktreesUsesRuntimeFileStorePaths(t *testing.T) {
	store := &testFileStore{
		absPaths: map[string]string{
			"main":    "/repo/main",
			"feature": "/repo/feature",
		},
	}

	content := NewSidebarWorktreesContent(store, []WorktreeInfo{
		{Path: "main", Branch: "main"},
		{Path: "feature", Branch: "feature"},
	}, "main")

	items := content.Items()
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if !items[0].IsCurrent {
		t.Fatalf("first item current = false, want true")
	}
	if items[1].Label != "feature  feature" {
		t.Fatalf("second item label = %q, want %q", items[1].Label, "feature  feature")
	}
}

func TestWorktreeDisplayPathUsesRuntimeHomeDir(t *testing.T) {
	store := &testFileStore{homeDir: "/home/diver"}

	display := worktreeDisplayPath(store, "/home/diver/prj/qedit", "")

	if display != "~/prj/qedit" {
		t.Fatalf("display path = %q, want %q", display, "~/prj/qedit")
	}
}
