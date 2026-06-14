package editor

import (
	"fmt"
	"strings"
)

type editorWindowAxis int

const (
	editorWindowLeaf editorWindowAxis = iota
	editorWindowVertical
	editorWindowHorizontal
)

type editorWindowDirection int

const (
	editorWindowLeft editorWindowDirection = iota
	editorWindowDown
	editorWindowUp
	editorWindowRight
)

type editorWindowState struct {
	root     *editorWindowNode
	activeID int
	nextID   int
}

type editorWindowView struct {
	cursor  Cursor
	scroll  int
	scrollX int
}

type editorWindowNode struct {
	parent      *editorWindowNode
	axis        editorWindowAxis
	first       *editorWindowNode
	second      *editorWindowNode
	id          int
	bufferIndex int
	view        editorWindowView
}

type editorWindowLayout struct {
	id          int
	bufferIndex int
	x           int
	y           int
	w           int
	h           int
}

func newEditorWindowState() editorWindowState {
	root := &editorWindowNode{id: 1, bufferIndex: 0}
	return editorWindowState{root: root, activeID: 1, nextID: 2}
}

func (n *editorWindowNode) isLeaf() bool {
	return n != nil && n.axis == editorWindowLeaf
}

func (e *Editor) ensureWindowState() {
	if e.windows.root == nil {
		e.windows = newEditorWindowState()
	}
	if e.windows.activeID == 0 {
		if leaf := firstWindowLeaf(e.windows.root); leaf != nil {
			e.windows.activeID = leaf.id
		}
	}
	if e.windows.nextID <= 0 {
		maxID := 0
		var leaves []*editorWindowNode
		collectWindowLeaves(e.windows.root, &leaves)
		for _, leaf := range leaves {
			if leaf.id > maxID {
				maxID = leaf.id
			}
		}
		e.windows.nextID = maxID + 1
	}
}

func (e *Editor) ensureActiveBufferManaged() int {
	if e.buffers == nil {
		return 0
	}
	if e.buffers.Count() == 0 {
		e.buffers.Add(e.snapshotBufferState())
	}
	idx := e.buffers.ActiveIndex()
	e.ensureWindowState()
	if leaf := e.activeWindowLeaf(); leaf != nil {
		if leaf.bufferIndex < 0 || leaf.bufferIndex >= e.buffers.Count() {
			leaf.bufferIndex = idx
		}
	}
	return idx
}

func (e *Editor) activeWindowLeaf() *editorWindowNode {
	e.ensureWindowState()
	return findWindowLeaf(e.windows.root, e.windows.activeID)
}

func findWindowLeaf(n *editorWindowNode, id int) *editorWindowNode {
	if n == nil {
		return nil
	}
	if n.isLeaf() {
		if n.id == id {
			return n
		}
		return nil
	}
	if found := findWindowLeaf(n.first, id); found != nil {
		return found
	}
	return findWindowLeaf(n.second, id)
}

func firstWindowLeaf(n *editorWindowNode) *editorWindowNode {
	if n == nil {
		return nil
	}
	if n.isLeaf() {
		return n
	}
	if leaf := firstWindowLeaf(n.first); leaf != nil {
		return leaf
	}
	return firstWindowLeaf(n.second)
}

func (e *Editor) windowLeaves() []*editorWindowNode {
	e.ensureWindowState()
	var leaves []*editorWindowNode
	collectWindowLeaves(e.windows.root, &leaves)
	return leaves
}

func collectWindowLeaves(n *editorWindowNode, leaves *[]*editorWindowNode) {
	if n == nil {
		return
	}
	if n.isLeaf() {
		*leaves = append(*leaves, n)
		return
	}
	collectWindowLeaves(n.first, leaves)
	collectWindowLeaves(n.second, leaves)
}

func (e *Editor) windowCount() int {
	return len(e.windowLeaves())
}

func (e *Editor) setActiveWindowBufferIndex(index int) {
	if index < 0 {
		return
	}
	e.ensureWindowState()
	if leaf := e.activeWindowLeaf(); leaf != nil {
		leaf.bufferIndex = index
	}
}

func (e *Editor) saveWindowView(leaf *editorWindowNode) {
	if leaf == nil || !leaf.isLeaf() {
		return
	}
	leaf.view.cursor = e.cursor
	leaf.view.scroll = e.viewport.scroll
	leaf.view.scrollX = e.viewport.scrollX
}

func (e *Editor) loadWindowView(leaf *editorWindowNode) {
	if leaf == nil || !leaf.isLeaf() {
		return
	}
	e.cursor = leaf.view.cursor
	e.viewport.scroll = leaf.view.scroll
	e.viewport.scrollX = leaf.view.scrollX
	if e.LineCount() > 0 && e.cursor.Row >= e.LineCount() {
		e.cursor.Row = e.LineCount() - 1
	}
	e.clampCursorCol()
}

func (e *Editor) syncActiveWindowView() {
	e.saveWindowView(e.activeWindowLeaf())
}

func (e *Editor) activePaneLayout() (editorWindowLayout, bool) {
	if e.windowCount() <= 1 {
		return editorWindowLayout{}, false
	}
	for _, layout := range e.navigationWindowLayouts() {
		if layout.id == e.windows.activeID {
			return layout, true
		}
	}
	return editorWindowLayout{}, false
}

func (e *Editor) syncActivePaneViewportSize() {
	if layout, ok := e.activePaneLayout(); ok {
		if layout.h > 0 {
			e.viewport.height = layout.h
		}
		if layout.w > 0 {
			e.viewport.width = layout.w
		}
	}
}

// paneViewHeight returns the visible line count for the active split pane.
func (e *Editor) paneViewHeight() int {
	if layout, ok := e.activePaneLayout(); ok && layout.h > 0 {
		return layout.h
	}
	h := e.viewport.height
	if h < 1 {
		return 1
	}
	return h
}

func (e *Editor) afterEditorViewChanged() {
	if e.windowCount() <= 1 {
		return
	}
	e.syncActivePaneViewportSize()
	e.ensureCursorVisible(e.paneViewHeight())
	e.syncActiveWindowView()
}

func (e *Editor) focusWindowByID(id int) bool {
	e.ensureActiveBufferManaged()
	leaf := findWindowLeaf(e.windows.root, id)
	if leaf == nil {
		e.setStatus("window not found")
		return false
	}
	if current := e.activeWindowLeaf(); current != nil && current.id != leaf.id {
		if e.buffers != nil && e.buffers.Count() > 0 {
			e.syncActiveBufferContent()
		}
		e.saveWindowView(current)
	}
	e.windows.activeID = leaf.id
	if e.buffers != nil && leaf.bufferIndex >= 0 && leaf.bufferIndex < e.buffers.Count() {
		if leaf.bufferIndex != e.buffers.ActiveIndex() {
			e.switchToBufferForWindowFocus(leaf.bufferIndex)
		}
	}
	e.loadWindowView(leaf)
	e.syncActivePaneViewportSize()
	e.clampCursorCol()
	e.ensureCursorVisible(e.paneViewHeight())
	return true
}

func (e *Editor) focusNextWindow() {
	leaves := e.windowLeaves()
	if len(leaves) <= 1 {
		e.setStatus("single window")
		return
	}
	active := e.windows.activeID
	for i, leaf := range leaves {
		if leaf.id == active {
			next := leaves[(i+1)%len(leaves)]
			e.focusWindowByID(next.id)
			e.setStatus("window " + fmt.Sprint((i+1)%len(leaves)+1) + "/" + fmt.Sprint(len(leaves)))
			return
		}
	}
	e.focusWindowByID(leaves[0].id)
}

func (e *Editor) focusDirectionalWindow(dir editorWindowDirection) {
	target := e.directionalWindowTarget(dir)
	if target == nil {
		e.setStatus("no window in that direction")
		return
	}
	e.focusWindowByID(target.id)
}

func (e *Editor) directionalWindowTarget(dir editorWindowDirection) *editorWindowLayout {
	layouts := e.navigationWindowLayouts()
	var active *editorWindowLayout
	for i := range layouts {
		if layouts[i].id == e.windows.activeID {
			active = &layouts[i]
			break
		}
	}
	if active == nil {
		return nil
	}
	var best *editorWindowLayout
	bestPrimary := 1 << 30
	bestSecondary := 1 << 30
	for i := range layouts {
		candidate := &layouts[i]
		if candidate.id == active.id {
			continue
		}
		primary, secondary, ok := windowDirectionDistance(*active, *candidate, dir)
		if !ok {
			continue
		}
		if primary < bestPrimary || (primary == bestPrimary && secondary < bestSecondary) {
			bestPrimary = primary
			bestSecondary = secondary
			best = candidate
		}
	}
	return best
}

func windowDirectionDistance(active, candidate editorWindowLayout, dir editorWindowDirection) (int, int, bool) {
	activeRight := active.x + active.w
	activeBottom := active.y + active.h
	candidateRight := candidate.x + candidate.w
	candidateBottom := candidate.y + candidate.h
	activeCenterX := active.x + active.w/2
	activeCenterY := active.y + active.h/2
	candidateCenterX := candidate.x + candidate.w/2
	candidateCenterY := candidate.y + candidate.h/2
	switch dir {
	case editorWindowLeft:
		if candidateRight > active.x || !rangesOverlap(active.y, activeBottom, candidate.y, candidateBottom) {
			return 0, 0, false
		}
		return active.x - candidateRight, absInt(activeCenterY - candidateCenterY), true
	case editorWindowRight:
		if candidate.x < activeRight || !rangesOverlap(active.y, activeBottom, candidate.y, candidateBottom) {
			return 0, 0, false
		}
		return candidate.x - activeRight, absInt(activeCenterY - candidateCenterY), true
	case editorWindowUp:
		if candidateBottom > active.y || !rangesOverlap(active.x, activeRight, candidate.x, candidateRight) {
			return 0, 0, false
		}
		return active.y - candidateBottom, absInt(activeCenterX - candidateCenterX), true
	case editorWindowDown:
		if candidate.y < activeBottom || !rangesOverlap(active.x, activeRight, candidate.x, candidateRight) {
			return 0, 0, false
		}
		return candidate.y - activeBottom, absInt(activeCenterX - candidateCenterX), true
	default:
		return 0, 0, false
	}
}

func rangesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	return aStart < bEnd && bStart < aEnd
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (e *Editor) navigationWindowLayouts() []editorWindowLayout {
	w := e.viewport.layoutW
	h := e.viewport.layoutH
	if w <= 0 {
		w = e.viewport.width
	}
	if h <= 0 {
		h = e.viewport.height
	}
	if e.windowCount() <= 1 {
		if w < 80 {
			w = 80
		}
		if h < 24 {
			h = 24
		}
	} else {
		if w <= 0 {
			w = 80
		}
		if h <= 0 {
			h = 24
		}
	}
	return e.windowLayouts(0, 0, w, h)
}

func (e *Editor) splitCurrentWindow(axis editorWindowAxis) {
	idx := e.ensureActiveBufferManaged()
	e.splitWindowWithBuffer(axis, idx, true)
}

func (e *Editor) splitNewWindow(axis editorWindowAxis) {
	e.ensureActiveBufferManaged()
	idx := e.addUntitledBufferState()
	e.splitWindowWithBuffer(axis, idx, true)
}

func (e *Editor) splitWindowWithBuffer(axis editorWindowAxis, bufferIndex int, focusNew bool) {
	e.ensureActiveBufferManaged()
	leaf := e.activeWindowLeaf()
	if leaf == nil {
		e.setStatus("window not found")
		return
	}
	if axis != editorWindowVertical && axis != editorWindowHorizontal {
		return
	}
	if e.viewport.layoutW <= 0 || e.viewport.layoutH <= 0 {
		w, h := e.viewport.width, e.viewport.height
		if w < 80 {
			w = 80
		}
		if h < 24 {
			h = 24
		}
		e.viewport.layoutW = w
		e.viewport.layoutH = h
	}
	e.saveWindowView(leaf)
	splitView := leaf.view
	old := &editorWindowNode{
		parent:      leaf,
		id:          leaf.id,
		bufferIndex: leaf.bufferIndex,
		view:        splitView,
	}
	newLeaf := &editorWindowNode{
		parent:      leaf,
		id:          e.windows.nextID,
		bufferIndex: bufferIndex,
		view:        splitView,
	}
	e.windows.nextID++
	leaf.axis = axis
	leaf.id = 0
	leaf.bufferIndex = 0
	leaf.first = old
	leaf.second = newLeaf
	if focusNew {
		e.focusWindowByID(newLeaf.id)
	} else {
		e.focusWindowByID(old.id)
	}
	if axis == editorWindowVertical {
		e.setStatus("vertical split")
	} else {
		e.setStatus("horizontal split")
	}
}

func (e *Editor) addUntitledBufferState() int {
	if e.buffers == nil {
		return 0
	}
	title := fmt.Sprintf("[No Name %d]", e.buffers.Count()+1)
	bs := newEditorBufferState("", title, "")
	return e.buffers.Add(bs)
}

func newEditorBufferState(filename, title, text string) *BufferState {
	return &BufferState{
		text:      NewTextBufferFromString(text),
		filename:  filename,
		title:     title,
		file:      editorFileState{diskContent: text, diskContentValid: true},
		highlight: editorHighlightState{start: -1, end: -1},
		mode:      ModeNormal,
	}
}

func (e *Editor) openBufferIndexForSplit(target string) (int, bool) {
	current := e.ensureActiveBufferManaged()
	target = strings.TrimSpace(target)
	if target == "" {
		return current, true
	}
	path := target
	if e.workspaceFileStore() != nil {
		if abs, err := e.workspaceAbs(target); err == nil {
			path = abs
		}
	}
	if e.buffers != nil {
		if idx := e.buffers.FindByPath(path); idx >= 0 {
			return idx, true
		}
	}
	data := []byte(nil)
	if e.workspaceFileStore() != nil {
		read, err := e.workspaceRead(path)
		if err != nil {
			if !e.workspaceIsNotExist(err) {
				e.setStatus(err.Error())
				return -1, false
			}
		} else {
			data = read
		}
	}
	bs := newEditorBufferState(path, "", string(data))
	if e.buffers == nil {
		return 0, true
	}
	return e.buffers.Add(bs), true
}

func (e *Editor) executeSplitCommand(args []string, axis editorWindowAxis) bool {
	idx, ok := e.openBufferIndexForSplit(strings.Join(args, " "))
	if !ok {
		return false
	}
	e.splitWindowWithBuffer(axis, idx, true)
	return false
}

func (e *Editor) closeCurrentWindow() {
	e.ensureActiveBufferManaged()
	if e.windowCount() <= 1 {
		e.setStatus("single window")
		return
	}
	leaf := e.activeWindowLeaf()
	if leaf == nil || leaf.parent == nil {
		e.setStatus("window not found")
		return
	}
	parent := leaf.parent
	sibling := parent.first
	if sibling == leaf {
		sibling = parent.second
	}
	replacementFocus := firstWindowLeaf(sibling)
	grand := parent.parent
	if grand == nil {
		e.windows.root = sibling
		sibling.parent = nil
	} else if grand.first == parent {
		grand.first = sibling
		sibling.parent = grand
	} else {
		grand.second = sibling
		sibling.parent = grand
	}
	if replacementFocus != nil {
		e.focusWindowByID(replacementFocus.id)
	}
	e.setStatus("window closed")
}

func (e *Editor) closeOtherWindows() {
	e.ensureActiveBufferManaged()
	leaf := e.activeWindowLeaf()
	if leaf == nil {
		e.setStatus("window not found")
		return
	}
	leaf.parent = nil
	e.windows.root = leaf
	e.windows.activeID = leaf.id
	e.setStatus("only current window")
}

func (e *Editor) swapWindow(dir editorWindowDirection) {
	e.ensureActiveBufferManaged()
	active := e.activeWindowLeaf()
	targetLayout := e.directionalWindowTarget(dir)
	if active == nil || targetLayout == nil {
		e.setStatus("no window in that direction")
		return
	}
	target := findWindowLeaf(e.windows.root, targetLayout.id)
	if target == nil {
		e.setStatus("window not found")
		return
	}
	active.bufferIndex, target.bufferIndex = target.bufferIndex, active.bufferIndex
	e.focusWindowByID(target.id)
	e.setStatus("windows swapped")
}

func (e *Editor) transposeWindowSplit() {
	e.ensureActiveBufferManaged()
	leaf := e.activeWindowLeaf()
	if leaf == nil || leaf.parent == nil {
		e.setStatus("single window")
		return
	}
	if leaf.parent.axis == editorWindowVertical {
		leaf.parent.axis = editorWindowHorizontal
	} else if leaf.parent.axis == editorWindowHorizontal {
		leaf.parent.axis = editorWindowVertical
	}
	e.setStatus("split transposed")
}

func (e *Editor) adjustWindowsAfterBufferRemove(removedIndex int) {
	if e.windows.root == nil || e.buffers == nil {
		return
	}
	activeIndex := e.buffers.ActiveIndex()
	for _, leaf := range e.windowLeaves() {
		switch {
		case leaf.bufferIndex == removedIndex:
			leaf.bufferIndex = activeIndex
		case leaf.bufferIndex > removedIndex:
			leaf.bufferIndex--
		}
	}
	if active := e.activeWindowLeaf(); active != nil {
		if active.bufferIndex < 0 || active.bufferIndex >= e.buffers.Count() {
			active.bufferIndex = activeIndex
		}
	}
}

func (e *Editor) windowLayouts(x, y, w, h int) []editorWindowLayout {
	e.ensureWindowState()
	var layouts []editorWindowLayout
	collectWindowLayouts(e.windows.root, x, y, w, h, &layouts)
	return layouts
}

func collectWindowLayouts(n *editorWindowNode, x, y, w, h int, layouts *[]editorWindowLayout) {
	if n == nil || w <= 0 || h <= 0 {
		return
	}
	if n.isLeaf() {
		*layouts = append(*layouts, editorWindowLayout{
			id:          n.id,
			bufferIndex: n.bufferIndex,
			x:           x,
			y:           y,
			w:           w,
			h:           h,
		})
		return
	}
	switch n.axis {
	case editorWindowVertical:
		if w < 3 {
			collectWindowLayouts(n.first, x, y, w, h, layouts)
			return
		}
		leftW := (w - 1) / 2
		rightW := w - leftW - 1
		collectWindowLayouts(n.first, x, y, leftW, h, layouts)
		collectWindowLayouts(n.second, x+leftW+1, y, rightW, h, layouts)
	case editorWindowHorizontal:
		if h < 3 {
			collectWindowLayouts(n.first, x, y, w, h, layouts)
			return
		}
		topH := (h - 1) / 2
		bottomH := h - topH - 1
		collectWindowLayouts(n.first, x, y, w, topH, layouts)
		collectWindowLayouts(n.second, x, y+topH+1, w, bottomH, layouts)
	default:
		collectWindowLayouts(n.first, x, y, w, h, layouts)
	}
}

func (e *Editor) drawWindowSeparators(s Screen, x, y, w, h int) {
	e.ensureWindowState()
	e.drawWindowNodeSeparators(s, e.windows.root, x, y, w, h)
}

func (e *Editor) drawWindowNodeSeparators(s Screen, n *editorWindowNode, x, y, w, h int) {
	if n == nil || n.isLeaf() || w <= 0 || h <= 0 {
		return
	}
	style := e.styleLineNumber
	if style == nil {
		style = e.styleMain
	}
	switch n.axis {
	case editorWindowVertical:
		if w < 3 {
			return
		}
		leftW := (w - 1) / 2
		rightW := w - leftW - 1
		sepX := x + leftW
		for row := 0; row < h; row++ {
			s.SetContent(sepX, y+row, '|', nil, style)
		}
		e.drawWindowNodeSeparators(s, n.first, x, y, leftW, h)
		e.drawWindowNodeSeparators(s, n.second, x+leftW+1, y, rightW, h)
	case editorWindowHorizontal:
		if h < 3 {
			return
		}
		topH := (h - 1) / 2
		bottomH := h - topH - 1
		sepY := y + topH
		for col := 0; col < w; col++ {
			s.SetContent(x+col, sepY, '-', nil, style)
		}
		e.drawWindowNodeSeparators(s, n.first, x, y, w, topH)
		e.drawWindowNodeSeparators(s, n.second, x, y+topH+1, w, bottomH)
	}
}
