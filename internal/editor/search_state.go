package editor

// SearchState holds search query and history state.
// It is embedded in Editor to keep field access stable during refactor.
type SearchState struct {
	searchQuery         []rune        // current search query
	searchCursor        int           // cursor position within search query
	searchMatches       []SearchMatch // all matches in the file
	searchMatchIndex    int           // current match index
	searchForward       bool          // search direction
	searchFuzzy         bool          // true = fuzzy search (cmd+f), false = exact (/)
	searchRegex         bool          // true = regex search (cmd+e)
	lastSearchQuery     string        // last search query for n/N
	searchHistory       []string      // search history (prefixed with /: F: or E:)
	searchHistoryIndex  int           // current position in search history (-1 = not browsing)
	searchHistoryPrefix string        // prefix for filtered search history
	searchHistoryPath   string        // search history file path
}
