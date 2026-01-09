// Package cachedstore provides a caching wrapper for Store implementations.
package cachedstore

// Backend defines the interface for cache storage backends.
// Implementations handle storage (memory, disk) and eviction strategy (LRU, MRU).
type Backend interface {
	// Get retrieves a cached shard. Returns nil, false if not found.
	Get(shardID int) ([]byte, bool)

	// Set stores a shard in the cache.
	Set(shardID int, data []byte)

	// Stats returns cache statistics.
	Stats() Stats
}

// Stats contains cache statistics.
type Stats struct {
	Hits   int64
	Misses int64
	Size   int // Current number of entries
}

// HitRate returns the cache hit rate as a percentage.
func (s Stats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}
