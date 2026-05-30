package editor

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

const (
	snapshotWidth  = 32
	snapshotHeight = 5
)

func TestRenderSnapshotBaseline(t *testing.T) {
	e := newTestEditor("hello", "world")
	e.display.lineNumberMode = LineNumberOff

	got := renderSnapshot(t, e, snapshotWidth, snapshotHeight)
	want := strings.Join([]string{
		"hello" + strings.Repeat(".", snapshotWidth-len("hello")),
		"world" + strings.Repeat(".", snapshotWidth-len("world")),
		strings.Repeat(".", snapshotWidth),
		".NORMAL.|.Helix.|.[N.Ln.1,.Col.1",
		strings.Repeat(".", snapshotWidth),
	}, "\n")

	if got != want {
		t.Fatalf("render snapshot mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func renderSnapshot(t *testing.T, e *Editor, w, h int) string {
	t.Helper()

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(w, h)

	e.Render(wrapScreen(s))

	return snapshotFromScreen(s)
}

func snapshotFromScreen(s tcell.SimulationScreen) string {
	cells, w, h := s.GetContents()
	lines := make([]string, h)
	for y := 0; y < h; y++ {
		var b strings.Builder
		b.Grow(w)
		for x := 0; x < w; x++ {
			cell := cells[y*w+x]
			r := ' '
			if len(cell.Runes) > 0 {
				r = cell.Runes[0]
			}
			if r == ' ' {
				r = '.'
			}
			b.WriteRune(r)
		}
		lines[y] = b.String()
	}
	return strings.Join(lines, "\n")
}
