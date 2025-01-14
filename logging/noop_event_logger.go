//go:build !windows

package logging

func RegisterEventLogger(rootLogger Logger) {}
