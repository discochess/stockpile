// Package codec provides compression and decompression for shard data.
package codec

import "io"

// Codec provides compression and decompression functionality.
type Codec interface {
	// Reader wraps r to decompress data read from it.
	Reader(r io.Reader) (io.ReadCloser, error)
	// Writer wraps w to compress data written to it.
	Writer(w io.Writer) (io.WriteCloser, error)
	// Extension returns the file extension without dot (e.g., "zst", "gz").
	// Returns empty string for no compression.
	Extension() string
}
