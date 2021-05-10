package utils

import (
	"io"
)

// TryClose attempts to close the target if it implements
// the right interface.
func TryClose(target interface{}) error {
	closer, ok := target.(io.Closer)
	if !ok {
		return nil
	}
	return closer.Close()
}

func ReadBytes(r io.Reader, toRead int) ([]byte, error) {
	buf := make([]byte, toRead)
	pos := 0

	for pos < toRead {
		n, err := r.Read(buf[pos:])
		if err != nil {
			return nil, err
		}
		pos += n
	}
	return buf, nil
}
