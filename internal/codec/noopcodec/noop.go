// Package noopcodec provides a no-op codec (no compression).
package noopcodec

import (
	"io"

	"github.com/discochess/stockpile/internal/codec"
)

// Compile-time check that Codec implements codec.Codec.
var _ codec.Codec = (*Codec)(nil)

// Codec implements no compression.
type Codec struct{}

// New returns a new no-op codec.
func New() *Codec {
	return &Codec{}
}

// Reader returns r wrapped as a ReadCloser (no decompression).
func (c *Codec) Reader(r io.Reader) (io.ReadCloser, error) {
	if rc, ok := r.(io.ReadCloser); ok {
		return rc, nil
	}
	return io.NopCloser(r), nil
}

// Writer returns w wrapped as a WriteCloser (no compression).
func (c *Codec) Writer(w io.Writer) (io.WriteCloser, error) {
	if wc, ok := w.(io.WriteCloser); ok {
		return wc, nil
	}
	return &nopWriteCloser{w}, nil
}

// Extension returns empty string.
func (c *Codec) Extension() string {
	return ""
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }
