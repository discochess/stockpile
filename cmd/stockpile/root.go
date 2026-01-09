package main

import (
	"github.com/spf13/cobra"
)

var (
	// Global flags.
	dataDir string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "stockpile",
	Short: "Fast lookups for 302M+ chess position evaluations",
	Long: `Stockpile is a CLI tool for managing and querying chess position
evaluations from the Lichess database.

It provides sub-5ms lookups with a warm cache for over 302 million
pre-computed Stockfish evaluations.

Examples:
  # Look up a position
  stockpile lookup "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -"

  # Build the database from Lichess source
  stockpile build --output ./data

  # Show statistics
  stockpile stats`,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&dataDir, "data-dir", "d", "./data", "directory containing evaluation data")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}
