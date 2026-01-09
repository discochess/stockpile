// Package fen provides FEN (Forsyth-Edwards Notation) parsing utilities.
package fen

import (
	"errors"
	"strings"
)

// ErrInvalidFEN indicates the FEN string is malformed.
var ErrInvalidFEN = errors.New("invalid FEN notation")

// Material represents the piece counts for both sides.
type Material struct {
	WhitePawns   int
	WhiteKnights int
	WhiteBishops int
	WhiteRooks   int
	WhiteQueens  int

	BlackPawns   int
	BlackKnights int
	BlackBishops int
	BlackRooks   int
	BlackQueens  int
}

// Normalize returns a normalized FEN string suitable for lookups.
// It extracts only the position, side to move, castling rights, and en passant square,
// ignoring the halfmove clock and fullmove number.
func Normalize(fen string) (string, error) {
	parts := strings.Fields(fen)
	if len(parts) < 4 {
		return "", ErrInvalidFEN
	}

	// Validate piece placement
	if !isValidPiecePlacement(parts[0]) {
		return "", ErrInvalidFEN
	}

	// Validate side to move
	if parts[1] != "w" && parts[1] != "b" {
		return "", ErrInvalidFEN
	}

	// Return normalized FEN (first 4 fields)
	return strings.Join(parts[:4], " "), nil
}

// ParseMaterial extracts material counts from a FEN string.
func ParseMaterial(fen string) (Material, error) {
	parts := strings.Fields(fen)
	if len(parts) == 0 {
		return Material{}, ErrInvalidFEN
	}

	var m Material
	for _, ch := range parts[0] {
		switch ch {
		case 'P':
			m.WhitePawns++
		case 'N':
			m.WhiteKnights++
		case 'B':
			m.WhiteBishops++
		case 'R':
			m.WhiteRooks++
		case 'Q':
			m.WhiteQueens++
		case 'p':
			m.BlackPawns++
		case 'n':
			m.BlackKnights++
		case 'b':
			m.BlackBishops++
		case 'r':
			m.BlackRooks++
		case 'q':
			m.BlackQueens++
		case 'K', 'k':
			// Kings are always present, don't count
		case '/', '1', '2', '3', '4', '5', '6', '7', '8':
			// Valid FEN characters, ignore
		default:
			return Material{}, ErrInvalidFEN
		}
	}

	return m, nil
}

// SideToMove returns "w" or "b" from a FEN string.
func SideToMove(fen string) (string, error) {
	parts := strings.Fields(fen)
	if len(parts) < 2 {
		return "", ErrInvalidFEN
	}
	if parts[1] != "w" && parts[1] != "b" {
		return "", ErrInvalidFEN
	}
	return parts[1], nil
}

// isValidPiecePlacement validates the piece placement part of a FEN.
func isValidPiecePlacement(placement string) bool {
	ranks := strings.Split(placement, "/")
	if len(ranks) != 8 {
		return false
	}

	for _, rank := range ranks {
		squares := 0
		for _, ch := range rank {
			switch {
			case ch >= '1' && ch <= '8':
				squares += int(ch - '0')
			case ch == 'P', ch == 'N', ch == 'B', ch == 'R', ch == 'Q', ch == 'K',
				ch == 'p', ch == 'n', ch == 'b', ch == 'r', ch == 'q', ch == 'k':
				squares++
			default:
				return false
			}
		}
		if squares != 8 {
			return false
		}
	}

	return true
}
