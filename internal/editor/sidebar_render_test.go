package editor

import "testing"

type fakeStyle struct {
	fg Color
	bg Color
}

func (s fakeStyle) Decompose() (Color, Color, AttrMask) {
	return s.fg, s.bg, 0
}

func (s fakeStyle) Foreground(c Color) Style {
	s.fg = c
	return s
}

func (s fakeStyle) Background(c Color) Style {
	s.bg = c
	return s
}

type fakeScreen struct {
	w     int
	h     int
	style []Style
	runes []rune
}

func newFakeScreen(w, h int) *fakeScreen {
	return &fakeScreen{
		w:     w,
		h:     h,
		style: make([]Style, w*h),
		runes: make([]rune, w*h),
	}
}

func (s *fakeScreen) Size() (w, h int) { return s.w, s.h }
func (s *fakeScreen) SetStyle(style Style) {
	// no-op
}
func (s *fakeScreen) Clear() {
	for i := range s.style {
		s.style[i] = nil
		s.runes[i] = 0
	}
}
func (s *fakeScreen) SetContent(x, y int, ch rune, comb []rune, style Style) {
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return
	}
	idx := y*s.w + x
	s.style[idx] = style
	s.runes[idx] = ch
}
func (s *fakeScreen) Show()                            {}
func (s *fakeScreen) ShowCursor(x, y int)              {}
func (s *fakeScreen) HideCursor()                      {}
func (s *fakeScreen) SetCursorStyle(style CursorStyle) {}

func TestSidebarSelectionKeepsForegrounds(t *testing.T) {
	screen := newFakeScreen(20, 4)

	sidebar := NewSidebar("20", 10, "20", false)
	sidebar.Content = NewSidebarMenuContent(true, true)
	sidebar.Focused = true
	sidebar.Content.SetIndex(0)

	base := fakeStyle{fg: 1, bg: 2}
	hotkey := fakeStyle{fg: 3, bg: 2}
	styles := SidebarStyles{
		Base:               base,
		Dir:                base,
		Selected:           fakeStyle{fg: 9, bg: 9},
		SelectedBackground: 7,
		Header:             base,
		Border:             base,
		Hidden:             base,
		Ignored:            base,
		Indicator:          base,
		Hotkey:             hotkey,
		Unavailable:        base,
		Current:            base,
		DiffAdd:            base,
		DiffDel:            base,
	}

	sidebar.Render(screen, styles, 0, 0, 20, 4)

	w := screen.w

	// Row 1 is the first item ("Files").
	row := 1
	labelStyle := screen.style[row*w+1]
	labelFg, labelBg, _ := labelStyle.Decompose()
	if labelFg != base.fg {
		t.Fatalf("label fg = %v, want %v", labelFg, base.fg)
	}
	if labelBg != Color(7) {
		t.Fatalf("label bg = %v, want %v", labelBg, 7)
	}

	hotkeyLabel := "Cmd+O"
	contentWidth := 20 - 1
	hotkeyX := contentWidth - stringWidth(hotkeyLabel) - 1
	hotkeyStyle := screen.style[row*w+hotkeyX]
	hotkeyFg, hotkeyBg, _ := hotkeyStyle.Decompose()
	if hotkeyFg != hotkey.fg {
		t.Fatalf("hotkey fg = %v, want %v", hotkeyFg, hotkey.fg)
	}
	if hotkeyBg != Color(7) {
		t.Fatalf("hotkey bg = %v, want %v", hotkeyBg, 7)
	}
}
