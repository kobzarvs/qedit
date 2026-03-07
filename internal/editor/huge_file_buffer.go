package editor

import (
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"sync"
	"time"
)

const hugeFileLineCacheSize = 256
const hugeFileSpanCacheSize = 4096
const hugeFilePageDataCacheSize = 8
const hugeFileCheckpointSpacing = 1024
const hugeFileLinePrefetch = 64
const hugeFileIndexCheckpointBatch = 64

var hugeFileInitialSampleBytes int64 = 2 << 20
var hugeFileMinByteAnchorSpacing int64 = 64 << 10
var hugeFileMaxByteAnchorSpacing int64 = 4 << 20
var hugeFileTargetByteAnchors int64 = 16384
var hugeFileByteAnchorSpacingOverride int64
var hugeFileIndexPersistInterval = 250 * time.Millisecond

var errHugeFileLineOutOfRange = errors.New("huge file line out of range")

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

	lineSpans    map[int]hugeFileLineSpan
	spanOrder    []int
	lineCache    map[int][]rune
	cacheOrder   []int
	lineEndings  map[int]string
	endingOrder  []int
	pageData     map[int]hugeFileCachedPageData
	pageOrder    []int
	warmInFlight map[int]struct{}
	closed       bool

	workers sync.WaitGroup
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

	if cachedIndex, ok := loadHugeFileIndexCache(path, meta); ok {
		b := &HugeFileBuffer{
			path:      path,
			sizeBytes: meta.Size,
			meta:      meta,
			reader:    reader,
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
			lineSpans:          make(map[int]hugeFileLineSpan),
			lineCache:          make(map[int][]rune),
			lineEndings:        make(map[int]string),
			pageData:           make(map[int]hugeFileCachedPageData),
			warmInFlight:       make(map[int]struct{}),
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

	initial, err := buildInitialHugeFileIndex(reader, meta.Size, byteAnchorSpacing)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}

	b := &HugeFileBuffer{
		path:      path,
		sizeBytes: meta.Size,
		meta:      meta,
		reader:    reader,
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
		lineSpans:          make(map[int]hugeFileLineSpan),
		lineCache:          make(map[int][]rune),
		lineEndings:        make(map[int]string),
		pageData:           make(map[int]hugeFileCachedPageData),
		warmInFlight:       make(map[int]struct{}),
	}

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
	}

	return hugeFileInitialIndex{
		checkpoints:        checkpoints,
		byteAnchors:        byteAnchors,
		pageAnchors:        pageAnchors,
		lineCount:          lineCount,
		estimatedLineCount: estimated,
		sampleOffset:       offset,
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
	if b.reader == nil {
		return nil
	}
	err := b.reader.Close()
	b.mu.Lock()
	b.reader = nil
	b.openReader = nil
	b.cancelIndex = nil
	b.lineSpans = nil
	b.spanOrder = nil
	b.lineCache = nil
	b.cacheOrder = nil
	b.lineEndings = nil
	b.endingOrder = nil
	b.pageData = nil
	b.pageOrder = nil
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
	if b == nil || b.reader == nil {
		return nil, false
	}
	if !b.canResolveLineQuick(row) {
		return nil, false
	}
	return b.Line(row), true
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
	return len(b.Line(row))
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
	if b == nil || b.reader == nil {
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

func (b *HugeFileBuffer) LineEnding(row int) string {
	if b == nil || b.reader == nil {
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
	if _, err := b.reader.Seek(span.end-prefixLen, io.SeekStart); err != nil {
		return ""
	}
	data := make([]byte, prefixLen+delimLen)
	if _, err := io.ReadFull(b.reader, data); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
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
	pageAnchor := b.nearestPageAnchor(row)
	best := hugeFileScanAnchor{
		row:       checkpoint.row,
		offset:    checkpoint.offset,
		lineStart: checkpoint.offset,
	}
	if pageAnchor.offset > best.offset {
		best = hugeFileScanAnchor{
			row:       pageAnchor.row,
			offset:    pageAnchor.offset,
			lineStart: pageAnchor.lineStart,
		}
	}
	return best
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

func (b *HugeFileBuffer) PrefetchLines(startRow, count int) error {
	return b.prefetchLinesFromReader(b.reader, startRow, count)
}

func (b *HugeFileBuffer) WarmLines(startRow, count int) {
	if b == nil || b.reader == nil || count <= 0 {
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
	if b == nil || reader == nil || count <= 0 {
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
	if b == nil || reader == nil {
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
	if _, err := reader.Seek(startSpan.start, io.SeekStart); err != nil {
		return err
	}
	data := make([]byte, readLen)
	if _, err := io.ReadFull(reader, data); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return err
	}
	cached := make([]hugeFileCachedLineEntry, 0, actualEndRow-startRow+1)
	endings := make([]hugeFileCachedLineEndingEntry, 0, actualEndRow-startRow)
	for row := startRow; row <= actualEndRow; row++ {
		span, ok := b.peekLineSpan(row)
		if !ok {
			continue
		}
		relStart := span.start - startSpan.start
		relEnd := span.end - startSpan.start
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
			line: []rune(string(lineData)),
		})
		if row < actualEndRow {
			nextSpan, ok := b.peekLineSpan(row + 1)
			if ok {
				delimStart := span.end - startSpan.start
				delimEnd := nextSpan.start - startSpan.start
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
	b.storeCachedLineEndings(endings)
	return nil
}

func (b *HugeFileBuffer) cacheLineSpansFromReader(reader io.ReadSeeker, startRow, endRow int) error {
	if b == nil || reader == nil {
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

	anchor := b.scanStartAnchor(startRow)
	if _, err := reader.Seek(anchor.offset, io.SeekStart); err != nil {
		return err
	}

	currentRow := anchor.row
	lineStart := anchor.lineStart
	fileOffset := anchor.offset
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
				if ch != '\n' {
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
				nextOffset := fileOffset + int64(i) + 1
				if len(pending) >= 256 {
					flushPending()
				}
				if currentRow >= endRow {
					flushPending()
					return nil
				}
				currentRow = nextRow
				lineStart = nextOffset
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

func (b *HugeFileBuffer) cachePageLineSpansFromReader(reader io.ReadSeeker, row int) error {
	if b == nil || reader == nil {
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
	if _, err := reader.Seek(start.offset, io.SeekStart); err != nil {
		return err
	}

	currentRow := start.row
	lineStart := start.lineStart
	fileOffset := start.offset
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
				if ch != '\n' {
					continue
				}
				pending = append(pending, hugeFileLineSpanEntry{
					row: currentRow,
					span: hugeFileLineSpan{
						start: lineStart,
						end:   fileOffset + int64(i),
					},
				})
				currentRow++
				lineStart = fileOffset + int64(i) + 1
				if len(pending) >= 256 {
					flushPending()
				}
				if fileOffset+int64(i)+1 >= end.offset && currentRow > end.row {
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
	if b == nil || reader == nil || startRow > endRow {
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
	if b == nil || reader == nil {
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
	if _, err := reader.Seek(startSpan.start, io.SeekStart); err != nil {
		return err
	}
	data := make([]byte, readLen)
	if _, err := io.ReadFull(reader, data); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return err
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
			line: []rune(string(lineData)),
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

func (b *HugeFileBuffer) touchCacheLocked(row int) {
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
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.lineCache == nil {
		b.lineCache = make(map[int][]rune)
	}
	if _, ok := b.lineCache[row]; ok {
		b.lineCache[row] = line
		b.touchCacheLocked(row)
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
	for _, entry := range entries {
		if _, ok := b.lineCache[entry.row]; ok {
			b.lineCache[entry.row] = entry.line
			b.touchCacheLocked(entry.row)
			continue
		}
		if len(b.cacheOrder) >= hugeFileLineCacheSize {
			evict := b.cacheOrder[0]
			delete(b.lineCache, evict)
			b.cacheOrder = b.cacheOrder[1:]
		}
		b.lineCache[entry.row] = entry.line
		b.cacheOrder = append(b.cacheOrder, entry.row)
	}
}

func (b *HugeFileBuffer) touchEndingCacheLocked(row int) {
	for i, cached := range b.endingOrder {
		if cached != row {
			continue
		}
		copy(b.endingOrder[i:], b.endingOrder[i+1:])
		b.endingOrder[len(b.endingOrder)-1] = row
		return
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
	if _, ok := b.lineEndings[row]; ok {
		b.lineEndings[row] = eol
		b.touchEndingCacheLocked(row)
		return
	}
	if len(b.endingOrder) >= hugeFileLineCacheSize {
		evict := b.endingOrder[0]
		delete(b.lineEndings, evict)
		b.endingOrder = b.endingOrder[1:]
	}
	b.lineEndings[row] = eol
	b.endingOrder = append(b.endingOrder, row)
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
	for _, entry := range entries {
		if _, ok := b.lineEndings[entry.row]; ok {
			b.lineEndings[entry.row] = entry.eol
			b.touchEndingCacheLocked(entry.row)
			continue
		}
		if len(b.endingOrder) >= hugeFileLineCacheSize {
			evict := b.endingOrder[0]
			delete(b.lineEndings, evict)
			b.endingOrder = b.endingOrder[1:]
		}
		b.lineEndings[entry.row] = entry.eol
		b.endingOrder = append(b.endingOrder, entry.row)
	}
}

func (b *HugeFileBuffer) touchPageDataLocked(startRow int) {
	for i, cached := range b.pageOrder {
		if cached != startRow {
			continue
		}
		copy(b.pageOrder[i:], b.pageOrder[i+1:])
		b.pageOrder[len(b.pageOrder)-1] = startRow
		return
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
	if _, ok := b.pageData[page.startRow]; ok {
		b.pageData[page.startRow] = page
		b.touchPageDataLocked(page.startRow)
		return
	}
	if len(b.pageOrder) >= hugeFilePageDataCacheSize {
		evict := b.pageOrder[0]
		delete(b.pageData, evict)
		b.pageOrder = b.pageOrder[1:]
	}
	b.pageData[page.startRow] = page
	b.pageOrder = append(b.pageOrder, page.startRow)
}

func (b *HugeFileBuffer) touchSpanCacheLocked(row int) {
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
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if b.lineSpans == nil {
		b.lineSpans = make(map[int]hugeFileLineSpan)
	}
	if _, ok := b.lineSpans[row]; ok {
		b.lineSpans[row] = span
		b.touchSpanCacheLocked(row)
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
	for _, entry := range entries {
		if _, ok := b.lineSpans[entry.row]; ok {
			b.lineSpans[entry.row] = entry.span
			b.touchSpanCacheLocked(entry.row)
			continue
		}
		if len(b.spanOrder) >= hugeFileSpanCacheSize {
			evict := b.spanOrder[0]
			delete(b.lineSpans, evict)
			b.spanOrder = b.spanOrder[1:]
		}
		b.lineSpans[entry.row] = entry.span
		b.spanOrder = append(b.spanOrder, entry.row)
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
	b.touchCacheLocked(row)
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
	b.touchEndingCacheLocked(row)
	return eol, true
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
	b.touchPageDataLocked(start)
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
	line := []rune(string(lineData))
	b.storeCachedLine(row, line)
	return line, true
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
	b.touchSpanCacheLocked(row)
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
