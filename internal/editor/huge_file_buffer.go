package editor

import (
	"fmt"
	"io"
)

const hugeFileLineCacheSize = 256

type HugeFileBuffer struct {
	path        string
	sizeBytes   int64
	reader      io.ReadSeekCloser
	lineOffsets []int64
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
	offsets, err := buildHugeFileLineIndex(reader)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	return &HugeFileBuffer{
		path:        path,
		sizeBytes:   sizeBytes,
		reader:      reader,
		lineOffsets: offsets,
		lineCache:   make(map[int][]rune),
	}, nil
}

func buildHugeFileLineIndex(reader io.ReadSeeker) ([]int64, error) {
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	offsets := []int64{0}
	buf := make([]byte, 1<<20)
	var offset int64
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			for i, b := range buf[:n] {
				if b == '\n' {
					offsets = append(offsets, offset+int64(i)+1)
				}
			}
			offset += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return offsets, nil
}

func (b *HugeFileBuffer) Close() error {
	if b == nil || b.reader == nil {
		return nil
	}
	err := b.reader.Close()
	b.reader = nil
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
	if b == nil || len(b.lineOffsets) == 0 {
		return 1
	}
	return len(b.lineOffsets)
}

func (b *HugeFileBuffer) LineLen(row int) int {
	return len(b.Line(row))
}

func (b *HugeFileBuffer) Line(row int) []rune {
	if b == nil || len(b.lineOffsets) == 0 {
		return nil
	}
	if row < 0 {
		row = 0
	}
	if row >= len(b.lineOffsets) {
		row = len(b.lineOffsets) - 1
	}
	if cached, ok := b.lineCache[row]; ok {
		b.touchCache(row)
		return cached
	}

	start := b.lineOffsets[row]
	end := b.sizeBytes
	if row+1 < len(b.lineOffsets) {
		end = b.lineOffsets[row+1] - 1
	}
	if end < start {
		end = start
	}
	length := end - start
	if length < 0 {
		length = 0
	}
	if _, err := b.reader.Seek(start, io.SeekStart); err != nil {
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
