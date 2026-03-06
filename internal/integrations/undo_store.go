package integrations

import (
	"os"
	"path/filepath"
	"strings"
)

// FileUndoStore persists undo logs in the local XDG state directory.
type FileUndoStore struct{}

func (FileUndoStore) Load(path string) ([]byte, error) {
	logPath := undoLogPath(path)
	if logPath == "" {
		return nil, nil
	}
	return os.ReadFile(logPath)
}

func (FileUndoStore) Save(path string, data []byte) error {
	logPath := undoLogPath(path)
	if logPath == "" {
		return nil
	}
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(logPath, data, 0o644)
}

func (FileUndoStore) Remove(path string) error {
	logPath := undoLogPath(path)
	if logPath == "" {
		return nil
	}
	return os.Remove(logPath)
}

func (FileUndoStore) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

func undoLogPath(filePath string) string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	encoded := strings.ReplaceAll(absPath, string(filepath.Separator), "_")
	encoded = strings.ReplaceAll(encoded, ":", "_")
	encoded = strings.ReplaceAll(encoded, " ", "_")
	return filepath.Join(stateDir, "qedit", "undo", encoded+".log")
}
