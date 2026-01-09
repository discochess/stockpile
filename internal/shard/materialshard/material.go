// Package materialshard implements material-based sharding for chess positions.
//
// Material-based sharding groups positions by piece counts, which provides
// better cache locality for game analysis since positions in the same game
// tend to have similar material configurations.
package materialshard

import (
	"github.com/discochess/stockpile/internal/fen"
	"github.com/discochess/stockpile/internal/shard"
)

// Strategy implements material-based sharding.
type Strategy struct{}

// Ensure Strategy implements shard.Strategy.
var _ shard.Strategy = (*Strategy)(nil)

// New creates a new material-based sharding strategy.
func New() *Strategy {
	return &Strategy{}
}

// Name returns the strategy name.
func (s *Strategy) Name() string {
	return "material"
}

// ShardID computes a shard ID based on the material configuration.
//
// The shard ID encodes:
// - Bits 0-2:   min(WhiteQueens, 7)
// - Bits 3-5:   min(BlackQueens, 7)
// - Bits 6-8:   min(WhiteRooks, 7)
// - Bits 9-11:  min(BlackRooks, 7)
// - Bits 12-14: min(WhiteBishops + WhiteKnights, 7)
// - Bits 15-17: min(BlackBishops + BlackKnights, 7)
// - Bit 18:     Side to move (0=white, 1=black)
//
// This produces up to 524,288 unique values, which is then reduced modulo totalShards.
func (s *Strategy) ShardID(fenStr string, totalShards int) int {
	mat, err := fen.ParseMaterial(fenStr)
	if err != nil {
		// Fall back to a hash-based approach for invalid FENs
		return hashFallback(fenStr, totalShards)
	}

	side, _ := fen.SideToMove(fenStr)

	var id uint32

	// Encode queens (3 bits each)
	id |= uint32(min(mat.WhiteQueens, 7)) << 0
	id |= uint32(min(mat.BlackQueens, 7)) << 3

	// Encode rooks (3 bits each)
	id |= uint32(min(mat.WhiteRooks, 7)) << 6
	id |= uint32(min(mat.BlackRooks, 7)) << 9

	// Encode minor pieces combined (3 bits each)
	whiteMinors := mat.WhiteBishops + mat.WhiteKnights
	blackMinors := mat.BlackBishops + mat.BlackKnights
	id |= uint32(min(whiteMinors, 7)) << 12
	id |= uint32(min(blackMinors, 7)) << 15

	// Encode side to move (1 bit)
	if side == "b" {
		id |= 1 << 18
	}

	return int(id % uint32(totalShards))
}

// hashFallback computes a simple hash for invalid or unparseable FENs.
func hashFallback(s string, totalShards int) int {
	var h uint32 = 2166136261 // FNV offset basis
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619 // FNV prime
	}
	return int(h % uint32(totalShards))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
