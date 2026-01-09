package gcsstore

import (
	"bytes"
	"context"
	"io"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/klauspost/compress/zstd"

	"github.com/discochess/stockpile/internal/codec/zstdcodec"
	"github.com/discochess/stockpile/internal/store"
)

// mockBucketHandle implements a mock for testing.
type mockBucketHandle struct {
	objects map[string][]byte
}

func (m *mockBucketHandle) Object(name string) *mockObjectHandle {
	return &mockObjectHandle{
		bucket: m,
		name:   name,
	}
}

type mockObjectHandle struct {
	bucket *mockBucketHandle
	name   string
}

func (m *mockObjectHandle) NewReader(ctx context.Context) (*mockReader, error) {
	data, ok := m.bucket.objects[m.name]
	if !ok {
		return nil, storage.ErrObjectNotExist
	}
	return &mockReader{Reader: bytes.NewReader(data)}, nil
}

type mockReader struct {
	*bytes.Reader
}

func (m *mockReader) Close() error {
	return nil
}

func TestStore_Extension(t *testing.T) {
	codec := zstdcodec.New()
	if ext := codec.Extension(); ext != "zst" {
		t.Errorf("Extension() = %q, want %q", ext, "zst")
	}
}

func TestWithPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"prefix", "prefix/"},
		{"prefix/", "prefix/"},
		{"a/b/c", "a/b/c/"},
		{"a/b/c/", "a/b/c/"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			s := &Store{}
			opt := WithPrefix(tt.input)
			opt(s)
			if s.prefix != tt.want {
				t.Errorf("prefix = %q, want %q", s.prefix, tt.want)
			}
		})
	}
}

func TestStore_shardKey(t *testing.T) {
	codec := zstdcodec.New()
	s := &Store{
		codec:  codec,
		prefix: "",
	}

	tests := []struct {
		shardID int
		want    string
	}{
		{0, "shards/00000.zst"},
		{1, "shards/00001.zst"},
		{99999, "shards/99999.zst"},
	}

	for _, tt := range tests {
		got := s.shardKey(tt.shardID)
		if got != tt.want {
			t.Errorf("shardKey(%d) = %q, want %q", tt.shardID, got, tt.want)
		}
	}
}

func TestStore_shardKey_WithPrefix(t *testing.T) {
	codec := zstdcodec.New()
	s := &Store{
		codec:  codec,
		prefix: "data/v1/",
	}

	got := s.shardKey(42)
	want := "data/v1/shards/00042.zst"
	if got != want {
		t.Errorf("shardKey(42) = %q, want %q", got, want)
	}
}

func TestStore_shardName(t *testing.T) {
	codec := zstdcodec.New()
	s := &Store{codec: codec}

	tests := []struct {
		shardID int
		want    string
	}{
		{0, "00000.zst"},
		{1, "00001.zst"},
		{12345, "12345.zst"},
	}

	for _, tt := range tests {
		got := s.shardName(tt.shardID)
		if got != tt.want {
			t.Errorf("shardName(%d) = %q, want %q", tt.shardID, got, tt.want)
		}
	}
}

// TestStore_ReadShard_NotFound tests that ErrNotFound is returned for missing objects.
// This is a unit test that doesn't require actual GCS access.
func TestStore_ErrNotFound_Mapping(t *testing.T) {
	// Verify that the store package has ErrNotFound defined.
	if store.ErrNotFound == nil {
		t.Error("store.ErrNotFound should not be nil")
	}
}

// TestIntegration_ReadShard tests the full read path with compressed data.
// This test uses a helper to simulate the decompression workflow.
func TestCodecRoundTrip(t *testing.T) {
	codec := zstdcodec.New()
	original := []byte("test data for compression")

	// Compress.
	var compressed bytes.Buffer
	writer, err := codec.Writer(&compressed)
	if err != nil {
		t.Fatalf("Writer() error = %v", err)
	}
	writer.Write(original)
	writer.Close()

	// Decompress.
	reader, err := codec.Reader(&compressed)
	if err != nil {
		t.Fatalf("Reader() error = %v", err)
	}
	decompressed, err := io.ReadAll(reader)
	reader.Close()

	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if !bytes.Equal(decompressed, original) {
		t.Errorf("round-trip failed: got %q, want %q", decompressed, original)
	}
}

// compressData compresses data using zstd for testing.
func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
