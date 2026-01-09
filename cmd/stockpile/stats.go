package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show statistics about the evaluation database",
	Long: `Display statistics about the evaluation database including:
- Number of shards
- Total size on disk
- Compression ratio (if available)`,
	RunE: runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	shardsDir := filepath.Join(dataDir, "shards")

	// Check if shards directory exists.
	if _, err := os.Stat(shardsDir); os.IsNotExist(err) {
		return fmt.Errorf("data directory %q does not exist; run 'stockpile build' first", dataDir)
	}

	// List shard files.
	entries, err := os.ReadDir(shardsDir)
	if err != nil {
		return fmt.Errorf("reading shards directory: %w", err)
	}

	// Filter to only .zst files and calculate total size.
	var shardCount int
	var totalSize int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zst") {
			continue
		}
		shardCount++
		info, err := entry.Info()
		if err != nil {
			continue
		}
		totalSize += info.Size()
	}

	if shardCount == 0 {
		fmt.Println("No shards found in data directory.")
		fmt.Println("Run 'stockpile build' to create the database.")
		return nil
	}

	fmt.Printf("Data directory: %s\n", dataDir)
	fmt.Printf("Shards:         %d\n", shardCount)
	fmt.Printf("Total size:     %s\n", formatBytes(totalSize))

	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
