package editor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const hugeFileIndexCacheVersion = 1

type hugeFileIndexCache struct {
	Version     int                  `json:"v"`
	Size        int64                `json:"size"`
	ModTimeUnix int64                `json:"mtime"`
	LineCount   int                  `json:"line_count"`
	Checkpoints []hugeFileCheckpoint `json:"checkpoints"`
}

func loadHugeFileIndexCache(filePath string, meta FileMetadata) ([]hugeFileCheckpoint, int, bool) {
	cachePath := hugeFileIndexCachePath(filePath)
	if cachePath == "" {
		return nil, 0, false
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, 0, false
	}
	var cache hugeFileIndexCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, 0, false
	}
	if cache.Version != hugeFileIndexCacheVersion {
		return nil, 0, false
	}
	if cache.Size != meta.Size || cache.ModTimeUnix != meta.ModTime.UnixNano() {
		return nil, 0, false
	}
	if cache.LineCount < 1 || len(cache.Checkpoints) == 0 {
		return nil, 0, false
	}
	if cache.Checkpoints[0].row != 0 || cache.Checkpoints[0].offset != 0 {
		return nil, 0, false
	}
	return cache.Checkpoints, cache.LineCount, true
}

func saveHugeFileIndexCache(filePath string, meta FileMetadata, checkpoints []hugeFileCheckpoint, lineCount int) error {
	cachePath := hugeFileIndexCachePath(filePath)
	if cachePath == "" {
		return nil
	}
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cache := hugeFileIndexCache{
		Version:     hugeFileIndexCacheVersion,
		Size:        meta.Size,
		ModTimeUnix: meta.ModTime.UnixNano(),
		LineCount:   lineCount,
		Checkpoints: checkpoints,
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0o644)
}

func hugeFileIndexCachePath(filePath string) string {
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
	return filepath.Join(stateDir, "qedit", "huge-index", encoded+".json")
}
