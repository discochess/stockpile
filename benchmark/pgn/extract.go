// Package pgn provides utilities for extracting FEN positions from PGN files.
package pgn

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/notnil/chess"
)

// ExtractFENs extracts all unique FEN positions from a PGN stream.
// It returns FENs in the order they appear across all games.
func ExtractFENs(r io.Reader) ([]string, error) {
	var fens []string
	seen := make(map[string]struct{})

	scanner := bufio.NewScanner(r)
	// Increase buffer size for long lines.
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var gameText strings.Builder
	inGame := false

	for scanner.Scan() {
		line := scanner.Text()

		// Detect game boundaries.
		if strings.HasPrefix(line, "[Event ") {
			if inGame && gameText.Len() > 0 {
				// Process previous game.
				gameFENs, err := extractFENsFromGame(gameText.String())
				if err == nil {
					for _, fen := range gameFENs {
						if _, ok := seen[fen]; !ok {
							seen[fen] = struct{}{}
							fens = append(fens, fen)
						}
					}
				}
				gameText.Reset()
			}
			inGame = true
		}

		if inGame {
			gameText.WriteString(line)
			gameText.WriteString("\n")
		}
	}

	// Process last game.
	if gameText.Len() > 0 {
		gameFENs, err := extractFENsFromGame(gameText.String())
		if err == nil {
			for _, fen := range gameFENs {
				if _, ok := seen[fen]; !ok {
					seen[fen] = struct{}{}
					fens = append(fens, fen)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading PGN: %w", err)
	}

	return fens, nil
}

// ExtractFENsFromGames extracts FENs from multiple PGN games in a reader.
// Unlike ExtractFENs, this preserves duplicates and game boundaries.
func ExtractFENsFromGames(r io.Reader) ([][]string, error) {
	var games [][]string

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var gameText strings.Builder
	inGame := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "[Event ") {
			if inGame && gameText.Len() > 0 {
				gameFENs, err := extractFENsFromGame(gameText.String())
				if err == nil && len(gameFENs) > 0 {
					games = append(games, gameFENs)
				}
				gameText.Reset()
			}
			inGame = true
		}

		if inGame {
			gameText.WriteString(line)
			gameText.WriteString("\n")
		}
	}

	// Process last game.
	if gameText.Len() > 0 {
		gameFENs, err := extractFENsFromGame(gameText.String())
		if err == nil && len(gameFENs) > 0 {
			games = append(games, gameFENs)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading PGN: %w", err)
	}

	return games, nil
}

func extractFENsFromGame(pgnText string) ([]string, error) {
	pgnFunc, err := chess.PGN(strings.NewReader(pgnText))
	if err != nil {
		return nil, err
	}

	game := chess.NewGame(pgnFunc)

	var fens []string
	positions := game.Positions()
	for _, pos := range positions {
		// Normalize FEN to 4 fields (no halfmove/fullmove counters).
		fen := pos.String()
		fen = normalizeFEN(fen)
		fens = append(fens, fen)
	}

	return fens, nil
}

// normalizeFEN normalizes a FEN to 4 fields (piece placement, side, castling, en passant).
func normalizeFEN(fen string) string {
	parts := strings.Fields(fen)
	if len(parts) < 4 {
		return fen
	}
	return strings.Join(parts[:4], " ")
}

// GameStats contains statistics about FEN extraction from games.
type GameStats struct {
	TotalGames      int
	TotalPositions  int
	UniquePositions int
	AvgMovesPerGame float64
}

// ExtractWithStats extracts FENs and returns statistics.
func ExtractWithStats(r io.Reader) ([]string, GameStats, error) {
	games, err := ExtractFENsFromGames(r)
	if err != nil {
		return nil, GameStats{}, err
	}

	seen := make(map[string]struct{})
	var fens []string
	var totalPositions int

	for _, gameFENs := range games {
		totalPositions += len(gameFENs)
		for _, fen := range gameFENs {
			if _, ok := seen[fen]; !ok {
				seen[fen] = struct{}{}
				fens = append(fens, fen)
			}
		}
	}

	var avgMoves float64
	if len(games) > 0 {
		avgMoves = float64(totalPositions) / float64(len(games))
	}

	stats := GameStats{
		TotalGames:      len(games),
		TotalPositions:  totalPositions,
		UniquePositions: len(fens),
		AvgMovesPerGame: avgMoves,
	}

	return fens, stats, nil
}
