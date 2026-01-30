package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileState stores the state of a single file
type FileState struct {
	CursorRow int    `json:"cursor_row"`
	CursorCol int    `json:"cursor_col"`
	ScrollY   int    `json:"scroll_y"`
	ScrollX   int    `json:"scroll_x"`
	Mode      string `json:"mode"` // "normal", "insert"
	// Selection state
	SelectionActive bool `json:"selection_active,omitempty"`
	SelectionStartRow int `json:"selection_start_row,omitempty"`
	SelectionStartCol int `json:"selection_start_col,omitempty"`
	SelectionEndRow   int `json:"selection_end_row,omitempty"`
	SelectionEndCol   int `json:"selection_end_col,omitempty"`
}

// RepoInfo stores repository-specific information
type RepoInfo struct {
	MainBranch string `json:"main_branch,omitempty"`
}

// Session stores the complete editor session state
type Session struct {
	Files       map[string]FileState `json:"files"`
	Repos       map[string]RepoInfo  `json:"repos,omitempty"` // keyed by repo root path
	ActiveFile  string               `json:"active_file,omitempty"`
	// Future: Tabs, Windows, Panels
	// Tabs        []TabState           `json:"tabs,omitempty"`
	// Windows     []WindowState        `json:"windows,omitempty"`
	LastSaved   time.Time            `json:"last_saved"`
}

// Manager handles session persistence
type Manager struct {
	mu       sync.RWMutex
	session  Session
	path     string
	dirty    bool
	stopChan chan struct{}
}

// NewManager creates a new session manager
func NewManager() (*Manager, error) {
	path, err := sessionPath()
	if err != nil {
		return nil, err
	}

	m := &Manager{
		session: Session{
			Files: make(map[string]FileState),
		},
		path:     path,
		stopChan: make(chan struct{}),
	}

	// Load existing session
	m.load()

	// Start autosave timer
	go m.autosaveLoop()

	return m, nil
}

func sessionPath() (string, error) {
	// XDG state directory
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(stateDir, "qedit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

func (m *Manager) load() {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return // No existing session, start fresh
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return
	}
	if session.Files == nil {
		session.Files = make(map[string]FileState)
	}
	if session.Repos == nil {
		session.Repos = make(map[string]RepoInfo)
	}
	m.session = session
}

// Save persists the session to disk
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.dirty {
		return nil
	}

	m.session.LastSaved = time.Now()
	data, err := json.MarshalIndent(m.session, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(m.path, data, 0o644); err != nil {
		return err
	}

	m.dirty = false
	return nil
}

// ForceSave saves even if not dirty
func (m *Manager) ForceSave() error {
	m.mu.Lock()
	m.dirty = true
	m.mu.Unlock()
	return m.Save()
}

// GetFileState returns the saved state for a file
func (m *Manager) GetFileState(absPath string) (FileState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.session.Files[absPath]
	return state, ok
}

// SetFileState updates the state for a file
func (m *Manager) SetFileState(absPath string, state FileState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.session.Files[absPath] = state
	m.session.ActiveFile = absPath
	m.dirty = true
}

// SetActiveFile sets the currently active file
func (m *Manager) SetActiveFile(absPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.session.ActiveFile = absPath
	m.dirty = true
}

// GetActiveFile returns the last active file
func (m *Manager) GetActiveFile() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session.ActiveFile
}

// GetRepoInfo returns saved info for a repository
func (m *Manager) GetRepoInfo(repoRoot string) (RepoInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.session.Repos[repoRoot]
	return info, ok
}

// SetRepoMainBranch saves the main branch for a repository
func (m *Manager) SetRepoMainBranch(repoRoot, mainBranch string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session.Repos == nil {
		m.session.Repos = make(map[string]RepoInfo)
	}
	info := m.session.Repos[repoRoot]
	info.MainBranch = mainBranch
	m.session.Repos[repoRoot] = info
	m.dirty = true
}

func (m *Manager) autosaveLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = m.Save()
		case <-m.stopChan:
			return
		}
	}
}

// Stop stops the autosave loop and saves final state
func (m *Manager) Stop() {
	close(m.stopChan)
	_ = m.ForceSave()
}
