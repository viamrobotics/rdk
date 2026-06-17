//go:build windows

package logging

import (
	"testing"

	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/Microsoft/go-winio/pkg/guid"
	"go.viam.com/test"
)

func TestWindowsNulls(t *testing.T) {
	logger := NewLogger("nulls")
	RegisterEventLogger(logger, "viam-server")
	logger.Info("this \x00 is a null")
	err := logger.Sync()
	test.That(t, err, test.ShouldBeNil)
}

// Same test as above, but for the ETW-based logger
func TestETWNulls(t *testing.T) {
	logger := NewLogger("etw-register-test")

	etlDir := t.TempDir()
	closer, err := RegisterETWLogger(logger, etlDir, ServerETW)
	test.That(t, closer, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	defer closer.Close()

	logger.Info("message through registered ETW appender")
	err = logger.Sync()
	test.That(t, err, test.ShouldBeNil)

	logger.Info("this \x00 is a null")
	err = logger.Sync()
	test.That(t, err, test.ShouldBeNil)
}

// TestETWAppenderLifecycle exercises Write/Close/Write-after-close on the
// appender directly (no session). Doesn't depend on logman or admin
// privileges — provider registration is unprivileged.
func TestETWAppenderLifecycle(t *testing.T) {
	g, err := guid.FromString(ServerETW.ProviderGUID)
	test.That(t, err, test.ShouldBeNil)

	provider, err := etw.NewProviderWithID("viam-server-test", g, nil)
	test.That(t, err, test.ShouldBeNil)

	a := &etwAppender{provider: provider, session: nil}

	logger := NewLogger("etw-test")
	logger.AddAppender(a)

	logger.Info("hello")
	logger.Info("contains \x00 a null")
	logger.Warn("warn level")
	logger.Error("error level")

	test.That(t, a.Close(), test.ShouldBeNil)

	// Write after close: there's no active session, so the provider is disabled
	// Must not panic.
	logger.Info("after close")
}
