package cachedstore

import (
	"context"

	"github.com/discochess/stockpile/internal/store"
)

// Compile-time check that Store implements store.Store.
var _ store.Store = (*Store)(nil)

// Store wraps another Store with caching.
type Store struct {
	underlying store.Store
	backend    Backend
}

// New creates a new cached store wrapping the given store.
func New(underlying store.Store, backend Backend) *Store {
	return &Store{
		underlying: underlying,
		backend:    backend,
	}
}

// ReadShard reads a shard, checking the cache first.
func (s *Store) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	// Check cache first.
	if data, ok := s.backend.Get(shardID); ok {
		return data, nil
	}

	// Cache miss - read from underlying store.
	data, err := s.underlying.ReadShard(ctx, shardID)
	if err != nil {
		return nil, err
	}

	// Cache the result.
	s.backend.Set(shardID, data)

	return data, nil
}

// Close closes the underlying store.
func (s *Store) Close() error {
	return s.underlying.Close()
}

// Stats returns cache statistics.
func (s *Store) Stats() Stats {
	return s.backend.Stats()
}
