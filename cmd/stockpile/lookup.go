package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/discochess/stockpile"
	"github.com/discochess/stockpile/internal/codec/zstdcodec"
	"github.com/discochess/stockpile/internal/store/cachedstore"
	"github.com/discochess/stockpile/internal/store/cachedstore/cachestrategy/lru"
	"github.com/discochess/stockpile/internal/store/cachedstore/memory"
	"github.com/discochess/stockpile/internal/store/diskstore"
)

var lookupCmd = &cobra.Command{
	Use:   "lookup [FEN]",
	Short: "Look up the evaluation for a chess position",
	Long: `Look up the Stockfish evaluation for a chess position given in FEN notation.

The FEN string should include at least the piece placement and side to move.
Castling rights and en passant square are optional.

Examples:
  # Starting position
  stockpile lookup "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -"

  # After 1.e4
  stockpile lookup "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3"`,
	Args: cobra.ExactArgs(1),
	RunE: runLookup,
}

var (
	outputJSON bool
	showTiming bool
)

func init() {
	lookupCmd.Flags().BoolVar(&outputJSON, "json", false, "output result as JSON")
	lookupCmd.Flags().BoolVar(&showTiming, "timing", false, "show lookup timing")
	rootCmd.AddCommand(lookupCmd)
}

func runLookup(cmd *cobra.Command, args []string) error {
	fen := args[0]

	// Check if data directory exists.
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return fmt.Errorf("data directory %q does not exist; run 'stockpile build' first", dataDir)
	}

	// Create store with caching.
	baseStore, err := diskstore.New(dataDir, zstdcodec.New())
	if err != nil {
		return fmt.Errorf("opening data directory: %w", err)
	}

	lruStrategy, err := lru.New(100)
	if err != nil {
		return fmt.Errorf("creating LRU strategy: %w", err)
	}
	st := cachedstore.New(baseStore, memory.New(lruStrategy, nil))

	// Create client.
	client, err := stockpile.New(
		stockpile.WithStore(st),
	)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}
	defer client.Close()

	// Perform lookup.
	ctx := context.Background()
	start := time.Now()

	eval, err := client.Lookup(ctx, fen)
	if err != nil {
		if errors.Is(err, stockpile.ErrNotFound) {
			return fmt.Errorf("position not found in database")
		}
		return fmt.Errorf("lookup failed: %w", err)
	}

	elapsed := time.Since(start)

	// Output result.
	if outputJSON {
		printEvalJSON(eval, elapsed)
	} else {
		printEvalText(eval, elapsed)
	}

	return nil
}

func printEvalText(eval *stockpile.Eval, elapsed time.Duration) {
	fmt.Printf("FEN:   %s\n", eval.FEN)
	fmt.Printf("Score: %s\n", eval.Score())
	fmt.Printf("Depth: %d\n", eval.Depth)
	for i, pv := range eval.PVs {
		fmt.Printf("PV %d:  %s (%s)\n", i+1, pv.Line, pv.Score())
	}
	if showTiming {
		fmt.Printf("Time:  %s\n", elapsed)
	}
}

func printEvalJSON(eval *stockpile.Eval, elapsed time.Duration) {
	fmt.Printf(`{"fen":%q,"score":%q,"depth":%d,"pvs":[`, eval.FEN, eval.Score(), eval.Depth)
	for i, pv := range eval.PVs {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Print("{")
		if pv.Centipawns != nil {
			fmt.Printf(`"cp":%d,`, *pv.Centipawns)
		}
		if pv.Mate != nil {
			fmt.Printf(`"mate":%d,`, *pv.Mate)
		}
		fmt.Printf(`"line":%q}`, pv.Line)
	}
	fmt.Print("]")
	if showTiming {
		fmt.Printf(`,"elapsed_ms":%d`, elapsed.Milliseconds())
	}
	fmt.Println("}")
}
