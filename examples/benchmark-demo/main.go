// Package main demonstrates benchmarking sharding strategies with the stockpile library.
//
// This example shows how to:
// - Compare different sharding strategies (material vs FNV32)
// - Simulate game lookup patterns
// - Analyze cache locality and shard switching
// - Perform statistical comparisons
package main

import (
	"fmt"
	"strings"

	"github.com/notnil/chess"

	"github.com/discochess/stockpile/benchmark/analysis"
	"github.com/discochess/stockpile/benchmark/simulation"
	"github.com/discochess/stockpile/internal/shard"
	"github.com/discochess/stockpile/internal/shard/fnvshard"
	"github.com/discochess/stockpile/internal/shard/materialshard"
)

// Sample games for demonstration (abbreviated moves).
var sampleGames = []string{
	// Game 1: Sicilian Defense
	`1. e4 c5 2. Nf3 d6 3. d4 cxd4 4. Nxd4 Nf6 5. Nc3 a6 6. Be2 e5 7. Nb3 Be7 8. O-O O-O 9. Be3 Be6 10. Qd2 Nbd7`,
	// Game 2: Queen's Gambit Declined
	`1. d4 d5 2. c4 e6 3. Nc3 Nf6 4. Bg5 Be7 5. e3 O-O 6. Nf3 Nbd7 7. Rc1 c6 8. Bd3 dxc4 9. Bxc4 Nd5 10. Bxe7 Qxe7`,
	// Game 3: Italian Game
	`1. e4 e5 2. Nf3 Nc6 3. Bc4 Bc5 4. c3 Nf6 5. d4 exd4 6. cxd4 Bb4+ 7. Bd2 Bxd2+ 8. Nbxd2 d5 9. exd5 Nxd5 10. O-O O-O`,
	// Game 4: Caro-Kann Defense
	`1. e4 c6 2. d4 d5 3. Nc3 dxe4 4. Nxe4 Bf5 5. Ng3 Bg6 6. h4 h6 7. Nf3 Nd7 8. h5 Bh7 9. Bd3 Bxd3 10. Qxd3 e6`,
	// Game 5: French Defense
	`1. e4 e6 2. d4 d5 3. Nc3 Nf6 4. Bg5 Be7 5. e5 Nfd7 6. Bxe7 Qxe7 7. f4 O-O 8. Nf3 c5 9. Qd2 Nc6 10. O-O-O cxd4`,
}

func main() {
	fmt.Println("Stockpile Sharding Strategy Benchmark Demo")
	fmt.Println("==========================================")
	fmt.Println()

	// Extract FENs from sample games.
	games := extractGames(sampleGames)
	var totalPositions int
	for _, g := range games {
		totalPositions += len(g)
	}

	fmt.Printf("Games: %d\n", len(games))
	fmt.Printf("Total positions: %d\n", totalPositions)
	fmt.Println()

	// Create sharding strategies to compare.
	strategies := []shard.Strategy{
		materialshard.New(),
		fnvshard.New(),
	}

	// Run simulation with 32K shards (default).
	totalShards := 32768
	sim := simulation.NewSimulator(totalShards, strategies...)
	results := sim.SimulateGames(games)

	// Display results for each strategy.
	fmt.Println("Results per Strategy")
	fmt.Println("--------------------")
	fmt.Println()

	for _, s := range strategies {
		res := results[s.Name()]
		metrics := simulation.ComputeMetrics(res)

		fmt.Printf("%s:\n", s.Name())
		fmt.Printf("  Avg shard switches/game: %.2f\n", metrics.AvgSwitchesPerGame)
		fmt.Printf("  Median switches/game:    %.0f\n", metrics.MedianSwitchesPerGame)
		fmt.Printf("  P90 switches/game:       %.0f\n", metrics.P90SwitchesPerGame)
		fmt.Printf("  Unique shards used:      %d (%.2f%%)\n",
			metrics.UniqueShards,
			float64(metrics.UniqueShards)/float64(totalShards)*100)
		fmt.Printf("  Est. cache hit rate:     %.1f%% (100-shard cache)\n",
			res.CacheHitRate(100))
		fmt.Println()
	}

	// Statistical comparison.
	fmt.Println("Statistical Comparison")
	fmt.Println("----------------------")
	fmt.Println()

	comparison := analysis.CompareStrategies(
		results[strategies[0].Name()],
		results[strategies[1].Name()],
		10000, // 10K bootstrap iterations.
		0.95,  // 95% confidence interval.
	)

	fmt.Printf("Mann-Whitney U test:\n")
	fmt.Printf("  U statistic: %.2f\n", comparison.MannWhitney.U)
	fmt.Printf("  Z score:     %.2f\n", comparison.MannWhitney.Z)
	fmt.Printf("  P-value:     %.4f\n", comparison.MannWhitney.PValue)
	fmt.Printf("  Significant: %v (p < 0.05)\n", comparison.MannWhitney.Significant)
	fmt.Println()

	fmt.Printf("Effect size (Cohen's d):\n")
	fmt.Printf("  d = %.2f (%s)\n", comparison.EffectSize.CohensD, comparison.EffectSize.Interpretation)
	fmt.Println()

	fmt.Printf("Bootstrap 95%% CI for mean difference:\n")
	fmt.Printf("  [%.2f, %.2f]\n", comparison.BootstrapCI.LowerBound, comparison.BootstrapCI.UpperBound)
	fmt.Println()

	// Conclusion.
	fmt.Println("Conclusion")
	fmt.Println("----------")
	if comparison.WinnerConfident {
		fmt.Printf("%s shows statistically significant improvement over %s.\n",
			comparison.Winner,
			otherStrategy(comparison.Winner, strategies[0].Name(), strategies[1].Name()))
		fmt.Printf("Fewer shard switches = better cache locality = faster lookups.\n")
	} else {
		fmt.Println("No statistically significant difference between strategies.")
		fmt.Println("More data may be needed to detect a difference.")
	}
}

func extractGames(pgns []string) [][]string {
	var games [][]string

	for _, pgn := range pgns {
		fens := extractFENs(pgn)
		if len(fens) > 0 {
			games = append(games, fens)
		}
	}

	return games
}

func extractFENs(pgnMoves string) []string {
	// Parse as PGN game.
	reader := strings.NewReader("[Event \"Demo\"]\n\n" + pgnMoves)
	pgnFunc, err := chess.PGN(reader)
	if err != nil {
		return nil
	}

	game := chess.NewGame(pgnFunc)
	positions := game.Positions()

	fens := make([]string, 0, len(positions))
	for _, pos := range positions {
		// Normalize to 4-field FEN.
		parts := strings.Fields(pos.String())
		if len(parts) >= 4 {
			fen := strings.Join(parts[:4], " ")
			fens = append(fens, fen)
		}
	}

	return fens
}

func otherStrategy(winner, s1, s2 string) string {
	if winner == s1 {
		return s2
	}
	return s1
}
