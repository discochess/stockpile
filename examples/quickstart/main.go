// Package main demonstrates basic stockpile usage.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/discochess/stockpile"
	"github.com/discochess/stockpile/internal/codec/zstdcodec"
	"github.com/discochess/stockpile/internal/store/cachedstore"
	"github.com/discochess/stockpile/internal/store/cachedstore/cachestrategy/lru"
	"github.com/discochess/stockpile/internal/store/cachedstore/memory"
	"github.com/discochess/stockpile/internal/store/diskstore"
)

func main() {
	// Get data directory from environment or use default.
	dataDir := os.Getenv("STOCKPILE_DATA")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Create the storage backend.
	baseStore, err := diskstore.New(dataDir, zstdcodec.New())
	if err != nil {
		log.Fatalf("Failed to open data directory: %v", err)
	}

	// Wrap with LRU cache (100 shards).
	lruStrategy, err := lru.New(100)
	if err != nil {
		log.Fatalf("Failed to create LRU strategy: %v", err)
	}
	st := cachedstore.New(baseStore, memory.New(lruStrategy, nil))

	// Create the stockpile client.
	client, err := stockpile.New(
		stockpile.WithStore(st),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Look up the starting position.
	ctx := context.Background()
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -"

	eval, err := client.Lookup(ctx, fen)
	if err != nil {
		if err == stockpile.ErrNotFound {
			fmt.Println("Position not found in database")
			return
		}
		log.Fatalf("Lookup failed: %v", err)
	}

	// Print the evaluation.
	fmt.Printf("Position: %s\n", eval.FEN)
	fmt.Printf("Score:    %s\n", eval.Score())
	fmt.Printf("Depth:    %d\n", eval.Depth)
	if pv := eval.BestPV(); pv != nil {
		fmt.Printf("PV:       %s\n", pv.Line)
	}
}
