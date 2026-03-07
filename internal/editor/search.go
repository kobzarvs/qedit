package editor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const hugeFileSearchMatchLimit = 10000

// LoadSearchHistory loads search history from file
func (e *Editor) LoadSearchHistory() {
	path := e.searchHistoryPath
	if path == "" || !e.hasHistoryPersistence() {
		return
	}
	history, err := e.loadHistory(path)
	if err != nil {
		return // File doesn't exist yet, that's ok
	}
	e.searchHistory = append(e.searchHistory, history...)
}

// saveSearchHistory saves search history to file
func (e *Editor) saveSearchHistory() {
	path := e.searchHistoryPath
	if path == "" || !e.hasHistoryPersistence() {
		return
	}
	// Keep only last 1000 searches
	history := e.searchHistory
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}
	_ = e.saveHistory(path, history)
}

// addSearchToHistory adds a search query to history with type prefix
// Prefix: "/:" for exact, "F:" for fuzzy, "E:" for regex
func (e *Editor) addSearchToHistory(query string) {
	if query == "" {
		return
	}
	var prefix string
	if e.searchRegex {
		prefix = "E:"
	} else if e.searchFuzzy {
		prefix = "F:"
	} else {
		prefix = "/:"
	}
	entry := prefix + query
	// Don't add duplicates consecutively
	if len(e.searchHistory) > 0 && e.searchHistory[len(e.searchHistory)-1] == entry {
		return
	}
	e.searchHistory = append(e.searchHistory, entry)
	e.saveSearchHistory()
}

// currentSearchPrefix returns the prefix for current search type
func (e *Editor) currentSearchPrefix() string {
	if e.searchRegex {
		return "E:"
	} else if e.searchFuzzy {
		return "F:"
	}
	return "/:"
}

// navigateSearchHistory navigates search history (direction: -1 for older, 1 for newer)
func (e *Editor) navigateSearchHistory(direction int) {
	if len(e.searchHistory) == 0 {
		return
	}

	prefix := e.currentSearchPrefix()

	// Save current query as prefix when starting history navigation
	if e.searchHistoryIndex == -1 && direction < 0 {
		e.searchHistoryPrefix = string(e.searchQuery)
	}

	// Find matching entries in history (filter by search type prefix)
	startIdx := e.searchHistoryIndex
	if startIdx == -1 {
		startIdx = len(e.searchHistory)
	}

	if direction < 0 {
		// Going back in history
		for i := startIdx - 1; i >= 0; i-- {
			entry := e.searchHistory[i]
			if strings.HasPrefix(entry, prefix) {
				query := strings.TrimPrefix(entry, prefix)
				// If we have a prefix filter, apply it
				if e.searchHistoryPrefix == "" || strings.HasPrefix(query, e.searchHistoryPrefix) {
					e.searchHistoryIndex = i
					e.searchQuery = []rune(query)
					e.searchCursor = len(e.searchQuery)
					e.updateSearchMatches()
					return
				}
			}
		}
	} else {
		// Going forward in history
		for i := startIdx + 1; i < len(e.searchHistory); i++ {
			entry := e.searchHistory[i]
			if strings.HasPrefix(entry, prefix) {
				query := strings.TrimPrefix(entry, prefix)
				if e.searchHistoryPrefix == "" || strings.HasPrefix(query, e.searchHistoryPrefix) {
					e.searchHistoryIndex = i
					e.searchQuery = []rune(query)
					e.searchCursor = len(e.searchQuery)
					e.updateSearchMatches()
					return
				}
			}
		}
		// No more forward - restore original prefix
		e.searchHistoryIndex = -1
		e.searchQuery = []rune(e.searchHistoryPrefix)
		e.searchCursor = len(e.searchQuery)
		e.updateSearchMatches()
	}
}
func (e *Editor) handleSearch(ev EventKey) bool {
	// Handle Cmd+Up/Down for navigating matches in file
	if ev.Modifiers()&ModMeta != 0 {
		switch ev.Key() {
		case KeyUp:
			// Navigate to previous match
			if len(e.searchMatches) > 0 {
				e.searchMatchIndex--
				if e.searchMatchIndex < 0 {
					e.searchMatchIndex = len(e.searchMatches) - 1
				}
				e.jumpToCurrentMatch()
			}
			return false
		case KeyDown:
			// Navigate to next match
			if len(e.searchMatches) > 0 {
				e.searchMatchIndex++
				if e.searchMatchIndex >= len(e.searchMatches) {
					e.searchMatchIndex = 0
				}
				e.jumpToCurrentMatch()
			}
			return false
		}
	}

	switch ev.Key() {
	case KeyEscape:
		e.mode = ModeNormal
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.searchMatches = nil
		e.searchHistoryIndex = -1
		return false
	case KeyCtrlC:
		e.mode = ModeNormal
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.searchMatches = nil
		e.searchHistoryIndex = -1
		return false
	case KeyEnter:
		// Confirm search and go to first/current match
		query := string(e.searchQuery)
		if query != "" {
			e.addSearchToHistory(query)
			e.lastSearchQuery = query
		}
		if len(e.searchMatches) > 0 {
			match := e.searchMatches[e.searchMatchIndex]
			e.cursor.Row = match.Row
			e.cursor.Col = match.Col
		}
		e.mode = ModeNormal
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.searchHistoryIndex = -1
		return false
	case KeyBackspace, KeyBackspace2:
		if e.searchCursor > 0 && len(e.searchQuery) > 0 {
			e.searchQuery = append(e.searchQuery[:e.searchCursor-1], e.searchQuery[e.searchCursor:]...)
			e.searchCursor--
			e.updateSearchMatches()
		}
		return false
	case KeyDelete:
		if e.searchCursor < len(e.searchQuery) {
			e.searchQuery = append(e.searchQuery[:e.searchCursor], e.searchQuery[e.searchCursor+1:]...)
			e.updateSearchMatches()
		}
		return false
	case KeyLeft, KeyCtrlB:
		if e.searchCursor > 0 {
			e.searchCursor--
		}
		return false
	case KeyRight, KeyCtrlF:
		if e.searchCursor < len(e.searchQuery) {
			e.searchCursor++
		}
		return false
	case KeyHome, KeyCtrlA:
		e.searchCursor = 0
		return false
	case KeyEnd, KeyCtrlE:
		e.searchCursor = len(e.searchQuery)
		return false
	case KeyUp, KeyCtrlP:
		// Navigate history (older)
		e.navigateSearchHistory(-1)
		return false
	case KeyDown, KeyCtrlN:
		// Navigate history (newer)
		e.navigateSearchHistory(1)
		return false
	case KeyCtrlU:
		e.searchQuery = e.searchQuery[:0]
		e.searchCursor = 0
		e.updateSearchMatches()
		return false
	case KeyCtrlW:
		if e.searchCursor > 0 {
			i := e.searchCursor - 1
			for i > 0 && e.searchQuery[i-1] == ' ' {
				i--
			}
			for i > 0 && e.searchQuery[i-1] != ' ' {
				i--
			}
			e.searchQuery = append(e.searchQuery[:i], e.searchQuery[e.searchCursor:]...)
			e.searchCursor = i
			e.updateSearchMatches()
		}
		return false
	case KeyRune:
		e.searchQuery = append(e.searchQuery[:e.searchCursor], append([]rune{ev.Rune()}, e.searchQuery[e.searchCursor:]...)...)
		e.searchCursor++
		e.updateSearchMatches()
		return false
	}
	return false
}

// updateSearchMatches performs fuzzy search and updates matches
func (e *Editor) updateSearchMatches() {
	e.searchMatches = nil
	e.searchMatchIndex = 0

	query := string(e.searchQuery)
	if query == "" {
		return
	}
	matchLimit := e.searchMatchLimit()
	truncated := false
	searchLineCount, partial := e.searchLineCount()

	// Regex search mode
	if e.searchRegex {
		re, err := regexp.Compile("(?i)" + query) // case-insensitive
		if err != nil {
			// Invalid regex, show error in status
			e.setStatus("regex error: " + err.Error())
			return
		}
		for row := 0; row < searchLineCount; row++ {
			line := e.line(row)
			lineStr := string(line)
			matches := re.FindAllStringIndex(lineStr, -1)
			for _, m := range matches {
				// Convert byte positions to rune positions
				col := utf8.RuneCountInString(lineStr[:m[0]])
				length := utf8.RuneCountInString(lineStr[m[0]:m[1]])
				e.searchMatches = append(e.searchMatches, SearchMatch{
					Row:    row,
					Col:    col,
					Length: length,
					Score:  1000,
				})
				if matchLimit > 0 && len(e.searchMatches) >= matchLimit {
					truncated = true
					break
				}
			}
			if truncated {
				break
			}
		}
	} else {
		queryLower := strings.ToLower(query)

		// Search through all lines
		for row := 0; row < searchLineCount; row++ {
			line := e.line(row)
			lineStr := string(line)
			lineLower := strings.ToLower(lineStr)

			// Find all exact substring matches in this line first
			offset := 0
			for {
				col := strings.Index(lineLower[offset:], queryLower)
				if col < 0 {
					break
				}
				e.searchMatches = append(e.searchMatches, SearchMatch{
					Row:    row,
					Col:    offset + col,
					Length: len(query),
					Score:  1000, // Exact match gets high score
				})
				if matchLimit > 0 && len(e.searchMatches) >= matchLimit {
					truncated = true
					break
				}
				offset += col + 1
				if offset >= len(lineLower) {
					break
				}
			}
			if truncated {
				break
			}

			// In fuzzy mode, find words containing all query letters
			if e.searchFuzzy {
				words := extractWords(line)
				for _, w := range words {
					// Skip if this word position is already covered by an exact match
					alreadyMatched := false
					for _, m := range e.searchMatches {
						if m.Row == row && w.start >= m.Col && w.start < m.Col+m.Length {
							alreadyMatched = true
							break
						}
					}
					if alreadyMatched {
						continue
					}

					// Check fuzzy match (sequential or chunk-based)
					if matchedPositions := fuzzyMatchWord(w.word, query); matchedPositions != nil {
						e.searchMatches = append(e.searchMatches, SearchMatch{
							Row:         row,
							Col:         w.start,
							Length:      len([]rune(w.word)),
							Score:       500, // Fuzzy match score
							MatchedCols: matchedPositions,
						})
						if matchLimit > 0 && len(e.searchMatches) >= matchLimit {
							truncated = true
							break
						}
					}
				}
				if truncated {
					break
				}
			}
			if truncated {
				break
			}
		}
	}

	if truncated {
		e.setStatus(fmt.Sprintf("search truncated to first %d matches in huge file mode", matchLimit))
	} else if partial {
		e.setStatus("search limited to indexed portion while huge file is still indexing")
	}

	// Sort by row, then by column
	sortSearchMatches(e.searchMatches)

	// Find match closest to cursor
	if len(e.searchMatches) > 0 {
		e.searchMatchIndex = 0
		for i, match := range e.searchMatches {
			if match.Row >= e.cursor.Row {
				e.searchMatchIndex = i
				break
			}
		}
		e.jumpToCurrentMatch()
	}
}

func (e *Editor) searchMatchLimit() int {
	if e.hugeFileActive() {
		return hugeFileSearchMatchLimit
	}
	return 0
}

func (e *Editor) searchLineCount() (int, bool) {
	if !e.hugeFileActive() {
		return e.LineCount(), false
	}
	if e.huge.buffer.IndexingComplete() {
		return e.LineCount(), false
	}
	return e.huge.buffer.IndexedLineCount(), true
}

// sequentialMatch checks if all query chars appear in word in order
// e.g., "anwl" matches "actionsWorld" as [a]ctio[n][W]or[l]d
// Returns matched positions (rune indices) or nil if no match
func sequentialMatch(word, query string) []int {
	wordRunes := []rune(strings.ToLower(word))
	queryLower := strings.ToLower(query)

	var positions []int
	wi := 0
	for _, qc := range queryLower {
		found := false
		for wi < len(wordRunes) {
			if wordRunes[wi] == qc {
				positions = append(positions, wi)
				wi++
				found = true
				break
			}
			wi++
		}
		if !found {
			return nil
		}
	}
	return positions
}

// chunkMatch checks if query can be split into 2 chunks that both exist in word
// e.g., "lidra" -> "li" + "dra" both found in "drawLine"
// Returns matched positions (rune indices) or nil if no match
func chunkMatch(word, query string) []int {
	if len(query) < 2 {
		return nil
	}
	wordLower := strings.ToLower(word)
	wordRunes := []rune(wordLower)
	queryLower := strings.ToLower(query)

	// Try all possible 2-chunk splits
	for i := 1; i < len(queryLower); i++ {
		chunk1 := queryLower[:i]
		chunk2 := queryLower[i:]

		idx1 := strings.Index(wordLower, chunk1)
		idx2 := strings.Index(wordLower, chunk2)

		// Both chunks must exist in word
		if idx1 >= 0 && idx2 >= 0 {
			var positions []int
			// Convert byte positions to rune positions and collect all matched runes
			runeIdx1 := utf8.RuneCountInString(wordLower[:idx1])
			runeIdx2 := utf8.RuneCountInString(wordLower[:idx2])
			chunk1Runes := []rune(chunk1)
			chunk2Runes := []rune(chunk2)

			for j := 0; j < len(chunk1Runes); j++ {
				positions = append(positions, runeIdx1+j)
			}
			for j := 0; j < len(chunk2Runes); j++ {
				pos := runeIdx2 + j
				// Avoid duplicates if chunks overlap
				duplicate := false
				for _, p := range positions {
					if p == pos {
						duplicate = true
						break
					}
				}
				if !duplicate {
					positions = append(positions, pos)
				}
			}

			// Sort positions
			for i := 0; i < len(positions); i++ {
				for j := i + 1; j < len(positions); j++ {
					if positions[j] < positions[i] {
						positions[i], positions[j] = positions[j], positions[i]
					}
				}
			}

			// Validate: matched positions should equal query length (ignoring overlaps)
			if len(positions) >= len(wordRunes) || len(positions) < len([]rune(query)) {
				continue
			}
			return positions
		}
	}
	return nil
}

// fuzzyMatchWord checks if word matches query using fuzzy algorithms
// Returns matched positions (rune indices) or nil if no match
func fuzzyMatchWord(word, query string) []int {
	// Try sequential match first (letters in order)
	if positions := sequentialMatch(word, query); positions != nil {
		return positions
	}
	// Try chunk match (query split into 2 parts, both found in word)
	if positions := chunkMatch(word, query); positions != nil {
		return positions
	}
	return nil
}

// isWordChar returns true if the rune is part of a word/identifier
func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// wordMatch holds a word and its position in a line
type wordMatch struct {
	word  string
	start int
	end   int
}

// extractWords extracts all words/identifiers from a line with their positions
func extractWords(line []rune) []wordMatch {
	var words []wordMatch
	i := 0
	for i < len(line) {
		// Skip non-word characters
		for i < len(line) && !isWordChar(line[i]) {
			i++
		}
		if i >= len(line) {
			break
		}
		// Collect word
		start := i
		for i < len(line) && isWordChar(line[i]) {
			i++
		}
		words = append(words, wordMatch{
			word:  string(line[start:i]),
			start: start,
			end:   i,
		})
	}
	return words
}

// sortSearchMatches sorts matches by row (for navigation)
func sortSearchMatches(matches []SearchMatch) {
	// Simple bubble sort (matches are usually small)
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Row < matches[i].Row ||
				(matches[j].Row == matches[i].Row && matches[j].Col < matches[i].Col) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
}

// jumpToCurrentMatch moves cursor to the current search match
func (e *Editor) jumpToCurrentMatch() {
	if e.searchMatchIndex < 0 || e.searchMatchIndex >= len(e.searchMatches) {
		return
	}
	match := e.searchMatches[e.searchMatchIndex]
	e.cursor.Row = match.Row
	e.cursor.Col = match.Col + match.Length // cursor at end of word
	e.ensureCursorVisible(e.viewHeightCached())

	// Select the whole matched word for editing (d/c/r/DEL)
	if match.Length > 0 {
		e.selectionActive = true
		e.selectionStart = Cursor{Row: match.Row, Col: match.Col}
		e.selectionEnd = Cursor{Row: match.Row, Col: match.Col + match.Length}
	}
}

// enterSearchMode enters search mode
func (e *Editor) enterSearchMode(forward bool, fuzzy bool, regex bool) {
	e.mode = ModeSearch
	e.searchQuery = e.searchQuery[:0]
	e.searchCursor = 0
	e.searchMatches = nil
	e.searchMatchIndex = 0
	e.searchForward = forward
	e.searchFuzzy = fuzzy
	e.searchRegex = regex
	if regex {
		e.modal.pendingKeys = "E"
	} else if fuzzy {
		e.modal.pendingKeys = "F"
	} else {
		e.modal.pendingKeys = "/"
	}
}

// searchNext goes to next match
func (e *Editor) searchNext() {
	if e.lastSearchQuery == "" {
		e.setStatus("no previous search")
		return
	}

	// Re-run search if matches are empty
	if len(e.searchMatches) == 0 {
		e.searchQuery = []rune(e.lastSearchQuery)
		e.updateSearchMatches()
	}

	if len(e.searchMatches) == 0 {
		e.setStatus("no matches")
		return
	}

	// Find next match after cursor
	found := false
	for i, match := range e.searchMatches {
		if match.Row > e.cursor.Row || (match.Row == e.cursor.Row && match.Col > e.cursor.Col) {
			e.searchMatchIndex = i
			found = true
			break
		}
	}
	if !found {
		e.searchMatchIndex = 0 // Wrap around
	}

	e.jumpToCurrentMatch()
	e.setStatus(fmt.Sprintf("[%d/%d] %s", e.searchMatchIndex+1, len(e.searchMatches), e.lastSearchQuery))
}

// searchPrev goes to previous match
func (e *Editor) searchPrev() {
	if e.lastSearchQuery == "" {
		e.setStatus("no previous search")
		return
	}

	// Re-run search if matches are empty
	if len(e.searchMatches) == 0 {
		e.searchQuery = []rune(e.lastSearchQuery)
		e.updateSearchMatches()
	}

	if len(e.searchMatches) == 0 {
		e.setStatus("no matches")
		return
	}

	// Find previous match before cursor
	found := false
	for i := len(e.searchMatches) - 1; i >= 0; i-- {
		match := e.searchMatches[i]
		if match.Row < e.cursor.Row || (match.Row == e.cursor.Row && match.Col < e.cursor.Col) {
			e.searchMatchIndex = i
			found = true
			break
		}
	}
	if !found {
		e.searchMatchIndex = len(e.searchMatches) - 1 // Wrap around
	}

	e.jumpToCurrentMatch()
	e.setStatus(fmt.Sprintf("[%d/%d] %s", e.searchMatchIndex+1, len(e.searchMatches), e.lastSearchQuery))
}
