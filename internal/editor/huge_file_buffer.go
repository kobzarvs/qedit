package editor

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"math"
	"runtime"
	"sort"
	"sync"
	"time"
	"unicode/utf8"
)

// lruTracker provides O(1) touch/add/evict for LRU caches using a doubly-linked
// list + map instead of the previous O(n) linear-scan slice approach.
type lruTracker struct {
	order *list.List
	index map[int]*list.Element
}

func newLRUTracker() lruTracker {
	return lruTracker{
		order: list.New(),
		index: make(map[int]*list.Element),
	}
}

// touch moves the key to the back (most recently used). O(1).
func (t *lruTracker) touch(key int) {
	if el, ok := t.index[key]; ok {
		t.order.MoveToBack(el)
	}
}

// add appends the key to the back. O(1). Caller must check for duplicates.
func (t *lruTracker) add(key int) {
	el := t.order.PushBack(key)
	t.index[key] = el
}

// evictOldest removes and returns the oldest key. O(1).
func (t *lruTracker) evictOldest() (int, bool) {
	front := t.order.Front()
	if front == nil {
		return 0, false
	}
	key := front.Value.(int)
	t.order.Remove(front)
	delete(t.index, key)
	return key, true
}

// remove deletes a specific key. O(1).
func (t *lruTracker) remove(key int) {
	if el, ok := t.index[key]; ok {
		t.order.Remove(el)
		delete(t.index, key)
	}
}

// len returns the number of tracked keys.
func (t *lruTracker) len() int {
	return t.order.Len()
}

const hugeFileLineCacheSize = 256
const hugeFileSpanCacheSize = 4096
const hugeFilePageDataCacheSize = 8
const hugeFileCheckpointSpacing = 1024
const hugeFileLinePrefetch = 64
const hugeFileIndexCheckpointBatch = 64
const hugeFileDirectReadThreshold int64 = 128 << 10
const hugeFileInitialSpanCacheLimit = 4096

var hugeFileInitialSampleBytes int64 = 2 << 20
var hugeFileMinByteAnchorSpacing int64 = 64 << 10
var hugeFileMaxByteAnchorSpacing int64 = 4 << 20
var hugeFileTargetByteAnchors int64 = 16384
var hugeFileByteAnchorSpacingOverride int64
var hugeFileIndexPersistInterval = 250 * time.Millisecond

var errHugeFileLineOutOfRange = errors.New("huge file line out of range")

// bytesToRunes converts bytes to runes. For pure-ASCII data, avoids the
// intermediate string allocation of []rune(string(data)) — single pass,
// single allocation.
func bytesToRunes(data []byte) []rune {
	// Quick ASCII check: if all bytes < 0x80 we can skip UTF-8 decoding.
	ascii := true
	for _, b := range data {
		if b >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		runes := make([]rune, len(data))
		for i, b := range data {
			runes[i] = rune(b)
		}
		return runes
	}
	return []rune(string(data))
}

type hugeFileCheckpoint struct {
	row    int
	offset int64
}

type hugeFilePageAnchor struct {
	row       int
	offset    int64
	lineStart int64
}

type hugeFileLineSpan struct {
	start int64
	end   int64
}

type hugeFileLineSpanEntry struct {
	row  int
	span hugeFileLineSpan
}

type hugeFileCachedLineEntry struct {
	row  int
	line []rune
}

type hugeFileCachedLineEndingEntry struct {
	row int
	eol string
}

type hugeFileLineInfo struct {
	runeLen   int
	asciiOnly bool
	hasTabs   bool
}

type hugeFileCachedLineInfoEntry struct {
	row  int
	info hugeFileLineInfo
}

type hugeFileCachedPageData struct {
	startRow    int
	endRow      int
	startOffset int64
	spans       []hugeFileLineSpan
	data        []byte
}

type hugeFileInitialIndex struct {
	checkpoints        []hugeFileCheckpoint
	byteAnchors        []hugeFileCheckpoint
	pageAnchors        []hugeFilePageAnchor
	lineSpans          []hugeFileLineSpanEntry
	lineCount          int
	estimatedLineCount int
	sampleOffset       int64
	fullyIndexed       bool
}

type HugeFileBuffer struct {
	path       string
	sizeBytes  int64
	meta       FileMetadata
	reader     io.ReadSeekCloser
	openReader func() (io.ReadSeekCloser, error)
	mmapData   []byte // memory-mapped file contents (nil if mmap unavailable)

	mu                 sync.RWMutex
	lineCount          int
	estimatedLineCount int
	checkpoints        []hugeFileCheckpoint
	byteAnchors        []hugeFileCheckpoint
	pageAnchors        []hugeFilePageAnchor
	byteAnchorSpacing  int64
	fullyIndexed       bool
	indexErr           error
	cancelIndex        chan struct{}
	indexDone          chan struct{}

	lineSpans   map[int]hugeFileLineSpan
	spanLRU     lruTracker
	spanSorted  []int // sorted row keys for O(log n) nearest-span lookup
	lineCache   map[int][]rune
	cacheLRU    lruTracker
	lineInfo    map[int]hugeFileLineInfo
	infoLRU     lruTracker
	lineEndings map[int]string
	endingLRU   lruTracker
	pageData    map[int]hugeFileCachedPageData
	pageLRU     lruTracker
	warmInFlight map[int]struct{}
	closed       bool

	workers sync.WaitGroup
}

// mmapSlice returns a sub-slice of the mmap'd data without copying.
// Returns nil if mmap is not available or range is invalid.
func (b *HugeFileBuffer) mmapSlice(start, end int64) []byte {
	if b.mmapData == nil || start < 0 || end < start || end > int64(len(b.mmapData)) {
		return nil
	}
	return b.mmapData[start:end]
}

// readBytesAt reads [start, end) from the file using mmap if available,
// otherwise falling back to the provided reader (seek+read).
// The returned slice must NOT be modified when sourced from mmap.
func (b *HugeFileBuffer) readBytesAt(reader io.ReadSeeker, start, end int64) ([]byte, error) {
	if start < 0 || end < start {
		return nil, nil
	}
	if data := b.mmapSlice(start, end); data != nil {
		return data, nil
	}
	if reader == nil {
		return nil, errHugeFileUnavailable
	}
	length := end - start
	if length == 0 {
		return nil, nil
	}
	if _, err := reader.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(reader, buf); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return buf, nil
}

func OpenHugeFileBuffer(path string, meta FileMetadata, fs FileStore) (*HugeFileBuffer, error) {
	if fs == nil {
		return nil, errFileStoreUnavailable()
	}
	byteAnchorSpacing := effectiveHugeFileByteAnchorSpacing(meta.Size)
	reader, err := fs.Open(path)
	if err != nil {
		return nil, err
	}

	// Try to mmap the file for zero-copy reads. Falls back to seek+read if mmap fails.
	mdata, _ := mmapFile(path, meta.Size)

	if cachedIndex, ok := loadHugeFileIndexCache(path, meta); ok {
		b := &HugeFileBuffer{
			path:      path,
			sizeBytes: meta.Size,
			meta:      meta,
			reader:    reader,
			mmapData:  mdata,
			openReader: func() (io.ReadSeekCloser, error) {
				return fs.Open(path)
			},
			lineCount:          cachedIndex.LineCount,
			estimatedLineCount: cachedIndex.EstimatedLineCount,
			checkpoints:        cachedIndex.Checkpoints,
			byteAnchors:        cachedIndex.ByteAnchors,
			pageAnchors:        cachedIndex.PageAnchors,
			byteAnchorSpacing:  byteAnchorSpacing,
			fullyIndexed:       cachedIndex.FullyIndexed,
			cancelIndex:        make(chan struct{}),
			indexDone:          make(chan struct{}),
			lineSpans:    make(map[int]hugeFileLineSpan),
			spanLRU:      newLRUTracker(),
			lineCache:    make(map[int][]rune),
			cacheLRU:     newLRUTracker(),
			lineInfo:     make(map[int]hugeFileLineInfo),
			infoLRU:      newLRUTracker(),
			lineEndings:  make(map[int]string),
			endingLRU:    newLRUTracker(),
			pageData:     make(map[int]hugeFileCachedPageData),
			pageLRU:      newLRUTracker(),
			warmInFlight: make(map[int]struct{}),
		}
		if cachedIndex.FullyIndexed {
			close(b.indexDone)
			return b, nil
		}
		indexReader, err := fs.Open(path)
		if err != nil {
			b.recordIndexError(err)
			close(b.indexDone)
			return b, nil
		}
		startCheckpoint := b.latestCachedAnchor()
		b.workers.Add(1)
		go func() {
			defer b.workers.Done()
			b.buildFullIndex(indexReader, startCheckpoint.offset, startCheckpoint.row)
		}()
		return b, nil
	}

	var initial hugeFileInitialIndex
	var initErr error
	if mdata != nil {
		initial, initErr = buildInitialHugeFileIndexFromMmap(mdata, meta.Size, byteAnchorSpacing)
	} else {
		initial, initErr = buildInitialHugeFileIndex(reader, meta.Size, byteAnchorSpacing)
	}
	if initErr != nil {
		if mdata != nil {
			_ = munmapFile(mdata)
		}
		_ = reader.Close()
		return nil, initErr
	}

	b := &HugeFileBuffer{
		path:      path,
		sizeBytes: meta.Size,
		meta:      meta,
		reader:    reader,
		mmapData:  mdata,
		openReader: func() (io.ReadSeekCloser, error) {
			return fs.Open(path)
		},
		lineCount:          initial.lineCount,
		estimatedLineCount: initial.estimatedLineCount,
		checkpoints:        initial.checkpoints,
		byteAnchors:        initial.byteAnchors,
		pageAnchors:        initial.pageAnchors,
		byteAnchorSpacing:  byteAnchorSpacing,
		fullyIndexed:       initial.fullyIndexed,
		cancelIndex:        make(chan struct{}),
		indexDone:          make(chan struct{}),
		lineSpans:    make(map[int]hugeFileLineSpan),
		spanLRU:      newLRUTracker(),
		lineCache:    make(map[int][]rune),
		cacheLRU:     newLRUTracker(),
		lineInfo:     make(map[int]hugeFileLineInfo),
		infoLRU:      newLRUTracker(),
		lineEndings:  make(map[int]string),
		endingLRU:    newLRUTracker(),
		pageData:     make(map[int]hugeFileCachedPageData),
		pageLRU:      newLRUTracker(),
		warmInFlight: make(map[int]struct{}),
	}
	b.storeLineSpansBatch(initial.lineSpans)

	if initial.fullyIndexed {
		_ = b.persistIndexCache()
		close(b.indexDone)
		return b, nil
	}

	indexReader, err := fs.Open(path)
	if err != nil {
		checkpoints, byteAnchors, pageAnchors, lineCount, fullErr := buildHugeFileLineIndex(reader, byteAnchorSpacing)
		if fullErr != nil {
			_ = reader.Close()
			return nil, fullErr
		}
		b.checkpoints = checkpoints
		b.byteAnchors = byteAnchors
		b.pageAnchors = pageAnchors
		b.lineCount = lineCount
		b.estimatedLineCount = lineCount
		b.fullyIndexed = true
		_ = b.persistIndexCache()
		close(b.indexDone)
		return b, nil
	}

	b.workers.Add(1)
	go func() {
		defer b.workers.Done()
		b.buildFullIndex(indexReader, initial.sampleOffset, initial.lineCount)
	}()
	return b, nil
}

func buildInitialHugeFileIndex(reader io.ReadSeeker, sizeBytes, byteAnchorSpacing int64) (hugeFileInitialIndex, error) {
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return hugeFileInitialIndex{}, err
	}

	limit := hugeFileInitialSampleBytes
	if limit <= 0 || (sizeBytes > 0 && limit > sizeBytes) {
		limit = sizeBytes
	}
	checkpoints := []hugeFileCheckpoint{{row: 0, offset: 0}}
	byteAnchors := []hugeFileCheckpoint{{row: 0, offset: 0}}
	pageAnchors := []hugeFilePageAnchor{{row: 0, offset: 0, lineStart: 0}}
	lineSpans := make([]hugeFileLineSpanEntry, 0, minInt(hugeFileInitialSpanCacheLimit, 256))
	nextByteAnchorOffset := byteAnchorSpacing
	nextPageAnchorOffset := byteAnchorSpacing
	buf := make([]byte, 256<<10)
	var offset int64
	lineCount := 1
	currentRow := 0
	lineStart := int64(0)
	fullyIndexed := false

	for limit <= 0 || offset < limit {
		readSize := len(buf)
		if limit > 0 {
			remaining := limit - offset
			if remaining <= 0 {
				break
			}
			if remaining < int64(readSize) {
				readSize = int(remaining)
			}
		}
		n, err := reader.Read(buf[:readSize])
		if n > 0 {
			for i, b := range buf[:n] {
				nextOffset := offset + int64(i) + 1
				if b != '\n' {
					for nextPageAnchorOffset > 0 && nextOffset >= nextPageAnchorOffset {
						pageAnchors = append(pageAnchors, hugeFilePageAnchor{
							row:       currentRow,
							offset:    nextPageAnchorOffset,
							lineStart: lineStart,
						})
						nextPageAnchorOffset += byteAnchorSpacing
					}
					continue
				}
				if len(lineSpans) < hugeFileInitialSpanCacheLimit {
					lineSpans = append(lineSpans, hugeFileLineSpanEntry{
						row: currentRow,
						span: hugeFileLineSpan{
							start: lineStart,
							end:   offset + int64(i),
						},
					})
				}
				lineCount++
				currentRow = lineCount - 1
				nextLineOffset := nextOffset
				lineStart = nextLineOffset
				if (lineCount-1)%hugeFileCheckpointSpacing == 0 {
					checkpoints = append(checkpoints, hugeFileCheckpoint{
						row:    currentRow,
						offset: nextLineOffset,
					})
				}
				if nextByteAnchorOffset > 0 && nextLineOffset >= nextByteAnchorOffset {
					byteAnchors = append(byteAnchors, hugeFileCheckpoint{
						row:    currentRow,
						offset: nextLineOffset,
					})
					for nextByteAnchorOffset > 0 && nextLineOffset >= nextByteAnchorOffset {
						nextByteAnchorOffset += byteAnchorSpacing
					}
				}
				for nextPageAnchorOffset > 0 && nextOffset >= nextPageAnchorOffset {
					pageAnchors = append(pageAnchors, hugeFilePageAnchor{
						row:       currentRow,
						offset:    nextPageAnchorOffset,
						lineStart: lineStart,
					})
					nextPageAnchorOffset += byteAnchorSpacing
				}
			}
			offset += int64(n)
			if sizeBytes > 0 && offset >= sizeBytes {
				fullyIndexed = true
				break
			}
		}
		if err == io.EOF {
			fullyIndexed = true
			break
		}
		if err != nil {
			return hugeFileInitialIndex{}, err
		}
	}

	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return hugeFileInitialIndex{}, err
	}

	estimated := lineCount
	if !fullyIndexed {
		estimated = estimateHugeFileLineCount(sizeBytes, offset, lineCount)
	} else if len(lineSpans) < hugeFileInitialSpanCacheLimit {
		lineSpans = append(lineSpans, hugeFileLineSpanEntry{
			row: currentRow,
			span: hugeFileLineSpan{
				start: lineStart,
				end:   sizeBytes,
			},
		})
	}

	return hugeFileInitialIndex{
		checkpoints:        checkpoints,
		byteAnchors:        byteAnchors,
		pageAnchors:        pageAnchors,
		lineSpans:          lineSpans,
		lineCount:          lineCount,
		estimatedLineCount: estimated,
		sampleOffset:       offset,
		fullyIndexed:       fullyIndexed,
	}, nil
}

// buildInitialHugeFileIndexFromMmap builds the initial index from mmap'd data
// using bytes.IndexByte for fast newline scanning. Same logic as the reader-based
// version but without any I/O syscalls.
func buildInitialHugeFileIndexFromMmap(data []byte, sizeBytes, byteAnchorSpacing int64) (hugeFileInitialIndex, error) {
	limit := hugeFileInitialSampleBytes
	if limit <= 0 || (sizeBytes > 0 && limit > sizeBytes) {
		limit = sizeBytes
	}
	if limit > int64(len(data)) {
		limit = int64(len(data))
	}

	checkpoints := []hugeFileCheckpoint{{row: 0, offset: 0}}
	byteAnchors := []hugeFileCheckpoint{{row: 0, offset: 0}}
	pageAnchors := []hugeFilePageAnchor{{row: 0, offset: 0, lineStart: 0}}
	lineSpans := make([]hugeFileLineSpanEntry, 0, minInt(hugeFileInitialSpanCacheLimit, 256))
	nextByteAnchorOffset := byteAnchorSpacing
	nextPageAnchorOffset := byteAnchorSpacing
	lineCount := 1
	currentRow := 0
	lineStart := int64(0)
	fullyIndexed := false

	pos := int64(0)
	scanData := data
	if limit > 0 && limit < int64(len(scanData)) {
		scanData = data[:limit]
	}

	for pos < int64(len(scanData)) {
		idx := bytes.IndexByte(scanData[pos:], '\n')
		if idx < 0 {
			// Advance page anchors for remaining data.
			endOffset := int64(len(scanData))
			for nextPageAnchorOffset > 0 && nextPageAnchorOffset <= endOffset {
				pageAnchors = append(pageAnchors, hugeFilePageAnchor{
					row:       currentRow,
					offset:    nextPageAnchorOffset,
					lineStart: lineStart,
				})
				nextPageAnchorOffset += byteAnchorSpacing
			}
			break
		}

		nlOffset := pos + int64(idx)
		nextOffset := nlOffset + 1

		// Advance page anchors up to this newline.
		for nextPageAnchorOffset > 0 && nextPageAnchorOffset <= nextOffset {
			pageAnchors = append(pageAnchors, hugeFilePageAnchor{
				row:       currentRow,
				offset:    nextPageAnchorOffset,
				lineStart: lineStart,
			})
			nextPageAnchorOffset += byteAnchorSpacing
		}

		if len(lineSpans) < hugeFileInitialSpanCacheLimit {
			lineSpans = append(lineSpans, hugeFileLineSpanEntry{
				row:  currentRow,
				span: hugeFileLineSpan{start: lineStart, end: nlOffset},
			})
		}
		lineCount++
		currentRow = lineCount - 1
		lineStart = nextOffset
		if (lineCount-1)%hugeFileCheckpointSpacing == 0 {
			checkpoints = append(checkpoints, hugeFileCheckpoint{
				row:    currentRow,
				offset: nextOffset,
			})
		}
		if nextByteAnchorOffset > 0 && nextOffset >= nextByteAnchorOffset {
			byteAnchors = append(byteAnchors, hugeFileCheckpoint{
				row:    currentRow,
				offset: nextOffset,
			})
			for nextByteAnchorOffset > 0 && nextOffset >= nextByteAnchorOffset {
				nextByteAnchorOffset += byteAnchorSpacing
			}
		}

		pos = nextOffset
	}

	if sizeBytes > 0 && (limit <= 0 || limit >= sizeBytes) {
		fullyIndexed = true
	}

	estimated := lineCount
	if !fullyIndexed {
		sampledBytes := limit
		if sampledBytes <= 0 {
			sampledBytes = int64(len(scanData))
		}
		estimated = estimateHugeFileLineCount(sizeBytes, sampledBytes, lineCount)
	} else if len(lineSpans) < hugeFileInitialSpanCacheLimit {
		lineSpans = append(lineSpans, hugeFileLineSpanEntry{
			row:  currentRow,
			span: hugeFileLineSpan{start: lineStart, end: sizeBytes},
		})
	}

	sampleOffset := int64(len(scanData))

	return hugeFileInitialIndex{
		checkpoints:        checkpoints,
		byteAnchors:        byteAnchors,
		pageAnchors:        pageAnchors,
		lineSpans:          lineSpans,
		lineCount:          lineCount,
		estimatedLineCount: estimated,
		sampleOffset:       sampleOffset,
		fullyIndexed:       fullyIndexed,
	}, nil
}

func buildHugeFileLineIndex(reader io.ReadSeeker, byteAnchorSpacing int64) ([]hugeFileCheckpoint, []hugeFileCheckpoint, []hugeFilePageAnchor, int, error) {
	initial, err := buildInitialHugeFileIndex(reader, 0, byteAnchorSpacing)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	return initial.checkpoints, initial.byteAnchors, initial.pageAnchors, initial.lineCount, nil
}

func effectiveHugeFileByteAnchorSpacing(sizeBytes int64) int64 {
	if hugeFileByteAnchorSpacingOverride > 0 {
		return hugeFileByteAnchorSpacingOverride
	}
	minSpacing := hugeFileMinByteAnchorSpacing
	if minSpacing <= 0 {
		minSpacing = 64 << 10
	}
	maxSpacing := hugeFileMaxByteAnchorSpacing
	if maxSpacing > 0 && maxSpacing < minSpacing {
		maxSpacing = minSpacing
	}
	if sizeBytes <= 0 || hugeFileTargetByteAnchors <= 0 {
		if maxSpacing > 0 && minSpacing > maxSpacing {
			return maxSpacing
		}
		return minSpacing
	}
	spacing := (sizeBytes + hugeFileTargetByteAnchors - 1) / hugeFileTargetByteAnchors
	if spacing < minSpacing {
		spacing = minSpacing
	}
	if maxSpacing > 0 && spacing > maxSpacing {
		spacing = maxSpacing
	}
	if rem := spacing % minSpacing; rem != 0 {
		spacing += minSpacing - rem
	}
	if maxSpacing > 0 && spacing > maxSpacing {
		return maxSpacing
	}
	return spacing
}

func estimateHugeFileLineCount(sizeBytes, sampledBytes int64, sampledLines int) int {
	if sampledLines < 1 {
		sampledLines = 1
	}
	if sampledBytes <= 0 || sizeBytes <= 0 {
		return sampledLines
	}
	avgBytesPerLine := float64(sampledBytes) / float64(sampledLines)
	if avgBytesPerLine < 1 {
		avgBytesPerLine = 1
	}
	estimate := int(math.Ceil(float64(sizeBytes) / avgBytesPerLine * 1.15))
	if estimate < sampledLines {
		estimate = sampledLines
	}
	return estimate
}

func (b *HugeFileBuffer) buildFullIndex(reader io.ReadSeekCloser, startOffset int64, startLineCount int) {
	defer close(b.indexDone)
	defer reader.Close()

	// mmap fast path: scan the byte slice directly.
	if b.mmapData != nil {
		b.buildFullIndexFromMmap(startOffset, startLineCount)
		return
	}

	if _, err := reader.Seek(startOffset, io.SeekStart); err != nil {
		b.recordIndexError(err)
		return
	}

	buf := make([]byte, 1<<20)
	var offset = startOffset
	lineCount := startLineCount
	currentRow := startLineCount - 1
	if currentRow < 0 {
		currentRow = 0
	}
	lineStart := startOffset
	pending := make([]hugeFileCheckpoint, 0, hugeFileIndexCheckpointBatch)
	nextByteAnchorOffset := b.nextByteAnchorOffset(startOffset)
	nextPageAnchorOffset := b.nextPageAnchorOffset(startOffset)
	lastPersist := time.Now()

	for {
		select {
		case <-b.cancelIndex:
			if len(pending) > 0 {
				b.appendCheckpoints(pending, lineCount)
			}
			_ = b.persistIndexCache()
			return
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			for i, ch := range buf[:n] {
				nextOffset := offset + int64(i) + 1
				if ch != '\n' {
					for nextPageAnchorOffset > 0 && nextOffset >= nextPageAnchorOffset {
						b.appendPageAnchor(currentRow, nextPageAnchorOffset, lineStart)
						nextPageAnchorOffset += b.byteAnchorSpacing
					}
					continue
				}
				lineCount++
				currentRow = lineCount - 1
				nextLineOffset := nextOffset
				lineStart = nextLineOffset
				if (lineCount-1)%hugeFileCheckpointSpacing == 0 {
					pending = append(pending, hugeFileCheckpoint{
						row:    currentRow,
						offset: nextLineOffset,
					})
				}
				if nextByteAnchorOffset > 0 && nextLineOffset >= nextByteAnchorOffset {
					b.appendByteAnchor(currentRow, nextLineOffset)
					for nextByteAnchorOffset > 0 && nextLineOffset >= nextByteAnchorOffset {
						nextByteAnchorOffset += b.byteAnchorSpacing
					}
				}
				for nextPageAnchorOffset > 0 && nextOffset >= nextPageAnchorOffset {
					b.appendPageAnchor(currentRow, nextPageAnchorOffset, lineStart)
					nextPageAnchorOffset += b.byteAnchorSpacing
				}
			}
			offset += int64(n)
			b.setIndexedLineCount(lineCount)
		}
		if len(pending) >= hugeFileIndexCheckpointBatch {
			b.appendCheckpoints(pending, lineCount)
			pending = pending[:0]
			if time.Since(lastPersist) >= hugeFileIndexPersistInterval {
				_ = b.persistIndexCache()
				lastPersist = time.Now()
			}
		}
		if err == io.EOF {
			if len(pending) > 0 {
				b.appendCheckpoints(pending, lineCount)
			}
			b.completeIndex(lineCount)
			return
		}
		if err != nil {
			if len(pending) > 0 {
				b.appendCheckpoints(pending, lineCount)
			}
			_ = b.persistIndexCache()
			b.recordIndexError(err)
			return
		}
	}
}

// buildFullIndexFromMmap scans the mmap'd data using parallel bytes.Count for
// newline counting, then a sequential bytes.IndexByte pass to build checkpoints
// and anchors. For files over ~16 MB, the parallel phase uses all CPU cores.
func (b *HugeFileBuffer) buildFullIndexFromMmap(startOffset int64, startLineCount int) {
	data := b.mmapData
	size := int64(len(data))
	if startOffset < 0 {
		startOffset = 0
	}

	// --- Phase 1: parallel newline count (for progress estimation) ----------
	// Split the data from startOffset..size across NumCPU goroutines.
	// Each goroutine uses bytes.Count which is SIMD-accelerated.
	const parallelThreshold = 16 << 20 // 16 MB
	remaining := size - startOffset
	numCPU := runtime.NumCPU()
	if numCPU < 1 {
		numCPU = 1
	}
	totalNewlines := 0

	if remaining >= parallelThreshold && numCPU > 1 {
		chunkSize := remaining / int64(numCPU)
		if chunkSize < 1<<20 {
			chunkSize = 1 << 20
		}
		nChunks := int(remaining / chunkSize)
		if nChunks < 1 {
			nChunks = 1
		}
		counts := make([]int, nChunks)
		var wg sync.WaitGroup
		for i := 0; i < nChunks; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				cStart := startOffset + int64(idx)*chunkSize
				cEnd := cStart + chunkSize
				if idx == nChunks-1 {
					cEnd = size
				}
				counts[idx] = bytes.Count(data[cStart:cEnd], []byte{'\n'})
			}(i)
		}
		wg.Wait()
		for _, c := range counts {
			totalNewlines += c
		}
	} else {
		totalNewlines = bytes.Count(data[startOffset:], []byte{'\n'})
	}

	// Pre-size slices to avoid repeated growth during sequential scan.
	estimatedLines := startLineCount + totalNewlines
	_ = estimatedLines // used for lineCount convergence

	// --- Phase 2: sequential scan with batched anchor/checkpoint appends ----
	lineCount := startLineCount
	currentRow := startLineCount - 1
	if currentRow < 0 {
		currentRow = 0
	}
	lineStart := startOffset

	pendingCp := make([]hugeFileCheckpoint, 0, hugeFileIndexCheckpointBatch)
	pendingBA := make([]hugeFileCheckpoint, 0, 256)
	pendingPA := make([]hugeFilePageAnchor, 0, 256)

	nextByteAnchorOffset := b.nextByteAnchorOffset(startOffset)
	nextPageAnchorOffset := b.nextPageAnchorOffset(startOffset)
	lastPersist := time.Now()

	// We process in large scan segments (~4 MB) to amortize cancel checks.
	const scanSegment = 4 << 20
	pos := startOffset

	for pos < size {
		// Check cancel between segments.
		select {
		case <-b.cancelIndex:
			b.flushIndexBatch(pendingCp, pendingBA, pendingPA, lineCount)
			_ = b.persistIndexCache()
			return
		default:
		}

		segEnd := pos + scanSegment
		if segEnd > size {
			segEnd = size
		}
		seg := data[pos:segEnd]

		scanPos := 0
		for scanPos < len(seg) {
			idx := bytes.IndexByte(seg[scanPos:], '\n')
			if idx < 0 {
				break
			}

			nlAbs := pos + int64(scanPos) + int64(idx)
			nextOffset := nlAbs + 1

			// Page anchors up to this newline.
			for nextPageAnchorOffset > 0 && nextPageAnchorOffset <= nextOffset {
				pendingPA = append(pendingPA, hugeFilePageAnchor{
					row: currentRow, offset: nextPageAnchorOffset, lineStart: lineStart,
				})
				nextPageAnchorOffset += b.byteAnchorSpacing
			}

			lineCount++
			currentRow = lineCount - 1
			lineStart = nextOffset

			if (lineCount-1)%hugeFileCheckpointSpacing == 0 {
				pendingCp = append(pendingCp, hugeFileCheckpoint{
					row: currentRow, offset: nextOffset,
				})
			}

			if nextByteAnchorOffset > 0 && nextOffset >= nextByteAnchorOffset {
				pendingBA = append(pendingBA, hugeFileCheckpoint{
					row: currentRow, offset: nextOffset,
				})
				for nextByteAnchorOffset > 0 && nextOffset >= nextByteAnchorOffset {
					nextByteAnchorOffset += b.byteAnchorSpacing
				}
			}

			scanPos += idx + 1
		}

		pos = segEnd

		// Flush batches when they're large enough (single lock acquisition each).
		if len(pendingCp) >= hugeFileIndexCheckpointBatch {
			b.appendCheckpoints(pendingCp, lineCount)
			pendingCp = pendingCp[:0]
			b.setIndexedLineCount(lineCount)
		}
		if len(pendingBA) >= 128 {
			b.appendByteAnchorsBatch(pendingBA)
			pendingBA = pendingBA[:0]
		}
		if len(pendingPA) >= 128 {
			b.appendPageAnchorsBatch(pendingPA)
			pendingPA = pendingPA[:0]
		}
		if time.Since(lastPersist) >= hugeFileIndexPersistInterval {
			_ = b.persistIndexCache()
			lastPersist = time.Now()
		}
	}

	// Handle trailing page anchors for data after last newline.
	endOffset := size
	for nextPageAnchorOffset > 0 && nextPageAnchorOffset <= endOffset {
		pendingPA = append(pendingPA, hugeFilePageAnchor{
			row: currentRow, offset: nextPageAnchorOffset, lineStart: lineStart,
		})
		nextPageAnchorOffset += b.byteAnchorSpacing
	}

	b.flushIndexBatch(pendingCp, pendingBA, pendingPA, lineCount)
	b.completeIndex(lineCount)
}

// flushIndexBatch writes any remaining pending checkpoints and anchors.
func (b *HugeFileBuffer) flushIndexBatch(
	pendingCp []hugeFileCheckpoint,
	pendingBA []hugeFileCheckpoint,
	pendingPA []hugeFilePageAnchor,
	lineCount int,
) {
	if len(pendingCp) > 0 {
		b.appendCheckpoints(pendingCp, lineCount)
	}
	if len(pendingBA) > 0 {
		b.appendByteAnchorsBatch(pendingBA)
	}
	if len(pendingPA) > 0 {
		b.appendPageAnchorsBatch(pendingPA)
	}
}

func (b *HugeFileBuffer) appendCheckpoints(checkpoints []hugeFileCheckpoint, lineCount int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if lineCount > b.lineCount {
		b.lineCount = lineCount
	}
	lastRow := -1
	if len(b.checkpoints) > 0 {
		lastRow = b.checkpoints[len(b.checkpoints)-1].row
	}
	for _, checkpoint := range checkpoints {
		if checkpoint.row <= lastRow {
			continue
		}
		b.checkpoints = append(b.checkpoints, checkpoint)
		lastRow = checkpoint.row
	}
}

func (b *HugeFileBuffer) appendByteAnchor(row int, offset int64) {
	if b == nil || b.byteAnchorSpacing <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.byteAnchors) == 0 {
		b.byteAnchors = []hugeFileCheckpoint{{row: 0, offset: 0}}
	}
	last := b.byteAnchors[len(b.byteAnchors)-1]
	if offset <= last.offset {
		return
	}
	b.byteAnchors = append(b.byteAnchors, hugeFileCheckpoint{row: row, offset: offset})
}

func (b *HugeFileBuffer) appendByteAnchorsBatch(anchors []hugeFileCheckpoint) {
	if b == nil || b.byteAnchorSpacing <= 0 || len(anchors) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.byteAnchors) == 0 {
		b.byteAnchors = []hugeFileCheckpoint{{row: 0, offset: 0}}
	}
	lastOffset := b.byteAnchors[len(b.byteAnchors)-1].offset
	for _, a := range anchors {
		if a.offset > lastOffset {
			b.byteAnchors = append(b.byteAnchors, a)
			lastOffset = a.offset
		}
	}
}

func (b *HugeFileBuffer) appendPageAnchor(row int, offset, lineStart int64) {
	if b == nil || b.byteAnchorSpacing <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.pageAnchors) == 0 {
		b.pageAnchors = []hugeFilePageAnchor{{row: 0, offset: 0, lineStart: 0}}
	}
	last := b.pageAnchors[len(b.pageAnchors)-1]
	if offset <= last.offset {
		return
	}
	b.pageAnchors = append(b.pageAnchors, hugeFilePageAnchor{
		row:       row,
		offset:    offset,
		lineStart: lineStart,
	})
}

func (b *HugeFileBuffer) appendPageAnchorsBatch(anchors []hugeFilePageAnchor) {
	if b == nil || b.byteAnchorSpacing <= 0 || len(anchors) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.pageAnchors) == 0 {
		b.pageAnchors = []hugeFilePageAnchor{{row: 0, offset: 0, lineStart: 0}}
	}
	lastOffset := b.pageAnchors[len(b.pageAnchors)-1].offset
	for _, a := range anchors {
		if a.offset > lastOffset {
			b.pageAnchors = append(b.pageAnchors, a)
			lastOffset = a.offset
		}
	}
}

func (b *HugeFileBuffer) setIndexedLineCount(lineCount int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if lineCount > b.lineCount {
		b.lineCount = lineCount
	}
}

func (b *HugeFileBuffer) completeIndex(lineCount int) {
	b.mu.Lock()
	b.lineCount = lineCount
	b.estimatedLineCount = lineCount
	b.fullyIndexed = true
	b.mu.Unlock()
	_ = b.persistIndexCache()
}

func (b *HugeFileBuffer) recordIndexError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.indexErr = err
}

func (b *HugeFileBuffer) persistIndexCache() error {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	checkpoints := append([]hugeFileCheckpoint(nil), b.checkpoints...)
	byteAnchors := append([]hugeFileCheckpoint(nil), b.byteAnchors...)
	pageAnchors := append([]hugeFilePageAnchor(nil), b.pageAnchors...)
	lineCount := b.lineCount
	estimatedLineCount := b.estimatedLineCount
	fullyIndexed := b.fullyIndexed
	meta := b.meta
	path := b.path
	b.mu.RUnlock()
	return saveHugeFileIndexCache(path, meta, checkpoints, byteAnchors, pageAnchors, lineCount, estimatedLineCount, fullyIndexed)
}

func (b *HugeFileBuffer) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	cancelIndex := b.cancelIndex
	b.closed = true
	b.mu.Unlock()
	if cancelIndex != nil {
		close(cancelIndex)
	}
	if b.indexDone != nil {
		<-b.indexDone
	}
	b.workers.Wait()
	if b.mmapData != nil {
		_ = munmapFile(b.mmapData)
		b.mmapData = nil
	}
	if b.reader == nil {
		return nil
	}
	err := b.reader.Close()
	b.mu.Lock()
	b.reader = nil
	b.openReader = nil
	b.cancelIndex = nil
	b.lineSpans = nil
	b.spanLRU = lruTracker{}
	b.spanSorted = nil
	b.lineCache = nil
	b.cacheLRU = lruTracker{}
	b.lineInfo = nil
	b.infoLRU = lruTracker{}
	b.lineEndings = nil
	b.endingLRU = lruTracker{}
	b.pageData = nil
	b.pageLRU = lruTracker{}
	b.warmInFlight = nil
	b.mu.Unlock()
	return err
}

func (b *HugeFileBuffer) Path() string {
	if b == nil {
		return ""
	}
	return b.path
}

func (b *HugeFileBuffer) OpenReader() (io.ReadSeekCloser, error) {
	if b == nil || b.openReader == nil {
		return nil, errHugeFileUnavailable
	}
	b.mu.RLock()
	closed := b.closed
	openReader := b.openReader
	b.mu.RUnlock()
	if closed {
		return nil, errHugeFileUnavailable
	}
	return openReader()
}

func (b *HugeFileBuffer) SizeBytes() int64 {
	if b == nil {
		return 0
	}
	return b.sizeBytes
}

func (b *HugeFileBuffer) LineCount() int {
	if b == nil {
		return 1
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.lineCount <= 0 {
		return 1
	}
	if b.fullyIndexed || b.estimatedLineCount <= b.lineCount {
		return b.lineCount
	}
	return b.estimatedLineCount
}

func (b *HugeFileBuffer) IndexingComplete() bool {
	if b == nil {
		return true
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.fullyIndexed
}

func (b *HugeFileBuffer) IndexedLineCount() int {
	if b == nil {
		return 1
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.lineCount <= 0 {
		return 1
	}
	return b.lineCount
}

func (b *HugeFileBuffer) CanPrefetchQuick(startRow, count int) bool {
	if b == nil || count <= 0 {
		return false
	}
	if startRow < 0 {
		startRow = 0
	}
	endRow := startRow + count - 1 + hugeFileLinePrefetch
	if endRow < startRow {
		endRow = startRow
	}
	return b.canResolveLineQuick(endRow)
}

func (b *HugeFileBuffer) TryLine(row int) ([]rune, bool) {
	if b == nil || (b.reader == nil && b.mmapData == nil) {
		return nil, false
	}
	if !b.canResolveLineQuick(row) {
		return nil, false
	}
	return b.Line(row), true
}

func (b *HugeFileBuffer) TryCachedLine(row int) ([]rune, bool) {
	if b == nil || (b.reader == nil && b.mmapData == nil) {
		return nil, false
	}
	lineCount := b.LineCount()
	if lineCount <= 0 {
		return nil, false
	}
	if row < 0 {
		row = 0
	}
	if row >= lineCount {
		row = lineCount - 1
	}
	if cached, ok := b.cachedLine(row); ok {
		return cached, true
	}
	span, ok := b.peekLineSpan(row)
	if !ok {
		return nil, false
	}
	if line, ok := b.cachedPageLine(row, span); ok {
		return line, true
	}
	return nil, false
}

func (b *HugeFileBuffer) TryCachedLineInfo(row int) (hugeFileLineInfo, bool) {
	if b == nil || (b.reader == nil && b.mmapData == nil) {
		return hugeFileLineInfo{}, false
	}
	lineCount := b.LineCount()
	if lineCount <= 0 {
		return hugeFileLineInfo{}, false
	}
	if row < 0 {
		row = 0
	}
	if row >= lineCount {
		row = lineCount - 1
	}
	if info, ok := b.cachedLineInfo(row); ok {
		return info, true
	}
	span, ok := b.peekLineSpan(row)
	if !ok {
		return hugeFileLineInfo{}, false
	}
	return b.cachedPageLineInfo(row, span)
}

func (b *HugeFileBuffer) TryCachedLineSegment(row, startCol, maxCols int) ([]rune, bool) {
	if b == nil || (b.reader == nil && b.mmapData == nil) {
		return nil, false
	}
	lineCount := b.LineCount()
	if lineCount <= 0 {
		return nil, false
	}
	if row < 0 {
		row = 0
	}
	if row >= lineCount {
		row = lineCount - 1
	}
	if maxCols <= 0 {
		return []rune{}, true
	}
	info, ok := b.TryCachedLineInfo(row)
	if !ok || !info.asciiOnly || info.hasTabs {
		return nil, false
	}
	if startCol < 0 {
		startCol = 0
	}
	if startCol > info.runeLen {
		startCol = info.runeLen
	}
	endCol := startCol + maxCols
	if endCol > info.runeLen {
		endCol = info.runeLen
	}
	if endCol <= startCol {
		return []rune{}, true
	}
	if cached, ok := b.cachedLine(row); ok {
		return append([]rune(nil), cached[startCol:endCol]...), true
	}
	page, ok := b.cachedPageData(row)
	if !ok {
		return nil, false
	}
	return b.cachedPageLineSegment(row, page, startCol, endCol)
}

func (b *HugeFileBuffer) WaitForIndexing() {
	if b == nil || b.indexDone == nil {
		return
	}
	<-b.indexDone
}

func (b *HugeFileBuffer) canResolveLineQuick(row int) bool {
	if row < 0 {
		return false
	}
	if b.hasCachedLine(row) {
		return true
	}
	if b.hasCachedLineSpan(row) {
		return true
	}
	if b.hasCachedPageRange(row, row) {
		return true
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.fullyIndexed {
		return row < b.lineCount
	}
	if row >= b.estimatedLineCount {
		return false
	}
	highestIndexedRow := b.lineCount - 1
	if highestIndexedRow < 0 {
		highestIndexedRow = 0
	}
	return row <= highestIndexedRow+hugeFileCheckpointSpacing
}

func (b *HugeFileBuffer) LineLen(row int) int {
	info, ok := b.LineInfo(row)
	if !ok {
		return len(b.Line(row))
	}
	return info.runeLen
}

func (b *HugeFileBuffer) RawSpanForRows(startRow, endRow int) (int64, int64, error) {
	if b == nil {
		return 0, 0, errHugeFileUnavailable
	}
	lineCount := b.LineCount()
	if startRow < 0 {
		startRow = 0
	}
	if endRow < startRow {
		endRow = startRow
	}
	if startRow > lineCount {
		startRow = lineCount
	}
	if endRow > lineCount {
		endRow = lineCount
	}

	var start int64
	if startRow >= lineCount {
		start = b.sizeBytes
	} else {
		span, err := b.resolveLineSpan(startRow)
		if err != nil {
			return 0, 0, err
		}
		start = span.start
	}

	var end int64
	if endRow >= lineCount {
		end = b.sizeBytes
	} else {
		span, err := b.resolveLineSpan(endRow)
		if err != nil {
			return 0, 0, err
		}
		end = span.start
	}
	if end < start {
		end = start
	}
	return start, end, nil
}

func (b *HugeFileBuffer) Line(row int) []rune {
	if b == nil || (b.reader == nil && b.mmapData == nil) {
		return nil
	}
	lineCount := b.LineCount()
	if lineCount <= 0 {
		return nil
	}
	if row < 0 {
		row = 0
	}
	if row >= lineCount {
		row = lineCount - 1
	}
	if cached, ok := b.cachedLine(row); ok {
		return cached
	}
	if _, ok := b.cachedPageData(row); ok {
		b.populateCachesFromPageData(row)
		if cached, ok := b.cachedLine(row); ok {
			return cached
		}
	}

	span, err := b.resolveLineSpan(row)
	if err != nil {
		if errors.Is(err, errHugeFileLineOutOfRange) {
			return nil
		}
		return []rune(fmt.Sprintf("[read error: %v]", err))
	}
	if line, ok := b.cachedPageLine(row, span); ok {
		return line
	}
	if err := b.cachePageDataFromReader(b.reader, row); err == nil {
		b.populateCachesFromPageData(row)
		if cached, ok := b.cachedLine(row); ok {
			return cached
		}
		if line, ok := b.cachedPageLine(row, span); ok {
			return line
		}
	} else if !errors.Is(err, errHugeFileLineOutOfRange) {
		return []rune(fmt.Sprintf("[read error: %v]", err))
	}
	if err := b.cachePageLinesFromReader(b.reader, row); err == nil {
		if cached, ok := b.cachedLine(row); ok {
			return cached
		}
	} else if !errors.Is(err, errHugeFileLineOutOfRange) {
		return []rune(fmt.Sprintf("[read error: %v]", err))
	}
	data, err := b.readBytesAt(b.reader, span.start, span.end)
	if err != nil {
		return []rune(fmt.Sprintf("[read error: %v]", err))
	}
	if len(data) > 0 && data[len(data)-1] == '\r' {
		data = data[:len(data)-1]
	}
	line := bytesToRunes(data)
	b.storeCachedLine(row, line)
	b.storeCachedLineInfo(row, analyzeHugeFileLineData(data))
	return line
}

func (b *HugeFileBuffer) LineInfo(row int) (hugeFileLineInfo, bool) {
	if b == nil || (b.reader == nil && b.mmapData == nil) {
		return hugeFileLineInfo{}, false
	}
	lineCount := b.LineCount()
	if lineCount <= 0 {
		return hugeFileLineInfo{}, false
	}
	if row < 0 {
		row = 0
	}
	if row >= lineCount {
		row = lineCount - 1
	}
	if info, ok := b.cachedLineInfo(row); ok {
		return info, true
	}
	if page, ok := b.cachedPageData(row); ok {
		if span, ok := page.spanForRow(row); ok {
			return b.cachedPageLineInfo(row, span)
		}
	}

	span, err := b.resolveLineSpan(row)
	if err != nil {
		return hugeFileLineInfo{}, false
	}
	if span.end <= span.start {
		info := hugeFileLineInfo{asciiOnly: true}
		b.storeCachedLineInfo(row, info)
		return info, true
	}
	if info, ok := b.cachedPageLineInfo(row, span); ok {
		return info, true
	}
	if span.end-span.start <= hugeFileDirectReadThreshold && b.cachePageDataFromReader(b.reader, row) == nil {
		if info, ok := b.cachedPageLineInfo(row, span); ok {
			return info, true
		}
	}
	info, err := b.analyzeLineSpan(row, span)
	if err != nil {
		return hugeFileLineInfo{}, false
	}
	b.storeCachedLineInfo(row, info)
	return info, true
}

func (b *HugeFileBuffer) LineSegment(row, startCol, maxCols int) ([]rune, bool) {
	if b == nil || (b.reader == nil && b.mmapData == nil) {
		return nil, false
	}
	lineCount := b.LineCount()
	if lineCount <= 0 {
		return nil, false
	}
	if row < 0 {
		row = 0
	}
	if row >= lineCount {
		row = lineCount - 1
	}
	if maxCols <= 0 {
		return []rune{}, true
	}

	info, ok := b.LineInfo(row)
	if !ok || !info.asciiOnly || info.hasTabs {
		return nil, false
	}
	if startCol < 0 {
		startCol = 0
	}
	if startCol > info.runeLen {
		startCol = info.runeLen
	}
	endCol := startCol + maxCols
	if endCol > info.runeLen {
		endCol = info.runeLen
	}
	if endCol <= startCol {
		return []rune{}, true
	}

	if cached, ok := b.cachedLine(row); ok {
		if startCol >= len(cached) {
			return []rune{}, true
		}
		if endCol > len(cached) {
			endCol = len(cached)
		}
		return append([]rune(nil), cached[startCol:endCol]...), true
	}

	if page, ok := b.cachedPageData(row); ok {
		if segment, ok := b.cachedPageLineSegment(row, page, startCol, endCol); ok {
			return segment, true
		}
	}

	span, err := b.resolveLineSpan(row)
	if err != nil {
		return nil, false
	}
	if span.end-span.start <= hugeFileDirectReadThreshold && b.cachePageDataFromReader(b.reader, row) == nil {
		if page, ok := b.cachedPageData(row); ok {
			if segment, ok := b.cachedPageLineSegment(row, page, startCol, endCol); ok {
				return segment, true
			}
		}
	}

	readStart := span.start + int64(startCol)
	readEnd := span.start + int64(endCol)
	if readStart < span.start {
		readStart = span.start
	}
	if readEnd <= readStart {
		return []rune{}, true
	}
	data, err := b.readBytesAt(b.reader, readStart, readEnd)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	segment := make([]rune, len(data))
	for i, ch := range data {
		segment[i] = rune(ch)
	}
	return segment, true
}

func (b *HugeFileBuffer) LinePrefix(row, maxBytes int) ([]rune, bool) {
	if b == nil || (b.reader == nil && b.mmapData == nil) {
		return nil, false
	}
	lineCount := b.LineCount()
	if lineCount <= 0 {
		return nil, false
	}
	if row < 0 {
		row = 0
	}
	if row >= lineCount {
		row = lineCount - 1
	}
	if maxBytes <= 0 {
		return []rune{}, true
	}

	if cached, ok := b.cachedLine(row); ok {
		if len(cached) <= maxBytes {
			return append([]rune(nil), cached...), true
		}
		return append([]rune(nil), cached[:maxBytes]...), true
	}
	if page, ok := b.cachedPageData(row); ok {
		if prefix, ok := b.cachedPageLinePrefix(row, page, maxBytes); ok {
			return prefix, true
		}
	}
	span, err := b.resolveLineSpan(row)
	if err != nil {
		return nil, false
	}
	if span.end <= span.start {
		return []rune{}, true
	}
	if span.end-span.start <= hugeFileDirectReadThreshold && b.cachePageDataFromReader(b.reader, row) == nil {
		if page, ok := b.cachedPageData(row); ok {
			if prefix, ok := b.cachedPageLinePrefix(row, page, maxBytes); ok {
				return prefix, true
			}
		}
	}

	readLen := span.end - span.start
	if readLen > int64(maxBytes) {
		readLen = int64(maxBytes)
	}
	if readLen <= 0 {
		return []rune{}, true
	}
	data, err := b.readBytesAt(b.reader, span.start, span.start+readLen)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return bytesToRunes(data), true
}

func (b *HugeFileBuffer) analyzeLineSpan(row int, span hugeFileLineSpan) (hugeFileLineInfo, error) {
	if b == nil || (b.reader == nil && b.mmapData == nil) {
		return hugeFileLineInfo{}, errHugeFileUnavailable
	}
	if span.end < span.start {
		span.end = span.start
	}
	if span.end == span.start {
		return hugeFileLineInfo{asciiOnly: true}, nil
	}

	// With mmap, we can analyze the data directly without copying.
	if data := b.mmapSlice(span.start, span.end); data != nil {
		if len(data) > 0 && data[len(data)-1] == '\r' {
			data = data[:len(data)-1]
		}
		return analyzeHugeFileLineData(data), nil
	}

	// Fallback: chunked read via reader.
	if b.reader == nil {
		return hugeFileLineInfo{}, errHugeFileUnavailable
	}
	length := span.end - span.start
	if _, err := b.reader.Seek(span.start, io.SeekStart); err != nil {
		return hugeFileLineInfo{}, err
	}

	buf := make([]byte, 64<<10)
	pending := make([]byte, 0, utf8.UTFMax)
	info := hugeFileLineInfo{asciiOnly: true}
	var read int64

	for read < length {
		chunkSize := len(buf)
		remaining := length - read
		if remaining < int64(chunkSize) {
			chunkSize = int(remaining)
		}
		n, err := io.ReadFull(b.reader, buf[:chunkSize])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return hugeFileLineInfo{}, err
		}
		if n == 0 {
			break
		}
		read += int64(n)
		chunk := buf[:n]
		if read == length && len(chunk) > 0 && chunk[len(chunk)-1] == '\r' {
			chunk = chunk[:len(chunk)-1]
		}

		data := chunk
		if len(pending) > 0 {
			combined := make([]byte, 0, len(pending)+len(chunk))
			combined = append(combined, pending...)
			combined = append(combined, chunk...)
			data = combined
			pending = pending[:0]
		}
		for len(data) > 0 {
			if data[0] < utf8.RuneSelf {
				if data[0] == '\t' {
					info.hasTabs = true
				}
				info.runeLen++
				data = data[1:]
				continue
			}
			if !utf8.FullRune(data) && read < length {
				pending = append(pending[:0], data...)
				break
			}
			info.asciiOnly = false
			_, size := utf8.DecodeRune(data)
			if size < 1 {
				size = 1
			}
			info.runeLen++
			data = data[size:]
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
	}
	for len(pending) > 0 {
		if pending[0] < utf8.RuneSelf {
			if pending[0] == '\t' {
				info.hasTabs = true
			}
			info.runeLen++
			pending = pending[1:]
			continue
		}
		info.asciiOnly = false
		_, size := utf8.DecodeRune(pending)
		if size < 1 {
			size = 1
		}
		info.runeLen++
		pending = pending[size:]
	}
	return info, nil
}

func (b *HugeFileBuffer) LineEnding(row int) string {
	if b == nil || (b.reader == nil && b.mmapData == nil) {
		return ""
	}
	lineCount := b.LineCount()
	if row < 0 || row >= lineCount-1 {
		return ""
	}
	if eol, ok := b.cachedLineEnding(row); ok {
		return eol
	}
	if _, ok := b.cachedPageData(row); ok {
		b.populateCachesFromPageData(row)
		if eol, ok := b.cachedLineEnding(row); ok {
			return eol
		}
	}
	span, err := b.resolveLineSpan(row)
	if err != nil {
		return ""
	}
	nextSpan, err := b.resolveLineSpan(row + 1)
	if err != nil {
		return ""
	}
	if eol, ok := b.cachedPageLineEnding(row, span, nextSpan); ok {
		return eol
	}
	if err := b.cachePageDataFromReader(b.reader, row); err == nil {
		b.populateCachesFromPageData(row)
		if eol, ok := b.cachedLineEnding(row); ok {
			return eol
		}
		if eol, ok := b.cachedPageLineEnding(row, span, nextSpan); ok {
			return eol
		}
	}
	if err := b.cachePageLinesFromReader(b.reader, row); err == nil {
		if eol, ok := b.cachedLineEnding(row); ok {
			return eol
		}
	}
	delimLen := nextSpan.start - span.end
	if delimLen <= 0 {
		return ""
	}
	prefixLen := int64(0)
	if span.end > span.start {
		prefixLen = 1
	}
	data, err := b.readBytesAt(b.reader, span.end-prefixLen, span.end-prefixLen+prefixLen+delimLen)
	if err != nil || len(data) == 0 {
		return ""
	}
	eol := string(data[prefixLen:])
	if prefixLen == 1 && len(data) > 0 && data[0] == '\r' {
		eol = "\r" + eol
	}
	b.storeCachedLineEnding(row, eol)
	return eol
}

func (b *HugeFileBuffer) resolveLineSpan(row int) (hugeFileLineSpan, error) {
	if cached, ok := b.cachedLineSpan(row); ok {
		return cached, nil
	}
	if span, ok := b.cachedPageSpan(row); ok {
		return span, nil
	}
	if err := b.cachePageLineSpansFromReader(b.reader, row); err == nil {
		if span, ok := b.cachedLineSpan(row); ok {
			return span, nil
		}
	} else if !errors.Is(err, errHugeFileLineOutOfRange) {
		return hugeFileLineSpan{}, err
	}
	if err := b.cacheLineSpansFromReader(b.reader, row, row+hugeFileLinePrefetch); err != nil {
		return hugeFileLineSpan{}, err
	}
	if span, ok := b.cachedLineSpan(row); ok {
		return span, nil
	}
	return hugeFileLineSpan{}, errHugeFileLineOutOfRange
}

func (b *HugeFileBuffer) nearestCheckpoint(row int) hugeFileCheckpoint {
	b.mu.RLock()
	defer b.mu.RUnlock()

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

func (b *HugeFileBuffer) nearestByteAnchor(row int) hugeFileCheckpoint {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.byteAnchors) == 0 {
		return hugeFileCheckpoint{row: 0, offset: 0}
	}
	idx := sort.Search(len(b.byteAnchors), func(i int) bool {
		return b.byteAnchors[i].row > row
	})
	if idx == 0 {
		return b.byteAnchors[0]
	}
	return b.byteAnchors[idx-1]
}

func (b *HugeFileBuffer) nearestPageAnchor(row int) hugeFilePageAnchor {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.pageAnchors) == 0 {
		return hugeFilePageAnchor{row: 0, offset: 0, lineStart: 0}
	}
	idx := sort.Search(len(b.pageAnchors), func(i int) bool {
		return b.pageAnchors[i].row > row
	})
	if idx == 0 {
		return b.pageAnchors[0]
	}
	return b.pageAnchors[idx-1]
}

func (b *HugeFileBuffer) nextPageAnchor(row int) (hugeFilePageAnchor, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.pageAnchors) < 2 {
		return hugeFilePageAnchor{}, false
	}
	idx := sort.Search(len(b.pageAnchors), func(i int) bool {
		return b.pageAnchors[i].row > row
	})
	if idx >= len(b.pageAnchors) {
		return hugeFilePageAnchor{}, false
	}
	return b.pageAnchors[idx], true
}

type hugeFileScanAnchor struct {
	row       int
	offset    int64
	lineStart int64
}

func (b *HugeFileBuffer) scanStartAnchor(row int) hugeFileScanAnchor {
	checkpoint := b.nearestCheckpoint(row)
	byteAnchor := b.nearestByteAnchor(row)
	pageAnchor := b.nearestPageAnchor(row)
	best := hugeFileScanAnchor{
		row:       checkpoint.row,
		offset:    checkpoint.offset,
		lineStart: checkpoint.offset,
	}
	if byteAnchor.offset > best.offset {
		best = hugeFileScanAnchor{
			row:       byteAnchor.row,
			offset:    byteAnchor.offset,
			lineStart: byteAnchor.offset,
		}
	}
	if pageAnchor.offset > best.offset {
		best = hugeFileScanAnchor{
			row:       pageAnchor.row,
			offset:    pageAnchor.offset,
			lineStart: pageAnchor.lineStart,
		}
	}
	if cachedSpanAnchor, ok := b.nearestCachedSpanScanAnchor(row); ok && cachedSpanAnchor.offset > best.offset {
		best = cachedSpanAnchor
	}
	if cachedPageAnchor, ok := b.nearestCachedPageScanAnchor(row); ok && cachedPageAnchor.offset > best.offset {
		best = cachedPageAnchor
	}
	return best
}

func (b *HugeFileBuffer) nearestCachedSpanScanAnchor(row int) (hugeFileScanAnchor, bool) {
	if b == nil || row < 0 {
		return hugeFileScanAnchor{}, false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.spanSorted) == 0 {
		return hugeFileScanAnchor{}, false
	}

	// Binary search: find the largest cached row ≤ target row. O(log n).
	idx := sort.SearchInts(b.spanSorted, row+1) - 1
	if idx < 0 {
		return hugeFileScanAnchor{}, false
	}
	bestRow := b.spanSorted[idx]
	span, ok := b.lineSpans[bestRow]
	if !ok {
		return hugeFileScanAnchor{}, false
	}
	return hugeFileScanAnchor{
		row:       bestRow,
		offset:    span.start,
		lineStart: span.start,
	}, true
}

func (b *HugeFileBuffer) nearestCachedPageScanAnchor(row int) (hugeFileScanAnchor, bool) {
	if b == nil || row < 0 {
		return hugeFileScanAnchor{}, false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.pageData) == 0 {
		return hugeFileScanAnchor{}, false
	}

	var best hugeFileScanAnchor
	found := false
	for _, page := range b.pageData {
		if page.startRow > row || len(page.spans) == 0 {
			continue
		}
		candidateRow := page.endRow
		candidateSpan := page.spans[len(page.spans)-1]
		if row <= page.endRow {
			idx := row - page.startRow
			if idx < 0 || idx >= len(page.spans) {
				continue
			}
			candidateRow = row
			candidateSpan = page.spans[idx]
		}
		candidate := hugeFileScanAnchor{
			row:       candidateRow,
			offset:    candidateSpan.start,
			lineStart: candidateSpan.start,
		}
		if !found || candidate.offset > best.offset {
			best = candidate
			found = true
		}
	}
	return best, found
}

func (b *HugeFileBuffer) latestCachedAnchor() hugeFileCheckpoint {
	b.mu.RLock()
	defer b.mu.RUnlock()

	best := hugeFileCheckpoint{row: 0, offset: 0}
	if len(b.checkpoints) > 0 {
		best = b.checkpoints[len(b.checkpoints)-1]
	}
	if len(b.byteAnchors) > 0 {
		lastByteAnchor := b.byteAnchors[len(b.byteAnchors)-1]
		if lastByteAnchor.offset > best.offset {
			best = lastByteAnchor
		}
	}
	return best
}

func (b *HugeFileBuffer) nextByteAnchorOffset(startOffset int64) int64 {
	if b == nil || b.byteAnchorSpacing <= 0 {
		return 0
	}
	aligned := b.byteAnchorSpacing
	if startOffset > 0 {
		aligned = ((startOffset / b.byteAnchorSpacing) + 1) * b.byteAnchorSpacing
	}
	b.mu.RLock()
	if len(b.byteAnchors) > 0 {
		last := b.byteAnchors[len(b.byteAnchors)-1].offset
		b.mu.RUnlock()
		next := last + b.byteAnchorSpacing
		if aligned > next {
			return aligned
		}
		return next
	}
	b.mu.RUnlock()
	return aligned
}

func (b *HugeFileBuffer) nextPageAnchorOffset(startOffset int64) int64 {
	if b == nil || b.byteAnchorSpacing <= 0 {
		return 0
	}
	aligned := b.byteAnchorSpacing
	if startOffset > 0 {
		aligned = ((startOffset / b.byteAnchorSpacing) + 1) * b.byteAnchorSpacing
	}
	b.mu.RLock()
	if len(b.pageAnchors) > 0 {
		last := b.pageAnchors[len(b.pageAnchors)-1].offset
		b.mu.RUnlock()
		next := last + b.byteAnchorSpacing
		if aligned > next {
			return aligned
		}
		return next
	}
	b.mu.RUnlock()
	return aligned
}

func (b *HugeFileBuffer) rememberCheckpoint(row int, offset int64) {
	if row <= 0 || row%hugeFileCheckpointSpacing != 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	lastRow := -1
	if len(b.checkpoints) > 0 {
		lastRow = b.checkpoints[len(b.checkpoints)-1].row
	}
	if row <= lastRow {
		return
	}
	b.checkpoints = append(b.checkpoints, hugeFileCheckpoint{row: row, offset: offset})
	if row+1 > b.lineCount {
		b.lineCount = row + 1
	}
}

func (b *HugeFileBuffer) observeScanOffset(row int, nextOffset, lineStart int64, nextPageAnchorOffset *int64) {
	if b == nil || nextPageAnchorOffset == nil {
		return
	}
	for *nextPageAnchorOffset > 0 && nextOffset >= *nextPageAnchorOffset {
		b.appendPageAnchor(row, *nextPageAnchorOffset, lineStart)
		*nextPageAnchorOffset += b.byteAnchorSpacing
	}
}

func (b *HugeFileBuffer) observeScanLineBoundary(row int, nextOffset, lineStart int64, nextByteAnchorOffset, nextPageAnchorOffset *int64) {
	if b == nil {
		return
	}
	b.rememberCheckpoint(row, nextOffset)
	if nextByteAnchorOffset != nil {
		for *nextByteAnchorOffset > 0 && nextOffset >= *nextByteAnchorOffset {
			b.appendByteAnchor(row, nextOffset)
			*nextByteAnchorOffset += b.byteAnchorSpacing
		}
	}
	b.observeScanOffset(row, nextOffset, lineStart, nextPageAnchorOffset)
}

func (b *HugeFileBuffer) PrefetchLines(startRow, count int) error {
	return b.prefetchLinesFromReader(b.reader, startRow, count)
}

func (b *HugeFileBuffer) WarmLines(startRow, count int) {
	if b == nil || (b.reader == nil && b.mmapData == nil) || count <= 0 {
		return
	}
	if startRow < 0 {
		startRow = 0
	}
	lineCount := b.LineCount()
	if startRow >= lineCount {
		return
	}
	endRow := startRow + count - 1 + hugeFileLinePrefetch
	if endRow >= lineCount {
		endRow = lineCount - 1
	}
	if b.hasCachedLineSpans(startRow, endRow) {
		b.warmCachedPageRange(startRow, endRow)
		return
	}

	// mmap fast path: all reads come from memory, no reader needed.
	// Run synchronously — no goroutine or file handle overhead.
	if b.mmapData != nil {
		_ = b.prefetchLinesFromReader(nil, startRow, count)
		return
	}

	warmKey := startRow / hugeFileLinePrefetch

	b.mu.Lock()
	if b.closed || b.openReader == nil {
		b.mu.Unlock()
		return
	}
	if _, ok := b.warmInFlight[warmKey]; ok {
		b.mu.Unlock()
		return
	}
	b.warmInFlight[warmKey] = struct{}{}
	openReader := b.openReader
	b.mu.Unlock()

	reader, err := openReader()
	if err != nil {
		b.finishWarm(warmKey)
		return
	}

	b.workers.Add(1)
	go func() {
		defer b.workers.Done()
		defer b.finishWarm(warmKey)
		defer reader.Close()
		_ = b.prefetchLinesFromReader(reader, startRow, count)
	}()
}

func (b *HugeFileBuffer) finishWarm(warmKey int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.warmInFlight == nil {
		return
	}
	delete(b.warmInFlight, warmKey)
}

func (b *HugeFileBuffer) warmCachedPageRange(startRow, endRow int) {
	if b == nil || startRow > endRow {
		return
	}
	currentRow := startRow
	for currentRow <= endRow {
		page, ok := b.cachedPageData(currentRow)
		if !ok || currentRow < page.startRow || currentRow > page.endRow {
			return
		}
		b.populateCachesFromPageData(currentRow)
		if page.endRow < currentRow {
			return
		}
		currentRow = page.endRow + 1
	}
}

func (b *HugeFileBuffer) prefetchLinesFromReader(reader io.ReadSeeker, startRow, count int) error {
	if b == nil || (reader == nil && b.mmapData == nil) || count <= 0 {
		return nil
	}
	if startRow < 0 {
		startRow = 0
	}
	lineCount := b.LineCount()
	if startRow >= lineCount {
		return nil
	}
	endRow := startRow + count - 1 + hugeFileLinePrefetch
	if endRow >= lineCount {
		endRow = lineCount - 1
	}
	if err := b.cacheLineSpansFromReader(reader, startRow, endRow); err != nil {
		return err
	}
	if err := b.cacheLinesFromCachedSpans(reader, startRow, endRow); err != nil {
		return err
	}
	b.primePageDataForRange(reader, startRow, endRow)
	return nil
}

func (b *HugeFileBuffer) cacheLinesFromCachedSpans(reader io.ReadSeeker, startRow, endRow int) error {
	if b == nil || (reader == nil && b.mmapData == nil) {
		return nil
	}
	startSpan, ok := b.peekLineSpan(startRow)
	if !ok {
		return nil
	}
	actualEndRow := endRow
	var endSpan hugeFileLineSpan
	foundEnd := false
	for actualEndRow >= startRow {
		endSpan, foundEnd = b.peekLineSpan(actualEndRow)
		if foundEnd {
			break
		}
		actualEndRow--
	}
	if !foundEnd {
		return nil
	}
	readLen := endSpan.end - startSpan.start
	if readLen < 0 {
		return nil
	}
	data, err := b.readBytesAt(reader, startSpan.start, endSpan.end)
	if err != nil {
		return err
	}
	baseOffset := startSpan.start
	cached := make([]hugeFileCachedLineEntry, 0, actualEndRow-startRow+1)
	infos := make([]hugeFileCachedLineInfoEntry, 0, actualEndRow-startRow+1)
	endings := make([]hugeFileCachedLineEndingEntry, 0, actualEndRow-startRow)
	for row := startRow; row <= actualEndRow; row++ {
		span, ok := b.peekLineSpan(row)
		if !ok {
			continue
		}
		relStart := span.start - baseOffset
		relEnd := span.end - baseOffset
		if relStart < 0 || relEnd < relStart || relEnd > int64(len(data)) {
			continue
		}
		lineData := data[relStart:relEnd]
		hadCR := len(lineData) > 0 && lineData[len(lineData)-1] == '\r'
		if hadCR {
			lineData = lineData[:len(lineData)-1]
		}
		cached = append(cached, hugeFileCachedLineEntry{
			row:  row,
			line: bytesToRunes(lineData),
		})
		infos = append(infos, hugeFileCachedLineInfoEntry{
			row:  row,
			info: analyzeHugeFileLineData(lineData),
		})
		if row < actualEndRow {
			nextSpan, ok := b.peekLineSpan(row + 1)
			if ok {
				delimStart := span.end - baseOffset
				delimEnd := nextSpan.start - baseOffset
				if delimStart >= 0 && delimEnd >= delimStart && delimEnd <= int64(len(data)) {
					eol := string(data[delimStart:delimEnd])
					if hadCR {
						eol = "\r" + eol
					}
					endings = append(endings, hugeFileCachedLineEndingEntry{
						row: row,
						eol: eol,
					})
				}
			}
		}
	}
	b.storeCachedLines(cached)
	b.storeCachedLineInfos(infos)
	b.storeCachedLineEndings(endings)
	return nil
}

func (b *HugeFileBuffer) cacheLineSpansFromReader(reader io.ReadSeeker, startRow, endRow int) error {
	if b == nil {
		return nil
	}
	if startRow < 0 {
		startRow = 0
	}
	lineCount := b.LineCount()
	if lineCount <= 0 {
		return nil
	}
	if endRow >= lineCount {
		endRow = lineCount - 1
	}
	if startRow > endRow {
		return nil
	}
	if b.hasCachedLineSpans(startRow, endRow) {
		return nil
	}

	// mmap fast path: scan byte slice directly — no syscalls.
	if b.mmapData != nil {
		return b.cacheLineSpansFromMmap(startRow, endRow)
	}

	if reader == nil {
		return nil
	}

	anchor := b.scanStartAnchor(startRow)
	if _, err := reader.Seek(anchor.offset, io.SeekStart); err != nil {
		return err
	}

	currentRow := anchor.row
	lineStart := anchor.lineStart
	fileOffset := anchor.offset
	nextByteAnchorOffset := b.nextByteAnchorOffset(anchor.offset)
	nextPageAnchorOffset := b.nextPageAnchorOffset(anchor.offset)
	buf := make([]byte, 64<<10)
	pending := make([]hugeFileLineSpanEntry, 0, 256)

	flushPending := func() {
		if len(pending) == 0 {
			return
		}
		b.storeLineSpansBatch(pending)
		pending = pending[:0]
	}

	for {
		if b.isCanceled() {
			flushPending()
			return nil
		}
		n, err := reader.Read(buf)
		if n > 0 {
			for i, ch := range buf[:n] {
				nextOffset := fileOffset + int64(i) + 1
				if ch != '\n' {
					b.observeScanOffset(currentRow, nextOffset, lineStart, &nextPageAnchorOffset)
					continue
				}
				pending = append(pending, hugeFileLineSpanEntry{
					row: currentRow,
					span: hugeFileLineSpan{
						start: lineStart,
						end:   fileOffset + int64(i),
					},
				})
				nextRow := currentRow + 1
				nextLineStart := nextOffset
				if len(pending) >= 256 {
					flushPending()
				}
				b.observeScanLineBoundary(nextRow, nextOffset, nextLineStart, &nextByteAnchorOffset, &nextPageAnchorOffset)
				if currentRow >= endRow {
					flushPending()
					return nil
				}
				currentRow = nextRow
				lineStart = nextLineStart
			}
			fileOffset += int64(n)
		}
		if err == io.EOF {
			pending = append(pending, hugeFileLineSpanEntry{
				row: currentRow,
				span: hugeFileLineSpan{
					start: lineStart,
					end:   b.sizeBytes,
				},
			})
			flushPending()
			return nil
		}
		if err != nil {
			flushPending()
			return err
		}
		flushPending()
	}
}

// cacheLineSpansFromMmap scans the mmap'd data directly to find line spans.
// Much faster than the reader path: no syscalls, no buffer copies.
// cacheLineSpansFromMmap scans the mmap'd data using bytes.IndexByte
// for SIMD-accelerated newline search to find line boundaries.
func (b *HugeFileBuffer) cacheLineSpansFromMmap(startRow, endRow int) error {
	anchor := b.scanStartAnchor(startRow)
	currentRow := anchor.row
	lineStart := anchor.lineStart
	nextByteAnchorOffset := b.nextByteAnchorOffset(anchor.offset)
	nextPageAnchorOffset := b.nextPageAnchorOffset(anchor.offset)
	pending := make([]hugeFileLineSpanEntry, 0, 256)
	data := b.mmapData
	size := int64(len(data))

	flushPending := func() {
		if len(pending) == 0 {
			return
		}
		b.storeLineSpansBatch(pending)
		pending = pending[:0]
	}

	pos := anchor.offset
	if pos < 0 {
		pos = 0
	}

	for pos < size {
		if b.isCanceled() {
			flushPending()
			return nil
		}

		idx := bytes.IndexByte(data[pos:], '\n')
		if idx < 0 {
			// No more newlines — last line spans to EOF.
			for nextPageAnchorOffset > 0 && nextPageAnchorOffset <= size {
				b.appendPageAnchor(currentRow, nextPageAnchorOffset, lineStart)
				nextPageAnchorOffset += b.byteAnchorSpacing
			}
			pending = append(pending, hugeFileLineSpanEntry{
				row:  currentRow,
				span: hugeFileLineSpan{start: lineStart, end: size},
			})
			flushPending()
			return nil
		}

		nlOffset := pos + int64(idx)
		nextOffset := nlOffset + 1

		// Advance page anchors up to this position.
		for nextPageAnchorOffset > 0 && nextPageAnchorOffset <= nextOffset {
			b.appendPageAnchor(currentRow, nextPageAnchorOffset, lineStart)
			nextPageAnchorOffset += b.byteAnchorSpacing
		}

		pending = append(pending, hugeFileLineSpanEntry{
			row:  currentRow,
			span: hugeFileLineSpan{start: lineStart, end: nlOffset},
		})
		nextRow := currentRow + 1
		nextLineStart := nextOffset
		if len(pending) >= 256 {
			flushPending()
		}
		b.observeScanLineBoundary(nextRow, nextOffset, nextLineStart, &nextByteAnchorOffset, &nextPageAnchorOffset)
		if currentRow >= endRow {
			flushPending()
			return nil
		}
		currentRow = nextRow
		lineStart = nextLineStart
		pos = nextOffset
	}
	// Last line (no trailing newline, pos reached EOF).
	pending = append(pending, hugeFileLineSpanEntry{
		row:  currentRow,
		span: hugeFileLineSpan{start: lineStart, end: size},
	})
	flushPending()
	return nil
}

func (b *HugeFileBuffer) cachePageLineSpansFromReader(reader io.ReadSeeker, row int) error {
	if b == nil || (reader == nil && b.mmapData == nil) {
		return nil
	}
	lineCount := b.LineCount()
	if lineCount <= 0 || row < 0 || row >= lineCount {
		return errHugeFileLineOutOfRange
	}
	if b.hasCachedLineSpan(row) {
		return nil
	}

	start := b.nearestPageAnchor(row)
	end, hasEnd := b.nextPageAnchor(row)
	if !hasEnd {
		return b.cacheLineSpansFromReader(reader, row, row+hugeFileLinePrefetch)
	}
	if start.offset == end.offset && start.row == end.row && start.lineStart == end.lineStart {
		return b.cacheLineSpansFromReader(reader, row, row+hugeFileLinePrefetch)
	}
	// mmap fast path: delegate to cacheLineSpansFromReader which uses mmap directly.
	if b.mmapData != nil {
		return b.cacheLineSpansFromReader(reader, start.row, end.row)
	}
	if reader == nil {
		return nil
	}
	if _, err := reader.Seek(start.offset, io.SeekStart); err != nil {
		return err
	}

	currentRow := start.row
	lineStart := start.lineStart
	fileOffset := start.offset
	nextByteAnchorOffset := b.nextByteAnchorOffset(start.offset)
	nextPageAnchorOffset := b.nextPageAnchorOffset(start.offset)
	buf := make([]byte, 64<<10)
	pending := make([]hugeFileLineSpanEntry, 0, 256)

	flushPending := func() {
		if len(pending) == 0 {
			return
		}
		b.storeLineSpansBatch(pending)
		pending = pending[:0]
	}

	for {
		if b.isCanceled() {
			flushPending()
			return nil
		}
		n, err := reader.Read(buf)
		if n > 0 {
			for i, ch := range buf[:n] {
				nextOffset := fileOffset + int64(i) + 1
				if ch != '\n' {
					b.observeScanOffset(currentRow, nextOffset, lineStart, &nextPageAnchorOffset)
					continue
				}
				pending = append(pending, hugeFileLineSpanEntry{
					row: currentRow,
					span: hugeFileLineSpan{
						start: lineStart,
						end:   fileOffset + int64(i),
					},
				})
				nextRow := currentRow + 1
				nextLineStart := nextOffset
				if len(pending) >= 256 {
					flushPending()
				}
				b.observeScanLineBoundary(nextRow, nextOffset, nextLineStart, &nextByteAnchorOffset, &nextPageAnchorOffset)
				currentRow = nextRow
				lineStart = nextLineStart
				if nextOffset >= end.offset && currentRow > end.row {
					flushPending()
					return nil
				}
			}
			fileOffset += int64(n)
		}
		if err == io.EOF {
			pending = append(pending, hugeFileLineSpanEntry{
				row: currentRow,
				span: hugeFileLineSpan{
					start: lineStart,
					end:   b.sizeBytes,
				},
			})
			flushPending()
			return nil
		}
		if err != nil {
			flushPending()
			return err
		}
		if fileOffset >= end.offset && currentRow > end.row {
			flushPending()
			return nil
		}
	}
}

func (b *HugeFileBuffer) cachePageLinesFromReader(reader io.ReadSeeker, row int) error {
	if b == nil || reader == nil {
		return nil
	}
	lineCount := b.LineCount()
	if lineCount <= 0 || row < 0 || row >= lineCount {
		return errHugeFileLineOutOfRange
	}
	if b.hasCachedLine(row) {
		return nil
	}
	if err := b.cachePageLineSpansFromReader(reader, row); err != nil && !errors.Is(err, errHugeFileLineOutOfRange) {
		return err
	}
	if err := b.cachePageDataFromReader(reader, row); err == nil {
		b.populateCachesFromPageData(row)
		if _, ok := b.cachedLine(row); ok {
			return nil
		}
	} else if !errors.Is(err, errHugeFileLineOutOfRange) {
		return err
	}

	start := b.nearestPageAnchor(row)
	end, hasEnd := b.nextPageAnchor(row)
	if !hasEnd {
		endRow := row + hugeFileLinePrefetch
		if endRow >= lineCount {
			endRow = lineCount - 1
		}
		if err := b.cacheLineSpansFromReader(reader, row, endRow); err != nil {
			return err
		}
		return b.cacheLinesFromCachedSpans(reader, row, endRow)
	}

	endRow := end.row
	if endRow < start.row {
		endRow = start.row
	}
	if endRow >= lineCount {
		endRow = lineCount - 1
	}
	return b.cacheLinesFromCachedSpans(reader, start.row, endRow)
}

func (b *HugeFileBuffer) primePageDataForRange(reader io.ReadSeeker, startRow, endRow int) {
	if b == nil || (reader == nil && b.mmapData == nil) || startRow > endRow {
		return
	}
	currentRow := startRow
	for currentRow <= endRow {
		if err := b.cachePageDataFromReader(reader, currentRow); err != nil {
			if errors.Is(err, errHugeFileLineOutOfRange) {
				return
			}
			return
		}
		b.populateCachesFromPageData(currentRow)
		page, ok := b.cachedPageData(currentRow)
		if !ok || page.endRow < currentRow {
			return
		}
		currentRow = page.endRow + 1
	}
}

func (b *HugeFileBuffer) cachePageDataFromReader(reader io.ReadSeeker, row int) error {
	if b == nil || (reader == nil && b.mmapData == nil) {
		return nil
	}
	lineCount := b.LineCount()
	if lineCount <= 0 || row < 0 || row >= lineCount {
		return errHugeFileLineOutOfRange
	}
	if _, ok := b.cachedPageData(row); ok {
		return nil
	}

	start := b.nearestPageAnchor(row)
	end, hasEnd := b.nextPageAnchor(row)
	endRow := row + hugeFileLinePrefetch
	if hasEnd {
		endRow = end.row
	}
	if endRow >= lineCount {
		endRow = lineCount - 1
	}
	if endRow < start.row {
		endRow = start.row
	}

	if err := b.cachePageLineSpansFromReader(reader, row); err != nil && !errors.Is(err, errHugeFileLineOutOfRange) {
		return err
	}
	if err := b.cacheLineSpansFromReader(reader, start.row, endRow); err != nil {
		return err
	}
	startSpan, ok := b.peekLineSpan(start.row)
	if !ok {
		return nil
	}
	actualEndRow := endRow
	var endSpan hugeFileLineSpan
	foundEnd := false
	for actualEndRow >= start.row {
		endSpan, foundEnd = b.peekLineSpan(actualEndRow)
		if foundEnd {
			break
		}
		actualEndRow--
	}
	if !foundEnd {
		return nil
	}
	readLen := endSpan.end - startSpan.start
	if readLen < 0 {
		return nil
	}
	data, err := b.readBytesAt(reader, startSpan.start, endSpan.end)
	if err != nil {
		return err
	}
	// If data came from mmap, we need a copy since pageData stores []byte that may be modified.
	if b.mmapData != nil && len(data) > 0 {
		copied := make([]byte, len(data))
		copy(copied, data)
		data = copied
	}
	b.storeCachedPageData(hugeFileCachedPageData{
		startRow:    start.row,
		endRow:      actualEndRow,
		startOffset: startSpan.start,
		spans:       b.collectCachedPageSpans(start.row, actualEndRow),
		data:        data,
	})
	return nil
}

func (b *HugeFileBuffer) populateCachesFromPageData(row int) {
	page, ok := b.cachedPageData(row)
	if !ok {
		return
	}
	b.restoreSpanCacheFromPageData(page)
	cached := make([]hugeFileCachedLineEntry, 0, page.endRow-page.startRow+1)
	infos := make([]hugeFileCachedLineInfoEntry, 0, page.endRow-page.startRow+1)
	endings := make([]hugeFileCachedLineEndingEntry, 0, page.endRow-page.startRow)
	for currentRow := page.startRow; currentRow <= page.endRow; currentRow++ {
		span, ok := page.spanForRow(currentRow)
		if !ok {
			continue
		}
		relStart := span.start - page.startOffset
		relEnd := span.end - page.startOffset
		if relStart < 0 || relEnd < relStart || relEnd > int64(len(page.data)) {
			continue
		}
		lineData := page.data[relStart:relEnd]
		hadCR := len(lineData) > 0 && lineData[len(lineData)-1] == '\r'
		if hadCR {
			lineData = lineData[:len(lineData)-1]
		}
		cached = append(cached, hugeFileCachedLineEntry{
			row:  currentRow,
			line: bytesToRunes(lineData),
		})
		infos = append(infos, hugeFileCachedLineInfoEntry{
			row:  currentRow,
			info: analyzeHugeFileLineData(lineData),
		})
		if nextSpan, ok := page.spanForRow(currentRow + 1); ok {
			delimStart := span.end - page.startOffset
			delimEnd := nextSpan.start - page.startOffset
			if delimStart >= 0 && delimEnd >= delimStart && delimEnd <= int64(len(page.data)) {
				eol := string(page.data[delimStart:delimEnd])
				if hadCR {
					eol = "\r" + eol
				}
				endings = append(endings, hugeFileCachedLineEndingEntry{
					row: currentRow,
					eol: eol,
				})
			}
		}
	}
	b.storeCachedLines(cached)
	b.storeCachedLineInfos(infos)
	b.storeCachedLineEndings(endings)
}

func (b *HugeFileBuffer) hasCachedLineSpans(startRow, endRow int) bool {
	if b.hasCachedPageRange(startRow, endRow) {
		return true
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for row := startRow; row <= endRow; row++ {
		if _, ok := b.lineSpans[row]; !ok {
			return false
		}
	}
	return true
}

func (b *HugeFileBuffer) hasCachedPageRange(startRow, endRow int) bool {
	if b == nil || startRow > endRow {
		return false
	}
	currentRow := startRow
	for currentRow <= endRow {
		page, ok := b.cachedPageData(currentRow)
		if !ok || currentRow < page.startRow || currentRow > page.endRow {
			return false
		}
		if page.endRow < currentRow {
			return false
		}
		currentRow = page.endRow + 1
	}
	return true
}

func (b *HugeFileBuffer) ensureCacheLRU() {
	if b.cacheLRU.order == nil {
		b.cacheLRU = newLRUTracker()
	}
}

func (b *HugeFileBuffer) ensureInfoLRU() {
	if b.infoLRU.order == nil {
		b.infoLRU = newLRUTracker()
	}
}

func (b *HugeFileBuffer) ensureEndingLRU() {
	if b.endingLRU.order == nil {
		b.endingLRU = newLRUTracker()
	}
}

func (b *HugeFileBuffer) ensurePageLRU() {
	if b.pageLRU.order == nil {
		b.pageLRU = newLRUTracker()
	}
}

func (b *HugeFileBuffer) ensureSpanLRU() {
	if b.spanLRU.order == nil {
		b.spanLRU = newLRUTracker()
	}
}

func (b *HugeFileBuffer) storeCachedLine(row int, line []rune) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.lineCache == nil {
		b.lineCache = make(map[int][]rune)
	}
	b.ensureCacheLRU()
	if _, ok := b.lineCache[row]; ok {
		b.lineCache[row] = line
		b.cacheLRU.touch(row)
		return
	}
	if b.cacheLRU.len() >= hugeFileLineCacheSize {
		if evict, ok := b.cacheLRU.evictOldest(); ok {
			delete(b.lineCache, evict)
		}
	}
	b.lineCache[row] = line
	b.cacheLRU.add(row)
}

func (b *HugeFileBuffer) storeCachedLines(entries []hugeFileCachedLineEntry) {
	if len(entries) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.lineCache == nil {
		b.lineCache = make(map[int][]rune)
	}
	b.ensureCacheLRU()
	for _, entry := range entries {
		if _, ok := b.lineCache[entry.row]; ok {
			b.lineCache[entry.row] = entry.line
			b.cacheLRU.touch(entry.row)
			continue
		}
		if b.cacheLRU.len() >= hugeFileLineCacheSize {
			if evict, ok := b.cacheLRU.evictOldest(); ok {
				delete(b.lineCache, evict)
			}
		}
		b.lineCache[entry.row] = entry.line
		b.cacheLRU.add(entry.row)
	}
}

func (b *HugeFileBuffer) storeCachedLineInfo(row int, info hugeFileLineInfo) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.lineInfo == nil {
		b.lineInfo = make(map[int]hugeFileLineInfo)
	}
	b.ensureInfoLRU()
	if _, ok := b.lineInfo[row]; ok {
		b.lineInfo[row] = info
		b.infoLRU.touch(row)
		return
	}
	if b.infoLRU.len() >= hugeFileLineCacheSize {
		if evict, ok := b.infoLRU.evictOldest(); ok {
			delete(b.lineInfo, evict)
		}
	}
	b.lineInfo[row] = info
	b.infoLRU.add(row)
}

func (b *HugeFileBuffer) storeCachedLineInfos(entries []hugeFileCachedLineInfoEntry) {
	if len(entries) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.lineInfo == nil {
		b.lineInfo = make(map[int]hugeFileLineInfo)
	}
	b.ensureInfoLRU()
	for _, entry := range entries {
		if _, ok := b.lineInfo[entry.row]; ok {
			b.lineInfo[entry.row] = entry.info
			b.infoLRU.touch(entry.row)
			continue
		}
		if b.infoLRU.len() >= hugeFileLineCacheSize {
			if evict, ok := b.infoLRU.evictOldest(); ok {
				delete(b.lineInfo, evict)
			}
		}
		b.lineInfo[entry.row] = entry.info
		b.infoLRU.add(entry.row)
	}
}

func (b *HugeFileBuffer) storeCachedLineEnding(row int, eol string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.lineEndings == nil {
		b.lineEndings = make(map[int]string)
	}
	b.ensureEndingLRU()
	if _, ok := b.lineEndings[row]; ok {
		b.lineEndings[row] = eol
		b.endingLRU.touch(row)
		return
	}
	if b.endingLRU.len() >= hugeFileLineCacheSize {
		if evict, ok := b.endingLRU.evictOldest(); ok {
			delete(b.lineEndings, evict)
		}
	}
	b.lineEndings[row] = eol
	b.endingLRU.add(row)
}

func (b *HugeFileBuffer) storeCachedLineEndings(entries []hugeFileCachedLineEndingEntry) {
	if len(entries) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.lineEndings == nil {
		b.lineEndings = make(map[int]string)
	}
	b.ensureEndingLRU()
	for _, entry := range entries {
		if _, ok := b.lineEndings[entry.row]; ok {
			b.lineEndings[entry.row] = entry.eol
			b.endingLRU.touch(entry.row)
			continue
		}
		if b.endingLRU.len() >= hugeFileLineCacheSize {
			if evict, ok := b.endingLRU.evictOldest(); ok {
				delete(b.lineEndings, evict)
			}
		}
		b.lineEndings[entry.row] = entry.eol
		b.endingLRU.add(entry.row)
	}
}

func (b *HugeFileBuffer) storeCachedPageData(page hugeFileCachedPageData) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.pageData == nil {
		b.pageData = make(map[int]hugeFileCachedPageData)
	}
	b.ensurePageLRU()
	if _, ok := b.pageData[page.startRow]; ok {
		b.pageData[page.startRow] = page
		b.pageLRU.touch(page.startRow)
		return
	}
	if b.pageLRU.len() >= hugeFilePageDataCacheSize {
		if evict, ok := b.pageLRU.evictOldest(); ok {
			delete(b.pageData, evict)
		}
	}
	b.pageData[page.startRow] = page
	b.pageLRU.add(page.startRow)
}

// spanSortedInsert adds row to the sorted slice in O(log n).
func (b *HugeFileBuffer) spanSortedInsert(row int) {
	idx := sort.SearchInts(b.spanSorted, row)
	if idx < len(b.spanSorted) && b.spanSorted[idx] == row {
		return // already present
	}
	b.spanSorted = append(b.spanSorted, 0)
	copy(b.spanSorted[idx+1:], b.spanSorted[idx:])
	b.spanSorted[idx] = row
}

// spanSortedRemove removes row from the sorted slice in O(log n + shift).
func (b *HugeFileBuffer) spanSortedRemove(row int) {
	idx := sort.SearchInts(b.spanSorted, row)
	if idx < len(b.spanSorted) && b.spanSorted[idx] == row {
		b.spanSorted = append(b.spanSorted[:idx], b.spanSorted[idx+1:]...)
	}
}

func (b *HugeFileBuffer) storeLineSpan(row int, span hugeFileLineSpan) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.lineSpans == nil {
		b.lineSpans = make(map[int]hugeFileLineSpan)
	}
	b.ensureSpanLRU()
	if _, ok := b.lineSpans[row]; ok {
		b.lineSpans[row] = span
		b.spanLRU.touch(row)
		return
	}
	if b.spanLRU.len() >= hugeFileSpanCacheSize {
		if evict, ok := b.spanLRU.evictOldest(); ok {
			delete(b.lineSpans, evict)
			b.spanSortedRemove(evict)
		}
	}
	b.lineSpans[row] = span
	b.spanLRU.add(row)
	b.spanSortedInsert(row)
}

func (b *HugeFileBuffer) storeLineSpansBatch(entries []hugeFileLineSpanEntry) {
	if len(entries) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.lineSpans == nil {
		b.lineSpans = make(map[int]hugeFileLineSpan)
	}
	b.ensureSpanLRU()
	for _, entry := range entries {
		if _, ok := b.lineSpans[entry.row]; ok {
			b.lineSpans[entry.row] = entry.span
			b.spanLRU.touch(entry.row)
			continue
		}
		if b.spanLRU.len() >= hugeFileSpanCacheSize {
			if evict, ok := b.spanLRU.evictOldest(); ok {
				delete(b.lineSpans, evict)
				b.spanSortedRemove(evict)
			}
		}
		b.lineSpans[entry.row] = entry.span
		b.spanLRU.add(entry.row)
		b.spanSortedInsert(entry.row)
	}
}

func (b *HugeFileBuffer) hasCachedLine(row int) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.lineCache == nil {
		return false
	}
	_, ok := b.lineCache[row]
	return ok
}

func (b *HugeFileBuffer) cachedLine(row int) ([]rune, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.lineCache == nil {
		return nil, false
	}
	cached, ok := b.lineCache[row]
	if !ok {
		return nil, false
	}
	b.cacheLRU.touch(row)
	return cached, true
}

func (b *HugeFileBuffer) cachedLineEnding(row int) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.lineEndings == nil {
		return "", false
	}
	eol, ok := b.lineEndings[row]
	if !ok {
		return "", false
	}
	b.endingLRU.touch(row)
	return eol, true
}

func (b *HugeFileBuffer) cachedLineInfo(row int) (hugeFileLineInfo, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.lineInfo == nil {
		return hugeFileLineInfo{}, false
	}
	info, ok := b.lineInfo[row]
	if !ok {
		return hugeFileLineInfo{}, false
	}
	b.infoLRU.touch(row)
	return info, true
}

func (b *HugeFileBuffer) cachedPageData(row int) (hugeFileCachedPageData, bool) {
	start := b.nearestPageAnchor(row).row
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pageData == nil {
		return hugeFileCachedPageData{}, false
	}
	page, ok := b.pageData[start]
	if !ok {
		return hugeFileCachedPageData{}, false
	}
	b.pageLRU.touch(start)
	return page, true
}

func (b *HugeFileBuffer) cachedPageSpan(row int) (hugeFileLineSpan, bool) {
	page, ok := b.cachedPageData(row)
	if !ok {
		return hugeFileLineSpan{}, false
	}
	span, ok := page.spanForRow(row)
	if !ok {
		return hugeFileLineSpan{}, false
	}
	b.restoreSpanCacheFromPageData(page)
	return span, true
}

func (b *HugeFileBuffer) cachedPageLine(row int, span hugeFileLineSpan) ([]rune, bool) {
	page, ok := b.cachedPageData(row)
	if !ok || row < page.startRow || row > page.endRow {
		return nil, false
	}
	relStart := span.start - page.startOffset
	relEnd := span.end - page.startOffset
	if relStart < 0 || relEnd < relStart || relEnd > int64(len(page.data)) {
		return nil, false
	}
	lineData := page.data[relStart:relEnd]
	if len(lineData) > 0 && lineData[len(lineData)-1] == '\r' {
		lineData = lineData[:len(lineData)-1]
	}
	line := bytesToRunes(lineData)
	b.storeCachedLine(row, line)
	b.storeCachedLineInfo(row, analyzeHugeFileLineData(lineData))
	return line, true
}

func (b *HugeFileBuffer) cachedPageLineSegment(row int, page hugeFileCachedPageData, startCol, endCol int) ([]rune, bool) {
	if row < page.startRow || row > page.endRow {
		return nil, false
	}
	span, ok := page.spanForRow(row)
	if !ok {
		return nil, false
	}
	relStart := span.start - page.startOffset + int64(startCol)
	relEnd := span.start - page.startOffset + int64(endCol)
	if relStart < 0 || relEnd < relStart || relEnd > int64(len(page.data)) {
		return nil, false
	}
	data := page.data[relStart:relEnd]
	segment := make([]rune, len(data))
	for i, ch := range data {
		segment[i] = rune(ch)
	}
	return segment, true
}

func (b *HugeFileBuffer) cachedPageLinePrefix(row int, page hugeFileCachedPageData, maxBytes int) ([]rune, bool) {
	if row < page.startRow || row > page.endRow {
		return nil, false
	}
	span, ok := page.spanForRow(row)
	if !ok {
		return nil, false
	}
	relStart := span.start - page.startOffset
	relEnd := span.end - page.startOffset
	if relStart < 0 || relEnd < relStart || relEnd > int64(len(page.data)) {
		return nil, false
	}
	if limit := relStart + int64(maxBytes); relEnd > limit {
		relEnd = limit
	}
	return bytesToRunes(page.data[relStart:relEnd]), true
}

func (b *HugeFileBuffer) cachedPageLineInfo(row int, span hugeFileLineSpan) (hugeFileLineInfo, bool) {
	page, ok := b.cachedPageData(row)
	if !ok || row < page.startRow || row > page.endRow {
		return hugeFileLineInfo{}, false
	}
	relStart := span.start - page.startOffset
	relEnd := span.end - page.startOffset
	if relStart < 0 || relEnd < relStart || relEnd > int64(len(page.data)) {
		return hugeFileLineInfo{}, false
	}
	lineData := page.data[relStart:relEnd]
	if len(lineData) > 0 && lineData[len(lineData)-1] == '\r' {
		lineData = lineData[:len(lineData)-1]
	}
	info := analyzeHugeFileLineData(lineData)
	b.storeCachedLineInfo(row, info)
	return info, true
}

func (b *HugeFileBuffer) cachedPageLineEnding(row int, span, nextSpan hugeFileLineSpan) (string, bool) {
	page, ok := b.cachedPageData(row)
	if !ok || row < page.startRow || row > page.endRow {
		return "", false
	}
	relStart := span.end - page.startOffset
	relEnd := nextSpan.start - page.startOffset
	if relStart < 0 || relEnd < relStart || relEnd > int64(len(page.data)) {
		return "", false
	}
	eol := string(page.data[relStart:relEnd])
	if relStart > 0 && relStart-1 < int64(len(page.data)) && page.data[relStart-1] == '\r' {
		eol = "\r" + eol
	}
	b.storeCachedLineEnding(row, eol)
	return eol, true
}

func analyzeHugeFileLineData(data []byte) hugeFileLineInfo {
	info := hugeFileLineInfo{
		asciiOnly: true,
	}
	for len(data) > 0 {
		if data[0] < utf8.RuneSelf {
			if data[0] == '\t' {
				info.hasTabs = true
			}
			info.runeLen++
			data = data[1:]
			continue
		}
		info.asciiOnly = false
		_, size := utf8.DecodeRune(data)
		if size < 1 {
			size = 1
		}
		info.runeLen++
		data = data[size:]
	}
	return info
}

func (b *HugeFileBuffer) collectCachedPageSpans(startRow, endRow int) []hugeFileLineSpan {
	if endRow < startRow {
		return nil
	}
	spans := make([]hugeFileLineSpan, 0, endRow-startRow+1)
	for row := startRow; row <= endRow; row++ {
		span, ok := b.peekLineSpan(row)
		if !ok {
			return nil
		}
		spans = append(spans, span)
	}
	return spans
}

func (b *HugeFileBuffer) restoreSpanCacheFromPageData(page hugeFileCachedPageData) {
	if len(page.spans) == 0 {
		return
	}
	entries := make([]hugeFileLineSpanEntry, 0, len(page.spans))
	for i, span := range page.spans {
		entries = append(entries, hugeFileLineSpanEntry{
			row:  page.startRow + i,
			span: span,
		})
	}
	b.storeLineSpansBatch(entries)
}

func (p hugeFileCachedPageData) spanForRow(row int) (hugeFileLineSpan, bool) {
	if row < p.startRow || row > p.endRow {
		return hugeFileLineSpan{}, false
	}
	idx := row - p.startRow
	if idx < 0 || idx >= len(p.spans) {
		return hugeFileLineSpan{}, false
	}
	return p.spans[idx], true
}

func (b *HugeFileBuffer) hasCachedLineSpan(row int) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.lineSpans == nil {
		return false
	}
	_, ok := b.lineSpans[row]
	return ok
}

func (b *HugeFileBuffer) cachedLineSpan(row int) (hugeFileLineSpan, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.lineSpans == nil {
		return hugeFileLineSpan{}, false
	}
	span, ok := b.lineSpans[row]
	if !ok {
		return hugeFileLineSpan{}, false
	}
	b.spanLRU.touch(row)
	return span, true
}

func (b *HugeFileBuffer) peekLineSpan(row int) (hugeFileLineSpan, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.lineSpans == nil {
		return hugeFileLineSpan{}, false
	}
	span, ok := b.lineSpans[row]
	return span, ok
}

func (b *HugeFileBuffer) isCanceled() bool {
	b.mu.RLock()
	cancelIndex := b.cancelIndex
	closed := b.closed
	b.mu.RUnlock()
	if closed {
		return true
	}
	if cancelIndex == nil {
		return false
	}
	select {
	case <-cancelIndex:
		return true
	default:
		return false
	}
}
