package editor

import (
	"strings"
	"testing"
)

func TestTutorCommandOpensVimTutorScratchBuffer(t *testing.T) {
	e := newTestEditor("draft")

	e.execCommand("tutor vim")

	if e.BehaviorProfile() != BehaviorProfileVim {
		t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileVim)
	}
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want normal", e.mode)
	}
	if e.document.filename != "" {
		t.Fatalf("filename = %q, want unnamed tutor buffer", e.document.filename)
	}
	if e.document.title != "[Tutor: Vim]" {
		t.Fatalf("title = %q, want Vim tutor title", e.document.title)
	}
	if e.document.dirty {
		t.Fatalf("new tutor buffer should start clean")
	}
	if !strings.Contains(e.Content(), "Welcome   to   the   VIM   Tutor") {
		t.Fatalf("content does not look like Vim tutor")
	}
	if e.BufferCount() != 2 {
		t.Fatalf("buffer count = %d, want 2", e.BufferCount())
	}

	e.gotoPrevBuffer()
	if got := e.Content(); got != "draft" {
		t.Fatalf("previous scratch content = %q, want draft", got)
	}
}

func TestTutorCommandDefaultsToCurrentProfile(t *testing.T) {
	e := newTestEditor()
	e.SetBehaviorProfile(BehaviorProfileHelix)

	e.execCommand("tutor")

	if e.BehaviorProfile() != BehaviorProfileHelix {
		t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileHelix)
	}
	if e.document.title != "[Tutor: Helix]" {
		t.Fatalf("title = %q, want Helix tutor title", e.document.title)
	}
	if !strings.Contains(e.Content(), "Welcome to the Helix tutorial") {
		t.Fatalf("content does not look like Helix tutor")
	}
}

func TestTutorCommandRejectsUnknownTutor(t *testing.T) {
	e := newTestEditor()

	e.execCommand("tutor basic")

	if e.ui.statusMessage != "usage: tutor [vim|helix]" {
		t.Fatalf("status = %q, want usage", e.ui.statusMessage)
	}
	if e.Content() != "" {
		t.Fatalf("content = %q, want unchanged empty buffer", e.Content())
	}
}
