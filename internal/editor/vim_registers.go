package editor

func (e *Editor) vimCommandPrefix(command string) string {
	prefix := e.profile.vim.count
	if e.profile.vim.registerName != 0 {
		prefix += `"` + string(e.profile.vim.registerName)
	}
	return prefix + command
}

func (e *Editor) vimSetActiveRegister(name rune) {
	e.profile.vim.registerName = name
}

func (e *Editor) vimWritesClipboard() bool {
	return e.profile.vim.registerName != '_'
}

func (e *Editor) vimStoreActiveRegister() {
	name := e.profile.vim.registerName
	if name == 0 || name == '_' {
		return
	}
	if e.profile.vim.registers == nil {
		e.profile.vim.registers = make(map[rune]editorClipboardState)
	}
	e.profile.vim.registers[name] = cloneClipboardState(e.clipboard)
}

func (e *Editor) vimLoadActiveRegister() bool {
	name := e.profile.vim.registerName
	if name == 0 {
		return true
	}
	if name == '_' {
		return false
	}
	value, ok := e.profile.vim.registers[name]
	if !ok {
		e.setStatus("register not set")
		return false
	}
	e.clipboard = cloneClipboardState(value)
	return true
}

func cloneClipboardState(in editorClipboardState) editorClipboardState {
	out := editorClipboardState{linewise: in.linewise}
	if len(in.lines) > 0 {
		out.lines = make([][]rune, len(in.lines))
		for i, line := range in.lines {
			out.lines[i] = append([]rune(nil), line...)
		}
	}
	return out
}
