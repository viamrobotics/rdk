//go:build !windows

package logging

import "io"

// RegisterETWLogger does nothing on non-Windows platforms. On Windows it
// registers an ETW provider as an Appender and starts a session capturing
// those events to a .etl file.
func RegisterETWLogger(rootLogger Logger, name, etlPath string) io.Closer {
	return nopCloser{}
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
