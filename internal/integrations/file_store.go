package integrations

import (
	"os"
	"path/filepath"

	"github.com/kobzarvs/qedit/internal/editor"
)

// FileStore provides OS-backed file operations for the active editor buffer.
type FileStore struct{}

func (FileStore) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

func (FileStore) HomeDir() (string, error) {
	return os.UserHomeDir()
}

func (FileStore) Read(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (FileStore) ReadDir(path string) ([]editor.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]editor.DirEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, editor.DirEntry{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
		})
	}
	return result, nil
}

func (FileStore) Write(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

func (FileStore) Stat(path string) (editor.FileMetadata, error) {
	info, err := os.Stat(path)
	if err != nil {
		return editor.FileMetadata{}, err
	}
	return editor.FileMetadata{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
	}, nil
}

func (FileStore) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}
