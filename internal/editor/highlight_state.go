package editor

import "sort"

type highlightRange struct {
	start int
	end   int
}

type editorHighlightState struct {
	spans  map[int][]HighlightSpan
	start  int
	end    int
	ranges []highlightRange
}

func (h *editorHighlightState) reset() {
	h.spans = nil
	h.start = -1
	h.end = -1
	h.ranges = nil
}

func (h *editorHighlightState) has() bool {
	if h.spans == nil {
		return false
	}
	if len(h.ranges) > 0 {
		return true
	}
	return h.start >= 0 && h.end >= h.start
}

func (h *editorHighlightState) lineCovered(line int) bool {
	if line < 0 || !h.has() {
		return false
	}
	if len(h.ranges) == 0 {
		return line >= h.start && line <= h.end
	}
	for _, r := range h.ranges {
		if line < r.start {
			return false
		}
		if line <= r.end {
			return true
		}
	}
	return false
}

func (h *editorHighlightState) covers(startLine, endLine int) bool {
	if startLine > endLine || !h.has() {
		return false
	}
	if len(h.ranges) == 0 {
		return h.start <= startLine && h.end >= endLine
	}
	for _, r := range h.ranges {
		if r.start > startLine {
			return false
		}
		if r.end >= endLine {
			return true
		}
	}
	return false
}

func (h *editorHighlightState) set(startLine, endLine int, spans map[int][]HighlightSpan) {
	if spans == nil || startLine < 0 || endLine < startLine {
		h.reset()
		return
	}
	h.spans = make(map[int][]HighlightSpan, len(spans))
	for line, lineSpans := range spans {
		h.spans[line] = normalizeHighlightSpans(lineSpans)
	}
	h.start = startLine
	h.end = endLine
	h.ranges = []highlightRange{{start: startLine, end: endLine}}
}

func (h *editorHighlightState) merge(startLine, endLine int, spans map[int][]HighlightSpan) {
	if spans == nil || startLine < 0 || endLine < startLine {
		return
	}
	if h.spans == nil {
		h.spans = make(map[int][]HighlightSpan, len(spans))
	}
	for line, lineSpans := range spans {
		h.spans[line] = normalizeHighlightSpans(lineSpans)
	}
	h.mergeRange(startLine, endLine)
}

func (h *editorHighlightState) mergeRange(startLine, endLine int) {
	if startLine < 0 || endLine < startLine {
		return
	}
	if len(h.ranges) == 0 {
		h.start = startLine
		h.end = endLine
		h.ranges = []highlightRange{{start: startLine, end: endLine}}
		return
	}
	merged := make([]highlightRange, 0, len(h.ranges)+1)
	inserted := false
	newRange := highlightRange{start: startLine, end: endLine}
	for _, r := range h.ranges {
		if r.end+1 < newRange.start {
			merged = append(merged, r)
			continue
		}
		if newRange.end+1 < r.start {
			if !inserted {
				merged = append(merged, newRange)
				inserted = true
			}
			merged = append(merged, r)
			continue
		}
		if r.start < newRange.start {
			newRange.start = r.start
		}
		if r.end > newRange.end {
			newRange.end = r.end
		}
	}
	if !inserted {
		merged = append(merged, newRange)
	}
	h.ranges = merged
	h.start = merged[0].start
	h.end = merged[len(merged)-1].end
}

func (h *editorHighlightState) rebuildRangesFromSpans() {
	if h.spans == nil || len(h.spans) == 0 {
		h.reset()
		return
	}
	lines := make([]int, 0, len(h.spans))
	for line := range h.spans {
		lines = append(lines, line)
	}
	sort.Ints(lines)
	h.ranges = h.ranges[:0]
	start := lines[0]
	prev := lines[0]
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if line == prev+1 {
			prev = line
			continue
		}
		h.ranges = append(h.ranges, highlightRange{start: start, end: prev})
		start = line
		prev = line
	}
	h.ranges = append(h.ranges, highlightRange{start: start, end: prev})
	h.start = h.ranges[0].start
	h.end = h.ranges[len(h.ranges)-1].end
}

func normalizeHighlightSpans(spans []HighlightSpan) []HighlightSpan {
	if len(spans) == 0 {
		return nil
	}
	out := make([]HighlightSpan, 0, len(spans))
	for _, span := range spans {
		if span.EndCol <= span.StartCol {
			continue
		}
		out = append(out, span)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartCol != out[j].StartCol {
			return out[i].StartCol < out[j].StartCol
		}
		if out[i].EndCol != out[j].EndCol {
			return out[i].EndCol < out[j].EndCol
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}
