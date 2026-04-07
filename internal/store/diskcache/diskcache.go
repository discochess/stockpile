// Package diskcache implements a disk-based cache that wraps a remote store.
// On cache miss, shards are downloaded from the remote store and cached
// on disk as compressed files. Writes are atomic — other processes will
// never see partial files.
package diskcache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/discochess/stockpile/internal/codec"
	"github.com/discochess/stockpile/internal/stats"
	"github.com/discochess/stockpile/internal/store"
)

// Compile-time check that Store implements store.Store.
var _ store.Store = (*Store)(nil)

// Store is a disk-based cache that wraps a remote store.
// On cache hit, the shard is read from disk. On cache miss,
// the shard is fetched from the remote store, cached on disk
// atomically, and returned.
type Store struct {
	remote    store.Store
	codec     codec.Codec
	shardsDir string
	tmpDir    string

	// Dedup concurrent cache misses for the same shard.
	group singleflight.Group

	collector stats.Collector
	logger    *zap.Logger
}

// Option configures a Store.
type Option func(*Store)

// WithStats sets the stats collector.
func WithStats(c stats.Collector) Option {
	return func(s *Store) {
		s.collector = c
	}
}

// WithLogger sets the logger.
func WithLogger(l *zap.Logger) Option {
	return func(s *Store) {
		s.logger = l
	}
}

// New creates a new disk cache store.
// The cacheDir is used to store cached shard files; it will be created
// if it does not exist. The codec handles compression/decompression for
// on-disk storage. The remote store is used as a fallback on cache miss.
func New(remote store.Store, cacheDir string, codec codec.Codec, opts ...Option) (*Store, error) {
	s := &Store{
		remote:    remote,
		codec:     codec,
		shardsDir: filepath.Join(cacheDir, "shards"),
		tmpDir:    filepath.Join(cacheDir, ".tmp"),
		collector: stats.NewNoop(),
		logger:    zap.NewNop(),
	}

	for _, opt := range opts {
		opt(s)
	}

	// Ensure directories exist.
	if err := os.MkdirAll(s.shardsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating shards directory: %w", err)
	}
	if err := os.MkdirAll(s.tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("creating temp directory: %w", err)
	}

	return s, nil
}

// ReadShard reads a shard, checking the disk cache first.
// On cache miss, the shard is fetched from the remote store and
// cached on disk atomically.
func (s *Store) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	// Check for cancellation before starting.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Try reading from disk cache.
	data, err := s.readFromDisk(shardID)
	if err == nil {
		s.collector.IncCounter(stats.MetricDiskCacheHits, 1)
		return data, nil
	}

	// Cache miss (or corrupt file). Fetch from remote.
	s.collector.IncCounter(stats.MetricDiskCacheMisses, 1)

	if !os.IsNotExist(err) {
		// File exists but is corrupt — log and re-fetch.
		s.logger.Warn("corrupt cached shard, re-fetching from remote",
			zap.Int("shardID", shardID),
			zap.Error(err),
		)
	}

	// Use singleflight to deduplicate concurrent misses for the same shard.
	key := strconv.Itoa(shardID)
	result, err, _ := s.group.Do(key, func() (interface{}, error) {
		decompressed, err := s.remote.ReadShard(ctx, shardID)
		if err != nil {
			return nil, err
		}

		// Cache on disk (best-effort; don't fail the read if caching fails).
		if writeErr := s.writeShard(shardID, decompressed); writeErr != nil {
			s.logger.Warn("failed to cache shard on disk",
				zap.Int("shardID", shardID),
				zap.Error(writeErr),
			)
		}

		return decompressed, nil
	})
	if err != nil {
		return nil, err
	}

	return result.([]byte), nil
}

// Close releases resources.
func (s *Store) Close() error {
	return s.remote.Close()
}

// readFromDisk reads a compressed shard file from the cache and decompresses it.
// Returns os.ErrNotExist if the file doesn't exist.
func (s *Store) readFromDisk(shardID int) ([]byte, error) {
	compressed, err := os.ReadFile(s.shardPath(shardID))
	if err != nil {
		return nil, err
	}

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

// writeShard compresses decompressed shard data and writes it to disk atomically.
// It writes to a temp file first, then renames to the final path.
func (s *Store) writeShard(shardID int, decompressed []byte) error {
	// Compress the data.
	var buf bytes.Buffer
	writer, err := s.codec.Writer(&buf)
	if err != nil {
		return fmt.Errorf("creating compressor: %w", err)
	}
	if _, err := writer.Write(decompressed); err != nil {
		writer.Close()
		return fmt.Errorf("compressing shard: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing compressor: %w", err)
	}

	// Write to temp file.
	tmpFile, err := os.CreateTemp(s.tmpDir, fmt.Sprintf("shard_%05d_*.tmp", shardID))
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Atomic rename to final path.
	finalPath := s.shardPath(shardID)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// shardPath returns the filesystem path for a cached shard.
func (s *Store) shardPath(shardID int) string {
	return filepath.Join(s.shardsDir, s.shardName(shardID))
}

// shardName returns the filename for a shard ID.
func (s *Store) shardName(shardID int) string {
	name := fmt.Sprintf("%05d", shardID)
	if ext := s.codec.Extension(); ext != "" {
		name += "." + ext
	}
	return name
}
