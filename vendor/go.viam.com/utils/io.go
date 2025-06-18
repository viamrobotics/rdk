package utils

import (
	"context"
	"io"
)

// ReadBytes ensures that all bytes requested to be read
// are read into a slice unless an error occurs. If the reader
// never returns the amount of bytes requested, this will block
// until the given context is done.
func ReadBytes(ctx context.Context, r io.Reader, toRead int) ([]byte, error) {
	buf := make([]byte, toRead)
	pos := 0

	for pos < toRead {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		n, err := r.Read(buf[pos:])
		if err != nil {
			return nil, err
		}
		pos += n
	}
	return buf, nil
}
