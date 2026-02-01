package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
)

func TestStylesFromConfigSidebarSelectedBackground(t *testing.T) {
	cfg := config.Default()
	cfg.Theme.SidebarSelectedBackground = "#555555"

	styles := StylesFromConfig(cfg)
	want := editor.Color(tcell.NewHexColor(0x555555))
	if styles.Sidebar.SelectedBackground != want {
		t.Fatalf("SelectedBackground = %v, want %v", styles.Sidebar.SelectedBackground, want)
	}
}

func TestStylesFromConfigBoxBorder(t *testing.T) {
	cfg := config.Default()
	cfg.Theme.BoxBorderForeground = "#010203"
	cfg.Theme.BoxBorderBackground = "#040506"

	styles := StylesFromConfig(cfg)
	fg, bg, _ := styles.BoxBorder.Decompose()
	if fg != editor.Color(tcell.NewHexColor(0x010203)) {
		t.Fatalf("BoxBorder fg = %v, want %v", fg, tcell.NewHexColor(0x010203))
	}
	if bg != editor.Color(tcell.NewHexColor(0x040506)) {
		t.Fatalf("BoxBorder bg = %v, want %v", bg, tcell.NewHexColor(0x040506))
	}
}
