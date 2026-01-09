package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/discochess/stockpile/internal/builder"
	"github.com/discochess/stockpile/internal/shard"
	"github.com/discochess/stockpile/internal/shard/fnvshard"
	"github.com/discochess/stockpile/internal/shard/materialshard"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the evaluation database from Lichess source",
	Long: `Download and process the Lichess evaluation database.

This command will:
1. Download the evaluation data from Lichess (or use a local file)
2. Distribute positions to shards using the configured strategy
3. Sort positions within each shard by FEN
4. Compress shards with zstd

The default source is the Lichess evaluation database:
  https://database.lichess.org/lichess_db_eval.jsonl.zst

Examples:
  # Build from default Lichess source
  stockpile build --output ./data

  # Build from a local file
  stockpile build --source ./lichess_db_eval.jsonl.zst --output ./data

  # Specify number of shards and strategy
  stockpile build --output ./data --shards 32768 --strategy material

  # Build and upload to GCS (for cronjobs)
  stockpile build --output-gcs gs://my-bucket/stockpile`,
	RunE: runBuild,
}

var (
	sourceURL    string
	outputDir    string
	outputGCS    string
	totalShards  int
	strategyName string
	workers      int
	maxMemoryMB  int
)

func init() {
	buildCmd.Flags().StringVar(&sourceURL, "source", builder.DefaultSourceURL, "source URL or local file path")
	buildCmd.Flags().StringVarP(&outputDir, "output", "o", "./data", "output directory for shards (local builds)")
	buildCmd.Flags().StringVar(&outputGCS, "output-gcs", "", "GCS path for output (gs://bucket/prefix)")
	buildCmd.Flags().IntVar(&totalShards, "shards", builder.DefaultTotalShards, "number of shards to create")
	buildCmd.Flags().StringVar(&strategyName, "strategy", "material", "sharding strategy: material, fnv32")
	buildCmd.Flags().IntVar(&workers, "workers", 4, "number of parallel workers for compression")
	buildCmd.Flags().IntVar(&maxMemoryMB, "max-memory", 1024, "max memory in MB before spilling to disk (lower = less RAM usage)")
	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) error {
	// Select strategy.
	var strategy shard.Strategy
	switch strategyName {
	case "material":
		strategy = materialshard.New()
	case "fnv32":
		strategy = fnvshard.New()
	default:
		return fmt.Errorf("unknown strategy: %s", strategyName)
	}

	// Check if source is a local file.
	isLocalFile := false
	if _, err := os.Stat(sourceURL); err == nil {
		isLocalFile = true
	}

	// Setup context with cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted, cleaning up...")
		cancel()
	}()

	// Determine output directory.
	localOutput := outputDir
	cleanupLocal := false
	if outputGCS != "" {
		// Build to temp directory, then upload to GCS.
		tmpDir, err := os.MkdirTemp("", "stockpile-build-*")
		if err != nil {
			return fmt.Errorf("creating temp directory: %w", err)
		}
		localOutput = tmpDir
		cleanupLocal = true
		defer func() {
			if cleanupLocal {
				os.RemoveAll(tmpDir)
			}
		}()
	}

	// Create builder.
	b := builder.NewBuilder(
		builder.WithSourceURL(sourceURL),
		builder.WithOutputDir(localOutput),
		builder.WithTotalShards(totalShards),
		builder.WithStrategy(strategy),
		builder.WithWorkers(workers),
		builder.WithMaxMemoryMB(maxMemoryMB),
		builder.WithProgress(builder.DefaultProgressFunc),
	)

	fmt.Printf("Building stockpile database\n")
	fmt.Printf("  Source:     %s\n", sourceURL)
	if outputGCS != "" {
		fmt.Printf("  Output:     %s (via local temp)\n", outputGCS)
	} else {
		fmt.Printf("  Output:     %s\n", localOutput)
	}
	fmt.Printf("  Shards:     %d\n", totalShards)
	fmt.Printf("  Strategy:   %s\n", strategy.Name())
	fmt.Printf("  Workers:    %d\n", workers)
	fmt.Printf("  Max Memory: %d MB\n", maxMemoryMB)
	fmt.Println()

	// Run build.
	var err error
	if isLocalFile {
		err = b.BuildFromFile(ctx, sourceURL, time.Time{})
	} else {
		err = b.Build(ctx)
	}
	if err != nil {
		return err
	}

	// Upload to GCS if specified.
	if outputGCS != "" {
		fmt.Println()
		fmt.Printf("[Upload] Uploading to %s...\n", outputGCS)

		uploader, err := builder.NewGCSUploader(ctx, outputGCS)
		if err != nil {
			return fmt.Errorf("creating GCS uploader: %w", err)
		}
		defer uploader.Close()

		if err := uploader.Upload(ctx, localOutput, builder.DefaultProgressFunc); err != nil {
			return fmt.Errorf("uploading to GCS: %w", err)
		}

		fmt.Println("[Upload] Done")
	}

	return nil
}
