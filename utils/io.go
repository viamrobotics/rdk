package utils

import "io"

// TryClose attempts to close the target if it implements
// the right interface.
func TryClose(target interface{}) error {
	closer, ok := target.(io.Closer)
	if !ok {
		return nil
	}
	return closer.Close()
}
