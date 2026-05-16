package editor

func (e *Editor) saveJumpPosition() {
	e.appendJumpPosition(e.cursor)
}

func (e *Editor) recordJump(from, to Cursor) {
	if from == to {
		return
	}
	e.appendJumpPosition(from)
	e.appendJumpPosition(to)
}

func (e *Editor) appendJumpPosition(pos Cursor) {
	if pos.Row < 0 || pos.Row >= e.LineCount() {
		return
	}
	if len(e.profile.jumps) > 0 && e.profile.jumpIndex >= 0 && e.profile.jumpIndex < len(e.profile.jumps)-1 {
		e.profile.jumps = e.profile.jumps[:e.profile.jumpIndex+1]
	}
	if len(e.profile.jumps) > 0 && e.profile.jumps[len(e.profile.jumps)-1] == pos {
		e.profile.jumpIndex = len(e.profile.jumps) - 1
		return
	}
	e.profile.jumps = append(e.profile.jumps, pos)
	e.profile.jumpIndex = len(e.profile.jumps) - 1
}

func (e *Editor) jumpBackward() {
	if len(e.profile.jumps) == 0 {
		e.setStatus("jumplist empty")
		return
	}
	if e.profile.jumpIndex <= 0 {
		e.profile.jumpIndex = 0
		e.cursor = e.profile.jumps[0]
		return
	}
	e.profile.jumpIndex--
	e.cursor = e.profile.jumps[e.profile.jumpIndex]
}

func (e *Editor) jumpForward() {
	if len(e.profile.jumps) == 0 {
		e.setStatus("jumplist empty")
		return
	}
	if e.profile.jumpIndex >= len(e.profile.jumps)-1 {
		e.profile.jumpIndex = len(e.profile.jumps) - 1
		e.cursor = e.profile.jumps[e.profile.jumpIndex]
		return
	}
	e.profile.jumpIndex++
	e.cursor = e.profile.jumps[e.profile.jumpIndex]
}
