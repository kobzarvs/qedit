package editor

import "testing"

type testHistoryStore struct {
	loadEntries  map[string][]string
	savedPath    string
	savedEntries []string
}

func (s *testHistoryStore) Load(path string) ([]string, error) {
	return append([]string(nil), s.loadEntries[path]...), nil
}

func (s *testHistoryStore) Save(path string, entries []string) error {
	s.savedPath = path
	s.savedEntries = append([]string(nil), entries...)
	return nil
}

func TestLoadCmdHistoryUsesRuntimeStore(t *testing.T) {
	e := newTestEditor("one")
	store := &testHistoryStore{
		loadEntries: map[string][]string{
			"/tmp/cmd_history": {"w", "q"},
		},
	}
	e.SetHistoryStore(store)
	e.commandLine.historyPath = "/tmp/cmd_history"

	e.LoadCmdHistory()

	if len(e.commandLine.history) != 2 {
		t.Fatalf("history len = %d, want 2", len(e.commandLine.history))
	}
	if e.commandLine.history[0] != "w" || e.commandLine.history[1] != "q" {
		t.Fatalf("history = %#v, want []string{\"w\", \"q\"}", e.commandLine.history)
	}
}

func TestSaveSearchHistoryUsesRuntimeStore(t *testing.T) {
	e := newTestEditor("one")
	store := &testHistoryStore{}
	e.SetHistoryStore(store)
	e.searchHistoryPath = "/tmp/search_history"
	e.searchHistory = []string{"/:one", "/:two"}

	e.saveSearchHistory()

	if store.savedPath != "/tmp/search_history" {
		t.Fatalf("saved path = %q, want %q", store.savedPath, "/tmp/search_history")
	}
	if len(store.savedEntries) != 2 {
		t.Fatalf("saved entries len = %d, want 2", len(store.savedEntries))
	}
	if store.savedEntries[0] != "/:one" || store.savedEntries[1] != "/:two" {
		t.Fatalf("saved entries = %#v, want []string{\"/:one\", \"/:two\"}", store.savedEntries)
	}
}
