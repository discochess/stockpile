// Package shard defines the sharding strategy interface for distributing
// chess positions across multiple shard files.
package shard

// Strategy defines a sharding algorithm that maps FEN positions to shard IDs.
type Strategy interface {
	// Name returns a human-readable name for this strategy.
	Name() string

	// ShardID computes the shard ID for a given FEN position.
	// The returned value is in the range [0, totalShards).
	//
	// Implementations should handle FEN normalization internally to ensure
	// consistent shard IDs for equivalent positions (e.g., positions that
	// differ only in halfmove/fullmove counters should map to the same shard).
	ShardID(fen string, totalShards int) int
}
