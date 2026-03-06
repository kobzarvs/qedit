package editor

import (
	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
)

type testStyle struct {
	style tcell.Style
}

func (s testStyle) Decompose() (Color, Color, AttrMask) {
	fg, bg, attrs := s.style.Decompose()
	return Color(fg), Color(bg), AttrMask(attrs)
}

func (s testStyle) Foreground(c Color) Style {
	return testStyle{style: s.style.Foreground(tcell.Color(c))}
}

func (s testStyle) Background(c Color) Style {
	return testStyle{style: s.style.Background(tcell.Color(c))}
}

func styleFrom(fg, bg tcell.Color) Style {
	return testStyle{style: tcell.StyleDefault.Foreground(fg).Background(bg)}
}

func applyTestStyles(e *Editor) {
	e.SetStyles(testStyles())
}

func testStyles() EditorStyles {
	main := styleFrom(tcell.ColorWhite, tcell.ColorBlack)
	status := styleFrom(tcell.ColorWhite, tcell.ColorBlack)
	command := styleFrom(tcell.ColorWhite, tcell.ColorBlack)
	selection := styleFrom(tcell.ColorWhite, tcell.ColorBlue)
	searchMatch := styleFrom(tcell.ColorBlack, tcell.ColorYellow)
	syntaxKeyword := styleFrom(tcell.ColorGreen, tcell.ColorBlack)
	syntaxUnknown := styleFrom(tcell.ColorRed, tcell.ColorBlack)
	layoutUS := styleFrom(tcell.ColorGreen, tcell.ColorBlack)
	layoutRU := styleFrom(tcell.ColorRed, tcell.ColorBlack)
	layoutOther := styleFrom(tcell.ColorYellow, tcell.ColorBlack)

	return EditorStyles{
		Main:                    main,
		Status:                  status,
		StatusWarning:           status,
		MergeLocal:              main,
		MergeRemote:             main,
		MergeHeader:             status,
		Command:                 command,
		CommandCheckmark:        command,
		LineNumber:              main,
		LineNumberActive:        main,
		Selection:               selection,
		SearchMatch:             searchMatch,
		SyntaxKeyword:           syntaxKeyword,
		SyntaxString:            main,
		SyntaxComment:           main,
		SyntaxType:              main,
		SyntaxFunction:          main,
		SyntaxNumber:            main,
		SyntaxConstant:          main,
		SyntaxOperator:          main,
		SyntaxPunctuation:       main,
		SyntaxField:             main,
		SyntaxBuiltin:           main,
		SyntaxUnknown:           syntaxUnknown,
		SyntaxVariable:          main,
		SyntaxParameter:         main,
		TableBorder:             main,
		Branch:                  main,
		MainBranch:              main,
		LayoutUS:                layoutUS,
		LayoutRU:                layoutRU,
		LayoutOther:             layoutOther,
		AutoComplete:            main,
		AutoCompleteHotkey:      main,
		AutoCompleteDescription: main,
		AutoCompleteGroup:       main,
		ScrollIndicator:         main,
		BranchMarker:            main,
		FilterActive:            selection,
		FilterInactive:          command,
		NotificationFade:        []Style{searchMatch},
		Sidebar: SidebarStyles{
			Base:            main,
			Dir:             main,
			Selected:        selection,
			Header:          main,
			Border:          main,
			Hidden:          main,
			Ignored:         main,
			Indicator:       main,
			Hotkey:          main,
			Unavailable:     main,
			Current:         main,
			DiffAdd:         main,
			DiffDel:         main,
			SearchMatch:     searchMatch,
			SearchMatchFile: searchMatch,
			SearchMatchDir:  searchMatch,
		},
	}
}

func optionsFromConfig(cfg config.Config) Options {
	return Options{
		TabWidth:             cfg.Editor.TabWidth,
		Profile:              cfg.Editor.Profile,
		LineNumbers:          cfg.Editor.LineNumbers,
		GitBranchSymbol:      cfg.Editor.GitBranchSymbol,
		SidebarWidth:         cfg.Editor.SidebarWidth,
		SidebarMinWidth:      cfg.Editor.SidebarMinWidth,
		SidebarMaxWidth:      cfg.Editor.SidebarMaxWidth,
		SidebarCloseOnSelect: cfg.Editor.SidebarCloseOnSelect,
		AutoReloadOnChanges:  cfg.Editor.AutoReloadOnChanges,
		KeymapNormal:         cfg.Keymap.Normal,
		KeymapInsert:         cfg.Keymap.Insert,
	}
}

type testScreen struct {
	screen tcell.Screen
}

func wrapScreen(screen tcell.Screen) Screen {
	return testScreen{screen: screen}
}

func (s testScreen) Size() (w, h int) {
	return s.screen.Size()
}

func (s testScreen) SetStyle(style Style) {
	s.screen.SetStyle(toTcellStyle(style))
}

func (s testScreen) Clear() {
	s.screen.Clear()
}

func (s testScreen) SetContent(x, y int, ch rune, comb []rune, style Style) {
	s.screen.SetContent(x, y, ch, comb, toTcellStyle(style))
}

func (s testScreen) Show() {
	s.screen.Show()
}

func (s testScreen) ShowCursor(x, y int) {
	s.screen.ShowCursor(x, y)
}

func (s testScreen) HideCursor() {
	s.screen.HideCursor()
}

func (s testScreen) SetCursorStyle(style CursorStyle) {
	switch style {
	case CursorStyleSteadyBar:
		s.screen.SetCursorStyle(tcell.CursorStyleSteadyBar)
	default:
		s.screen.SetCursorStyle(tcell.CursorStyleSteadyBlock)
	}
}

func toTcellStyle(style Style) tcell.Style {
	if style == nil {
		return tcell.StyleDefault
	}
	switch s := style.(type) {
	case testStyle:
		return s.style
	case *testStyle:
		return s.style
	default:
		fg, bg, attrs := style.Decompose()
		return tcell.StyleDefault.
			Foreground(tcell.Color(fg)).
			Background(tcell.Color(bg)).
			Attributes(tcell.AttrMask(attrs))
	}
}

type testKeyAdapter struct {
	ev *tcell.EventKey
}

func wrapKey(ev *tcell.EventKey) EventKey {
	return testKeyAdapter{ev: ev}
}

func (k testKeyAdapter) Key() Key {
	return toEditorKey(k.ev.Key())
}

func (k testKeyAdapter) Rune() rune {
	return k.ev.Rune()
}

func (k testKeyAdapter) Modifiers() ModMask {
	return toEditorModMask(k.ev.Modifiers())
}

type testMouseAdapter struct {
	ev *tcell.EventMouse
}

func wrapMouse(ev *tcell.EventMouse) EventMouse {
	return testMouseAdapter{ev: ev}
}

func (m testMouseAdapter) Buttons() MouseButton {
	return toEditorButtons(m.ev.Buttons())
}

func (m testMouseAdapter) Position() (x, y int) {
	return m.ev.Position()
}

func toEditorKey(key tcell.Key) Key {
	switch key {
	case tcell.KeyRune:
		return KeyRune
	case tcell.KeyEscape:
		return KeyEscape
	case tcell.KeyEnter:
		return KeyEnter
	case tcell.KeyTab:
		return KeyTab
	case tcell.KeyBacktab:
		return KeyBacktab
	case tcell.KeyBackspace:
		return KeyBackspace
	case tcell.KeyBackspace2:
		return KeyBackspace2
	case tcell.KeyDelete:
		return KeyDelete
	case tcell.KeyUp:
		return KeyUp
	case tcell.KeyDown:
		return KeyDown
	case tcell.KeyLeft:
		return KeyLeft
	case tcell.KeyRight:
		return KeyRight
	case tcell.KeyPgUp:
		return KeyPgUp
	case tcell.KeyPgDn:
		return KeyPgDn
	case tcell.KeyHome:
		return KeyHome
	case tcell.KeyEnd:
		return KeyEnd
	case tcell.KeyCtrlA:
		return KeyCtrlA
	case tcell.KeyCtrlB:
		return KeyCtrlB
	case tcell.KeyCtrlC:
		return KeyCtrlC
	case tcell.KeyCtrlD:
		return KeyCtrlD
	case tcell.KeyCtrlE:
		return KeyCtrlE
	case tcell.KeyCtrlF:
		return KeyCtrlF
	case tcell.KeyCtrlG:
		return KeyCtrlG
	case tcell.KeyCtrlH:
		return KeyCtrlH
	case tcell.KeyCtrlI:
		return KeyCtrlI
	case tcell.KeyCtrlJ:
		return KeyCtrlJ
	case tcell.KeyCtrlK:
		return KeyCtrlK
	case tcell.KeyCtrlL:
		return KeyCtrlL
	case tcell.KeyCtrlM:
		return KeyCtrlM
	case tcell.KeyCtrlN:
		return KeyCtrlN
	case tcell.KeyCtrlO:
		return KeyCtrlO
	case tcell.KeyCtrlP:
		return KeyCtrlP
	case tcell.KeyCtrlQ:
		return KeyCtrlQ
	case tcell.KeyCtrlR:
		return KeyCtrlR
	case tcell.KeyCtrlS:
		return KeyCtrlS
	case tcell.KeyCtrlT:
		return KeyCtrlT
	case tcell.KeyCtrlU:
		return KeyCtrlU
	case tcell.KeyCtrlV:
		return KeyCtrlV
	case tcell.KeyCtrlW:
		return KeyCtrlW
	case tcell.KeyCtrlX:
		return KeyCtrlX
	case tcell.KeyCtrlY:
		return KeyCtrlY
	case tcell.KeyCtrlZ:
		return KeyCtrlZ
	case tcell.KeyF1:
		return KeyF1
	case tcell.KeyF2:
		return KeyF2
	case tcell.KeyF3:
		return KeyF3
	case tcell.KeyF4:
		return KeyF4
	case tcell.KeyF5:
		return KeyF5
	case tcell.KeyF6:
		return KeyF6
	case tcell.KeyF7:
		return KeyF7
	case tcell.KeyF8:
		return KeyF8
	case tcell.KeyF9:
		return KeyF9
	case tcell.KeyF10:
		return KeyF10
	case tcell.KeyF11:
		return KeyF11
	case tcell.KeyF12:
		return KeyF12
	default:
		return KeyUnknown
	}
}

func toEditorModMask(mask tcell.ModMask) ModMask {
	var out ModMask
	if mask&tcell.ModCtrl != 0 {
		out |= ModCtrl
	}
	if mask&tcell.ModShift != 0 {
		out |= ModShift
	}
	if mask&tcell.ModAlt != 0 {
		out |= ModAlt
	}
	if mask&tcell.ModMeta != 0 {
		out |= ModMeta
	}
	return out
}

func toEditorButtons(mask tcell.ButtonMask) MouseButton {
	var out MouseButton
	if mask&tcell.Button1 != 0 {
		out |= Button1
	}
	if mask&tcell.WheelUp != 0 {
		out |= WheelUp
	}
	if mask&tcell.WheelDown != 0 {
		out |= WheelDown
	}
	if mask&tcell.WheelLeft != 0 {
		out |= WheelLeft
	}
	if mask&tcell.WheelRight != 0 {
		out |= WheelRight
	}
	return out
}
