// Package main demonstrates analyzing a chess game with stockpile.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/notnil/chess"

	"github.com/discochess/stockpile"
	"github.com/discochess/stockpile/internal/codec/zstdcodec"
	"github.com/discochess/stockpile/internal/store/cachedstore"
	"github.com/discochess/stockpile/internal/store/cachedstore/cachestrategy/lru"
	"github.com/discochess/stockpile/internal/store/cachedstore/memory"
	"github.com/discochess/stockpile/internal/store/diskstore"
)

// Sample game: Kasparov vs Topalov, Wijk aan Zee 1999 (Kasparov's Immortal)
const samplePGN = `[Event "Hoogovens"]
[Site "Wijk aan Zee NED"]
[Date "1999.01.20"]
[Round "4"]
[White "Kasparov, Garry"]
[Black "Topalov, Veselin"]
[Result "1-0"]

1. e4 d6 2. d4 Nf6 3. Nc3 g6 4. Be3 Bg7 5. Qd2 c6 6. f3 b5 7. Nge2 Nbd7
8. Bh6 Bxh6 9. Qxh6 Bb7 10. a3 e5 11. O-O-O Qe7 12. Kb1 a6 13. Nc1 O-O-O
14. Nb3 exd4 15. Rxd4 c5 16. Rd1 Nb6 17. g3 Kb8 18. Na5 Ba8 19. Bh3 d5
20. Qf4+ Ka7 21. Rhe1 d4 22. Nd5 Nbxd5 23. exd5 Qd6 24. Rxd4 cxd4 25. Re7+ Kb6
26. Qxd4+ Kxa5 27. b4+ Ka4 28. Qc3 Qxd5 29. Ra7 Bb7 30. Rxb7 Qc4 31. Qxf6 Kxa3
32. Qxa6+ Kxb4 33. c3+ Kxc3 34. Qa1+ Kd2 35. Qb2+ Kd1 36. Bf1 Rd2 37. Rd7 Rxd7
38. Bxc4 bxc4 39. Qxh8 Rd3 40. Qa8 c3 41. Qa4+ Ke1 42. f4 f5 43. Kc1 Rd2
44. Qa7 1-0`

func main() {
	// Get data directory from environment or use default.
	dataDir := os.Getenv("STOCKPILE_DATA")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Create storage with caching.
	baseStore, err := diskstore.New(dataDir, zstdcodec.New())
	if err != nil {
		log.Fatalf("Failed to open data directory: %v", err)
	}

	lruStrategy, err := lru.New(100)
	if err != nil {
		log.Fatalf("Failed to create LRU strategy: %v", err)
	}
	st := cachedstore.New(baseStore, memory.New(lruStrategy, nil))

	client, err := stockpile.New(
		stockpile.WithStore(st),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Parse the game.
	pgnFunc, err := chess.PGN(strings.NewReader(samplePGN))
	if err != nil {
		log.Fatalf("Failed to parse PGN: %v", err)
	}

	game := chess.NewGame(pgnFunc)
	positions := game.Positions()
	moves := game.Moves()

	fmt.Println("Kasparov vs Topalov, Wijk aan Zee 1999")
	fmt.Println("======================================")
	fmt.Println()

	ctx := context.Background()
	start := time.Now()
	var found, notFound int
	var totalEval int

	// Analyze each position.
	fmt.Printf("%-4s %-8s %-10s %s\n", "Move", "Eval", "Depth", "Best Move")
	fmt.Println(strings.Repeat("-", 50))

	for i, pos := range positions {
		// Normalize FEN (4 fields).
		fenParts := strings.Fields(pos.String())
		if len(fenParts) > 4 {
			fenParts = fenParts[:4]
		}
		fen := strings.Join(fenParts, " ")

		eval, err := client.Lookup(ctx, fen)

		moveStr := ""
		if i > 0 && i-1 < len(moves) {
			moveNum := (i + 1) / 2
			if i%2 == 1 {
				moveStr = fmt.Sprintf("%d.", moveNum)
			} else {
				moveStr = fmt.Sprintf("%d...", moveNum)
			}
			moveStr += moves[i-1].String()
		} else {
			moveStr = "Start"
		}

		if err == nil {
			found++
			evalStr := eval.Score()
			bestMove := ""
			if pv := eval.BestPV(); pv != nil {
				parts := strings.Fields(pv.Line)
				if len(parts) > 0 {
					bestMove = parts[0]
				}
				if pv.Centipawns != nil {
					totalEval += abs(*pv.Centipawns)
				}
			}
			fmt.Printf("%-4s %-8s %-10d %s\n", moveStr, evalStr, eval.Depth, bestMove)
		} else if err == stockpile.ErrNotFound {
			notFound++
			fmt.Printf("%-4s %-8s %-10s %s\n", moveStr, "N/A", "-", "(not in DB)")
		} else {
			log.Printf("Lookup error for %s: %v", fen, err)
		}
	}

	elapsed := time.Since(start)

	// Summary.
	fmt.Println()
	fmt.Println("Summary")
	fmt.Println("-------")
	fmt.Printf("Positions analyzed: %d\n", len(positions))
	fmt.Printf("Found in database:  %d (%.1f%%)\n", found, float64(found)/float64(len(positions))*100)
	fmt.Printf("Not found:          %d\n", notFound)
	fmt.Printf("Total time:         %s\n", elapsed)
	fmt.Printf("Avg per position:   %s\n", elapsed/time.Duration(len(positions)))

	if found > 0 {
		fmt.Printf("Avg |eval|:         %.1f cp\n", float64(totalEval)/float64(found))
	}

	// Cache stats.
	cacheStats := st.Stats()
	fmt.Printf("Cache hit rate:     %.1f%%\n", cacheStats.HitRate())
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
