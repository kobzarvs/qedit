package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const statusMessageTopThreshold = 40

func (e *Editor) showTopStatusMessage(w int) bool {
	if e.statusMessage == "" {
		return false
	}
	if e.autoReloadInProgress {
		return true
	}
	msgLen := utf8.RuneCountInString(e.statusMessage)
	return msgLen >= statusMessageTopThreshold
}

func (e *Editor) renderTopStatusMessage(s Screen, w int) {
	if w <= 0 {
		return
	}
	msgRunes := []rune(e.statusMessage)
	if len(msgRunes) > w {
		msgRunes = msgRunes[:w]
	}
	style := e.styleStatus
	if e.externalChange != ExternalChangeNone || e.autoReloadInProgress {
		style = e.styleStatusWarning
	}
	for x := 0; x < w; x++ {
		r := ' '
		if x < len(msgRunes) {
			r = msgRunes[x]
		}
		s.SetContent(x, 0, r, nil, style)
	}
}

func (e *Editor) renderNotification(s Screen, w int, now time.Time) {
	if w <= 0 || e.notificationMessage == "" {
		return
	}
	msgRunes := []rune(e.notificationMessage)
	if len(msgRunes) > w {
		msgRunes = msgRunes[:w]
	}
	style := e.notificationStyle(now)
	for x := 0; x < w; x++ {
		r := ' '
		if x < len(msgRunes) {
			r = msgRunes[x]
		}
		s.SetContent(x, 0, r, nil, style)
	}
}

func (e *Editor) renderStatusline(s Screen, w, y int, showTopMessage bool) {
	mode := "NORMAL"
	if e.mode == ModeInsert {
		mode = "INSERT"
	} else if e.mode == ModeCommand {
		mode = "COMMAND"
	} else if e.mode == ModeBranchPicker {
		mode = "BRANCHES"
	} else if e.mode == ModeSearch {
		mode = "SEARCH"
	} else if e.mode == ModeMerge {
		mode = "MERGE"
	}
	name := e.filename
	if name == "" {
		name = "[No Name]"
	} else {
		// Show relative path from cwd if possible
		if cwd, err := os.Getwd(); err == nil {
			absName := name
			if !filepath.IsAbs(name) {
				absName, _ = filepath.Abs(name)
			}
			if rel, err := filepath.Rel(cwd, absName); err == nil && !strings.HasPrefix(rel, "..") {
				name = rel
			} else {
				name = filepath.Base(name)
			}
		} else {
			name = filepath.Base(name)
		}
	}
	flags := ""
	if e.dirty {
		flags += "[*]"
	}
	if e.externalChange != ExternalChangeNone {
		flags += "[!]"
	}

	// Buffer indicator: [2/5] when multiple buffers are open
	bufIndicator := ""
	if e.buffers != nil && e.buffers.Count() > 1 {
		bufIndicator = fmt.Sprintf("[%d/%d]", e.buffers.ActiveIndex()+1, e.buffers.Count())
	}

	msg := e.statusMessage
	if showTopMessage {
		msg = ""
	}
	namePart := name
	if bufIndicator != "" {
		namePart = bufIndicator + " " + name
	}
	status := fmt.Sprintf(" %s | %s %s", mode, namePart, flags)
	if msg != "" {
		status = fmt.Sprintf(" %s | %s %s | %s ", mode, namePart, flags, msg)
	}
	row := e.cursor.Row + 1
	col := 1
	if e.cursor.Row >= 0 && e.cursor.Row < e.LineCount() {
		col = visualCol(e.text.Line(e.cursor.Row), e.cursor.Col, e.tabWidth) + 1
	}

	// Build right part, tracking branch position for styling
	rightParts := []string{fmt.Sprintf(" Ln %d, Col %d", row, col)}
	branchText := ""
	if e.gitBranch != "" {
		branchText = formatGitBranch(e.gitBranchSymbol, e.gitBranch)
		rightParts = append(rightParts, branchText)
	}
	layoutText := ""
	if e.layoutName != "" {
		layoutText = e.layoutName + " "
		rightParts = append(rightParts, layoutText)
	}
	right := strings.Join(rightParts, " | ")

	line := composeStatusLine(status, right, w)
	lineStr := string(line)
	statusMsgStart := -1
	statusMsgEnd := -1
	if msg != "" {
		if idx := strings.LastIndex(lineStr, msg); idx >= 0 {
			statusMsgStart = utf8.RuneCountInString(lineStr[:idx])
			statusMsgEnd = statusMsgStart + utf8.RuneCountInString(msg)
		}
	}

	// Find branch position in the composed line (using rune indices)
	branchStart := -1
	branchEnd := -1
	if branchText != "" {
		branchRunes := []rune(branchText)
		if idx := strings.Index(lineStr, branchText); idx >= 0 {
			// Convert byte index to rune index
			branchStart = utf8.RuneCountInString(lineStr[:idx])
			branchEnd = branchStart + len(branchRunes)
		}
	}

	// Find layout position in the composed line (using rune indices)
	layoutStart := -1
	layoutEnd := -1
	if layoutText != "" {
		layoutRunes := []rune(layoutText)
		if idx := strings.Index(lineStr, layoutText); idx >= 0 {
			layoutStart = utf8.RuneCountInString(lineStr[:idx])
			layoutEnd = layoutStart + len(layoutRunes)
		}
	}

	// Choose branch style based on whether it's the main branch
	branchStyle := e.styleBranch
	if e.IsMainBranch() {
		branchStyle = e.styleMainBranch
	}

	// Choose layout style based on layout name
	layoutStyle := e.styleLayoutOther
	switch {
	case strings.HasPrefix(e.layoutName, "EN"):
		layoutStyle = e.styleLayoutUS
	case strings.HasPrefix(e.layoutName, "RU"):
		layoutStyle = e.styleLayoutRU
	}

	for x, r := range line {
		if x >= w {
			break
		}
		style := e.styleStatus
		if branchStart >= 0 && x >= branchStart && x < branchEnd {
			style = branchStyle
		} else if layoutStart >= 0 && x >= layoutStart && x < layoutEnd {
			style = layoutStyle
		} else if statusMsgStart >= 0 && x >= statusMsgStart && x < statusMsgEnd && (e.externalChange != ExternalChangeNone || e.autoReloadInProgress) {
			style = e.styleStatusWarning
		}
		s.SetContent(x, y, r, nil, style)
	}
}
