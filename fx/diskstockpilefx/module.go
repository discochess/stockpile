// Package diskstockpilefx provides an fx module for a disk-backed stockpile client.
package diskstockpilefx

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/discochess/stockpile"
	"github.com/discochess/stockpile/internal/codec/zstdcodec"
	"github.com/discochess/stockpile/internal/stats"
	"github.com/discochess/stockpile/internal/stats/logger"
	"github.com/discochess/stockpile/internal/store/cachedstore"
	"github.com/discochess/stockpile/internal/store/cachedstore/cachestrategy/lru"
	"github.com/discochess/stockpile/internal/store/cachedstore/memory"
	"github.com/discochess/stockpile/internal/store/diskstore"
)

// Config holds configuration for the disk-backed stockpile client.
type Config struct {
	// DataDir is the directory containing the stockpile data.
	DataDir string

	// CacheSize is the number of shards to cache in memory.
	// Default is 100.
	CacheSize int
}

// Module provides a disk-backed stockpile client.
// Requires a *zap.Logger to be provided.
var Module = fx.Module("diskstockpile",
	fx.Provide(
		newStatsCollector,
		newClient,
	),
)

func newStatsCollector(log *zap.Logger) stats.Collector {
	return logger.New(log.Named("stockpile.stats"))
}

// Params holds dependencies for creating the client.
type Params struct {
	fx.In

	Config    Config
	Logger    *zap.Logger
	Collector stats.Collector
	Lifecycle fx.Lifecycle
}

// Result holds the provided client.
type Result struct {
	fx.Out

	Client *stockpile.Client
}

func newClient(p Params) (Result, error) {
	cacheSize := p.Config.CacheSize
	if cacheSize <= 0 {
		cacheSize = 100
	}

	baseStore, err := diskstore.New(p.Config.DataDir, zstdcodec.New())
	if err != nil {
		return Result{}, err
	}

	lruStrategy, err := lru.New(cacheSize)
	if err != nil {
		return Result{}, err
	}

	st := cachedstore.New(baseStore, memory.New(lruStrategy, p.Collector))

	client, err := stockpile.New(
		stockpile.WithStore(st),
		stockpile.WithStats(p.Collector),
		stockpile.WithLogger(p.Logger.Named("stockpile")),
	)
	if err != nil {
		return Result{}, err
	}

	p.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})

	return Result{Client: client}, nil
}
