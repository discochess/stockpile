// Package zstdcodec provides a zstd compression codec.
package zstdcodec

import (
	"io"

	"github.com/klauspost/compress/zstd"

	"github.com/discochess/stockpile/internal/codec"
)

// Compile-time check that Codec implements codec.Codec.
var _ codec.Codec = (*Codec)(nil)

// Codec implements zstd compression.
type Codec struct{}

// New returns a new zstd codec.
func New() *Codec {
	return &Codec{}
}

// Reader wraps r to decompress zstd data.
func (c *Codec) Reader(r io.Reader) (io.ReadCloser, error) {
	decoder, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}
	return decoder.IOReadCloser(), nil
}

// Writer wraps w to compress data with zstd.
func (c *Codec) Writer(w io.Writer) (io.WriteCloser, error) {
	return zstd.NewWriter(w)
}

// Extension returns "zst".
func (c *Codec) Extension() string {
	return "zst"
}
