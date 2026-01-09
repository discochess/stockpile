// Package stockpile provides fast lookups into pre-computed chess position
// evaluations from the Lichess database.
//
// Example usage:
//
//	client, err := stockpile.New(
//	    stockpile.WithDataDir("/path/to/data"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	eval, err := client.Lookup(ctx, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Evaluation: %s\n", eval.Score())
package stockpile

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/discochess/stockpile/internal/search"
	"github.com/discochess/stockpile/internal/shard"
	"github.com/discochess/stockpile/internal/stats"
	"github.com/discochess/stockpile/internal/store"
)

// Sentinel errors for well-defined error conditions.
var (
	// ErrNotFound indicates the position was not found in the database.
	ErrNotFound = errors.New("stockpile: position not found")

	// ErrClosed indicates the client has been closed.
	ErrClosed = errors.New("stockpile: client closed")

	// ErrNoStore indicates no store was provided.
	ErrNoStore = errors.New("stockpile: no store provided")
)

// Client provides access to the Lichess evaluation database.
// A Client is safe for concurrent use by multiple goroutines.
type Client struct {
	store         store.Store
	shardStrategy shard.Strategy
	totalShards   int
	stats         stats.Collector
	logger        *zap.Logger
	closed        atomic.Bool
}

// New creates a new Client with the given options.
// If no options are provided, sensible defaults are used.
func New(opts ...Option) (*Client, error) {
	cfg := defaultOptions()
	for _, opt := range opts {
		opt.apply(&cfg)
	}

	c := &Client{
		store:         cfg.store,
		shardStrategy: cfg.shardStrategy,
		totalShards:   cfg.totalShards,
		stats:         cfg.stats,
		logger:        cfg.logger,
	}

	if c.store == nil {
		return nil, ErrNoStore
	}

	c.logger.Debug("client initialized",
		zap.Int("totalShards", c.totalShards),
		zap.String("shardStrategy", c.shardStrategy.Name()),
	)

	return c, nil
}

// Lookup returns the evaluation for a given FEN position.
// Returns ErrNotFound if the position is not in the database.
func (c *Client) Lookup(ctx context.Context, fen string) (*Eval, error) {
	if c.closed.Load() {
		return nil, ErrClosed
	}

	c.stats.IncCounter(stats.MetricLookups, 1)

	shardID := c.shardStrategy.ShardID(fen, c.totalShards)

	shardData, err := c.fetchShard(ctx, shardID)
	if err != nil {
		return nil, fmt.Errorf("fetching shard %d: %w", shardID, err)
	}

	eval, err := c.searchShard(shardData, fen)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.stats.IncCounter(stats.MetricMisses, 1)
		}
		return nil, err
	}

	c.stats.IncCounter(stats.MetricHits, 1)
	return eval, nil
}


// Close releases all resources associated with the client.
// After Close, the client should not be used.
func (c *Client) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return ErrClosed
	}

	if c.store != nil {
		if err := c.store.Close(); err != nil {
			return fmt.Errorf("closing store: %w", err)
		}
	}

	return nil
}

// ShardStrategy returns the sharding strategy used by this client.
func (c *Client) ShardStrategy() shard.Strategy {
	return c.shardStrategy
}

// Store returns the storage backend used by this client.
func (c *Client) Store() store.Store {
	return c.store
}

// fetchShard fetches a shard from storage.
func (c *Client) fetchShard(ctx context.Context, shardID int) ([]byte, error) {
	c.stats.IncCounter(stats.MetricShardFetches, 1)
	return c.store.ReadShard(ctx, shardID)
}

// searchShard searches for a position within shard data.
// The shard data is expected to be sorted JSONL (already decompressed by store).
func (c *Client) searchShard(data []byte, fenStr string) (*Eval, error) {
	record, err := search.Search(data, fenStr)
	if err != nil {
		if errors.Is(err, search.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Convert internal record to public Eval type.
	return recordToEval(record), nil
}

// recordToEval converts an internal search.EvalRecord to a public Eval.
func recordToEval(r *search.EvalRecord) *Eval {
	eval := &Eval{
		FEN: r.FEN,
	}

	// Use the first (best) evaluation if available.
	if len(r.Evals) > 0 {
		best := r.Evals[0]
		eval.Depth = best.Depth
		eval.Knodes = best.Knodes

		// Copy all PVs.
		eval.PVs = make([]PV, len(best.PVs))
		for i, pv := range best.PVs {
			eval.PVs[i] = PV{
				Centipawns: pv.CP,
				Mate:       pv.Mate,
				Line:       pv.Line,
			}
		}
	}

	return eval
}
