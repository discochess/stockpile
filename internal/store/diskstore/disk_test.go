package diskstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/discochess/stockpile/internal/codec/noopcodec"
	"github.com/discochess/stockpile/internal/store"
)

func TestStore_ReadShard(t *testing.T) {
	dir := t.TempDir()
	codec := noopcodec.New() // Use noop codec for simple testing.

	// Create shard file manually.
	shardsDir := filepath.Join(dir, "shards")
	if err := os.MkdirAll(shardsDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	data := []byte("shard data")
	shardPath := filepath.Join(shardsDir, "00001") // No extension with noop codec.
	if err := os.WriteFile(shardPath, data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	s, err := New(dir, codec)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Read shard.
	got, err := s.ReadShard(ctx, 1)
	if err != nil {
		t.Fatalf("ReadShard() error = %v", err)
	}

	if string(got) != string(data) {
		t.Errorf("ReadShard() = %q, want %q", got, data)
	}
}

func TestStore_ReadShardNotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, noopcodec.New())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	_, err = s.ReadShard(ctx, 99999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ReadShard() error = %v, want ErrNotFound", err)
	}
}

func TestNew_InvalidPath(t *testing.T) {
	_, err := New("/nonexistent/path", noopcodec.New())
	if err == nil {
		t.Error("New() with invalid path should return error")
	}
}

func TestNew_NotDirectory(t *testing.T) {
	// Create a file, not a directory.
	f, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	_, err = New(f.Name(), noopcodec.New())
	if err == nil {
		t.Error("New() with file (not directory) should return error")
	}
}
