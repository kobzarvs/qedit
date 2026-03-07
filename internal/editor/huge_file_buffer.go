package editor

import (
	"fmt"
	"io"
	"sort"
)

const hugeFileLineCacheSize = 256
const hugeFileSpanCacheSize = 4096
const hugeFileCheckpointSpacing = 1024
const hugeFileLinePrefetch = 64

type hugeFileCheckpoint struct {
	row    int
	offset int64
}

type hugeFileLineSpan struct {
	start int64
	end   int64
}

type HugeFileBuffer struct {
	path        string
	sizeBytes   int64
	reader      io.ReadSeekCloser
	lineCount   int
	checkpoints []hugeFileCheckpoint
	lineSpans   map[int]hugeFileLineSpan
	spanOrder   []int
	lineCache   map[int][]rune
	cacheOrder  []int
}

func OpenHugeFileBuffer(path string, sizeBytes int64, fs FileStore) (*HugeFileBuffer, error) {
	if fs == nil {
		return nil, errFileStoreUnavailable()
	}
	reader, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	checkpoints, lineCount, err := buildHugeFileLineIndex(reader)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	return &HugeFileBuffer{
		path:        path,
		sizeBytes:   sizeBytes,
		reader:      reader,
		lineCount:   lineCount,
		checkpoints: checkpoints,
		lineSpans:   make(map[int]hugeFileLineSpan),
		lineCache:   make(map[int][]rune),
	}, nil
}

func buildHugeFileLineIndex(reader io.ReadSeeker) ([]hugeFileCheckpoint, int, error) {
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, 0, err
	}
	checkpoints := []hugeFileCheckpoint{{row: 0, offset: 0}}
	buf := make([]byte, 1<<20)
	var offset int64
	lineCount := 1
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			for i, b := range buf[:n] {
				if b == '\n' {
					lineCount++
					if (lineCount-1)%hugeFileCheckpointSpacing == 0 {
						checkpoints = append(checkpoints, hugeFileCheckpoint{
							row:    lineCount - 1,
							offset: offset + int64(i) + 1,
						})
					}
				}
			}
			offset += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, 0, err
	}
	return checkpoints, lineCount, nil
}

func (b *HugeFileBuffer) Close() error {
	if b == nil || b.reader == nil {
		return nil
	}
	err := b.reader.Close()
	b.reader = nil
	b.lineSpans = nil
	b.spanOrder = nil
	b.lineCache = nil
	b.cacheOrder = nil
	return err
}

func (b *HugeFileBuffer) Path() string {
	if b == nil {
		return ""
	}
	return b.path
}

func (b *HugeFileBuffer) SizeBytes() int64 {
	if b == nil {
		return 0
	}
	return b.sizeBytes
}

func (b *HugeFileBuffer) LineCount() int {
	if b == nil || b.lineCount <= 0 {
		return 1
	}
	return b.lineCount
}

func (b *HugeFileBuffer) LineLen(row int) int {
	return len(b.Line(row))
}

func (b *HugeFileBuffer) Line(row int) []rune {
	if b == nil || b.lineCount <= 0 {
		return nil
	}
	if row < 0 {
		row = 0
	}
	if row >= b.lineCount {
		row = b.lineCount - 1
	}
	if cached, ok := b.lineCache[row]; ok {
		b.touchCache(row)
		return cached
	}

	span, err := b.resolveLineSpan(row)
	if err != nil {
		return []rune(fmt.Sprintf("[read error: %v]", err))
	}
	length := span.end - span.start
	if length < 0 {
		length = 0
	}
	if _, err := b.reader.Seek(span.start, io.SeekStart); err != nil {
		return []rune(fmt.Sprintf("[read error: %v]", err))
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(b.reader, data); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return []rune(fmt.Sprintf("[read error: %v]", err))
	}
	if len(data) > 0 && data[len(data)-1] == '\r' {
		data = data[:len(data)-1]
	}
	line := []rune(string(data))
	b.storeCachedLine(row, line)
	return line
}

func (b *HugeFileBuffer) resolveLineSpan(row int) (hugeFileLineSpan, error) {
	if cached, ok := b.lineSpans[row]; ok {
		b.touchSpanCache(row)
		return cached, nil
	}

	checkpoint := b.nearestCheckpoint(row)
	if _, err := b.reader.Seek(checkpoint.offset, io.SeekStart); err != nil {
		return hugeFileLineSpan{}, err
	}

	currentRow := checkpoint.row
	lineStart := checkpoint.offset
	fileOffset := checkpoint.offset
	targetRow := row + hugeFileLinePrefetch
	if targetRow >= b.lineCount {
		targetRow = b.lineCount - 1
	}

	buf := make([]byte, 64<<10)
	var targetSpan hugeFileLineSpan
	foundTarget := false

	for {
		n, err := b.reader.Read(buf)
		if n > 0 {
			for i, ch := range buf[:n] {
				if ch != '\n' {
					continue
				}
				lineEnd := fileOffset + int64(i)
				span := hugeFileLineSpan{start: lineStart, end: lineEnd}
				b.storeLineSpan(currentRow, span)
				if currentRow == row {
					targetSpan = span
					foundTarget = true
				}
				if currentRow >= targetRow {
					return targetSpan, nil
				}
				currentRow++
				lineStart = fileOffset + int64(i) + 1
			}
			fileOffset += int64(n)
		}
		if err == io.EOF {
			span := hugeFileLineSpan{start: lineStart, end: b.sizeBytes}
			b.storeLineSpan(currentRow, span)
			if currentRow == row {
				targetSpan = span
				foundTarget = true
			}
			if foundTarget {
				return targetSpan, nil
			}
			break
		}
		if err != nil {
			return hugeFileLineSpan{}, err
		}
	}

	if foundTarget {
		return targetSpan, nil
	}
	return hugeFileLineSpan{}, fmt.Errorf("line %d out of range", row)
}

func (b *HugeFileBuffer) nearestCheckpoint(row int) hugeFileCheckpoint {
	if len(b.checkpoints) == 0 {
		return hugeFileCheckpoint{row: 0, offset: 0}
	}
	idx := sort.Search(len(b.checkpoints), func(i int) bool {
		return b.checkpoints[i].row > row
	})
	if idx == 0 {
		return b.checkpoints[0]
	}
	return b.checkpoints[idx-1]
}

func (b *HugeFileBuffer) touchCache(row int) {
	for i, cached := range b.cacheOrder {
		if cached != row {
			continue
		}
		copy(b.cacheOrder[i:], b.cacheOrder[i+1:])
		b.cacheOrder[len(b.cacheOrder)-1] = row
		return
	}
}

func (b *HugeFileBuffer) storeCachedLine(row int, line []rune) {
	if b.lineCache == nil {
		b.lineCache = make(map[int][]rune)
	}
	if _, ok := b.lineCache[row]; ok {
		b.lineCache[row] = line
		b.touchCache(row)
		return
	}
	if len(b.cacheOrder) >= hugeFileLineCacheSize {
		evict := b.cacheOrder[0]
		delete(b.lineCache, evict)
		b.cacheOrder = b.cacheOrder[1:]
	}
	b.lineCache[row] = line
	b.cacheOrder = append(b.cacheOrder, row)
}

func (b *HugeFileBuffer) touchSpanCache(row int) {
	for i, cached := range b.spanOrder {
		if cached != row {
			continue
		}
		copy(b.spanOrder[i:], b.spanOrder[i+1:])
		b.spanOrder[len(b.spanOrder)-1] = row
		return
	}
}

func (b *HugeFileBuffer) storeLineSpan(row int, span hugeFileLineSpan) {
	if b.lineSpans == nil {
		b.lineSpans = make(map[int]hugeFileLineSpan)
	}
	if _, ok := b.lineSpans[row]; ok {
		b.lineSpans[row] = span
		b.touchSpanCache(row)
		return
	}
	if len(b.spanOrder) >= hugeFileSpanCacheSize {
		evict := b.spanOrder[0]
		delete(b.lineSpans, evict)
		b.spanOrder = b.spanOrder[1:]
	}
	b.lineSpans[row] = span
	b.spanOrder = append(b.spanOrder, row)
}
