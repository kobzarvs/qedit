package editor

// Color is an opaque color value used by UI styles.
// Concrete implementations are provided by the UI layer.
type Color uint64

// AttrMask is an opaque attribute mask returned by Style.Decompose.
type AttrMask uint

// Style represents a UI style for rendering.
type Style interface {
	Decompose() (fg Color, bg Color, attrs AttrMask)
	Foreground(c Color) Style
	Background(c Color) Style
}

// Screen is an abstract rendering surface.
type Screen interface {
	Size() (w, h int)
	SetStyle(style Style)
	Clear()
	SetContent(x, y int, ch rune, comb []rune, style Style)
	Show()
	ShowCursor(x, y int)
	HideCursor()
	SetCursorStyle(style CursorStyle)
}

// CursorStyle describes how the cursor is rendered.
type CursorStyle int

const (
	CursorStyleSteadyBlock CursorStyle = iota
	CursorStyleSteadyBar
)

// ModMask represents key modifiers.
type ModMask uint16

const (
	ModCtrl ModMask = 1 << iota
	ModShift
	ModAlt
	ModMeta
)

// Key represents a key code (non-rune keys and control keys).
type Key int

const (
	KeyRune Key = iota
	KeyEscape
	KeyEnter
	KeyTab
	KeyBacktab
	KeyBackspace
	KeyBackspace2
	KeyDelete
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyPgUp
	KeyPgDn
	KeyHome
	KeyEnd
	KeyCtrlA
	KeyCtrlB
	KeyCtrlC
	KeyCtrlD
	KeyCtrlE
	KeyCtrlF
	KeyCtrlG
	KeyCtrlH
	KeyCtrlI
	KeyCtrlJ
	KeyCtrlK
	KeyCtrlL
	KeyCtrlM
	KeyCtrlN
	KeyCtrlO
	KeyCtrlP
	KeyCtrlQ
	KeyCtrlR
	KeyCtrlS
	KeyCtrlT
	KeyCtrlU
	KeyCtrlV
	KeyCtrlW
	KeyCtrlX
	KeyCtrlY
	KeyCtrlZ
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyUnknown
)

// MouseButton represents mouse buttons and wheel actions.
type MouseButton uint16

const (
	Button1 MouseButton = 1 << iota
	WheelUp
	WheelDown
	WheelLeft
	WheelRight
)

// EventKey is an abstract keyboard event.
type EventKey interface {
	Key() Key
	Rune() rune
	Modifiers() ModMask
}

// EventMouse is an abstract mouse event.
type EventMouse interface {
	Buttons() MouseButton
	Position() (x, y int)
}
