// Command analyze-games analyzes positions from a PGN file using stockpile.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/discochess/stockpile"
	"github.com/discochess/stockpile/internal/codec/zstdcodec"
	"github.com/discochess/stockpile/internal/store/cachedstore"
	"github.com/discochess/stockpile/internal/store/cachedstore/cachestrategy/lru"
	"github.com/discochess/stockpile/internal/store/cachedstore/memory"
	"github.com/discochess/stockpile/internal/store/diskstore"
	"github.com/notnil/chess"
)

func main() {
	pgnFile := flag.String("pgn", "./data/DrNykterstein.pgn", "PGN file to analyze")
	dataDir := flag.String("data", "./data", "stockpile data directory")
	maxGames := flag.Int("games", 10, "max games to analyze")
	flag.Parse()

	// Initialize stockpile client
	baseStore, err := diskstore.New(*dataDir, zstdcodec.New())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening data: %v\n", err)
		os.Exit(1)
	}

	lruStrategy, _ := lru.New(1000) // Cache 1000 shards
	st := cachedstore.New(baseStore, memory.New(lruStrategy, nil))

	client, err := stockpile.New(stockpile.WithStore(st))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Open PGN file
	f, err := os.Open(*pgnFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening PGN: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	ctx := context.Background()
	scanner := chess.NewScanner(f)

	var totalPositions, foundPositions int
	var totalLookupTime time.Duration
	gamesAnalyzed := 0

	for scanner.Scan() && gamesAnalyzed < *maxGames {
		game := scanner.Next()
		gamesAnalyzed++

		white := game.GetTagPair("White")
		black := game.GetTagPair("Black")
		result := game.GetTagPair("Result")

		fmt.Printf("\n=== Game %d: %s vs %s (%s) ===\n",
			gamesAnalyzed,
			white.Value, black.Value, result.Value)

		// Analyze each position
		positions := game.Positions()
		gameFound := 0

		for i, pos := range positions {
			fen := pos.String()
			// Normalize: take first 4 fields
			parts := strings.Fields(fen)
			if len(parts) >= 4 {
				fen = strings.Join(parts[:4], " ")
			}

			start := time.Now()
			eval, err := client.Lookup(ctx, fen)
			elapsed := time.Since(start)
			totalLookupTime += elapsed
			totalPositions++

			if err == nil {
				foundPositions++
				gameFound++
				// Show evaluation for key positions (every 10 moves)
				if i%10 == 0 || i == len(positions)-1 {
					fmt.Printf("  Move %2d: %s (%v)\n", i/2+1, eval.Score(), elapsed)
				}
			}
		}

		hitRate := float64(gameFound) / float64(len(positions)) * 100
		fmt.Printf("  Positions: %d/%d found (%.1f%%)\n", gameFound, len(positions), hitRate)
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Games analyzed: %d\n", gamesAnalyzed)
	fmt.Printf("Total positions: %d\n", totalPositions)
	fmt.Printf("Found: %d (%.1f%%)\n", foundPositions, float64(foundPositions)/float64(totalPositions)*100)
	fmt.Printf("Avg lookup: %v\n", totalLookupTime/time.Duration(totalPositions))
}
