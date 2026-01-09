// Package memstore provides an in-memory store implementation for testing.
package memstore

import (
	"context"
	"sync"

	"github.com/discochess/stockpile/internal/store"
)

// Compile-time check that Store implements store.Store.
var _ store.Store = (*Store)(nil)

// Store is an in-memory store for testing.
type Store struct {
	mu     sync.RWMutex
	shards map[int][]byte
}

// New creates a new in-memory store.
func New() *Store {
	return &Store{
		shards: make(map[int][]byte),
	}
}

// SetShard sets the data for a shard (for test setup).
// The data is copied to prevent caller mutations from affecting the store.
func (s *Store) SetShard(shardID int, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]byte, len(data))
	copy(copied, data)
	s.shards[shardID] = copied
}

// ReadShard reads a shard from memory.
func (s *Store) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.shards[shardID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return data, nil
}

// Close is a no-op for the memory store.
func (s *Store) Close() error {
	return nil
}
