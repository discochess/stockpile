// Package s3store implements an AWS S3 storage backend.
package s3store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/discochess/stockpile/internal/codec"
	"github.com/discochess/stockpile/internal/store"
)

// Compile-time check that Store implements store.Store.
var _ store.Store = (*Store)(nil)

// Store is an AWS S3 storage backend.
type Store struct {
	client *s3.Client
	bucket string
	prefix string
	codec  codec.Codec
}

// New creates a new S3 store.
// The bucket must already exist.
// The codec handles compression/decompression.
func New(ctx context.Context, bucketName string, c codec.Codec, opts ...Option) (*Store, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	s := &Store{
		client: s3.NewFromConfig(cfg),
		bucket: bucketName,
		codec:  c,
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// Option configures a Store.
type Option func(*Store) error

// WithPrefix sets a key prefix for all operations.
func WithPrefix(prefix string) Option {
	return func(s *Store) error {
		s.prefix = strings.TrimSuffix(prefix, "/")
		if s.prefix != "" {
			s.prefix += "/"
		}
		return nil
	}
}

// WithRegion sets the AWS region.
func WithRegion(region string) Option {
	return func(s *Store) error {
		cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
		if err != nil {
			return fmt.Errorf("loading AWS config with region: %w", err)
		}
		s.client = s3.NewFromConfig(cfg)
		return nil
	}
}

// WithEndpoint sets a custom endpoint (for S3-compatible services like MinIO).
func WithEndpoint(endpoint string) Option {
	return func(s *Store) error {
		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			return fmt.Errorf("loading AWS config for endpoint: %w", err)
		}
		s.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
		return nil
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

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.shardKey(shardID)),
	}

	result, err := s.client.GetObject(ctx, input)
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("reading shard: %w", err)
	}
	defer result.Body.Close()

	// Decompress using codec.
	decompressor, err := s.codec.Reader(result.Body)
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
	// S3 client doesn't need explicit closing.
	return nil
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
