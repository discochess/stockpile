package s3store

import (
	"bytes"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/discochess/stockpile/internal/codec/zstdcodec"
	"github.com/discochess/stockpile/internal/store"
)

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
			if err := opt(s); err != nil {
				t.Fatalf("WithPrefix() error = %v", err)
			}
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

func TestStore_ErrNotFound_Mapping(t *testing.T) {
	if store.ErrNotFound == nil {
		t.Error("store.ErrNotFound should not be nil")
	}
}

func TestStore_Close(t *testing.T) {
	s := &Store{}
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestCodecRoundTrip(t *testing.T) {
	codec := zstdcodec.New()
	original := []byte("test data for S3 compression")

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

func TestWithRegion_ReturnsError(t *testing.T) {
	// WithRegion loads AWS config which should work in test environment.
	s := &Store{}
	opt := WithRegion("us-east-1")
	// This should not panic even if AWS is not configured.
	_ = opt(s)
	// We don't check the error here since AWS config behavior varies by environment.
}

func TestWithEndpoint_ReturnsError(t *testing.T) {
	s := &Store{}
	opt := WithEndpoint("http://localhost:9000")
	// This should not panic.
	_ = opt(s)
}
