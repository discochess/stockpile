// Package stats provides a unified interface for collecting metrics.
package stats

// Metric names used throughout the library.
const (
	// Client metrics.
	MetricLookups      = "stockpile_lookups_total"
	MetricHits         = "stockpile_hits_total"
	MetricMisses       = "stockpile_misses_total"
	MetricShardFetches = "stockpile_shard_fetches_total"

	// Cache metrics.
	MetricCacheHits   = "stockpile_cache_hits_total"
	MetricCacheMisses = "stockpile_cache_misses_total"
	MetricCacheSize   = "stockpile_cache_size"
)

// Collector defines the interface for collecting metrics.
type Collector interface {
	// IncCounter increments a counter metric by delta.
	IncCounter(name string, delta int64)

	// SetGauge sets a gauge metric to value.
	SetGauge(name string, value int64)

	// ObserveHistogram records a value in a histogram metric.
	ObserveHistogram(name string, value float64)
}
