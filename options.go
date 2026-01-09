package stockpile

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/discochess/stockpile/internal/builder"
	"github.com/discochess/stockpile/internal/codec/zstdcodec"
	"github.com/discochess/stockpile/internal/shard"
	"github.com/discochess/stockpile/internal/shard/fnvshard"
	"github.com/discochess/stockpile/internal/shard/materialshard"
	"github.com/discochess/stockpile/internal/stats"
	"github.com/discochess/stockpile/internal/store"
	"github.com/discochess/stockpile/internal/store/diskstore"
)

// Option configures a Client.
type Option interface {
	apply(*options)
}

// options holds the client configuration.
type options struct {
	store         store.Store
	shardStrategy shard.Strategy
	totalShards   int
	stats         stats.Collector
	logger        *zap.Logger
}

// defaultOptions returns the default configuration.
func defaultOptions() options {
	return options{
		shardStrategy: materialshard.New(),
		totalShards:   32768, // 2^15 shards
		stats:         stats.NewNoop(),
		logger:        zap.NewNop(),
	}
}

// optionFunc wraps a function to implement Option.
type optionFunc func(*options)

// Compile-time check that optionFunc implements Option.
var _ Option = optionFunc(nil)

func (f optionFunc) apply(o *options) { f(o) }

// WithStore sets the storage backend to use.
func WithStore(s store.Store) Option {
	return optionFunc(func(o *options) {
		o.store = s
	})
}

// WithShardStrategy sets the sharding strategy to use.
// If not set, material-based sharding is used.
func WithShardStrategy(s shard.Strategy) Option {
	return optionFunc(func(o *options) {
		o.shardStrategy = s
	})
}

// WithTotalShards sets the total number of shards.
// Default is 32768 (2^15).
func WithTotalShards(n int) Option {
	return optionFunc(func(o *options) {
		o.totalShards = n
	})
}

// WithStats sets the stats collector.
// If not set, a no-op collector is used.
func WithStats(c stats.Collector) Option {
	return optionFunc(func(o *options) {
		o.stats = c
	})
}

// WithLogger sets the logger.
// If not set, a no-op logger is used.
func WithLogger(l *zap.Logger) Option {
	return optionFunc(func(o *options) {
		o.logger = l
	})
}

// WithDataDir configures the client from a data directory.
// It reads the manifest.json to auto-configure shard count and strategy,
// and creates a disk-based store with zstd compression.
// This is the recommended way to create a client for local data.
func WithDataDir(dir string) (Option, error) {
	manifest, err := builder.ReadManifest(dir)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	st, err := diskstore.New(dir, zstdcodec.New())
	if err != nil {
		return nil, fmt.Errorf("creating store: %w", err)
	}

	var strategy shard.Strategy
	switch manifest.Strategy {
	case "material":
		strategy = materialshard.New()
	case "fnv32":
		strategy = fnvshard.New()
	default:
		return nil, fmt.Errorf("unknown strategy in manifest: %s", manifest.Strategy)
	}

	return optionFunc(func(o *options) {
		o.store = st
		o.totalShards = manifest.TotalShards
		o.shardStrategy = strategy
	}), nil
}
