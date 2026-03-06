package plugins

import (
	"testing"

	"github.com/kobzarvs/qedit/internal/editor"
)

func TestProfileSidebarPluginRegistersSidebarMode(t *testing.T) {
	ed := editor.New(editor.Options{})
	if err := NewRegistry(
		NewProfileSidebarPlugin(),
	).Apply(ed); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	ed.OpenSidebarMode(ProfileSidebarMode)

	if ed.CurrentSidebarMode() != ProfileSidebarMode {
		t.Fatalf("sidebar mode = %v, want %v", ed.CurrentSidebarMode(), ProfileSidebarMode)
	}
}
