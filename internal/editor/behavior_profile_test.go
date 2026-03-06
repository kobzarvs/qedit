package editor

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestEditorDefaultsToHelixBehaviorProfile(t *testing.T) {
	e := New(Options{})

	if got := e.BehaviorProfile(); got != BehaviorProfileHelix {
		t.Fatalf("BehaviorProfile = %q, want %q", got, BehaviorProfileHelix)
	}
}

func TestRegisterBehaviorProfileOverridesInputDispatch(t *testing.T) {
	e := New(Options{})
	called := false
	e.RegisterBehaviorProfile(BehaviorProfile{
		Name: "test",
		HandleKey: func(*Editor, EventKey) bool {
			called = true
			return false
		},
	})
	if !e.SetBehaviorProfile("test") {
		t.Fatalf("SetBehaviorProfile returned false")
	}

	quit := e.HandleKey(wrapKey(tcell.NewEventKey(tcell.KeyRune, 'x', 0)))

	if quit {
		t.Fatalf("HandleKey returned quit for custom profile")
	}
	if !called {
		t.Fatalf("expected custom profile handler to be called")
	}
}

func TestUnknownBehaviorProfileFallsBackToHelix(t *testing.T) {
	e := New(Options{Profile: "missing"})

	if got := e.BehaviorProfile(); got != BehaviorProfileHelix {
		t.Fatalf("BehaviorProfile = %q, want %q", got, BehaviorProfileHelix)
	}
}
