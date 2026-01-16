//go:build unix

package stacktrace

import (
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/logging"
)

func TestStackTraceSignalHandlerReceivesSignal(t *testing.T) {
	// Use an observed logger to capture log output
	logger, logObserver := logging.NewObservedTestLogger(t)

	_, cleanup := NewSignalHandler(logger)
	defer cleanup()

	// Send SIGUSR1 to ourselves
	err := syscall.Kill(os.Getpid(), syscall.SIGUSR1)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		// Check that the stack trace was logged
		logs := logObserver.All()
		foundSignalLog := false
		for _, log := range logs {
			if strings.Contains(log.Message, "Received SIGUSR1") {
				foundSignalLog = true
				break
			}
		}
		test.That(tb, foundSignalLog, test.ShouldBeTrue)
	})
}

func TestStackTraceHandlerSetCallback(t *testing.T) {
	logger, _ := logging.NewObservedTestLogger(t)

	handler, cleanup := NewSignalHandler(logger)
	defer cleanup()

	// Track if callback was called
	var callbackCalled atomic.Bool

	handler.SetCallback(func() {
		callbackCalled.Store(true)
	})

	// Send SIGUSR1 to ourselves
	err := syscall.Kill(os.Getpid(), syscall.SIGUSR1)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		// Verify the callback was invoked
		test.That(tb, callbackCalled.Load(), test.ShouldBeTrue)
	})
}

func TestStackTraceHandlerCallbackCanBeChanged(t *testing.T) {
	logger, _ := logging.NewObservedTestLogger(t)

	handler, cleanup := NewSignalHandler(logger)
	defer cleanup()

	// Track which callback was called
	var callback1Called atomic.Bool
	var callback2Called atomic.Bool

	handler.SetCallback(func() {
		callback1Called.Store(true)
	})

	// Change to second callback
	handler.SetCallback(func() {
		callback2Called.Store(true)
	})

	// Send SIGUSR1 to ourselves
	err := syscall.Kill(os.Getpid(), syscall.SIGUSR1)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		// Verify only the second callback was invoked
		test.That(tb, callback1Called.Load(), test.ShouldBeFalse)
		test.That(tb, callback2Called.Load(), test.ShouldBeTrue)
	})
}
