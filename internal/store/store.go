// Package store defines the storage backend interface for reading shard files.
package store

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a shard does not exist in the store.
var ErrNotFound = errors.New("store: shard not found")

// Store defines the interface for storage backends.
// Implementations handle path formats and storage details internally.
type Store interface {
	// ReadShard reads the content of the given shard.
	// The returned data may be compressed depending on the implementation.
	ReadShard(ctx context.Context, shardID int) ([]byte, error)

	// Close releases any resources held by the store.
	Close() error
}
