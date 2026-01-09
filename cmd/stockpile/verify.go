package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/discochess/stockpile/internal/codec/zstdcodec"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the integrity of the evaluation database",
	Long: `Verify that all shards in the database are valid.

This command checks:
- Each shard can be decompressed
- Each shard contains valid JSONL
- Positions are sorted within each shard`,
	RunE: runVerify,
}

var (
	verifyQuick bool
)

func init() {
	verifyCmd.Flags().BoolVar(&verifyQuick, "quick", false, "only check first and last entries in each shard")
	rootCmd.AddCommand(verifyCmd)
}

func runVerify(cmd *cobra.Command, args []string) error {
	shardsDir := filepath.Join(dataDir, "shards")

	// Check if shards directory exists.
	if _, err := os.Stat(shardsDir); os.IsNotExist(err) {
		return fmt.Errorf("data directory %q does not exist", dataDir)
	}

	// List shard files.
	entries, err := os.ReadDir(shardsDir)
	if err != nil {
		return fmt.Errorf("reading shards directory: %w", err)
	}

	// Filter to only .zst files.
	var shardFiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zst") {
			continue
		}
		shardFiles = append(shardFiles, filepath.Join(shardsDir, entry.Name()))
	}

	if len(shardFiles) == 0 {
		fmt.Println("No shards found in data directory.")
		return nil
	}

	fmt.Printf("Verifying %d shards...\n", len(shardFiles))

	codec := zstdcodec.New()

	var errCount int
	for i, path := range shardFiles {
		name := filepath.Base(path)
		if verbose {
			fmt.Printf("  [%d/%d] %s\n", i+1, len(shardFiles), name)
		}

		// Read shard.
		compressed, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("  ERROR: %s: %v\n", name, err)
			errCount++
			continue
		}

		// Decompress using codec.
		reader, err := codec.Reader(bytes.NewReader(compressed))
		if err != nil {
			fmt.Printf("  ERROR: %s: creating decompressor: %v\n", name, err)
			errCount++
			continue
		}

		decompressed, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Printf("  ERROR: %s: decompression failed: %v\n", name, err)
			errCount++
			continue
		}

		// Check JSONL structure.
		if err := verifyJSONL(decompressed, verifyQuick); err != nil {
			fmt.Printf("  ERROR: %s: %v\n", name, err)
			errCount++
			continue
		}
	}

	if errCount > 0 {
		return fmt.Errorf("%d shards failed verification", errCount)
	}

	fmt.Println("All shards verified successfully.")
	return nil
}

func verifyJSONL(data []byte, quick bool) error {
	lines := splitLinesForVerify(data)
	if len(lines) == 0 {
		return fmt.Errorf("empty shard")
	}

	// Check sorting by extracting FENs.
	var prevFEN string
	indicesToCheck := make([]int, 0, len(lines))

	if quick {
		// Only check first and last.
		indicesToCheck = append(indicesToCheck, 0)
		if len(lines) > 1 {
			indicesToCheck = append(indicesToCheck, len(lines)-1)
		}
	} else {
		// Check all lines.
		for i := range lines {
			indicesToCheck = append(indicesToCheck, i)
		}
	}

	for _, idx := range indicesToCheck {
		fen := extractFENForVerify(lines[idx])
		if fen == "" {
			return fmt.Errorf("line %d: invalid JSON or missing FEN", idx+1)
		}

		if prevFEN != "" && fen < prevFEN {
			return fmt.Errorf("lines not sorted: %q comes after %q", fen, prevFEN)
		}
		prevFEN = fen
	}

	return nil
}

func splitLinesForVerify(data []byte) [][]byte {
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

func extractFENForVerify(line []byte) string {
	const prefix = `"fen":"`
	s := string(line)
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return ""
	}
	start := idx + len(prefix)
	end := strings.Index(s[start:], `"`)
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}
