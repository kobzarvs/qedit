package editor

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

type diffWordRange struct {
	start int
	end   int
}

type gitDiffPreviewLine struct {
	text       []rune
	sign       rune
	kind       conflictLineKind
	oldLine    int
	newLine    int
	actualLine int
	wordRanges []diffWordRange
}

type gitDiffPreviewHunk struct {
	rowStart int
	rowEnd   int
	oldStart int
	oldCount int
	newStart int
	newCount int
}

type gitDiffPreviewState struct {
	path           string
	lines          []gitDiffPreviewLine
	hunks          []gitDiffPreviewHunk
	changesVersion uint64
	oldDigits      int
	newDigits      int
	active         bool
}

type gitDiffPreviewCursorAnchor struct {
	sign       rune
	oldLine    int
	newLine    int
	actualLine int
	text       string
	valid      bool
}

type parsedGitDiffLine struct {
	sign rune
	text string
}

type parsedGitDiffHunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	lines    []parsedGitDiffLine
}

var unifiedGitHunkHeaderRe = regexp.MustCompile(`@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$`)

func buildGitDiffPreview(path, patch, content string, untracked bool) (gitDiffPreviewState, bool) {
	if strings.TrimSpace(patch) == "" && untracked {
		return buildUntrackedGitDiffPreview(path, content)
	}

	hunks := parseGitDiffHunks(patch)
	if len(hunks) == 0 {
		return gitDiffPreviewState{}, false
	}

	currentLines := diffPreviewContentLines(content)
	lines, previewHunks := buildGitDiffPreviewLines(currentLines, hunks)
	if len(lines) == 0 {
		return gitDiffPreviewState{}, false
	}
	return newGitDiffPreviewState(path, lines, previewHunks), true
}

func buildUntrackedGitDiffPreview(path, content string) (gitDiffPreviewState, bool) {
	currentLines := diffPreviewContentLines(content)
	if len(currentLines) == 0 {
		currentLines = []string{""}
	}
	lines := make([]gitDiffPreviewLine, 0, len(currentLines))
	hunks := []gitDiffPreviewHunk{{
		rowStart: 0,
		rowEnd:   0,
		oldStart: 0,
		oldCount: 0,
		newStart: 1,
	}}
	for i, part := range currentLines {
		lines = append(lines, gitDiffPreviewLine{
			text:       []rune(part),
			sign:       '+',
			kind:       conflictRemote,
			newLine:    i + 1,
			actualLine: i,
		})
	}
	hunks[0].newCount = len(currentLines)
	hunks[0].rowEnd = len(lines) - 1
	return newGitDiffPreviewState(path, lines, hunks), true
}

func parseGitDiffHunks(patch string) []parsedGitDiffHunk {
	scanner := bufio.NewScanner(strings.NewReader(patch))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	hunks := make([]parsedGitDiffHunk, 0, 4)
	current := -1
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "@@ ") {
			matches := unifiedGitHunkHeaderRe.FindStringSubmatch(line)
			if len(matches) < 5 {
				current = -1
				continue
			}
			oldStart, err1 := strconv.Atoi(matches[1])
			newStart, err2 := strconv.Atoi(matches[3])
			if err1 != nil || err2 != nil {
				current = -1
				continue
			}
			oldCount := 1
			if matches[2] != "" {
				if v, err := strconv.Atoi(matches[2]); err == nil {
					oldCount = v
				}
			}
			newCount := 1
			if matches[4] != "" {
				if v, err := strconv.Atoi(matches[4]); err == nil {
					newCount = v
				}
			}
			hunks = append(hunks, parsedGitDiffHunk{
				oldStart: oldStart,
				oldCount: oldCount,
				newStart: newStart,
				newCount: newCount,
			})
			current = len(hunks) - 1
			continue
		}
		if current < 0 || len(line) == 0 {
			continue
		}
		switch line[0] {
		case ' ', '+', '-':
			hunks[current].lines = append(hunks[current].lines, parsedGitDiffLine{
				sign: rune(line[0]),
				text: line[1:],
			})
		}
	}
	return hunks
}

func buildGitDiffPreviewLines(currentLines []string, hunks []parsedGitDiffHunk) ([]gitDiffPreviewLine, []gitDiffPreviewHunk) {
	lines := make([]gitDiffPreviewLine, 0, len(currentLines)+len(hunks)*4)
	previewHunks := make([]gitDiffPreviewHunk, 0, len(hunks))

	currentIdx := 0
	oldLine := 1
	newLine := 1

	appendUnchanged := func(text string, actualLine int) {
		lines = append(lines, gitDiffPreviewLine{
			text:       []rune(text),
			sign:       ' ',
			kind:       conflictNone,
			oldLine:    oldLine,
			newLine:    newLine,
			actualLine: actualLine,
		})
		oldLine++
		newLine++
	}

	for _, hunk := range hunks {
		targetIdx := hunk.newStart - 1
		if targetIdx < currentIdx {
			targetIdx = currentIdx
		}
		if targetIdx > len(currentLines) {
			targetIdx = len(currentLines)
		}
		for currentIdx < targetIdx {
			appendUnchanged(currentLines[currentIdx], currentIdx)
			currentIdx++
		}

		rowStart := len(lines)

		deletes := make([]gitDiffPreviewLine, 0, 4)
		adds := make([]gitDiffPreviewLine, 0, 4)
		flushChangeGroup := func() {
			if len(deletes) == 0 && len(adds) == 0 {
				return
			}
			applyInlineWordDiff(deletes, adds)
			lines = append(lines, deletes...)
			lines = append(lines, adds...)
			deletes = deletes[:0]
			adds = adds[:0]
		}

		for _, op := range hunk.lines {
			switch op.sign {
			case ' ':
				flushChangeGroup()
				text := op.text
				actualLine := -1
				if currentIdx < len(currentLines) {
					text = currentLines[currentIdx]
					actualLine = currentIdx
					currentIdx++
				}
				appendUnchanged(text, actualLine)
			case '-':
				deletes = append(deletes, gitDiffPreviewLine{
					text:       []rune(op.text),
					sign:       '-',
					kind:       conflictLocal,
					oldLine:    oldLine,
					actualLine: -1,
				})
				oldLine++
			case '+':
				text := op.text
				actualLine := -1
				if currentIdx < len(currentLines) {
					text = currentLines[currentIdx]
					actualLine = currentIdx
					currentIdx++
				}
				adds = append(adds, gitDiffPreviewLine{
					text:       []rune(text),
					sign:       '+',
					kind:       conflictRemote,
					newLine:    newLine,
					actualLine: actualLine,
				})
				newLine++
			}
		}
		flushChangeGroup()

		rowEnd := len(lines) - 1
		previewHunks = append(previewHunks, gitDiffPreviewHunk{
			rowStart: rowStart,
			rowEnd:   rowEnd,
			oldStart: hunk.oldStart,
			oldCount: hunk.oldCount,
			newStart: hunk.newStart,
			newCount: hunk.newCount,
		})
	}

	for currentIdx < len(currentLines) {
		appendUnchanged(currentLines[currentIdx], currentIdx)
		currentIdx++
	}

	return lines, previewHunks
}

func applyInlineWordDiff(deletes, adds []gitDiffPreviewLine) {
	pairCount := len(deletes)
	if len(adds) < pairCount {
		pairCount = len(adds)
	}
	for i := 0; i < pairCount; i++ {
		delRanges, addRanges := diffWordRanges(deletes[i].text, adds[i].text)
		deletes[i].wordRanges = delRanges
		adds[i].wordRanges = addRanges
	}
}

func diffWordRanges(a, b []rune) ([]diffWordRange, []diffWordRange) {
	prefix := commonPrefixLen(a, b)
	suffix := commonSuffixLen(a[prefix:], b[prefix:])

	aEnd := len(a) - suffix
	bEnd := len(b) - suffix
	if aEnd < prefix {
		aEnd = prefix
	}
	if bEnd < prefix {
		bEnd = prefix
	}

	var aRanges []diffWordRange
	if prefix < aEnd {
		start, end := expandDiffRangeToWordBoundary(a, prefix, aEnd)
		if start < end {
			aRanges = append(aRanges, diffWordRange{start: start, end: end})
		}
	}
	var bRanges []diffWordRange
	if prefix < bEnd {
		start, end := expandDiffRangeToWordBoundary(b, prefix, bEnd)
		if start < end {
			bRanges = append(bRanges, diffWordRange{start: start, end: end})
		}
	}
	return aRanges, bRanges
}

func commonPrefixLen(a, b []rune) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

func commonSuffixLen(a, b []rune) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[len(a)-1-i] == b[len(b)-1-i] {
		i++
	}
	return i
}

func expandDiffRangeToWordBoundary(line []rune, start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if end > len(line) {
		end = len(line)
	}
	for start > 0 && start < len(line) && isWordRune(line[start-1]) && isWordRune(line[start]) {
		start--
	}
	for end > 0 && end < len(line) && isWordRune(line[end-1]) && isWordRune(line[end]) {
		end++
	}
	return start, end
}

func diffPreviewContentLines(content string) []string {
	if content == "" {
		return nil
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.Split(content, "\n")
}

func newGitDiffPreviewState(path string, lines []gitDiffPreviewLine, hunks []gitDiffPreviewHunk) gitDiffPreviewState {
	maxOld := 1
	maxNew := 1
	for _, line := range lines {
		if line.oldLine > maxOld {
			maxOld = line.oldLine
		}
		if line.newLine > maxNew {
			maxNew = line.newLine
		}
	}
	oldDigits := len(strconv.Itoa(maxOld))
	newDigits := len(strconv.Itoa(maxNew))
	if oldDigits < 2 {
		oldDigits = 2
	}
	if newDigits < 2 {
		newDigits = 2
	}
	return gitDiffPreviewState{
		path:      path,
		lines:     lines,
		hunks:     hunks,
		oldDigits: oldDigits,
		newDigits: newDigits,
	}
}

func (e *Editor) gitDiffPreviewActive() bool {
	if !e.git.diffPreview.active || e.mode != ModeMerge || e.hasConflictBlocks() || e.document.filename == "" || e.document.dirty {
		return false
	}
	currentPath := e.normalizedPath(e.document.filename)
	if currentPath == "" {
		currentPath = e.document.filename
	}
	return currentPath != "" && currentPath == e.git.diffPreview.path && len(e.git.diffPreview.lines) > 0
}

func (e *Editor) syncGitDiffPreview() {
	if e.mode != ModeMerge || e.hasConflictBlocks() || e.document.filename == "" || e.document.dirty || !e.gitDiffHasCurrentFileHunks() {
		if e.git.diffPreview.active {
			e.deactivateGitDiffPreview()
		}
		return
	}
	if !e.ensureGitDiffPreview() {
		return
	}
	if !e.git.diffPreview.active {
		e.activateGitDiffPreview()
	}
}

func (e *Editor) ensureGitDiffPreview() bool {
	currentPath := e.normalizedPath(e.document.filename)
	if currentPath == "" {
		currentPath = e.document.filename
	}
	if currentPath == "" {
		return false
	}
	if e.git.diffPreview.path == currentPath && e.git.diffPreview.changesVersion == e.git.changesVersion && len(e.git.diffPreview.lines) > 0 {
		return true
	}

	root := e.git.root
	if root == "" {
		root = e.detectGitRoot()
		if root != "" {
			e.git.root = root
		}
	}
	if root == "" {
		return false
	}

	wasActive := e.git.diffPreview.active
	anchor := gitDiffPreviewCursorAnchor{}
	anchorCol := e.cursor.Col
	if wasActive {
		anchor = e.gitDiffPreviewCursorAnchor()
		if anchor.actualLine >= 0 {
			anchorCol = e.cursor.Col
		}
	}

	patch, err := e.gitDiffPatch(root, currentPath)
	if err != nil {
		e.setStatus(err.Error())
		return false
	}
	preview, ok := buildGitDiffPreview(currentPath, patch, e.Content(), e.gitChangeStatus(currentPath) == "?")
	if !ok {
		e.clearGitDiffPreview()
		return false
	}
	preview.path = currentPath
	preview.changesVersion = e.git.changesVersion
	preview.active = wasActive
	e.git.diffPreview = preview

	if wasActive {
		row := e.gitDiffPreviewRowForAnchor(anchor)
		if row < 0 {
			row = e.gitDiffPreviewRowForHighlight()
		}
		if row < 0 {
			row = 0
		}
		e.cursor.Row = row
		e.cursor.Col = anchorCol
		e.clampCursorCol()
	}
	return true
}

func (e *Editor) gitDiffPreviewCursorAnchor() gitDiffPreviewCursorAnchor {
	if len(e.git.diffPreview.lines) == 0 {
		return gitDiffPreviewCursorAnchor{}
	}
	row := e.cursor.Row
	if row < 0 {
		row = 0
	}
	if row >= len(e.git.diffPreview.lines) {
		row = len(e.git.diffPreview.lines) - 1
	}
	line := e.git.diffPreview.lines[row]
	return gitDiffPreviewCursorAnchor{
		sign:       line.sign,
		oldLine:    line.oldLine,
		newLine:    line.newLine,
		actualLine: line.actualLine,
		text:       string(line.text),
		valid:      true,
	}
}

func (e *Editor) gitDiffPreviewRowForAnchor(anchor gitDiffPreviewCursorAnchor) int {
	if !anchor.valid || len(e.git.diffPreview.lines) == 0 {
		return -1
	}

	if anchor.sign == '-' && anchor.oldLine > 0 {
		for i, line := range e.git.diffPreview.lines {
			if line.sign == '-' && line.oldLine == anchor.oldLine {
				if anchor.text == "" || string(line.text) == anchor.text {
					return i
				}
			}
		}
	}

	if anchor.sign == '+' && anchor.newLine > 0 {
		for i, line := range e.git.diffPreview.lines {
			if line.sign == '+' && line.newLine == anchor.newLine {
				if anchor.text == "" || string(line.text) == anchor.text {
					return i
				}
			}
		}
	}

	if anchor.actualLine >= 0 {
		return e.gitDiffPreviewRowForActual(anchor.actualLine)
	}

	for i, line := range e.git.diffPreview.lines {
		if line.sign == anchor.sign && line.oldLine == anchor.oldLine && line.newLine == anchor.newLine {
			if anchor.text == "" || string(line.text) == anchor.text {
				return i
			}
		}
	}

	return -1
}

func (e *Editor) activateGitDiffPreview() bool {
	if !e.ensureGitDiffPreview() {
		return false
	}
	if e.git.diffPreview.active {
		return true
	}
	row := e.gitDiffPreviewRowForHighlight()
	if row < 0 {
		row = e.gitDiffPreviewRowForActual(e.cursor.Row)
	}
	if row < 0 {
		row = 0
	}
	e.git.diffPreview.active = true
	e.cursor.Row = row
	if e.cursor.Col > e.lineLen(row) {
		e.cursor.Col = e.lineLen(row)
	}
	return true
}

func (e *Editor) deactivateGitDiffPreview() bool {
	if !e.git.diffPreview.active {
		return false
	}
	row, col := e.gitDiffPreviewActualCursor()
	e.git.diffPreview.active = false
	e.cursor.Row = row
	e.cursor.Col = col
	e.clampCursorCol()
	return true
}

func (e *Editor) clearGitDiffPreview() {
	if e.git.diffPreview.active {
		e.deactivateGitDiffPreview()
	}
	e.git.diffPreview = gitDiffPreviewState{}
}

func (e *Editor) gitDiffPreviewRowForHighlight() int {
	highlight := e.git.diffHighlight
	if highlight == nil {
		return -1
	}
	for _, hunk := range e.git.diffPreview.hunks {
		rangeStart := hunk.newStart - 1
		if rangeStart < 0 {
			rangeStart = 0
		}
		rangeEnd := rangeStart
		if hunk.newCount > 0 {
			rangeEnd = rangeStart + hunk.newCount - 1
		}
		if rangeStart == highlight.StartLine && rangeEnd == highlight.EndLine {
			if hunk.rowStart <= hunk.rowEnd {
				return hunk.rowStart
			}
			if hunk.rowStart < len(e.git.diffPreview.lines) {
				return hunk.rowStart
			}
			if len(e.git.diffPreview.lines) > 0 {
				return len(e.git.diffPreview.lines) - 1
			}
			return -1
		}
	}
	return -1
}

func (e *Editor) gitDiffPreviewRowForActual(actualRow int) int {
	if len(e.git.diffPreview.lines) == 0 {
		return -1
	}
	lastMatch := -1
	for i, line := range e.git.diffPreview.lines {
		if line.actualLine < 0 {
			continue
		}
		if line.actualLine >= actualRow {
			return i
		}
		lastMatch = i
	}
	if lastMatch >= 0 {
		return lastMatch
	}
	return 0
}

func (e *Editor) gitDiffPreviewActualCursor() (int, int) {
	if len(e.git.diffPreview.lines) == 0 {
		return 0, 0
	}
	row := e.cursor.Row
	if row < 0 {
		row = 0
	}
	if row >= len(e.git.diffPreview.lines) {
		row = len(e.git.diffPreview.lines) - 1
	}
	if actual := e.git.diffPreview.lines[row].actualLine; actual >= 0 {
		return actual, e.cursor.Col
	}
	for i := row + 1; i < len(e.git.diffPreview.lines); i++ {
		if actual := e.git.diffPreview.lines[i].actualLine; actual >= 0 {
			return actual, e.cursor.Col
		}
	}
	for i := row - 1; i >= 0; i-- {
		if actual := e.git.diffPreview.lines[i].actualLine; actual >= 0 {
			return actual, e.cursor.Col
		}
	}
	return 0, 0
}

func (e *Editor) gitDiffPreviewLine(row int) (gitDiffPreviewLine, bool) {
	if !e.gitDiffPreviewActive() || row < 0 || row >= len(e.git.diffPreview.lines) {
		return gitDiffPreviewLine{}, false
	}
	return e.git.diffPreview.lines[row], true
}

func (e *Editor) gitDiffPreviewGutterWidth() int {
	if !e.gitDiffPreviewActive() {
		return 0
	}
	return 1 + e.git.diffPreview.oldDigits + 1 + 1 + e.git.diffPreview.newDigits + 1 + 1
}

func (e *Editor) gitDiffChangeStatus(absPath string) GitFileChange {
	target := absPath
	if normalized := e.normalizedPath(absPath); normalized != "" {
		target = normalized
	}
	for _, change := range e.git.changes {
		path := change.AbsPath
		if normalized := e.normalizedPath(change.AbsPath); normalized != "" {
			path = normalized
		}
		if path == target {
			return change
		}
	}
	return GitFileChange{}
}

func (e *Editor) gitChangeStatus(absPath string) string {
	return e.gitDiffChangeStatus(absPath).Status
}

func (e *Editor) gitDiffWordHighlighted(lineIdx, col int) bool {
	line, ok := e.gitDiffPreviewLine(lineIdx)
	if !ok || len(line.wordRanges) == 0 {
		return false
	}
	for _, wordRange := range line.wordRanges {
		if col >= wordRange.start && col < wordRange.end {
			return true
		}
	}
	return false
}

func (e *Editor) gitDiffPreviewAllowsAction(action string) bool {
	switch action {
	case actionMoveLeft, actionMoveRight, actionMoveUp, actionMoveDown,
		actionWordLeft, actionWordRight, actionLineStart, actionLineEnd,
		actionFileStart, actionFileEnd, actionPageUp, actionPageDown,
		actionToggleLineNumbers,
		actionBranchPicker, actionWorktreeMenu, actionWorktreeNew,
		actionWorktreeSwitch, actionWorktreeRemove, actionWorktreeRefresh,
		actionOpenFileTree, actionToggleSidebar, actionToggleSidebarFocus,
		actionFocusSidebar, actionFocusPrevPane, actionFocusNextPane,
		actionFocusEditor, actionFocusCommandLine,
		actionEnterCommand, actionMergeMode, actionQuit,
		actionScrollUp, actionScrollDown,
		actionWordForward, actionWordBackward, actionWordEnd,
		actionGotoMode, actionGotoLine, actionGotoLinePrompt,
		actionGotoFirstLine, actionGotoFileEnd,
		actionFindChar, actionFindCharBackward, actionTillChar, actionTillCharBackward,
		actionYank, actionToggleSelect, actionExtendLine, actionCollapseSelection,
		actionFlipSelection, actionSpaceMode, actionMatchMode, actionViewMode,
		actionSearchForward, actionSearchBackward, actionSearchFuzzy,
		actionSearchRegex, actionSearchNext, actionSearchPrev,
		actionGitNextChange, actionGitPrevChange,
		actionTerminalZoomIn, actionExpandSelection, actionShrinkSelection,
		actionSave, actionBufferPicker, actionGotoNextBuffer,
		actionGotoPrevBuffer, actionGotoLastAccessed, actionCloseBuffer:
		return true
	default:
		return false
	}
}
