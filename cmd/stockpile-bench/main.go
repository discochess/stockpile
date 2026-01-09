// Package main provides the stockpile-bench CLI tool for benchmarking
// sharding strategies with real game data.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"

	"github.com/discochess/stockpile/benchmark/analysis"
	"github.com/discochess/stockpile/benchmark/pgn"
	"github.com/discochess/stockpile/benchmark/reporting"
	"github.com/discochess/stockpile/benchmark/simulation"
	"github.com/discochess/stockpile/internal/shard"
	"github.com/discochess/stockpile/internal/shard/fnvshard"
	"github.com/discochess/stockpile/internal/shard/materialshard"
)

var (
	gamesFile     string
	strategyNames []string
	totalShards   int
	outputFormat  string
	outputFile    string
	verbose       bool
)

var rootCmd = &cobra.Command{
	Use:   "stockpile-bench",
	Short: "Benchmark sharding strategies for stockpile",
	Long: `stockpile-bench compares different sharding strategies using real game data.

It simulates lookup patterns from PGN games and measures shard switching
frequency to determine which strategy provides better cache locality.

Examples:
  # Run benchmark with default strategies
  stockpile-bench run --games games.pgn

  # Run benchmark with specific strategies
  stockpile-bench run --games games.pgn --strategies material,fnv32

  # Output as markdown report
  stockpile-bench run --games games.pgn --format markdown --output report.md`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the benchmark simulation",
	RunE:  runBenchmark,
}

func init() {
	runCmd.Flags().StringVarP(&gamesFile, "games", "g", "", "PGN file containing games (supports .zst)")
	runCmd.Flags().StringSliceVarP(&strategyNames, "strategies", "s", []string{"material", "fnv32"}, "strategies to compare")
	runCmd.Flags().IntVar(&totalShards, "shards", 32768, "total number of shards")
	runCmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "output format: text, markdown")
	runCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file (default: stdout)")
	runCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	runCmd.MarkFlagRequired("games")

	rootCmd.AddCommand(runCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	// Open games file.
	var reader io.Reader
	file, err := os.Open(gamesFile)
	if err != nil {
		return fmt.Errorf("opening games file: %w", err)
	}
	defer file.Close()

	// Handle zstd compression.
	if strings.HasSuffix(gamesFile, ".zst") {
		decoder, err := zstd.NewReader(file)
		if err != nil {
			return fmt.Errorf("creating zstd decoder: %w", err)
		}
		defer decoder.Close()
		reader = decoder
	} else {
		reader = file
	}

	// Extract FENs from games.
	if verbose {
		fmt.Fprintln(os.Stderr, "Extracting positions from games...")
	}

	games, err := pgn.ExtractFENsFromGames(reader)
	if err != nil {
		return fmt.Errorf("extracting FENs: %w", err)
	}

	if len(games) == 0 {
		return fmt.Errorf("no games found in %s", gamesFile)
	}

	var totalPositions int
	for _, g := range games {
		totalPositions += len(g)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Extracted %d positions from %d games\n", totalPositions, len(games))
	}

	// Create strategies.
	strategies := make([]shard.Strategy, 0, len(strategyNames))
	for _, name := range strategyNames {
		s, err := createStrategy(name)
		if err != nil {
			return err
		}
		strategies = append(strategies, s)
	}

	// Run simulation.
	if verbose {
		fmt.Fprintln(os.Stderr, "Running simulation...")
	}

	sim := simulation.NewSimulator(totalShards, strategies...)
	results := sim.SimulateGames(games)

	// Perform statistical comparison.
	var comparison *analysis.StrategyComparison
	if len(strategies) >= 2 {
		comparison = analysis.CompareStrategies(
			results[strategies[0].Name()],
			results[strategies[1].Name()],
			10000, // Bootstrap iterations.
			0.95,  // 95% confidence.
		)
	}

	// Output results.
	var output io.Writer = os.Stdout
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		output = f
	}

	switch outputFormat {
	case "markdown":
		return writeMarkdownReport(output, games, results, comparison)
	default:
		return writeTextReport(output, games, results, comparison)
	}
}

func createStrategy(name string) (shard.Strategy, error) {
	switch strings.ToLower(name) {
	case "material":
		return materialshard.New(), nil
	case "fnv32":
		return fnvshard.New(), nil
	default:
		return nil, fmt.Errorf("unknown strategy: %s", name)
	}
}

func writeTextReport(w io.Writer, games [][]string, results map[string]*simulation.AggregateResult, comp *analysis.StrategyComparison) error {
	var totalPositions int
	for _, g := range games {
		totalPositions += len(g)
	}

	fmt.Fprintf(w, "Stockpile Sharding Strategy Benchmark\n")
	fmt.Fprintf(w, "=====================================\n\n")
	fmt.Fprintf(w, "Games: %d\n", len(games))
	fmt.Fprintf(w, "Positions: %d\n", totalPositions)
	fmt.Fprintf(w, "Shards: %d\n\n", totalShards)

	fmt.Fprintf(w, "Results:\n")
	fmt.Fprintf(w, "--------\n\n")

	for name, res := range results {
		metrics := simulation.ComputeMetrics(res)
		fmt.Fprintf(w, "%s:\n", name)
		fmt.Fprintf(w, "  Avg switches/game: %.2f\n", metrics.AvgSwitchesPerGame)
		fmt.Fprintf(w, "  Median switches:   %.0f\n", metrics.MedianSwitchesPerGame)
		fmt.Fprintf(w, "  P90 switches:      %.0f\n", metrics.P90SwitchesPerGame)
		fmt.Fprintf(w, "  Unique shards:     %d\n", metrics.UniqueShards)
		fmt.Fprintf(w, "  Est. cache hit:    %.1f%%\n\n", res.CacheHitRate(100))
	}

	if comp != nil {
		fmt.Fprintf(w, "Statistical Analysis:\n")
		fmt.Fprintf(w, "---------------------\n\n")
		fmt.Fprintln(w, comp.Summary())
	}

	return nil
}

func writeMarkdownReport(w io.Writer, games [][]string, results map[string]*simulation.AggregateResult, comp *analysis.StrategyComparison) error {
	var totalPositions int
	for _, g := range games {
		totalPositions += len(g)
	}

	report := reporting.NewMarkdownReport(w)
	report.WriteHeader("Stockpile Sharding Strategy Benchmark")
	report.WriteMethodology(len(games), totalPositions)
	report.WriteSummaryTable(results)

	if comp != nil {
		report.WriteComparison(comp)
	}

	report.WriteFooter()
	return nil
}
