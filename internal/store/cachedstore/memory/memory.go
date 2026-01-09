// Package memory implements an in-memory cache backend.
package memory

import (
	"sync/atomic"

	"github.com/discochess/stockpile/internal/stats"
	"github.com/discochess/stockpile/internal/store/cachedstore"
	"github.com/discochess/stockpile/internal/store/cachedstore/cachestrategy"
)

// Compile-time check that Backend implements cachedstore.Backend.
var _ cachedstore.Backend = (*Backend)(nil)

// Backend is a thread-safe in-memory cache backend.
type Backend struct {
	strategy  cachestrategy.Strategy
	collector stats.Collector

	hits   atomic.Int64
	misses atomic.Int64
}

// New creates a new memory backend with the given eviction strategy.
// The collector is optional; if nil, a no-op collector is used.
func New(strategy cachestrategy.Strategy, collector stats.Collector) *Backend {
	if collector == nil {
		collector = stats.NewNoop()
	}
	return &Backend{
		strategy:  strategy,
		collector: collector,
	}
}

// Get retrieves shard data from the cache.
func (b *Backend) Get(shardID int) ([]byte, bool) {
	val, ok := b.strategy.Get(shardID)
	if ok {
		b.hits.Add(1)
		b.collector.IncCounter(stats.MetricCacheHits, 1)
		return val, true
	}
	b.misses.Add(1)
	b.collector.IncCounter(stats.MetricCacheMisses, 1)
	return nil, false
}

// Set stores shard data in the cache.
func (b *Backend) Set(shardID int, data []byte) {
	b.strategy.Add(shardID, data)
	b.collector.SetGauge(stats.MetricCacheSize, int64(b.strategy.Len()))
}

// Stats returns current cache statistics.
func (b *Backend) Stats() cachedstore.Stats {
	return cachedstore.Stats{
		Hits:   b.hits.Load(),
		Misses: b.misses.Load(),
		Size:   b.strategy.Len(),
	}
}

// Len returns the number of items in the cache.
func (b *Backend) Len() int {
	return b.strategy.Len()
}
