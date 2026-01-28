//go:build unix

package stacktrace

import (
	"os"
	"syscall"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/logging"
)

func TestStackTraceSignalHandlerReceivesSignal(t *testing.T) {
	// Use an observed logger to capture log output
	logger, logs := logging.NewObservedTestLogger(t)

	cleanup := NewSignalHandler(logger)
	defer cleanup()

	// Send SIGUSR1 to ourselves
	err := syscall.Kill(os.Getpid(), syscall.SIGUSR1)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessageSnippet("Received SIGUSR1, dumping stack traces").Len(), test.ShouldEqual, 1)
	})
}
