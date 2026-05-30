package editor

import (
	"strings"
	"testing"
)

func TestBasicProfileTypesWithoutEnteringInsert(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileBasic, "")

	pressKeyScript(t, e, "i")

	if got := e.Content(); got != "i" {
		t.Fatalf("content = %q, want %q", got, "i")
	}
	if e.BehaviorProfile() != BehaviorProfileBasic {
		t.Fatalf("profile = %q, want %q", e.BehaviorProfile(), BehaviorProfileBasic)
	}
}

func TestVimProfileEnterInsertAndType(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileVim, "")

	pressKeyScript(t, e, "ix<esc>")

	if got := e.Content(); got != "x" {
		t.Fatalf("content = %q, want %q", got, "x")
	}
	if e.mode != ModeNormal {
		t.Fatalf("mode = %v, want %v", e.mode, ModeNormal)
	}
}

func TestVimProfileDeleteWordWithDw(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileVim, "one two")

	pressKeyScript(t, e, "dw")

	if got := e.Content(); got != "two" {
		t.Fatalf("content = %q, want %q", got, "two")
	}
}

func TestVimProfileLinewiseYankPaste(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileVim, "one", "two")

	pressKeyScript(t, e, "yyp")

	if got := e.Content(); got != "one\none\ntwo" {
		t.Fatalf("content = %q, want %q", got, "one\\none\\ntwo")
	}
}

func TestBasicProfileAltXOpensCommandLine(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileBasic, "one")

	pressKeyScript(t, e, "<alt+x>")

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

	pressKeyScript(t, e, "<alt+m>")

	if e.mode != ModeMerge {
		t.Fatalf("mode = %v, want %v", e.mode, ModeMerge)
	}
	if !e.mergeReviewActive() {
		t.Fatalf("expected merge review active")
	}
}
