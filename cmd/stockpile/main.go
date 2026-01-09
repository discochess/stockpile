// Package main provides the stockpile CLI tool for managing and querying
// chess position evaluations from the Lichess database.
package main

import (
	"os"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
