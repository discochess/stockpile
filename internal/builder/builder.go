package builder

import (
	"bufio"
	"bytes"
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

// writeShard sorts and writes a single shard.
func (b *Builder) writeShard(ctx context.Context, shardID int, collector *shardCollector) (int, error) {
	// Get all records.
	records, err := collector.GetAll()
	if err != nil {
		return 0, err
	}

	if len(records) == 0 {
		return 0, nil
	}

	// Sort by FEN.
	sort.Slice(records, func(i, j int) bool {
		fenI := extractFEN(records[i])
		fenJ := extractFEN(records[j])
		return fenI < fenJ
	})

	// Write compressed shard.
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

	for _, record := range records {
		if _, err := encoder.Write(record); err != nil {
			return 0, err
		}
		if _, err := encoder.Write([]byte("\n")); err != nil {
			return 0, err
		}
	}

	return len(records), nil
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
	spilledFile  string // non-empty if spilled to disk
	spilledCount int    // number of records spilled
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

	// Open temp file for append (create if doesn't exist).
	tempFile := filepath.Join(c.tempDir, fmt.Sprintf("shard_%05d.tmp", c.shardID))

	file, err := os.OpenFile(tempFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening temp file: %w", err)
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
	c.spilledFile = tempFile
	c.spilledCount += len(c.records) // Accumulate across multiple spills
	c.records = nil
	c.memoryBytes = 0

	return nil
}

func (c *shardCollector) Count() int {
	return len(c.records) + c.spilledCount
}

func (c *shardCollector) GetAll() ([][]byte, error) {
	var allRecords [][]byte

	// Load spilled records if any.
	if c.spilledFile != "" {
		file, err := os.Open(c.spilledFile)
		if err != nil {
			return nil, fmt.Errorf("opening spilled file: %w", err)
		}
		defer file.Close()

		reader := bufio.NewReader(file)
		for {
			// Read 4-byte length prefix.
			lengthBuf := make([]byte, 4)
			if _, err := io.ReadFull(reader, lengthBuf); err != nil {
				if err == io.EOF {
					break
				}
				return nil, fmt.Errorf("reading length: %w", err)
			}
			length := uint32(lengthBuf[0])<<24 | uint32(lengthBuf[1])<<16 |
				uint32(lengthBuf[2])<<8 | uint32(lengthBuf[3])

			record := make([]byte, length)
			if _, err := io.ReadFull(reader, record); err != nil {
				return nil, fmt.Errorf("reading record: %w", err)
			}
			allRecords = append(allRecords, record)
		}
	}

	// Append in-memory records.
	allRecords = append(allRecords, c.records...)

	return allRecords, nil
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
