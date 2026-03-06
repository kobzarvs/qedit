package editor

import (
	"fmt"
	"sort"
	"strings"
)

func (e *Editor) renderSpaceMenu(s Screen, w, viewHeight int) {
	if !e.spaceMenuActive {
		return
	}
	if w < 20 || viewHeight < 5 {
		return
	}

	// Find the maximum label width
	maxLabelWidth := 0
	for _, item := range SpaceMenuItems {
		labelWidth := len(item.Label) + 6 // "x   Label"
		if labelWidth > maxLabelWidth {
			maxLabelWidth = labelWidth
		}
	}

	// Box dimensions
	boxWidth := maxLabelWidth + 4
	if boxWidth > w-4 {
		boxWidth = w - 4
	}
	innerWidth := boxWidth - 2

	listHeight := len(SpaceMenuItems)
	if listHeight > viewHeight-3 {
		listHeight = viewHeight - 3
	}
	boxHeight := listHeight + 2

	// Position at bottom right, above status line
	x0 := w - boxWidth - 1
	if x0 < 0 {
		x0 = 0
	}
	y0 := viewHeight - boxHeight
	if y0 < 0 {
		y0 = 0
	}

	borderStyle := e.styleBoxBorder
	if borderStyle == nil {
		borderStyle = e.styleStatus
	}
	itemStyle := e.styleCommand
	dimStyle := e.styleLineNumber // for unimplemented items

	// Draw border
	topLeft := '┌'
	topRight := '┐'
	bottomLeft := '└'
	bottomRight := '┘'
	hLine := '─'
	vLine := '│'

	// Top border with title
	title := "Space"
	titleRunes := []rune(title)

	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = topLeft
		} else if x == boxWidth-1 {
			ch = topRight
		}
		s.SetContent(x0+x, y0, ch, nil, borderStyle)
	}

	// Embed title in top border
	if len(titleRunes)+2 <= boxWidth-2 {
		for i, r := range titleRunes {
			s.SetContent(x0+1+i, y0, r, nil, borderStyle)
		}
	}

	// Bottom border
	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = bottomLeft
		} else if x == boxWidth-1 {
			ch = bottomRight
		}
		s.SetContent(x0+x, y0+boxHeight-1, ch, nil, borderStyle)
	}

	// Side borders and content
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, vLine, nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, vLine, nil, borderStyle)

		// Clear interior
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, itemStyle)
		}
	}

	// Draw menu items
	for i := 0; i < listHeight; i++ {
		if i >= len(SpaceMenuItems) {
			break
		}
		item := SpaceMenuItems[i]
		lineY := y0 + 1 + i

		// Choose style based on whether item is implemented
		style := itemStyle
		if !item.Implemented {
			style = dimStyle
		}

		// Clear line
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, lineY, ' ', nil, style)
		}

		// Format: " k   Label text"
		keyStr := string(item.Key)
		label := " " + keyStr + "   " + item.Label

		runes := []rune(label)
		if len(runes) > innerWidth {
			runes = runes[:innerWidth]
		}

		for j, r := range runes {
			s.SetContent(x0+1+j, lineY, r, nil, style)
		}
	}
}

// renderMenu renders a generic mode menu popup
func (e *Editor) renderMenu(s Screen, w, viewHeight int, title string, items []SpaceMenuItem) {
	if w < 20 || viewHeight < 5 {
		return
	}

	// Find the maximum label width
	maxLabelWidth := 0
	for _, item := range items {
		labelWidth := len(item.Label) + 6 // "x   Label"
		if labelWidth > maxLabelWidth {
			maxLabelWidth = labelWidth
		}
	}

	// Box dimensions
	boxWidth := maxLabelWidth + 4
	if boxWidth > w-4 {
		boxWidth = w - 4
	}
	innerWidth := boxWidth - 2

	listHeight := len(items)
	if listHeight > viewHeight-3 {
		listHeight = viewHeight - 3
	}
	boxHeight := listHeight + 2

	// Position at bottom right, above status line
	x0 := w - boxWidth - 1
	if x0 < 0 {
		x0 = 0
	}
	y0 := viewHeight - boxHeight
	if y0 < 0 {
		y0 = 0
	}

	borderStyle := e.styleBoxBorder
	if borderStyle == nil {
		borderStyle = e.styleStatus
	}
	itemStyle := e.styleCommand
	dimStyle := e.styleLineNumber

	// Draw border
	topLeft := '┌'
	topRight := '┐'
	bottomLeft := '└'
	bottomRight := '┘'
	hLine := '─'
	vLine := '│'

	// Top border with title
	titleRunes := []rune(title)

	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = topLeft
		} else if x == boxWidth-1 {
			ch = topRight
		}
		s.SetContent(x0+x, y0, ch, nil, borderStyle)
	}

	// Embed title in top border
	if len(titleRunes)+2 <= boxWidth-2 {
		for i, r := range titleRunes {
			s.SetContent(x0+1+i, y0, r, nil, borderStyle)
		}
	}

	// Bottom border
	for x := 0; x < boxWidth; x++ {
		ch := hLine
		if x == 0 {
			ch = bottomLeft
		} else if x == boxWidth-1 {
			ch = bottomRight
		}
		s.SetContent(x0+x, y0+boxHeight-1, ch, nil, borderStyle)
	}

	// Side borders and content
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, vLine, nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, vLine, nil, borderStyle)

		// Clear interior
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, itemStyle)
		}
	}

	// Draw menu items
	for i := 0; i < listHeight; i++ {
		if i >= len(items) {
			break
		}
		item := items[i]
		lineY := y0 + 1 + i

		// Choose style based on whether item is implemented
		style := itemStyle
		if !item.Implemented {
			style = dimStyle
		}

		// Clear line
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, lineY, ' ', nil, style)
		}

		// Format: " k   Label text"
		keyStr := string(item.Key)
		label := " " + keyStr + "   " + item.Label

		runes := []rune(label)
		if len(runes) > innerWidth {
			runes = runes[:innerWidth]
		}

		for j, r := range runes {
			s.SetContent(x0+1+j, lineY, r, nil, style)
		}
	}
}

// renderKeybindingsHelp renders a help popup showing all keybindings
func (e *Editor) renderKeybindingsHelp(s Screen, w, viewHeight int) {
	if w < 40 || viewHeight < 10 {
		return
	}

	// Keybinding entry with group info
	type keybinding struct {
		key    string
		action string
		desc   string
		group  string
	}

	// Action to group mapping
	actionGroups := map[string]string{
		// Navigation
		"move_left": "Navigation", "move_right": "Navigation", "move_up": "Navigation", "move_down": "Navigation",
		"word_left": "Navigation", "word_right": "Navigation", "word_forward": "Navigation", "word_backward": "Navigation", "word_end": "Navigation",
		"line_start": "Navigation", "line_end": "Navigation", "file_start": "Navigation", "file_end": "Navigation",
		"page_up": "Navigation", "page_down": "Navigation", "scroll_up": "Navigation", "scroll_down": "Navigation",
		// Editing
		"delete": "Editing", "change": "Editing", "yank": "Editing", "paste": "Editing", "paste_before": "Editing",
		"open_below": "Editing", "open_above": "Editing", "append": "Editing", "append_line_end": "Editing",
		"insert_line_start": "Editing", "join_lines": "Editing", "replace_char": "Editing", "delete_line": "Editing",
		"indent": "Editing", "unindent": "Editing", "insert_line_above": "Editing",
		// Selection
		"toggle_select": "Selection", "extend_line": "Selection", "collapse_selection": "Selection", "select_all": "Selection",
		// Search
		"search_forward": "Search", "search_backward": "Search", "search_next": "Search", "search_prev": "Search",
		"find_char": "Search", "find_char_backward": "Search", "till_char": "Search", "till_char_backward": "Search",
		// Git
		"git_next_change": "Git", "git_prev_change": "Git",
		"worktree_menu": "Git", "worktree_new": "Git", "worktree_switch": "Git", "worktree_remove": "Git", "worktree_refresh": "Git",
		// Modes
		"enter_insert": "Modes", "enter_command": "Modes", "goto_mode": "Modes", "match_mode": "Modes",
		"view_mode": "Modes", "space_mode": "Modes", "merge_mode": "Modes",
		// History
		"undo": "History", "redo": "History",
		// Other
		"quit": "Other", "branch_picker": "Other", "toggle_line_numbers": "Other",
	}

	// Action descriptions
	bindingDescs := map[string]string{
		"move_left": "Move cursor left", "move_right": "Move cursor right",
		"move_up": "Move cursor up", "move_down": "Move cursor down",
		"word_left": "Move to previous word", "word_right": "Move to next word",
		"word_forward": "Move to next word", "word_backward": "Move to previous word", "word_end": "Move to word end",
		"line_start": "Move to line start", "line_end": "Move to line end",
		"file_start": "Move to file start", "file_end": "Move to file end",
		"page_up": "Page up", "page_down": "Page down",
		"scroll_up": "Scroll up", "scroll_down": "Scroll down",
		"enter_insert": "Enter insert mode", "enter_command": "Enter command mode",
		"quit": "Quit editor", "undo": "Undo", "redo": "Redo",
		"delete": "Delete selection", "change": "Change (delete + insert)",
		"yank": "Yank (copy)", "paste": "Paste after", "paste_before": "Paste before",
		"open_below": "Open line below", "open_above": "Open line above",
		"append": "Append after cursor", "append_line_end": "Append at line end",
		"insert_line_start": "Insert at line start", "join_lines": "Join lines",
		"toggle_select": "Toggle select mode", "extend_line": "Extend to full line",
		"collapse_selection": "Collapse selection", "select_all": "Select all",
		"indent": "Indent", "unindent": "Unindent",
		"goto_mode": "Goto mode (g)", "match_mode": "Match mode (m)", "view_mode": "View mode (z)", "space_mode": "Space menu",
		"merge_mode": "Merge mode (Shift+M)",
		"find_char":  "Find char (f)", "find_char_backward": "Find char back (F)",
		"till_char": "Till char (t)", "till_char_backward": "Till char back (T)",
		"search_forward": "Search /", "search_backward": "Search ?",
		"search_next": "Next match (n)", "search_prev": "Prev match (N)",
		"git_next_change": "Next git change", "git_prev_change": "Prev git change",
		"worktree_menu": "Worktree menu", "worktree_new": "New worktree", "worktree_switch": "Switch worktree",
		"worktree_remove": "Remove worktree", "worktree_refresh": "Refresh worktrees",
		"replace_char": "Replace char (r)", "delete_line": "Delete line",
		"branch_picker": "Branch picker", "insert_line_above": "Insert line above",
		"toggle_line_numbers": "Toggle line numbers",
	}

	// Build bindings list grouped
	var allBindings []keybinding
	for key, action := range e.keymap.normal {
		desc := bindingDescs[action]
		if desc == "" {
			desc = action
		}
		group := actionGroups[action]
		if group == "" {
			group = "Other"
		}
		allBindings = append(allBindings, keybinding{key, action, desc, group})
	}
	allBindings = append(allBindings, keybinding{
		key:    "space-?",
		action: "show_keybindings",
		desc:   "Show all keybindings",
		group:  "Help",
	})

	// Sort by group, then action, then key (stable order)
	sort.Slice(allBindings, func(i, j int) bool {
		if allBindings[i].group != allBindings[j].group {
			return allBindings[i].group < allBindings[j].group
		}
		if allBindings[i].action != allBindings[j].action {
			return allBindings[i].action < allBindings[j].action
		}
		return allBindings[i].key < allBindings[j].key
	})

	// Apply filters (fuzzy match per column)
	filterKey := strings.ToLower(string(e.keybindingsHelpFilterKey))
	filterAct := strings.ToLower(string(e.keybindingsHelpFilterAct))
	filterDesc := strings.ToLower(string(e.keybindingsHelpFilterDesc))
	var filteredBindings []keybinding
	for _, b := range allBindings {
		matchKey := filterKey == "" || fuzzyMatch(filterKey, strings.ToLower(b.key))
		matchAct := filterAct == "" || fuzzyMatch(filterAct, strings.ToLower(b.action))
		matchDesc := filterDesc == "" || fuzzyMatch(filterDesc, strings.ToLower(b.desc))
		if matchKey && matchAct && matchDesc {
			filteredBindings = append(filteredBindings, b)
		}
	}

	// Build display list with group headers
	type displayRow struct {
		text       string
		isHeader   bool
		isGroupHdr bool
	}
	var rows []displayRow

	lastGroup := ""
	for _, b := range filteredBindings {
		if b.group != lastGroup {
			if lastGroup != "" {
				rows = append(rows, displayRow{"", false, false}) // blank line between groups
			}
			rows = append(rows, displayRow{b.group, false, true})
			lastGroup = b.group
		}
		keyCol := fmt.Sprintf("%-18s", b.key)
		actionCol := fmt.Sprintf("%-21s", b.action)
		rows = append(rows, displayRow{" " + keyCol + actionCol + b.desc, false, false})
	}

	// Box dimensions - wider
	boxWidth := w - 4
	if boxWidth > 100 {
		boxWidth = 100
	}
	boxHeight := viewHeight - 2
	innerWidth := boxWidth - 2
	// Header/filter row (1) + separator (1) = 2 fixed rows + borders (2) = 4
	listHeight := boxHeight - 4

	// Clamp scroll
	maxScroll := len(rows) - listHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if e.keybindingsHelpScroll > maxScroll {
		e.keybindingsHelpScroll = maxScroll
	}
	if e.keybindingsHelpScroll < 0 {
		e.keybindingsHelpScroll = 0
	}

	// Center popup
	x0 := (w - boxWidth) / 2
	y0 := (viewHeight - boxHeight) / 2

	borderStyle := e.styleBoxBorder
	if borderStyle == nil {
		borderStyle = e.styleStatus
	}
	contentStyle := e.styleCommand
	headerStyle := e.styleStatus

	// Draw border
	for x := 0; x < boxWidth; x++ {
		ch := '─'
		if x == 0 {
			ch = '┌'
		} else if x == boxWidth-1 {
			ch = '┐'
		}
		s.SetContent(x0+x, y0, ch, nil, borderStyle)
		ch = '─'
		if x == 0 {
			ch = '└'
		} else if x == boxWidth-1 {
			ch = '┘'
		}
		s.SetContent(x0+x, y0+boxHeight-1, ch, nil, borderStyle)
	}

	// Title centered
	title := "Keybindings"
	titleRunes := []rune(title)
	titleStart := (boxWidth - len(titleRunes)) / 2
	for i, r := range titleRunes {
		s.SetContent(x0+titleStart+i, y0, r, nil, borderStyle)
	}

	// Hints at bottom left
	hints := "Up,Down,Home,End,Tab,Esc"
	for i, r := range hints {
		if i+1 < boxWidth-1 {
			s.SetContent(x0+1+i, y0+boxHeight-1, r, nil, borderStyle)
		}
	}

	// Side borders and clear interior
	for y := 1; y < boxHeight-1; y++ {
		s.SetContent(x0, y0+y, '│', nil, borderStyle)
		s.SetContent(x0+boxWidth-1, y0+y, '│', nil, borderStyle)
		for x := 1; x < boxWidth-1; x++ {
			s.SetContent(x0+x, y0+y, ' ', nil, contentStyle)
		}
	}

	// Row 1: Column headers with filter inputs
	filterActiveStyle := e.styleFilterActive
	if filterActiveStyle == nil {
		filterActiveStyle = contentStyle
	}
	filterInactiveStyle := e.styleFilterInactive
	if filterInactiveStyle == nil {
		filterInactiveStyle = contentStyle
	}

	// Draw column headers with filters
	col := 1
	// Key column
	keyLabel := " Key "
	for _, r := range keyLabel {
		s.SetContent(x0+col, y0+1, r, nil, headerStyle)
		col++
	}
	// Key filter box [11 chars]
	keyFilter := string(e.keybindingsHelpFilterKey)
	keyFilterStyle := filterInactiveStyle
	if e.keybindingsHelpFilterFocus == 0 {
		keyFilterStyle = filterActiveStyle
		keyFilter += "_"
	}
	for i := 0; i < 11; i++ {
		ch := ' '
		if i < len(keyFilter) {
			ch = rune(keyFilter[i])
		}
		s.SetContent(x0+col+i, y0+1, ch, nil, keyFilterStyle)
	}
	col += 13

	// Action column
	actLabel := " Action "
	for _, r := range actLabel {
		s.SetContent(x0+col, y0+1, r, nil, headerStyle)
		col++
	}
	// Action filter box [10 chars]
	actFilter := string(e.keybindingsHelpFilterAct)
	actFilterStyle := filterInactiveStyle
	if e.keybindingsHelpFilterFocus == 1 {
		actFilterStyle = filterActiveStyle
		actFilter += "_"
	}
	for i := 0; i < 10; i++ {
		ch := ' '
		if i < len(actFilter) {
			ch = rune(actFilter[i])
		}
		s.SetContent(x0+col+i, y0+1, ch, nil, actFilterStyle)
	}
	col += 13

	// Description column
	descLabel := " Description "
	for _, r := range descLabel {
		s.SetContent(x0+col, y0+1, r, nil, headerStyle)
		col++
	}
	// Description filter box
	descFilter := string(e.keybindingsHelpFilterDesc)
	descFilterStyle := filterInactiveStyle
	if e.keybindingsHelpFilterFocus == 2 {
		descFilterStyle = filterActiveStyle
		descFilter += "_"
	}
	remainingWidth := innerWidth - col
	if remainingWidth < 0 {
		remainingWidth = 0
	}
	if remainingWidth > 15 {
		remainingWidth = 15
	}
	for i := 0; i < remainingWidth; i++ {
		ch := ' '
		if i < len(descFilter) {
			ch = rune(descFilter[i])
		}
		s.SetContent(x0+col+i, y0+1, ch, nil, descFilterStyle)
	}

	// Row 2: Separator
	for x := 1; x < boxWidth-1; x++ {
		s.SetContent(x0+x, y0+2, '─', nil, borderStyle)
	}

	// Draw scrollable content starting at row 3
	for i := 0; i < listHeight; i++ {
		idx := i + e.keybindingsHelpScroll
		if idx >= len(rows) {
			break
		}
		row := rows[idx]
		lineY := y0 + 3 + i

		if row.isGroupHdr {
			// Draw group name centered with full row background
			groupRunes := []rune(row.text)
			groupLen := len(groupRunes)
			leftPad := (innerWidth - groupLen) / 2
			if leftPad < 0 {
				leftPad = 0
			}
			for j := 0; j < innerWidth; j++ {
				ch := ' '
				if j >= leftPad && j < leftPad+groupLen {
					ch = groupRunes[j-leftPad]
				}
				s.SetContent(x0+1+j, lineY, ch, nil, contentStyle)
			}
		} else {
			runes := []rune(row.text)
			if len(runes) > innerWidth {
				runes = runes[:innerWidth]
			}
			for j := 0; j < innerWidth; j++ {
				ch := ' '
				if j < len(runes) {
					ch = runes[j]
				}
				s.SetContent(x0+1+j, lineY, ch, nil, contentStyle)
			}
		}
	}

	// Scroll indicator
	if len(rows) > listHeight {
		scrollInfo := fmt.Sprintf(" %d/%d ", e.keybindingsHelpScroll+1, max(1, len(rows)-listHeight+1))
		infoRunes := []rune(scrollInfo)
		startX := x0 + boxWidth - len(infoRunes) - 1
		for i, r := range infoRunes {
			s.SetContent(startX+i, y0+boxHeight-1, r, nil, borderStyle)
		}
	}
}
