// Package gzipcodec provides a gzip compression codec.
package gzipcodec

import (
	"compress/gzip"
	"io"

	"github.com/discochess/stockpile/internal/codec"
)

// Compile-time check that Codec implements codec.Codec.
var _ codec.Codec = (*Codec)(nil)

// Codec implements gzip compression.
type Codec struct{}

// New returns a new gzip codec.
func New() *Codec {
	return &Codec{}
}

// Reader wraps r to decompress gzip data.
func (c *Codec) Reader(r io.Reader) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}

// Writer wraps w to compress data with gzip.
func (c *Codec) Writer(w io.Writer) (io.WriteCloser, error) {
	return gzip.NewWriter(w), nil
}

// Extension returns "gz".
func (c *Codec) Extension() string {
	return "gz"
}
