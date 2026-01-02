//go:build windows

package logging

import (
	"testing"

	"go.viam.com/test"
)

func TestWindowsNulls(t *testing.T) {
	logger := NewLogger("nulls")
	RegisterEventLogger(logger, "viam-server")
	logger.Info("this \x00 is a null")
	err := logger.Sync()
	test.That(t, err, test.ShouldBeNil)
}
