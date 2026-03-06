package integrations

import (
	"os"
	"path/filepath"
	"strings"
)

// FileHistoryStore persists line-based histories in the local filesystem.
type FileHistoryStore struct{}

func (FileHistoryStore) Load(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func (FileHistoryStore) Save(path string, entries []string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data := strings.Join(entries, "\n")
	return os.WriteFile(path, []byte(data), 0o644)
}
