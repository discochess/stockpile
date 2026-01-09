//go:build e2e

package stockpile_test

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/discochess/stockpile"
	"github.com/discochess/stockpile/internal/codec/zstdcodec"
	"github.com/discochess/stockpile/internal/store/cachedstore"
	"github.com/discochess/stockpile/internal/store/cachedstore/cachestrategy/lru"
	"github.com/discochess/stockpile/internal/store/cachedstore/memory"
	"github.com/discochess/stockpile/internal/store/diskstore"
)

func TestE2E_RealData(t *testing.T) {
	sourceFile := "./data/lichess_db_eval.jsonl.zst"
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		t.Skip("Skipping: lichess_db_eval.jsonl.zst not found in data/")
	}

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "stockpile-e2e-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sampleFile := filepath.Join(tmpDir, "sample.jsonl")
	dataDir := filepath.Join(tmpDir, "data")

	// Step 1: Extract sample (first 10,000 positions)
	t.Log("📦 Extracting 10,000 sample positions...")
	start := time.Now()
	fens, err := extractSample(sourceFile, sampleFile, 10000)
	if err != nil {
		t.Fatalf("Error extracting sample: %v", err)
	}
	t.Logf("   Extracted %d positions in %v", len(fens), time.Since(start))

	// Step 2: Build shards
	t.Log("🔨 Building shards...")
	start = time.Now()
	cmd := exec.Command("go", "run", "./cmd/stockpile", "build",
		"--source", sampleFile,
		"--output", dataDir,
		"--shards", "64",
		"--workers", "4",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Error building: %v", err)
	}
	t.Logf("   Built shards in %v", time.Since(start))

	// Step 3: Test lookups
	t.Log("🔍 Testing lookups...")

	baseStore, err := diskstore.New(dataDir, zstdcodec.New())
	if err != nil {
		t.Fatalf("Error opening store: %v", err)
	}

	lruStrategy, _ := lru.New(100)
	st := cachedstore.New(baseStore, memory.New(lruStrategy, nil))

	client, err := stockpile.New(
		stockpile.WithStore(st),
		stockpile.WithTotalShards(64), // Must match build shards
	)
	if err != nil {
		t.Fatalf("Error creating client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	found := 0
	var totalTime time.Duration

	testCount := min(100, len(fens))
	for i := 0; i < testCount; i++ {
		fen := fens[i]
		start := time.Now()
		eval, err := client.Lookup(ctx, fen)
		elapsed := time.Since(start)
		totalTime += elapsed

		if err == nil {
			found++
			if i < 5 {
				shortFen := fen
				if len(fen) > 50 {
					shortFen = fen[:50] + "..."
				}
				t.Logf("   ✓ %s", shortFen)
				t.Logf("     Score: %s, Depth: %d, Time: %v", eval.Score(), eval.Depth, elapsed)
			}
		}
	}

	t.Logf("📊 Results:")
	t.Logf("   Tested:    %d positions", testCount)
	t.Logf("   Found:     %d (%.1f%%)", found, float64(found)/float64(testCount)*100)
	t.Logf("   Avg time:  %v", totalTime/time.Duration(testCount))

	if found < testCount/2 {
		t.Errorf("Expected to find at least 50%% of positions, found %d/%d", found, testCount)
	}
}

func extractSample(source, dest string, count int) ([]string, error) {
	f, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	decoder, err := zstd.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()

	out, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	scanner := bufio.NewScanner(decoder)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	var fens []string
	n := 0
	for scanner.Scan() && n < count {
		line := scanner.Text()
		out.WriteString(line + "\n")

		// Extract FEN for later testing
		if idx := strings.Index(line, `"fen":"`); idx >= 0 {
			start := idx + 7
			end := strings.Index(line[start:], `"`)
			if end > 0 {
				fens = append(fens, line[start:start+end])
			}
		}
		n++
	}

	return fens, scanner.Err()
}
