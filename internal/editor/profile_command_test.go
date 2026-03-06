package editor

import "testing"

func TestProfileCommandSwitchesBehaviorProfile(t *testing.T) {
	e := New(Options{Profile: BehaviorProfileHelix})

	e.execCommand("profile basic")

	if got := e.BehaviorProfile(); got != BehaviorProfileBasic {
		t.Fatalf("profile = %q, want %q", got, BehaviorProfileBasic)
	}
	req, ok := e.ConsumeRuntimeRequest()
	if !ok {
		t.Fatalf("expected persistence request")
	}
	if req.Kind != RuntimeRequestPersistProfile {
		t.Fatalf("request kind = %v, want %v", req.Kind, RuntimeRequestPersistProfile)
	}
	if req.Value != BehaviorProfileBasic || req.PrevValue != BehaviorProfileHelix {
		t.Fatalf("request = %+v, want basic <- helix", req)
	}
}
