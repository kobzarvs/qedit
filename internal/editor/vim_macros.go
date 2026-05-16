package editor

func (e *Editor) vimStartMacroRecording(name rune) {
	if name == 0 || name == '_' {
		return
	}
	if e.profile.vim.macros == nil {
		e.profile.vim.macros = make(map[rune][]vimRepeatKey)
	}
	e.profile.vim.macroRecording = true
	e.profile.vim.macroRegister = name
	e.profile.vim.macros[name] = e.profile.vim.macros[name][:0]
	e.setStatus("recording")
}

func (e *Editor) vimStopMacroRecording() {
	if !e.profile.vim.macroRecording {
		return
	}
	e.profile.vim.lastMacroRegister = e.profile.vim.macroRegister
	e.profile.vim.macroRecording = false
	e.profile.vim.macroRegister = 0
	e.setStatus("recorded")
}

func (e *Editor) vimRecordMacroEvent(ev EventKey) {
	if e.profile.vim.replayingMacro || !e.profile.vim.macroRecording {
		return
	}
	name := e.profile.vim.macroRegister
	if name == 0 {
		return
	}
	e.profile.vim.macros[name] = append(e.profile.vim.macros[name], vimRepeatKey{
		key:  ev.Key(),
		r:    ev.Rune(),
		mods: ev.Modifiers(),
	})
}

func (e *Editor) vimReplayMacro(name rune, count int) {
	if name == '@' {
		name = e.profile.vim.lastMacroRegister
	}
	if name == 0 {
		e.setStatus("no macro")
		return
	}
	keys := append([]vimRepeatKey(nil), e.profile.vim.macros[name]...)
	if len(keys) == 0 {
		e.setStatus("macro not set")
		return
	}
	if count < 1 {
		count = 1
	}
	e.profile.vim.lastMacroRegister = name
	e.profile.vim.replayingMacro = true
	defer func() {
		e.profile.vim.replayingMacro = false
	}()
	for i := 0; i < count; i++ {
		for _, key := range keys {
			e.HandleKey(vimRepeatEventKey{
				key:  key.key,
				r:    key.r,
				mods: key.mods,
			})
		}
	}
}
