package editor

type editorBindingsState struct {
	keymap     keymapSet
	actionHook func(action string)
}
