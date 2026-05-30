package editor

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVimBufferCommandsThroughFullKeySimulation(t *testing.T) {
	e, paths := newProfileBufferEditor(t, BehaviorProfileVim)

	pressKeyScript(t, e, ":ls<enter>")
	if e.sidebar == nil || !e.sidebar.Visible || e.sidebar.Content == nil ||
		e.sidebar.Content.Mode() != SidebarModeBuffers {
		t.Fatalf("sidebar = %#v visible=%v, want open buffers sidebar after :ls", e.sidebar.Content, e.sidebar.Visible)
	}
	if !strings.Contains(e.ui.statusMessage, "buffers:") ||
		!strings.Contains(e.ui.statusMessage, filepath.Base(paths[0])) ||
		!strings.Contains(e.ui.statusMessage, filepath.Base(paths[2])) {
		t.Fatalf("status = %q, want buffer list with file names", e.ui.statusMessage)
	}

	pressKeyScript(t, e, ":b 1<enter>")
	assertActiveBuffer(t, e, 0, paths[0], "alpha")

	pressKeyScript(t, e, ":b#<enter>")
	assertActiveBuffer(t, e, 2, paths[2], "gamma")

	pressKeyScript(t, e, ":bp<enter>")
	assertActiveBuffer(t, e, 1, paths[1], "beta")

	pressKeyScript(t, e, ":bnext<enter>")
	assertActiveBuffer(t, e, 2, paths[2], "gamma")

	pressKeyScript(t, e, ":buffer alpha<enter>")
	assertActiveBuffer(t, e, 0, paths[0], "alpha")
}

func TestHelixBufferNavigationThroughFullKeySimulation(t *testing.T) {
	e, paths := newProfileBufferEditor(t, BehaviorProfileHelix)

	pressKeyScript(t, e, "gp")
	assertActiveBuffer(t, e, 1, paths[1], "beta")

	pressKeyScript(t, e, "gn")
	assertActiveBuffer(t, e, 2, paths[2], "gamma")

	pressKeyScript(t, e, "ga")
	assertActiveBuffer(t, e, 1, paths[1], "beta")

	pressKeyScript(t, e, "<space>b")
	if e.sidebar == nil || !e.sidebar.Visible || e.sidebar.Content == nil || e.sidebar.Content.Mode() != SidebarModeBuffers {
		t.Fatalf("sidebar content = %#v visible=%v, want open buffers sidebar", e.sidebar.Content, e.sidebar.Visible)
	}
}

func TestBufferDeleteCommandsRespectDirtyStateThroughFullKeySimulation(t *testing.T) {
	e, paths := newProfileBufferEditor(t, BehaviorProfileVim)

	pressKeyScript(t, e, ":b 1<enter>iX<esc>")
	assertActiveBuffer(t, e, 0, paths[0], "Xalpha")
	if !e.document.dirty {
		t.Fatalf("active buffer should be dirty after edit")
	}

	pressKeyScript(t, e, ":bd<enter>")
	assertActiveBuffer(t, e, 0, paths[0], "Xalpha")
	if e.BufferCount() != 3 {
		t.Fatalf("buffer count = %d, want dirty close to keep all buffers", e.BufferCount())
	}
	if !strings.Contains(e.ui.statusMessage, "unsaved changes") {
		t.Fatalf("status = %q, want dirty warning", e.ui.statusMessage)
	}

	pressKeyScript(t, e, ":bd!<enter>")
	assertActiveBuffer(t, e, 0, paths[1], "beta")
	if e.BufferCount() != 2 {
		t.Fatalf("buffer count = %d, want forced close to remove one buffer", e.BufferCount())
	}
}

func TestBufferDeleteCanTargetInactiveBuffersThroughFullKeySimulation(t *testing.T) {
	e, paths := newProfileBufferEditor(t, BehaviorProfileVim)

	pressKeyScript(t, e, ":bd 2<enter>")
	assertActiveBuffer(t, e, 1, paths[2], "gamma")
	if e.BufferCount() != 2 {
		t.Fatalf("buffer count = %d, want 2", e.BufferCount())
	}

	pressKeyScript(t, e, ":b 1<enter>")
	assertActiveBuffer(t, e, 0, paths[0], "alpha")
}

func TestBasicProfileCanUseBufferCommandsThroughCommandLine(t *testing.T) {
	e, paths := newProfileBufferEditor(t, BehaviorProfileBasic)

	pressKeyScript(t, e, "<alt+x>b 1<enter>")
	assertActiveBuffer(t, e, 0, paths[0], "alpha")
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want command line to return to normal after command execution", e.mode)
	}

	pressKeyScript(t, e, "<alt+x>bn<enter>")
	assertActiveBuffer(t, e, 1, paths[1], "beta")
}

func newProfileBufferEditor(t *testing.T, profile string) (*Editor, []string) {
	t.Helper()
	e := newSimulatedProfileEditor(profile, "")
	dir := t.TempDir()
	paths := []string{
		filepath.Join(dir, "alpha.txt"),
		filepath.Join(dir, "beta.txt"),
		filepath.Join(dir, "gamma.txt"),
	}
	contents := []string{"alpha", "beta", "gamma"}
	for i, path := range paths {
		if err := e.LoadFileContent(path, []byte(contents[i])); err != nil {
			t.Fatalf("LoadFileContent(%s) returned error: %v", path, err)
		}
	}
	e.SetBehaviorProfile(profile)
	if profile == BehaviorProfileBasic {
		e.mode = ModeInsert
	} else {
		e.mode = ModeNormal
	}
	return e, paths
}

func assertActiveBuffer(t *testing.T, e *Editor, wantIndex int, wantPath string, wantContent string) {
	t.Helper()
	if got := e.ActiveBufferIndex(); got != wantIndex {
		t.Fatalf("active buffer index = %d, want %d", got, wantIndex)
	}
	if got := e.Filename(); got != wantPath {
		t.Fatalf("filename = %q, want %q", got, wantPath)
	}
	if got := e.Content(); got != wantContent {
		t.Fatalf("content = %q, want %q", got, wantContent)
	}
}
