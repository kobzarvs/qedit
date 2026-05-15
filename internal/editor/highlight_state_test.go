package editor

import "testing"

func TestMergeHighlightsKeepsDisjointRanges(t *testing.T) {
	e := New(Options{})
	e.MergeHighlights(10, 12, map[int][]HighlightSpan{
		10: {{StartCol: 0, EndCol: 1, Kind: "keyword"}},
		11: {{StartCol: 0, EndCol: 1, Kind: "keyword"}},
		12: {{StartCol: 0, EndCol: 1, Kind: "keyword"}},
	})
	e.MergeHighlights(20, 22, map[int][]HighlightSpan{
		20: {{StartCol: 0, EndCol: 1, Kind: "string"}},
		21: {{StartCol: 0, EndCol: 1, Kind: "string"}},
		22: {{StartCol: 0, EndCol: 1, Kind: "string"}},
	})

	if !e.HighlightsCover(10, 12) {
		t.Fatalf("expected first range to stay cached")
	}
	if !e.HighlightsCover(20, 22) {
		t.Fatalf("expected second range to stay cached")
	}
	if e.HighlightsCover(12, 20) {
		t.Fatalf("unexpected gap coverage across disjoint ranges")
	}
}

func TestSetHighlightsNormalizesAndSortsLineSpans(t *testing.T) {
	e := New(Options{})
	e.SetHighlights(0, 0, map[int][]HighlightSpan{
		0: {
			{StartCol: 8, EndCol: 10, Kind: "string"},
			{StartCol: 2, EndCol: 5, Kind: "keyword"},
			{StartCol: 5, EndCol: 5, Kind: "comment"},
		},
	})

	spans := e.highlight.spans[0]
	if len(spans) != 2 {
		t.Fatalf("len(spans) = %d, want 2", len(spans))
	}
	if spans[0].StartCol != 2 || spans[1].StartCol != 8 {
		t.Fatalf("spans not sorted: %#v", spans)
	}
}

func TestAdjustHighlightsDropsEditedRowsAndShiftsFollowingRows(t *testing.T) {
	e := New(Options{})
	e.SetHighlights(0, 3, map[int][]HighlightSpan{
		0: {{StartCol: 0, EndCol: 1, Kind: "keyword"}},
		1: {{StartCol: 0, EndCol: 1, Kind: "string"}},
		2: {{StartCol: 0, EndCol: 1, Kind: "comment"}},
		3: {{StartCol: 0, EndCol: 1, Kind: "number"}},
	})

	e.AdjustHighlights(1, 1, 2)

	if e.highlight.lineCovered(1) {
		t.Fatalf("edited row should not stay covered by stale highlights")
	}
	if !e.highlight.lineCovered(0) {
		t.Fatalf("row before edit should remain covered")
	}
	if got := e.highlight.spans[3]; len(got) != 1 || got[0].Kind != "comment" {
		t.Fatalf("row after edit not shifted correctly: %#v", got)
	}
	if got := e.highlight.spans[4]; len(got) != 1 || got[0].Kind != "number" {
		t.Fatalf("second row after edit not shifted correctly: %#v", got)
	}
}

func TestClipHighlightSpansAndWalker(t *testing.T) {
	spans := normalizeHighlightSpans([]HighlightSpan{
		{StartCol: 0, EndCol: 1000, Kind: "plain"},
		{StartCol: 120, EndCol: 140, Kind: "keyword"},
		{StartCol: 150, EndCol: 170, Kind: "comment"},
	})
	clipped := clipHighlightSpans(spans, 110, 160)
	// After flattening overlaps: [110,120)=plain, [120,140)=keyword, [140,150)=plain, [150,160)=comment
	if len(clipped) != 4 {
		t.Fatalf("len(clipped) = %d, want 4; spans=%#v", len(clipped), clipped)
	}
	if clipped[0].StartCol != 110 || clipped[0].EndCol != 120 || clipped[0].Kind != "plain" {
		t.Fatalf("clipped[0] = %#v, want [110,120) plain", clipped[0])
	}

	walker := newHighlightWalker(clipped)
	if kind, ok := walker.kindAt(115); !ok || kind != "plain" {
		t.Fatalf("kindAt(115) = (%q,%v), want plain,true", kind, ok)
	}
	if kind, ok := walker.kindAt(125); !ok || kind != "keyword" {
		t.Fatalf("kindAt(125) = (%q,%v), want keyword,true", kind, ok)
	}
	if kind, ok := walker.kindAt(155); !ok || kind != "comment" {
		t.Fatalf("kindAt(155) = (%q,%v), want comment,true", kind, ok)
	}
}
