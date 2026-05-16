package editor

func (e *Editor) vimStartRepeatRecording(prefix string) {
	if e.profile.vim.replayingRepeat {
		return
	}
	e.profile.vim.recordingRepeat = e.profile.vim.recordingRepeat[:0]
	e.profile.vim.recordingRepeatActive = true
	for _, r := range prefix {
		e.profile.vim.recordingRepeat = append(e.profile.vim.recordingRepeat, vimRepeatKey{
			key: KeyRune,
			r:   r,
		})
	}
}

func (e *Editor) vimRecordRepeatEvent(ev EventKey) {
	if e.profile.vim.replayingRepeat || !e.profile.vim.recordingRepeatActive {
		return
	}
	e.profile.vim.recordingRepeat = append(e.profile.vim.recordingRepeat, vimRepeatKey{
		key:  ev.Key(),
		r:    ev.Rune(),
		mods: ev.Modifiers(),
	})
}

func (e *Editor) vimFinishRepeatRecording() {
	if e.profile.vim.replayingRepeat || !e.profile.vim.recordingRepeatActive {
		return
	}
	if len(e.profile.vim.recordingRepeat) > 0 {
		e.profile.vim.repeatKeys = append(e.profile.vim.repeatKeys[:0], e.profile.vim.recordingRepeat...)
	}
	e.profile.vim.recordingRepeat = e.profile.vim.recordingRepeat[:0]
	e.profile.vim.recordingRepeatActive = false
}

func (e *Editor) vimCancelRepeatRecording() {
	if e.profile.vim.replayingRepeat {
		return
	}
	e.profile.vim.recordingRepeat = e.profile.vim.recordingRepeat[:0]
	e.profile.vim.recordingRepeatActive = false
}

func (e *Editor) vimFinishRepeatRecordingForOperator(operator string) {
	if operator == "c" {
		return
	}
	if operator == "y" {
		e.vimCancelRepeatRecording()
		return
	}
	e.vimFinishRepeatRecording()
}

func (e *Editor) vimCancelRepeatRecordingForOperator(operator string) {
	if operator == "y" {
		return
	}
	e.vimCancelRepeatRecording()
}

func (e *Editor) replayVimRepeat(count int) {
	if count < 1 {
		count = 1
	}
	if len(e.profile.vim.repeatKeys) == 0 {
		e.setStatus("no change to repeat")
		return
	}
	keys := append([]vimRepeatKey(nil), e.profile.vim.repeatKeys...)
	e.profile.vim.replayingRepeat = true
	defer func() {
		e.profile.vim.replayingRepeat = false
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
