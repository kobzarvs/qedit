package editor

import (
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"sync"
)

const hugeFileLineCacheSize = 256
const hugeFileSpanCacheSize = 4096
const hugeFileCheckpointSpacing = 1024
const hugeFileLinePrefetch = 64
const hugeFileIndexCheckpointBatch = 64

var hugeFileInitialSampleBytes int64 = 2 << 20

var errHugeFileLineOutOfRange = errors.New("huge file line out of range")

type hugeFileCheckpoint struct {
	row    int
	offset int64
}

type hugeFileLineSpan struct {
	start int64
	end   int64
}

type hugeFileInitialIndex struct {
	checkpoints        []hugeFileCheckpoint
	lineCount          int
	estimatedLineCount int
	sampleOffset       int64
	fullyIndexed       bool
}

type HugeFileBuffer struct {
	path      string
	sizeBytes int64
	reader    io.ReadSeekCloser

	mu                 sync.RWMutex
	lineCount          int
	estimatedLineCount int
	checkpoints        []hugeFileCheckpoint
	fullyIndexed       bool
	indexErr           error
	cancelIndex        chan struct{}
	indexDone          chan struct{}

	lineSpans  map[int]hugeFileLineSpan
	spanOrder  []int
	lineCache  map[int][]rune
	cacheOrder []int
}

func OpenHugeFileBuffer(path string, sizeBytes int64, fs FileStore) (*HugeFileBuffer, error) {
	if fs == nil {
		return nil, errFileStoreUnavailable()
	}
	reader, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	initial, err := buildInitialHugeFileIndex(reader, sizeBytes)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}

	b := &HugeFileBuffer{
		path:               path,
		sizeBytes:          sizeBytes,
		reader:             reader,
		lineCount:          initial.lineCount,
		estimatedLineCount: initial.estimatedLineCount,
		checkpoints:        initial.checkpoints,
		fullyIndexed:       initial.fullyIndexed,
		cancelIndex:        make(chan struct{}),
		indexDone:          make(chan struct{}),
		lineSpans:          make(map[int]hugeFileLineSpan),
		lineCache:          make(map[int][]rune),
	}

	if initial.fullyIndexed {
		close(b.indexDone)
		return b, nil
	}

	indexReader, err := fs.Open(path)
	if err != nil {
		checkpoints, lineCount, fullErr := buildHugeFileLineIndex(reader)
		if fullErr != nil {
			_ = reader.Close()
			return nil, fullErr
		}
		b.checkpoints = checkpoints
		b.lineCount = lineCount
		b.estimatedLineCount = lineCount
		b.fullyIndexed = true
		close(b.indexDone)
		return b, nil
	}

	go b.buildFullIndex(indexReader, initial.sampleOffset, initial.lineCount)
	return b, nil
}

func buildInitialHugeFileIndex(reader io.ReadSeeker, sizeBytes int64) (hugeFileInitialIndex, error) {
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return hugeFileInitialIndex{}, err
	}

	limit := hugeFileInitialSampleBytes
	if limit <= 0 || (sizeBytes > 0 && limit > sizeBytes) {
		limit = sizeBytes
	}
	checkpoints := []hugeFileCheckpoint{{row: 0, offset: 0}}
	buf := make([]byte, 256<<10)
	var offset int64
	lineCount := 1
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
				if b != '\n' {
					continue
				}
				lineCount++
				if (lineCount-1)%hugeFileCheckpointSpacing == 0 {
					checkpoints = append(checkpoints, hugeFileCheckpoint{
						row:    lineCount - 1,
						offset: offset + int64(i) + 1,
					})
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
		lineCount:          lineCount,
		estimatedLineCount: estimated,
		sampleOffset:       offset,
		fullyIndexed:       fullyIndexed,
	}, nil
}

func buildHugeFileLineIndex(reader io.ReadSeeker) ([]hugeFileCheckpoint, int, error) {
	initial, err := buildInitialHugeFileIndex(reader, 0)
	if err != nil {
		return nil, 0, err
	}
	return initial.checkpoints, initial.lineCount, nil
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
	pending := make([]hugeFileCheckpoint, 0, hugeFileIndexCheckpointBatch)

	for {
		select {
		case <-b.cancelIndex:
			return
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			for i, ch := range buf[:n] {
				if ch != '\n' {
					continue
				}
				lineCount++
				if (lineCount-1)%hugeFileCheckpointSpacing == 0 {
					pending = append(pending, hugeFileCheckpoint{
						row:    lineCount - 1,
						offset: offset + int64(i) + 1,
					})
				}
			}
			offset += int64(n)
		}
		if len(pending) >= hugeFileIndexCheckpointBatch {
			b.appendCheckpoints(pending, lineCount)
			pending = pending[:0]
		}
		if err == io.EOF {
			if len(pending) > 0 {
				b.appendCheckpoints(pending, lineCount)
			}
			b.completeIndex(lineCount)
			return
		}
		if err != nil {
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

func (b *HugeFileBuffer) completeIndex(lineCount int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lineCount = lineCount
	b.estimatedLineCount = lineCount
	b.fullyIndexed = true
}

func (b *HugeFileBuffer) recordIndexError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.indexErr = err
}

func (b *HugeFileBuffer) Close() error {
	if b == nil {
		return nil
	}
	if b.cancelIndex != nil {
		close(b.cancelIndex)
		b.cancelIndex = nil
	}
	if b.indexDone != nil {
		<-b.indexDone
	}
	if b.reader == nil {
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
	if _, ok := b.lineCache[row]; ok {
		return true
	}
	if _, ok := b.lineSpans[row]; ok {
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
	if cached, ok := b.lineCache[row]; ok {
		b.touchCache(row)
		return cached
	}

	span, err := b.resolveLineSpan(row)
	if err != nil {
		if errors.Is(err, errHugeFileLineOutOfRange) {
			return nil
		}
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
	if err := b.cacheLineSpans(row, row+hugeFileLinePrefetch); err != nil {
		return hugeFileLineSpan{}, err
	}
	if span, ok := b.lineSpans[row]; ok {
		b.touchSpanCache(row)
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
	if b == nil || b.reader == nil || count <= 0 {
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
	if err := b.cacheLineSpans(startRow, endRow); err != nil {
		return err
	}
	startSpan, ok := b.lineSpans[startRow]
	if !ok {
		return nil
	}
	actualEndRow := endRow
	var endSpan hugeFileLineSpan
	foundEnd := false
	for actualEndRow >= startRow {
		endSpan, foundEnd = b.lineSpans[actualEndRow]
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
	if _, err := b.reader.Seek(startSpan.start, io.SeekStart); err != nil {
		return err
	}
	data := make([]byte, readLen)
	if _, err := io.ReadFull(b.reader, data); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return err
	}
	for row := startRow; row <= actualEndRow; row++ {
		span, ok := b.lineSpans[row]
		if !ok {
			continue
		}
		relStart := span.start - startSpan.start
		relEnd := span.end - startSpan.start
		if relStart < 0 || relEnd < relStart || relEnd > int64(len(data)) {
			continue
		}
		lineData := data[relStart:relEnd]
		if len(lineData) > 0 && lineData[len(lineData)-1] == '\r' {
			lineData = lineData[:len(lineData)-1]
		}
		b.storeCachedLine(row, []rune(string(lineData)))
	}
	return nil
}

func (b *HugeFileBuffer) cacheLineSpans(startRow, endRow int) error {
	if b == nil || b.reader == nil {
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

	checkpoint := b.nearestCheckpoint(startRow)
	if _, err := b.reader.Seek(checkpoint.offset, io.SeekStart); err != nil {
		return err
	}

	currentRow := checkpoint.row
	lineStart := checkpoint.offset
	fileOffset := checkpoint.offset
	buf := make([]byte, 64<<10)

	for {
		n, err := b.reader.Read(buf)
		if n > 0 {
			for i, ch := range buf[:n] {
				if ch != '\n' {
					continue
				}
				b.storeLineSpan(currentRow, hugeFileLineSpan{
					start: lineStart,
					end:   fileOffset + int64(i),
				})
				nextRow := currentRow + 1
				nextOffset := fileOffset + int64(i) + 1
				b.rememberCheckpoint(nextRow, nextOffset)
				if currentRow >= endRow {
					return nil
				}
				currentRow = nextRow
				lineStart = nextOffset
			}
			fileOffset += int64(n)
		}
		if err == io.EOF {
			b.storeLineSpan(currentRow, hugeFileLineSpan{
				start: lineStart,
				end:   b.sizeBytes,
			})
			b.completeIndex(currentRow + 1)
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (b *HugeFileBuffer) hasCachedLineSpans(startRow, endRow int) bool {
	for row := startRow; row <= endRow; row++ {
		if _, ok := b.lineSpans[row]; !ok {
			return false
		}
	}
	return true
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
