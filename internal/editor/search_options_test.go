package editor

import (
	"strings"
	"testing"
)

func TestVimSearchOptionsThroughFullKeySimulation(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileVim, "ignore", "Ignore", "IGNORE")

	pressKeyScript(t, e, "/ignore<enter>")
	if len(e.searchMatches) != 1 {
		t.Fatalf("case-sensitive matches = %d, want 1", len(e.searchMatches))
	}
	if e.cursor.Row != 0 {
		t.Fatalf("cursor row = %d, want first lowercase match", e.cursor.Row)
	}

	pressKeyScript(t, e, ":set ic<enter>n")
	if len(e.searchMatches) != 3 {
		t.Fatalf("ignorecase matches = %d, want 3", len(e.searchMatches))
	}
	if e.cursor.Row != 1 {
		t.Fatalf("cursor row after ignorecase n = %d, want 1", e.cursor.Row)
	}

	pressKeyScript(t, e, ":set noic<enter>n")
	if len(e.searchMatches) != 1 {
		t.Fatalf("noignorecase matches = %d, want 1", len(e.searchMatches))
	}
	if e.cursor.Row != 0 {
		t.Fatalf("cursor row after noignorecase n = %d, want wrap to lowercase match", e.cursor.Row)
	}

	pressKeyScript(t, e, ":set invic<enter>n")
	if !e.searchIgnoreCase {
		t.Fatalf("searchIgnoreCase = false, want true after invic")
	}
	if e.cursor.Row != 1 {
		t.Fatalf("cursor row after invic n = %d, want 1", e.cursor.Row)
	}
}

func TestVimSearchHighlightCommandsThroughFullKeySimulation(t *testing.T) {
	e := newSimulatedProfileEditor(BehaviorProfileVim, "one", "one")

	pressKeyScript(t, e, "/one<enter>")
	if len(e.searchMatches) != 2 {
		t.Fatalf("matches = %d, want 2", len(e.searchMatches))
	}

	pressKeyScript(t, e, ":nohlsearch<enter>")
	if len(e.searchMatches) != 0 {
		t.Fatalf("matches after nohlsearch = %d, want 0", len(e.searchMatches))
	}
	if e.lastSearchQuery != "one" {
		t.Fatalf("lastSearchQuery = %q, want one", e.lastSearchQuery)
	}

	pressKeyScript(t, e, ":set hls is<enter>")
	if !e.searchHighlight {
		t.Fatalf("searchHighlight = false, want true")
	}
	if !strings.Contains(e.ui.statusMessage, "hlsearch") || !strings.Contains(e.ui.statusMessage, "incsearch") {
		t.Fatalf("status = %q, want hlsearch and incsearch", e.ui.statusMessage)
	}
}
