package builder

import (
	"bufio"
	"bytes"
	"container/heap"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/discochess/stockpile/internal/shard"
	"github.com/discochess/stockpile/internal/shard/materialshard"
)

const (
	// DefaultTotalShards is the default number of shards to create.
	DefaultTotalShards = 32768

	// DefaultSourceURL is the Lichess evaluation database URL.
	DefaultSourceURL = "https://database.lichess.org/lichess_db_eval.jsonl.zst"
)

// Builder builds the stockpile database from source data.
type Builder struct {
	sourceURL     string
	outputDir     string
	totalShards   int
	strategy      shard.Strategy
	progress      ProgressFunc
	tempDir       string
	maxMemoryMB   int
	workersCount  int
}

// Option configures the Builder.
type Option func(*Builder)

// WithSourceURL sets the source URL.
func WithSourceURL(url string) Option {
	return func(b *Builder) { b.sourceURL = url }
}

// WithOutputDir sets the output directory.
func WithOutputDir(dir string) Option {
	return func(b *Builder) { b.outputDir = dir }
}

// WithTotalShards sets the number of shards.
func WithTotalShards(n int) Option {
	return func(b *Builder) { b.totalShards = n }
}

// WithStrategy sets the sharding strategy.
func WithStrategy(s shard.Strategy) Option {
	return func(b *Builder) { b.strategy = s }
}

// WithProgress sets the progress callback.
func WithProgress(fn ProgressFunc) Option {
	return func(b *Builder) { b.progress = fn }
}

// WithTempDir sets the temporary directory for intermediate files.
func WithTempDir(dir string) Option {
	return func(b *Builder) { b.tempDir = dir }
}

// WithMaxMemoryMB sets the maximum memory usage in MB.
func WithMaxMemoryMB(mb int) Option {
	return func(b *Builder) { b.maxMemoryMB = mb }
}

// WithWorkers sets the number of parallel workers for compression.
func WithWorkers(n int) Option {
	return func(b *Builder) { b.workersCount = n }
}

// NewBuilder creates a new Builder with the given options.
func NewBuilder(opts ...Option) *Builder {
	b := &Builder{
		sourceURL:    DefaultSourceURL,
		outputDir:    "./data",
		totalShards:  DefaultTotalShards,
		strategy:     materialshard.New(),
		progress:     DefaultProgressFunc,
		maxMemoryMB:  2048,
		workersCount: 4,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Build downloads and processes the evaluation database.
func (b *Builder) Build(ctx context.Context) error {
	startTime := time.Now()

	// Create output directory.
	shardsDir := filepath.Join(b.outputDir, "shards")
	if err := os.MkdirAll(shardsDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Create temp directory.
	if b.tempDir == "" {
		b.tempDir = filepath.Join(b.outputDir, ".tmp")
	}
	if err := os.MkdirAll(b.tempDir, 0755); err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(b.tempDir)

	// Download source file.
	downloadPath := filepath.Join(b.tempDir, "source.jsonl.zst")
	b.reportProgress(Progress{Phase: "download", StartTime: startTime})

	downloader := NewDownloader()
	if err := downloader.DownloadToFile(ctx, b.sourceURL, downloadPath, b.progress); err != nil {
		return fmt.Errorf("downloading source: %w", err)
	}

	// Process the downloaded file.
	return b.BuildFromFile(ctx, downloadPath, startTime)
}

// BuildFromFile builds the database from a local file.
func (b *Builder) BuildFromFile(ctx context.Context, sourcePath string, startTime time.Time) error {
	if startTime.IsZero() {
		startTime = time.Now()
	}

	// Clean and create output directory.
	shardsDir := filepath.Join(b.outputDir, "shards")
	if err := os.RemoveAll(shardsDir); err != nil {
		return fmt.Errorf("cleaning shards directory: %w", err)
	}
	if err := os.MkdirAll(shardsDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Create temp directory.
	if b.tempDir == "" {
		b.tempDir = filepath.Join(b.outputDir, ".tmp")
	}
	if err := os.MkdirAll(b.tempDir, 0755); err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(b.tempDir)

	// Open source file.
	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer file.Close()

	// Wrap with zstd decoder if needed.
	var reader io.Reader = file
	if filepath.Ext(sourcePath) == ".zst" {
		decoder, err := zstd.NewReader(file)
		if err != nil {
			return fmt.Errorf("creating zstd decoder: %w", err)
		}
		defer decoder.Close()
		reader = decoder
	}

	// Process records into shards.
	return b.processRecords(ctx, reader, shardsDir, startTime)
}

// processRecords reads records and distributes them to shards.
func (b *Builder) processRecords(ctx context.Context, reader io.Reader, shardsDir string, startTime time.Time) error {
	// Create shard collectors with memory tracking.
	collectors := make([]*shardCollector, b.totalShards)
	tracker := newMemoryTracker(b.maxMemoryMB, collectors)
	for i := range collectors {
		collectors[i] = newShardCollector(i, b.tempDir, tracker)
	}
	tracker.collectors = collectors // Update reference after creation

	// Read and distribute records.
	b.reportProgress(Progress{Phase: "sort", StartTime: startTime})

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max line.

	var recordsRead int64
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Extract FEN for sharding.
		fen := extractFEN(line)
		if fen == "" {
			continue
		}

		// Determine shard.
		shardID := b.strategy.ShardID(fen, b.totalShards)
		if err := collectors[shardID].Add(line); err != nil {
			return fmt.Errorf("adding to shard %d: %w", shardID, err)
		}

		recordsRead++
		if recordsRead%100000 == 0 {
			b.reportProgress(Progress{
				Phase:       "sort",
				RecordsRead: recordsRead,
				StartTime:   startTime,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading source: %w", err)
	}

	// Write shards.
	b.reportProgress(Progress{
		Phase:       "shard",
		RecordsRead: recordsRead,
		ShardsTotal: b.totalShards,
		StartTime:   startTime,
	})

	var recordsWritten int64
	var shardsCreated int
	var mu sync.Mutex

	// Process shards in parallel.
	sem := make(chan struct{}, b.workersCount)
	errCh := make(chan error, b.totalShards)
	var wg sync.WaitGroup

	for i, collector := range collectors {
		if collector.Count() == 0 {
			continue
		}

		wg.Add(1)
		go func(shardID int, c *shardCollector) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// Sort and write shard.
			count, err := b.writeShard(ctx, shardID, c)
			if err != nil {
				errCh <- fmt.Errorf("writing shard %d: %w", shardID, err)
				return
			}

			mu.Lock()
			recordsWritten += int64(count)
			shardsCreated++
			b.reportProgress(Progress{
				Phase:          "shard",
				RecordsRead:    recordsRead,
				RecordsWritten: recordsWritten,
				ShardsCreated:  shardsCreated,
				ShardsTotal:    b.totalShards,
				StartTime:      startTime,
			})
			mu.Unlock()
		}(i, collector)
	}

	wg.Wait()
	close(errCh)

	// Check for errors.
	for err := range errCh {
		if err != nil {
			return err
		}
	}

	b.reportProgress(Progress{
		Phase:          "done",
		RecordsRead:    recordsRead,
		RecordsWritten: recordsWritten,
		ShardsCreated:  shardsCreated,
		ShardsTotal:    b.totalShards,
		StartTime:      startTime,
	})

	// Write manifest.
	manifest := &Manifest{
		Version:     1,
		TotalShards: b.totalShards,
		Strategy:    b.strategy.Name(),
		RecordCount: recordsWritten,
		ShardCount:  shardsCreated,
		BuiltAt:     time.Now(),
		SourceURL:   b.sourceURL,
		Compression: "zstd",
	}
	if err := WriteManifest(b.outputDir, manifest); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

// writeShard streams sorted records to a compressed shard file.
func (b *Builder) writeShard(ctx context.Context, shardID int, collector *shardCollector) (int, error) {
	if collector.Count() == 0 {
		return 0, nil
	}

	// Create output file with streaming zstd compression.
	shardPath := filepath.Join(b.outputDir, "shards", fmt.Sprintf("%05d.zst", shardID))
	file, err := os.Create(shardPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	encoder, err := zstd.NewWriter(file, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		return 0, err
	}
	defer encoder.Close()

	// Stream sorted records from merge sort.
	recordCh, errCh := collector.StreamSorted(ctx)

	count := 0
	for record := range recordCh {
		if _, err := encoder.Write(record); err != nil {
			return 0, err
		}
		if _, err := encoder.Write([]byte("\n")); err != nil {
			return 0, err
		}
		count++
	}

	// Check for streaming errors.
	if err := <-errCh; err != nil {
		return 0, err
	}

	return count, nil
}

func (b *Builder) reportProgress(p Progress) {
	if b.progress != nil {
		b.progress(p)
	}
}

// shardCollector collects records for a single shard.
// It can spill records to disk when memory is constrained.
type shardCollector struct {
	shardID      int
	records      [][]byte
	memoryBytes  int64
	tempDir      string
	spilledFiles []string // sorted temp files from spills
	spilledCount int      // number of records spilled
	spillCount   int      // number of spill operations (for unique filenames)
	memTracker   *memoryTracker
}

// memoryTracker tracks total memory usage across all collectors.
type memoryTracker struct {
	mu          sync.Mutex
	totalBytes  int64
	maxBytes    int64
	collectors  []*shardCollector
}

func newMemoryTracker(maxMB int, collectors []*shardCollector) *memoryTracker {
	return &memoryTracker{
		maxBytes:   int64(maxMB) * 1024 * 1024,
		collectors: collectors,
	}
}

func (m *memoryTracker) add(bytes int64) {
	m.mu.Lock()
	m.totalBytes += bytes
	m.mu.Unlock()
}

func (m *memoryTracker) remove(bytes int64) {
	m.mu.Lock()
	m.totalBytes -= bytes
	m.mu.Unlock()
}

func (m *memoryTracker) shouldSpill() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalBytes > m.maxBytes
}

// spillUntilUnderLimit spills collectors until memory is under the limit.
func (m *memoryTracker) spillUntilUnderLimit() error {
	spillCount := 0
	for m.shouldSpill() {
		m.mu.Lock()
		var largest *shardCollector
		var largestMem int64
		for _, c := range m.collectors {
			// Allow spilling shards with in-memory records, even if previously spilled
			if c.memoryBytes > largestMem && len(c.records) > 0 {
				largest = c
				largestMem = c.memoryBytes
			}
		}
		m.mu.Unlock()

		if largest == nil || largestMem == 0 {
			break // Nothing more to spill
		}

		if err := largest.spillToDisk(); err != nil {
			return err
		}
		spillCount++
	}

	// Hint to GC to release memory after significant spilling
	if spillCount > 0 {
		runtime.GC()
	}
	return nil
}

func newShardCollector(shardID int, tempDir string, tracker *memoryTracker) *shardCollector {
	return &shardCollector{
		shardID:    shardID,
		records:    make([][]byte, 0, 1000),
		tempDir:    tempDir,
		memTracker: tracker,
	}
}

func (c *shardCollector) Add(record []byte) error {
	// Make a copy since the scanner reuses the buffer.
	recordCopy := make([]byte, len(record))
	copy(recordCopy, record)
	c.records = append(c.records, recordCopy)

	recordSize := int64(len(recordCopy) + 24) // 24 bytes slice header overhead estimate
	c.memoryBytes += recordSize
	c.memTracker.add(recordSize)

	// Check if we need to spill.
	if c.memTracker.shouldSpill() {
		if err := c.memTracker.spillUntilUnderLimit(); err != nil {
			return fmt.Errorf("spilling to disk: %w", err)
		}
	}

	return nil
}

func (c *shardCollector) spillToDisk() error {
	if len(c.records) == 0 {
		return nil
	}

	// Sort records by FEN before writing (for external merge sort).
	sort.Slice(c.records, func(i, j int) bool {
		return extractFEN(c.records[i]) < extractFEN(c.records[j])
	})

	// Create unique temp file for this spill.
	tempFile := filepath.Join(c.tempDir, fmt.Sprintf("shard_%05d_%d.tmp", c.shardID, c.spillCount))
	c.spillCount++

	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	// Write records with length prefix for reading back.
	writer := bufio.NewWriter(file)
	for _, record := range c.records {
		// Write 4-byte length prefix.
		length := uint32(len(record))
		if _, err := writer.Write([]byte{
			byte(length >> 24),
			byte(length >> 16),
			byte(length >> 8),
			byte(length),
		}); err != nil {
			file.Close()
			return fmt.Errorf("writing length: %w", err)
		}
		if _, err := writer.Write(record); err != nil {
			file.Close()
			return fmt.Errorf("writing record: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		file.Close()
		return fmt.Errorf("flushing: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Update tracker and free memory.
	c.memTracker.remove(c.memoryBytes)
	c.spilledFiles = append(c.spilledFiles, tempFile)
	c.spilledCount += len(c.records)
	c.records = nil
	c.memoryBytes = 0

	return nil
}

func (c *shardCollector) Count() int {
	return len(c.records) + c.spilledCount
}

// mergeEntry represents a record from one of the sorted sources.
type mergeEntry struct {
	record []byte
	fen    string
	source int // index of the source (0=in-memory, 1+=spilled files)
}

// mergeHeap is a min-heap ordered by FEN for k-way merge sort.
type mergeHeap []mergeEntry

func (h mergeHeap) Len() int           { return len(h) }
func (h mergeHeap) Less(i, j int) bool { return h[i].fen < h[j].fen }
func (h mergeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *mergeHeap) Push(x any)        { *h = append(*h, x.(mergeEntry)) }
func (h *mergeHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// sortedFileReader reads length-prefixed records from a sorted temp file.
type sortedFileReader struct {
	file   *os.File
	reader *bufio.Reader
}

func newSortedFileReader(path string) (*sortedFileReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &sortedFileReader{
		file:   file,
		reader: bufio.NewReader(file),
	}, nil
}

func (r *sortedFileReader) Next() ([]byte, error) {
	// Read 4-byte length prefix.
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(r.reader, lengthBuf); err != nil {
		return nil, err // EOF or error
	}
	length := uint32(lengthBuf[0])<<24 | uint32(lengthBuf[1])<<16 |
		uint32(lengthBuf[2])<<8 | uint32(lengthBuf[3])

	record := make([]byte, length)
	if _, err := io.ReadFull(r.reader, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (r *sortedFileReader) Close() error {
	return r.file.Close()
}

// StreamSorted returns records in sorted order using external merge sort.
// It yields records one at a time via channel, never loading all records into memory.
func (c *shardCollector) StreamSorted(ctx context.Context) (<-chan []byte, <-chan error) {
	recordCh := make(chan []byte, 100) // Buffer for smoother streaming
	errCh := make(chan error, 1)

	go func() {
		defer close(recordCh)
		defer close(errCh)

		// Sort in-memory records.
		sort.Slice(c.records, func(i, j int) bool {
			return extractFEN(c.records[i]) < extractFEN(c.records[j])
		})

		// If no spilled files, just yield in-memory records.
		if len(c.spilledFiles) == 0 {
			for _, record := range c.records {
				select {
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				case recordCh <- record:
				}
			}
			return
		}

		// Open readers for spilled files.
		readers := make([]*sortedFileReader, len(c.spilledFiles))
		for i, path := range c.spilledFiles {
			reader, err := newSortedFileReader(path)
			if err != nil {
				errCh <- fmt.Errorf("opening spilled file %s: %w", path, err)
				return
			}
			readers[i] = reader
			defer readers[i].Close()
		}

		// Initialize heap with first record from each source.
		h := &mergeHeap{}
		heap.Init(h)

		// Add first in-memory record (source 0).
		inMemIdx := 0
		if len(c.records) > 0 {
			heap.Push(h, mergeEntry{
				record: c.records[0],
				fen:    extractFEN(c.records[0]),
				source: 0,
			})
			inMemIdx = 1
		}

		// Add first record from each spilled file (sources 1+).
		for i, reader := range readers {
			record, err := reader.Next()
			if err == io.EOF {
				continue
			}
			if err != nil {
				errCh <- fmt.Errorf("reading from spilled file: %w", err)
				return
			}
			heap.Push(h, mergeEntry{
				record: record,
				fen:    extractFEN(record),
				source: i + 1, // 1-indexed for spilled files
			})
		}

		// K-way merge.
		for h.Len() > 0 {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			// Pop smallest.
			entry := heap.Pop(h).(mergeEntry)

			// Yield record.
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case recordCh <- entry.record:
			}

			// Refill from the same source.
			if entry.source == 0 {
				// In-memory source.
				if inMemIdx < len(c.records) {
					heap.Push(h, mergeEntry{
						record: c.records[inMemIdx],
						fen:    extractFEN(c.records[inMemIdx]),
						source: 0,
					})
					inMemIdx++
				}
			} else {
				// Spilled file source.
				reader := readers[entry.source-1]
				record, err := reader.Next()
				if err == io.EOF {
					continue // This source exhausted.
				}
				if err != nil {
					errCh <- fmt.Errorf("reading from spilled file: %w", err)
					return
				}
				heap.Push(h, mergeEntry{
					record: record,
					fen:    extractFEN(record),
					source: entry.source,
				})
			}
		}
	}()

	return recordCh, errCh
}

// extractFEN extracts the FEN from a JSON line.
func extractFEN(line []byte) string {
	const prefix = `"fen":"`
	idx := bytes.Index(line, []byte(prefix))
	if idx < 0 {
		return ""
	}

	start := idx + len(prefix)
	end := bytes.IndexByte(line[start:], '"')
	if end < 0 {
		return ""
	}

	return string(line[start : start+end])
}
