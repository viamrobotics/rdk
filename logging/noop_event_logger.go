//go:build !windows

package logging

// RegisterEventLogger does nothing on Unix. On Windows it will add an `Appender` for logging to
// windows event system.
func RegisterEventLogger(rootLogger Logger) {}
