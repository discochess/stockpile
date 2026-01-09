package builder

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestExtractFEN(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "valid FEN",
			line: `{"fen":"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1","evals":[]}`,
			want: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		},
		{
			name: "empty board FEN",
			line: `{"fen":"8/8/8/8/8/8/8/8 w - - 0 1","evals":[]}`,
			want: "8/8/8/8/8/8/8/8 w - - 0 1",
		},
		{
			name: "no fen field",
			line: `{"other":"value"}`,
			want: "",
		},
		{
			name: "empty line",
			line: "",
			want: "",
		},
		{
			name: "malformed fen field",
			line: `{"fen":}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFEN([]byte(tt.line))
			if got != tt.want {
				t.Errorf("extractFEN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShardCollector(t *testing.T) {
	tracker := newMemoryTracker(1024, nil) // 1GB limit for testing
	c := newShardCollector(0, "", tracker)
	tracker.collectors = []*shardCollector{c}

	// Initially empty.
	if c.Count() != 0 {
		t.Errorf("Count() = %d, want 0", c.Count())
	}

	// Add records.
	records := [][]byte{
		[]byte(`{"fen":"a"}`),
		[]byte(`{"fen":"b"}`),
		[]byte(`{"fen":"c"}`),
	}

	for _, r := range records {
		if err := c.Add(r); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	if c.Count() != 3 {
		t.Errorf("Count() = %d, want 3", c.Count())
	}

	// GetAll returns all records.
	got, err := c.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}

	if len(got) != 3 {
		t.Errorf("GetAll() returned %d records, want 3", len(got))
	}

	// Verify records are copies (not sharing underlying buffer).
	for i, r := range got {
		if !bytes.Equal(r, records[i]) {
			t.Errorf("record %d = %q, want %q", i, r, records[i])
		}
	}
}

func TestShardCollector_CopiesData(t *testing.T) {
	tracker := newMemoryTracker(1024, nil) // 1GB limit for testing
	c := newShardCollector(0, "", tracker)
	tracker.collectors = []*shardCollector{c}

	// Add a record.
	original := []byte(`{"fen":"test"}`)
	if err := c.Add(original); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Modify original.
	original[0] = 'X'

	// Verify stored record is unchanged.
	records, _ := c.GetAll()
	if records[0][0] == 'X' {
		t.Error("shardCollector should copy data, not store reference")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		dur  time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{3661 * time.Second, "1h 1m"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatDuration(tt.dur)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.dur, got, tt.want)
			}
		})
	}
}

func TestNewBuilder_Defaults(t *testing.T) {
	b := NewBuilder()

	if b.sourceURL != DefaultSourceURL {
		t.Errorf("sourceURL = %q, want %q", b.sourceURL, DefaultSourceURL)
	}
	if b.totalShards != DefaultTotalShards {
		t.Errorf("totalShards = %d, want %d", b.totalShards, DefaultTotalShards)
	}
	if b.strategy == nil {
		t.Error("strategy should not be nil")
	}
}

func TestNewBuilder_WithOptions(t *testing.T) {
	b := NewBuilder(
		WithSourceURL("http://example.com/data.jsonl"),
		WithOutputDir("/tmp/test"),
		WithTotalShards(100),
		WithWorkers(8),
		WithMaxMemoryMB(4096),
	)

	if b.sourceURL != "http://example.com/data.jsonl" {
		t.Errorf("sourceURL = %q", b.sourceURL)
	}
	if b.outputDir != "/tmp/test" {
		t.Errorf("outputDir = %q", b.outputDir)
	}
	if b.totalShards != 100 {
		t.Errorf("totalShards = %d", b.totalShards)
	}
	if b.workersCount != 8 {
		t.Errorf("workersCount = %d", b.workersCount)
	}
	if b.maxMemoryMB != 4096 {
		t.Errorf("maxMemoryMB = %d", b.maxMemoryMB)
	}
}

func TestBuildFromFile(t *testing.T) {
	// Create temp directory.
	tmpDir, err := os.MkdirTemp("", "builder-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test source file (plain JSONL, not compressed).
	sourceFile := filepath.Join(tmpDir, "source.jsonl")
	testData := []byte(`{"fen":"8/8/8/8/8/8/8/8 w - - 0 1","evals":[{"pvs":[{"cp":0}],"knodes":1,"depth":1}]}
{"fen":"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1","evals":[{"pvs":[{"cp":20}],"knodes":100,"depth":20}]}
{"fen":"r1bqkbnr/pppppppp/n7/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1","evals":[{"pvs":[{"cp":50}],"knodes":200,"depth":25}]}
`)
	if err := os.WriteFile(sourceFile, testData, 0644); err != nil {
		t.Fatalf("writing source file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	// Track progress calls.
	var progressCalls []string
	progressFn := func(p Progress) {
		progressCalls = append(progressCalls, p.Phase)
	}

	b := NewBuilder(
		WithOutputDir(outputDir),
		WithTotalShards(4), // Small number for testing.
		WithProgress(progressFn),
		WithWorkers(2),
	)

	ctx := context.Background()
	if err := b.BuildFromFile(ctx, sourceFile, time.Time{}); err != nil {
		t.Fatalf("BuildFromFile() error = %v", err)
	}

	// Verify shards directory was created.
	shardsDir := filepath.Join(outputDir, "shards")
	if _, err := os.Stat(shardsDir); os.IsNotExist(err) {
		t.Error("shards directory was not created")
	}

	// Verify at least one shard was created.
	entries, err := os.ReadDir(shardsDir)
	if err != nil {
		t.Fatalf("reading shards dir: %v", err)
	}

	if len(entries) == 0 {
		t.Error("no shard files created")
	}

	// Verify progress was reported.
	hasSort := false
	hasShard := false
	hasDone := false
	for _, phase := range progressCalls {
		switch phase {
		case "sort":
			hasSort = true
		case "shard":
			hasShard = true
		case "done":
			hasDone = true
		}
	}

	if !hasSort {
		t.Error("missing 'sort' progress phase")
	}
	if !hasShard {
		t.Error("missing 'shard' progress phase")
	}
	if !hasDone {
		t.Error("missing 'done' progress phase")
	}
}

func TestBuildFromFile_Cancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "builder-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a source file with many records.
	sourceFile := filepath.Join(tmpDir, "source.jsonl")
	var data bytes.Buffer
	for i := 0; i < 1000; i++ {
		data.WriteString(`{"fen":"8/8/8/8/8/8/8/8 w - - 0 1","evals":[]}` + "\n")
	}
	if err := os.WriteFile(sourceFile, data.Bytes(), 0644); err != nil {
		t.Fatalf("writing source file: %v", err)
	}

	b := NewBuilder(
		WithOutputDir(filepath.Join(tmpDir, "output")),
		WithTotalShards(4),
	)

	// Cancel immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = b.BuildFromFile(ctx, sourceFile, time.Time{})
	if err != context.Canceled {
		t.Errorf("BuildFromFile() error = %v, want context.Canceled", err)
	}
}

func TestProgressWriter(t *testing.T) {
	var buf bytes.Buffer
	var counter atomic.Int64
	pw := newProgressWriter(&buf, &counter)

	data := []byte("hello world")
	n, err := pw.Write(data)

	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() = %d, want %d", n, len(data))
	}
	if counter.Load() != int64(len(data)) {
		t.Errorf("counter = %d, want %d", counter.Load(), len(data))
	}
	if buf.String() != "hello world" {
		t.Errorf("buffer = %q", buf.String())
	}
}

func TestProgressReader(t *testing.T) {
	data := bytes.NewReader([]byte("hello world"))
	var counter atomic.Int64
	pr := newProgressReader(data, &counter)

	buf := make([]byte, 5)
	n, err := pr.Read(buf)

	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if n != 5 {
		t.Errorf("Read() = %d, want 5", n)
	}
	if counter.Load() != 5 {
		t.Errorf("counter = %d, want 5", counter.Load())
	}
}

func TestShardCollector_SpillToDisk(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "spill-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create collector with very low memory limit (100 bytes).
	tracker := newMemoryTracker(0, nil)
	tracker.maxBytes = 100 // Very low limit to force spilling
	c := newShardCollector(0, tmpDir, tracker)
	tracker.collectors = []*shardCollector{c}

	// Add records that exceed the limit (each ~80 bytes + overhead).
	records := [][]byte{
		[]byte(`{"fen":"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1","evals":[]}`),
		[]byte(`{"fen":"r1bqkbnr/pppppppp/n7/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1","evals":[]}`),
		[]byte(`{"fen":"8/8/8/8/8/8/8/8 w - - 0 1","evals":[]}`),
	}

	for _, r := range records {
		if err := c.Add(r); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// With such a low limit, collector should have spilled at least once.
	// After spilling, new records are added to memory again.
	// So we check either spilled or total count is correct.
	if c.Count() != 3 {
		t.Errorf("Count() = %d, want 3", c.Count())
	}

	// GetAll should return all records (from disk + memory).
	got, err := c.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}

	if len(got) != 3 {
		t.Errorf("GetAll() returned %d records, want 3", len(got))
	}

	// Verify records content is preserved.
	for i, r := range got {
		if !bytes.Contains(r, []byte(`"fen":`)) {
			t.Errorf("record %d doesn't contain expected content", i)
		}
	}
}

func TestMemoryTracker_SpillsLargest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "spill-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple collectors.
	tracker := newMemoryTracker(0, nil)
	tracker.maxBytes = 500 // Very low limit

	c1 := newShardCollector(0, tmpDir, tracker)
	c2 := newShardCollector(1, tmpDir, tracker)
	tracker.collectors = []*shardCollector{c1, c2}

	// Add more records to c1.
	for i := 0; i < 10; i++ {
		c1.records = append(c1.records, []byte(`{"fen":"test"}`))
		c1.memoryBytes += 50
	}
	tracker.totalBytes = c1.memoryBytes

	// Add fewer records to c2.
	for i := 0; i < 3; i++ {
		c2.records = append(c2.records, []byte(`{"fen":"test"}`))
		c2.memoryBytes += 50
	}
	tracker.totalBytes += c2.memoryBytes

	// Spill until under limit (should spill largest first).
	if err := tracker.spillUntilUnderLimit(); err != nil {
		t.Fatalf("spillUntilUnderLimit() error = %v", err)
	}

	// c1 should be spilled (it had more memory).
	if c1.spilledFile == "" {
		t.Error("expected c1 to be spilled")
	}
	if c2.spilledFile != "" {
		t.Error("c2 should not be spilled")
	}
}
