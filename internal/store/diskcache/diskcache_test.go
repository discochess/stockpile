package diskcache

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/discochess/stockpile/internal/codec/noopcodec"
	"github.com/discochess/stockpile/internal/store"
)

// countingStore wraps a store.Store and counts ReadShard calls.
type countingStore struct {
	store.Store
	calls atomic.Int64
}

func (s *countingStore) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	s.calls.Add(1)
	return s.Store.ReadShard(ctx, shardID)
}

// staticStore returns fixed data for any shard.
type staticStore struct {
	data []byte
}

func (s *staticStore) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	return s.data, nil
}

func (s *staticStore) Close() error { return nil }

func TestReadShard_CacheMiss(t *testing.T) {
	dir := t.TempDir()
	remote := &countingStore{Store: &staticStore{data: []byte("shard-data")}}

	s, err := New(remote, dir, noopcodec.New())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	// First read: cache miss, should call remote.
	data, err := s.ReadShard(context.Background(), 42)
	if err != nil {
		t.Fatalf("ReadShard() error = %v", err)
	}
	if string(data) != "shard-data" {
		t.Errorf("ReadShard() = %q, want %q", data, "shard-data")
	}
	if remote.calls.Load() != 1 {
		t.Errorf("remote calls = %d, want 1", remote.calls.Load())
	}

	// Verify file exists on disk.
	path := filepath.Join(dir, "shards", "00042")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("cached file not found: %v", err)
	}

	// Second read: cache hit, should NOT call remote.
	data, err = s.ReadShard(context.Background(), 42)
	if err != nil {
		t.Fatalf("ReadShard() second call error = %v", err)
	}
	if string(data) != "shard-data" {
		t.Errorf("ReadShard() second call = %q, want %q", data, "shard-data")
	}
	if remote.calls.Load() != 1 {
		t.Errorf("remote calls after cache hit = %d, want 1", remote.calls.Load())
	}
}

func TestReadShard_CacheHit(t *testing.T) {
	dir := t.TempDir()
	remote := &countingStore{Store: &staticStore{data: []byte("remote-data")}}

	s, err := New(remote, dir, noopcodec.New())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	// Pre-populate cache.
	path := filepath.Join(dir, "shards", "00007")
	if err := os.WriteFile(path, []byte("cached-data"), 0644); err != nil {
		t.Fatalf("pre-populating cache: %v", err)
	}

	data, err := s.ReadShard(context.Background(), 7)
	if err != nil {
		t.Fatalf("ReadShard() error = %v", err)
	}
	if string(data) != "cached-data" {
		t.Errorf("ReadShard() = %q, want %q", data, "cached-data")
	}
	if remote.calls.Load() != 0 {
		t.Errorf("remote calls = %d, want 0", remote.calls.Load())
	}
}

func TestReadShard_CorruptCache(t *testing.T) {
	dir := t.TempDir()
	remote := &countingStore{Store: &staticStore{data: []byte("good-data")}}

	// Use zstd codec so that garbage bytes fail decompression.
	// With noopcodec, any bytes are valid. Instead, we use a codec
	// that wraps Reader with a check. Simpler: use noopcodec but
	// test with a file that the codec will happily read.
	// Actually, with noopcodec, corrupt data is just different data.
	// Let's use zstdcodec to get a real decompression error.

	// Use a custom codec that fails on specific magic bytes.
	s, err := New(remote, dir, &failReaderCodec{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	// Pre-populate cache with data that will fail decompression.
	path := filepath.Join(dir, "shards", "00003.fail")
	if err := os.WriteFile(path, []byte("corrupt"), 0644); err != nil {
		t.Fatalf("pre-populating cache: %v", err)
	}

	// Should fall back to remote and re-cache.
	data, err := s.ReadShard(context.Background(), 3)
	if err != nil {
		t.Fatalf("ReadShard() error = %v", err)
	}
	if string(data) != "good-data" {
		t.Errorf("ReadShard() = %q, want %q", data, "good-data")
	}
	if remote.calls.Load() != 1 {
		t.Errorf("remote calls = %d, want 1", remote.calls.Load())
	}
}

func TestReadShard_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	remote := &staticStore{data: []byte("data")}

	s, err := New(remote, dir, noopcodec.New())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	if _, err := s.ReadShard(context.Background(), 1); err != nil {
		t.Fatalf("ReadShard() error = %v", err)
	}

	// Verify no temp files remain.
	tmpDir := filepath.Join(dir, ".tmp")
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("reading tmp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("temp files remaining: %d", len(entries))
	}
}

func TestReadShard_DiskWriteFailure(t *testing.T) {
	dir := t.TempDir()
	remote := &staticStore{data: []byte("remote-data")}

	s, err := New(remote, dir, noopcodec.New())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	// Make shards dir read-only so writes fail.
	shardsDir := filepath.Join(dir, "shards")
	if err := os.Chmod(shardsDir, 0555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// Also make tmp dir read-only.
	tmpDir := filepath.Join(dir, ".tmp")
	if err := os.Chmod(tmpDir, 0555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(shardsDir, 0755)
		os.Chmod(tmpDir, 0755)
	})

	// Should still return data from remote even though disk write fails.
	data, err := s.ReadShard(context.Background(), 5)
	if err != nil {
		t.Fatalf("ReadShard() error = %v", err)
	}
	if string(data) != "remote-data" {
		t.Errorf("ReadShard() = %q, want %q", data, "remote-data")
	}
}

func TestReadShard_Singleflight(t *testing.T) {
	dir := t.TempDir()

	// Use a slow remote that we can control.
	startCh := make(chan struct{})
	remote := &blockingStore{
		data:    []byte("data"),
		startCh: startCh,
	}

	s, err := New(remote, dir, noopcodec.New())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([][]byte, goroutines)
	errs := make([]error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = s.ReadShard(context.Background(), 99)
		}()
	}

	// Unblock all goroutines.
	close(startCh)
	wg.Wait()

	for i := range goroutines {
		if errs[i] != nil {
			t.Errorf("goroutine %d: error = %v", i, errs[i])
		}
		if string(results[i]) != "data" {
			t.Errorf("goroutine %d: data = %q, want %q", i, results[i], "data")
		}
	}

	// Remote should have been called exactly once due to singleflight.
	if remote.calls.Load() != 1 {
		t.Errorf("remote calls = %d, want 1", remote.calls.Load())
	}
}

func TestReadShard_RemoteNotFound(t *testing.T) {
	dir := t.TempDir()
	remote := &notFoundStore{}

	s, err := New(remote, dir, noopcodec.New())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	_, err = s.ReadShard(context.Background(), 1)
	if err != store.ErrNotFound {
		t.Errorf("ReadShard() error = %v, want store.ErrNotFound", err)
	}
}

func TestClose(t *testing.T) {
	dir := t.TempDir()
	closed := false
	remote := &closeTrackingStore{closed: &closed}

	s, err := New(remote, dir, noopcodec.New())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if !closed {
		t.Error("Close() did not close the remote store")
	}
}

// blockingStore blocks until startCh is closed, then returns data.
type blockingStore struct {
	data    []byte
	startCh chan struct{}
	calls   atomic.Int64
}

func (s *blockingStore) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	<-s.startCh
	s.calls.Add(1)
	return s.data, nil
}

func (s *blockingStore) Close() error { return nil }

// notFoundStore always returns store.ErrNotFound.
type notFoundStore struct{}

func (s *notFoundStore) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	return nil, store.ErrNotFound
}

func (s *notFoundStore) Close() error { return nil }

// closeTrackingStore tracks whether Close was called.
type closeTrackingStore struct {
	closed *bool
}

func (s *closeTrackingStore) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	return nil, store.ErrNotFound
}

func (s *closeTrackingStore) Close() error {
	*s.closed = true
	return nil
}

// failReaderCodec is a codec whose Reader always fails on Read.
type failReaderCodec struct{}

func (c *failReaderCodec) Reader(r io.Reader) (io.ReadCloser, error) {
	return &failReader{}, nil
}

func (c *failReaderCodec) Writer(w io.Writer) (io.WriteCloser, error) {
	return &passWriter{w}, nil
}

func (c *failReaderCodec) Extension() string { return "fail" }

type failReader struct{}

func (r *failReader) Read([]byte) (int, error) { return 0, os.ErrInvalid }
func (r *failReader) Close() error             { return nil }

type passWriter struct {
	w io.Writer
}

func (w *passWriter) Write(p []byte) (int, error) { return w.w.Write(p) }
func (w *passWriter) Close() error                { return nil }
