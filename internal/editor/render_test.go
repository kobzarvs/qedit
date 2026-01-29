package editor

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
)

func TestRenderCommandlinePlacement(t *testing.T) {
	e := newTestEditor("abc")
	e.mode = ModeCommand
	e.cmd = []rune("w")

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(20, 5)

	e.Render(s)

	cells, w, h := s.GetContents()
	cmdCell := cells[(h-1)*w]
	if len(cmdCell.Runes) == 0 || cmdCell.Runes[0] != ':' {
		t.Fatalf("command line first rune = %q, want ':'", cmdCell.Runes)
	}
	statusCell := cells[(h-2)*w]
	if len(statusCell.Runes) > 0 && statusCell.Runes[0] == ':' {
		t.Fatalf("status line starts with ':'")
	}
}

func TestRenderCommandlineIdleBlank(t *testing.T) {
	e := newTestEditor("abc")
	e.mode = ModeNormal

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(20, 5)

	e.Render(s)
	cells, w, h := s.GetContents()
	cmdCell := cells[(h-1)*w]
	if len(cmdCell.Runes) == 0 {
		t.Fatalf("command line empty")
	}
	if cmdCell.Runes[0] != ' ' {
		t.Fatalf("command line first rune = %q, want space", cmdCell.Runes[0])
	}
}

func TestRenderCursorWithTab(t *testing.T) {
	cfg := config.Default()
	cfg.Editor.LineNumbers = "off"
	e := New(cfg)
	e.lines = [][]rune{[]rune("a\tb")}
	e.cursor = Cursor{Row: 0, Col: 2}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(20, 5)

	e.Render(s)
	x, y, visible := s.GetCursor()
	if !visible {
		t.Fatalf("cursor not visible")
	}
	wantX := visualCol(e.lines[0], e.cursor.Col, e.tabWidth)
	if x != wantX {
		t.Fatalf("cursor x = %d, want %d", x, wantX)
	}
	if y != 0 {
		t.Fatalf("cursor y = %d, want 0", y)
	}
}

func TestRenderSelectionStyle(t *testing.T) {
	cfg := config.Default()
	cfg.Editor.LineNumbers = "off"
	e := New(cfg)
	e.lines = [][]rune{[]rune("abc")}
	e.selectionActive = true
	e.selectionStart = Cursor{Row: 0, Col: 1}
	e.selectionEnd = Cursor{Row: 0, Col: 2}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(10, 3)

	e.Render(s)
	cells, w, _ := s.GetContents()
	normal := cells[0*w+0].Style
	selected := cells[0*w+1].Style
	_, bgNormal, _ := normal.Decompose()
	_, bgSelected, _ := selected.Decompose()
	if bgSelected == bgNormal {
		t.Fatalf("selection background not applied")
	}
}

func TestRenderSyntaxHighlightStyle(t *testing.T) {
	cfg := config.Default()
	cfg.Editor.LineNumbers = "off"
	e := New(cfg)
	e.lines = [][]rune{[]rune("abc")}
	e.SetHighlights(0, 0, map[int][]HighlightSpan{
		0: {
			{StartCol: 0, EndCol: 1, Kind: "keyword"},
		},
	})

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer s.Fini()
	s.SetSize(10, 3)

	e.Render(s)
	cells, w, _ := s.GetContents()
	unknownStyle := cells[0*w+1].Style
	hlStyle := cells[0*w+0].Style
	fgUnknown, _, _ := unknownStyle.Decompose()
	fgHl, _, _ := hlStyle.Decompose()
	if fgUnknown == fgHl {
		t.Fatalf("highlight foreground not applied")
	}
}
