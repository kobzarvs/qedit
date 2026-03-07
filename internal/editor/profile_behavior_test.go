package editor

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestBasicProfileTypesWithoutEnteringInsert(t *testing.T) {
	e := New(Options{Profile: BehaviorProfileBasic})

	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyRune, 'i', 0)))

	if got := e.Content(); got != "i" {
		t.Fatalf("content = %q, want %q", got, "i")
	}
	if e.BehaviorProfile() != BehaviorProfileBasic {
		t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileBasic)
	}
}

func TestVimProfileEnterInsertAndType(t *testing.T) {
	e := New(Options{Profile: BehaviorProfileVim})

	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyRune, 'i', 0)))
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyRune, 'x', 0)))
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0)))

	if got := e.Content(); got != "x" {
		t.Fatalf("content = %q, want %q", got, "x")
	}
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want %v", e.mode, ModeNormal)
	}
}

func TestVimProfileDeleteWordWithDw(t *testing.T) {
	e := New(Options{Profile: BehaviorProfileVim})
	e.text = NewTextBufferFromString("one two")

	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyRune, 'd', 0)))
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyRune, 'w', 0)))

	if got := e.Content(); got != "two" {
		t.Fatalf("content = %q, want %q", got, "two")
	}
}

func TestVimProfileLinewiseYankPaste(t *testing.T) {
	e := New(Options{Profile: BehaviorProfileVim})
	e.text = NewTextBufferFromString("one\ntwo")

	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyRune, 'y', 0)))
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyRune, 'y', 0)))
	e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyRune, 'p', 0)))

	if got := e.Content(); got != "one\none\ntwo" {
		t.Fatalf("content = %q, want %q", got, "one\\none\\ntwo")
	}
}

func TestBasicProfileAltXOpensCommandLine(t *testing.T) {
	e := newTestEditor("one")
	e.SetBehaviorProfile(BehaviorProfileBasic)

	e.HandleKey(eventForKeyString(t, "alt+x"))

	if e.mode != ModeCommand {
		t.Fatalf("mode = %v, want %v", e.mode, ModeCommand)
	}
}

func TestBasicProfileAltMEntersMergeReview(t *testing.T) {
	merged := strings.Join([]string{
		"alpha",
		"<<<<<<< local",
		"local",
		"=======",
		"remote",
		">>>>>>> remote",
		"charlie",
		"",
	}, "\n")
	cleaned, blocks := buildConflictView(merged)

	e := newTestEditor()
	e.SetBehaviorProfile(BehaviorProfileBasic)
	e.replaceBuffer(cleaned, true)
	e.conflicts.blocks = blocks
	e.conflicts.dirty = false
	e.mode = ModeInsert

	e.HandleKey(eventForKeyString(t, "alt+m"))

	if e.mode != ModeMerge {
		t.Fatalf("mode = %v, want %v", e.mode, ModeMerge)
	}
	if !e.mergeReviewActive() {
		t.Fatalf("expected merge review active")
	}
}
