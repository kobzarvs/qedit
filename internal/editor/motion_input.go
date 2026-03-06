package editor

import (
	"path/filepath"
	"strconv"
)

// handleGotoKey handles the second key after 'g' prefix
func (e *Editor) handleGotoKey(ch rune) bool {
	// Handle LSP goto commands
	switch ch {
	case 'd':
		e.lastCommand = "gd"
		return e.lspGoto("definition")
	case 'D':
		e.lastCommand = "gD"
		return e.lspGoto("declaration")
	case 'y':
		e.lastCommand = "gy"
		return e.lspGoto("typeDefinition")
	case 'r':
		e.lastCommand = "gr"
		return e.lspGoto("references")
	case 'i':
		e.lastCommand = "gi"
		return e.lspGoto("implementation")
	case 't':
		e.lastCommand = "gt"
		e.scrollCursorToTop()
		return false
	case 'c':
		e.lastCommand = "gc"
		e.centerCursorLine()
		return false
	case 'b':
		e.lastCommand = "gb"
		e.scrollCursorToBottom()
		return false
	}

	var action string
	switch ch {
	case 'g':
		action = actionGotoFirstLine
	case 'e':
		action = actionGotoFileEnd
	case 'h':
		action = actionLineStart
	case 'l':
		action = actionLineEnd
	case 's':
		action = actionFileStart // same as gg
	case 'a':
		action = actionGotoLastAccessed
	case 'n':
		action = actionGotoNextBuffer
	case 'p':
		action = actionGotoPrevBuffer
	default:
		return false
	}

	// Record the executed command
	e.lastCommand = "g" + string(ch)

	// In select mode, extend selection
	if e.selectMode && isMotionAction(action) {
		before := e.cursor
		result := e.execAction(action)
		if before != e.cursor {
			e.selectionEnd = e.cursor
		}
		return result
	}

	return e.execAction(action)
}

// lspGoto performs an LSP goto operation
func (e *Editor) lspGoto(method string) bool {
	if e.lspGotoFunc == nil {
		e.setStatus("LSP: callback not set")
		return false
	}
	if e.filename == "" {
		e.setStatus("LSP: no file open")
		return false
	}

	locations, err := e.lspGotoFunc(method, e.filename, e.cursor.Row, e.cursor.Col)
	if err != nil {
		e.setStatus("LSP: " + err.Error())
		return false
	}
	if len(locations) == 0 {
		e.setStatus("LSP: no " + method + " found")
		return false
	}

	// For references or multiple results, show picker
	if method == "references" || len(locations) > 1 {
		title := "References"
		if method == "implementation" {
			title = "Implementations"
		} else if method == "definition" {
			title = "Definitions"
		} else if method == "declaration" {
			title = "Declarations"
		} else if method == "typeDefinition" {
			title = "Type Definitions"
		}
		e.showRefsPicker(title, locations)
		return false
	}

	// Single result: jump directly
	loc := locations[0]
	currentAbs, _ := filepath.Abs(e.filename)
	if loc.Path != currentAbs && loc.Path != e.filename {
		e.requestOpenLocation(loc.Path, loc.StartLine, loc.StartCol)
		return false
	}

	e.cursor.Row = loc.StartLine
	e.cursor.Col = loc.StartCol
	e.ensureCursorVisible(e.viewHeightCached())
	e.setStatus(method + " → line " + strconv.Itoa(loc.StartLine+1))
	return false
}

// handleMatchKey handles the second key after 'm' prefix
func (e *Editor) handleMatchKey(ch rune) bool {
	e.lastCommand = "m" + string(ch)

	switch ch {
	case 'm':
		e.goToMatchingBracket()
	case 'a':
		e.setStatus("select around (not implemented)")
	case 'i':
		e.setStatus("select inside (not implemented)")
	case 's':
		e.setStatus("surround add (not implemented)")
	case 'r':
		e.setStatus("surround replace (not implemented)")
	case 'd':
		e.setStatus("surround delete (not implemented)")
	default:
		return false
	}
	return false
}

// setPendingFindChar sets up pending char find (f/F/t/T)
func (e *Editor) setPendingFindChar(action string) {
	e.pendingAction = action
}

// handlePendingChar processes char input for pending action
func (e *Editor) handlePendingChar(ch rune) bool {
	action := e.pendingAction
	e.pendingAction = ""

	// For f, F, t, T - Helix style: anchor moves to old cursor, selection covers jump
	isSelectingAction := action == actionFindChar || action == actionFindCharBackward ||
		action == actionTillChar || action == actionTillCharBackward

	anchor := e.cursor

	var result bool
	switch action {
	case actionFindChar:
		e.lastFindChar = ch
		e.lastFindForward = true
		e.lastFindTill = false
		result = e.findCharForward(ch, false)
	case actionFindCharBackward:
		e.lastFindChar = ch
		e.lastFindForward = false
		e.lastFindTill = false
		result = e.findCharBackward(ch, false)
	case actionTillChar:
		e.lastFindChar = ch
		e.lastFindForward = true
		e.lastFindTill = true
		result = e.findCharForward(ch, true)
	case actionTillCharBackward:
		e.lastFindChar = ch
		e.lastFindForward = false
		e.lastFindTill = true
		result = e.findCharBackward(ch, true)
	case actionReplaceChar:
		return e.replaceCharAtCursor(ch)
	default:
		return false
	}

	// Set selection from anchor to new cursor position (inclusive of cursor char)
	if isSelectingAction && anchor != e.cursor {
		e.selectionActive = true
		e.selectionStart = anchor
		// Selection end is exclusive, so add 1 to include the character at cursor
		e.selectionEnd = Cursor{Row: e.cursor.Row, Col: e.cursor.Col + 1}
		e.selectMode = true
	}

	return result
}
