package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// GCSUploader uploads build output to Google Cloud Storage.
type GCSUploader struct {
	client *storage.Client
	bucket *storage.BucketHandle
	prefix string
}

// NewGCSUploader creates a new GCS uploader.
// gcsPath should be in the format "gs://bucket/prefix".
func NewGCSUploader(ctx context.Context, gcsPath string) (*GCSUploader, error) {
	bucket, prefix, err := parseGCSPath(gcsPath)
	if err != nil {
		return nil, err
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating GCS client: %w", err)
	}

	return &GCSUploader{
		client: client,
		bucket: client.Bucket(bucket),
		prefix: prefix,
	}, nil
}

// parseGCSPath parses "gs://bucket/prefix" into bucket and prefix.
func parseGCSPath(gcsPath string) (bucket, prefix string, err error) {
	if !strings.HasPrefix(gcsPath, "gs://") {
		return "", "", fmt.Errorf("invalid GCS path: must start with gs://")
	}

	path := strings.TrimPrefix(gcsPath, "gs://")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid GCS path: missing bucket name")
	}

	bucket = parts[0]
	if len(parts) > 1 {
		prefix = strings.TrimSuffix(parts[1], "/")
		if prefix != "" {
			prefix += "/"
		}
	}

	return bucket, prefix, nil
}

// Upload uploads the built shards and manifest from localDir to GCS.
// It uploads new shards first (overwriting), then cleans up stale shards.
// This "upload first, cleanup after" approach minimizes downtime.
func (u *GCSUploader) Upload(ctx context.Context, localDir string, progress ProgressFunc) error {
	shardsDir := filepath.Join(localDir, "shards")

	// Read local shards.
	entries, err := os.ReadDir(shardsDir)
	if err != nil {
		return fmt.Errorf("reading shards directory: %w", err)
	}

	// Track uploaded shard names for cleanup.
	uploadedShards := make(map[string]bool)

	// Upload new shards (overwrites existing).
	var uploaded int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		localPath := filepath.Join(shardsDir, entry.Name())
		gcsKey := u.prefix + "shards/" + entry.Name()

		if err := u.uploadFile(ctx, localPath, gcsKey); err != nil {
			return fmt.Errorf("uploading %s: %w", entry.Name(), err)
		}

		uploadedShards[entry.Name()] = true
		uploaded++
		if progress != nil && uploaded%100 == 0 {
			progress(Progress{
				Phase:         "upload",
				ShardsCreated: uploaded,
				ShardsTotal:   len(entries),
			})
		}
	}

	// Upload manifest.
	manifestPath := filepath.Join(localDir, manifestFilename)
	if _, err := os.Stat(manifestPath); err == nil {
		gcsKey := u.prefix + manifestFilename
		if err := u.uploadFile(ctx, manifestPath, gcsKey); err != nil {
			return fmt.Errorf("uploading manifest: %w", err)
		}
	}

	// Clean up stale shards (exist in GCS but not in new build).
	if err := u.cleanStaleShards(ctx, uploadedShards); err != nil {
		// Log but don't fail - stale shards are harmless.
		fmt.Printf("[Upload] Warning: failed to clean stale shards: %v\n", err)
	}

	if progress != nil {
		progress(Progress{
			Phase:         "upload",
			ShardsCreated: uploaded,
			ShardsTotal:   len(entries),
		})
	}

	return nil
}

// cleanStaleShards deletes shard files in GCS that aren't in the new build.
func (u *GCSUploader) cleanStaleShards(ctx context.Context, currentShards map[string]bool) error {
	prefix := u.prefix + "shards/"
	it := u.bucket.Objects(ctx, &storage.Query{Prefix: prefix})

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("listing objects: %w", err)
		}

		// Extract shard filename from full path.
		shardName := strings.TrimPrefix(attrs.Name, prefix)
		if currentShards[shardName] {
			continue // Keep this shard.
		}

		// Delete stale shard.
		if err := u.bucket.Object(attrs.Name).Delete(ctx); err != nil {
			return fmt.Errorf("deleting stale shard %s: %w", attrs.Name, err)
		}
	}

	return nil
}

// uploadFile uploads a single file to GCS.
func (u *GCSUploader) uploadFile(ctx context.Context, localPath, gcsKey string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	obj := u.bucket.Object(gcsKey)
	writer := obj.NewWriter(ctx)

	if _, err := io.Copy(writer, file); err != nil {
		writer.Close()
		return err
	}

	return writer.Close()
}

// UploadManifest uploads just the manifest to GCS (for updates without full rebuild).
func (u *GCSUploader) UploadManifest(ctx context.Context, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	obj := u.bucket.Object(u.prefix + manifestFilename)
	writer := obj.NewWriter(ctx)

	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return err
	}

	return writer.Close()
}

// Close releases resources.
func (u *GCSUploader) Close() error {
	return u.client.Close()
}
