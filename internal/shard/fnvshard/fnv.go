// Package fnvshard implements FNV-1a hash-based sharding for chess positions.
//
// This provides uniform distribution across shards but no locality benefits.
// Used primarily as a baseline for benchmarking against material-based sharding.
package fnvshard

import (
	"github.com/discochess/stockpile/internal/fen"
	"github.com/discochess/stockpile/internal/shard"
)

// Strategy implements FNV-1a hash-based sharding.
type Strategy struct{}

// Ensure Strategy implements shard.Strategy.
var _ shard.Strategy = (*Strategy)(nil)

// New creates a new FNV-based sharding strategy.
func New() *Strategy {
	return &Strategy{}
}

// Name returns the strategy name.
func (s *Strategy) Name() string {
	return "fnv32"
}

// ShardID computes a shard ID using FNV-1a hash of the normalized FEN string.
func (s *Strategy) ShardID(fenStr string, totalShards int) int {
	// Normalize FEN to ensure consistent hashing for equivalent positions.
	normalized, err := fen.Normalize(fenStr)
	if err != nil {
		// Fall back to hashing the raw FEN for invalid inputs.
		normalized = fenStr
	}
	h := fnv1a32(normalized)
	return int(h % uint32(totalShards))
}

// fnv1a32 computes the FNV-1a 32-bit hash of a string.
func fnv1a32(s string) uint32 {
	var h uint32 = 2166136261 // FNV offset basis
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619 // FNV prime
	}
	return h
}
