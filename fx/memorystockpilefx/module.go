// Package memorystockpilefx provides an fx module for an in-memory stockpile client.
// Useful for testing.
package memorystockpilefx

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/discochess/stockpile"
	"github.com/discochess/stockpile/internal/stats"
	"github.com/discochess/stockpile/internal/stats/logger"
	"github.com/discochess/stockpile/internal/store/memstore"
)

// Module provides an in-memory stockpile client for testing.
// Requires a *zap.Logger to be provided.
var Module = fx.Module("memorystockpile",
	fx.Provide(
		newStatsCollector,
		newMemStore,
		newClient,
	),
)

func newStatsCollector(log *zap.Logger) stats.Collector {
	return logger.New(log.Named("stockpile.stats"))
}

func newMemStore() *memstore.Store {
	return memstore.New()
}

// Params holds dependencies for creating the client.
type Params struct {
	fx.In

	Logger    *zap.Logger
	Collector stats.Collector
	Store     *memstore.Store
	Lifecycle fx.Lifecycle
}

// Result holds the provided client and store.
type Result struct {
	fx.Out

	Client *stockpile.Client
	Store  *memstore.Store // Exposed for test setup
}

func newClient(p Params) (Result, error) {
	client, err := stockpile.New(
		stockpile.WithStore(p.Store),
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

	return Result{
		Client: client,
		Store:  p.Store,
	}, nil
}
