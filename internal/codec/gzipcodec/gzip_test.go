package gzipcodec

import (
	"bytes"
	"io"
	"testing"
)

func TestCodec_Extension(t *testing.T) {
	c := New()
	if got := c.Extension(); got != "gz" {
		t.Errorf("Extension() = %q, want %q", got, "gz")
	}
}

func TestCodec_RoundTrip(t *testing.T) {
	c := New()
	original := []byte("Hello, World! This is test data for gzip compression.")

	// Compress.
	var compressed bytes.Buffer
	writer, err := c.Writer(&compressed)
	if err != nil {
		t.Fatalf("Writer() error = %v", err)
	}
	if _, err := writer.Write(original); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Decompress.
	reader, err := c.Reader(&compressed)
	if err != nil {
		t.Fatalf("Reader() error = %v", err)
	}
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if !bytes.Equal(decompressed, original) {
		t.Errorf("Round-trip failed: got %q, want %q", decompressed, original)
	}
}

func TestCodec_RoundTrip_LargeData(t *testing.T) {
	c := New()
	original := bytes.Repeat([]byte("ABCDEFGHIJ"), 10000) // 100KB of repetitive data

	var compressed bytes.Buffer
	writer, err := c.Writer(&compressed)
	if err != nil {
		t.Fatalf("Writer() error = %v", err)
	}
	if _, err := writer.Write(original); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify compression ratio for repetitive data.
	if compressed.Len() >= len(original) {
		t.Errorf("Expected compression, got %d bytes from %d bytes", compressed.Len(), len(original))
	}

	reader, err := c.Reader(&compressed)
	if err != nil {
		t.Fatalf("Reader() error = %v", err)
	}
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	reader.Close()

	if !bytes.Equal(decompressed, original) {
		t.Error("Round-trip failed for large data")
	}
}

func TestCodec_RoundTrip_EmptyData(t *testing.T) {
	c := New()
	original := []byte{}

	var compressed bytes.Buffer
	writer, err := c.Writer(&compressed)
	if err != nil {
		t.Fatalf("Writer() error = %v", err)
	}
	if _, err := writer.Write(original); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reader, err := c.Reader(&compressed)
	if err != nil {
		t.Fatalf("Reader() error = %v", err)
	}
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	reader.Close()

	if !bytes.Equal(decompressed, original) {
		t.Errorf("Round-trip failed for empty data: got %q", decompressed)
	}
}

func TestCodec_Reader_InvalidData(t *testing.T) {
	c := New()
	invalidData := bytes.NewReader([]byte("not gzip data"))

	_, err := c.Reader(invalidData)
	if err == nil {
		t.Error("Reader() expected error for invalid gzip data, got nil")
	}
}
