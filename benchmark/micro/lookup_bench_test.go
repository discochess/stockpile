package micro

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/discochess/stockpile"
	"github.com/discochess/stockpile/internal/codec/zstdcodec"
	"github.com/discochess/stockpile/internal/store/cachedstore"
	"github.com/discochess/stockpile/internal/store/cachedstore/cachestrategy/lru"
	"github.com/discochess/stockpile/internal/store/cachedstore/memory"
	"github.com/discochess/stockpile/internal/store/diskstore"
)

// BenchmarkLookup_ColdCache measures lookup latency with an empty cache.
// Requires DATA_DIR environment variable pointing to built stockpile data.
func BenchmarkLookup_ColdCache(b *testing.B) {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		b.Skip("DATA_DIR not set; skipping benchmark")
	}

	st, err := diskstore.New(dataDir, zstdcodec.New())
	if err != nil {
		b.Fatalf("creating store: %v", err)
	}
	defer st.Close()

	client, err := stockpile.New(
		stockpile.WithStore(st),
		// No cache - cold lookup every time.
	)
	if err != nil {
		b.Fatalf("creating client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Lookup(ctx, fen)
		if err != nil && err != stockpile.ErrNotFound {
			b.Fatalf("lookup error: %v", err)
		}
	}
}

// BenchmarkLookup_WarmCache measures lookup latency with a warm cache.
func BenchmarkLookup_WarmCache(b *testing.B) {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		b.Skip("DATA_DIR not set; skipping benchmark")
	}

	baseStore, err := diskstore.New(dataDir, zstdcodec.New())
	if err != nil {
		b.Fatalf("creating store: %v", err)
	}

	lruStrategy, err := lru.New(1000)
	if err != nil {
		b.Fatalf("creating LRU strategy: %v", err)
	}
	st := cachedstore.New(baseStore, memory.New(lruStrategy, nil))

	client, err := stockpile.New(
		stockpile.WithStore(st),
	)
	if err != nil {
		b.Fatalf("creating client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -"

	// Warm up the cache.
	_, _ = client.Lookup(ctx, fen)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Lookup(ctx, fen)
		if err != nil && err != stockpile.ErrNotFound {
			b.Fatalf("lookup error: %v", err)
		}
	}
}

// BenchmarkLookup_VariedPositions measures lookup with different positions.
func BenchmarkLookup_VariedPositions(b *testing.B) {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		b.Skip("DATA_DIR not set; skipping benchmark")
	}

	baseStore, err := diskstore.New(dataDir, zstdcodec.New())
	if err != nil {
		b.Fatalf("creating store: %v", err)
	}

	lruStrategy, err := lru.New(100)
	if err != nil {
		b.Fatalf("creating LRU strategy: %v", err)
	}
	st := cachedstore.New(baseStore, memory.New(lruStrategy, nil))

	client, err := stockpile.New(
		stockpile.WithStore(st),
	)
	if err != nil {
		b.Fatalf("creating client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Common chess positions.
	positions := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -",           // Starting
		"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3",        // 1.e4
		"rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6",      // 1.e4 e5
		"rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq -",     // 1.e4 e5 2.Nf3
		"r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq -",   // Italian setup
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fen := positions[i%len(positions)]
		_, err := client.Lookup(ctx, fen)
		if err != nil && err != stockpile.ErrNotFound {
			b.Fatalf("lookup error: %v", err)
		}
	}
}

// BenchmarkZstdDecompress measures zstd decompression speed.
func BenchmarkZstdDecompress(b *testing.B) {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		b.Skip("DATA_DIR not set; skipping benchmark")
	}

	// Read a sample shard.
	shardPath := filepath.Join(dataDir, "shards", "00000.jsonl.zst")
	data, err := os.ReadFile(shardPath)
	if err != nil {
		b.Skipf("could not read shard: %v", err)
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		b.Fatalf("creating decoder: %v", err)
	}
	defer decoder.Close()

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := decoder.DecodeAll(data, nil)
		if err != nil {
			b.Fatalf("decode error: %v", err)
		}
	}
}

// BenchmarkBinarySearch measures binary search speed on decompressed data.
func BenchmarkBinarySearch(b *testing.B) {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		b.Skip("DATA_DIR not set; skipping benchmark")
	}

	// Read and decompress a sample shard.
	shardPath := filepath.Join(dataDir, "shards", "00000.jsonl.zst")
	compressed, err := os.ReadFile(shardPath)
	if err != nil {
		b.Skipf("could not read shard: %v", err)
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		b.Fatalf("creating decoder: %v", err)
	}
	defer decoder.Close()

	data, err := decoder.DecodeAll(compressed, nil)
	if err != nil {
		b.Fatalf("decode error: %v", err)
	}

	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Binary search simulation.
		_ = searchFEN(data, fen)
	}
}

// searchFEN is a simplified binary search for benchmarking.
func searchFEN(data []byte, target string) bool {
	lines := splitLines(data)
	lo, hi := 0, len(lines)-1

	for lo <= hi {
		mid := (lo + hi) / 2
		fen := extractFEN(lines[mid])

		if fen == target {
			return true
		} else if fen < target {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return false
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func extractFEN(line []byte) string {
	const prefix = `"fen":"`
	for i := 0; i < len(line)-len(prefix); i++ {
		if string(line[i:i+len(prefix)]) == prefix {
			start := i + len(prefix)
			for j := start; j < len(line); j++ {
				if line[j] == '"' {
					return string(line[start:j])
				}
			}
		}
	}
	return ""
}

// TestMicroBenchmarksCompile ensures the benchmarks compile.
func TestMicroBenchmarksCompile(t *testing.T) {
	// This test just ensures the benchmark code compiles.
	_ = fmt.Sprintf("benchmarks compile")
}
