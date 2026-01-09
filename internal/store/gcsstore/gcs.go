// Package gcsstore implements a Google Cloud Storage backend.
package gcsstore

import (
	"context"
	"fmt"
	"io"
	"strings"

	"cloud.google.com/go/storage"

	"github.com/discochess/stockpile/internal/codec"
	"github.com/discochess/stockpile/internal/store"
)

// Compile-time check that Store implements store.Store.
var _ store.Store = (*Store)(nil)

// Store is a Google Cloud Storage backend.
type Store struct {
	client *storage.Client
	bucket *storage.BucketHandle
	prefix string
	codec  codec.Codec
}

// New creates a new GCS store.
// The bucket must already exist.
// The codec handles compression/decompression.
func New(ctx context.Context, bucketName string, c codec.Codec, opts ...Option) (*Store, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating GCS client: %w", err)
	}

	s := &Store{
		client: client,
		bucket: client.Bucket(bucketName),
		codec:  c,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Option configures a Store.
type Option func(*Store)

// WithPrefix sets a key prefix for all operations.
func WithPrefix(prefix string) Option {
	return func(s *Store) {
		s.prefix = strings.TrimSuffix(prefix, "/")
		if s.prefix != "" {
			s.prefix += "/"
		}
	}
}

// ReadShard reads and decompresses the content of the given shard.
func (s *Store) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	// Check for cancellation before starting.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	obj := s.bucket.Object(s.shardKey(shardID))

	reader, err := obj.NewReader(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("creating reader: %w", err)
	}
	defer reader.Close()

	// Decompress using codec.
	decompressor, err := s.codec.Reader(reader)
	if err != nil {
		return nil, fmt.Errorf("creating decompressor: %w", err)
	}
	defer decompressor.Close()

	data, err := io.ReadAll(decompressor)
	if err != nil {
		return nil, fmt.Errorf("decompressing shard: %w", err)
	}

	return data, nil
}

// Close releases resources.
func (s *Store) Close() error {
	return s.client.Close()
}

// shardKey returns the full object key for a shard.
func (s *Store) shardKey(shardID int) string {
	return s.prefix + "shards/" + s.shardName(shardID)
}

// shardName returns the filename for a shard ID.
func (s *Store) shardName(shardID int) string {
	name := fmt.Sprintf("%05d", shardID)
	if ext := s.codec.Extension(); ext != "" {
		name += "." + ext
	}
	return name
}
