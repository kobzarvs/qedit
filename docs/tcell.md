# tcell notes

Sources:
- https://pkg.go.dev/github.com/gdamore/tcell/v2

Key points:
- Screen interface includes SetCursorStyle(CursorStyle, ...Color).
- Cursor style changes may be ignored if terminal does not support them.
- CursorStyle includes steady/blinking block/underline/bar variants.

Relevant excerpt (non-verbatim summary):
- Use Screen.SetCursorStyle to request cursor shape; terminals may ignore it.
