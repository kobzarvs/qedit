package editor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const hugeFileIndexCacheVersion = 3

type hugeFileCachedIndex struct {
	Checkpoints        []hugeFileCheckpoint
	ByteAnchors        []hugeFileCheckpoint
	PageAnchors        []hugeFilePageAnchor
	LineCount          int
	EstimatedLineCount int
	FullyIndexed       bool
}

type hugeFileIndexCache struct {
	Version            int                  `json:"v"`
	Size               int64                `json:"size"`
	ModTimeUnix        int64                `json:"mtime"`
	LineCount          int                  `json:"line_count"`
	EstimatedLineCount int                  `json:"estimated_line_count,omitempty"`
	FullyIndexed       bool                 `json:"fully_indexed"`
	Checkpoints        []hugeFileCheckpoint `json:"checkpoints"`
	ByteAnchors        []hugeFileCheckpoint `json:"byte_anchors,omitempty"`
	PageAnchors        []hugeFilePageAnchor `json:"page_anchors,omitempty"`
}

func loadHugeFileIndexCache(filePath string, meta FileMetadata) (hugeFileCachedIndex, bool) {
	cachePath := hugeFileIndexCachePath(filePath)
	if cachePath == "" {
		return hugeFileCachedIndex{}, false
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return hugeFileCachedIndex{}, false
	}
	var cache hugeFileIndexCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return hugeFileCachedIndex{}, false
	}
	if cache.Version != hugeFileIndexCacheVersion {
		return hugeFileCachedIndex{}, false
	}
	if cache.Size != meta.Size || cache.ModTimeUnix != meta.ModTime.UnixNano() {
		return hugeFileCachedIndex{}, false
	}
	if cache.LineCount < 1 || len(cache.Checkpoints) == 0 {
		return hugeFileCachedIndex{}, false
	}
	if cache.Checkpoints[0].row != 0 || cache.Checkpoints[0].offset != 0 {
		return hugeFileCachedIndex{}, false
	}
	estimated := cache.EstimatedLineCount
	if estimated < cache.LineCount {
		estimated = cache.LineCount
	}
	return hugeFileCachedIndex{
		Checkpoints:        cache.Checkpoints,
		ByteAnchors:        ensureHugeFileAnchors(cache.ByteAnchors),
		PageAnchors:        ensureHugeFilePageAnchors(cache.PageAnchors),
		LineCount:          cache.LineCount,
		EstimatedLineCount: estimated,
		FullyIndexed:       cache.FullyIndexed,
	}, true
}

func saveHugeFileIndexCache(filePath string, meta FileMetadata, checkpoints, byteAnchors []hugeFileCheckpoint, pageAnchors []hugeFilePageAnchor, lineCount, estimatedLineCount int, fullyIndexed bool) error {
	cachePath := hugeFileIndexCachePath(filePath)
	if cachePath == "" {
		return nil
	}
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cache := hugeFileIndexCache{
		Version:            hugeFileIndexCacheVersion,
		Size:               meta.Size,
		ModTimeUnix:        meta.ModTime.UnixNano(),
		LineCount:          lineCount,
		EstimatedLineCount: estimatedLineCount,
		FullyIndexed:       fullyIndexed,
		Checkpoints:        checkpoints,
		ByteAnchors:        ensureHugeFileAnchors(byteAnchors),
		PageAnchors:        ensureHugeFilePageAnchors(pageAnchors),
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0o644)
}

func ensureHugeFileAnchors(anchors []hugeFileCheckpoint) []hugeFileCheckpoint {
	if len(anchors) == 0 {
		return []hugeFileCheckpoint{{row: 0, offset: 0}}
	}
	if anchors[0].row == 0 && anchors[0].offset == 0 {
		return anchors
	}
	return append([]hugeFileCheckpoint{{row: 0, offset: 0}}, anchors...)
}

func ensureHugeFilePageAnchors(anchors []hugeFilePageAnchor) []hugeFilePageAnchor {
	if len(anchors) == 0 {
		return []hugeFilePageAnchor{{row: 0, offset: 0, lineStart: 0}}
	}
	if anchors[0].row == 0 && anchors[0].offset == 0 && anchors[0].lineStart == 0 {
		return anchors
	}
	return append([]hugeFilePageAnchor{{row: 0, offset: 0, lineStart: 0}}, anchors...)
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
