// Package diskstore implements a disk-based filesystem storage backend.
package diskstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/discochess/stockpile/internal/codec"
	"github.com/discochess/stockpile/internal/store"
)

// Compile-time check that Store implements store.Store.
var _ store.Store = (*Store)(nil)

// Store is a disk-based filesystem storage backend.
type Store struct {
	root  string
	codec codec.Codec
}

// New creates a new disk store rooted at the given directory.
// The directory must exist. The codec handles compression/decompression.
func New(root string, codec codec.Codec) (*Store, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat root directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	return &Store{
		root:  root,
		codec: codec,
	}, nil
}

// ReadShard reads and decompresses the content of the given shard.
func (s *Store) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	// Check for cancellation before starting I/O.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	path := s.shardPath(shardID)

	compressed, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("reading shard: %w", err)
	}

	// Decompress using codec.
	reader, err := s.codec.Reader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("creating decompressor: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("decompressing shard: %w", err)
	}

	return data, nil
}

// Close releases any resources held by the store.
func (s *Store) Close() error {
	return nil
}

// shardPath returns the filesystem path for a shard.
func (s *Store) shardPath(shardID int) string {
	return filepath.Join(s.root, "shards", s.shardName(shardID))
}

// shardName returns the filename for a shard ID.
func (s *Store) shardName(shardID int) string {
	name := fmt.Sprintf("%05d", shardID)
	if ext := s.codec.Extension(); ext != "" {
		name += "." + ext
	}
	return name
}
